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
		return rules.Validate(obj, tax.AddonContext(bis.V1))
	}
	glnIdentity := func(code string) *org.Identity {
		return &org.Identity{
			Code: cbc.Code(code),
			Ext:  tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: "0088"}),
		}
	}

	t.Run("valid GLN inbox passes (COMMON-R040)", func(t *testing.T) {
		in := &org.Inbox{Scheme: "0088", Code: "1234567890128"}
		if err := validate(in); err != nil {
			assert.NotContains(t, err.Error(), "GLN")
		}
	})

	t.Run("GLN inbox with wrong check digit is rejected (COMMON-R040)", func(t *testing.T) {
		in := &org.Inbox{Scheme: "0088", Code: "1234567890123"}
		assert.ErrorContains(t, validate(in), "GLN electronic address must have a valid GS1 format")
	})

	t.Run("non-numeric GLN inbox is rejected (COMMON-R040)", func(t *testing.T) {
		in := &org.Inbox{Scheme: "0088", Code: "12345678901A8"}
		assert.ErrorContains(t, validate(in), "GLN electronic address must have a valid GS1 format")
	})

	t.Run("inbox under another scheme is not checked (COMMON-R040)", func(t *testing.T) {
		in := &org.Inbox{Scheme: "0208", Code: "0123456789"}
		if err := validate(in); err != nil {
			assert.NotContains(t, err.Error(), "GLN")
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
}
