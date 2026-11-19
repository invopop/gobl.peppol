package pint

import (
	"slices"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// IdentityTypePassport is the PINT AE legal registration type of a passport (BTAE-15, BTAE-16).
const IdentityTypePassport cbc.Code = "PAS"

// identityTypesAE are the AE regime's identity types that are PINT AE legal
// registration identifier types.
var identityTypesAE = []cbc.Code{
	ae.IdentityTypeTradeLicense,
	ae.IdentityTypeEmiratesID,
	ae.IdentityTypeCabinetDecision,
}

// orgIdentityRules validates identities on UAE documents.
func orgIdentityRules() *rules.Set {
	return rules.For(new(org.Identity),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.When(
				org.IdentityKeyIn(org.IdentityKeyPassport),
				rules.Field("country",
					rules.Assert("01", "passport must have the issuing country (IBR-012-AE, IBR-010-AE)", is.Present),
				),
			),
			// IBR-101-AE exempts the endpoints 9900000098 and 9900000099, but an identity can't see its party's endpoint.
			rules.When(
				org.IdentityTypeIn(ae.IdentityTypeTradeLicense),
				rules.Field("description",
					rules.Assert("02", "trade license must have the issuing authority in the description (IBR-172-AE, IBR-101-AE)", is.Present),
				),
			),
		),
	)
}

// IdentityTypeAE returns the PINT AE legal registration identifier type of an
// identity, or an empty code if it has none.
func IdentityTypeAE(id *org.Identity) cbc.Code {
	switch {
	case id == nil:
		return ""
	case id.Key == org.IdentityKeyPassport:
		return IdentityTypePassport
	case id.Type.In(identityTypesAE...):
		return id.Type
	}
	return ""
}

// SetIdentityTypeAE sets the identity's key or type from a PINT AE legal
// registration identifier type, and reports whether the type is known.
func SetIdentityTypeAE(id *org.Identity, typ cbc.Code) bool {
	switch {
	case typ == IdentityTypePassport:
		id.Key = org.IdentityKeyPassport
	case typ.In(identityTypesAE...):
		id.Type = typ
	default:
		return false
	}
	return true
}

// normalizeIdentityScopes scopes the supplier and customer identities where PINT AE reads them: legal
// for the registration types, and tax for the supplier's TIN.
func normalizeIdentityScopes(inv *bill.Invoice) {
	if inv == nil {
		return
	}
	setIdentityScopes(inv.Supplier, org.IdentityScopeLegal, org.IdentityScopeTax)
	setIdentityScopes(inv.Customer, org.IdentityScopeLegal)
}

// setIdentityScopes gives unscoped identities their PINT AE scope, if it's one of scopes and still free, as
// EN 16931 allows one identity per scope.
func setIdentityScopes(p *org.Party, scopes ...cbc.Key) {
	if p == nil {
		return
	}
	taken := make(map[cbc.Key]bool)
	for _, id := range p.Identities {
		if id != nil {
			taken[id.Scope] = true
		}
	}
	for _, id := range p.Identities {
		if id == nil || id.Scope != "" {
			continue
		}
		if scope := identityScopeAE(id); scope.In(scopes...) && !taken[scope] {
			id.Scope = scope
			taken[scope] = true
		}
	}
}

// identityScopeAE returns the scope PINT AE reads an identity from, or an
// empty key if it has none.
func identityScopeAE(id *org.Identity) cbc.Key {
	if id.Type == ae.IdentityTypeTIN {
		return org.IdentityScopeTax
	}
	if IdentityTypeAE(id) != "" {
		return org.IdentityScopeLegal
	}
	return ""
}

// IdentitySchemeGTIN is the ISO 6523 scheme of a GTIN.
const IdentitySchemeGTIN cbc.Code = "0160"

// GTINIdentity returns the first identity with the GTIN scheme, or nil.
func GTINIdentity(ids []*org.Identity) *org.Identity {
	for _, id := range ids {
		if id != nil && id.Ext.Get(iso.ExtKeySchemeID) == IdentitySchemeGTIN {
			return id
		}
	}
	return nil
}

// identityScopeIn returns a predicate that reports whether an identity has one of the scopes, "" for none.
func identityScopeIn(scopes ...cbc.Key) func(any) bool {
	return func(val any) bool {
		id, ok := val.(*org.Identity)
		return ok && id != nil && id.Scope.In(scopes...)
	}
}

// identitiesScopeIn returns a predicate that reports whether any of the
// identities has one of the scopes, the empty one for none.
func identitiesScopeIn(scopes ...cbc.Key) func(any) bool {
	match := identityScopeIn(scopes...)
	return func(val any) bool {
		ids, ok := val.([]*org.Identity)
		return ok && slices.ContainsFunc(ids, func(id *org.Identity) bool { return match(id) })
	}
}
