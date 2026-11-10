package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func instructionsWithMeansCode(code string) *pay.Instructions {
	return &pay.Instructions{
		Key: pay.MeansKeyCreditTransfer,
		Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: cbc.Code(code)}),
	}
}

func TestPayInstructionsRules(t *testing.T) {
	validate := func(instr *pay.Instructions) error {
		return rules.Validate(instr, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.AU.Tax()))
	}

	t.Run("credit transfer with account number is accepted", func(t *testing.T) {
		instr := instructionsWithMeansCode("30")
		instr.CreditTransfer = []*pay.CreditTransfer{{Number: "033547 1234567"}}
		assert.NoError(t, validate(instr))
	})

	t.Run("SEPA credit transfer with IBAN is accepted", func(t *testing.T) {
		instr := instructionsWithMeansCode("58")
		instr.CreditTransfer = []*pay.CreditTransfer{{IBAN: "GB33BUKB20201555555555"}}
		assert.NoError(t, validate(instr))
	})

	t.Run("credit transfer without account details is rejected (ALIGNED-IBRP-018)", func(t *testing.T) {
		instr := instructionsWithMeansCode("30")
		err := validate(instr)
		assert.ErrorContains(t, err, "credit transfer instructions must include a payment account identifier")
	})

	t.Run("SEPA credit transfer without account details is rejected (ALIGNED-IBRP-018)", func(t *testing.T) {
		instr := instructionsWithMeansCode("58")
		err := validate(instr)
		assert.ErrorContains(t, err, "credit transfer instructions must include a payment account identifier")
	})

	t.Run("credit transfer entry missing both IBAN and number is rejected (ALIGNED-IBRP-016)", func(t *testing.T) {
		instr := instructionsWithMeansCode("30")
		instr.CreditTransfer = []*pay.CreditTransfer{{BIC: "ANZBAU3M", Name: "ANZ Bank"}}
		err := validate(instr)
		assert.ErrorContains(t, err, "credit transfer information must include a payment account identifier")
	})

	t.Run("credit transfer entry without account is rejected regardless of means code (ALIGNED-IBRP-016)", func(t *testing.T) {
		instr := &pay.Instructions{
			Key:            pay.MeansKeyOnline,
			Ext:            tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "68"}),
			CreditTransfer: []*pay.CreditTransfer{{Name: "ANZ Bank"}},
		}
		err := validate(instr)
		assert.ErrorContains(t, err, "credit transfer information must include a payment account identifier")
	})

	t.Run("other payment means are exempt from the account identifier rule", func(t *testing.T) {
		instr := &pay.Instructions{
			Key: pay.MeansKeyCash,
			Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "10"}),
		}
		assert.NoError(t, validate(instr))
	})

	t.Run("NZ documents are also checked", func(t *testing.T) {
		instr := instructionsWithMeansCode("30")
		err := rules.Validate(instr, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.NZ.Tax()))
		assert.ErrorContains(t, err, "credit transfer instructions must include a payment account identifier")
	})

	t.Run("documents outside AU and NZ are exempt", func(t *testing.T) {
		instr := instructionsWithMeansCode("30")
		instr.CreditTransfer = []*pay.CreditTransfer{{Name: "Bank"}}
		err := rules.Validate(instr, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.SG.Tax()))
		assert.NoError(t, err)
	})
}

func TestPayInstructionsRulesAE(t *testing.T) {
	validate := func(instr *pay.Instructions) error {
		return rules.Validate(instr, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.AE.Tax()))
	}

	t.Run("credit transfer with IBAN is accepted", func(t *testing.T) {
		instr := instructionsWithMeansCode("30")
		instr.CreditTransfer = []*pay.CreditTransfer{{IBAN: "AE070331234567890123456"}}
		assert.NoError(t, validate(instr))
	})

	t.Run("credit transfer with account number is accepted", func(t *testing.T) {
		instr := instructionsWithMeansCode("30")
		instr.CreditTransfer = []*pay.CreditTransfer{{Number: "1234567890"}}
		assert.NoError(t, validate(instr))
	})

	t.Run("credit transfer without credit transfer details is rejected (IBR-192-AE)", func(t *testing.T) {
		err := validate(instructionsWithMeansCode("30"))
		assert.ErrorContains(t, err, "GOBL-PEPPOL-PINT-PAY-INSTRUCTIONS-02")
	})

	t.Run("credit transfer without IBAN or account number is rejected (IBR-192-AE)", func(t *testing.T) {
		instr := instructionsWithMeansCode("30")
		instr.CreditTransfer = []*pay.CreditTransfer{{BIC: "EBILAEAD", Name: "Emirates NBD"}}
		err := validate(instr)
		assert.ErrorContains(t, err, "GOBL-PEPPOL-PINT-PAY-INSTRUCTIONS-02")
	})

	t.Run("other payment means are not checked", func(t *testing.T) {
		for _, code := range []string{"58", "31"} {
			instr := instructionsWithMeansCode(code)
			assert.NoError(t, validate(instr), "means %s", code)
			instr.CreditTransfer = []*pay.CreditTransfer{{BIC: "EBILAEAD"}}
			assert.NoError(t, validate(instr), "means %s with BIC only", code)
		}
	})

	t.Run("the A-NZ account rules do not apply", func(t *testing.T) {
		instr := &pay.Instructions{
			Key:            pay.MeansKeyOnline,
			Ext:            tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "68"}),
			CreditTransfer: []*pay.CreditTransfer{{Name: "Emirates NBD"}},
		}
		assert.NoError(t, validate(instr))
	})

	t.Run("the AE rule does not apply outside AE", func(t *testing.T) {
		instr := instructionsWithMeansCode("30")
		instr.CreditTransfer = []*pay.CreditTransfer{{BIC: "EBILAEAD"}}
		err := rules.Validate(instr, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.SG.Tax()))
		assert.NoError(t, err)
	})
}
