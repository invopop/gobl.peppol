package bis_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/bis"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func TestOrgGLN(t *testing.T) {
	validate := func(obj any) error {
		return rules.Validate(obj, tax.AddonContext(bis.V3))
	}
	glnEndpoint := func(code string) *org.Endpoint {
		return &org.Endpoint{URI: cbc.URI("iso6523-actorid-upis::0088:" + code)}
	}
	glnIdentity := func(code string) *org.Identity {
		return &org.Identity{
			Code: cbc.Code(code),
			Ext:  tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: "0088"}),
		}
	}

	t.Run("valid GLN endpoint passes (COMMON-R040)", func(t *testing.T) {
		if err := validate(glnEndpoint("1234567890128")); err != nil {
			assert.NotContains(t, err.Error(), "GLN")
		}
	})

	t.Run("GLN endpoint with wrong check digit is rejected (COMMON-R040)", func(t *testing.T) {
		assert.ErrorContains(t, validate(glnEndpoint("1234567890123")), "GLN electronic address must have a valid GS1 format")
	})

	t.Run("non-numeric GLN endpoint is rejected (COMMON-R040)", func(t *testing.T) {
		assert.ErrorContains(t, validate(glnEndpoint("12345678901A8")), "GLN electronic address must have a valid GS1 format")
	})

	t.Run("GLN endpoint that is not 13 digits is rejected (COMMON-R040)", func(t *testing.T) {
		assert.ErrorContains(t, validate(glnEndpoint("00")), "GLN electronic address must have a valid GS1 format")
	})

	t.Run("endpoint under another scheme is not checked (COMMON-R040)", func(t *testing.T) {
		for _, uri := range []cbc.URI{"iso6523-actorid-upis::0208:0123456789", "mailto:billing@example.com"} {
			if err := validate(&org.Endpoint{URI: uri}); err != nil {
				assert.NotContains(t, err.Error(), "GLN")
			}
		}
	})

	t.Run("valid GLN identity passes (COMMON-R040)", func(t *testing.T) {
		if err := validate(glnIdentity("4000001000005")); err != nil {
			assert.NotContains(t, err.Error(), "GLN")
		}
	})

	t.Run("GLN identity with wrong check digit is rejected (COMMON-R040)", func(t *testing.T) {
		assert.ErrorContains(t, validate(glnIdentity("4000001000006")), "GLN identifier must have a valid GS1 format")
	})

	t.Run("GLN identity that is not 13 digits is rejected (COMMON-R040)", func(t *testing.T) {
		assert.ErrorContains(t, validate(glnIdentity("00")), "GLN identifier must have a valid GS1 format")
	})
}
