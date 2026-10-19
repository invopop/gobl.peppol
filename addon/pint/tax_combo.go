package pint

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
)

// taxCategoryMap maps GOBL tax keys to the UNTDID 5305 tax category codes that
// PINT recognises, mirroring EN 16931's VAT key mapping.
var taxCategoryMap = tax.ExtensionsOf(cbc.CodeMap{
	tax.KeyStandard:       en16931.TaxCategoryStandard,
	tax.KeyZero:           en16931.TaxCategoryZero,
	tax.KeyExempt:         en16931.TaxCategoryExempt,
	tax.KeyReverseCharge:  en16931.TaxCategoryReverseCharge,
	tax.KeyIntraCommunity: en16931.TaxCategoryIntraCommunity,
	tax.KeyExport:         en16931.TaxCategoryExport,
	tax.KeyOutsideScope:   en16931.TaxCategoryOutsideScope,
})

// normalizeTaxCombo re-maps the UNTDID tax category code from the tax combo's
// key using the EN 16931 key-to-code mapping.
func normalizeTaxCombo(tc *tax.Combo) {
	if tc == nil || tc.Key.IsEmpty() {
		return
	}
	if code := taxCategoryMap.Get(tc.Key); code != cbc.CodeEmpty {
		tc.Ext = tc.Ext.Set(untdid.ExtKeyTaxCategory, code)
	}
}

func taxComboRules() *rules.Set {
	return rules.For(new(tax.Combo),
		// PINT recognises GST (and other non-VAT schemes) as standard, zero or
		// exempt rated, so the EN 16931 rule that forces every non-VAT category
		// to outside scope must not apply here.
		rules.Ignore("GOBL-EU-EN16931-TAX-COMBO-05"),

		rules.Field("ext",
			rules.Assert("01", "invoice line tax category code must be a recognised PINT UNTDID 5305 code",
				tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryMap.Values()...),
			),
		),
	)
}
