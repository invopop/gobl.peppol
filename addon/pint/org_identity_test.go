package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
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

func TestNormalizeIdentityScopes(t *testing.T) {
	scopes := func(p *org.Party) []cbc.Key {
		out := make([]cbc.Key, len(p.Identities))
		for i, id := range p.Identities {
			out[i] = id.Scope
		}
		return out
	}
	// normalize returns the scopes the identities get on the supplier and on
	// the customer.
	normalize := func(regime l10n.TaxCountryCode, ids func() []*org.Identity) (supplier, customer []cbc.Key) {
		inv := &bill.Invoice{
			Supplier: &org.Party{Name: "Supplier", Identities: ids()},
			Customer: &org.Party{Name: "Customer", Identities: ids()},
		}
		norm.Normalize(inv, tax.AddonContext(en16931.V2017, pint.V1), tax.RegimeContext(regime))
		return scopes(inv.Supplier), scopes(inv.Customer)
	}
	tin := func() *org.Identity { return &org.Identity{Key: ae.IdentityKeyTIN, Code: "1234567897"} }
	eid := func() *org.Identity { return &org.Identity{Key: ae.IdentityKeyEmiratesID, Code: "784199012345671"} }
	passport := func() *org.Identity {
		return &org.Identity{Key: org.IdentityKeyPassport, Code: "P1234567890", Country: "IN"}
	}
	list := func(ids ...func() *org.Identity) func() []*org.Identity {
		return func() []*org.Identity {
			out := make([]*org.Identity, len(ids))
			for i, id := range ids {
				out[i] = id()
			}
			return out
		}
	}

	t.Run("legal registrations get the legal scope, and the supplier's TIN the tax scope", func(t *testing.T) {
		supplier, customer := normalize(l10n.AE.Tax(), list(tin, passport))
		assert.Equal(t, []cbc.Key{org.IdentityScopeTax, org.IdentityScopeLegal}, supplier)
		assert.Equal(t, []cbc.Key{"", org.IdentityScopeLegal}, customer, "PINT AE has no customer TIN registration")

		for _, key := range []cbc.Key{ae.IdentityKeyTradeLicense, ae.IdentityKeyEmiratesID, ae.IdentityKeyCabinetDecision} {
			id := func() *org.Identity { return &org.Identity{Key: key, Code: "784199012345671"} }
			supplier, customer = normalize(l10n.AE.Tax(), list(id))
			assert.Equal(t, []cbc.Key{org.IdentityScopeLegal}, supplier, key)
			assert.Equal(t, []cbc.Key{org.IdentityScopeLegal}, customer, key)
		}
	})

	t.Run("only the first identity of each scope gets it", func(t *testing.T) {
		supplier, customer := normalize(l10n.AE.Tax(), list(eid, passport))
		assert.Equal(t, []cbc.Key{org.IdentityScopeLegal, ""}, supplier)
		assert.Equal(t, []cbc.Key{org.IdentityScopeLegal, ""}, customer)
		supplier, _ = normalize(l10n.AE.Tax(), list(tin, tin))
		assert.Equal(t, []cbc.Key{org.IdentityScopeTax, ""}, supplier)
	})

	t.Run("a scope another identity already has is not given again", func(t *testing.T) {
		legal := func() *org.Identity { return &org.Identity{Scope: org.IdentityScopeLegal, Code: "CN-1234567"} }
		supplier, _ := normalize(l10n.AE.Tax(), list(passport, legal))
		assert.Equal(t, []cbc.Key{"", org.IdentityScopeLegal}, supplier)
	})

	t.Run("an explicit scope is kept", func(t *testing.T) {
		id := func() *org.Identity {
			id := eid()
			id.Scope = org.IdentityScopeTax
			return id
		}
		supplier, _ := normalize(l10n.AE.Tax(), list(id))
		assert.Equal(t, []cbc.Key{org.IdentityScopeTax}, supplier)
	})

	t.Run("other identities get no scope", func(t *testing.T) {
		other := func() *org.Identity { return &org.Identity{Type: "XYZ", Code: "123"} }
		supplier, _ := normalize(l10n.AE.Tax(), list(other))
		assert.Equal(t, []cbc.Key{""}, supplier)
	})

	t.Run("other parties are left alone", func(t *testing.T) {
		inv := &bill.Invoice{
			Supplier: &org.Party{Name: "Supplier"},
			Ordering: &bill.Ordering{Seller: &org.Party{Name: "Tax Rep", Identities: []*org.Identity{eid()}}},
		}
		norm.Normalize(inv, tax.AddonContext(en16931.V2017, pint.V1), tax.RegimeContext(l10n.AE.Tax()))
		assert.Empty(t, inv.Ordering.Seller.Identities[0].Scope)
	})

	t.Run("only under the AE regime", func(t *testing.T) {
		supplier, customer := normalize(l10n.AU.Tax(), list(tin, passport))
		assert.Equal(t, []cbc.Key{"", ""}, supplier)
		assert.Equal(t, []cbc.Key{"", ""}, customer)
	})
}
