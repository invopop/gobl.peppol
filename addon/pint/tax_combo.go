package pint

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

var taxCategoryMap = tax.ExtensionsOf(cbc.CodeMap{
	tax.KeyStandard:       en16931.TaxCategoryStandard,
	tax.KeyZero:           en16931.TaxCategoryZero,
	tax.KeyExempt:         en16931.TaxCategoryExempt,
	tax.KeyReverseCharge:  en16931.TaxCategoryReverseCharge,
	tax.KeyIntraCommunity: en16931.TaxCategoryIntraCommunity,
	tax.KeyExport:         en16931.TaxCategoryExport,
	tax.KeyOutsideScope:   en16931.TaxCategoryOutsideScope,
})

var taxCategoryCodesAUNZ = []cbc.Code{
	en16931.TaxCategoryExempt,
	en16931.TaxCategoryStandard,
	en16931.TaxCategoryZero,
	en16931.TaxCategoryExport,
	en16931.TaxCategoryOutsideScope,
}

// Profit margin scheme lines keep E in GOBL and export as N in UBL.
var taxCategoryCodesAE = []cbc.Code{
	en16931.TaxCategoryExempt,
	en16931.TaxCategoryStandard,
	en16931.TaxCategoryZero,
	en16931.TaxCategoryOutsideScope,
	en16931.TaxCategoryReverseCharge,
}

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
		rules.When(
			is.InContext(tax.RegimeIn(regimesAUNZ...)),
			rules.Field("ext",
				rules.Assert("02", "tax category code must be an A-NZ UNCL5305 code: E, S, Z, G or O (ALIGNED-IBRP-CL-01-AUNZ)",
					tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesAUNZ...),
				),
			),
			rules.When(
				is.Func("tax combo is standard rated", taxComboIsStandard),
				rules.Field("percent",
					rules.Assert("03", "tax percentage must be greater than 0 (ALIGNED-IBRP-S-05/06/07-AUNZ)",
						num.Positive,
					),
				),
			),
			rules.When(
				is.Func("tax combo is zero rated", taxComboIsZero),
				rules.Field("percent",
					rules.Assert("04", "tax percentage must be 0.00 (ALIGNED-IBRP-Z-05/06/07-AUNZ)",
						num.Equals(num.MakePercentage(0, 2)),
					),
				),
			),
		),

		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.Field("ext",
				rules.Assert("05", "tax category code must be an AE UNCL5305 code: E, S, Z, O or AE (IBR-139-AE)",
					tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesAE...),
				),
			),

			rules.When(
				is.Func("tax combo is standard rated", taxComboIsStandard),
				rules.Field("percent",
					rules.Assert("06", "tax percentage must be 5.00 (IBR-190-AE, ALIGNED-IBRP-S-05/06/07)",
						num.Equals(num.MakePercentage(5, 2)),
					),
				),
			),

			rules.When(
				is.Func("tax combo is zero rated", taxComboIsZero),
				rules.Field("percent",
					rules.Assert("07", "tax percentage must be 0.00 (ALIGNED-IBRP-Z-05/06/07, IBR-120-AE)",
						num.Equals(num.MakePercentage(0, 2)),
					),
				),
			),

			// The Schematron doesn't check PINT AE's code list.
			rules.Field("ext",
				rules.Assert("08", "VAT exemption reason must be one of PINT AE's: DL8.46.1, DL8.46.2, DL8.46.3 or DL8.46.4 (code list Aligned-TaxExemptionCodes)",
					tax.ExtensionHasValidCode(ExtKeyExemption),
				),
			),
		),
	)
}

func taxComboIsStandard(val any) bool {
	tc, ok := val.(*tax.Combo)
	return ok && tc != nil && tc.Key == tax.KeyStandard
}

func taxComboIsZero(val any) bool {
	tc, ok := val.(*tax.Combo)
	return ok && tc != nil && tc.Key == tax.KeyZero
}
