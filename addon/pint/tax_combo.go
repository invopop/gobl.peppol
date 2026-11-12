package pint

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// TaxCategoryMarginScheme is the PINT AE VAT category for supplies under the
// profit margin scheme, "Standard rate additional VAT". The official code list
// writes it with a Greek capital Nu (U+039D), but the Schematron tests and
// examples use the Latin N, so we follow them.
const TaxCategoryMarginScheme cbc.Code = "N"

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
	ae.TaxKeyMarginScheme: TaxCategoryMarginScheme,
})

// taxCategoryCodesAUNZ lists the subset of UNTDID 5305 tax category codes
// accepted by the A-NZ PINT specification.
var taxCategoryCodesAUNZ = []cbc.Code{
	en16931.TaxCategoryExempt,
	en16931.TaxCategoryStandard,
	en16931.TaxCategoryZero,
	en16931.TaxCategoryExport,
	en16931.TaxCategoryOutsideScope,
}

// taxCategoryCodesAE lists the subset of UNTDID 5305 tax category codes
// accepted by the AE PINT specification.
var taxCategoryCodesAE = []cbc.Code{
	en16931.TaxCategoryExempt,
	en16931.TaxCategoryStandard,
	en16931.TaxCategoryZero,
	en16931.TaxCategoryOutsideScope,
	en16931.TaxCategoryReverseCharge,
	TaxCategoryMarginScheme,
}

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
		rules.When(
			is.InContext(tax.RegimeIn(regimesAUNZ...)),
			rules.Field("ext",
				rules.Assert("02", "tax category code must be an A-NZ UNCL5305 code: E, S, Z, G or O (ALIGNED-IBRP-CL-01-AUNZ)",
					tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesAUNZ...),
				),
			),
		),

		// TODO: rules 04 and 06 should also apply to AUNZ.
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			// EN 16931 checks VAT category codes against its own list, which has
			// no PINT AE margin scheme category (N). Rule 01 checks them against
			// the PINT list instead.
			rules.Ignore("GOBL-EU-EN16931-TAX-COMBO-02"),
			rules.Field("ext",
				rules.Assert("03", "tax category code must be an AE UNCL5305 code: E, S, Z, O, AE or N (IBR-139-AE)",
					tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesAE...),
				),
			),

			rules.When(
				is.Func("tax combo is standard rated", taxComboIsStandard),
				rules.Field("percent",
					rules.Assert("04", "tax percentage must be a positive number (ALIGNED-IBRP-S-05/06/07)",
						num.Positive,
					),
					rules.Assert("05", "tax percentage must be 5.00 (IBR-190-AE)",
						num.Equals(num.MakePercentage(5, 2)),
					),
				),
			),

			rules.When(
				is.Func("tax combo is zero rated", taxComboIsZero),
				rules.Field("percent",
					rules.Assert("06", "tax percentage must be 0.00 (ALIGNED-IBRP-Z-05/06/07, IBR-120-AE)",
						num.Equals(num.MakePercentage(0, 2)),
					),
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
