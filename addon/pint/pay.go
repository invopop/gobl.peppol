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

// payInstructionsRules validates payment instructions on A-NZ and UAE documents.
func payInstructionsRules() *rules.Set {
	return rules.For(new(pay.Instructions),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAUNZ...)),
			rules.When(is.Func("payment means is credit transfer", instructionsMeansIn(meansCodeCreditTransfer, meansCodeSEPACreditTransfer)),
				rules.Field("credit_transfer",
					rules.Assert("01", "invoice credit transfer instructions must include a payment account identifier (ALIGNED-IBRP-018)", is.Present),
				),
			),
		),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			// IBR-192-AE checks only 30, not 58. Every credit transfer needs an account, though gobl.ubl writes only the first.
			rules.When(is.Func("payment means is credit transfer", instructionsMeansIn(meansCodeCreditTransfer)),
				rules.Field("credit_transfer",
					rules.Assert("02", "credit transfer is required with the payment means 30 (IBR-192-AE)", is.Present),
					rules.Each(
						rules.Assert("03", "credit transfer must include an IBAN or account number with the payment means 30 (IBR-192-AE)",
							is.Func("has IBAN or account number", creditTransferHasAccountID),
						),
					),
				),
			),
		),
	)
}

// creditTransferRules enforces that any credit transfer information provided
// on Australian and New Zealand documents carries a payment account identifier.
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

// instructionsMeansIn returns a predicate that reports whether the payment
// means code (ibt-081) of the instructions is one of the codes.
func instructionsMeansIn(codes ...cbc.Code) func(any) bool {
	return func(val any) bool {
		instr, ok := val.(*pay.Instructions)
		return ok && instr != nil && instr.Ext.Get(untdid.ExtKeyPaymentMeans).In(codes...)
	}
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
