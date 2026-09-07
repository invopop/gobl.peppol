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

func gstCategoryTotal(categories ...string) *tax.CategoryTotal {
	ct := &tax.CategoryTotal{Code: tax.CategoryGST}
	for _, cat := range categories {
		ct.Rates = append(ct.Rates, &tax.RateTotal{
			Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: cbc.Code(cat)}),
		})
	}
	return ct
}

func TestTaxCategoryTotalRules(t *testing.T) {
	validate := func(ct *tax.CategoryTotal) error {
		return rules.Validate(ct, tax.AddonContext(aunz.V1))
	}

	t.Run("GST breakdown with only outside scope is accepted", func(t *testing.T) {
		assert.NoError(t, validate(gstCategoryTotal("O")))
	})

	t.Run("GST breakdown without outside scope is accepted", func(t *testing.T) {
		assert.NoError(t, validate(gstCategoryTotal("S", "Z")))
	})

	t.Run("GST breakdown mixing outside scope with other categories is rejected (ALIGNED-IBRP-O-11)", func(t *testing.T) {
		err := validate(gstCategoryTotal("O", "S"))
		assert.ErrorContains(t, err, "not subject to tax cannot be combined")
	})

	t.Run("non-GST categories are exempt from the rule", func(t *testing.T) {
		ct := gstCategoryTotal("O", "S")
		ct.Code = "VAT"
		assert.NoError(t, validate(ct))
	})
}
