package pint

import (
	"strings"

	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

// emirateCodes are the country subdivision codes PINT AE accepts for UAE
// addresses (IBR-128-AE). They are PINT AE's own list, not ISO 3166-2.
var emirateCodes = []cbc.Code{
	"AUH", "DXB", "SHJ", "UAQ", "FUJ", "AJM", "RAK",
}

// endpointSchemeUAE is the Peppol participant identifier scheme for UAE
// endpoints, which the PINT AE legal registration rules depend on.
const endpointSchemeUAE cbc.Code = "0235"

// endpointSchemeMailto is the URI scheme of email endpoints, which gobl.ubl
// writes as cbc:EndpointID with the scheme endpointSchemeEmail.
const endpointSchemeMailto = "mailto"

// endpointSchemeEmail is the electronic address scheme gobl.ubl writes for
// email addresses.
const endpointSchemeEmail cbc.Code = "EM"

// partyHasIdentifier reports whether the party can be identified through a tax
// identity code or at least one organisation identity.
func partyHasIdentifier(val any) bool {
	p, ok := val.(*org.Party)
	return ok && p != nil && ((p.TaxID != nil && p.TaxID.Code != "") || len(p.Identities) > 0)
}

// partyHasTaxRegistration reports whether the party has a tax registration:
// a tax ID code or an identity with the tax scope. IBR-103-AE accepts any
// buyer PartyTaxScheme/CompanyID, and gobl.ubl writes either one there.
func partyHasTaxRegistration(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	if p.TaxID != nil && p.TaxID.Code != "" {
		return true
	}
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeTax {
			return true
		}
	}
	return false
}

// partyHasLegalIdentity reports whether the party has an identity with the
// legal scope. gobl.ubl writes the first one as
// PartyLegalEntity/CompanyID, the buyer legal registration identifier
// IBR-136-AE requires on 480 and 81.
func partyHasLegalIdentity(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeLegal {
			return true
		}
	}
	return false
}

// partyHasOneIdentification reports whether gobl.ubl writes at most one
// cac:PartyIdentification for the party, as PINT allows a single buyer
// identifier (ibr-sr-16). gobl.ubl writes every identity there except those
// with the tax scope and the first with the legal scope.
func partyHasOneIdentification(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	n := 0
	legal := false
	for _, id := range p.Identities {
		switch {
		case id == nil || id.Scope == org.IdentityScopeTax:
		case id.Scope == org.IdentityScopeLegal && !legal:
			legal = true
		default:
			n++
		}
	}
	return n <= 1
}

// partyFirstAddress returns the party's first address, the only one gobl.ubl
// writes as cac:PostalAddress, or nil if there is none.
func partyFirstAddress(val any) *org.Address {
	p, ok := val.(*org.Party)
	if !ok || p == nil || len(p.Addresses) == 0 {
		return nil
	}
	return p.Addresses[0]
}

// partyAddressIsComplete reports whether the party's first address has a
// street, city and region, written as cbc:StreetName, cbc:CityName and
// cbc:CountrySubentity (IBR-143-AE, IBR-144-AE). A party without an address
// passes: EN 16931 already requires supplier and customer addresses (BR-8,
// BR-10).
func partyAddressIsComplete(val any) bool {
	a := partyFirstAddress(val)
	return a == nil || (a.Street != "" && a.Locality != "" && a.Region != "")
}

// partyAddressHasEmirate reports whether the party's first address, when in
// the UAE, has an emirate code as its region (IBR-128-AE). Addresses in other
// countries pass, and so does a party without addresses: it is for the
// supplier and customer, whose addresses EN 16931 requires (BR-8, BR-10).
func partyAddressHasEmirate(val any) bool {
	return partyFirstAddress(val) == nil || partyPostalAddressHasEmirate(val)
}

// partyPostalAddressHasEmirate reports whether the cac:PostalAddress gobl.ubl
// writes for the party, when in the UAE, has an emirate code as its region
// (IBR-128-AE). gobl.ubl writes the country of the tax ID over the address's
// own, and writes an address with only that country for a party without
// addresses, which fails the rule.
func partyPostalAddressHasEmirate(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	a := partyFirstAddress(p)
	inAE := a != nil && a.Country == l10n.AE.ISO()
	if p.TaxID != nil && p.TaxID.Code != "" {
		inAE = p.TaxID.Country.Code() == l10n.AE
	}
	return !inAE || (a != nil && cbc.Code(a.Region).In(emirateCodes...))
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

// partyHasEndpointID reports whether gobl.ubl writes the party's electronic
// address as a cbc:EndpointID that PINT AE accepts: an ISO 6523 endpoint or an
// inbox with a scheme. An email address is written with the scheme EM, which
// is not in PINT AE's list (ibr-cl-25). A party without endpoints passes, as
// rule 04 reports it.
func partyHasEndpointID(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	if len(p.Endpoints) == 0 {
		return true
	}
	s, _ := partyEndpointID(p)
	return s != "" && s != endpointSchemeEmail
}

// partyHasUAEEndpointWith returns a predicate that reports whether the party's
// electronic address uses the UAE Peppol scheme 0235 with a code that match
// accepts.
func partyHasUAEEndpointWith(match func(code string) bool) func(any) bool {
	return func(val any) bool {
		p, ok := val.(*org.Party)
		if !ok || p == nil {
			return false
		}
		scheme, code := partyEndpointID(p)
		return scheme == endpointSchemeUAE && match(code)
	}
}

var (
	// partyHasUAEEndpoint is the condition of the legal registration rules.
	partyHasUAEEndpoint = partyHasUAEEndpointWith(func(string) bool { return true })
	// partyHasUAEEndpointNotStartingWith1or9 is the condition of IBR-149-AE.
	partyHasUAEEndpointNotStartingWith1or9 = partyHasUAEEndpointWith(func(code string) bool {
		return !strings.HasPrefix(code, "1") && !strings.HasPrefix(code, "9")
	})
	// partyHasUAEEndpointNotTIN is the condition of IBR-135-AE: a code that
	// is not a TIN, 1XXXXXXXXX.
	partyHasUAEEndpointNotTIN = partyHasUAEEndpointWith(func(code string) bool {
		return !tinRegexp.MatchString(code)
	})
)

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

// partyTaxIdentityCount returns how many of the party's identities have the
// tax scope. gobl.ubl writes each one as a cac:PartyTaxScheme.
func partyTaxIdentityCount(p *org.Party) int {
	n := 0
	for _, id := range p.Identities {
		if identityHasTaxScope(id) {
			n++
		}
	}
	return n
}

// partyHasAtMostOneTaxRegistration reports whether the party has at most one
// tax registration, counting the tax ID code and the tax scope identities, as
// the buyer can only have one PartyTaxScheme/CompanyID (IBR-179-AE).
func partyHasAtMostOneTaxRegistration(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	n := partyTaxIdentityCount(p)
	if p.TaxID != nil && p.TaxID.Code != "" {
		n++
	}
	return n <= 1
}

// partyHasIdentificationOrTaxRegistration reports whether the party has an
// identity gobl.ubl writes as cac:PartyIdentification (any identity that is
// neither a tax registration nor the first legal one) or a tax registration
// (IBR-135-AE).
func partyHasIdentificationOrTaxRegistration(val any) bool {
	if partyHasTaxRegistration(val) {
		return true
	}
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	legal := partyFirstLegalIdentity(p)
	for _, id := range p.Identities {
		if id != nil && id != legal && id.Scope != org.IdentityScopeTax {
			return true
		}
	}
	return false
}
