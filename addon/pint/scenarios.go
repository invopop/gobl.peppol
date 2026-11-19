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
	// TagOutsideTaxScope marks a PINT AE document as out of scope of VAT,
	// the UAE "commercial invoice".
	TagOutsideTaxScope cbc.Key = "outside-tax-scope"

	// UNTDID 1001 codes PINT AE uses for out-of-scope documents. 480 is in
	// UNTDID D.23A but not in the EN 16931 code list, so it lives here.
	docTypeInvoiceOutsideScope     cbc.Code = "480"
	docTypeCreditNoteGoodsServices cbc.Code = "81"

	// UNTDID 1001 codes the EN 16931 scenarios set.
	docTypeCommercialInvoice    cbc.Code = "380"
	docTypeCreditNote           cbc.Code = "381"
	docTypeSelfBilledCreditNote cbc.Code = "261"
	docTypeSelfBilledInvoice    cbc.Code = "389"
)

// docTypesAE are the UNTDID 1001 codes PINT AE accepts: ibr-cl-01's list for
// billing, and the self-billing codes.
var docTypesAE = []cbc.Code{
	docTypeCommercialInvoice,
	docTypeInvoiceOutsideScope,
	docTypeCreditNote,
	docTypeCreditNoteGoodsServices,
	docTypeSelfBilledInvoice,
	docTypeSelfBilledCreditNote,
}

// invoiceTags are the document tags the PINT addon adds to invoices.
var invoiceTags = &tax.TagSet{
	Schema: bill.ShortSchemaInvoice,
	List: append([]*cbc.Definition{
		{
			Key: TagOutsideTaxScope,
			Name: i18n.String{
				i18n.EN: "Outside Tax Scope",
			},
			Desc: i18n.String{
				i18n.EN: "Marks a PINT AE document as out of scope of VAT, the UAE 'commercial invoice'. Invoices are sent with document type 480 and credit notes with 81. Only for UAE documents.",
			},
		},
	}, transactionTypeTags...),
}

// invoiceScenarios set 480 and 81 on out-of-scope documents, replacing the 380 or 381 that EN 16931 sets first.
var invoiceScenarios = &tax.ScenarioSet{
	Schema: bill.ShortSchemaInvoice,
	List: []*tax.Scenario{
		// ** Out of scope of VAT (UAE only) **
		{
			Types:  []cbc.Key{bill.InvoiceTypeStandard},
			Tags:   []cbc.Key{TagOutsideTaxScope},
			Filter: invoiceIsAE,
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				untdid.ExtKeyDocumentType: docTypeInvoiceOutsideScope,
			}),
		},
		{
			Types:  []cbc.Key{bill.InvoiceTypeCreditNote},
			Tags:   []cbc.Key{TagOutsideTaxScope},
			Filter: invoiceIsAE,
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				untdid.ExtKeyDocumentType: docTypeCreditNoteGoodsServices,
			}),
		},
	},
}

// invoiceIsAE reports whether the scenario document is a UAE invoice, as the addon's scenarios
// apply under every regime.
func invoiceIsAE(doc any) bool {
	inv, ok := doc.(*bill.Invoice)
	return ok && inv != nil && inv.GetRegime() == l10n.AE.Tax()
}
