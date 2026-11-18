package ubl

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/catalogues/icc"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/tax"
	"github.com/invopop/gobl/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func aeCategory(id string, percent *string) goblubl.TaxCategory {
	return goblubl.TaxCategory{
		ID:        &goblubl.IDType{Value: id},
		Percent:   percent,
		TaxScheme: &goblubl.TaxScheme{ID: goblubl.IDType{Value: "VAT"}},
	}
}

func aedAmount(v string) goblubl.Amount {
	return goblubl.Amount{CurrencyID: strPtr("AED"), Value: v}
}

func TestExportAEUUID(t *testing.T) {
	out := new(goblubl.Invoice)
	inv := &bill.Invoice{}
	exportAEUUID(out, inv)
	assert.Empty(t, out.UUID, "no UUID to write")

	inv.UUID = uuid.MustParse("0195ce71-dc9c-72c8-bf2c-9890a4a9f0a2")
	exportAEUUID(out, inv)
	assert.Equal(t, "0195ce71-dc9c-72c8-bf2c-9890a4a9f0a2", out.UUID)
}

func TestImportAEUUID(t *testing.T) {
	inv := new(bill.Invoice)
	importAEUUID(&goblubl.Invoice{UUID: " 0195ce71-dc9c-72c8-bf2c-9890a4a9f0a2 "}, inv)
	assert.Equal(t, "0195ce71-dc9c-72c8-bf2c-9890a4a9f0a2", inv.UUID.String())

	for _, bad := range []string{"", "not-a-uuid"} {
		inv = new(bill.Invoice)
		importAEUUID(&goblubl.Invoice{UUID: bad}, inv)
		assert.True(t, inv.UUID.IsZero(), bad)
	}
}

func TestExportAEPrice(t *testing.T) {
	t.Run("adds the base quantity and a zero discount on the net price", func(t *testing.T) {
		line := &goblubl.InvoiceLine{
			InvoicedQuantity: &goblubl.Quantity{UnitCode: "HUR", Value: "10"},
			Price:            &goblubl.Price{PriceAmount: aedAmount("100.00")},
		}
		exportAEPrice(line)
		require.NotNil(t, line.Price.BaseQuantity)
		assert.Equal(t, goblubl.Quantity{UnitCode: "HUR", Value: "1"}, *line.Price.BaseQuantity)
		ac := line.Price.AllowanceCharge
		require.NotNil(t, ac)
		assert.False(t, ac.ChargeIndicator)
		assert.Equal(t, "0.00", ac.Amount.Value)
		require.NotNil(t, ac.BaseAmount)
		assert.Equal(t, "100.00", ac.BaseAmount.Value)
	})

	t.Run("writes the zero discount with the decimals of the price", func(t *testing.T) {
		line := &goblubl.InvoiceLine{
			InvoicedQuantity: &goblubl.Quantity{UnitCode: "C62", Value: "1"},
			Price:            &goblubl.Price{PriceAmount: aedAmount("10.1234")},
		}
		exportAEPrice(line)
		assert.Equal(t, "0.0000", line.Price.AllowanceCharge.Amount.Value)
	})

	t.Run("uses the credited quantity's unit on credit notes", func(t *testing.T) {
		line := &goblubl.InvoiceLine{
			CreditedQuantity: &goblubl.Quantity{UnitCode: "C62", Value: "1"},
			Price:            &goblubl.Price{PriceAmount: aedAmount("5.00")},
		}
		exportAEPrice(line)
		assert.Equal(t, "C62", line.Price.BaseQuantity.UnitCode)
	})

	t.Run("keeps a price discount gobl.ubl already wrote", func(t *testing.T) {
		existing := &goblubl.AllowanceCharge{Amount: aedAmount("20.00"), BaseAmount: &goblubl.Amount{Value: "120.00"}}
		line := &goblubl.InvoiceLine{
			InvoicedQuantity: &goblubl.Quantity{UnitCode: "C62", Value: "1"},
			Price:            &goblubl.Price{PriceAmount: aedAmount("100.00"), AllowanceCharge: existing},
		}
		exportAEPrice(line)
		assert.Same(t, existing, line.Price.AllowanceCharge)
	})

	t.Run("lines without a price are left alone", func(t *testing.T) {
		line := &goblubl.InvoiceLine{}
		exportAEPrice(line)
		assert.Nil(t, line.Price)
	})
}

func TestAERate(t *testing.T) {
	zero := strPtr("0")
	assert.Nil(t, aeRate(&goblubl.IDType{Value: "E"}, zero), "exempt has no rate")
	rc := aeRate(&goblubl.IDType{Value: "AE"}, zero)
	require.NotNil(t, rc)
	assert.Equal(t, "5", *rc, "reverse charge is written at 5")
	ms := aeRate(&goblubl.IDType{Value: "N"}, zero)
	require.NotNil(t, ms)
	assert.Equal(t, "5", *ms, "margin scheme is written at 5")
	assert.Equal(t, strPtr("5"), aeRate(&goblubl.IDType{Value: "S"}, strPtr("5")), "standard keeps its rate")
	assert.Nil(t, aeRate(&goblubl.IDType{Value: "O"}, nil), "outside scope keeps no rate")
	assert.Equal(t, zero, aeRate(nil, zero), "no category, no change")
}

func TestMergeAETaxSubtotals(t *testing.T) {
	exempt := func(base, code string) goblubl.TaxSubtotal {
		c := aeCategory("E", nil)
		c.TaxExemptionReasonCode = strPtr(code)
		return goblubl.TaxSubtotal{TaxableAmount: aedAmount(base), TaxAmount: aedAmount("0.00"), TaxCategory: c}
	}
	standard := goblubl.TaxSubtotal{
		TaxableAmount: aedAmount("1000.00"),
		TaxAmount:     aedAmount("50.00"),
		TaxCategory:   aeCategory("S", strPtr("5")),
	}

	t.Run("merges breakdowns of the same category and drops differing codes", func(t *testing.T) {
		out := mergeAETaxSubtotals([]goblubl.TaxSubtotal{standard, exempt("200.00", "VATEX-1"), exempt("280.00", "VATEX-2")})
		require.Len(t, out, 2)
		assert.Equal(t, "1000.00", out[0].TaxableAmount.Value)
		assert.Equal(t, "480.00", out[1].TaxableAmount.Value)
		assert.Equal(t, "0.00", out[1].TaxAmount.Value)
		assert.Nil(t, out[1].TaxCategory.TaxExemptionReasonCode)
	})

	t.Run("keeps a code all merged breakdowns share", func(t *testing.T) {
		out := mergeAETaxSubtotals([]goblubl.TaxSubtotal{exempt("200.00", "VATEX-1"), exempt("100.00", "VATEX-1")})
		require.Len(t, out, 1)
		assert.Equal(t, "300.00", out[0].TaxableAmount.Value)
		assert.Equal(t, strPtr("VATEX-1"), out[0].TaxCategory.TaxExemptionReasonCode)
	})

	t.Run("does not merge different rates", func(t *testing.T) {
		zero := goblubl.TaxSubtotal{TaxableAmount: aedAmount("10.00"), TaxAmount: aedAmount("0.00"), TaxCategory: aeCategory("S", strPtr("0"))}
		out := mergeAETaxSubtotals([]goblubl.TaxSubtotal{standard, zero})
		assert.Len(t, out, 2)
	})

	t.Run("merges rates written differently (ALIGNED-IBRP-Z-01)", func(t *testing.T) {
		zero := func(base, pc string) goblubl.TaxSubtotal {
			return goblubl.TaxSubtotal{TaxableAmount: aedAmount(base), TaxAmount: aedAmount("0.00"), TaxCategory: aeCategory("Z", strPtr(pc))}
		}
		out := mergeAETaxSubtotals([]goblubl.TaxSubtotal{zero("10.00", "0"), zero("20.00", "0.0")})
		require.Len(t, out, 1)
		assert.Equal(t, "30.00", out[0].TaxableAmount.Value)
	})
}

func TestExportAETaxTotal(t *testing.T) {
	tt := &goblubl.TaxTotal{TaxSubtotal: []goblubl.TaxSubtotal{
		{TaxableAmount: aedAmount("100.00"), TaxAmount: aedAmount("0.00"), TaxCategory: aeCategory("AE", strPtr("0"))},
		{TaxableAmount: aedAmount("50.00"), TaxAmount: aedAmount("0.00"), TaxCategory: aeCategory("E", strPtr("0"))},
	}}
	exportAETaxTotal(tt)
	require.Len(t, tt.TaxSubtotal, 2)
	assert.Equal(t, strPtr("5"), tt.TaxSubtotal[0].TaxCategory.Percent)
	assert.Nil(t, tt.TaxSubtotal[1].TaxCategory.Percent)
	require.NotNil(t, tt.TaxIncludedIndicator)
	assert.False(t, *tt.TaxIncludedIndicator)

	// The VAT total in AED has no breakdowns, and no indicator.
	aed := &goblubl.TaxTotal{TaxAmount: aedAmount("5.00")}
	exportAETaxTotal(aed)
	assert.Nil(t, aed.TaxIncludedIndicator)
}

func TestExportAEPartyTaxID(t *testing.T) {
	party := func(companyID string) *goblubl.Party {
		return &goblubl.Party{PartyTaxScheme: []goblubl.PartyTaxScheme{{
			CompanyID: &goblubl.IDType{Value: companyID},
			TaxScheme: &goblubl.TaxScheme{ID: goblubl.IDType{Value: "VAT"}},
		}}}
	}

	t.Run("writes a UAE TRN without its prefix", func(t *testing.T) {
		out := party("AE100000001600003")
		exportAEPartyTaxID(out, &tax.Identity{Country: "AE", Code: "100000001600003"})
		assert.Equal(t, "100000001600003", out.PartyTaxScheme[0].CompanyID.Value)
	})

	t.Run("keeps the prefix of other countries' VAT numbers", func(t *testing.T) {
		out := party("DE111111125")
		exportAEPartyTaxID(out, &tax.Identity{Country: "DE", Code: "111111125"})
		assert.Equal(t, "DE111111125", out.PartyTaxScheme[0].CompanyID.Value)
	})

	t.Run("leaves other tax registrations alone", func(t *testing.T) {
		out := party("1234567897")
		exportAEPartyTaxID(out, &tax.Identity{Country: "AE", Code: "100000001600003"})
		assert.Equal(t, "1234567897", out.PartyTaxScheme[0].CompanyID.Value)
	})

	t.Run("parties without a tax ID are skipped", func(t *testing.T) {
		out := party("AE100000001600003")
		exportAEPartyTaxID(out, nil)
		assert.Equal(t, "AE100000001600003", out.PartyTaxScheme[0].CompanyID.Value)
	})
}

func TestExportAEPartyIdentities(t *testing.T) {
	// export builds the UBL party the way gobl.ubl does, then adds the AE
	// details.
	export := func(p *org.Party) *goblubl.Party {
		out := goblubl.NewParty(p)
		exportAEPartyIdentities(out, p)
		return out
	}
	legal := func(typ cbc.Code, code string) *org.Identity {
		return &org.Identity{Type: typ, Scope: org.IdentityScopeLegal, Code: cbc.Code(code)}
	}
	passport := func(code string) *org.Identity {
		return &org.Identity{Key: org.IdentityKeyPassport, Scope: org.IdentityScopeLegal, Code: cbc.Code(code)}
	}

	t.Run("trade licence gets TL and its authority", func(t *testing.T) {
		id := legal(ae.IdentityTypeTradeLicense, "112345678900003")
		id.Description = "Trade License issuing Authority"
		cid := export(&org.Party{Name: "Seller", Identities: []*org.Identity{id}}).PartyLegalEntity.CompanyID
		assert.Equal(t, "112345678900003", cid.Value)
		assert.Equal(t, "TL", *cid.SchemeAgencyID)
		assert.Equal(t, "Trade License issuing Authority", *cid.SchemeAgencyName)
	})

	t.Run("passport gets PAS and its issuing country", func(t *testing.T) {
		id := passport("P1234567890")
		id.Country = "IN"
		cid := export(&org.Party{Name: "Buyer", Identities: []*org.Identity{id}}).PartyLegalEntity.CompanyID
		assert.Equal(t, "PAS", *cid.SchemeAgencyID)
		assert.Equal(t, "IN", *cid.SchemeAgencyName)
	})

	t.Run("Emirates ID and Cabinet Decision get their type only", func(t *testing.T) {
		for _, typ := range []cbc.Code{ae.IdentityTypeEmiratesID, ae.IdentityTypeCabinetDecision} {
			id := legal(typ, "784199012345671")
			id.Description = "not an authority"
			cid := export(&org.Party{Name: "Party", Identities: []*org.Identity{id}}).PartyLegalEntity.CompanyID
			assert.Equal(t, typ.String(), *cid.SchemeAgencyID, typ)
			assert.Nil(t, cid.SchemeAgencyName, typ)
		}
	})

	t.Run("passport ignores the description", func(t *testing.T) {
		id := passport("P1234567890")
		id.Description = "not a country"
		cid := export(&org.Party{Name: "Buyer", Identities: []*org.Identity{id}}).PartyLegalEntity.CompanyID
		assert.Nil(t, cid.SchemeAgencyName)
	})

	t.Run("legal identity without a PINT AE type is left alone", func(t *testing.T) {
		cid := export(&org.Party{Name: "Party", Identities: []*org.Identity{
			{Scope: org.IdentityScopeLegal, Code: "CN-1234567"},
		}}).PartyLegalEntity.CompanyID
		assert.Nil(t, cid.SchemeAgencyID)
		assert.Nil(t, cid.SchemeAgencyName)
	})

	t.Run("the first legal identity is described, after a tax one", func(t *testing.T) {
		cid := export(&org.Party{Name: "Party", Identities: []*org.Identity{
			{Type: ae.IdentityTypeTIN, Scope: org.IdentityScopeTax, Code: "1234567897"},
			legal(ae.IdentityTypeEmiratesID, "784199012345671"),
			legal(ae.IdentityTypeCabinetDecision, "CD-1"),
		}}).PartyLegalEntity.CompanyID
		assert.Equal(t, "784199012345671", cid.Value)
		assert.Equal(t, "EID", *cid.SchemeAgencyID)
	})

	t.Run("TIN is written under the TIN scheme, next to the VAT TRN", func(t *testing.T) {
		out := export(&org.Party{
			Name:  "Seller",
			TaxID: &tax.Identity{Country: "AE", Code: "100000001600003"},
			Identities: []*org.Identity{
				{Type: ae.IdentityTypeTIN, Scope: org.IdentityScopeTax, Code: "1234567897"},
			},
		})
		require.Len(t, out.PartyTaxScheme, 2)
		assert.Equal(t, "VAT", out.PartyTaxScheme[0].TaxScheme.ID.Value)
		assert.Equal(t, "1234567897", out.PartyTaxScheme[1].CompanyID.Value)
		assert.Equal(t, "TIN", out.PartyTaxScheme[1].TaxScheme.ID.Value)
	})

	t.Run("other tax identities keep the scheme gobl.ubl gives them", func(t *testing.T) {
		out := export(&org.Party{Name: "Seller", Identities: []*org.Identity{
			{Scope: org.IdentityScopeTax, Code: "1234567897"},
		}})
		require.Len(t, out.PartyTaxScheme, 1)
		assert.Equal(t, goblubl.TaxSchemeTaxRegistration, out.PartyTaxScheme[0].TaxScheme.ID.Value)
	})

	t.Run("a TIN identification has the scheme 0235", func(t *testing.T) {
		out := export(&org.Party{Name: "Buyer", Identities: []*org.Identity{
			{Type: ae.IdentityTypeTIN, Code: "1234567897"},
		}})
		require.Len(t, out.PartyIdentification, 1)
		assert.Equal(t, "1234567897", out.PartyIdentification[0].ID.Value)
		assert.Equal(t, "0235", *out.PartyIdentification[0].ID.SchemeID)
	})

	t.Run("nil parties are skipped", func(t *testing.T) {
		assert.NotPanics(t, func() {
			exportAEPartyIdentities(nil, &org.Party{})
			exportAEPartyIdentities(&goblubl.Party{}, nil)
		})
	})
}

func TestImportAEPartyIdentities(t *testing.T) {
	companyID := func(agencyID, agencyName string) *goblubl.Party {
		cid := &goblubl.IDType{Value: "112345678900003"}
		if agencyID != "" {
			cid.SchemeAgencyID = &agencyID
		}
		if agencyName != "" {
			cid.SchemeAgencyName = &agencyName
		}
		return &goblubl.Party{PartyLegalEntity: &goblubl.PartyLegalEntity{CompanyID: cid}}
	}
	// imported builds the GOBL party the way gobl.ubl imports it, then reads
	// the AE details.
	imported := func(in *goblubl.Party, supplier bool) *org.Party {
		p := goblubl.ParseParty(in)
		importAEPartyIdentities(in, p, supplier)
		return p
	}

	t.Run("trade licence with its authority", func(t *testing.T) {
		id := imported(companyID("TL", "Trade License issuing Authority"), false).Identities[0]
		assert.Equal(t, ae.IdentityTypeTradeLicense, id.Type)
		assert.Equal(t, "Trade License issuing Authority", id.Description)
	})

	t.Run("an identification with the scheme 0235 is a TIN", func(t *testing.T) {
		scheme := "0235"
		in := &goblubl.Party{PartyIdentification: []goblubl.Identification{
			{ID: &goblubl.IDType{Value: "1234567897", SchemeID: &scheme}},
		}}
		id := imported(in, false).Identities[0]
		assert.Equal(t, ae.IdentityTypeTIN, id.Type)
		assert.False(t, id.Ext.Has(iso.ExtKeySchemeID))
	})

	t.Run("passport with its issuing country", func(t *testing.T) {
		id := imported(companyID("PAS", "IN"), false).Identities[0]
		assert.Equal(t, org.IdentityKeyPassport, id.Key)
		assert.Equal(t, l10n.ISOCountryCode("IN"), id.Country)
		assert.Empty(t, id.Description)
	})

	t.Run("Emirates ID and Cabinet Decision, without an authority", func(t *testing.T) {
		for _, typ := range []cbc.Code{ae.IdentityTypeEmiratesID, ae.IdentityTypeCabinetDecision} {
			id := imported(companyID(typ.String(), "Some Authority"), false).Identities[0]
			assert.Equal(t, typ, id.Type, typ)
			assert.Empty(t, id.Description, typ)
		}
	})

	t.Run("unknown or missing types are left alone", func(t *testing.T) {
		for _, typ := range []string{"XYZ", "CL", ""} {
			id := imported(companyID(typ, "Some Authority"), false).Identities[0]
			assert.Empty(t, id.Key, typ)
			assert.Empty(t, id.Type, typ)
			assert.Empty(t, id.Description, typ)
		}
	})

	t.Run("supplier non-VAT tax registration is the TIN", func(t *testing.T) {
		in := companyID("TL", "Authority")
		in.PartyTaxScheme = []goblubl.PartyTaxScheme{
			{CompanyID: &goblubl.IDType{Value: "198765432200003"}, TaxScheme: &goblubl.TaxScheme{ID: goblubl.IDType{Value: "VAT"}}},
			{CompanyID: &goblubl.IDType{Value: "1234567897"}, TaxScheme: &goblubl.TaxScheme{ID: goblubl.IDType{Value: "TIN"}}},
		}
		p := imported(in, true)
		require.Len(t, p.Identities, 2)
		assert.Equal(t, ae.IdentityTypeTIN, p.Identities[1].Type)
		assert.Equal(t, cbc.Code("1234567897"), p.Identities[1].Code)

		p = imported(in, false)
		assert.Empty(t, p.Identities[1].Type, "only the seller has a TIN")

		// Any scheme other than VAT, like the official example's TAX.
		in.PartyTaxScheme[1].TaxScheme.ID.Value = "TAX"
		p = imported(in, true)
		assert.Equal(t, ae.IdentityTypeTIN, p.Identities[1].Type)
	})

	t.Run("nil parties are skipped", func(t *testing.T) {
		assert.NotPanics(t, func() {
			importAEPartyIdentities(nil, &org.Party{}, true)
			importAEPartyIdentities(&goblubl.Party{}, nil, true)
		})
	})
}

func TestImportAEPartyEndpoint(t *testing.T) {
	t.Run("the electronic address replaces the inbox gobl.ubl imports", func(t *testing.T) {
		in := &goblubl.Party{EndpointID: &goblubl.EndpointID{SchemeID: "0235", Value: "1357902467"}}
		p := &org.Party{Inboxes: []*org.Inbox{
			{Scheme: "0235", Code: "1357902467"},
			{Key: "other", Code: "ABC"},
		}}
		importAEPartyEndpoint(in, p)
		require.Len(t, p.Endpoints, 1)
		assert.Equal(t, cbc.URI("iso6523-actorid-upis::0235:1357902467"), p.Endpoints[0].URI)
		require.Len(t, p.Inboxes, 1)
		assert.Equal(t, cbc.Code("ABC"), p.Inboxes[0].Code)
	})

	t.Run("an email address is read as a mailto endpoint", func(t *testing.T) {
		in := &goblubl.Party{EndpointID: &goblubl.EndpointID{SchemeID: goblubl.SchemeIDEmail, Value: "billing@example.ae"}}
		p := &org.Party{Inboxes: []*org.Inbox{{Email: "billing@example.ae"}}}
		importAEPartyEndpoint(in, p)
		require.Len(t, p.Endpoints, 1)
		assert.Equal(t, cbc.URI("mailto:billing@example.ae"), p.Endpoints[0].URI)
		assert.Empty(t, p.Inboxes)
	})

	t.Run("parties without an electronic address are left alone", func(t *testing.T) {
		p := &org.Party{}
		importAEPartyEndpoint(&goblubl.Party{}, p)
		assert.Empty(t, p.Endpoints)
		assert.NotPanics(t, func() {
			importAEPartyEndpoint(nil, p)
			importAEPartyEndpoint(&goblubl.Party{}, nil)
		})
	})
}

func TestImportAEMarginScheme(t *testing.T) {
	combo := func(category cbc.Code) *tax.Combo {
		five := num.MakePercentage(5, 2)
		return &tax.Combo{
			Category: tax.CategoryVAT,
			Percent:  &five,
			Ext:      tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: category}),
		}
	}
	inv := &bill.Invoice{
		Lines:     []*bill.Line{{Taxes: tax.Set{combo("N")}}, {Taxes: tax.Set{combo("S")}}},
		Discounts: []*bill.Discount{{Taxes: tax.Set{combo("N")}}},
	}
	importAEMarginScheme(inv)

	ms := inv.Lines[0].Taxes[0]
	assert.Equal(t, tax.KeyExempt, ms.Key)
	assert.Nil(t, ms.Percent, "margin scheme lines charge no VAT")
	assert.Equal(t, tax.KeyExempt, inv.Discounts[0].Taxes[0].Key)
	assert.True(t, inv.HasTags(pint.TagMarginScheme), "the tag keeps N on the lines")

	other := inv.Lines[1].Taxes[0]
	assert.Empty(t, other.Key, "other categories are left to EN 16931")
	assert.NotNil(t, other.Percent)

	inv = &bill.Invoice{Lines: []*bill.Line{{Taxes: tax.Set{combo("S")}}}}
	importAEMarginScheme(inv)
	assert.False(t, inv.HasTags(pint.TagMarginScheme))
}

func TestImportAEOutsideScope(t *testing.T) {
	importCode := func(invoiceCode, creditNoteCode string) *bill.Invoice {
		in := new(goblubl.Invoice)
		if invoiceCode != "" {
			in.InvoiceTypeCode = &goblubl.IDType{Value: invoiceCode}
		}
		if creditNoteCode != "" {
			in.CreditNoteTypeCode = &goblubl.IDType{Value: creditNoteCode}
		}
		inv := &bill.Invoice{Type: bill.InvoiceTypeOther}
		importAEOutsideTaxScope(in, inv)
		return inv
	}

	inv := importCode("480", "")
	assert.Equal(t, bill.InvoiceTypeStandard, inv.Type)
	assert.True(t, inv.HasTags(pint.TagOutsideTaxScope))

	inv = importCode("", "81")
	assert.Equal(t, bill.InvoiceTypeCreditNote, inv.Type)
	assert.True(t, inv.HasTags(pint.TagOutsideTaxScope))

	inv = importCode("380", "")
	assert.Equal(t, bill.InvoiceTypeOther, inv.Type, "other codes are left to gobl.ubl")
	assert.False(t, inv.HasTags(pint.TagOutsideTaxScope))

	t.Run("does not add the tag twice", func(t *testing.T) {
		in := &goblubl.Invoice{InvoiceTypeCode: &goblubl.IDType{Value: "480"}}
		inv := &bill.Invoice{Tags: tax.WithTags(pint.TagOutsideTaxScope)}
		importAEOutsideTaxScope(in, inv)
		assert.Equal(t, []cbc.Key{pint.TagOutsideTaxScope}, inv.Tags.List)
	})
}

func TestExportAETransactionType(t *testing.T) {
	invoice := func(ext cbc.CodeMap) *bill.Invoice {
		return &bill.Invoice{Tax: &bill.Tax{Ext: tax.ExtensionsOf(ext)}}
	}

	t.Run("writes the code and the beneficiary and principal IDs", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAETransactionType(out, invoice(cbc.CodeMap{
			pint.ExtKeyTransactionType: "10000100",
			pint.ExtKeyBeneficiaryID:   "189098765401003",
			pint.ExtKeyPrincipalID:     "155667745601003",
		}))
		assert.Equal(t, "10000100", out.ProfileExecutionID)
		assert.Equal(t, "189098765401003", out.BuyerCustomerParty.Party.PartyIdentification[0].ID.Value)
		assert.Equal(t, "155667745601003", out.SellerSupplierParty.Party.PartyIdentification[0].ID.Value)
	})

	t.Run("writes no parties without IDs", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAETransactionType(out, invoice(cbc.CodeMap{pint.ExtKeyTransactionType: "00000000"}))
		assert.Equal(t, "00000000", out.ProfileExecutionID)
		assert.Nil(t, out.BuyerCustomerParty)
		assert.Nil(t, out.SellerSupplierParty)
	})

	t.Run("invoices without tax details are skipped", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAETransactionType(out, new(bill.Invoice))
		assert.Empty(t, out.ProfileExecutionID)
	})
}

func TestImportAETransactionType(t *testing.T) {
	t.Run("reads the code as extension and tags, with the IDs", func(t *testing.T) {
		in := &goblubl.Invoice{
			ProfileExecutionID:  " 10000101 ",
			BuyerCustomerParty:  &goblubl.CustomerParty{Party: identifiedParty("189098765401003")},
			SellerSupplierParty: &goblubl.SupplierParty{Party: identifiedParty("155667745601003")},
		}
		inv := new(bill.Invoice)
		importAETransactionType(in, inv)
		assert.Equal(t, cbc.Code("10000101"), inv.Tax.GetExt(pint.ExtKeyTransactionType))
		assert.Equal(t, cbc.Code("189098765401003"), inv.Tax.GetExt(pint.ExtKeyBeneficiaryID))
		assert.Equal(t, cbc.Code("155667745601003"), inv.Tax.GetExt(pint.ExtKeyPrincipalID))
		assert.Equal(t, []cbc.Key{pint.TagFreeTradeZone, pint.TagDisclosedAgent, tax.TagExport}, inv.Tags.List)
	})

	t.Run("existing tags are not repeated", func(t *testing.T) {
		inv := &bill.Invoice{Tags: tax.WithTags(tax.TagExport)}
		importAETransactionType(&goblubl.Invoice{ProfileExecutionID: "00000001"}, inv)
		assert.Equal(t, []cbc.Key{tax.TagExport}, inv.Tags.List)
	})

	t.Run("nothing to read leaves the invoice alone", func(t *testing.T) {
		inv := new(bill.Invoice)
		importAETransactionType(&goblubl.Invoice{BuyerCustomerParty: &goblubl.CustomerParty{}}, inv)
		assert.Nil(t, inv.Tax)
		assert.Empty(t, inv.Tags.List)
	})
}

func TestAECreditNoteReason(t *testing.T) {
	invoice := func(reason cbc.Code) *bill.Invoice {
		return &bill.Invoice{Tax: &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyCreditNoteReason: reason})}}
	}
	creditNote := func() *goblubl.Invoice {
		return &goblubl.Invoice{CreditNoteTypeCode: &goblubl.IDType{Value: "381"}}
	}

	t.Run("writes the reason on credit notes", func(t *testing.T) {
		out := creditNote()
		exportAECreditNoteReason(out, invoice("VD"))
		require.Len(t, out.DiscrepancyResponse, 1)
		assert.Equal(t, "VD", out.DiscrepancyResponse[0].ResponseCode.Value)
	})

	t.Run("never on invoices, which UBL doesn't allow it on", func(t *testing.T) {
		out := &goblubl.Invoice{InvoiceTypeCode: &goblubl.IDType{Value: "380"}}
		exportAECreditNoteReason(out, invoice("VD"))
		assert.Empty(t, out.DiscrepancyResponse)
	})

	t.Run("nothing without a reason", func(t *testing.T) {
		out := creditNote()
		exportAECreditNoteReason(out, new(bill.Invoice))
		assert.Empty(t, out.DiscrepancyResponse)
	})

	t.Run("reads the first reason back", func(t *testing.T) {
		in := creditNote()
		in.DiscrepancyResponse = []goblubl.Response{
			{},
			{ResponseCode: &goblubl.IDType{Value: " VD "}},
			{ResponseCode: &goblubl.IDType{Value: "DL8.61.1.E"}},
		}
		inv := new(bill.Invoice)
		importAECreditNoteReason(in, inv)
		assert.Equal(t, cbc.Code("VD"), inv.Tax.GetExt(pint.ExtKeyCreditNoteReason))
	})
}

func TestAEBillingFrequency(t *testing.T) {
	invoice := func(freq cbc.Code) *bill.Invoice {
		return &bill.Invoice{Tax: &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyBillingFrequency: freq})}}
	}

	t.Run("writes the frequency on the ordering period", func(t *testing.T) {
		out := &goblubl.Invoice{InvoicePeriod: []goblubl.Period{{StartDate: "2025-01-01", EndDate: "2025-01-31"}}}
		exportAEBillingFrequency(out, invoice("MTH"))
		require.Len(t, out.InvoicePeriod, 1)
		assert.Equal(t, "MTH", out.InvoicePeriod[0].DescriptionCode)
		assert.Equal(t, "2025-01-01", out.InvoicePeriod[0].StartDate)
	})

	t.Run("writes a period of its own without an ordering period", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEBillingFrequency(out, invoice("QTR"))
		require.Len(t, out.InvoicePeriod, 1)
		assert.Equal(t, "QTR", out.InvoicePeriod[0].DescriptionCode)
	})

	t.Run("nothing without a frequency", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEBillingFrequency(out, new(bill.Invoice))
		assert.Empty(t, out.InvoicePeriod)
	})

	t.Run("reads the frequency back", func(t *testing.T) {
		in := &goblubl.Invoice{InvoicePeriod: []goblubl.Period{{DescriptionCode: " OTH "}}}
		inv := new(bill.Invoice)
		importAEBillingFrequency(in, inv)
		assert.Equal(t, cbc.Code("OTH"), inv.Tax.GetExt(pint.ExtKeyBillingFrequency))

		inv = new(bill.Invoice)
		importAEBillingFrequency(new(goblubl.Invoice), inv)
		assert.Nil(t, inv.Tax)
	})
}

func TestAELineExemption(t *testing.T) {
	ublLine := func() goblubl.InvoiceLine {
		return goblubl.InvoiceLine{Item: &goblubl.Item{ClassifiedTaxCategory: &goblubl.ClassifiedTaxCategory{
			ID: &goblubl.IDType{Value: "E"},
		}}}
	}
	goblLine := func(code cbc.Code) *bill.Line {
		c := &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyExempt}
		if code != "" {
			c.Ext = tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyExemption: code})
		}
		return &bill.Line{Taxes: tax.Set{c}}
	}

	t.Run("writes the exemption reason on the line", func(t *testing.T) {
		line := ublLine()
		exportAELineExemption(&line, goblLine("DL8.46.2"), nil)
		require.NotNil(t, line.Item.ClassifiedTaxCategory.TaxExemptionReasonCode)
		assert.Equal(t, "DL8.46.2", *line.Item.ClassifiedTaxCategory.TaxExemptionReasonCode)
	})

	t.Run("nothing without a reason", func(t *testing.T) {
		line := ublLine()
		exportAELineExemption(&line, goblLine(""), nil)
		assert.Nil(t, line.Item.ClassifiedTaxCategory.TaxExemptionReasonCode)
		exportAELineExemption(&line, nil, nil)
		assert.Nil(t, line.Item.ClassifiedTaxCategory.TaxExemptionReasonCode)
	})

	t.Run("reads the reasons back onto the line combos", func(t *testing.T) {
		code := " DL8.46.4 "
		with := ublLine()
		with.Item.ClassifiedTaxCategory.TaxExemptionReasonCode = &code
		inv := &bill.Invoice{Lines: []*bill.Line{goblLine(""), goblLine("")}}
		importAELineExemptions([]goblubl.InvoiceLine{ublLine(), with}, inv)
		assert.Empty(t, inv.Lines[0].Taxes[0].Ext.Get(pint.ExtKeyExemption))
		assert.Equal(t, cbc.Code("DL8.46.4"), inv.Lines[1].Taxes[0].Ext.Get(pint.ExtKeyExemption))
	})

	t.Run("lines that don't match are skipped", func(t *testing.T) {
		code := "DL8.46.4"
		with := ublLine()
		with.Item.ClassifiedTaxCategory.TaxExemptionReasonCode = &code
		inv := &bill.Invoice{Lines: []*bill.Line{goblLine(""), goblLine("")}}
		importAELineExemptions([]goblubl.InvoiceLine{with}, inv)
		assert.Empty(t, inv.Lines[0].Taxes[0].Ext.Get(pint.ExtKeyExemption))
	})
}

func TestAELineGoods(t *testing.T) {
	gtin := func(scheme cbc.Code, code cbc.Code) *org.Identity {
		return &org.Identity{Code: code, Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: scheme})}
	}
	goblLine := func(goodsType cbc.Code, ids ...*org.Identity) *bill.Line {
		it := &org.Item{Name: "Gold bars", Identities: ids}
		if goodsType != "" {
			it.Ext = tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyGoodsType: goodsType})
		}
		return &bill.Line{Item: it}
	}

	t.Run("writes the goods type as the nature code", func(t *testing.T) {
		line := goblubl.InvoiceLine{Item: &goblubl.Item{}}
		exportAELineGoods(&line, goblLine("DL8.48.8.1"))
		require.NotNil(t, line.Item.CommodityClassification)
		require.Len(t, *line.Item.CommodityClassification, 1)
		assert.Equal(t, "DL8.48.8.1", *(*line.Item.CommodityClassification)[0].NatureCode)
	})

	t.Run("on the first classification when there is one", func(t *testing.T) {
		list := "HS"
		line := goblubl.InvoiceLine{Item: &goblubl.Item{CommodityClassification: &[]goblubl.CommodityClassification{
			{ItemClassificationCode: &goblubl.IDType{Value: "7108", ListID: &list}},
		}}}
		exportAELineGoods(&line, goblLine("DL8.48.8.1"))
		require.Len(t, *line.Item.CommodityClassification, 1)
		cc := (*line.Item.CommodityClassification)[0]
		assert.Equal(t, "DL8.48.8.1", *cc.NatureCode)
		assert.Equal(t, "7108", cc.ItemClassificationCode.Value)
	})

	t.Run("writes the GTIN as the standard identifier", func(t *testing.T) {
		other := "0088"
		line := goblubl.InvoiceLine{Item: &goblubl.Item{StandardItemIdentification: &goblubl.ItemIdentification{
			ID: &goblubl.IDType{SchemeID: &other, Value: "5410000000000"},
		}}}
		exportAELineGoods(&line, goblLine("", gtin("0088", "5410000000000"), gtin("0160", "4006381333931")))
		id := line.Item.StandardItemIdentification.ID
		assert.Equal(t, "0160", *id.SchemeID)
		assert.Equal(t, "4006381333931", id.Value)
	})

	t.Run("nothing without a goods type or GTIN", func(t *testing.T) {
		line := goblubl.InvoiceLine{Item: &goblubl.Item{}}
		exportAELineGoods(&line, goblLine(""))
		assert.Nil(t, line.Item.CommodityClassification)
		assert.Nil(t, line.Item.StandardItemIdentification)
		exportAELineGoods(&line, nil)
		assert.Nil(t, line.Item.CommodityClassification)
	})

	t.Run("reads the goods type back onto the items", func(t *testing.T) {
		code := " DL8.48.3.2 "
		with := goblubl.InvoiceLine{Item: &goblubl.Item{CommodityClassification: &[]goblubl.CommodityClassification{{NatureCode: &code}}}}
		without := goblubl.InvoiceLine{Item: &goblubl.Item{}}
		inv := &bill.Invoice{Lines: []*bill.Line{goblLine(""), goblLine("")}}
		importAELineGoods([]goblubl.InvoiceLine{without, with}, inv)
		assert.Empty(t, inv.Lines[0].Item.Ext.Get(pint.ExtKeyGoodsType))
		assert.Equal(t, cbc.Code("DL8.48.3.2"), inv.Lines[1].Item.Ext.Get(pint.ExtKeyGoodsType))
	})

	t.Run("lines that don't match are skipped", func(t *testing.T) {
		code := "DL8.48.3.2"
		with := goblubl.InvoiceLine{Item: &goblubl.Item{CommodityClassification: &[]goblubl.CommodityClassification{{NatureCode: &code}}}}
		inv := &bill.Invoice{Lines: []*bill.Line{goblLine(""), goblLine("")}}
		importAELineGoods([]goblubl.InvoiceLine{with}, inv)
		assert.Empty(t, inv.Lines[0].Item.Ext.Get(pint.ExtKeyGoodsType))
	})
}

func TestAELineItems(t *testing.T) {
	list := func(v string) *string { return &v }
	classification := func(listID, code string) goblubl.CommodityClassification {
		return goblubl.CommodityClassification{ItemClassificationCode: &goblubl.IDType{Value: code, ListID: list(listID)}}
	}

	t.Run("moves the service accounting codes to additional identifiers", func(t *testing.T) {
		line := goblubl.InvoiceLine{Item: &goblubl.Item{CommodityClassification: &[]goblubl.CommodityClassification{
			classification("HS", "8471"), classification("SAC", "998713"),
		}}}
		exportAELineSAC(&line)
		require.Len(t, *line.Item.CommodityClassification, 1)
		assert.Equal(t, "8471", (*line.Item.CommodityClassification)[0].ItemClassificationCode.Value)
		require.Len(t, line.Item.AdditionalItemIdentification, 1)
		id := line.Item.AdditionalItemIdentification[0].ID
		assert.Equal(t, "SAC", *id.SchemeID)
		assert.Equal(t, "998713", id.Value)
	})

	t.Run("no classifications left when all were SAC", func(t *testing.T) {
		line := goblubl.InvoiceLine{Item: &goblubl.Item{CommodityClassification: &[]goblubl.CommodityClassification{
			classification("SAC", "998713"),
		}}}
		exportAELineSAC(&line)
		assert.Nil(t, line.Item.CommodityClassification)
		assert.Len(t, line.Item.AdditionalItemIdentification, 1)
	})

	t.Run("writes the item type on the first classification", func(t *testing.T) {
		line := goblubl.InvoiceLine{Item: &goblubl.Item{CommodityClassification: &[]goblubl.CommodityClassification{
			classification("HS", "8471"),
		}}}
		exportAELineItemType(&line, &bill.Line{Item: &org.Item{Key: pint.ItemKeyGoodsServices}})
		cc := (*line.Item.CommodityClassification)[0]
		assert.Equal(t, "B", *cc.CommodityCode)
		assert.Equal(t, "8471", cc.ItemClassificationCode.Value)
	})

	t.Run("nothing for items without an AE type", func(t *testing.T) {
		line := goblubl.InvoiceLine{Item: &goblubl.Item{}}
		exportAELineItemType(&line, &bill.Line{Item: &org.Item{Key: "other"}})
		assert.Nil(t, line.Item.CommodityClassification)
		exportAELineItemType(&line, nil)
		assert.Nil(t, line.Item.CommodityClassification)
	})

	t.Run("reads the item type and service accounting codes back", func(t *testing.T) {
		with := goblubl.InvoiceLine{Item: &goblubl.Item{
			AdditionalItemIdentification: []goblubl.ItemIdentification{{ID: &goblubl.IDType{SchemeID: list("SAC"), Value: " 998713 "}}},
			CommodityClassification:      &[]goblubl.CommodityClassification{{CommodityCode: list("S")}},
		}}
		inv := &bill.Invoice{Lines: []*bill.Line{{Item: &org.Item{}}, {Item: &org.Item{}}}}
		importAELineItems([]goblubl.InvoiceLine{{Item: &goblubl.Item{}}, with}, inv)
		assert.Empty(t, inv.Lines[0].Item.Key)
		assert.Empty(t, inv.Lines[0].Item.Identities)
		assert.Equal(t, org.ItemKeyServices, inv.Lines[1].Item.Key)
		require.Len(t, inv.Lines[1].Item.Identities, 1)
		assert.Equal(t, "SAC", inv.Lines[1].Item.Identities[0].Label)
		assert.Equal(t, cbc.Code("998713"), inv.Lines[1].Item.Identities[0].Code)
	})

	t.Run("lines that don't match are skipped", func(t *testing.T) {
		with := goblubl.InvoiceLine{Item: &goblubl.Item{CommodityClassification: &[]goblubl.CommodityClassification{{CommodityCode: list("G")}}}}
		inv := &bill.Invoice{Lines: []*bill.Line{{Item: &org.Item{}}, {Item: &org.Item{}}}}
		importAELineItems([]goblubl.InvoiceLine{with}, inv)
		assert.Empty(t, inv.Lines[0].Item.Key)
	})
}

func TestAEDeliveryPartyID(t *testing.T) {
	delivery := func(r *org.Party) *bill.Invoice {
		return &bill.Invoice{Delivery: &bill.DeliveryDetails{Receiver: r}}
	}
	partyID := func(out *goblubl.Invoice) string {
		require.Len(t, out.Delivery, 1)
		require.NotNil(t, out.Delivery[0].DeliveryParty)
		require.Len(t, out.Delivery[0].DeliveryParty.PartyIdentification, 1)
		return out.Delivery[0].DeliveryParty.PartyIdentification[0].ID.Value
	}

	t.Run("writes the receiver's TRN", func(t *testing.T) {
		out := &goblubl.Invoice{Delivery: []*goblubl.Delivery{{DeliveryParty: &goblubl.Party{PartyName: &goblubl.PartyName{Name: "Warehouse"}}}}}
		exportAEDeliveryPartyID(out, delivery(&org.Party{
			Name:       "Warehouse",
			TaxID:      &tax.Identity{Country: "AE", Code: "155667745601003"},
			Identities: []*org.Identity{{Type: ae.IdentityTypeTIN, Code: "1345678906"}},
		}))
		assert.Equal(t, "155667745601003", partyID(out))
		assert.Equal(t, "Warehouse", out.Delivery[0].DeliveryParty.PartyName.Name)
	})

	t.Run("or its TIN without a TRN", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEDeliveryPartyID(out, delivery(&org.Party{
			Identities: []*org.Identity{{Type: ae.IdentityTypeTIN, Code: "1345678906"}},
		}))
		assert.Equal(t, "1345678906", partyID(out))
	})

	t.Run("nothing without a UAE tax ID or TIN", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEDeliveryPartyID(out, delivery(&org.Party{Name: "Warehouse", TaxID: &tax.Identity{Country: "GB", Code: "123456789"}}))
		exportAEDeliveryPartyID(out, delivery(nil))
		exportAEDeliveryPartyID(out, &bill.Invoice{})
		assert.Empty(t, out.Delivery)
	})

	t.Run("reads a TRN back as the receiver's tax ID", func(t *testing.T) {
		in := &goblubl.Invoice{Delivery: []*goblubl.Delivery{{DeliveryParty: identifiedParty(" 155667745601003 ")}}}
		inv := delivery(&org.Party{Name: "Warehouse"})
		importAEDeliveryPartyID(in, inv)
		r := inv.Delivery.Receiver
		assert.Equal(t, "Warehouse", r.Name)
		require.NotNil(t, r.TaxID)
		assert.Equal(t, l10n.TaxCountryCode("AE"), r.TaxID.Country)
		assert.Equal(t, cbc.Code("155667745601003"), r.TaxID.Code)
		assert.Empty(t, r.Identities)
	})

	t.Run("and anything else as a TIN, on a new receiver", func(t *testing.T) {
		in := &goblubl.Invoice{Delivery: []*goblubl.Delivery{{DeliveryParty: identifiedParty("1345678906")}}}
		inv := new(bill.Invoice)
		importAEDeliveryPartyID(in, inv)
		require.NotNil(t, inv.Delivery)
		r := inv.Delivery.Receiver
		require.NotNil(t, r)
		assert.Nil(t, r.TaxID)
		require.Len(t, r.Identities, 1)
		assert.Equal(t, ae.IdentityTypeTIN, r.Identities[0].Type)
		assert.Equal(t, cbc.Code("1345678906"), r.Identities[0].Code)
	})

	t.Run("nothing to read without an ID", func(t *testing.T) {
		inv := new(bill.Invoice)
		importAEDeliveryPartyID(&goblubl.Invoice{Delivery: []*goblubl.Delivery{{DeliveryParty: &goblubl.Party{}}}}, inv)
		importAEDeliveryPartyID(new(goblubl.Invoice), inv)
		assert.Nil(t, inv.Delivery)
	})
}

func TestAEIncoterms(t *testing.T) {
	delivery := func(key cbc.Key, code cbc.Code) *bill.Invoice {
		return &bill.Invoice{Delivery: &bill.DeliveryDetails{Ext: tax.ExtensionsOf(cbc.CodeMap{key: code})}}
	}

	t.Run("writes the Incoterms as the delivery terms", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEIncoterms(out, delivery(icc.ExtKeyIncoterms2020, "CIF"))
		require.Len(t, out.Delivery, 1)
		require.Len(t, out.Delivery[0].DeliveryTerms, 1)
		id := out.Delivery[0].DeliveryTerms[0].ID
		assert.Equal(t, "CIF", id.Value)
		require.NotNil(t, id.SchemeID)
		assert.Equal(t, "Incoterms", *id.SchemeID)
	})

	t.Run("also from the 2010 edition", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEIncoterms(out, delivery(icc.ExtKeyIncoterms2010, "DAT"))
		assert.Equal(t, "DAT", out.Delivery[0].DeliveryTerms[0].ID.Value)
	})

	t.Run("on the delivery gobl.ubl wrote", func(t *testing.T) {
		out := &goblubl.Invoice{Delivery: []*goblubl.Delivery{{DeliveryLocation: &goblubl.Location{ID: &goblubl.IDType{Value: "5790000436101"}}}}}
		exportAEIncoterms(out, delivery(icc.ExtKeyIncoterms2020, "CIF"))
		require.Len(t, out.Delivery, 1)
		assert.Equal(t, "CIF", out.Delivery[0].DeliveryTerms[0].ID.Value)
		assert.Equal(t, "5790000436101", out.Delivery[0].DeliveryLocation.ID.Value)
	})

	t.Run("nothing without Incoterms", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEIncoterms(out, &bill.Invoice{Delivery: &bill.DeliveryDetails{}})
		assert.Empty(t, out.Delivery)
		exportAEIncoterms(out, &bill.Invoice{})
		assert.Empty(t, out.Delivery)
	})

	t.Run("reads the Incoterms back as the 2020 edition", func(t *testing.T) {
		in := &goblubl.Invoice{Delivery: []*goblubl.Delivery{{DeliveryTerms: []goblubl.DeliveryTerms{{ID: goblubl.IDType{Value: " CIF "}}}}}}
		inv := new(bill.Invoice)
		importAEIncoterms(in, inv)
		require.NotNil(t, inv.Delivery)
		assert.Equal(t, cbc.Code("CIF"), inv.Delivery.Ext.Get(icc.ExtKeyIncoterms2020))

		// DAT is only in the 2010 edition.
		in = &goblubl.Invoice{Delivery: []*goblubl.Delivery{{DeliveryTerms: []goblubl.DeliveryTerms{{ID: goblubl.IDType{Value: "DAT"}}}}}}
		inv = new(bill.Invoice)
		importAEIncoterms(in, inv)
		assert.Equal(t, cbc.Code("DAT"), inv.Delivery.Ext.Get(icc.ExtKeyIncoterms2010))
		assert.Empty(t, inv.Delivery.Ext.Get(icc.ExtKeyIncoterms2020))

		// A code in neither edition is kept under the current one.
		in = &goblubl.Invoice{Delivery: []*goblubl.Delivery{{DeliveryTerms: []goblubl.DeliveryTerms{{ID: goblubl.IDType{Value: "XYZ"}}}}}}
		inv = new(bill.Invoice)
		importAEIncoterms(in, inv)
		assert.Equal(t, cbc.Code("XYZ"), inv.Delivery.Ext.Get(icc.ExtKeyIncoterms2020))

		inv = new(bill.Invoice)
		importAEIncoterms(new(goblubl.Invoice), inv)
		assert.Nil(t, inv.Delivery)
	})
}

func TestExportAELineAmounts(t *testing.T) {
	line := func(category string) *goblubl.InvoiceLine {
		return &goblubl.InvoiceLine{Item: &goblubl.Item{
			ClassifiedTaxCategory: &goblubl.ClassifiedTaxCategory{ID: &goblubl.IDType{Value: category}},
		}}
	}
	goblLine := func(total string, key cbc.Key, percent *num.Percentage) *bill.Line {
		a, err := num.AmountFromString(total)
		require.NoError(t, err)
		return &bill.Line{Total: &a, Taxes: tax.Set{{Category: tax.CategoryVAT, Key: key, Percent: percent}}}
	}

	t.Run("standard rated line gets its VAT and the total with VAT", func(t *testing.T) {
		out := line("S")
		exportAELineAmounts(out, goblLine("10486.00", tax.KeyStandard, num.NewPercentage(5, 2)), currency.AED, nil)
		ext := out.ItemPriceExtension
		require.NotNil(t, ext)
		assert.Equal(t, "11010.30", ext.Amount.Value)
		assert.Equal(t, "AED", *ext.Amount.CurrencyID)
		require.Len(t, ext.TaxTotal, 1)
		assert.Equal(t, "524.30", ext.TaxTotal[0].TaxAmount.Value)
	})

	t.Run("line VAT is rounded to the currency", func(t *testing.T) {
		out := line("S")
		exportAELineAmounts(out, goblLine("99.99", tax.KeyStandard, num.NewPercentage(5, 2)), currency.USD, nil)
		assert.Equal(t, "5.00", out.ItemPriceExtension.TaxTotal[0].TaxAmount.Value)
		assert.Equal(t, "104.99", out.ItemPriceExtension.Amount.Value)
		assert.Equal(t, "USD", *out.ItemPriceExtension.Amount.CurrencyID, "no rate, so the document currency")
	})

	t.Run("other currencies are converted to AED", func(t *testing.T) {
		out := line("S")
		rate := &currency.ExchangeRate{From: currency.USD, To: currency.AED, Amount: num.MakeAmount(367285, 5)}
		exportAELineAmounts(out, goblLine("99.99", tax.KeyStandard, num.NewPercentage(5, 2)), currency.USD, rate)
		ext := out.ItemPriceExtension
		assert.Equal(t, "AED", *ext.Amount.CurrencyID)
		assert.Equal(t, "AED", *ext.TaxTotal[0].TaxAmount.CurrencyID)
		assert.Equal(t, "18.36", ext.TaxTotal[0].TaxAmount.Value, "5.00 USD")
		assert.Equal(t, "385.61", ext.Amount.Value, "99.99 USD is 367.25 AED, plus the VAT")
	})

	t.Run("zero rated and reverse charge lines get a zero VAT amount", func(t *testing.T) {
		for _, c := range []struct {
			category string
			key      cbc.Key
		}{{"Z", tax.KeyZero}, {"AE", tax.KeyReverseCharge}} {
			out := line(c.category)
			exportAELineAmounts(out, goblLine("200.00", c.key, nil), currency.AED, nil)
			ext := out.ItemPriceExtension
			assert.Equal(t, "200.00", ext.Amount.Value, c.category)
			require.Len(t, ext.TaxTotal, 1, c.category)
			assert.Equal(t, "0.00", ext.TaxTotal[0].TaxAmount.Value, c.category)
		}
	})

	t.Run("exempt lines get no VAT amount", func(t *testing.T) {
		out := line("E")
		exportAELineAmounts(out, goblLine("120.00", tax.KeyExempt, nil), currency.AED, nil)
		assert.Equal(t, "120.00", out.ItemPriceExtension.Amount.Value)
		assert.Empty(t, out.ItemPriceExtension.TaxTotal)
	})

	t.Run("lines without a total are left alone", func(t *testing.T) {
		out := line("S")
		exportAELineAmounts(out, &bill.Line{}, currency.AED, nil)
		assert.Nil(t, out.ItemPriceExtension)
	})
}

func TestExportAEExchangeRate(t *testing.T) {
	invoice := func(cur currency.Code, rates ...*currency.ExchangeRate) *bill.Invoice {
		return &bill.Invoice{
			Currency:      cur,
			ExchangeRates: rates,
			Totals:        &bill.Totals{TotalWithTax: num.MakeAmount(24875000, 2)},
		}
	}

	t.Run("writes the rate and the AED total with VAT", func(t *testing.T) {
		out := new(goblubl.Invoice)
		inv := invoice(currency.USD, &currency.ExchangeRate{From: currency.USD, To: currency.AED, Amount: num.MakeAmount(367285, 5)})
		exportAEExchangeRate(out, inv)
		require.NotNil(t, out.TaxExchangeRate)
		assert.Equal(t, "USD", *out.TaxExchangeRate.SourceCurrencyCode)
		assert.Equal(t, "AED", *out.TaxExchangeRate.TargetCurrencyCode)
		assert.Equal(t, "3.67285", *out.TaxExchangeRate.CalculationRate)
		require.Len(t, out.AdditionalDocumentReference, 1)
		ref := out.AdditionalDocumentReference[0]
		assert.Equal(t, "AED", ref.ID.Value)
		assert.Equal(t, "aedtotal-incl-vat", ref.DocumentTypeCode)
		assert.Equal(t, "913621.44", ref.DocumentDescription, "the Exports.xml example: 248750 USD at 3.67285")
	})

	t.Run("AED invoices need neither", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEExchangeRate(out, invoice(currency.AED))
		assert.Nil(t, out.TaxExchangeRate)
		assert.Empty(t, out.AdditionalDocumentReference)
	})

	t.Run("invoices without a rate to AED are left to the PINT addon", func(t *testing.T) {
		out := new(goblubl.Invoice)
		inv := invoice(currency.USD, &currency.ExchangeRate{From: currency.USD, To: currency.EUR, Amount: num.MakeAmount(92, 2)})
		exportAEExchangeRate(out, inv)
		assert.Nil(t, out.TaxExchangeRate)
		assert.Empty(t, out.AdditionalDocumentReference)
	})
}

func TestAEJoinNotes(t *testing.T) {
	assert.Nil(t, aeJoinNotes(nil))
	assert.Equal(t, []string{"First"}, aeJoinNotes([]string{"First"}))
	assert.Equal(t, []string{"First\n\nSecond"}, aeJoinNotes([]string{"First", "Second"}))
}

func TestExportAEIssueTime(t *testing.T) {
	out := new(goblubl.Invoice)
	exportAEIssueTime(out, &bill.Invoice{})
	assert.Empty(t, out.IssueTime)

	exportAEIssueTime(out, &bill.Invoice{IssueTime: cal.NewTime(17, 31, 0)})
	assert.Equal(t, "17:31:00", out.IssueTime)
}

func TestAECustomsReference(t *testing.T) {
	t.Run("writes the customs reference as the statement reference", func(t *testing.T) {
		inv := &bill.Invoice{Tax: &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyCustomsReference: "EXP010101234"})}}
		out := new(goblubl.Invoice)
		exportAECustomsReference(out, inv)
		require.Len(t, out.StatementDocumentReference, 1)
		assert.Equal(t, "EXP010101234", out.StatementDocumentReference[0].ID.Value)
	})

	t.Run("nothing without one", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAECustomsReference(out, &bill.Invoice{})
		assert.Empty(t, out.StatementDocumentReference)
	})

	t.Run("reads the first one back", func(t *testing.T) {
		in := &goblubl.Invoice{StatementDocumentReference: []goblubl.Reference{
			{ID: goblubl.IDType{Value: " "}},
			{ID: goblubl.IDType{Value: " EXP010101234 "}},
			{ID: goblubl.IDType{Value: "EXP2"}},
		}}
		inv := new(bill.Invoice)
		importAECustomsReference(in, inv)
		assert.Equal(t, cbc.Code("EXP010101234"), inv.Tax.GetExt(pint.ExtKeyCustomsReference))
	})
}

func TestExportAEContractValue(t *testing.T) {
	inv := &bill.Invoice{Ordering: &bill.Ordering{Contracts: []*org.DocumentRef{
		{Code: "CT-1", Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyContractValue: "AED 200000"})},
		{Code: "CT-2", Description: "Framework agreement"},
	}}}

	t.Run("writes the value of each contract", func(t *testing.T) {
		out := &goblubl.Invoice{ContractDocumentReference: []goblubl.Reference{
			{ID: goblubl.IDType{Value: "CT-1"}},
			{ID: goblubl.IDType{Value: "CT-2"}},
		}}
		exportAEContractValue(out, inv)
		assert.Equal(t, "AED 200000", out.ContractDocumentReference[0].DocumentDescription)
		assert.Empty(t, out.ContractDocumentReference[1].DocumentDescription)
	})

	t.Run("references that don't match are skipped", func(t *testing.T) {
		out := &goblubl.Invoice{ContractDocumentReference: []goblubl.Reference{{ID: goblubl.IDType{Value: "CT-1"}}}}
		exportAEContractValue(out, inv)
		assert.Empty(t, out.ContractDocumentReference[0].DocumentDescription)
		exportAEContractValue(out, &bill.Invoice{})
		assert.Empty(t, out.ContractDocumentReference[0].DocumentDescription)
	})

	t.Run("reads it back from the description gobl.ubl imports", func(t *testing.T) {
		in := &goblubl.Invoice{ContractDocumentReference: []goblubl.Reference{
			{ID: goblubl.IDType{Value: "CT-1"}, DocumentDescription: " AED 200000 "},
			{ID: goblubl.IDType{Value: "CT-2"}},
		}}
		got := &bill.Invoice{Ordering: &bill.Ordering{Contracts: []*org.DocumentRef{
			{Code: "CT-1", Description: "AED 200000"},
			{Code: "CT-2"},
		}}}
		importAEContractValue(in, got)
		assert.Equal(t, cbc.Code("AED 200000"), got.Ordering.Contracts[0].Ext.Get(pint.ExtKeyContractValue))
		assert.Empty(t, got.Ordering.Contracts[0].Description)
		assert.True(t, got.Ordering.Contracts[1].Ext.IsZero())
	})
}

func TestExportAEPayment(t *testing.T) {
	iban := cbc.Code("AE070331234567890123456")
	ublInvoice := func() *goblubl.Invoice {
		return &goblubl.Invoice{PaymentMeans: []goblubl.PaymentMeans{{
			PayeeFinancialAccount: &goblubl.FinancialAccount{ID: &goblubl.IDType{Value: iban.String()}},
		}}}
	}
	goblInvoice := func(ct *pay.CreditTransfer, dd *pay.DueDate) *bill.Invoice {
		inv := &bill.Invoice{Currency: currency.AED, Payment: &bill.PaymentDetails{
			Instructions: &pay.Instructions{CreditTransfer: []*pay.CreditTransfer{ct}},
		}}
		if dd != nil {
			inv.Payment.Terms = &pay.Terms{DueDates: []*pay.DueDate{dd}}
		}
		return inv
	}
	amount := num.MakeAmount(105000, 2)
	dueDate := func() *pay.DueDate {
		return &pay.DueDate{Date: cal.NewDate(2025, 2, 6), Amount: &amount}
	}

	t.Run("writes the IBAN scheme and the branch address", func(t *testing.T) {
		out := ublInvoice()
		exportAEPayment(out, goblInvoice(&pay.CreditTransfer{
			IBAN:   iban,
			Branch: &org.Address{Street: "Baniyas Road", StreetExtra: "Deira", Locality: "Dubai", Region: "DXB", Code: "777", Country: "AE"},
		}, nil))
		acc := out.PaymentMeans[0].PayeeFinancialAccount
		assert.Equal(t, "IBAN", *acc.ID.SchemeID)
		addr := acc.FinancialInstitutionBranch.Address
		require.NotNil(t, addr)
		assert.Equal(t, "Baniyas Road", *addr.StreetName)
		assert.Equal(t, "Deira", *addr.AdditionalStreetName)
		assert.Equal(t, "Dubai", *addr.CityName)
		assert.Equal(t, "DXB", *addr.CountrySubentity)
		assert.Equal(t, "777", *addr.PostalZone)
		assert.Equal(t, "AE", addr.Country.IdentificationCode)
		assert.Nil(t, out.PaymentMeans[0].ID, "no terms to refer to the instructions")
		assert.Nil(t, out.PaymentTerms)
	})

	t.Run("an account number has no scheme, and no branch no address", func(t *testing.T) {
		out := ublInvoice()
		exportAEPayment(out, goblInvoice(&pay.CreditTransfer{Number: iban}, nil))
		acc := out.PaymentMeans[0].PayeeFinancialAccount
		assert.Nil(t, acc.ID.SchemeID)
		assert.Nil(t, acc.FinancialInstitutionBranch)
	})

	t.Run("writes the first due date as the terms of the instructions", func(t *testing.T) {
		out := ublInvoice()
		out.PaymentTerms = &goblubl.PaymentTerms{Note: "Net 15 days"}
		exportAEPayment(out, goblInvoice(&pay.CreditTransfer{IBAN: iban}, dueDate()))
		assert.Equal(t, "1", *out.PaymentMeans[0].ID)
		assert.Equal(t, "1", out.PaymentTerms.PaymentMeansID)
		assert.Equal(t, "Net 15 days", out.PaymentTerms.Note)
		assert.Equal(t, aedAmount("1050.00"), *out.PaymentTerms.Amount)
		assert.Equal(t, "2025-02-06", out.PaymentTerms.InstallmentDueDate)
	})

	t.Run("no terms for a due date in another currency", func(t *testing.T) {
		out := ublInvoice()
		dd := dueDate()
		dd.Currency = currency.USD
		exportAEPayment(out, goblInvoice(&pay.CreditTransfer{IBAN: iban}, dd))
		assert.Nil(t, out.PaymentTerms)
		assert.Nil(t, out.PaymentMeans[0].ID)
	})

	t.Run("nothing without payment instructions", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEPayment(out, goblInvoice(&pay.CreditTransfer{IBAN: iban}, dueDate()))
		assert.Nil(t, out.PaymentTerms)
	})

	t.Run("writes the scheme of the remittance reference and reads it back", func(t *testing.T) {
		out := ublInvoice()
		out.PaymentMeans[0].PaymentID = &goblubl.IDType{Value: "REM-1"}
		inv := goblInvoice(&pay.CreditTransfer{IBAN: iban}, nil)
		inv.Payment.Instructions.Ext = tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyRemittanceScheme: "ABA"})
		exportAEPayment(out, inv)
		assert.Equal(t, "ABA", *out.PaymentMeans[0].PaymentID.SchemeID)

		got := goblInvoice(&pay.CreditTransfer{IBAN: iban}, nil)
		importAERemittanceScheme(out, got)
		assert.Equal(t, cbc.Code("ABA"), got.Payment.Instructions.Ext.Get(pint.ExtKeyRemittanceScheme))
	})

	t.Run("no scheme without one on the reference", func(t *testing.T) {
		out := ublInvoice()
		out.PaymentMeans[0].PaymentID = &goblubl.IDType{Value: "REM-1"}
		got := goblInvoice(&pay.CreditTransfer{IBAN: iban}, nil)
		exportAEPayment(out, got)
		assert.Nil(t, out.PaymentMeans[0].PaymentID.SchemeID)
		importAERemittanceScheme(out, got)
		assert.True(t, got.Payment.Instructions.Ext.IsZero())
	})
}

func TestAELineReferences(t *testing.T) {
	ext := tax.ExtensionsOf(cbc.CodeMap{
		pint.ExtKeyPurchaseOrder:  "PO-1",
		pint.ExtKeyDespatchAdvice: "DA-1",
	})

	t.Run("writes the purchase order next to the order line", func(t *testing.T) {
		line := goblubl.InvoiceLine{OrderLineReference: &goblubl.OrderLineReference{LineID: "3"}}
		exportAELineReferences(&line, &bill.Line{Order: "3", Ext: ext})
		assert.Equal(t, "3", line.OrderLineReference.LineID)
		assert.Equal(t, "PO-1", line.OrderLineReference.OrderReference.ID)
		assert.Equal(t, "NA", line.DespatchLineReference.LineID)
		assert.Equal(t, "DA-1", line.DespatchLineReference.DocumentReference.ID.Value)
	})

	t.Run("without an order line, NA stands for it and is dropped on import", func(t *testing.T) {
		line := goblubl.InvoiceLine{}
		exportAELineReferences(&line, &bill.Line{Ext: ext})
		assert.Equal(t, "NA", line.OrderLineReference.LineID)

		inv := &bill.Invoice{Lines: []*bill.Line{{Order: "NA"}}}
		importAELineReferences([]goblubl.InvoiceLine{line}, inv)
		assert.Empty(t, inv.Lines[0].Order)
		assert.Equal(t, cbc.Code("PO-1"), inv.Lines[0].Ext.Get(pint.ExtKeyPurchaseOrder))
		assert.Equal(t, cbc.Code("DA-1"), inv.Lines[0].Ext.Get(pint.ExtKeyDespatchAdvice))
	})

	t.Run("nothing without the extensions", func(t *testing.T) {
		line := goblubl.InvoiceLine{OrderLineReference: &goblubl.OrderLineReference{LineID: "3"}}
		exportAELineReferences(&line, &bill.Line{Order: "3"})
		assert.Nil(t, line.OrderLineReference.OrderReference)
		assert.Nil(t, line.DespatchLineReference)

		inv := &bill.Invoice{Lines: []*bill.Line{{Order: "3"}}}
		importAELineReferences([]goblubl.InvoiceLine{line}, inv)
		assert.Equal(t, cbc.Code("3"), inv.Lines[0].Order)
		assert.True(t, inv.Lines[0].Ext.IsZero())
	})
}

func TestAEChargeExemptions(t *testing.T) {
	combo := func(code cbc.Code) tax.Set {
		c := &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyExempt}
		if code != "" {
			c.Ext = tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyExemption: code})
		}
		return tax.Set{c}
	}
	category := func(code *string) []*goblubl.TaxCategory {
		return []*goblubl.TaxCategory{{ID: &goblubl.IDType{Value: "E"}, TaxExemptionReasonCode: code}}
	}

	t.Run("writes the reasons of the charges, then the discounts", func(t *testing.T) {
		inv := &bill.Invoice{
			Charges:   []*bill.Charge{{Taxes: combo("DL8.46.3")}},
			Discounts: []*bill.Discount{{Taxes: combo("")}, {Taxes: combo("DL8.46.1")}},
		}
		out := &goblubl.Invoice{AllowanceCharge: []goblubl.AllowanceCharge{
			{ChargeIndicator: true, TaxCategory: category(nil)},
			{TaxCategory: category(nil)},
			{TaxCategory: category(nil)},
		}}
		exportAEChargeExemptions(out, inv)
		assert.Equal(t, "DL8.46.3", *out.AllowanceCharge[0].TaxCategory[0].TaxExemptionReasonCode)
		assert.Nil(t, out.AllowanceCharge[1].TaxCategory[0].TaxExemptionReasonCode)
		assert.Equal(t, "DL8.46.1", *out.AllowanceCharge[2].TaxCategory[0].TaxExemptionReasonCode)
	})

	t.Run("reads them back in the order of the document", func(t *testing.T) {
		in := &goblubl.Invoice{AllowanceCharge: []goblubl.AllowanceCharge{
			{TaxCategory: category(strPtr(" DL8.46.1 "))},
			{ChargeIndicator: true, TaxCategory: category(strPtr("DL8.46.3"))},
			{TaxCategory: category(nil)},
		}}
		inv := &bill.Invoice{
			Charges:   []*bill.Charge{{Taxes: combo("")}},
			Discounts: []*bill.Discount{{Taxes: combo("")}, {Taxes: combo("")}},
		}
		importAEChargeExemptions(in, inv)
		assert.Equal(t, cbc.Code("DL8.46.3"), inv.Charges[0].Taxes[0].Ext.Get(pint.ExtKeyExemption))
		assert.Equal(t, cbc.Code("DL8.46.1"), inv.Discounts[0].Taxes[0].Ext.Get(pint.ExtKeyExemption))
		assert.Empty(t, inv.Discounts[1].Taxes[0].Ext.Get(pint.ExtKeyExemption))
	})

	t.Run("charges that don't match are skipped", func(t *testing.T) {
		inv := &bill.Invoice{Discounts: []*bill.Discount{{Taxes: combo("DL8.46.1")}}}
		out := new(goblubl.Invoice)
		exportAEChargeExemptions(out, inv)
		in := &goblubl.Invoice{AllowanceCharge: []goblubl.AllowanceCharge{
			{TaxCategory: category(strPtr("DL8.46.2"))},
			{TaxCategory: category(strPtr("DL8.46.2"))},
		}}
		importAEChargeExemptions(in, inv)
		assert.Equal(t, cbc.Code("DL8.46.1"), inv.Discounts[0].Taxes[0].Ext.Get(pint.ExtKeyExemption))
	})
}

func TestExportAEChargeRates(t *testing.T) {
	category := func(id string) []*goblubl.TaxCategory {
		return []*goblubl.TaxCategory{{ID: &goblubl.IDType{Value: id}, Percent: strPtr("0")}}
	}
	out := &goblubl.Invoice{AllowanceCharge: []goblubl.AllowanceCharge{
		{ChargeIndicator: true, TaxCategory: category("AE")},
		{TaxCategory: category("AE")},
		{TaxCategory: category("E")},
	}}
	exportAEChargeRates(out)
	assert.Equal(t, "5", *out.AllowanceCharge[0].TaxCategory[0].Percent, "reverse charge charge (aligned-ibrp-ae-07)")
	assert.Equal(t, "5", *out.AllowanceCharge[1].TaxCategory[0].Percent, "reverse charge allowance (aligned-ibrp-ae-06)")
	assert.Equal(t, "0", *out.AllowanceCharge[2].TaxCategory[0].Percent, "exempt keeps 0 (aligned-ibrp-e-06)")
}

func TestAELineExemptionReason(t *testing.T) {
	ublLine := func(category string, reason *string) goblubl.InvoiceLine {
		return goblubl.InvoiceLine{Item: &goblubl.Item{ClassifiedTaxCategory: &goblubl.ClassifiedTaxCategory{
			ID: &goblubl.IDType{Value: category}, TaxExemptionReason: reason,
		}}}
	}
	goblLine := func(category cbc.Code) *bill.Line {
		return &bill.Line{Taxes: tax.Set{{
			Category: tax.CategoryVAT,
			Ext:      tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: category}),
		}}}
	}
	note := func(category cbc.Code, text string) *tax.Note {
		return &tax.Note{
			Category: tax.CategoryVAT,
			Text:     text,
			Ext:      tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: category}),
		}
	}

	t.Run("writes the tax note of the line's category", func(t *testing.T) {
		tx := &bill.Tax{Notes: []*tax.Note{note("O", "Out of scope"), note("E", "Exempt supply")}}
		line := ublLine("E", nil)
		exportAELineExemption(&line, goblLine("E"), tx)
		require.NotNil(t, line.Item.ClassifiedTaxCategory.TaxExemptionReason)
		assert.Equal(t, "Exempt supply", *line.Item.ClassifiedTaxCategory.TaxExemptionReason)
	})

	t.Run("nothing on standard rated lines or without a note", func(t *testing.T) {
		tx := &bill.Tax{Notes: []*tax.Note{note("S", "Standard"), note("O", "Out of scope")}}
		line := ublLine("S", nil)
		exportAELineExemption(&line, goblLine("S"), tx)
		assert.Nil(t, line.Item.ClassifiedTaxCategory.TaxExemptionReason)
		line = ublLine("E", nil)
		exportAELineExemption(&line, goblLine("E"), tx)
		assert.Nil(t, line.Item.ClassifiedTaxCategory.TaxExemptionReason)
	})

	t.Run("reads the text back as a tax note, once per category", func(t *testing.T) {
		inv := &bill.Invoice{Lines: []*bill.Line{goblLine("E"), goblLine("E"), goblLine("O")}}
		importAELineExemptions([]goblubl.InvoiceLine{
			ublLine("E", strPtr(" Exempt supply ")),
			ublLine("E", strPtr("Another text")),
			ublLine("O", strPtr(" ")),
		}, inv)
		require.Len(t, inv.Tax.Notes, 1)
		assert.Equal(t, "Exempt supply", inv.Tax.Notes[0].Text)
		assert.Equal(t, cbc.Code("E"), inv.Tax.Notes[0].Ext.Get(untdid.ExtKeyTaxCategory))
	})
}

func TestAELineSchemeVersions(t *testing.T) {
	version := func(v cbc.Code) tax.Extensions {
		return tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeySchemeVersion: v})
	}
	hs, sac := pint.IdentityLabelHS, pint.IdentityLabelSAC

	t.Run("writes the versions of the classification and the service accounting code", func(t *testing.T) {
		line := goblubl.InvoiceLine{Item: &goblubl.Item{
			CommodityClassification: &[]goblubl.CommodityClassification{
				{ItemClassificationCode: &goblubl.IDType{ListID: &hs, Value: "8471"}},
				{ItemClassificationCode: &goblubl.IDType{ListID: &hs, Value: "8472"}},
			},
			AdditionalItemIdentification: []goblubl.ItemIdentification{
				{ID: &goblubl.IDType{SchemeID: &sac, Value: "998713"}},
			},
		}}
		exportAELineSchemeVersions(&line, &bill.Line{Item: &org.Item{Identities: []*org.Identity{
			{Label: hs, Code: "8471", Ext: version("2022")},
			{Label: hs, Code: "8472"},
			{Label: sac, Code: "998713", Ext: version("1.0")},
		}}})
		ccs := *line.Item.CommodityClassification
		assert.Equal(t, "2022", *ccs[0].ItemClassificationCode.ListVersionID)
		assert.Nil(t, ccs[1].ItemClassificationCode.ListVersionID)
		assert.Equal(t, "1.0", *line.Item.AdditionalItemIdentification[0].ID.SchemeVersionID)
	})

	t.Run("reads them back onto the identities", func(t *testing.T) {
		line := goblubl.InvoiceLine{Item: &goblubl.Item{
			CommodityClassification: &[]goblubl.CommodityClassification{
				{ItemClassificationCode: &goblubl.IDType{ListID: &hs, ListVersionID: strPtr("2022"), Value: "8471"}},
			},
			AdditionalItemIdentification: []goblubl.ItemIdentification{
				{ID: &goblubl.IDType{SchemeID: &sac, SchemeVersionID: strPtr("1.0"), Value: "998713"}},
			},
		}}
		inv := &bill.Invoice{Lines: []*bill.Line{{Item: &org.Item{Identities: []*org.Identity{
			{Label: hs, Code: "8471"},
		}}}}}
		importAELineItems([]goblubl.InvoiceLine{line}, inv)
		ids := inv.Lines[0].Item.Identities
		require.Len(t, ids, 2)
		assert.Equal(t, cbc.Code("2022"), ids[0].Ext.Get(pint.ExtKeySchemeVersion))
		assert.Equal(t, cbc.Code("1.0"), ids[1].Ext.Get(pint.ExtKeySchemeVersion))
	})
}

func TestAELineBatch(t *testing.T) {
	t.Run("writes the batch number as the lot of the item", func(t *testing.T) {
		line := goblubl.InvoiceLine{Item: &goblubl.Item{}}
		exportAELineBatch(&line, &bill.Line{Item: &org.Item{
			Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyBatchNumber: "Lot 1"}),
		}})
		require.Len(t, line.Item.ItemInstance, 1)
		assert.Equal(t, "Lot 1", line.Item.ItemInstance[0].LotIdentification.LotNumberID)
	})

	t.Run("nothing without one", func(t *testing.T) {
		line := goblubl.InvoiceLine{Item: &goblubl.Item{}}
		exportAELineBatch(&line, &bill.Line{Item: &org.Item{}})
		assert.Empty(t, line.Item.ItemInstance)
	})

	t.Run("reads it back onto the item", func(t *testing.T) {
		line := goblubl.InvoiceLine{Item: &goblubl.Item{ItemInstance: []goblubl.ItemInstance{
			{},
			{LotIdentification: &goblubl.LotIdentification{LotNumberID: " Lot 1 "}},
		}}}
		inv := &bill.Invoice{Lines: []*bill.Line{{Item: &org.Item{}}}}
		importAELineGoods([]goblubl.InvoiceLine{line}, inv)
		assert.Equal(t, cbc.Code("Lot 1"), inv.Lines[0].Item.Ext.Get(pint.ExtKeyBatchNumber))
	})
}
