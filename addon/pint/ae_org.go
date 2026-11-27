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

const (
	endpointSchemeMailto          = "mailto"
	endpointSchemeEmail  cbc.Code = "EM"
	tinScheme            cbc.Code = "0235"
	identitySchemeGTIN   cbc.Code = "0160"
)

// Predefined UAE Peppol endpoint codes (PINT AE BIS 1.5.3, IBT-049).
const (
	endpointDeemedSupply     = "9900000097"
	endpointNoPeppolID       = "9900000098"
	endpointNoPeppolIDExport = "9900000099"
)

var identityTypesAE = []cbc.Code{
	ae.IdentityTypeTradeLicense,
	ae.IdentityTypeEmiratesID,
	ae.IdentityTypeCabinetDecision,
}

// normalizeDeemedSupplyAE sets the predefined UAE deemed supply endpoint (PINT AE BIS 1.5.3, IBT-049).
func normalizeDeemedSupplyAE(inv *bill.Invoice) {
	if inv == nil || inv.Customer == nil || !ParseTransactionType(inv.Tax.GetExt(ExtKeyTransactionType)).DeemedSupply {
		return
	}
	inv.Customer.Endpoints = []*org.Endpoint{
		{URI: cbc.URI(iso.ActorIDScheme + "::" + tinScheme.String() + ":" + endpointDeemedSupply)},
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
				ib.Scheme = tinScheme
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
			URI: cbc.URI(iso.ActorIDScheme + "::" + tinScheme.String() + ":" + tin.String()),
		})
	}
}

func normalizeIdentityScopes(inv *bill.Invoice) {
	if inv == nil {
		return
	}
	setIdentityScopes(inv.Supplier, org.IdentityScopeLegal, org.IdentityScopeTax)
	setIdentityScopes(inv.Customer, org.IdentityScopeLegal)
}

func normalizeOrgItemAE(it *org.Item) {
	if it == nil {
		return
	}
	switch it.Key {
	case org.ItemKeyGoods:
		it.Ext = it.Ext.SetOneOf(ExtKeyItemType, ItemTypeGoods, ItemTypeGoodsServices)
	case org.ItemKeyServices:
		it.Ext = it.Ext.SetOneOf(ExtKeyItemType, ItemTypeServices, ItemTypeGoodsServices)
	}
}

func orgPartyRules() *rules.Set {
	return rules.For(new(org.Party),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			// IBR-183-AE exempts other 1XXXXXXXXX endpoints, but IBR-180-AE still requires a type.
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

func orgAddressRules() *rules.Set {
	return rules.For(new(org.Address),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.When(
				is.Func("UAE address", addressIsAE),
				rules.Field("region",
					rules.Assert("01", "region must be an emirate code (AUH, DXB, SHJ, UAQ, FUJ, AJM or RAK) when the country is AE (IBR-128-AE)",
						is.Present, is.Matches("^(AUH|DXB|SHJ|UAQ|FUJ|AJM|RAK)$"),
					),
				),
			),
		),
	)
}

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
			// IBR-101-AE exempts 9900000098/9900000099, but an identity rule cannot inspect its party's endpoint.
			rules.When(
				org.IdentityTypeIn(ae.IdentityTypeTradeLicense),
				rules.Field("description",
					rules.Assert("02", "trade license must have the issuing authority in the description (IBR-172-AE, IBR-101-AE)", is.Present),
				),
			),
		),
	)
}

func orgItemRules() *rules.Set {
	return rules.For(new(org.Item),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.When(
				is.Func("goods or goods and services", itemExtIn(ExtKeyItemType, ItemTypeGoods, ItemTypeGoodsServices)),
				rules.Field("identities",
					rules.Assert("01", "goods or goods and services item needs an HS item classification, an identity with key \"hsn\" (IBR-184-AE, IBR-186-AE)",
						org.IdentitiesKeyIn(org.IdentityKeyHSN),
					),
				),
			),
			rules.Field("ext",
				rules.Assert("02", "item type must be G, S or B (BTAE-13)",
					tax.ExtensionHasValidCode(ExtKeyItemType),
				),
			),
		),
	)
}

func partyEndpointID(p *org.Party) (cbc.Code, string) {
	if e := p.Endpoint(iso.ActorIDScheme); e != nil {
		// ISO 6523 URI format: "iso6523-actorid-upis::<scheme>:<code>".
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

func partyHasNonPredefinedUAEEndpoint(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	scheme, code := partyEndpointID(p)
	return scheme == tinScheme && code != endpointNoPeppolID && code != endpointNoPeppolIDExport
}

func supplierTIN(p *org.Party) cbc.Code {
	for _, id := range p.Identities {
		if id != nil && id.Type == ae.IdentityTypeTIN {
			return id.Code
		}
	}
	if scheme, code := partyEndpointID(p); scheme == tinScheme {
		return cbc.Code(code)
	}
	if p.TaxID == nil || p.TaxID.Country != l10n.AE.Tax() || len(p.TaxID.Code) < 10 {
		return ""
	}
	return p.TaxID.Code[:10]
}

func addressIsAE(val any) bool {
	a, ok := val.(*org.Address)
	return ok && a != nil && a.Country == l10n.AE.ISO()
}

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

func identityScopeAE(id *org.Identity) cbc.Key {
	if id.Type == ae.IdentityTypeTIN {
		return org.IdentityScopeTax
	}
	if id.Key == org.IdentityKeyPassport || id.Type.In(identityTypesAE...) {
		return org.IdentityScopeLegal
	}
	return ""
}

func identityScopeIn(scopes ...cbc.Key) func(any) bool {
	return func(val any) bool {
		id, ok := val.(*org.Identity)
		return ok && id != nil && id.Scope.In(scopes...)
	}
}

func identitiesScopeIn(scopes ...cbc.Key) func(any) bool {
	match := identityScopeIn(scopes...)
	return func(val any) bool {
		ids, ok := val.([]*org.Identity)
		return ok && slices.ContainsFunc(ids, func(id *org.Identity) bool { return match(id) })
	}
}

func itemExtIn(key cbc.Key, codes ...cbc.Code) func(any) bool {
	return func(val any) bool {
		it, ok := val.(*org.Item)
		return ok && it != nil && it.Ext.Get(key).In(codes...)
	}
}
