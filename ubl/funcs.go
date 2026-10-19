package ubl

import (
	"slices"
	"strings"

	"github.com/invopop/gobl"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// exportBISBilling applies the Peppol BIS Billing rules to an exported
// invoice.
func exportBISBilling(_ *goblubl.Format, _ *gobl.Envelope, doc goblubl.Document) error {
	out, ok := doc.(*goblubl.Invoice)
	if !ok {
		return nil
	}
	if len(out.Note) > 1 {
		out.Note = []string{strings.Join(out.Note, "\n\n")}
	}
	return nil
}

// exportInvoiceResponse applies the Peppol Invoice Response data model to an
// exported application response.
func exportInvoiceResponse(_ *goblubl.Format, env *gobl.Envelope, doc goblubl.Document) error {
	st, ok := env.Extract().(*bill.Status)
	out, ok2 := doc.(*goblubl.ApplicationResponse)
	if !ok || !ok2 {
		return nil
	}
	out.UBLVersionID = ""
	out.UUID = ""
	trimToResponseParty(out.SenderParty)
	trimToResponseParty(out.ReceiverParty)
	for i, dr := range out.DocumentResponse {
		dr.Response.Description = nil
		exportDocumentResponse(dr, st.Lines[i])
	}
	return nil
}

// importInvoiceResponse maps the Peppol Invoice Response codes of an imported
// application response.
func importInvoiceResponse(_ *goblubl.Format, doc goblubl.Document, env *gobl.Envelope) error {
	in, ok := doc.(*goblubl.ApplicationResponse)
	st, ok2 := env.Extract().(*bill.Status)
	if !ok || !ok2 {
		return nil
	}
	for i, dr := range in.DocumentResponse {
		if dr != nil {
			importStatusLine(st.Lines[i], dr)
		}
	}
	return nil
}

// importPINT applies the Peppol PINT rules to an imported invoice.
func importPINT(_ *goblubl.Format, doc goblubl.Document, env *gobl.Envelope) error {
	inv, ok := env.Extract().(*bill.Invoice)
	in, ok2 := doc.(*goblubl.Invoice)
	if !ok || !ok2 {
		return nil
	}
	for _, p := range goblubl.InvoiceParties(inv, in) {
		importPartyTaxID(p.UBL, p.GOBL)
	}
	return nil
}

// importPartyTaxID promotes the party's tax registration to its tax ID. PINT
// replaces the EN 16931 VAT scheme with the jurisdiction's own.
func importPartyTaxID(in *goblubl.Party, out *org.Party) {
	if out.TaxID != nil {
		return
	}
	for _, pts := range in.PartyTaxScheme {
		if pts.CompanyID == nil || pts.CompanyID.Value == "" || pts.TaxScheme == nil {
			continue
		}
		code := cbc.Code(pts.CompanyID.Value)
		out.TaxID = &tax.Identity{
			Country: l10n.TaxCountryCode(in.CountryCode()),
			Code:    code,
		}
		if scheme := cbc.Code(pts.TaxScheme.ID.Value); out.TaxID.GetScheme() != scheme {
			out.TaxID.Scheme = scheme
		}
		out.Identities = slices.DeleteFunc(out.Identities, func(id *org.Identity) bool {
			return id.Scope == org.IdentityScopeTax && id.Code == code
		})
		return
	}
}
