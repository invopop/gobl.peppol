package aunz

import (
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// taxCategoryTotalRules enforces the A-NZ rules over the calculated tax
// breakdown of the invoice totals.
func taxCategoryTotalRules() *rules.Set {
	return rules.For(new(tax.CategoryTotal),
		rules.Assert("01", "GST breakdown not subject to tax cannot be combined with other tax categories (ALIGNED-IBRP-O-11-AUNZ)",
			is.Func("outside scope tax is exclusive", categoryTotalOutsideScopeIsExclusive),
		),
	)
}

// categoryTotalOutsideScopeIsExclusive reports whether a GST rate breakdown
// outside the scope of tax ("O") is the only breakdown in the category, as the
// specification forbids combining it with any other tax category.
func categoryTotalOutsideScopeIsExclusive(val any) bool {
	ct, ok := val.(*tax.CategoryTotal)
	if !ok || ct == nil || ct.Code != tax.CategoryGST {
		return true
	}
	var outside, other bool
	for _, rate := range ct.Rates {
		if rate == nil {
			continue
		}
		if rate.Ext.Get(untdid.ExtKeyTaxCategory) == taxCategoryOutsideScope {
			outside = true
		} else {
			other = true
		}
	}
	return !outside || !other
}
