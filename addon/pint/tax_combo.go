package pint

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// TaxCategoryMarginScheme is the PINT AE margin scheme VAT category.
const TaxCategoryMarginScheme cbc.Code = "N"

// taxCategoryMap maps GOBL tax keys to the UNTDID 5305 tax category codes that
// PINT recognises, mirroring EN 16931's VAT key mapping. The PINT AE margin
// scheme category is not here, as it is only for UAE documents.
var taxCategoryMap = tax.ExtensionsOf(cbc.CodeMap{
	tax.KeyStandard:       en16931.TaxCategoryStandard,
	tax.KeyZero:           en16931.TaxCategoryZero,
	tax.KeyExempt:         en16931.TaxCategoryExempt,
	tax.KeyReverseCharge:  en16931.TaxCategoryReverseCharge,
	tax.KeyIntraCommunity: en16931.TaxCategoryIntraCommunity,
	tax.KeyExport:         en16931.TaxCategoryExport,
	tax.KeyOutsideScope:   en16931.TaxCategoryOutsideScope,
})

// exemptionCodesAE are PINT AE's reasons for exemption from tax (code list Aligned-TaxExemptionCodes): the
// UAE VAT exemptions of Article 46 of Federal Decree-Law No. 8 of 2017.
var exemptionCodesAE = []cbc.Code{
	"DL8.46.1", // certain financial services
	"DL8.46.2", // residential buildings
	"DL8.46.3", // bare land
	"DL8.46.4", // local passenger transport
}

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

// normalizeMarginSchemeAE gives N to the exempt VAT lines without a VATEX code of a UAE margin scheme
// invoice. Discounts, charges and lines with a reason keep E, so the rules reject them.
func normalizeMarginSchemeAE(inv *bill.Invoice) {
	if inv == nil || !inv.HasTags(TagMarginScheme) {
		return
	}
	for _, l := range inv.Lines {
		if l == nil {
			continue
		}
		for _, c := range l.Taxes {
			if c != nil && c.Category == tax.CategoryVAT && c.Key == tax.KeyExempt && !c.Ext.Has(cef.ExtKeyVATEX) {
				c.Ext = c.Ext.Set(untdid.ExtKeyTaxCategory, TaxCategoryMarginScheme)
			}
		}
	}
}

// normalizeMarginSchemeNoteAE ties the margin scheme note to N, and keys an imported one so the scenario
// doesn't add a second. It normalizes notes as the scenario adds the note after the invoice is normalized.
func normalizeMarginSchemeNoteAE(n *tax.Note) {
	if n == nil || n.Category != tax.CategoryVAT {
		return
	}
	if n.Key.IsEmpty() && n.Ext.Get(untdid.ExtKeyTaxCategory) == TaxCategoryMarginScheme {
		n.Key = TagMarginScheme
	}
	if n.Key == TagMarginScheme {
		n.Ext = n.Ext.Set(untdid.ExtKeyTaxCategory, TaxCategoryMarginScheme)
	}
}

// taxComboRules validates tax combos on PINT documents.
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

		// TODO: A-NZ has the same zero rate rule (ALIGNED-IBRP-Z-05/06/07) and a positive standard rate (ALIGNED-IBRP-S-05/06/07).
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			// EN 16931's and PINT's category lists have no N; IBR-139-AE checks the PINT AE list instead.
			rules.Ignore("GOBL-EU-EN16931-TAX-COMBO-02", "GOBL-PEPPOL-PINT-TAX-COMBO-01"),
			rules.Field("ext",
				rules.Assert("03", "tax category code must be an AE UNCL5305 code: E, S, Z, O, AE or N (IBR-139-AE)",
					tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesAE...),
				),
			),

			rules.When(
				is.Func("tax combo is standard rated", taxComboIsStandard),
				rules.Field("percent",
					rules.Assert("04", "tax percentage must be 5.00 (IBR-190-AE, ALIGNED-IBRP-S-05/06/07)",
						num.Equals(num.MakePercentage(5, 2)),
					),
				),
			),

			rules.When(
				is.Func("tax combo is zero rated", taxComboIsZero),
				rules.Field("percent",
					rules.Assert("05", "tax percentage must be 0.00 (ALIGNED-IBRP-Z-05/06/07, IBR-120-AE)",
						num.Equals(num.MakePercentage(0, 2)),
					),
				),
			),

			rules.Field("ext",
				rules.When(
					is.AllOf(
						tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
						tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, en16931.TaxCategoryExempt),
					),
					rules.Assert("06", "exempt VAT needs a cef-vatex exemption reason: DL8.46.1, DL8.46.2, DL8.46.3 or DL8.46.4 (IBR-167-AE, IBR-168-AE, IBR-169-AE)",
						tax.ExtensionsRequire(cef.ExtKeyVATEX),
					),
				),
			),

			// The Schematron doesn't check PINT AE's code list.
			rules.Field("ext",
				rules.Assert("07", "VAT exemption reason must be one of PINT AE's: DL8.46.1, DL8.46.2, DL8.46.3 or DL8.46.4 (code list Aligned-TaxExemptionCodes)",
					tax.ExtensionsHasCodes(cef.ExtKeyVATEX, exemptionCodesAE...),
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

// taxCategoryCodesOutsideScope are the VAT category codes allowed on outside scope documents.
var taxCategoryCodesOutsideScope = []cbc.Code{
	en16931.TaxCategoryExempt,
	en16931.TaxCategoryOutsideScope,
}
