package bis

import (
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
)

func payInstructionsRules() *rules.Set {
	return rules.For(new(pay.Instructions),
		rules.When(is.Func("direct debit", instructionsAreDirectDebit),
			rules.Field("direct_debit",
				rules.Assert("01", "direct debit details are required (PEPPOL-EN16931-R061)", is.Present),
				rules.Field("ref",
					rules.Assert("02", "direct debit mandate reference is required (PEPPOL-EN16931-R061)", is.Present),
				),
			),
		),
	)
}

// instructionsAreDirectDebit reports whether the payment instructions use a
// direct-debit means of payment.
func instructionsAreDirectDebit(val any) bool {
	inst, ok := val.(*pay.Instructions)
	return ok && inst != nil && inst.Key.HasPrefix(pay.MeansKeyDirectDebit)
}
