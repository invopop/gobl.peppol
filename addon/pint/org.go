package pint

import (
	"slices"
	"strings"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// endpointSchemeMailto is the URI scheme of email endpoints, which gobl.ubl
// writes as cbc:EndpointID with the scheme endpointSchemeEmail.
const endpointSchemeMailto = "mailto"

// endpointSchemeEmail is the electronic address scheme gobl.ubl writes for
// email addresses.
const endpointSchemeEmail cbc.Code = "EM"

// endpointDeemedSupply is the predefined UAE Peppol endpoint code for deemed
// supplies (PINT AE BIS 1.5.3, IBT-049).
const endpointDeemedSupply = "9900000097"

// PINT AE party identity keys for roles written to dedicated UBL party elements.
const (
	// IdentityKeyPrincipal identifies the principal a disclosed agent bills on behalf of (BTAE-14).
	IdentityKeyPrincipal cbc.Key = "principal"
	// IdentityKeyBeneficiary identifies the beneficiary of a supply in a free trade zone (BTAE-01).
	IdentityKeyBeneficiary cbc.Key = "beneficiary"
)

// principalIDPattern is a UAE TRN ending with 03, as PINT AE asks of the principal ID (IBR-132-AE).
const principalIDPattern = `^1\d{12}03$`

// orgPartyRules validates parties on PINT documents, with the AE rules on UAE documents only.
func orgPartyRules() *rules.Set {
	return rules.For(new(org.Party),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			// IBR-183-AE also exempts other 1XXXXXXXXX endpoints, but IBR-180-AE still asks them for a type.
			rules.When(
				is.Func("has UAE Peppol endpoint other than 9900000098 or 9900000099", partyHasNonPredefinedUAEEndpoint),
				rules.Field("identities", rules.Each(
					rules.When(
						is.Func("legal scope", identityScopeIn(org.IdentityScopeLegal)),
						rules.Assert("01", "legal identity must be a trade license, Emirates ID, passport or Cabinet Decision with a UAE Peppol endpoint other than 9900000098 or 9900000099 (IBR-173-AE, IBR-181-AE, IBR-180-AE, IBR-183-AE)",
							is.Or(org.IdentityKeyIn(org.IdentityKeyPassport), org.IdentityTypeIn(identityTypesAE...)),
						),
					),
				)),
			),
		),
	)
}

// partyHasOneIdentification reports whether gobl.ubl writes at most one cac:PartyIdentification.
func partyHasOneIdentification(val any) bool {
	p, ok := val.(*org.Party)
	return ok && p != nil && len(partyIdentifications(p)) <= 1
}

// partyIdentifications returns the party's identities gobl.ubl writes as
// cac:PartyIdentification: all but the tax scope ones, the first legal one,
// and the PINT AE principal and beneficiary identities written elsewhere.
func partyIdentifications(p *org.Party) []*org.Identity {
	legal := partyFirstLegalIdentity(p)
	var ids []*org.Identity
	for _, id := range p.Identities {
		if id != nil && id != legal && id.Scope != org.IdentityScopeTax &&
			!id.Key.In(IdentityKeyPrincipal, IdentityKeyBeneficiary) {
			ids = append(ids, id)
		}
	}
	return ids
}

// partyEndpointID returns the scheme and code gobl.ubl writes as the party's cbc:EndpointID.
func partyEndpointID(p *org.Party) (cbc.Code, string) {
	if e := p.Endpoint(iso.ActorIDScheme); e != nil {
		// An ISO 6523 URI reads "iso6523-actorid-upis::<scheme>:<code>".
		scheme, code, ok := strings.Cut(strings.TrimPrefix(e.URI.Opaque(), ":"), ":")
		if ok && scheme != "" && code != "" {
			return cbc.Code(scheme), code
		}
	}
	if e := p.Endpoint(endpointSchemeMailto); e != nil && e.URI.Opaque() != "" {
		return endpointSchemeEmail, e.URI.Opaque()
	}
	for _, ib := range p.Inboxes {
		if ib == nil {
			continue
		}
		if ib.Email != "" {
			return endpointSchemeEmail, ib.Email
		}
		if ib.Scheme != "" {
			return ib.Scheme, ib.Code.String()
		}
	}
	return "", ""
}

// partyHasNonPredefinedUAEEndpoint reports whether the party's electronic
// address, as gobl.ubl chooses it, uses the UAE Peppol scheme 0235 with a
// code other than the predefined buyer endpoints 9900000098 and 9900000099.
func partyHasNonPredefinedUAEEndpoint(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	scheme, code := partyEndpointID(p)
	return scheme == TINScheme && code != "9900000098" && code != "9900000099"
}

// partyFirstLegalIdentity returns the party's first identity with the legal
// scope, the one gobl.ubl writes as cac:PartyLegalEntity/cbc:CompanyID, or nil
// if there is none. PINT AE principal and beneficiary identities are skipped.
func partyFirstLegalIdentity(val any) *org.Identity {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return nil
	}
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeLegal &&
			!id.Key.In(IdentityKeyPrincipal, IdentityKeyBeneficiary) {
			return id
		}
	}
	return nil
}

// normalizeDeemedSupplyAE overrides the customer's endpoints (and any Peppol
// inbox) to the predefined UAE deemed supply endpoint (0235:9900000097) when
// the invoice is tagged as a deemed supply (PINT AE BIS 1.5.3, IBT-049).
// Because norm traverses children before parents, this runs after EN 16931's
// inbox-to-endpoint migration.
func normalizeDeemedSupplyAE(inv *bill.Invoice) {
	if inv == nil || inv.Customer == nil || !inv.HasTags(TagDeemedSupply) {
		return
	}
	inv.Customer.Endpoints = []*org.Endpoint{
		{URI: cbc.URI(iso.ActorIDScheme + "::" + TINScheme.String() + ":" + endpointDeemedSupply)},
	}
	if len(inv.Customer.Inboxes) == 0 {
		return
	}
	var inboxes []*org.Inbox
	replaced := false
	for _, ib := range inv.Customer.Inboxes {
		if ib == nil {
			continue
		}
		if ib.Key == org.InboxKeyPeppol || ib.Scheme != cbc.CodeEmpty {
			if !replaced {
				ib.Scheme = TINScheme
				ib.Code = endpointDeemedSupply
				inboxes = append(inboxes, ib)
				replaced = true
			}
			continue
		}
		inboxes = append(inboxes, ib)
	}
	inv.Customer.Inboxes = inboxes
}

// normalizeSupplierAE adds the TIN identity (IBT-032) and 0235:<TIN> endpoint of a UAE supplier, unless it
// already has a TIN or tax identity, or any endpoint or inbox.
// TODO: derive the customer's endpoint from its TRN too.
func normalizeSupplierAE(inv *bill.Invoice) {
	if inv == nil || inv.Supplier == nil {
		return
	}
	p := inv.Supplier
	tin := supplierTIN(p)
	if tin == "" {
		return
	}
	if !slices.ContainsFunc(p.Identities, func(id *org.Identity) bool {
		return id != nil && (id.Type == ae.IdentityTypeTIN || id.Scope == org.IdentityScopeTax)
	}) {
		p.Identities = append(p.Identities, &org.Identity{
			Type:  ae.IdentityTypeTIN,
			Code:  tin,
			Scope: org.IdentityScopeTax,
		})
	}
	if len(p.Endpoints) == 0 && len(p.Inboxes) == 0 {
		p.Endpoints = append(p.Endpoints, &org.Endpoint{
			URI: cbc.URI(iso.ActorIDScheme + "::" + TINScheme.String() + ":" + tin.String()),
		})
	}
}

// supplierTIN returns the supplier's TIN: the one it gives as a TIN identity or as its UAE Peppol endpoint, or
// else the first 10 digits of its TRN. A tax group member must give its own, as its TRN is the group's.
func supplierTIN(p *org.Party) cbc.Code {
	for _, id := range p.Identities {
		if id != nil && id.Type == ae.IdentityTypeTIN {
			return id.Code
		}
	}
	if scheme, code := partyEndpointID(p); scheme == TINScheme {
		return cbc.Code(code)
	}
	if p.TaxID == nil || p.TaxID.Country != l10n.AE.Tax() || len(p.TaxID.Code) < 10 {
		return ""
	}
	return p.TaxID.Code[:10]
}

// emiratePattern matches the emirate codes PINT AE accepts, its own list rather than ISO 3166-2.
const emiratePattern = `^(AUH|DXB|SHJ|UAQ|FUJ|AJM|RAK)$`

// orgAddressRules validates addresses on UAE documents.
func orgAddressRules() *rules.Set {
	return rules.For(new(org.Address),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.When(
				is.Func("UAE address", addressIsAE),
				rules.Field("region",
					rules.Assert("01", "region must be an emirate code (AUH, DXB, SHJ, UAQ, FUJ, AJM or RAK) when the country is AE (IBR-128-AE)",
						is.Present, is.Matches(emiratePattern),
					),
				),
			),
		),
	)
}

// addressIsAE reports whether an address is in the UAE.
func addressIsAE(val any) bool {
	a, ok := val.(*org.Address)
	return ok && a != nil && a.Country == l10n.AE.ISO()
}

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
			rules.When(
				org.IdentityKeyIn(IdentityKeyPrincipal),
				rules.Field("code",
					rules.Assert("03", "principal identity code must be a TRN of 15 digits starting with 1 and ending with 03 (IBR-132-AE)",
						is.Matches(principalIDPattern),
					),
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
		if id != nil && !id.Key.In(IdentityKeyPrincipal, IdentityKeyBeneficiary) {
			taken[id.Scope] = true
		}
	}
	for _, id := range p.Identities {
		if id == nil || id.Scope != "" || id.Key.In(IdentityKeyPrincipal, IdentityKeyBeneficiary) {
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

// ItemKeyGoodsServices is the item key for PINT AE's item type B, both goods and services.
const ItemKeyGoodsServices cbc.Key = "goods-services"

// PINT AE item types, written as the item's
// cac:CommodityClassification/cbc:CommodityCode (BTAE-13).
const (
	ItemTypeGoods         cbc.Code = "G"
	ItemTypeServices      cbc.Code = "S"
	ItemTypeGoodsServices cbc.Code = "B"
)

// Labels of the item identities PINT AE uses.
const (
	IdentityLabelHS  = "HS"  // HS item classification
	IdentityLabelSAC = "SAC" // service accounting code
)

// ItemTypeAE returns the PINT AE item type for an item key, or an empty code
// if the key has none.
func ItemTypeAE(key cbc.Key) cbc.Code {
	switch key {
	case org.ItemKeyGoods:
		return ItemTypeGoods
	case org.ItemKeyServices:
		return ItemTypeServices
	case ItemKeyGoodsServices:
		return ItemTypeGoodsServices
	}
	return ""
}

// ItemKeyAE returns the item key for a PINT AE item type, or an empty key if
// the type is unknown.
func ItemKeyAE(typ cbc.Code) cbc.Key {
	switch typ {
	case ItemTypeGoods:
		return org.ItemKeyGoods
	case ItemTypeServices:
		return org.ItemKeyServices
	case ItemTypeGoodsServices:
		return ItemKeyGoodsServices
	}
	return ""
}

// orgItemRules validates items on UAE documents.
func orgItemRules() *rules.Set {
	return rules.For(new(org.Item),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.When(
				is.Func("goods or goods and services", itemKeyIn(org.ItemKeyGoods, ItemKeyGoodsServices)),
				rules.Field("identities",
					rules.Assert("01", "goods or goods and services item needs an HS item classification, an identity with the label HS (IBR-184-AE, IBR-186-AE)",
						is.Func("has HS", identitiesHaveLabel(IdentityLabelHS)),
					),
				),
			),
		),
	)
}

// itemKeyIn returns a predicate that reports whether an item has one of the keys.
func itemKeyIn(keys ...cbc.Key) func(any) bool {
	return func(val any) bool {
		it, ok := val.(*org.Item)
		return ok && it != nil && it.Key.In(keys...)
	}
}

// identitiesHaveLabel returns a predicate that reports whether one of the item classifications,
// the identities with a label and no scheme ID, has the label.
func identitiesHaveLabel(label string) func(any) bool {
	return func(val any) bool {
		ids, ok := val.([]*org.Identity)
		if !ok {
			return false
		}
		for _, id := range ids {
			if id != nil && id.Label == label && !id.Ext.Has(iso.ExtKeySchemeID) {
				return true
			}
		}
		return false
	}
}
