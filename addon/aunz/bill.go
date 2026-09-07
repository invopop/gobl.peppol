package aunz

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
)

const (
	// countryAU is the ISO country code for Australia.
	countryAU = "AU"
	// countryNZ is the ISO country code for New Zealand.
	countryNZ = "NZ"

	// abnSchemeID is the ISO 6523 scheme identifier (0151) for an Australian endpoint
	abnSchemeID = "0151"
	// nzbnSchemeID is the ISO 6523 scheme identifier (0088) for a New Zealand endpoint
	nzbnSchemeID = "0088"
)

// billInvoiceRules enforces the A-NZ business rules.
func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		rules.Field("supplier",
			rules.Assert("01", "invoice supplier established in Australia must have an ABN legal identity (ALIGNED-IBR-001-AUNZ)",
				is.Func("has ABN legal identity", partyHasABN),
			),
			rules.Assert("02", "invoice supplier established in New Zealand must have an NZBN legal identity (ALIGNED-IBR-002-AUNZ)",
				is.Func("has NZBN legal identity", partyHasNZBN),
			),
		),
		rules.Field("customer",
			rules.Assert("03", "invoice customer established in Australia must have an ABN legal identity (ALIGNED-IBR-004-AUNZ)",
				is.Func("has ABN legal identity", partyHasABN),
			),
			rules.Assert("04", "invoice customer established in New Zealand must have an NZBN legal identity (ALIGNED-IBR-005-AUNZ)",
				is.Func("has NZBN legal identity", partyHasNZBN),
			),
		),
	)
}

// partyCountry returns the party's country, preferring the postal address
// country (ibt-040/ibt-055) and falling back to the tax identity country.
func partyCountry(p *org.Party) string {
	if p == nil {
		return ""
	}
	for _, a := range p.Addresses {
		if a != nil && a.Country != "" {
			return a.Country.String()
		}
	}
	if p.TaxID != nil {
		return p.TaxID.Country.String()
	}
	return ""
}

// partyHasLegalScheme reports whether the party carries a legal registration
// identity issued under the given ISO 6523 scheme.
func partyHasLegalScheme(p *org.Party, scheme string) bool {
	for _, id := range p.Identities {
		if id.Scope == org.IdentityScopeLegal && id.Ext.Get(iso.ExtKeySchemeID).String() == scheme {
			return true
		}
	}
	return false
}

// partyHasABN reports whether a party established in Australia carries its ABN
// as a legal identity; parties established elsewhere are not subject to the rule.
func partyHasABN(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true // presence of the party is asserted by the base PINT addon
	}
	if partyCountry(p) != countryAU {
		return true
	}
	return partyHasLegalScheme(p, abnSchemeID)
}

// partyHasNZBN reports whether a party established in New Zealand carries its
// NZBN as a legal identity; parties established elsewhere are not subject to the
// rule.
func partyHasNZBN(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true // presence of the party is asserted by the base PINT addon
	}
	if partyCountry(p) != countryNZ {
		return true
	}
	return partyHasLegalScheme(p, nzbnSchemeID)
}
