package bis

import (
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
)

// glnSchemeID is the ISO 6523 scheme identifier for GS1 Global Location Numbers.
const glnSchemeID cbc.Code = "0088"

func orgInboxRules() *rules.Set {
	return rules.For(new(org.Inbox),
		rules.Assert("01", "GLN electronic address must have a valid GS1 format (PEPPOL-COMMON-R040)",
			is.Func("valid GLN inbox", inboxHasValidGLN),
		),
	)
}

func orgIdentityRules() *rules.Set {
	return rules.For(new(org.Identity),
		rules.Assert("01", "GLN identifier must have a valid GS1 format (PEPPOL-COMMON-R040)",
			is.Func("valid GLN identity", identityHasValidGLN),
		),
	)
}

// inboxHasValidGLN reports whether an inbox using the GLN scheme carries a code
// with a valid GS1 check digit. Inboxes under other schemes are not constrained.
func inboxHasValidGLN(val any) bool {
	in, ok := val.(*org.Inbox)
	if !ok || in == nil || in.Scheme != glnSchemeID || in.Code == "" {
		return true
	}
	return isValidGLN(in.Code.String())
}

// identityHasValidGLN reports whether an identity using the GLN scheme carries a
// code with a valid GS1 check digit. Identities under other schemes are not
// constrained.
func identityHasValidGLN(val any) bool {
	id, ok := val.(*org.Identity)
	if !ok || id == nil || id.Ext.Get(iso.ExtKeySchemeID) != glnSchemeID || id.Code == "" {
		return true
	}
	return isValidGLN(id.Code.String())
}

// isValidGLN reports whether the code is numeric and its last digit matches the
// GS1 modulo-10 check digit, mirroring the Peppol u:gln schematron function.
func isValidGLN(code string) bool {
	if len(code) < 2 {
		return false
	}
	sum := 0
	for i := len(code) - 2; i >= 0; i-- {
		c := code[i]
		if c < '0' || c > '9' {
			return false
		}
		d := int(c - '0')
		if (len(code)-2-i)%2 == 0 {
			d *= 3
		}
		sum += d
	}
	last := code[len(code)-1]
	if last < '0' || last > '9' {
		return false
	}
	return (10-sum%10)%10 == int(last-'0')
}
