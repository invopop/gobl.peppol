package pint

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// TINScheme is the ISO 6523 ICD the UAE Federal Tax Authority registered for the TIN, which is also the
// scheme of UAE Peppol endpoints (0235:<TIN>).
const TINScheme cbc.Code = "0235"

// UAE number formats PINT AE reads.
const (
	// TINPattern is the UAE TIN format: 10 digits starting with 1.
	TINPattern = `^1\d{9}$`
	// TRNPattern is the UAE TRN format: 15 digits starting with its TIN.
	TRNPattern = `^1\d{14}$`
	// vatTRNSuffixPattern is the trailing 03 of a UAE TRN for VAT.
	vatTRNSuffixPattern = `03$`
)

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
