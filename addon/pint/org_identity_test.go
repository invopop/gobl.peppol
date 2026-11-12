package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/norm"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func TestIdentityTypeAE(t *testing.T) {
	tests := map[cbc.Key]cbc.Code{
		ae.IdentityKeyTradeLicense:    pint.IdentityTypeTradeLicense,
		ae.IdentityKeyEmiratesID:      pint.IdentityTypeEmiratesID,
		org.IdentityKeyPassport:       pint.IdentityTypePassport,
		ae.IdentityKeyCabinetDecision: pint.IdentityTypeCabinetDecision,
		ae.IdentityKeyTIN:             "",
		cbc.KeyEmpty:                  "",
	}
	for key, want := range tests {
		assert.Equal(t, want, pint.IdentityTypeAE(key), key)
	}
}

func TestIdentityKeyAE(t *testing.T) {
	tests := map[cbc.Code]cbc.Key{
		"TL":  ae.IdentityKeyTradeLicense,
		"CL":  ae.IdentityKeyTradeLicense,
		"EID": ae.IdentityKeyEmiratesID,
		"PAS": org.IdentityKeyPassport,
		"CD":  ae.IdentityKeyCabinetDecision,
		"XYZ": "",
		"":    "",
	}
	for typ, want := range tests {
		assert.Equal(t, want, pint.IdentityKeyAE(typ), typ)
	}
}

func TestNormalizePartyIdentityScopes(t *testing.T) {
	normalize := func(regime l10n.TaxCountryCode, ids ...*org.Identity) []cbc.Key {
		p := &org.Party{Name: "Party", Identities: ids}
		norm.Normalize(p, tax.AddonContext(en16931.V2017, pint.V1), tax.RegimeContext(regime))
		scopes := make([]cbc.Key, len(p.Identities))
		for i, id := range p.Identities {
			scopes[i] = id.Scope
		}
		return scopes
	}
	tin := func() *org.Identity { return &org.Identity{Key: ae.IdentityKeyTIN, Code: "1234567890"} }
	eid := func() *org.Identity { return &org.Identity{Key: ae.IdentityKeyEmiratesID, Code: "784199012345671"} }
	passport := func() *org.Identity {
		return &org.Identity{Key: org.IdentityKeyPassport, Code: "P1234567890", Country: "IN"}
	}

	t.Run("TIN gets the tax scope and legal registrations the legal scope", func(t *testing.T) {
		assert.Equal(t, []cbc.Key{org.IdentityScopeTax, org.IdentityScopeLegal},
			normalize(l10n.AE.Tax(), tin(), passport()))
		for _, key := range []cbc.Key{ae.IdentityKeyTradeLicense, ae.IdentityKeyEmiratesID, ae.IdentityKeyCabinetDecision} {
			assert.Equal(t, []cbc.Key{org.IdentityScopeLegal},
				normalize(l10n.AE.Tax(), &org.Identity{Key: key, Code: "784199012345671"}), key)
		}
	})

	t.Run("only the first identity of each scope gets it", func(t *testing.T) {
		assert.Equal(t, []cbc.Key{org.IdentityScopeLegal, ""},
			normalize(l10n.AE.Tax(), eid(), passport()))
		assert.Equal(t, []cbc.Key{org.IdentityScopeTax, ""},
			normalize(l10n.AE.Tax(), tin(), tin()))
	})

	t.Run("a scope another identity already has is not given again", func(t *testing.T) {
		legal := &org.Identity{Scope: org.IdentityScopeLegal, Code: "CN-1234567"}
		assert.Equal(t, []cbc.Key{"", org.IdentityScopeLegal},
			normalize(l10n.AE.Tax(), passport(), legal))
	})

	t.Run("an explicit scope is kept", func(t *testing.T) {
		id := eid()
		id.Scope = org.IdentityScopeTax
		assert.Equal(t, []cbc.Key{org.IdentityScopeTax}, normalize(l10n.AE.Tax(), id))
	})

	t.Run("other identities get no scope", func(t *testing.T) {
		assert.Equal(t, []cbc.Key{""},
			normalize(l10n.AE.Tax(), &org.Identity{Type: "XYZ", Code: "123"}))
	})

	t.Run("only under the AE regime", func(t *testing.T) {
		assert.Equal(t, []cbc.Key{"", ""}, normalize(l10n.AU.Tax(), tin(), passport()))
	})
}
