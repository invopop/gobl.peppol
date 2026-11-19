package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/norm"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeSupplierAE(t *testing.T) {
	// normalize returns the supplier after normalizing an invoice under the
	// regime.
	normalize := func(regime l10n.TaxCountryCode, p *org.Party) *org.Party {
		inv := &bill.Invoice{Supplier: p}
		norm.Normalize(inv, tax.AddonContext(en16931.V2017, pint.V1), tax.RegimeContext(regime))
		return inv.Supplier
	}
	trn := func() *org.Party {
		return &org.Party{Name: "Supplier", TaxID: &tax.Identity{Country: "AE", Code: "100000001600003"}}
	}
	tinIdentity := &org.Identity{Type: ae.IdentityTypeTIN, Code: "1000000016", Scope: org.IdentityScopeTax}
	tinEndpoint := &org.Endpoint{URI: "iso6523-actorid-upis::0235:1000000016"}

	t.Run("the TRN gives the TIN identity and the UAE Peppol endpoint", func(t *testing.T) {
		p := normalize(l10n.AE.Tax(), trn())
		assert.Equal(t, []*org.Identity{tinIdentity}, p.Identities)
		assert.Equal(t, []*org.Endpoint{tinEndpoint}, p.Endpoints)
	})

	t.Run("a TIN or tax scope identity is kept", func(t *testing.T) {
		for _, id := range []*org.Identity{
			{Type: ae.IdentityTypeTIN, Code: "1234567897"},
			{Scope: org.IdentityScopeTax, Code: "1234567897"},
		} {
			p := trn()
			p.Identities = []*org.Identity{id}
			p = normalize(l10n.AE.Tax(), p)
			assert.Len(t, p.Identities, 1)
			assert.Equal(t, cbc.Code("1234567897"), p.Identities[0].Code)
		}
	})

	t.Run("a Peppol endpoint is kept", func(t *testing.T) {
		p := trn()
		p.Endpoints = []*org.Endpoint{{URI: "iso6523-actorid-upis::0088:5790000435975"}}
		assert.Len(t, normalize(l10n.AE.Tax(), p).Endpoints, 1)
	})

	t.Run("an existing Peppol inbox stops the UAE endpoint", func(t *testing.T) {
		p := trn()
		p.Inboxes = []*org.Inbox{{Scheme: "0235", Code: "1234567897"}}
		assert.Empty(t, normalize(l10n.AE.Tax(), p).Endpoints)
	})

	t.Run("an email endpoint or inbox is kept, without the UAE Peppol endpoint", func(t *testing.T) {
		p := trn()
		p.Endpoints = []*org.Endpoint{{URI: "mailto:billing@example.com"}}
		assert.NotContains(t, normalize(l10n.AE.Tax(), p).Endpoints, tinEndpoint)

		p = trn()
		p.Inboxes = []*org.Inbox{{Email: "billing@example.com"}}
		assert.Empty(t, normalize(l10n.AE.Tax(), p).Endpoints)
	})

	t.Run("a tax ID that is not a valid TRN gives nothing", func(t *testing.T) {
		// The last one has the TRN format, but its TIN fails the check digit.
		for _, code := range []cbc.Code{"", "10000000160000", "200000001600003", "1000000016000A3", "100000001700003"} {
			p := trn()
			p.TaxID.Code = code
			p = normalize(l10n.AE.Tax(), p)
			assert.Empty(t, p.Identities, code)
			assert.Empty(t, p.Endpoints, code)
		}
		p := trn()
		p.TaxID = &tax.Identity{Country: "SA", Code: "100000001600003"}
		p = normalize(l10n.AE.Tax(), p)
		assert.Empty(t, p.Identities)
		assert.Empty(t, p.Endpoints)
	})

	t.Run("only under the AE regime", func(t *testing.T) {
		p := normalize(l10n.AU.Tax(), trn())
		assert.Empty(t, p.Identities)
		assert.Empty(t, p.Endpoints)
	})
}

func TestOrgPartyRulesAE(t *testing.T) {
	const (
		faultAddress   = "GOBL-PEPPOL-PINT-ORG-PARTY-01"
		faultEmirate   = "GOBL-PEPPOL-PINT-ORG-PARTY-02"
		faultLegalType = "GOBL-PEPPOL-PINT-ORG-PARTY-03"
	)
	standard := func() *bill.Invoice { return aeDocument(bill.InvoiceTypeStandard) }
	trn := func() *tax.Identity { return &tax.Identity{Country: "AE", Code: "100000003200003"} }
	oman := func() []*org.Address {
		return []*org.Address{{Street: "Street 1", Locality: "Muscat", Region: "Muscat", Country: "OM"}}
	}

	t.Run("a UAE tax ID makes every address count as in the UAE (IBR-128-AE)", func(t *testing.T) {
		inv := standard()
		inv.Supplier.Addresses = oman()
		assert.Contains(t, aeFaults(t, inv), faultEmirate)

		inv = standard()
		inv.Ordering = &bill.Ordering{Seller: &org.Party{Name: "Tax Rep", TaxID: trn(), Addresses: oman()}}
		assert.Contains(t, aeFaults(t, inv), faultEmirate)

		inv = standard()
		inv.Supplier.Addresses = oman()
		inv.Supplier.Addresses[0].Region = "DXB"
		assert.NotContains(t, aeFaults(t, inv), faultEmirate)
	})

	t.Run("a UAE tax ID needs an address, as one is written (IBR-128-AE)", func(t *testing.T) {
		inv := standard()
		inv.Ordering = &bill.Ordering{Issuer: &org.Party{Name: "Issuer", TaxID: trn()}}
		assert.Contains(t, aeFaults(t, inv), faultAddress)

		inv = standard()
		inv.Payment = &bill.PaymentDetails{Payee: &org.Party{Name: "Payee", TaxID: trn()}}
		assert.Contains(t, aeFaults(t, inv), faultAddress)
	})

	t.Run("UAE addresses are left to the address rules (IBR-128-AE)", func(t *testing.T) {
		inv := standard()
		inv.Supplier.Addresses[0].Region = "Dubai"
		assert.NotContains(t, aeFaults(t, inv), faultEmirate)
	})

	t.Run("other parties are not checked (IBR-128-AE)", func(t *testing.T) {
		inv := standard()
		inv.Lines[0].Seller = &org.Party{
			Name:      "Maker",
			TaxID:     &tax.Identity{Country: "DE", Code: "111111125"},
			Addresses: []*org.Address{{Street: "Hauptstr. 1", Locality: "Berlin", Region: "Berlin", Country: "DE"}},
		}
		inv.Ordering = &bill.Ordering{Issuer: &org.Party{Name: "Issuer"}}
		out := aeFaults(t, inv)
		assert.NotContains(t, out, faultAddress)
		assert.NotContains(t, out, faultEmirate)
	})

	t.Run("legal identity needs a PINT AE type with a UAE endpoint (IBR-173-AE, IBR-181-AE, IBR-180-AE, IBR-183-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "CN-1234567"}}
		assert.Contains(t, aeFaults(t, inv), faultLegalType)

		inv = aeIdentityDocument()
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "CN-7654321"}}
		assert.Contains(t, aeFaults(t, inv), faultLegalType)
	})

	t.Run("customer legal identity without a type is fine without a UAE endpoint", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Customer.Endpoints = validEndpoints("1345678906")
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "CN-7654321"}}
		assert.NotContains(t, aeFaults(t, inv), faultLegalType)
	})

	t.Run("customer with a predefined UAE endpoint needs no legal identity type (IBR-180-AE, IBR-183-AE)", func(t *testing.T) {
		for _, code := range []string{"9900000098", "9900000099"} {
			inv := aeIdentityDocument()
			inv.Customer.Endpoints = uaeEndpoints(code)
			inv.Customer.Identities = []*org.Identity{
				{Scope: org.IdentityScopeLegal, Code: "CN-7654321"},
			}
			assert.NotContains(t, aeFaults(t, inv), faultLegalType, code)
		}
	})

	t.Run("customer with another predefined UAE endpoint still needs one (IBR-180-AE, IBR-183-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Customer.Endpoints = uaeEndpoints("9900000097")
		inv.Customer.Identities = []*org.Identity{
			{Scope: org.IdentityScopeLegal, Code: "CN-7654321"},
		}
		assert.Contains(t, aeFaults(t, inv), faultLegalType)
	})

	t.Run("each PINT AE type is accepted (IBR-173-AE, IBR-183-AE)", func(t *testing.T) {
		for _, id := range []*org.Identity{
			{Type: ae.IdentityTypeEmiratesID, Code: "784199012345671"},
			{Key: org.IdentityKeyPassport, Code: "P1234567890", Country: "IN"},
			{Type: ae.IdentityTypeCabinetDecision, Code: "CD-12-2020"},
		} {
			inv := aeIdentityDocument()
			inv.Supplier.Identities = []*org.Identity{id}
			assert.Empty(t, aeFaults(t, inv), id.Key)
		}
	})

	t.Run("every legal identity is checked (IBR-173-AE)", func(t *testing.T) {
		// EN 16931 already rejects a second legal identity.
		inv := aeIdentityDocument()
		inv.Supplier.Identities[0].Scope = org.IdentityScopeLegal
		inv.Supplier.Identities = append(inv.Supplier.Identities,
			&org.Identity{Scope: org.IdentityScopeLegal, Code: "CN-1234567"})
		assert.Contains(t, aeFaults(t, inv), faultLegalType)
	})
}
