package bis

import (
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// vatexTaxCategoryMap binds each CEF VATEX exemption reason code that Peppol
// pins to its required UNTDID 5305 tax category code
// (PEPPOL-EN16931-P0104 through P0111).
var vatexTaxCategoryMap = map[cbc.Code]cbc.Code{
	"VATEX-EU-G":  "G",
	"VATEX-EU-O":  "O",
	"VATEX-EU-IC": "K",
	"VATEX-EU-AE": "AE",
	"VATEX-EU-D":  "E",
	"VATEX-EU-F":  "E",
	"VATEX-EU-I":  "E",
	"VATEX-EU-J":  "E",
}

func taxComboRules() *rules.Set {
	return rules.For(new(tax.Combo),
		rules.Assert("01", "tax category code must match the VATEX exemption reason code (PEPPOL-EN16931-P0104..P0111)",
			is.Func("VATEX matches tax category", comboVATEXMatchesCategory),
		),
	)
}

// comboVATEXMatchesCategory reports whether, when a Peppol-pinned VATEX code is
// present, the tax category code matches the code that VATEX code requires.
// VATEX codes outside the pinned set are not constrained here.
func comboVATEXMatchesCategory(val any) bool {
	tc, ok := val.(*tax.Combo)
	if !ok || tc == nil {
		return true
	}
	vatex := tc.Ext.Get(cef.ExtKeyVATEX)
	if vatex.IsEmpty() {
		return true
	}
	want, pinned := vatexTaxCategoryMap[vatex]
	if !pinned {
		return true
	}
	return tc.Ext.Get(untdid.ExtKeyTaxCategory) == want
}
