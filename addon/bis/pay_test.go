package bis_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/bis"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func TestPayInstructionsRules(t *testing.T) {
	validate := func(i *pay.Instructions) error {
		return rules.Validate(i, tax.AddonContext(bis.V3))
	}

	t.Run("direct debit without direct debit details is rejected (R061)", func(t *testing.T) {
		i := &pay.Instructions{Key: pay.MeansKeyDirectDebit}
		assert.ErrorContains(t, validate(i), "direct debit details are required")
	})

	t.Run("direct debit with mandate reference passes", func(t *testing.T) {
		i := &pay.Instructions{Key: pay.MeansKeyDirectDebit, DirectDebit: &pay.DirectDebit{Ref: "MANDATE-1"}}
		if err := validate(i); err != nil {
			assert.NotContains(t, err.Error(), "mandate reference")
		}
	})

	t.Run("SEPA direct debit without direct debit details is rejected (R061)", func(t *testing.T) {
		i := &pay.Instructions{Key: pay.MeansKeyDirectDebit.With(pay.MeansKeySEPA)}
		assert.ErrorContains(t, rules.Validate(i, tax.AddonContext(bis.V3)), "direct debit details are required")
	})

	t.Run("direct debit with empty mandate reference is rejected (R061)", func(t *testing.T) {
		i := &pay.Instructions{Key: pay.MeansKeyDirectDebit, DirectDebit: &pay.DirectDebit{Creditor: "CREDITOR-1"}}
		assert.ErrorContains(t, validate(i), "mandate reference is required")
	})

	t.Run("credit transfer is not subject to the rule", func(t *testing.T) {
		i := &pay.Instructions{Key: pay.MeansKeyCreditTransfer}
		if err := validate(i); err != nil {
			assert.NotContains(t, err.Error(), "mandate reference")
		}
	})
}
