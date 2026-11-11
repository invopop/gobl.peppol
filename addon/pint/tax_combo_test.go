package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/norm"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

// Both en16931 (required by PINT) and the PINT addon are applied, mirroring how
// they resolve on a real document, so we can confirm PINT's normalizer runs
// after en16931's and wins.

func TestTaxComboNormalization(t *testing.T) {
	t.Run("standard GST rate maps to S (not O)", func(t *testing.T) {
		p := num.MakePercentage(10, 2)
		c := &tax.Combo{
			Category: tax.CategoryGST,
			Key:      tax.KeyStandard,
			Percent:  &p,
		}
		norm.Normalize(c, tax.AddonContext(en16931.V2017, pint.V1))
		assert.Equal(t, "S", c.Ext.Get(untdid.ExtKeyTaxCategory).String())
	})

	t.Run("zero-rated GST maps to Z", func(t *testing.T) {
		p := num.MakePercentage(0, 2)
		c := &tax.Combo{
			Category: tax.CategoryGST,
			Key:      tax.KeyZero,
			Percent:  &p,
		}
		norm.Normalize(c, tax.AddonContext(en16931.V2017, pint.V1))
		assert.Equal(t, "Z", c.Ext.Get(untdid.ExtKeyTaxCategory).String())
	})

	t.Run("exempt GST maps to E", func(t *testing.T) {
		c := &tax.Combo{
			Category: tax.CategoryGST,
			Key:      tax.KeyExempt,
		}
		norm.Normalize(c, tax.AddonContext(en16931.V2017, pint.V1))
		assert.Equal(t, "E", c.Ext.Get(untdid.ExtKeyTaxCategory).String())
	})

	t.Run("standard VAT still maps to S", func(t *testing.T) {
		p := num.MakePercentage(20, 2)
		c := &tax.Combo{
			Category: tax.CategoryVAT,
			Key:      tax.KeyStandard,
			Percent:  &p,
		}
		norm.Normalize(c, tax.AddonContext(en16931.V2017, pint.V1))
		assert.Equal(t, "S", c.Ext.Get(untdid.ExtKeyTaxCategory).String())
	})
}

func gstCombo(category string) *tax.Combo {
	return &tax.Combo{
		Category: tax.CategoryGST,
		Ext:      tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: cbc.Code(category)}),
	}
}

func TestTaxComboRules(t *testing.T) {
	validate := func(tc *tax.Combo) error {
		return rules.Validate(tc, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.AU.Tax()))
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

	t.Run("documents outside AU and NZ accept the full PINT set", func(t *testing.T) {
		err := rules.Validate(gstCombo("K"), tax.AddonContext(pint.V1), tax.RegimeContext(l10n.SG.Tax()))
		assert.NoError(t, err)
	})
}

func vatCombo(key cbc.Key, category string, percent *num.Percentage) *tax.Combo {
	return &tax.Combo{
		Category: tax.CategoryVAT,
		Key:      key,
		Percent:  percent,
		Ext:      tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: cbc.Code(category)}),
	}
}

func validateIn(tc *tax.Combo, regime l10n.TaxCountryCode) error {
	return rules.Validate(tc, tax.AddonContext(pint.V1), tax.RegimeContext(regime))
}

func TestTaxComboRulesAE(t *testing.T) {
	ae := l10n.AE.Tax()

	t.Run("AE subset categories are accepted (IBR-139-AE)", func(t *testing.T) {
		for cat, key := range map[string]cbc.Key{
			"E":  tax.KeyExempt,
			"O":  tax.KeyOutsideScope,
			"AE": tax.KeyReverseCharge,
		} {
			err := validateIn(vatCombo(key, cat, nil), ae)
			assert.NoError(t, err, "category %s", cat)
		}
		assert.NoError(t, validateIn(vatCombo(tax.KeyStandard, "S", num.NewPercentage(5, 2)), ae))
		assert.NoError(t, validateIn(vatCombo(tax.KeyZero, "Z", num.NewPercentage(0, 2)), ae))
	})

	t.Run("categories outside the AE subset are rejected (IBR-139-AE)", func(t *testing.T) {
		for _, cat := range []string{"G", "K", "L", "M"} {
			err := validateIn(vatCombo(cbc.KeyEmpty, cat, nil), ae)
			assert.ErrorContains(t, err, "GOBL-PEPPOL-PINT-TAX-COMBO-03", "category %s", cat)
		}
	})

	t.Run("AE subset does not apply outside AE", func(t *testing.T) {
		assert.NoError(t, validateIn(vatCombo(cbc.KeyEmpty, "K", nil), l10n.SG.Tax()))
	})

	t.Run("standard rate must be greater than zero (ALIGNED-IBRP-S-05/06/07)", func(t *testing.T) {
		err := validateIn(vatCombo(tax.KeyStandard, "S", num.NewPercentage(0, 2)), ae)
		assert.ErrorContains(t, err, "GOBL-PEPPOL-PINT-TAX-COMBO-04")
		// The Schematron reports both S-05 and IBR-190 for a 0% standard rate.
		assert.ErrorContains(t, err, "GOBL-PEPPOL-PINT-TAX-COMBO-05")
	})

	t.Run("standard rate must be 5% (IBR-190-AE)", func(t *testing.T) {
		err := validateIn(vatCombo(tax.KeyStandard, "S", num.NewPercentage(10, 2)), ae)
		assert.ErrorContains(t, err, "GOBL-PEPPOL-PINT-TAX-COMBO-05")
		assert.NotContains(t, err.Error(), "GOBL-PEPPOL-PINT-TAX-COMBO-04")
	})

	t.Run("5% check compares numbers, not strings", func(t *testing.T) {
		assert.NoError(t, validateIn(vatCombo(tax.KeyStandard, "S", num.NewPercentage(50, 3)), ae))
	})

	t.Run("zero rate must be 0% (ALIGNED-IBRP-Z-05/06/07, IBR-120-AE)", func(t *testing.T) {
		err := validateIn(vatCombo(tax.KeyZero, "Z", num.NewPercentage(5, 2)), ae)
		assert.ErrorContains(t, err, "GOBL-PEPPOL-PINT-TAX-COMBO-06")
	})

	t.Run("rate rules do not apply outside AE", func(t *testing.T) {
		for _, regime := range []l10n.TaxCountryCode{l10n.SG.Tax(), l10n.AU.Tax()} {
			assert.NoError(t, validateIn(vatCombo(tax.KeyStandard, "S", num.NewPercentage(0, 2)), regime), regime)
			assert.NoError(t, validateIn(vatCombo(tax.KeyStandard, "S", num.NewPercentage(10, 2)), regime), regime)
			assert.NoError(t, validateIn(vatCombo(tax.KeyZero, "Z", num.NewPercentage(5, 2)), regime), regime)
		}
	})
}
