package pint

import (
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// vatTRNSuffixPattern is the trailing 03 of a UAE TRN for VAT.
const vatTRNSuffixPattern = `03$`

// taxIdentityRules validates tax identities on UAE documents.
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
