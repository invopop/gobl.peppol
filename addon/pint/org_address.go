package pint

import (
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// emiratePattern matches the emirate codes PINT AE accepts, its own list rather than ISO 3166-2.
const emiratePattern = `^(AUH|DXB|SHJ|UAQ|FUJ|AJM|RAK)$`

// orgAddressRules validates addresses on UAE documents.
func orgAddressRules() *rules.Set {
	return rules.For(new(org.Address),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.When(
				is.Func("UAE address", addressIsAE),
				rules.Field("region",
					rules.Assert("01", "region must be an emirate code (AUH, DXB, SHJ, UAQ, FUJ, AJM or RAK) when the country is AE (IBR-128-AE)",
						is.Present, is.Matches(emiratePattern),
					),
				),
			),
		),
	)
}

// addressIsAE reports whether an address is in the UAE.
func addressIsAE(val any) bool {
	a, ok := val.(*org.Address)
	return ok && a != nil && a.Country == l10n.AE.ISO()
}
