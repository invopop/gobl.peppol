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
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeDeemedSupplyAE(t *testing.T) {
	want := []*org.Endpoint{{URI: "iso6523-actorid-upis::0235:9900000097"}}
	normalize := func(regime l10n.TaxCountryCode, inv *bill.Invoice) {
		norm.Normalize(inv, tax.AddonContext(en16931.V2017, pint.V1), tax.RegimeContext(regime))
	}

	deemedTax := func() *bill.Tax {
		return &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyTransactionType: "01000000"})}
	}

	t.Run("sets deemed supply endpoint when customer has none", func(t *testing.T) {
		inv := &bill.Invoice{
			Tax:      deemedTax(),
			Customer: &org.Party{Name: "Buyer"},
		}
		normalize(l10n.AE.Tax(), inv)
		assert.Equal(t, want, inv.Customer.Endpoints)
	})

	t.Run("overrides existing customer endpoints", func(t *testing.T) {
		inv := &bill.Invoice{
			Tax: deemedTax(),
			Customer: &org.Party{
				Name:      "Buyer",
				Endpoints: uaeEndpoints("1345678906"),
			},
		}
		normalize(l10n.AE.Tax(), inv)
		assert.Equal(t, want, inv.Customer.Endpoints)
	})

	t.Run("overrides endpoint and Peppol inbox while keeping non-Peppol inboxes, and stays idempotent", func(t *testing.T) {
		inv := &bill.Invoice{
			Tax: deemedTax(),
			Customer: &org.Party{
				Name: "Buyer",
				Inboxes: []*org.Inbox{
					{Email: "buyer@example.com"},
					{Key: org.InboxKeyPeppol, Scheme: "0235", Code: "1345678906"},
					{Scheme: "0088", Code: "1234567890128"},
				},
			},
		}
		wantInboxes := []*org.Inbox{
			{Email: "buyer@example.com"},
			{Key: org.InboxKeyPeppol, Scheme: "0235", Code: "9900000097"},
		}
		normalize(l10n.AE.Tax(), inv)
		assert.Equal(t, want, inv.Customer.Endpoints)
		assert.Equal(t, wantInboxes, inv.Customer.Inboxes)
		// Second pass must remain idempotent and not re-append from the inbox.
		normalize(l10n.AE.Tax(), inv)
		assert.Equal(t, want, inv.Customer.Endpoints)
		assert.Equal(t, wantInboxes, inv.Customer.Inboxes)
	})

	t.Run("leaves customer untouched without the deemed supply flag or outside AE", func(t *testing.T) {
		orig := uaeEndpoints("1345678906")
		inv := &bill.Invoice{
			Customer: &org.Party{Name: "Buyer", Endpoints: orig},
		}
		normalize(l10n.AE.Tax(), inv)
		assert.Equal(t, orig, inv.Customer.Endpoints)

		inv = &bill.Invoice{
			Tax:      deemedTax(),
			Customer: &org.Party{Name: "Buyer", Endpoints: orig},
		}
		normalize(l10n.AU.Tax(), inv)
		assert.Equal(t, orig, inv.Customer.Endpoints)
	})
}

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

	t.Run("the supplier's own TIN comes before its TRN's, which a tax group shares", func(t *testing.T) {
		own := &org.Identity{Type: ae.IdentityTypeTIN, Code: "1234567897", Scope: org.IdentityScopeTax}
		ownEndpoint := &org.Endpoint{URI: "iso6523-actorid-upis::0235:1234567897"}

		p := trn()
		p.Identities = []*org.Identity{{Type: ae.IdentityTypeTIN, Code: "1234567897"}}
		assert.Equal(t, []*org.Endpoint{ownEndpoint}, normalize(l10n.AE.Tax(), p).Endpoints, "from the TIN identity")

		p = trn()
		p.Endpoints = []*org.Endpoint{ownEndpoint}
		assert.Equal(t, []*org.Identity{own}, normalize(l10n.AE.Tax(), p).Identities, "from the endpoint")

		p = trn()
		p.Inboxes = []*org.Inbox{{Scheme: "0235", Code: "1234567897"}}
		got := normalize(l10n.AE.Tax(), p)
		assert.Equal(t, []*org.Identity{own}, got.Identities, "from the inbox")
		assert.Empty(t, got.Endpoints, "from the inbox")
	})

	t.Run("an endpoint under another scheme is kept and leaves the TRN's", func(t *testing.T) {
		p := trn()
		p.Endpoints = []*org.Endpoint{{URI: "iso6523-actorid-upis::0088:5790000435975"}}
		got := normalize(l10n.AE.Tax(), p)
		assert.Equal(t, []*org.Identity{tinIdentity}, got.Identities)
		assert.Len(t, got.Endpoints, 1)
	})

	t.Run("an email endpoint or inbox is kept, without the UAE Peppol endpoint", func(t *testing.T) {
		p := trn()
		p.Endpoints = []*org.Endpoint{{URI: "mailto:billing@example.com"}}
		assert.NotContains(t, normalize(l10n.AE.Tax(), p).Endpoints, tinEndpoint)

		p = trn()
		p.Inboxes = []*org.Inbox{{Email: "billing@example.com"}}
		assert.Empty(t, normalize(l10n.AE.Tax(), p).Endpoints)
	})

	t.Run("a tax ID without a code or from another country gives nothing", func(t *testing.T) {
		p := trn()
		p.TaxID.Code = ""
		p = normalize(l10n.AE.Tax(), p)
		assert.Empty(t, p.Identities)
		assert.Empty(t, p.Endpoints)

		p = trn()
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
	const faultLegalType = "GOBL-PEPPOL-PINT-ORG-PARTY-01"

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

func TestOrgAddressRulesAE(t *testing.T) {
	const fault = "GOBL-PEPPOL-PINT-ORG-ADDRESS-01"
	standard := func() *bill.Invoice { return aeDocument(bill.InvoiceTypeStandard) }

	t.Run("UAE address region must be an emirate code (IBR-128-AE)", func(t *testing.T) {
		for _, region := range []string{"Dubai", ""} {
			inv := standard()
			inv.Customer.Addresses[0].Region = region
			assert.Contains(t, aeFaults(t, inv), fault, region)
		}
	})

	t.Run("every emirate code is accepted (IBR-128-AE)", func(t *testing.T) {
		for _, code := range []string{"AUH", "DXB", "SHJ", "UAQ", "FUJ", "AJM", "RAK"} {
			inv := standard()
			inv.Supplier.Addresses[0].Region = code
			assert.NotContains(t, aeFaults(t, inv), fault, code)
		}
	})

	t.Run("every party's UAE address is checked (IBR-128-AE)", func(t *testing.T) {
		for name, set := range map[string]func(inv *bill.Invoice, p *org.Party){
			"tax representative": func(inv *bill.Invoice, p *org.Party) { inv.Ordering = &bill.Ordering{Seller: p} },
			"issuer":             func(inv *bill.Invoice, p *org.Party) { inv.Ordering = &bill.Ordering{Issuer: p} },
			"line seller":        func(inv *bill.Invoice, p *org.Party) { inv.Lines[0].Seller = p },
			"delivery receiver":  func(inv *bill.Invoice, p *org.Party) { inv.Delivery = &bill.DeliveryDetails{Receiver: p} },
			"payee":              func(inv *bill.Invoice, p *org.Party) { inv.Payment = &bill.PaymentDetails{Payee: p} },
		} {
			inv := standard()
			set(inv, &org.Party{Name: "Party", Addresses: aeAddress("Dubai")})
			assert.Contains(t, aeFaults(t, inv), fault, name)

			inv = standard()
			set(inv, &org.Party{Name: "Party", Addresses: aeAddress("DXB")})
			assert.NotContains(t, aeFaults(t, inv), fault, name)
		}
	})

	t.Run("bank branch UAE address is checked too (IBR-128-AE)", func(t *testing.T) {
		inv := standard()
		inv.Payment = &bill.PaymentDetails{Instructions: &pay.Instructions{
			Key: pay.MeansKeyCreditTransfer,
			CreditTransfer: []*pay.CreditTransfer{{
				IBAN:   "AE070331234567890123456",
				Branch: &org.Address{Locality: "Dubai", Region: "Dubai", Country: "AE"},
			}},
		}}
		assert.Contains(t, aeFaults(t, inv), fault)
	})

	t.Run("foreign address region is not checked (IBR-128-AE)", func(t *testing.T) {
		inv := standard()
		inv.Customer.TaxID = nil
		inv.Customer.Addresses = []*org.Address{{Street: "Hauptstr. 1", Locality: "Berlin", Region: "Berlin", Country: "DE"}}
		assert.NotContains(t, aeFaults(t, inv), fault)
	})
}

func TestNormalizeIdentityScopes(t *testing.T) {
	scopes := func(p *org.Party) []cbc.Key {
		out := make([]cbc.Key, len(p.Identities))
		for i, id := range p.Identities {
			out[i] = id.Scope
		}
		return out
	}
	// normalize returns the scopes the identities get on the supplier and on
	// the customer.
	normalize := func(regime l10n.TaxCountryCode, ids func() []*org.Identity) (supplier, customer []cbc.Key) {
		inv := &bill.Invoice{
			Supplier: &org.Party{Name: "Supplier", Identities: ids()},
			Customer: &org.Party{Name: "Customer", Identities: ids()},
		}
		norm.Normalize(inv, tax.AddonContext(en16931.V2017, pint.V1), tax.RegimeContext(regime))
		return scopes(inv.Supplier), scopes(inv.Customer)
	}
	tin := func() *org.Identity { return &org.Identity{Type: ae.IdentityTypeTIN, Code: "1234567897"} }
	eid := func() *org.Identity { return &org.Identity{Type: ae.IdentityTypeEmiratesID, Code: "784199012345671"} }
	passport := func() *org.Identity {
		return &org.Identity{Key: org.IdentityKeyPassport, Code: "P1234567890", Country: "IN"}
	}
	list := func(ids ...func() *org.Identity) func() []*org.Identity {
		return func() []*org.Identity {
			out := make([]*org.Identity, len(ids))
			for i, id := range ids {
				out[i] = id()
			}
			return out
		}
	}

	t.Run("legal registrations get the legal scope, and the supplier's TIN the tax scope", func(t *testing.T) {
		supplier, customer := normalize(l10n.AE.Tax(), list(tin, passport))
		assert.Equal(t, []cbc.Key{org.IdentityScopeTax, org.IdentityScopeLegal}, supplier)
		assert.Equal(t, []cbc.Key{"", org.IdentityScopeLegal}, customer, "PINT AE has no customer TIN registration")

		for _, typ := range []cbc.Code{ae.IdentityTypeTradeLicense, ae.IdentityTypeEmiratesID, ae.IdentityTypeCabinetDecision} {
			id := func() *org.Identity { return &org.Identity{Type: typ, Code: "784199012345671"} }
			supplier, customer = normalize(l10n.AE.Tax(), list(id))
			assert.Equal(t, []cbc.Key{org.IdentityScopeLegal}, supplier, typ)
			assert.Equal(t, []cbc.Key{org.IdentityScopeLegal}, customer, typ)
		}
	})

	t.Run("only the first identity of each scope gets it", func(t *testing.T) {
		supplier, customer := normalize(l10n.AE.Tax(), list(eid, passport))
		assert.Equal(t, []cbc.Key{org.IdentityScopeLegal, ""}, supplier)
		assert.Equal(t, []cbc.Key{org.IdentityScopeLegal, ""}, customer)
		supplier, _ = normalize(l10n.AE.Tax(), list(tin, tin))
		assert.Equal(t, []cbc.Key{org.IdentityScopeTax, ""}, supplier)
	})

	t.Run("a scope another identity already has is not given again", func(t *testing.T) {
		legal := func() *org.Identity { return &org.Identity{Scope: org.IdentityScopeLegal, Code: "CN-1234567"} }
		supplier, _ := normalize(l10n.AE.Tax(), list(passport, legal))
		assert.Equal(t, []cbc.Key{"", org.IdentityScopeLegal}, supplier)
	})

	t.Run("an explicit scope is kept", func(t *testing.T) {
		id := func() *org.Identity {
			id := eid()
			id.Scope = org.IdentityScopeTax
			return id
		}
		supplier, _ := normalize(l10n.AE.Tax(), list(id))
		assert.Equal(t, []cbc.Key{org.IdentityScopeTax}, supplier)
	})

	t.Run("other identities get no scope", func(t *testing.T) {
		other := func() *org.Identity { return &org.Identity{Type: "XYZ", Code: "123"} }
		supplier, _ := normalize(l10n.AE.Tax(), list(other))
		assert.Equal(t, []cbc.Key{""}, supplier)
	})

	t.Run("other parties are left alone", func(t *testing.T) {
		inv := &bill.Invoice{
			Supplier: &org.Party{Name: "Supplier"},
			Ordering: &bill.Ordering{Seller: &org.Party{Name: "Tax Rep", Identities: []*org.Identity{eid()}}},
		}
		norm.Normalize(inv, tax.AddonContext(en16931.V2017, pint.V1), tax.RegimeContext(l10n.AE.Tax()))
		assert.Empty(t, inv.Ordering.Seller.Identities[0].Scope)
	})

	t.Run("only under the AE regime", func(t *testing.T) {
		supplier, customer := normalize(l10n.AU.Tax(), list(tin, passport))
		assert.Equal(t, []cbc.Key{"", ""}, supplier)
		assert.Equal(t, []cbc.Key{"", ""}, customer)
	})
}

func TestOrgIdentityRulesAE(t *testing.T) {
	const (
		fault          = "GOBL-PEPPOL-PINT-ORG-IDENTITY-01"
		faultAuthority = "GOBL-PEPPOL-PINT-ORG-IDENTITY-02"
	)

	t.Run("passport needs the issuing country (IBR-012-AE, IBR-010-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = []*org.Identity{{Key: org.IdentityKeyPassport, Code: "P9876543210"}}
		assert.Contains(t, aeFaults(t, inv), fault)

		inv = aeIdentityDocument()
		inv.Customer.Identities = []*org.Identity{{Key: org.IdentityKeyPassport, Code: "P1234567890"}}
		assert.Contains(t, aeFaults(t, inv), fault)

		inv = aeIdentityDocument()
		inv.Customer.Identities = []*org.Identity{{Key: org.IdentityKeyPassport, Code: "P1234567890", Country: "IN"}}
		assert.NotContains(t, aeFaults(t, inv), fault)
	})
	t.Run("trade licence needs the issuing authority (IBR-172-AE, IBR-101-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities[0].Description = ""
		assert.Contains(t, aeFaults(t, inv), faultAuthority)

		// IBR-101-AE exempts these buyers, but the rule can't see the endpoint.
		inv = aeIdentityDocument()
		inv.Customer.Endpoints = uaeEndpoints("9900000098")
		inv.Customer.Identities[0].Description = ""
		assert.Contains(t, aeFaults(t, inv), faultAuthority)

		assert.NotContains(t, aeFaults(t, aeIdentityDocument()), faultAuthority)
	})
}

func TestNormalizeOrgItemAE(t *testing.T) {
	normalize := func(regime l10n.TaxCountryCode, it *org.Item) {
		norm.Normalize(it, tax.AddonContext(en16931.V2017, pint.V1), tax.RegimeContext(regime))
	}

	t.Run("maps goods to G and services to S, updating when key changes", func(t *testing.T) {
		it := &org.Item{Key: org.ItemKeyGoods, Name: "Laptop"}
		normalize(l10n.AE.Tax(), it)
		assert.Equal(t, pint.ItemTypeGoods, it.Ext.Get(pint.ExtKeyItemType))

		// Changing goods -> services must not leave stale G behind.
		it.Key = org.ItemKeyServices
		normalize(l10n.AE.Tax(), it)
		assert.Equal(t, pint.ItemTypeServices, it.Ext.Get(pint.ExtKeyItemType))

		// Changing services -> goods must not leave stale S behind.
		it.Key = org.ItemKeyGoods
		normalize(l10n.AE.Tax(), it)
		assert.Equal(t, pint.ItemTypeGoods, it.Ext.Get(pint.ExtKeyItemType))
	})

	t.Run("preserves manual B even when item key is goods or services", func(t *testing.T) {
		for _, key := range []cbc.Key{org.ItemKeyGoods, org.ItemKeyServices, ""} {
			it := &org.Item{
				Key:  key,
				Name: "Bundle",
				Ext:  tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyItemType: pint.ItemTypeGoodsServices}),
			}
			normalize(l10n.AE.Tax(), it)
			assert.Equal(t, pint.ItemTypeGoodsServices, it.Ext.Get(pint.ExtKeyItemType), key)
		}
	})

	t.Run("leaves empty key untouched and does not run outside AE", func(t *testing.T) {
		it := &org.Item{Name: "Item"}
		normalize(l10n.AE.Tax(), it)
		assert.Empty(t, it.Ext.Get(pint.ExtKeyItemType))

		auItem := &org.Item{Key: org.ItemKeyGoods, Name: "Laptop"}
		normalize(l10n.AU.Tax(), auItem)
		assert.Empty(t, auItem.Ext.Get(pint.ExtKeyItemType))
	})
}

func TestOrgItemRulesAE(t *testing.T) {
	const (
		faultGoodsHS  = "GOBL-PEPPOL-PINT-ORG-ITEM-01"
		faultItemType = "GOBL-PEPPOL-PINT-ORG-ITEM-02"
	)
	hsn := &org.Identity{Key: org.IdentityKeyHSN, Code: "8471"}
	sac := &org.Identity{Label: "SAC", Code: "998713"}
	itemWithKey := func(key cbc.Key, ids ...*org.Identity) *bill.Invoice {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Item.Key = key
		inv.Lines[0].Item.Identities = ids
		return inv
	}
	itemWithType := func(typ cbc.Code, ids ...*org.Identity) *bill.Invoice {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Item.Ext = tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyItemType: typ})
		inv.Lines[0].Item.Identities = ids
		return inv
	}

	t.Run("G and B items need an HS classification with key hsn (IBR-184-AE, IBR-186-AE)", func(t *testing.T) {
		for _, typ := range []cbc.Code{pint.ItemTypeGoods, pint.ItemTypeGoodsServices} {
			out := aeFaults(t, itemWithType(typ))
			assert.Contains(t, out, faultGoodsHS, typ)
			assert.Contains(t, out, `"hsn"`, typ)
			assert.Contains(t, aeFaults(t, itemWithType(typ, sac)), faultGoodsHS, typ)
			assert.Contains(t, aeFaults(t, itemWithType(typ, &org.Identity{Label: "HS", Code: "8471"})), faultGoodsHS, typ)
			assert.NotContains(t, aeFaults(t, itemWithType(typ, hsn)), faultGoodsHS, typ)
		}

		// Via item key "goods" (normalized to pint-ae-item-type: G).
		assert.Contains(t, aeFaults(t, itemWithKey(org.ItemKeyGoods)), faultGoodsHS)
		assert.NotContains(t, aeFaults(t, itemWithKey(org.ItemKeyGoods, hsn)), faultGoodsHS)

		// Services and empty item type need no HS classification.
		assert.NotContains(t, aeFaults(t, itemWithKey(org.ItemKeyServices)), faultGoodsHS)
		assert.NotContains(t, aeFaults(t, itemWithKey("")), faultGoodsHS)
	})

	t.Run("item type must be G, S or B (BTAE-13)", func(t *testing.T) {
		for _, typ := range []cbc.Code{pint.ItemTypeGoods, pint.ItemTypeServices, pint.ItemTypeGoodsServices} {
			assert.NotContains(t, aeFaults(t, itemWithType(typ, hsn)), faultItemType, typ)
		}
		assert.Contains(t, aeFaults(t, itemWithType("X")), faultItemType)
	})
}
