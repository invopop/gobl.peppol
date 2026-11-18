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

// partyFirstAddressHasEmirate reports whether the party's first address,
// when in the UAE, has an emirate code as its region (IBR-128-AE). gobl.ubl
// writes it as the delivery's cac:DeliveryLocation/cac:Address, with the
// address's own country. A party without addresses passes.
func partyFirstAddressHasEmirate(val any) bool {
	a := partyFirstAddress(val)
	return a == nil || a.Country != l10n.AE.ISO() || cbc.Code(a.Region).In(emirateCodes...)
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
// address as a cbc:EndpointID that PINT accepts: an ISO 6523 endpoint or an
// inbox with a scheme. An email address is written with the scheme EM, which
// is not in PINT's list (ibr-cl-25). A party without endpoints passes, as
// rules 02 and 04 report it.
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

// endpointCodesPredefined are the predefined UAE Peppol endpoint codes that
// IBR-180-AE, IBR-183-AE and IBR-101-AE exempt from the buyer legal
// registration identifier type and authority.
var endpointCodesPredefined = []string{"9900000098", "9900000099"}

// endpointCodeDeemedSupply is the predefined UAE Peppol endpoint code of the
// buyer of a deemed supply (PINT AE BIS 1.5.3).
const endpointCodeDeemedSupply = "9900000097"

// endpoint1or9Regexp matches the UAE Peppol endpoint codes of 10 digits
// starting with 1 or 9, which need no buyer legal registration identifier
// (IBR-149-AE).
var endpoint1or9Regexp = regexp.MustCompile(`^[19]\d{9}$`)

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
	// partyHasUAEEndpointPredefined reports whether the party has one of the
	// predefined UAE endpoints, which IBR-180-AE, IBR-183-AE and IBR-101-AE
	// exempt.
	partyHasUAEEndpointPredefined = partyHasUAEEndpointWith(func(code string) bool {
		return slices.Contains(endpointCodesPredefined, code)
	})
	// partyHasUAEEndpointNotPredefined is the condition of IBR-180-AE and
	// IBR-183-AE.
	partyHasUAEEndpointNotPredefined = partyHasUAEEndpointWith(func(code string) bool {
		return !slices.Contains(endpointCodesPredefined, code)
	})
	// partyHasUAEEndpointDeemedSupply reports whether the party has the
	// predefined UAE endpoint of a deemed supply buyer.
	partyHasUAEEndpointDeemedSupply = partyHasUAEEndpointWith(func(code string) bool {
		return code == endpointCodeDeemedSupply
	})
	// partyHasUAEEndpointNot1or9 is the condition of IBR-149-AE: a code
	// that is not 1XXXXXXXXX or 9XXXXXXXXX.
	partyHasUAEEndpointNot1or9 = partyHasUAEEndpointWith(func(code string) bool {
		return !endpoint1or9Regexp.MatchString(code)
	})
	// partyHasUAEEndpointNotTIN is the condition of IBR-135-AE: a code that
	// is not a TIN, 1XXXXXXXXX.
	partyHasUAEEndpointNotTIN = partyHasUAEEndpointWith(func(code string) bool {
		return !tinRegexpAE.MatchString(code)
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
	return ok && p != nil && len(partyIdentifications(p)) > 0
}

// tinRegexpAE matches a UAE TIN, in the AE regime's format. UAE Peppol
// endpoints (0235) are TINs too, so it also tells whether an endpoint is one
// (IBR-135-AE).
var tinRegexpAE = regexp.MustCompile(ae.TINPattern)

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
			URI: cbc.URI(iso.ActorIDScheme + "::" + endpointSchemeUAE.String() + ":" + tin.String()),
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
