package pint

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// billLineRules enforces the PINT AE line rules on UAE documents.
func billLineRules() *rules.Set {
	return rules.For(new(bill.Line),
		// TODO: IBR-145-AE matches ALIGNED-IBRP-050/059-AUNZ (GST). Extend to
		// A-NZ after discussing it with colleagues.
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.Field("taxes",
				rules.Assert("01", "line must have a VAT tax combo (IBR-145-AE)",
					tax.SetHasCategory(tax.CategoryVAT),
				),
			),
			rules.Field("item",
				rules.Field("description",
					rules.Assert("02", "item description is required (IBR-125-AE)", is.Present),
				),
			),
			rules.When(
				is.Func("reverse charge VAT", lineHasReverseChargeVAT),
				rules.Field("item",
					rules.Field("ext",
						rules.Assert("03", "reverse charge line item needs the type of goods or services (IBR-166-AE)",
							tax.ExtensionsRequire(ExtKeyGoodsType),
						),
					),
				),
			),
			// The IBR-006-AE test checks every goods type, not only those of
			// reverse charge lines as its text says.
			rules.Field("item",
				rules.Field("ext",
					rules.Assert("04", "type of goods or services must be from the PINT AE code list (IBR-006-AE)",
						tax.ExtensionHasValidCode(ExtKeyGoodsType),
					),
				),
			),
			rules.When(
				is.Func("reverse charge VAT", lineHasReverseChargeVAT),
				rules.Field("item",
					rules.Field("identities",
						rules.Assert("05", "reverse charge line item needs a GTIN identity, scheme 0160 (IBR-174-AE)",
							org.IdentitiesExtensionIn(iso.ExtKeySchemeID, IdentitySchemeGTIN),
						),
					),
				),
			),
		),
	)
}

// lineHasReverseChargeVAT reports whether a line's VAT combo has the reverse
// charge category AE, the code gobl.ubl writes as the line's
// cac:ClassifiedTaxCategory/cbc:ID and IBR-103-AE, IBR-166-AE and IBR-174-AE
// test.
func lineHasReverseChargeVAT(val any) bool {
	l, ok := val.(*bill.Line)
	if !ok || l == nil {
		return false
	}
	c := l.Taxes.Get(tax.CategoryVAT)
	return c != nil && c.Ext.Get(untdid.ExtKeyTaxCategory) == en16931.TaxCategoryReverseCharge
}
