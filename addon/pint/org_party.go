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

// orgPartyRules validates parties on UAE documents.
//
// TODO: a delivery receiver with a UAE tax ID and a foreign address gets an emirate error PINT AE wouldn't.
func orgPartyRules() *rules.Set {
	return rules.For(new(org.Party),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			// gobl.ubl writes a UAE tax ID's country over the address's, so the address counts as UAE.
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
			// IBR-183-AE also exempts other 1XXXXXXXXX endpoints, but IBR-180-AE still asks them for a type.
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

// partyHasOneIdentification reports whether gobl.ubl writes at most one cac:PartyIdentification.
func partyHasOneIdentification(val any) bool {
	p, ok := val.(*org.Party)
	return ok && p != nil && len(partyIdentifications(p)) <= 1
}

// payeeIsNotSupplier reports whether the payee differs from the supplier by legal identity, else by
// identification, else by name.
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

// Patterns of the UAE Peppol endpoint codes the PINT AE rules depend on.
const (
	endpointPatternPredefined   = `^99000000(98|99)$`
	endpointPatternDeemedSupply = `^9900000097$`
	endpointPattern1or9         = `^[19]\d{9}$`
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

// normalizeSupplierAE adds the TIN identity (IBT-032) and 0235:<TIN> endpoint a UAE supplier's TRN implies,
// unless it already has a TIN or tax identity, or any endpoint or inbox.
// TODO: derive the customer's endpoint from its TRN too.
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

// partyTINFromTRN returns the TIN a valid UAE TRN starts with, or "", so an invalid TRN gives one error,
// not a second one on a derived TIN.
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
