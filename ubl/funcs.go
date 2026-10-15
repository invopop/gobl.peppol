package ubl

import (
	"slices"

	"github.com/invopop/gobl"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

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
// replaces the EN 16931 VAT scheme with the jurisdiction's own, GST in A-NZ,
// which the base import reads as a plain tax registration instead of BT-31.
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
