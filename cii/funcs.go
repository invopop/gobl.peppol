package cii

import (
	"strings"

	"github.com/invopop/gobl"
	goblcii "github.com/invopop/gobl.cii"
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
