package aunz

import (
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
)

const (
	// taxCategoryStandard is the UNCL5305 code for standard rated taxes.
	taxCategoryStandard cbc.Code = "S"
	// taxCategoryZero is the UNCL5305 code for zero rated taxes.
	taxCategoryZero cbc.Code = "Z"
	// taxCategoryExempt is the UNCL5305 code for taxes exempt from tax.
	taxCategoryExempt cbc.Code = "E"
	// taxCategoryExport is the UNCL5305 code for exports.
	taxCategoryExport cbc.Code = "G"
	// taxCategoryOutsideScope is the UNCL5305 code for taxes outside the scope of tax.
	taxCategoryOutsideScope cbc.Code = "O"
)

// taxCategoryCodes lists the subset of UNCL5305 tax category codes accepted by
// the A-NZ PINT specification.
var taxCategoryCodes = []cbc.Code{
	taxCategoryExempt,
	taxCategoryStandard,
	taxCategoryZero,
	taxCategoryExport,
	taxCategoryOutsideScope,
}

// taxComboRules enforces the A-NZ tax category rules.
func taxComboRules() *rules.Set {
	return rules.For(new(tax.Combo),
		rules.Field("ext",
			rules.Assert("01", "tax category code must be an A-NZ UNCL5305 code: E, S, Z, G or O (ALIGNED-IBRP-CL-01-AUNZ)",
				tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodes...),
			),
		),
	)
}
