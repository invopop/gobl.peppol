package au

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
)

// abnSchemeID is the ISO 6523 scheme identifier (0151) that marks an Australian
// Business Number carried as a party legal identity.
const abnSchemeID = "0151"

func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		rules.Field("customer",
			rules.Assert("01", "invoice customer established in Australia must have an ABN legal identity (ALIGNED-IBR-004-AUNZ)",
				is.Func("has ABN legal identity", partyHasABN),
			),
		),
	)
}

// partyIsAustralian reports whether the party's tax identity is registered in
// Australia.
func partyIsAustralian(p *org.Party) bool {
	return p.TaxID != nil && p.TaxID.Country.String() == "AU"
}

// partyHasABN reports whether an Australian party carries its ABN as a legal
// registration identity; parties established outside Australia are not subject
// to the rule.
func partyHasABN(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true // presence of the party is asserted by the base PINT addon
	}
	if !partyIsAustralian(p) {
		return true
	}
	for _, id := range p.Identities {
		if id.Scope == org.IdentityScopeLegal && id.Ext.Get(iso.ExtKeySchemeID).String() == abnSchemeID {
			return true
		}
	}
	return false
}
