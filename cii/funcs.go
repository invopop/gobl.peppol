package cii

import (
	"fmt"
	"strings"

	"github.com/invopop/gobl"
	goblcii "github.com/invopop/gobl.cii"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// exportBISBilling applies the Peppol BIS Billing rules to an exported
// invoice.
func exportBISBilling(_ *goblcii.Format, _ *gobl.Envelope, doc goblcii.Document) error {
	out, ok := doc.(*goblcii.Invoice)
	if !ok || out.ExchangedDocument == nil {
		return nil
	}
	out.ExchangedDocument.IncludedNote = joinNotes(out.ExchangedDocument.IncludedNote)
	return nil
}

// joinNotes merges the notes into one, as Peppol BIS allows a single note on
// the document (PEPPOL-EN16931-R002), keeping the subject code only when every
// note shares it.
func joinNotes(notes []*goblcii.Note) []*goblcii.Note {
	if len(notes) <= 1 {
		return notes
	}
	texts := make([]string, 0, len(notes))
	subject := notes[0].SubjectCode
	for _, n := range notes {
		texts = append(texts, n.Content)
		if n.SubjectCode != subject {
			subject = ""
		}
	}
	return []*goblcii.Note{{Content: strings.Join(texts, "\n\n"), SubjectCode: subject}}
}

// importEndpoints adds the seller's (BT-34) and buyer's (BT-49) electronic
// address as an endpoint, since Peppol documents always arrive through Peppol
// whatever the EAS scheme, and keeps the inbox the base import reads it into.
func importEndpoints(_ *goblcii.Format, doc goblcii.Document, env *gobl.Envelope) error {
	inv, ok := env.Extract().(*bill.Invoice)
	in, ok2 := doc.(*goblcii.Invoice)
	if !ok || !ok2 || in.Transaction == nil || in.Transaction.Agreement == nil {
		return nil
	}
	if p := in.Transaction.Agreement.Seller; p != nil && inv.Supplier != nil {
		importPartyEndpoint(inv.Supplier, p.URIUniversalCommunication)
	}
	if p := in.Transaction.Agreement.Buyer; p != nil && inv.Customer != nil {
		importPartyEndpoint(inv.Customer, p.URIUniversalCommunication)
	}
	return nil
}

// importPartyEndpoint adds the party's endpoint for an EAS scheme and address,
// as an ISO 6523 URI, unless it already has one for that URI scheme.
func importPartyEndpoint(party *org.Party, uc *goblcii.URIUniversalCommunication) {
	if uc == nil || uc.ID == nil || uc.ID.SchemeID == "" || uc.ID.Value == "" {
		return
	}
	uri := cbc.URI(fmt.Sprintf("%s::%s:%s", iso.ActorIDScheme, uc.ID.SchemeID, uc.ID.Value))
	if party.Endpoint(uri.Scheme()) == nil {
		party.Endpoints = append(party.Endpoints, &org.Endpoint{URI: uri})
	}
}
