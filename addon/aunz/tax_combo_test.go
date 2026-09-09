package aunz_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/aunz"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func gstCombo(category string) *tax.Combo {
	return &tax.Combo{
		Category: tax.CategoryGST,
		Ext:      tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: cbc.Code(category)}),
	}
}

func TestTaxComboRules(t *testing.T) {
	validate := func(tc *tax.Combo) error {
		return rules.Validate(tc, tax.AddonContext(aunz.V1))
	}

	t.Run("A-NZ subset categories are accepted", func(t *testing.T) {
		for _, cat := range []string{"E", "S", "Z", "G", "O"} {
			err := validate(gstCombo(cat))
			assert.NoError(t, err, "category %s", cat)
		}
	})

	t.Run("categories outside the A-NZ subset are rejected (ALIGNED-IBRP-CL-01)", func(t *testing.T) {
		for _, cat := range []string{"AE", "K", "L", "M"} {
			err := validate(gstCombo(cat))
			assert.ErrorContains(t, err, "tax category code must be an A-NZ UNCL5305 code", "category %s", cat)
		}
	})
}
