package ubl

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
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
		assert.Equal(t, "0", ac.Amount.Value)
		require.NotNil(t, ac.BaseAmount)
		assert.Equal(t, "100.00", ac.BaseAmount.Value)
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
}

func TestExportAEPartyTaxID(t *testing.T) {
	party := func(companyID string) *goblubl.Party {
		return &goblubl.Party{PartyTaxScheme: []goblubl.PartyTaxScheme{{
			CompanyID: &goblubl.IDType{Value: companyID},
			TaxScheme: &goblubl.TaxScheme{ID: goblubl.IDType{Value: "VAT"}},
		}}}
	}

	t.Run("writes a UAE TRN without its prefix", func(t *testing.T) {
		out := party("AE100000000000003")
		exportAEPartyTaxID(out, &tax.Identity{Country: "AE", Code: "100000000000003"})
		assert.Equal(t, "100000000000003", out.PartyTaxScheme[0].CompanyID.Value)
	})

	t.Run("keeps the prefix of other countries' VAT numbers", func(t *testing.T) {
		out := party("DE111111125")
		exportAEPartyTaxID(out, &tax.Identity{Country: "DE", Code: "111111125"})
		assert.Equal(t, "DE111111125", out.PartyTaxScheme[0].CompanyID.Value)
	})

	t.Run("leaves other tax registrations alone", func(t *testing.T) {
		out := party("1234567890")
		exportAEPartyTaxID(out, &tax.Identity{Country: "AE", Code: "100000000000003"})
		assert.Equal(t, "1234567890", out.PartyTaxScheme[0].CompanyID.Value)
	})

	t.Run("parties without a tax ID are skipped", func(t *testing.T) {
		out := party("AE100000000000003")
		exportAEPartyTaxID(out, nil)
		assert.Equal(t, "AE100000000000003", out.PartyTaxScheme[0].CompanyID.Value)
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
	legal := func(key cbc.Key, code string) *org.Identity {
		return &org.Identity{Key: key, Scope: org.IdentityScopeLegal, Code: cbc.Code(code)}
	}

	t.Run("trade licence gets TL and its authority", func(t *testing.T) {
		id := legal(ae.IdentityKeyTradeLicense, "112345678900003")
		id.Description = "Trade License issuing Authority"
		cid := export(&org.Party{Name: "Seller", Identities: []*org.Identity{id}}).PartyLegalEntity.CompanyID
		assert.Equal(t, "112345678900003", cid.Value)
		assert.Equal(t, "TL", *cid.SchemeAgencyID)
		assert.Equal(t, "Trade License issuing Authority", *cid.SchemeAgencyName)
	})

	t.Run("passport gets PAS and its issuing country", func(t *testing.T) {
		id := legal(org.IdentityKeyPassport, "P1234567890")
		id.Country = "IN"
		cid := export(&org.Party{Name: "Buyer", Identities: []*org.Identity{id}}).PartyLegalEntity.CompanyID
		assert.Equal(t, "PAS", *cid.SchemeAgencyID)
		assert.Equal(t, "IN", *cid.SchemeAgencyName)
	})

	t.Run("Emirates ID and Cabinet Decision get their type only", func(t *testing.T) {
		for key, typ := range map[cbc.Key]string{
			ae.IdentityKeyEmiratesID:      "EID",
			ae.IdentityKeyCabinetDecision: "CD",
		} {
			id := legal(key, "784199012345671")
			id.Description = "not an authority"
			cid := export(&org.Party{Name: "Party", Identities: []*org.Identity{id}}).PartyLegalEntity.CompanyID
			assert.Equal(t, typ, *cid.SchemeAgencyID, key)
			assert.Nil(t, cid.SchemeAgencyName, key)
		}
	})

	t.Run("passport ignores the description", func(t *testing.T) {
		id := legal(org.IdentityKeyPassport, "P1234567890")
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
			{Key: ae.IdentityKeyTIN, Scope: org.IdentityScopeTax, Code: "1234567890"},
			legal(ae.IdentityKeyEmiratesID, "784199012345671"),
			legal(ae.IdentityKeyCabinetDecision, "CD-1"),
		}}).PartyLegalEntity.CompanyID
		assert.Equal(t, "784199012345671", cid.Value)
		assert.Equal(t, "EID", *cid.SchemeAgencyID)
	})

	t.Run("TIN is written under the TIN scheme, next to the VAT TRN", func(t *testing.T) {
		out := export(&org.Party{
			Name:  "Seller",
			TaxID: &tax.Identity{Country: "AE", Code: "100000000000003"},
			Identities: []*org.Identity{
				{Key: ae.IdentityKeyTIN, Scope: org.IdentityScopeTax, Code: "1234567890"},
			},
		})
		require.Len(t, out.PartyTaxScheme, 2)
		assert.Equal(t, "VAT", out.PartyTaxScheme[0].TaxScheme.ID.Value)
		assert.Equal(t, "1234567890", out.PartyTaxScheme[1].CompanyID.Value)
		assert.Equal(t, "TIN", out.PartyTaxScheme[1].TaxScheme.ID.Value)
	})

	t.Run("other tax identities keep the scheme gobl.ubl gives them", func(t *testing.T) {
		out := export(&org.Party{Name: "Seller", Identities: []*org.Identity{
			{Scope: org.IdentityScopeTax, Code: "1234567890"},
		}})
		require.Len(t, out.PartyTaxScheme, 1)
		assert.Equal(t, goblubl.TaxSchemeTaxRegistration, out.PartyTaxScheme[0].TaxScheme.ID.Value)
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
		assert.Equal(t, ae.IdentityKeyTradeLicense, id.Key)
		assert.Equal(t, "Trade License issuing Authority", id.Description)
	})

	t.Run("CL is read as a trade licence", func(t *testing.T) {
		id := imported(companyID("CL", "Dubai Economy and Tourism"), false).Identities[0]
		assert.Equal(t, ae.IdentityKeyTradeLicense, id.Key)
		assert.Equal(t, "Dubai Economy and Tourism", id.Description)
	})

	t.Run("passport with its issuing country", func(t *testing.T) {
		id := imported(companyID("PAS", "IN"), false).Identities[0]
		assert.Equal(t, org.IdentityKeyPassport, id.Key)
		assert.Equal(t, l10n.ISOCountryCode("IN"), id.Country)
		assert.Empty(t, id.Description)
	})

	t.Run("Emirates ID and Cabinet Decision, without an authority", func(t *testing.T) {
		for typ, key := range map[string]cbc.Key{"EID": ae.IdentityKeyEmiratesID, "CD": ae.IdentityKeyCabinetDecision} {
			id := imported(companyID(typ, "Some Authority"), false).Identities[0]
			assert.Equal(t, key, id.Key, typ)
			assert.Empty(t, id.Description, typ)
		}
	})

	t.Run("unknown or missing types are left alone", func(t *testing.T) {
		for _, typ := range []string{"XYZ", ""} {
			id := imported(companyID(typ, "Some Authority"), false).Identities[0]
			assert.Empty(t, id.Key, typ)
			assert.Empty(t, id.Description, typ)
		}
	})

	t.Run("supplier non-VAT tax registration is the TIN", func(t *testing.T) {
		in := companyID("TL", "Authority")
		in.PartyTaxScheme = []goblubl.PartyTaxScheme{
			{CompanyID: &goblubl.IDType{Value: "198765432102003"}, TaxScheme: &goblubl.TaxScheme{ID: goblubl.IDType{Value: "VAT"}}},
			{CompanyID: &goblubl.IDType{Value: "1234567890"}, TaxScheme: &goblubl.TaxScheme{ID: goblubl.IDType{Value: "TIN"}}},
		}
		p := imported(in, true)
		require.Len(t, p.Identities, 2)
		assert.Equal(t, ae.IdentityKeyTIN, p.Identities[1].Key)
		assert.Equal(t, cbc.Code("1234567890"), p.Identities[1].Code)

		p = imported(in, false)
		assert.Empty(t, p.Identities[1].Key, "only the seller has a TIN")
	})

	t.Run("nil parties are skipped", func(t *testing.T) {
		assert.NotPanics(t, func() {
			importAEPartyIdentities(nil, &org.Party{}, true)
			importAEPartyIdentities(&goblubl.Party{}, nil, true)
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
	assert.Equal(t, ae.TaxKeyMarginScheme, ms.Key)
	assert.Nil(t, ms.Percent, "margin scheme lines charge no VAT")
	assert.Equal(t, ae.TaxKeyMarginScheme, inv.Discounts[0].Taxes[0].Key)

	other := inv.Lines[1].Taxes[0]
	assert.Empty(t, other.Key, "other categories are left to EN 16931")
	assert.NotNil(t, other.Percent)
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
		importAEOutsideScope(in, inv)
		return inv
	}

	inv := importCode("480", "")
	assert.Equal(t, bill.InvoiceTypeStandard, inv.Type)
	assert.True(t, inv.HasTags(pint.TagOutsideScope))

	inv = importCode("", "81")
	assert.Equal(t, bill.InvoiceTypeCreditNote, inv.Type)
	assert.True(t, inv.HasTags(pint.TagOutsideScope))

	inv = importCode("380", "")
	assert.Equal(t, bill.InvoiceTypeOther, inv.Type, "other codes are left to gobl.ubl")
	assert.False(t, inv.HasTags(pint.TagOutsideScope))

	t.Run("does not add the tag twice", func(t *testing.T) {
		in := &goblubl.Invoice{InvoiceTypeCode: &goblubl.IDType{Value: "480"}}
		inv := &bill.Invoice{Tags: tax.WithTags(pint.TagOutsideScope)}
		importAEOutsideScope(in, inv)
		assert.Equal(t, []cbc.Key{pint.TagOutsideScope}, inv.Tags.List)
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
		exportAELineExemption(&line, goblLine("DL8.46.2"))
		require.NotNil(t, line.Item.ClassifiedTaxCategory.TaxExemptionReasonCode)
		assert.Equal(t, "DL8.46.2", *line.Item.ClassifiedTaxCategory.TaxExemptionReasonCode)
	})

	t.Run("nothing without a reason", func(t *testing.T) {
		line := ublLine()
		exportAELineExemption(&line, goblLine(""))
		assert.Nil(t, line.Item.ClassifiedTaxCategory.TaxExemptionReasonCode)
		exportAELineExemption(&line, nil)
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

func TestAEIncoterms(t *testing.T) {
	incoterms := &org.Identity{Label: pint.IdentityLabelIncoterms, Code: "CIF"}
	gln := &org.Identity{Code: "5790000436101", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: "0088"})}
	withLocation := func(id string) *goblubl.Invoice {
		return &goblubl.Invoice{Delivery: []*goblubl.Delivery{{DeliveryLocation: &goblubl.Location{ID: &goblubl.IDType{Value: id}}}}}
	}

	t.Run("writes the Incoterms as the delivery terms", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEIncoterms(out, &bill.Invoice{Delivery: &bill.DeliveryDetails{Identities: []*org.Identity{incoterms}}})
		require.Len(t, out.Delivery, 1)
		require.NotNil(t, out.Delivery[0].DeliveryTerms)
		assert.Equal(t, "CIF", out.Delivery[0].DeliveryTerms.ID)
	})

	t.Run("keeps the Incoterms out of the delivery location ID", func(t *testing.T) {
		out := withLocation("CIF")
		exportAEIncoterms(out, &bill.Invoice{Delivery: &bill.DeliveryDetails{Identities: []*org.Identity{incoterms, gln}}})
		id := out.Delivery[0].DeliveryLocation.ID
		require.NotNil(t, id)
		assert.Equal(t, "5790000436101", id.Value)
		assert.Equal(t, "0088", *id.SchemeID)

		out = withLocation("CIF")
		exportAEIncoterms(out, &bill.Invoice{Delivery: &bill.DeliveryDetails{Identities: []*org.Identity{incoterms}}})
		assert.Nil(t, out.Delivery[0].DeliveryLocation.ID)
	})

	t.Run("nothing without Incoterms", func(t *testing.T) {
		out := withLocation("5790000436101")
		exportAEIncoterms(out, &bill.Invoice{Delivery: &bill.DeliveryDetails{Identities: []*org.Identity{gln}}})
		assert.Nil(t, out.Delivery[0].DeliveryTerms)
		assert.Equal(t, "5790000436101", out.Delivery[0].DeliveryLocation.ID.Value)
		exportAEIncoterms(out, &bill.Invoice{})
		assert.Nil(t, out.Delivery[0].DeliveryTerms)
	})

	t.Run("reads the Incoterms back", func(t *testing.T) {
		in := &goblubl.Invoice{Delivery: []*goblubl.Delivery{{DeliveryTerms: &goblubl.DeliveryTerms{ID: " CIF "}}}}
		inv := new(bill.Invoice)
		importAEIncoterms(in, inv)
		require.NotNil(t, inv.Delivery)
		require.Len(t, inv.Delivery.Identities, 1)
		assert.Equal(t, pint.IdentityLabelIncoterms, inv.Delivery.Identities[0].Label)
		assert.Equal(t, cbc.Code("CIF"), inv.Delivery.Identities[0].Code)

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
		exportAELineAmounts(out, goblLine("10486.00", tax.KeyStandard, num.NewPercentage(5, 2)), currency.AED)
		ext := out.ItemPriceExtension
		require.NotNil(t, ext)
		assert.Equal(t, "11010.30", ext.Amount.Value)
		assert.Equal(t, "AED", *ext.Amount.CurrencyID)
		require.Len(t, ext.TaxTotal, 1)
		assert.Equal(t, "524.30", ext.TaxTotal[0].TaxAmount.Value)
	})

	t.Run("line VAT is rounded to the currency", func(t *testing.T) {
		out := line("S")
		exportAELineAmounts(out, goblLine("99.99", tax.KeyStandard, num.NewPercentage(5, 2)), currency.USD)
		assert.Equal(t, "5.00", out.ItemPriceExtension.TaxTotal[0].TaxAmount.Value)
		assert.Equal(t, "104.99", out.ItemPriceExtension.Amount.Value)
		assert.Equal(t, "USD", *out.ItemPriceExtension.Amount.CurrencyID, "document currency, as in the examples")
	})

	t.Run("zero rated and reverse charge lines get a zero VAT amount", func(t *testing.T) {
		for _, c := range []struct {
			category string
			key      cbc.Key
		}{{"Z", tax.KeyZero}, {"AE", tax.KeyReverseCharge}} {
			out := line(c.category)
			exportAELineAmounts(out, goblLine("200.00", c.key, nil), currency.AED)
			ext := out.ItemPriceExtension
			assert.Equal(t, "200.00", ext.Amount.Value, c.category)
			require.Len(t, ext.TaxTotal, 1, c.category)
			assert.Equal(t, "0.00", ext.TaxTotal[0].TaxAmount.Value, c.category)
		}
	})

	t.Run("exempt lines get no VAT amount", func(t *testing.T) {
		out := line("E")
		exportAELineAmounts(out, goblLine("120.00", tax.KeyExempt, nil), currency.AED)
		assert.Equal(t, "120.00", out.ItemPriceExtension.Amount.Value)
		assert.Empty(t, out.ItemPriceExtension.TaxTotal)
	})

	t.Run("lines without a total are left alone", func(t *testing.T) {
		out := line("S")
		exportAELineAmounts(out, &bill.Line{}, currency.AED)
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
		assert.Equal(t, "AED 913621.44", ref.DocumentDescription, "the Exports.xml example: 248750 USD at 3.67285")
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
