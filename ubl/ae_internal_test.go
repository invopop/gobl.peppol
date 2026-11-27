package ubl

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/icc"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/tax"
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

func TestExportAEPrice(t *testing.T) {
	t.Run("writes the zero discount with the decimals of the price", func(t *testing.T) {
		line := &goblubl.InvoiceLine{
			InvoicedQuantity: &goblubl.Quantity{UnitCode: "C62", Value: "1"},
			Price:            &goblubl.Price{PriceAmount: aedAmount("10.1234")},
		}
		exportAEPrice(line)
		assert.Equal(t, "0.0000", line.Price.AllowanceCharge.Amount.Value)
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
}

func TestAERate(t *testing.T) {
	zero := strPtr("0")
	assert.Nil(t, aeRate(&goblubl.IDType{Value: "E"}, zero, "5"), "exempt has no rate")
	rc := aeRate(&goblubl.IDType{Value: "AE"}, zero, "5")
	require.NotNil(t, rc)
	assert.Equal(t, "5", *rc, "reverse charge is written at the standard rate")
	ms := aeRate(&goblubl.IDType{Value: "N"}, zero, "5")
	require.NotNil(t, ms)
	assert.Equal(t, "5", *ms, "margin scheme is written at the standard rate")
	assert.Equal(t, strPtr("5"), aeRate(&goblubl.IDType{Value: "S"}, strPtr("5"), "5"), "standard keeps its rate")
	assert.Nil(t, aeRate(&goblubl.IDType{Value: "O"}, nil, "5"), "outside scope keeps no rate")
	assert.Equal(t, zero, aeRate(nil, zero, "5"), "no category, no change")
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
	imported := func(in *goblubl.Party, supplier bool) *org.Party {
		p := goblubl.ParseParty(in)
		importAEPartyIdentities(in, p, supplier)
		return p
	}

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

	t.Run("unknown or missing types are left alone", func(t *testing.T) {
		for _, typ := range []string{"XYZ", ""} {
			id := imported(companyID(typ, "Some Authority"), false).Identities[0]
			assert.Empty(t, id.Key, typ)
			assert.Empty(t, id.Type, typ)
			assert.Empty(t, id.Description, typ)
		}
	})

	t.Run("nil parties are skipped", func(t *testing.T) {
		assert.NotPanics(t, func() {
			importAEPartyIdentities(nil, &org.Party{}, true)
			importAEPartyIdentities(&goblubl.Party{}, nil, true)
		})
	})
}

func TestExportAEPartyIdentities(t *testing.T) {
	export := func(p *org.Party) *goblubl.Party {
		out := goblubl.NewParty(p)
		exportAEPartyIdentities(out, p)
		return out
	}

	t.Run("legal identity without a PINT AE type is left alone", func(t *testing.T) {
		cid := export(&org.Party{Name: "Party", Identities: []*org.Identity{
			{Scope: org.IdentityScopeLegal, Code: "CN-1234567"},
		}}).PartyLegalEntity.CompanyID
		assert.Nil(t, cid.SchemeAgencyID)
		assert.Nil(t, cid.SchemeAgencyName)
	})

	t.Run("parties without a company ID are left alone", func(t *testing.T) {
		out := export(&org.Party{Name: "Party"})
		require.NotNil(t, out.PartyLegalEntity)
		assert.Nil(t, out.PartyLegalEntity.CompanyID)
	})

	t.Run("nil parties are skipped", func(t *testing.T) {
		assert.NotPanics(t, func() {
			exportAEPartyIdentities(nil, &org.Party{})
			exportAEPartyIdentities(&goblubl.Party{}, nil)
		})
	})
}

func TestIdentityTypeAE(t *testing.T) {
	tests := []struct {
		id   *org.Identity
		want cbc.Code
	}{
		{&org.Identity{Type: ae.IdentityTypeTradeLicense}, "TL"},
		{&org.Identity{Type: ae.IdentityTypeEmiratesID}, "EID"},
		{&org.Identity{Key: org.IdentityKeyPassport}, identityTypePassport},
		{&org.Identity{Type: ae.IdentityTypeCabinetDecision}, "CD"},
		{&org.Identity{Type: ae.IdentityTypeTIN}, ""},
		{&org.Identity{}, ""},
		{nil, ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, identityTypeAE(tt.id), tt.id)
	}
}

func TestSetIdentityTypeAE(t *testing.T) {
	tests := map[cbc.Code]org.Identity{
		"TL":  {Type: ae.IdentityTypeTradeLicense},
		"EID": {Type: ae.IdentityTypeEmiratesID},
		"PAS": {Key: org.IdentityKeyPassport},
		"CD":  {Type: ae.IdentityTypeCabinetDecision},
		"XYZ": {},
		"":    {},
	}
	for typ, want := range tests {
		id := new(org.Identity)
		known := want.Key != "" || want.Type != ""
		assert.Equal(t, known, setIdentityTypeAE(id, typ), typ)
		assert.Equal(t, want, *id, typ)
	}
}

func TestExportAETransactionType(t *testing.T) {
	t.Run("writes beneficiary and principal IDs even without flags", func(t *testing.T) {
		out := new(goblubl.Invoice)
		inv := &bill.Invoice{
			Tax: &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{
				pint.ExtKeyTransactionType: "00000000",
				pint.ExtKeyBeneficiaryID:   "189098765401003",
				pint.ExtKeyPrincipalID:     "155667745601003",
			})},
		}
		exportAETransactionType(out, inv)
		assert.Equal(t, "00000000", out.ProfileExecutionID)
		require.NotNil(t, out.BuyerCustomerParty)
		assert.Equal(t, "189098765401003", out.BuyerCustomerParty.Party.PartyIdentification[0].ID.Value)
		require.NotNil(t, out.SellerSupplierParty)
		assert.Equal(t, "155667745601003", out.SellerSupplierParty.Party.PartyIdentification[0].ID.Value)
	})
}

func TestImportAETransactionType(t *testing.T) {
	t.Run("reads transaction type, beneficiary and principal IDs into tax extensions", func(t *testing.T) {
		inv := new(bill.Invoice)
		in := &goblubl.Invoice{
			ProfileExecutionID:  "10000100",
			BuyerCustomerParty:  &goblubl.CustomerParty{Party: identifiedParty("189098765401003")},
			SellerSupplierParty: &goblubl.SupplierParty{Party: identifiedParty("155667745601003")},
		}
		importAETransactionType(in, inv)
		require.NotNil(t, inv.Tax)
		assert.Equal(t, cbc.Code("10000100"), inv.Tax.GetExt(pint.ExtKeyTransactionType))
		assert.Equal(t, cbc.Code("189098765401003"), inv.Tax.GetExt(pint.ExtKeyBeneficiaryID))
		assert.Equal(t, cbc.Code("155667745601003"), inv.Tax.GetExt(pint.ExtKeyPrincipalID))
		assert.Empty(t, inv.Tags.List)
	})

	t.Run("nothing to read leaves the invoice alone", func(t *testing.T) {
		inv := new(bill.Invoice)
		importAETransactionType(&goblubl.Invoice{BuyerCustomerParty: &goblubl.CustomerParty{}}, inv)
		assert.Nil(t, inv.Tax)
		assert.Empty(t, inv.Tags.List)
	})
}

func TestExportAEMarginScheme(t *testing.T) {
	buildUBL := func() *goblubl.Invoice {
		vatScheme := &goblubl.TaxScheme{ID: goblubl.IDType{Value: "VAT"}}
		return &goblubl.Invoice{
			InvoiceLines: []goblubl.InvoiceLine{
				{Item: &goblubl.Item{ClassifiedTaxCategory: &goblubl.ClassifiedTaxCategory{ID: &goblubl.IDType{Value: "E"}, TaxScheme: vatScheme}}},
				{Item: &goblubl.Item{ClassifiedTaxCategory: &goblubl.ClassifiedTaxCategory{ID: &goblubl.IDType{Value: "S"}, TaxScheme: vatScheme}}},
			},
			CreditNoteLines: []goblubl.InvoiceLine{
				{Item: &goblubl.Item{ClassifiedTaxCategory: &goblubl.ClassifiedTaxCategory{ID: &goblubl.IDType{Value: "E"}, TaxScheme: vatScheme}}},
			},
			TaxTotal: []goblubl.TaxTotal{
				{TaxSubtotal: []goblubl.TaxSubtotal{
					{TaxCategory: aeCategory("E", nil)},
					{TaxCategory: aeCategory("S", strPtr("5"))},
				}},
			},
		}
	}

	t.Run("rewrites E to N on invoice lines, credit note lines and tax subtotals when flag is set", func(t *testing.T) {
		out := buildUBL()
		inv := &bill.Invoice{
			Tax: &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyTransactionType: "00100000"})},
		}
		exportAEMarginScheme(out, inv)

		assert.Equal(t, "N", out.InvoiceLines[0].Item.ClassifiedTaxCategory.ID.Value)
		assert.Equal(t, "S", out.InvoiceLines[1].Item.ClassifiedTaxCategory.ID.Value)
		assert.Equal(t, "N", out.CreditNoteLines[0].Item.ClassifiedTaxCategory.ID.Value)
		assert.Equal(t, "N", out.TaxTotal[0].TaxSubtotal[0].TaxCategory.ID.Value)
		assert.Equal(t, "S", out.TaxTotal[0].TaxSubtotal[1].TaxCategory.ID.Value)
	})

	t.Run("leaves E untouched when margin scheme flag is unset", func(t *testing.T) {
		out := buildUBL()
		inv := &bill.Invoice{
			Tax: &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyTransactionType: "00000000"})},
		}
		exportAEMarginScheme(out, inv)

		assert.Equal(t, "E", out.InvoiceLines[0].Item.ClassifiedTaxCategory.ID.Value)
		assert.Equal(t, "E", out.CreditNoteLines[0].Item.ClassifiedTaxCategory.ID.Value)
		assert.Equal(t, "E", out.TaxTotal[0].TaxSubtotal[0].TaxCategory.ID.Value)
	})
}

func TestImportAEMarginScheme(t *testing.T) {
	marginInv := func() *bill.Invoice {
		return &bill.Invoice{
			Lines: []*bill.Line{{
				Taxes: tax.Set{{
					Category: tax.CategoryVAT,
					Percent:  num.NewPercentage(5, 2),
					Ext:      tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: taxCategoryMarginScheme}),
				}},
			}},
		}
	}

	t.Run("rewrites N to exempt E and sets MarginScheme bit while preserving other flags", func(t *testing.T) {
		inv := marginInv()
		inv.Tax = &bill.Tax{Notes: []*tax.Note{{
			Category: tax.CategoryVAT,
			Text:     "Margin scheme",
			Ext:      tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: taxCategoryMarginScheme}),
		}}}
		importAETransactionType(&goblubl.Invoice{ProfileExecutionID: "10000000"}, inv)
		importAEMarginScheme(inv)

		assert.Equal(t, tax.KeyExempt, inv.Lines[0].Taxes[0].Key)
		assert.Nil(t, inv.Lines[0].Taxes[0].Percent)
		assert.Equal(t, cbc.Code(taxCategoryExempt), inv.Lines[0].Taxes[0].Ext.Get(untdid.ExtKeyTaxCategory))
		assert.Equal(t, cbc.Code(taxCategoryExempt), inv.Tax.Notes[0].Ext.Get(untdid.ExtKeyTaxCategory))
		assert.Equal(t, cbc.Code("10100000"), inv.Tax.GetExt(pint.ExtKeyTransactionType))
		assert.Empty(t, inv.Tags.List)
	})

	t.Run("empty ProfileExecutionID with N lines produces 00100000", func(t *testing.T) {
		inv := marginInv()
		importAETransactionType(&goblubl.Invoice{}, inv)
		importAEMarginScheme(inv)

		assert.Equal(t, tax.KeyExempt, inv.Lines[0].Taxes[0].Key)
		assert.Equal(t, cbc.Code(taxCategoryExempt), inv.Lines[0].Taxes[0].Ext.Get(untdid.ExtKeyTaxCategory))
		assert.Equal(t, cbc.Code("00100000"), inv.Tax.GetExt(pint.ExtKeyTransactionType))
	})

	t.Run("malformed ProfileExecutionID is not silently sanitized", func(t *testing.T) {
		inv := marginInv()
		importAETransactionType(&goblubl.Invoice{ProfileExecutionID: "0010000X"}, inv)
		importAEMarginScheme(inv)

		assert.Equal(t, tax.KeyExempt, inv.Lines[0].Taxes[0].Key)
		assert.Equal(t, cbc.Code(taxCategoryExempt), inv.Lines[0].Taxes[0].Ext.Get(untdid.ExtKeyTaxCategory))
		assert.Equal(t, cbc.Code("0010000X"), inv.Tax.GetExt(pint.ExtKeyTransactionType))
	})
}

func TestAECreditNoteReason(t *testing.T) {
	t.Run("exports from and imports into pint-ae-credit-note-reason without modifying tags or preceding", func(t *testing.T) {
		for _, code := range []cbc.Code{pint.CreditNoteReasonVolumeDiscount, "DL8.61.1.E"} {
			out := &goblubl.Invoice{CreditNoteTypeCode: &goblubl.IDType{Value: "381"}}
			exportAECreditNoteReason(out, &bill.Invoice{
				Tax: &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyCreditNoteReason: code})},
			})
			require.Len(t, out.DiscrepancyResponse, 1)
			assert.Equal(t, code.String(), out.DiscrepancyResponse[0].ResponseCode.Value)

			inv := &bill.Invoice{Preceding: []*org.DocumentRef{{Code: "INV-1"}}}
			importAECreditNoteReason(out, inv)
			assert.Equal(t, code, inv.Tax.GetExt(pint.ExtKeyCreditNoteReason))
			assert.Empty(t, inv.Tags.List)
			assert.Empty(t, inv.Preceding[0].Reason)
		}
	})
}

func TestAELineIdentitiesAndItemType(t *testing.T) {
	t.Run("preserves buyer ID, GTIN, HSN, SAC and other classification, prepending HS", func(t *testing.T) {
		l := &bill.Line{
			Item: &org.Item{
				Key: org.ItemKeyGoods,
				Ext: tax.ExtensionsOf(cbc.CodeMap{
					pint.ExtKeyItemType:  pint.ItemTypeGoods,
					pint.ExtKeyGoodsType: "DL8.48.8.1",
				}),
				Identities: []*org.Identity{
					{Code: "BUYER-1"},
					{Code: "4006381333931", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeGTIN})},
					{Label: "UNSPSC", Code: "43211500"},
					{Key: org.IdentityKeyHSN, Code: "8471"},
					{Label: "SAC", Code: "998713"},
				},
			},
		}
		line := &goblubl.InvoiceLine{
			Item: &goblubl.Item{
				BuyersItemIdentification: &goblubl.ItemIdentification{ID: &goblubl.IDType{Value: "BUYER-1"}},
			},
		}
		exportAELineIdentities(line, l)
		exportAELineGoods(line, l)
		exportAELineItemType(line, l)

		require.NotNil(t, line.Item.BuyersItemIdentification)
		assert.Equal(t, "BUYER-1", line.Item.BuyersItemIdentification.ID.Value)

		require.NotNil(t, line.Item.StandardItemIdentification)
		assert.Equal(t, "4006381333931", line.Item.StandardItemIdentification.ID.Value)
		assert.Equal(t, "0160", *line.Item.StandardItemIdentification.ID.SchemeID)

		require.NotNil(t, line.Item.CommodityClassification)
		ccs := *line.Item.CommodityClassification
		require.Len(t, ccs, 2)
		// HS is prepended so NatureCode (BTAE-09) and CommodityCode (BTAE-13) land on it.
		assert.Equal(t, "HS", *ccs[0].ItemClassificationCode.ListID)
		assert.Equal(t, "8471", ccs[0].ItemClassificationCode.Value)
		assert.Equal(t, "DL8.48.8.1", *ccs[0].NatureCode)
		assert.Equal(t, "G", *ccs[0].CommodityCode)
		assert.Equal(t, "UNSPSC", *ccs[1].ItemClassificationCode.ListID)
		assert.Equal(t, "43211500", ccs[1].ItemClassificationCode.Value)

		require.Len(t, line.Item.AdditionalItemIdentification, 1)
		assert.Equal(t, "SAC", *line.Item.AdditionalItemIdentification[0].ID.SchemeID)
		assert.Equal(t, "998713", line.Item.AdditionalItemIdentification[0].ID.Value)
	})

	t.Run("schemed hsn identity is not duplicated into CommodityClassification", func(t *testing.T) {
		l := &bill.Line{
			Item: &org.Item{
				Identities: []*org.Identity{
					{Key: org.IdentityKeyHSN, Code: "8471", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: "0088"})},
				},
			},
		}
		line := &goblubl.InvoiceLine{
			Item: &goblubl.Item{
				StandardItemIdentification: &goblubl.ItemIdentification{
					ID: &goblubl.IDType{SchemeID: strPtr("0088"), Value: "8471"},
				},
			},
		}
		exportAELineIdentities(line, l)
		assert.Nil(t, line.Item.CommodityClassification)
		assert.Nil(t, line.Item.BuyersItemIdentification)
		require.NotNil(t, line.Item.StandardItemIdentification)
		assert.Equal(t, "8471", line.Item.StandardItemIdentification.ID.Value)
	})

	t.Run("user-written label HS lands after key hsn and existing AdditionalItemIdentification is preserved", func(t *testing.T) {
		l := &bill.Line{
			Item: &org.Item{
				Identities: []*org.Identity{
					{Label: "HS", Code: "9999"},
					{Key: org.IdentityKeyHSN, Code: "8471"},
					{Label: "SAC", Code: "998713"},
				},
			},
		}
		existing := goblubl.ItemIdentification{ID: &goblubl.IDType{SchemeID: strPtr("OTHER"), Value: "X1"}}
		line := &goblubl.InvoiceLine{
			Item: &goblubl.Item{
				AdditionalItemIdentification: []goblubl.ItemIdentification{existing},
			},
		}
		exportAELineIdentities(line, l)
		require.NotNil(t, line.Item.CommodityClassification)
		ccs := *line.Item.CommodityClassification
		require.Len(t, ccs, 2)
		assert.Equal(t, "8471", ccs[0].ItemClassificationCode.Value)
		assert.Equal(t, "HS", *ccs[0].ItemClassificationCode.ListID)
		assert.Equal(t, "9999", ccs[1].ItemClassificationCode.Value)
		assert.Equal(t, "HS", *ccs[1].ItemClassificationCode.ListID)

		require.Len(t, line.Item.AdditionalItemIdentification, 2)
		assert.Equal(t, existing, line.Item.AdditionalItemIdentification[0])
		assert.Equal(t, "998713", line.Item.AdditionalItemIdentification[1].ID.Value)
	})

	t.Run("clears BuyersItemIdentification when only HSN is present", func(t *testing.T) {
		l := &bill.Line{
			Item: &org.Item{
				Identities: []*org.Identity{{Key: org.IdentityKeyHSN, Code: "8471"}},
			},
		}
		line := &goblubl.InvoiceLine{
			Item: &goblubl.Item{
				BuyersItemIdentification: &goblubl.ItemIdentification{ID: &goblubl.IDType{Value: "8471"}},
			},
		}
		exportAELineIdentities(line, l)
		assert.Nil(t, line.Item.BuyersItemIdentification)
		require.NotNil(t, line.Item.CommodityClassification)
		require.Len(t, *line.Item.CommodityClassification, 1)
		assert.Equal(t, "HS", *(*line.Item.CommodityClassification)[0].ItemClassificationCode.ListID)
		assert.Equal(t, "8471", (*line.Item.CommodityClassification)[0].ItemClassificationCode.Value)
	})

	t.Run("import converts HS label to key hsn and restores pint-ae-item-type and item key", func(t *testing.T) {
		for _, tc := range []struct {
			code    string
			wantKey cbc.Key
		}{
			{"G", org.ItemKeyGoods},
			{"S", org.ItemKeyServices},
			{"B", ""},
		} {
			inLine := goblubl.InvoiceLine{
				Item: &goblubl.Item{
					CommodityClassification: &[]goblubl.CommodityClassification{{
						CommodityCode: strPtr(tc.code),
					}},
				},
			}
			inv := &bill.Invoice{
				Lines: []*bill.Line{{
					Item: &org.Item{
						Identities: []*org.Identity{
							{Label: "HS", Code: "8471"},
							{Label: "UNSPSC", Code: "43211500"},
						},
					},
				}},
			}
			importAELineItems([]goblubl.InvoiceLine{inLine}, inv)
			it := inv.Lines[0].Item
			assert.Equal(t, cbc.Code(tc.code), it.Ext.Get(pint.ExtKeyItemType), tc.code)
			assert.Equal(t, tc.wantKey, it.Key, tc.code)
			assert.Equal(t, org.IdentityKeyHSN, it.Identities[0].Key)
			assert.Empty(t, it.Identities[0].Label)
			assert.Equal(t, "UNSPSC", it.Identities[1].Label)
			assert.Empty(t, it.Identities[1].Key)
		}
	})
}

func TestAEIncoterms(t *testing.T) {
	t.Run("writes the Incoterms as the delivery terms", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEIncoterms(out, &bill.Invoice{Delivery: &bill.DeliveryDetails{
			Ext: tax.ExtensionsOf(cbc.CodeMap{icc.ExtKeyIncoterms2020: "CIF"}),
		}})
		require.Len(t, out.Delivery, 1)
		require.Len(t, out.Delivery[0].DeliveryTerms, 1)
		id := out.Delivery[0].DeliveryTerms[0].ID
		assert.Equal(t, "CIF", id.Value)
		require.NotNil(t, id.SchemeID)
		assert.Equal(t, "Incoterms", *id.SchemeID)
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

func TestExportAELines(t *testing.T) {
	t.Run("lines without an item, price or total are left alone", func(t *testing.T) {
		lines := []goblubl.InvoiceLine{{}}
		exportAELines(lines, &bill.Invoice{Currency: currency.AED, Lines: []*bill.Line{{}}})
		assert.Equal(t, goblubl.InvoiceLine{}, lines[0])
	})
}

func TestExportAEExchangeRate(t *testing.T) {
	t.Run("invoices without a rate to AED are left to the PINT addon", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEExchangeRate(out, &bill.Invoice{
			Currency:      currency.USD,
			ExchangeRates: []*currency.ExchangeRate{{From: currency.USD, To: currency.EUR, Amount: num.MakeAmount(92, 2)}},
		})
		assert.Nil(t, out.TaxExchangeRate)
		assert.Empty(t, out.AdditionalDocumentReference)
	})
}

func TestExportAECreditTransfer(t *testing.T) {
	t.Run("writes the branch address on a new branch", func(t *testing.T) {
		acc := &goblubl.FinancialAccount{ID: strPtr("AE070331234567890123456")}
		exportAECreditTransfer(acc, &pay.CreditTransfer{
			IBAN:   "AE070331234567890123456",
			Branch: &org.Address{Street: "Baniyas Road", Locality: "Dubai", Region: "DXB", Country: "AE"},
		})
		require.NotNil(t, acc.FinancialInstitutionBranch)
		require.NotNil(t, acc.FinancialInstitutionBranch.Address)
		assert.Equal(t, "Dubai", *acc.FinancialInstitutionBranch.Address.CityName)
	})
}

func TestImportAEChargeExemptions(t *testing.T) {
	t.Run("charges that don't match are skipped", func(t *testing.T) {
		exempt := &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyExempt, Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyExemption: "DL8.46.1"})}
		inv := &bill.Invoice{Discounts: []*bill.Discount{{Taxes: tax.Set{exempt}}}}
		category := []*goblubl.TaxCategory{{ID: &goblubl.IDType{Value: "E"}, TaxExemptionReasonCode: strPtr("DL8.46.2")}}
		in := &goblubl.Invoice{AllowanceCharge: []goblubl.AllowanceCharge{{TaxCategory: category}, {TaxCategory: category}}}
		importAEChargeExemptions(in, inv)
		assert.Equal(t, cbc.Code("DL8.46.1"), inv.Discounts[0].Taxes[0].Ext.Get(pint.ExtKeyExemption))
	})
}

func TestClearAECEFVATEX(t *testing.T) {
	t.Run("clears cef-vatex copied by gobl.ubl", func(t *testing.T) {
		inv := &bill.Invoice{
			Lines: []*bill.Line{{Taxes: tax.Set{{Category: tax.CategoryVAT, Ext: tax.ExtensionsOf(cbc.CodeMap{
				cef.ExtKeyVATEX:      "DL8.46.1",
				pint.ExtKeyExemption: "DL8.46.1",
			})}}}},
			Discounts: []*bill.Discount{{Taxes: tax.Set{{Category: tax.CategoryVAT, Ext: tax.ExtensionsOf(cbc.CodeMap{
				cef.ExtKeyVATEX:      "DL8.46.2",
				pint.ExtKeyExemption: "DL8.46.2",
			})}}}},
			Charges: []*bill.Charge{{Taxes: tax.Set{{Category: tax.CategoryVAT, Ext: tax.ExtensionsOf(cbc.CodeMap{
				cef.ExtKeyVATEX:      "DL8.46.3",
				pint.ExtKeyExemption: "DL8.46.3",
			})}}}},
		}
		clearAECEFVATEX(inv)
		assert.False(t, inv.Lines[0].Taxes[0].Ext.Has(cef.ExtKeyVATEX))
		assert.Equal(t, cbc.Code("DL8.46.1"), inv.Lines[0].Taxes[0].Ext.Get(pint.ExtKeyExemption))
		assert.False(t, inv.Discounts[0].Taxes[0].Ext.Has(cef.ExtKeyVATEX))
		assert.Equal(t, cbc.Code("DL8.46.2"), inv.Discounts[0].Taxes[0].Ext.Get(pint.ExtKeyExemption))
		assert.False(t, inv.Charges[0].Taxes[0].Ext.Has(cef.ExtKeyVATEX))
		assert.Equal(t, cbc.Code("DL8.46.3"), inv.Charges[0].Taxes[0].Ext.Get(pint.ExtKeyExemption))
	})
}

func TestAEDeliveryPartyID(t *testing.T) {
	delivery := func(r *org.Party) *bill.Invoice {
		return &bill.Invoice{Delivery: &bill.DeliveryDetails{Receiver: r}}
	}

	t.Run("writes the receiver's TIN without a TRN", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEDeliveryPartyID(out, delivery(&org.Party{
			Identities: []*org.Identity{{Type: ae.IdentityTypeTIN, Code: "1345678906"}},
		}))
		require.Len(t, out.Delivery, 1)
		require.NotNil(t, out.Delivery[0].DeliveryParty)
		require.Len(t, out.Delivery[0].DeliveryParty.PartyIdentification, 1)
		assert.Equal(t, "1345678906", out.Delivery[0].DeliveryParty.PartyIdentification[0].ID.Value)
	})

	t.Run("nothing without a receiver", func(t *testing.T) {
		out := new(goblubl.Invoice)
		exportAEDeliveryPartyID(out, delivery(nil))
		assert.Empty(t, out.Delivery)
	})

	t.Run("reads anything but a TRN as a TIN, on a new receiver", func(t *testing.T) {
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
}
