package pint

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/tax"
)

// Document tags and the UNTDID 1001 codes they map to.
const (
	// TagOutsideScope marks a PINT AE document as out of scope of VAT,
	// the UAE "commercial invoice".
	TagOutsideScope cbc.Key = "outside-scope"

	// UNTDID 1001 codes PINT AE uses for out-of-scope documents. 480 is in
	// UNTDID D.23A but not in the EN 16931 code list, so it lives here.
	docTypeInvoiceOutsideScope     cbc.Code = "480"
	docTypeCreditNoteGoodsServices cbc.Code = "81"

	// UNTDID 1001 codes the EN 16931 scenarios set on standard invoices and
	// credit notes, which some PINT AE rules check.
	docTypeCommercialInvoice cbc.Code = "380"
	docTypeCreditNote        cbc.Code = "381"
	// docTypeSelfBilledCreditNote is the code the EN 16931 scenarios set on
	// self-billed credit notes.
	docTypeSelfBilledCreditNote cbc.Code = "261"
)

// invoiceTags are the document tags the PINT addon adds to invoices.
var invoiceTags = &tax.TagSet{
	Schema: bill.ShortSchemaInvoice,
	List: append([]*cbc.Definition{
		{
			Key: TagOutsideScope,
			Name: i18n.String{
				i18n.EN: "Outside Scope",
			},
			Desc: i18n.String{
				i18n.EN: "Marks a PINT AE document as out of scope of VAT, the UAE 'commercial invoice'. Invoices are sent with document type 480 and credit notes with 81. Has no effect outside the UAE.",
			},
		},
	}, transactionTypeTags...),
}

// invoiceScenarios set the UNTDID 1001 document type for PINT AE's
// out-of-scope documents. EN 16931's scenarios run first and set 380 or 381
// from the invoice type; these match afterwards and replace the code, the same
// way EN maps the self-billed tag to 389.
var invoiceScenarios = &tax.ScenarioSet{
	Schema: bill.ShortSchemaInvoice,
	List: []*tax.Scenario{
		// ** Out of scope of VAT (UAE only) **
		{
			Types:  []cbc.Key{bill.InvoiceTypeStandard},
			Tags:   []cbc.Key{TagOutsideScope},
			Filter: invoiceIsAE,
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				untdid.ExtKeyDocumentType: docTypeInvoiceOutsideScope,
			}),
		},
		{
			Types:  []cbc.Key{bill.InvoiceTypeCreditNote},
			Tags:   []cbc.Key{TagOutsideScope},
			Filter: invoiceIsAE,
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				untdid.ExtKeyDocumentType: docTypeCreditNoteGoodsServices,
			}),
		},
	},
}

// invoiceIsAE reports whether the scenario document is a UAE invoice. Tags and
// scenarios belong to the addon, which runs for every regime, so the AE-only
// scenarios use it as their filter.
func invoiceIsAE(doc any) bool {
	inv, ok := doc.(*bill.Invoice)
	return ok && inv != nil && inv.GetRegime() == l10n.AE.Tax()
}
