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

	t.Run("a Peppol endpoint or inbox is kept", func(t *testing.T) {
		p := trn()
		p.Endpoints = []*org.Endpoint{{URI: "iso6523-actorid-upis::0088:5790000435975"}}
		assert.Len(t, normalize(l10n.AE.Tax(), p).Endpoints, 1)

		p = trn()
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
