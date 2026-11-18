package pint

import (
	"regexp"
	"slices"
	"strings"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pkg/luhn"
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

// orgPartyRules asks every party of a UAE document with a UAE tax ID for an
// address with an emirate code as its region (IBR-128-AE). gobl.ubl writes
// the country of a party's tax ID over the address's own in
// cac:PostalAddress, and an address with only that country when the party
// has none, so every address of the party is in the UAE there. The address
// rules already check the addresses whose own country is AE.
//
// It also checks the type of the legal identities of the parties with a UAE
// Peppol endpoint (IBR-173-AE, IBR-181-AE, IBR-180-AE, IBR-183-AE). Every one
// of those tests only fires with a 0235 cbc:EndpointID, so only UAE Peppol
// participants are checked; foreign payees, receivers and factors have none.
//
// TODO: gobl.ubl writes the delivery receiver's address with its own
// country, as cac:DeliveryLocation/cac:Address, so a receiver with a UAE tax
// ID and a foreign address gets an emirate error PINT AE wouldn't give.
func orgPartyRules() *rules.Set {
	return rules.For(new(org.Party),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.When(
				is.Func("UAE tax ID", partyHasAETaxID),
				rules.Field("addresses",
					rules.Assert("01", "address is required with a UAE tax ID (IBR-128-AE)", is.Present),
					rules.Each(
						rules.When(
							is.Not(is.Func("UAE address", addressIsAE)),
							rules.Field("region",
								rules.Assert("02", "region must be an emirate code with a UAE tax ID (IBR-128-AE)",
									is.Present, is.Matches(emiratePattern),
								),
							),
						),
					),
				),
			),
			// IBR-180-AE and IBR-183-AE exempt the buyers with the predefined
			// endpoints 9900000098 and 9900000099. IBR-183-AE also exempts
			// other 1XXXXXXXXX endpoints, but IBR-180-AE still asks them for a
			// type, and GOBL can only write the four in the list.
			rules.When(
				is.Func("has UAE Peppol endpoint other than 9900000098 or 9900000099", partyHasUAEEndpoint(endpointPatternPredefined, false)),
				rules.Field("identities", rules.Each(
					rules.When(
						is.Func("legal scope", identityScopeIn(org.IdentityScopeLegal)),
						rules.Assert("03", "legal identity must be a trade license, Emirates ID, passport or Cabinet Decision with a UAE Peppol endpoint other than 9900000098 or 9900000099 (IBR-173-AE, IBR-181-AE, IBR-180-AE, IBR-183-AE)",
							is.Or(org.IdentityKeyIn(org.IdentityKeyPassport), org.IdentityTypeIn(identityTypesAE...)),
						),
					),
				)),
			),
		),
	)
}

// partyHasAETaxID reports whether the party has a UAE tax ID with a code,
// which gobl.ubl writes with its country.
func partyHasAETaxID(val any) bool {
	p, ok := val.(*org.Party)
	return ok && p != nil && p.TaxID != nil && p.TaxID.Code != "" && p.TaxID.Country.Code() == l10n.AE
}

// partyHasOneIdentification reports whether gobl.ubl writes at most one
// cac:PartyIdentification for the party, as PINT allows a single buyer
// identifier (ibr-sr-16). gobl.ubl writes every identity there except those
// with the tax scope and the first with the legal scope.
func partyHasOneIdentification(val any) bool {
	p, ok := val.(*org.Party)
	return ok && p != nil && len(partyIdentifications(p)) <= 1
}

// payeeIsNotSupplier reports whether the invoice's payee differs from the
// supplier, compared as ibr-017 does: on the legal identities gobl.ubl writes
// as cac:PartyLegalEntity/cbc:CompanyID when both have one, else on the
// identities it writes as cac:PartyIdentification when both have any, else on
// the payee's name against the supplier's, regardless of case. Identities
// match on code and the scheme gobl.ubl writes. An invoice without a payee
// passes.
func payeeIsNotSupplier(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Payment == nil || inv.Payment.Payee == nil || inv.Supplier == nil {
		return true
	}
	payee, supplier := inv.Payment.Payee, inv.Supplier
	if p, s := partyFirstLegalIdentity(payee), partyFirstLegalIdentity(supplier); p != nil && s != nil {
		return p.Code != s.Code || p.Ext.Get(iso.ExtKeySchemeID) != s.Ext.Get(iso.ExtKeySchemeID)
	}
	if ps, ss := partyIdentifications(payee), partyIdentifications(supplier); len(ps) > 0 && len(ss) > 0 {
		for _, p := range ps {
			if slices.ContainsFunc(ss, func(s *org.Identity) bool {
				return p.Code == s.Code && identificationScheme(p) == identificationScheme(s)
			}) {
				return false
			}
		}
		return true
	}
	name := payee.Name
	if payee.Alias != "" {
		// gobl.ubl writes the alias as the payee's cac:PartyName.
		name = payee.Alias
	}
	return name == "" || supplier.Name == "" ||
		!strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(supplier.Name))
}

// partyIdentifications returns the party's identities gobl.ubl writes as
// cac:PartyIdentification: all but the tax scope ones and the first legal
// one.
func partyIdentifications(p *org.Party) []*org.Identity {
	legal := partyFirstLegalIdentity(p)
	var ids []*org.Identity
	for _, id := range p.Identities {
		if id != nil && id != legal && id.Scope != org.IdentityScopeTax {
			ids = append(ids, id)
		}
	}
	return ids
}

// identificationScheme returns the scheme gobl.ubl writes as the @schemeID
// of an identity's cac:PartyIdentification: its ISO 6523 scheme, or its type
// when it has no extensions.
func identificationScheme(id *org.Identity) cbc.Code {
	if id.Ext.IsZero() {
		return id.Type
	}
	return id.Ext.Get(iso.ExtKeySchemeID)
}

// partyEndpointID returns the scheme and code of the party's electronic
// address, chosen as gobl.ubl does for cbc:EndpointID: the first ISO 6523
// endpoint, or else the first email endpoint, or else the first inbox with an
// email or a scheme. Email addresses have the scheme EM.
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

// Patterns of the UAE Peppol endpoint codes the PINT AE rules depend on.
const (
	// endpointPatternPredefined matches the predefined endpoints 9900000098
	// and 9900000099, which IBR-180-AE and IBR-183-AE exempt.
	endpointPatternPredefined = `^99000000(98|99)$`
	// endpointPatternDeemedSupply matches the predefined endpoint of the
	// buyer of a deemed supply (PINT AE BIS 1.5.3).
	endpointPatternDeemedSupply = `^9900000097$`
	// endpointPattern1or9 matches the codes of 10 digits starting with 1 or
	// 9, which need no buyer legal registration identifier (IBR-149-AE).
	endpointPattern1or9 = `^[19]\d{9}$`
)

// partyHasUAEEndpoint returns a predicate that reports whether the party's
// electronic address, as gobl.ubl chooses it, uses the UAE Peppol scheme 0235
// with a code that matches the pattern, or that doesn't when match is false.
func partyHasUAEEndpoint(pattern string, match bool) func(any) bool {
	re := regexp.MustCompile(pattern)
	return func(val any) bool {
		p, ok := val.(*org.Party)
		if !ok || p == nil {
			return false
		}
		scheme, code := partyEndpointID(p)
		return scheme == ae.TINScheme && re.MatchString(code) == match
	}
}

// partyFirstLegalIdentity returns the party's first identity with the legal
// scope, the one gobl.ubl writes as cac:PartyLegalEntity/cbc:CompanyID, or nil
// if there is none.
func partyFirstLegalIdentity(val any) *org.Identity {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return nil
	}
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeLegal {
			return id
		}
	}
	return nil
}

// trnRegexpAE matches a UAE TRN, in the AE regime's format. Its first 10
// digits are the TIN.
var trnRegexpAE = regexp.MustCompile(ae.TRNPattern)

// normalizeSupplierAE fills in what the supplier's UAE TRN implies, when it
// is missing: the TIN the TRN starts with, as a tax scope identity of the
// TIN type (IBT-032), and the UAE Peppol endpoint, 0235 with the TIN, the
// one the UAE PASR gives every end user. An identity of the TIN type or with
// the tax scope is kept as it is, and the endpoint is only added to a
// supplier with no endpoint or inbox at all.
//
// TODO: the customer's endpoint could be derived from its TRN the same way,
// as every UAE end user is 0235:<TIN>, but it is not done yet.
func normalizeSupplierAE(inv *bill.Invoice) {
	if inv == nil || inv.Supplier == nil {
		return
	}
	p := inv.Supplier
	tin := partyTINFromTRN(p)
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
			URI: cbc.URI(iso.ActorIDScheme + "::" + ae.TINScheme.String() + ":" + tin.String()),
		})
	}
}

// partyTINFromTRN returns the TIN the party's UAE tax ID starts with, when
// the code has the TRN format and the TIN a valid check digit, as the AE
// regime checks them. It returns an empty code otherwise, so an invalid TRN
// gives one error, not a second one on a derived TIN.
func partyTINFromTRN(p *org.Party) cbc.Code {
	if p.TaxID == nil || p.TaxID.Country != l10n.AE.Tax() {
		return ""
	}
	code := p.TaxID.Code.String()
	if !trnRegexpAE.MatchString(code) || !luhn.Check(cbc.Code(code[:10])) {
		return ""
	}
	return cbc.Code(code[:10])
}
