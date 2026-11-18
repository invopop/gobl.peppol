package pint

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/ae"
)

// PINT AE legal registration identifier types, written as @schemeAgencyID on
// the seller and buyer cbc:CompanyID (BTAE-15, BTAE-16). The trade licence
// (TL), Emirates ID (EID) and Cabinet Decision (CD) are the AE regime's
// identity types of the same codes; the passport is GOBL's passport key.
const (
	// IdentityTypePassport is a passport.
	IdentityTypePassport cbc.Code = "PAS"
)

// identityTypesAE are the AE regime's identity types that are PINT AE legal
// registration identifier types.
var identityTypesAE = []cbc.Code{
	ae.IdentityTypeTradeLicense,
	ae.IdentityTypeEmiratesID,
	ae.IdentityTypeCabinetDecision,
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

// normalizeIdentityScopes gives the UAE identities of the supplier and
// customer the scope gobl.ubl needs to write them where PINT AE reads them:
// legal for the trade licence, Emirates ID, passport and Cabinet Decision,
// written as cac:PartyLegalEntity/cbc:CompanyID, and tax for the supplier's
// TIN, written as a cac:PartyTaxScheme. PINT AE has no TIN tax registration
// for the customer, whose TIN stays a plain identity.
func normalizeIdentityScopes(inv *bill.Invoice) {
	if inv == nil {
		return
	}
	setIdentityScopes(inv.Supplier, org.IdentityScopeLegal, org.IdentityScopeTax)
	setIdentityScopes(inv.Customer, org.IdentityScopeLegal)
}

// setIdentityScopes gives the party's identities without a scope the one
// they have in PINT AE, when it is one of the given scopes and no other
// identity of the party has it, as EN 16931 allows one identity per scope
// (BT-30, BT-47, BT-31, BT-48).
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

// IdentitySchemeGTIN is the ISO 6523 scheme of a GTIN, the item standard
// identifier reverse charge lines need (IBR-174-AE).
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

// identityHasLegalScope reports whether an identity has the legal scope.
// EN 16931 allows one per party (GOBL-EU-EN16931-ORG-PARTY-02), the one
// gobl.ubl writes as cac:PartyLegalEntity/cbc:CompanyID.
func identityHasLegalScope(val any) bool {
	id, ok := val.(*org.Identity)
	return ok && id != nil && id.Scope == org.IdentityScopeLegal
}

// identityHasTaxScope reports whether an identity has the tax scope.
func identityHasTaxScope(val any) bool {
	id, ok := val.(*org.Identity)
	return ok && id != nil && id.Scope == org.IdentityScopeTax
}
