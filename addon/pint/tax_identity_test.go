package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func TestTaxIdentityRulesAE(t *testing.T) {
	const faultTRN = "GOBL-PEPPOL-PINT-TAX-IDENTITY-01"
	validateIn := func(id *tax.Identity, regime l10n.TaxCountryCode) error {
		return rules.Validate(id, tax.AddonContext(pint.V1), tax.RegimeContext(regime))
	}
	validate := func(id *tax.Identity) error { return validateIn(id, l10n.AE.Tax()) }

	t.Run("valid TRN is accepted (IBR-132-AE)", func(t *testing.T) {
		assert.NoError(t, validate(&tax.Identity{Country: "AE", Code: "100000001600003"}))
	})

	t.Run("malformed TRNs are rejected (IBR-132-AE)", func(t *testing.T) {
		for name, code := range map[string]cbc.Code{
			"does not start with 1": "200000000000003",
			"does not end with 03":  "100000001600004",
			"too short":             "10000000000003",
			"too long":              "1000000000000003",
		} {
			err := validate(&tax.Identity{Country: "AE", Code: code})
			assert.ErrorContains(t, err, faultTRN, name)
		}
	})

	t.Run("identity without a code is accepted (IBR-132-AE)", func(t *testing.T) {
		assert.NoError(t, validate(&tax.Identity{Country: "AE"}))
	})

	t.Run("non-UAE identities are not checked (IBR-132-AE)", func(t *testing.T) {
		err := validate(&tax.Identity{Country: "DE", Code: "111111125"})
		if err != nil {
			assert.NotContains(t, err.Error(), faultTRN)
		}
	})

	t.Run("the rule does not apply outside AE", func(t *testing.T) {
		err := validateIn(&tax.Identity{Country: "AE", Code: "200000000000003"}, l10n.AU.Tax())
		if err != nil {
			assert.NotContains(t, err.Error(), faultTRN)
		}
	})
}
