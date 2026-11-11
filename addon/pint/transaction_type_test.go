package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionType(t *testing.T) {
	t.Run("each flag has its position", func(t *testing.T) {
		tests := map[cbc.Code]pint.TransactionType{
			"10000000": {FreeTradeZone: true},
			"01000000": {DeemedSupply: true},
			"00100000": {MarginScheme: true},
			"00010000": {Summary: true},
			"00001000": {ContinuousSupply: true},
			"00000100": {DisclosedAgent: true},
			"00000010": {ECommerce: true},
			"00000001": {Export: true},
			"00000000": {},
		}
		for code, want := range tests {
			assert.Equal(t, want, pint.ParseTransactionType(code), code)
			assert.Equal(t, code, want.Code(), code)
		}
	})

	t.Run("several flags", func(t *testing.T) {
		tt := pint.ParseTransactionType("10000101")
		assert.Equal(t, pint.TransactionType{FreeTradeZone: true, DisclosedAgent: true, Export: true}, tt)
		assert.Equal(t, []cbc.Key{pint.TagFreeTradeZone, pint.TagDisclosedAgent, tax.TagExport}, tt.Tags())
	})

	t.Run("a missing or short code gives no flags beyond its length", func(t *testing.T) {
		assert.Equal(t, pint.TransactionType{}, pint.ParseTransactionType(""))
		assert.Equal(t, pint.TransactionType{FreeTradeZone: true}, pint.ParseTransactionType("1"))
	})
}

func TestNormalizeTransactionType(t *testing.T) {
	code := func(inv *bill.Invoice) cbc.Code {
		t.Helper()
		require.NoError(t, inv.Calculate())
		return inv.Tax.GetExt(pint.ExtKeyTransactionType)
	}

	t.Run("no tags give 00000000", func(t *testing.T) {
		assert.Equal(t, cbc.Code("00000000"), code(aeDocument(bill.InvoiceTypeStandard)))
	})

	t.Run("tags set their flags", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard, pint.TagSummary, pint.TagECommerce, tax.TagExport)
		assert.Equal(t, cbc.Code("00010011"), code(inv))
	})

	t.Run("a code set directly is kept without tags", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Tax = &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyTransactionType: "00001000"})}
		assert.Equal(t, cbc.Code("00001000"), code(inv))
	})

	t.Run("tags replace an earlier code", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard, pint.TagContinuousSupply)
		inv.Tax = &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyTransactionType: "00000000"})}
		assert.Equal(t, cbc.Code("00001000"), code(inv))
	})

	t.Run("only under the AE regime", func(t *testing.T) {
		inv := scenarioInvoice(l10n.AU.Tax(), bill.InvoiceTypeStandard)
		require.NoError(t, inv.Calculate())
		assert.Empty(t, inv.Tax.GetExt(pint.ExtKeyTransactionType))
	})
}
