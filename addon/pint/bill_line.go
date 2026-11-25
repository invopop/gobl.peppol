package pint

import (
	"github.com/invopop/gobl.peppol/catalogues/uae"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// billLineRules validates invoice lines on PINT documents, with stricter rules for A-NZ and UAE.
func billLineRules() *rules.Set {
	return rules.For(new(bill.Line),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAUNZ...)),
			rules.Field("taxes",
				rules.Assert("01", "line must have a GST tax combo (ALIGNED-IBRP-050-AUNZ)",
					tax.SetHasCategory(tax.CategoryGST),
				),
			),
		),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.Field("taxes",
				rules.Assert("02", "line must have a VAT tax combo (IBR-145-AE)",
					tax.SetHasCategory(tax.CategoryVAT),
				),
			),
			rules.Field("item",
				rules.Field("description",
					rules.Assert("03", "item description is required (IBR-125-AE)", is.Present),
				),
			),
			rules.When(
				is.Func("reverse charge VAT", lineHasReverseChargeVAT),
				rules.Field("item",
					rules.Field("ext",
						rules.Assert("04", "reverse charge line item needs the type of goods or services in the ae-goods-type extension (IBR-166-AE)",
							tax.ExtensionsRequire(uae.ExtKeyGoodsType),
						),
					),
				),
			),
			// The IBR-006-AE test checks every goods type, not only those of reverse charge lines as its text says.
			rules.Field("item",
				rules.Field("ext",
					rules.Assert("05", "type of goods or services must be from the PINT AE code list (IBR-006-AE)",
						tax.ExtensionHasValidCode(uae.ExtKeyGoodsType),
					),
				),
			),
			rules.When(
				is.Func("reverse charge VAT", lineHasReverseChargeVAT),
				rules.Field("item",
					rules.Field("identities",
						rules.Assert("06", `reverse charge line item needs a GTIN identity (key "gtin") (IBR-174-AE)`,
							org.IdentitiesExtensionIn(iso.ExtKeySchemeID, IdentitySchemeGTIN),
						),
					),
				),
			),
		),
		// A-NZ and AE ask for their own category above.
		rules.When(
			is.Not(is.InContext(tax.RegimeIn(append(regimesAUNZ, regimesAE...)...))),
			rules.Field("taxes",
				rules.Assert("07", "line taxes are required (ibr-sr-58)", is.Present),
			),
		),
	)
}

// lineHasReverseChargeVAT reports whether the line's VAT category is AE, reverse charge.
func lineHasReverseChargeVAT(val any) bool {
	l, ok := val.(*bill.Line)
	if !ok || l == nil {
		return false
	}
	c := l.Taxes.Get(tax.CategoryVAT)
	return c != nil && c.Ext.Get(untdid.ExtKeyTaxCategory) == en16931.TaxCategoryReverseCharge
}
