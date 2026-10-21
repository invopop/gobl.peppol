package bis

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

func taxComboRules() *rules.Set {
	return rules.For(new(tax.Combo),
		rules.Field("ext",
			rules.Assert("01", "tax category code must match the VATEX exemption reason code (PEPPOL-EN16931-P0104..P0111)",
				is.Func("VATEX matches tax category", extVATEXMatchesCategory),
			),
		),
	)
}

// extVATEXMatchesCategory reports whether the UNTDID 5305 tax category code is
// the one Peppol requires for the CEF VATEX exemption reason code.
func extVATEXMatchesCategory(val any) bool {
	ext, ok := val.(*tax.Extensions)
	if !ok || ext == nil {
		return false
	}
	category := ext.Get(untdid.ExtKeyTaxCategory)
	switch ext.Get(cef.ExtKeyVATEX) {
	case "VATEX-EU-G":
		return category == en16931.TaxCategoryExport
	case "VATEX-EU-O":
		return category == en16931.TaxCategoryOutsideScope
	case "VATEX-EU-IC":
		return category == en16931.TaxCategoryIntraCommunity
	case "VATEX-EU-AE":
		return category == en16931.TaxCategoryReverseCharge
	case "VATEX-EU-D", "VATEX-EU-F", "VATEX-EU-I", "VATEX-EU-J":
		return category == en16931.TaxCategoryExempt
	default:
		return true
	}
}
