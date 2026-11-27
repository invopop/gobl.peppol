package pint

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/tax"
)

const (
	docTypeInvoiceOutsideScope     cbc.Code = "480"
	docTypeCreditNoteGoodsServices cbc.Code = "81"
	docTypeCommercialInvoice       cbc.Code = "380"
	docTypeCreditNote              cbc.Code = "381"
	docTypeSelfBilledCreditNote    cbc.Code = "261"
	docTypeSelfBilledInvoice       cbc.Code = "389"
)

// Allowed UNTDID 1001 codes for PINT AE (ibr-cl-01).
var docTypesAE = []cbc.Code{
	docTypeCommercialInvoice,
	docTypeInvoiceOutsideScope,
	docTypeCreditNote,
	docTypeCreditNoteGoodsServices,
	docTypeSelfBilledInvoice,
	docTypeSelfBilledCreditNote,
}

// Sets 480/81 on out-of-scope UAE documents (IBR-151-AE, IBR-122-AE).
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

func invoiceIsOutsideTaxScopeAE(doc any) bool {
	inv, ok := doc.(*bill.Invoice)
	return ok && inv != nil && inv.GetRegime() == l10n.AE.Tax() && !inv.HasTags(tax.TagSelfBilled) &&
		!ParseTransactionType(inv.Tax.GetExt(ExtKeyTransactionType)).MarginScheme &&
		len(inv.Lines) > 0 && !invoiceHasLineComboOtherThanEorO(inv)
}
