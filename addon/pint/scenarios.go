package pint

import (
	"slices"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/tax"
)

// TagVolumeDiscount marks a UAE volume discount credit note (BTAE-03 reason "VD"),
// which must not carry a preceding invoice reference (IBR-055-AE).
const TagVolumeDiscount cbc.Key = "volume-discount"

// UNTDID 1001 document types.
const (
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
	List: append(slices.Clone(transactionTypeTags), &cbc.Definition{
		Key:  TagVolumeDiscount,
		Name: i18n.String{i18n.EN: "Volume Discount"},
		Desc: i18n.String{i18n.EN: "PINT AE: credit note issued for a volume discount (reason VD). Must not have a preceding invoice reference."},
	}),
}

// invoiceScenarios set 480 and 81 on UAE documents whose lines are all exempt or outside the scope of
// VAT, as PINT AE requires (IBR-151-AE, IBR-122-AE), replacing the 380 or 381 that EN 16931 sets first.
var invoiceScenarios = &tax.ScenarioSet{
	Schema: bill.ShortSchemaInvoice,
	List: []*tax.Scenario{
		{
			Types:  []cbc.Key{bill.InvoiceTypeStandard},
			Filter: invoiceIsOutsideTaxScopeAE,
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				untdid.ExtKeyDocumentType: docTypeInvoiceOutsideScope,
			}),
		},
		{
			Types:  []cbc.Key{bill.InvoiceTypeCreditNote},
			Filter: invoiceIsOutsideTaxScopeAE,
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				untdid.ExtKeyDocumentType: docTypeCreditNoteGoodsServices,
			}),
		},
	},
}

// invoiceIsOutsideTaxScopeAE reports whether the scenario document is a UAE invoice whose lines are all
// exempt or outside the scope of VAT. Self-billed documents keep 389 and 261.
func invoiceIsOutsideTaxScopeAE(doc any) bool {
	inv, ok := doc.(*bill.Invoice)
	return ok && inv != nil && inv.GetRegime() == l10n.AE.Tax() && !inv.HasTags(tax.TagSelfBilled) &&
		len(inv.Lines) > 0 && !invoiceHasLineComboOtherThanEorO(inv)
}
