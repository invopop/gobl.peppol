package pint

import (
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

const (
	// meansCodeCreditTransfer is the UNTDID 4461 code (30) for a local or non-SEPA international credit transfer.
	meansCodeCreditTransfer cbc.Code = "30"
	// meansCodeSEPACreditTransfer is the UNTDID 4461 code (58) for a SEPA credit transfer.
	meansCodeSEPACreditTransfer cbc.Code = "58"
)

// payInstructionsRules enforces the payment instruction rules of the
// jurisdictions: A-NZ on Australian and New Zealand documents, and AE on UAE
// documents.
func payInstructionsRules() *rules.Set {
	return rules.For(new(pay.Instructions),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAUNZ...)),
			rules.When(is.Func("payment means is credit transfer", instructionsMeanCreditTransfer),
				rules.Field("credit_transfer",
					rules.Assert("01", "invoice credit transfer instructions must include a payment account identifier (ALIGNED-IBRP-018)", is.Present),
				),
			),
		),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.When(is.Func("payment means is credit transfer", instructionsMeanCreditTransferAE),
				rules.Assert("02", "first credit transfer must include an IBAN or account number (IBR-192-AE)",
					is.Func("has a payment account identifier", instructionsHaveAccountID),
				),
			),
		),
	)
}

// creditTransferRules enforces that any credit transfer information provided
// on Australian and New Zealand documents carries a payment account identifier.
// AE has no such rule: IBR-192-AE only applies to credit transfer means (30),
// so it is checked from the payment instructions instead.
func creditTransferRules() *rules.Set {
	return rules.For(new(pay.CreditTransfer),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAUNZ...)),
			rules.Assert("01", "credit transfer information must include a payment account identifier (ALIGNED-IBRP-016)",
				is.Func("has IBAN or account number", creditTransferHasAccountID),
			),
		),
	)
}

// instructionsMeanCreditTransfer reports whether the payment means code
// (ibt-081) of the instructions identifies a local, non-SEPA international or
// SEPA credit transfer.
func instructionsMeanCreditTransfer(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil {
		return false
	}
	code := instr.Ext.Get(untdid.ExtKeyPaymentMeans)
	return code == meansCodeCreditTransfer || code == meansCodeSEPACreditTransfer
}

// instructionsMeanCreditTransferAE reports whether the payment means code
// (ibt-081) of the instructions is 30, a credit transfer. Unlike the A-NZ
// predicate, SEPA (58) is not included, as IBR-192-AE only checks 30.
func instructionsMeanCreditTransferAE(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil {
		return false
	}
	code := instr.Ext.Get(untdid.ExtKeyPaymentMeans)
	return code == meansCodeCreditTransfer
}

// creditTransferHasAccountID reports whether the credit transfer carries an
// account identifier (ibt-084), either an IBAN or an account number.
func creditTransferHasAccountID(val any) bool {
	ct, ok := val.(*pay.CreditTransfer)
	if !ok || ct == nil {
		return true
	}
	return ct.IBAN != "" || ct.Number != ""
}

// instructionsHaveAccountID reports whether the instructions' first credit
// transfer carries an account identifier (ibt-084), either an IBAN or an
// account number. gobl.ubl only writes the first credit transfer as
// cac:PayeeFinancialAccount, so the others can't satisfy IBR-192-AE. A missing
// credit transfer list fails, so one check covers both cases of the rule.
func instructionsHaveAccountID(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil {
		return true
	}
	if len(instr.CreditTransfer) == 0 {
		return false
	}
	ct := instr.CreditTransfer[0]
	return ct != nil && (ct.IBAN != "" || ct.Number != "")
}
