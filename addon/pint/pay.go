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
	meansCodeCreditTransfer     cbc.Code = "30"
	meansCodeSEPACreditTransfer cbc.Code = "58"
)

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
			// IBR-192-AE checks only 30, not 58.
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

func instructionsMeansIn(codes ...cbc.Code) func(any) bool {
	return func(val any) bool {
		instr, ok := val.(*pay.Instructions)
		return ok && instr != nil && instr.Ext.Get(untdid.ExtKeyPaymentMeans).In(codes...)
	}
}

func creditTransferHasAccountID(val any) bool {
	ct, ok := val.(*pay.CreditTransfer)
	if !ok || ct == nil {
		return true
	}
	return ct.IBAN != "" || ct.Number != ""
}
