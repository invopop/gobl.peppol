package aunz

import (
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
)

const (
	// meansCodeCreditTransfer is the UNTDID 4461 code (30) for a local or non-SEPA international credit transfer.
	meansCodeCreditTransfer cbc.Code = "30"
	// meansCodeSEPACreditTransfer is the UNTDID 4461 code (58) for a SEPA credit transfer.
	meansCodeSEPACreditTransfer cbc.Code = "58"
)

// payInstructionsRules enforces the A-NZ payment instruction rules.
func payInstructionsRules() *rules.Set {
	return rules.For(new(pay.Instructions),
		rules.When(is.Func("payment means is credit transfer", instructionsMeanCreditTransfer),
			rules.Field("credit_transfer",
				rules.Assert("01", "invoice credit transfer instructions must include a payment account identifier (ALIGNED-IBRP-018)", is.Present),
			),
		),
	)
}

// creditTransferRules enforces that any credit transfer information provided
// carries a payment account identifier.
func creditTransferRules() *rules.Set {
	return rules.For(new(pay.CreditTransfer),
		rules.Assert("01", "credit transfer information must include a payment account identifier (ALIGNED-IBRP-016)",
			is.Func("has IBAN or account number", creditTransferHasAccountID),
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

// creditTransferHasAccountID reports whether the credit transfer carries an
// account identifier (ibt-084), either an IBAN or an account number.
func creditTransferHasAccountID(val any) bool {
	ct, ok := val.(*pay.CreditTransfer)
	if !ok || ct == nil {
		return true
	}
	return ct.IBAN != "" || ct.Number != ""
}
