package pint

import (
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// taxIdentityRules enforces the UAE TRN format on every UAE tax identity of a
// UAE document (IBR-132-AE). The AE regime only checks for 15 digits; PINT AE
// also requires a leading 1 and a trailing 03.
func taxIdentityRules() *rules.Set {
	return rules.For(new(tax.Identity),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.When(tax.IdentityIn("AE"),
				rules.Field("code",
					rules.AssertIfPresent("01", "UAE TRN must be 15 digits, start with 1 and end with 03 (IBR-132-AE)",
						is.Matches(`^1\d{12}03$`)),
				),
			),
		),
	)
}
