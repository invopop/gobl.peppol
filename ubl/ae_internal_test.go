package ubl

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
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

func aeAmount(v string) goblubl.Amount {
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
			Price:            &goblubl.Price{PriceAmount: aeAmount("100.00")},
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
			Price:            &goblubl.Price{PriceAmount: aeAmount("5.00")},
		}
		exportAEPrice(line)
		assert.Equal(t, "C62", line.Price.BaseQuantity.UnitCode)
	})

	t.Run("keeps a price discount gobl.ubl already wrote", func(t *testing.T) {
		existing := &goblubl.AllowanceCharge{Amount: aeAmount("20.00"), BaseAmount: &goblubl.Amount{Value: "120.00"}}
		line := &goblubl.InvoiceLine{
			InvoicedQuantity: &goblubl.Quantity{UnitCode: "C62", Value: "1"},
			Price:            &goblubl.Price{PriceAmount: aeAmount("100.00"), AllowanceCharge: existing},
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
	assert.Equal(t, strPtr("5"), aeRate(&goblubl.IDType{Value: "S"}, strPtr("5")), "standard keeps its rate")
	assert.Nil(t, aeRate(&goblubl.IDType{Value: "O"}, nil), "outside scope keeps no rate")
	assert.Equal(t, zero, aeRate(nil, zero), "no category, no change")
}

func TestMergeAETaxSubtotals(t *testing.T) {
	exempt := func(base, code string) goblubl.TaxSubtotal {
		c := aeCategory("E", nil)
		c.TaxExemptionReasonCode = strPtr(code)
		return goblubl.TaxSubtotal{TaxableAmount: aeAmount(base), TaxAmount: aeAmount("0.00"), TaxCategory: c}
	}
	standard := goblubl.TaxSubtotal{
		TaxableAmount: aeAmount("1000.00"),
		TaxAmount:     aeAmount("50.00"),
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
		zero := goblubl.TaxSubtotal{TaxableAmount: aeAmount("10.00"), TaxAmount: aeAmount("0.00"), TaxCategory: aeCategory("S", strPtr("0"))}
		out := mergeAETaxSubtotals([]goblubl.TaxSubtotal{standard, zero})
		assert.Len(t, out, 2)
	})
}

func TestExportAETaxTotal(t *testing.T) {
	tt := &goblubl.TaxTotal{TaxSubtotal: []goblubl.TaxSubtotal{
		{TaxableAmount: aeAmount("100.00"), TaxAmount: aeAmount("0.00"), TaxCategory: aeCategory("AE", strPtr("0"))},
		{TaxableAmount: aeAmount("50.00"), TaxAmount: aeAmount("0.00"), TaxCategory: aeCategory("E", strPtr("0"))},
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
