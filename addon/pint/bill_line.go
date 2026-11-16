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
			rules.Field("taxes", rules.Each(
				rules.When(
					is.Func("exempt VAT", comboIsExemptVAT),
					rules.Field("ext",
						rules.Assert("03", "exempt line needs a UAE VAT exemption reason (IBR-167-AE)",
							tax.ExtensionsRequire(ExtKeyExemption),
						),
					),
				),
			)),
			rules.When(
				is.Func("reverse charge VAT", lineHasReverseChargeVAT),
				rules.Field("item",
					rules.Field("ext",
						rules.Assert("04", "reverse charge line item needs the type of goods or services (IBR-166-AE)",
							tax.ExtensionsRequire(ExtKeyGoodsType),
						),
					),
				),
			),
			// The IBR-006-AE test checks every goods type, not only those of
			// reverse charge lines as its text says.
			rules.Field("item",
				rules.Field("ext",
					rules.Assert("05", "type of goods or services must be from the PINT AE code list (IBR-006-AE)",
						tax.ExtensionHasValidCode(ExtKeyGoodsType),
					),
				),
			),
			rules.When(
				is.Func("reverse charge VAT", lineHasReverseChargeVAT),
				rules.Field("item",
					rules.Field("identities",
						rules.Assert("06", "reverse charge line item needs a GTIN identity, scheme 0160 (IBR-174-AE)",
							org.IdentitiesExtensionIn(iso.ExtKeySchemeID, IdentitySchemeGTIN),
						),
					),
				),
			),
			rules.Field("item",
				rules.When(
					is.Func("goods", itemKeyIs(org.ItemKeyGoods)),
					rules.Field("identities",
						rules.Assert("07", "goods item needs an HS item classification (IBR-184-AE)",
							is.Func("has HS", identitiesHaveLabel(IdentityLabelHS)),
						),
					),
				),
				rules.When(
					is.Func("services", itemKeyIs(org.ItemKeyServices)),
					rules.Field("identities",
						rules.Assert("08", "services item needs a service accounting code (IBR-185-AE)",
							is.Func("has SAC", identitiesHaveLabel(IdentityLabelSAC)),
						),
					),
				),
				rules.When(
					is.Func("goods and services", itemKeyIs(ItemKeyGoodsServices)),
					rules.Field("identities",
						rules.Assert("09", "goods and services item needs an HS item classification and a service accounting code (IBR-186-AE)",
							is.Func("has HS and SAC", identitiesHaveLabel(IdentityLabelHS, IdentityLabelSAC)),
						),
					),
				),
				// gobl.ubl writes identities with a label and no scheme ID as item
				// classifications, with the label as their list. FormatAE writes the
				// SAC ones as service accounting codes instead (IBR-189-AE).
				// IBR-188-AE's text asks for HS on every classification, but its
				// test passes when any one is HS.
				rules.Field("identities",
					rules.Assert("10", "item classifications must include one from the HS list (IBR-188-AE)",
						is.Func("classifications include HS", identitiesClassifiedWithHS),
					),
				),
			),
			rules.Field("seller",
				rules.Assert("11", "line seller region must be an emirate code when the country is AE (IBR-128-AE)",
					is.Func("postal address region is an emirate", partyPostalAddressHasEmirate),
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
