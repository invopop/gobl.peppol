package bis

import (
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
)

func payInstructionsRules() *rules.Set {
	return rules.For(new(pay.Instructions),
		rules.Assert("01", "direct debit mandate reference is required (PEPPOL-EN16931-R061)",
			is.Func("direct debit has mandate reference", instructionsDirectDebitHasMandate),
		),
	)
}

// instructionsDirectDebitHasMandate reports whether direct-debit payment
// instructions carry a mandate reference. Non-direct-debit instructions are not
// subject to the rule.
func instructionsDirectDebitHasMandate(val any) bool {
	inst, ok := val.(*pay.Instructions)
	if !ok || inst == nil {
		return true
	}
	if !inst.Key.HasPrefix(pay.MeansKeyDirectDebit) {
		return true
	}
	return inst.DirectDebit != nil && inst.DirectDebit.Ref != ""
}
