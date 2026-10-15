package bis_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/bis"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func comboWith(vatex, category string) *tax.Combo {
	return &tax.Combo{
		Category: tax.CategoryVAT,
		Ext: tax.ExtensionsOf(cbc.CodeMap{
			cef.ExtKeyVATEX:          cbc.Code(vatex),
			untdid.ExtKeyTaxCategory: cbc.Code(category),
		}),
	}
}

func TestTaxComboVATEX(t *testing.T) {
	validate := func(c *tax.Combo) error {
		return rules.Validate(c, tax.AddonContext(bis.V1))
	}

	t.Run("matching VATEX and category pass (P0104..P0111)", func(t *testing.T) {
		for _, tc := range []struct{ vatex, cat string }{
			{"VATEX-EU-G", "G"}, {"VATEX-EU-O", "O"}, {"VATEX-EU-IC", "K"},
			{"VATEX-EU-AE", "AE"}, {"VATEX-EU-D", "E"}, {"VATEX-EU-J", "E"},
		} {
			if err := validate(comboWith(tc.vatex, tc.cat)); err != nil {
				assert.NotContains(t, err.Error(), "VATEX")
			}
		}
	})

	t.Run("mismatched VATEX and category rejected", func(t *testing.T) {
		assert.ErrorContains(t, validate(comboWith("VATEX-EU-AE", "S")), "VATEX")
		assert.ErrorContains(t, validate(comboWith("VATEX-EU-IC", "E")), "VATEX")
	})

	t.Run("unpinned VATEX code is not constrained", func(t *testing.T) {
		if err := validate(comboWith("VATEX-EU-79-C", "E")); err != nil {
			assert.NotContains(t, err.Error(), "VATEX")
		}
	})
}
