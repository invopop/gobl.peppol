package bis

import (
	"regexp"
	"strings"

	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
)

// glnSchemeID is the ISO 6523 scheme identifier for GS1 Global Location Numbers.
const glnSchemeID = "0088"

var (
	// glnRegexp matches the 13 digits of a GS1 Global Location Number.
	glnRegexp = regexp.MustCompile(`^\d{13}$`)
	// glnEndpointRegexp matches an ISO 6523 endpoint URI under the GLN scheme.
	glnEndpointRegexp = regexp.MustCompile(`^` + iso.ActorIDScheme + `::` + glnSchemeID + `:`)
)

func orgEndpointRules() *rules.Set {
	return rules.For(new(org.Endpoint),
		rules.Field("uri",
			rules.When(is.MatchesRegexp(glnEndpointRegexp),
				rules.Assert("01", "GLN electronic address must have a valid GS1 format (PEPPOL-COMMON-R040)",
					is.Func("GS1 check digit", glnCheckDigitValid),
				),
			),
		),
	)
}

func orgIdentityRules() *rules.Set {
	return rules.For(new(org.Identity),
		rules.When(is.Func("GLN identity", identityIsGLN),
			rules.Field("code",
				rules.AssertIfPresent("01", "GLN identifier must have a valid GS1 format (PEPPOL-COMMON-R040)",
					is.MatchesRegexp(glnRegexp),
					is.Func("GS1 check digit", glnCheckDigitValid),
				),
			),
		),
	)
}

// identityIsGLN reports whether the identity is issued under the GLN scheme.
func identityIsGLN(val any) bool {
	id, ok := val.(*org.Identity)
	return ok && id != nil && id.Ext.Get(iso.ExtKeySchemeID) == glnSchemeID
}

// glnCheckDigitValid reports whether the last digit of a 13-digit GLN
//
//	matches the GS1 modulo-10 check digit computed from the first twelve.
func glnCheckDigitValid(val any) bool {
	var code string
	switch v := val.(type) {
	case cbc.URI:
		code = strings.TrimPrefix(v.String(), glnEndpointRegexp.FindString(v.String()))
	case cbc.Code:
		code = v.String()
	default:
		return false
	}
	if !glnRegexp.MatchString(code) {
		return false
	}
	sum := 0
	for i := 0; i < 12; i++ {
		d := int(code[i] - '0')
		if i%2 == 1 {
			d *= 3
		}
		sum += d
	}
	return (10-sum%10)%10 == int(code[12]-'0')
}
