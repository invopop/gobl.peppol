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
	tests := []struct {
		id   *org.Identity
		want cbc.Code
	}{
		{&org.Identity{Type: ae.IdentityTypeTradeLicense}, "TL"},
		{&org.Identity{Type: ae.IdentityTypeEmiratesID}, "EID"},
		{&org.Identity{Key: org.IdentityKeyPassport}, pint.IdentityTypePassport},
		{&org.Identity{Type: ae.IdentityTypeCabinetDecision}, "CD"},
		{&org.Identity{Type: ae.IdentityTypeTIN}, ""},
		{&org.Identity{}, ""},
		{nil, ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, pint.IdentityTypeAE(tt.id), tt.id)
	}
}

func TestSetIdentityTypeAE(t *testing.T) {
	tests := map[cbc.Code]org.Identity{
		"TL":  {Type: ae.IdentityTypeTradeLicense},
		"EID": {Type: ae.IdentityTypeEmiratesID},
		"PAS": {Key: org.IdentityKeyPassport},
		"CD":  {Type: ae.IdentityTypeCabinetDecision},
		"XYZ": {},
		"":    {},
	}
	for typ, want := range tests {
		id := new(org.Identity)
		known := want.Key != "" || want.Type != ""
		assert.Equal(t, known, pint.SetIdentityTypeAE(id, typ), typ)
		assert.Equal(t, want, *id, typ)
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
	tin := func() *org.Identity { return &org.Identity{Type: ae.IdentityTypeTIN, Code: "1234567897"} }
	eid := func() *org.Identity { return &org.Identity{Type: ae.IdentityTypeEmiratesID, Code: "784199012345671"} }
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

		for _, typ := range []cbc.Code{ae.IdentityTypeTradeLicense, ae.IdentityTypeEmiratesID, ae.IdentityTypeCabinetDecision} {
			id := func() *org.Identity { return &org.Identity{Type: typ, Code: "784199012345671"} }
			supplier, customer = normalize(l10n.AE.Tax(), list(id))
			assert.Equal(t, []cbc.Key{org.IdentityScopeLegal}, supplier, typ)
			assert.Equal(t, []cbc.Key{org.IdentityScopeLegal}, customer, typ)
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

func TestOrgIdentityRulesAE(t *testing.T) {
	const (
		fault          = "GOBL-PEPPOL-PINT-ORG-IDENTITY-01"
		faultAuthority = "GOBL-PEPPOL-PINT-ORG-IDENTITY-02"
	)

	t.Run("passport needs the issuing country (IBR-012-AE, IBR-010-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = []*org.Identity{{Key: org.IdentityKeyPassport, Code: "P9876543210"}}
		assert.Contains(t, aeFaults(t, inv), fault)

		inv = aeIdentityDocument()
		inv.Customer.Identities = []*org.Identity{{Key: org.IdentityKeyPassport, Code: "P1234567890"}}
		assert.Contains(t, aeFaults(t, inv), fault)

		inv = aeIdentityDocument()
		inv.Customer.Identities = []*org.Identity{{Key: org.IdentityKeyPassport, Code: "P1234567890", Country: "IN"}}
		assert.NotContains(t, aeFaults(t, inv), fault)
	})
	t.Run("trade licence needs the issuing authority (IBR-172-AE, IBR-101-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities[0].Description = ""
		assert.Contains(t, aeFaults(t, inv), faultAuthority)

		// IBR-101-AE exempts these buyers, but the rule can't see the endpoint.
		inv = aeIdentityDocument()
		inv.Customer.Endpoints = uaeEndpoints("9900000098")
		inv.Customer.Identities[0].Description = ""
		assert.Contains(t, aeFaults(t, inv), faultAuthority)

		assert.NotContains(t, aeFaults(t, aeIdentityDocument()), faultAuthority)
	})
}
