package pint

import (
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// taxIdentityRules requires the trailing 03 of a VAT registration on every
// UAE tax identity of a UAE document (IBR-132-AE). The rest of IBR-132-AE's
// format, 15 digits starting with 1, is checked by the AE regime, together
// with the check digit of the TIN they start with.
// vatTRNSuffixPattern is the trailing 03 of a UAE TRN for VAT (IBR-132-AE).
const vatTRNSuffixPattern = `03$`

func taxIdentityRules() *rules.Set {
	return rules.For(new(tax.Identity),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.When(tax.IdentityIn("AE"),
				rules.Field("code",
					rules.AssertIfPresent("01", "UAE TRN must end with 03 (IBR-132-AE)",
						is.Matches(vatTRNSuffixPattern)),
				),
			),
		),
	)
}
