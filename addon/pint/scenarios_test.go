package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scenarioInvoice is a minimal invoice for the given regime that can be
// calculated, so the regime and addon scenarios run on it.
func scenarioInvoice(regime l10n.TaxCountryCode, typ cbc.Key, tags ...cbc.Key) *bill.Invoice {
	category, cur := tax.CategoryVAT, currency.AED
	if regime == l10n.AU.Tax() {
		category, cur = tax.CategoryGST, currency.AUD
	}
	return &bill.Invoice{
		Regime:    tax.WithRegime(regime),
		Addons:    tax.WithAddons(pint.V1),
		Tags:      tax.WithTags(tags...),
		Type:      typ,
		Currency:  cur,
		Series:    "TEST",
		Code:      "0001",
		IssueDate: cal.MakeDate(2026, 11, 10),
		Supplier:  &org.Party{Name: "Supplier"},
		Customer:  &org.Party{Name: "Customer"},
		Lines: []*bill.Line{
			{
				Quantity: num.MakeAmount(1, 0),
				Item:     &org.Item{Name: "Item", Price: num.NewAmount(10000, 2)},
				Taxes:    tax.Set{{Category: category, Key: tax.KeyOutsideScope}},
			},
		},
	}
}

func documentType(t *testing.T, inv *bill.Invoice) cbc.Code {
	t.Helper()
	require.NoError(t, inv.Calculate())
	require.NotNil(t, inv.Tax)
	return inv.Tax.Ext.Get(untdid.ExtKeyDocumentType)
}

func TestInvoiceScenarios(t *testing.T) {
	ae := l10n.AE.Tax()

	t.Run("AE out-of-scope invoice is 480", func(t *testing.T) {
		inv := scenarioInvoice(ae, bill.InvoiceTypeStandard, pint.TagOutsideScope)
		assert.Equal(t, cbc.Code("480"), documentType(t, inv))
	})

	t.Run("AE out-of-scope credit note is 81", func(t *testing.T) {
		inv := scenarioInvoice(ae, bill.InvoiceTypeCreditNote, pint.TagOutsideScope)
		assert.Equal(t, cbc.Code("81"), documentType(t, inv))
	})

	t.Run("AE invoice without the tag keeps EN's 380", func(t *testing.T) {
		inv := scenarioInvoice(ae, bill.InvoiceTypeStandard)
		assert.Equal(t, cbc.Code("380"), documentType(t, inv))
	})

	t.Run("AE credit note without the tag keeps EN's 381", func(t *testing.T) {
		inv := scenarioInvoice(ae, bill.InvoiceTypeCreditNote)
		assert.Equal(t, cbc.Code("381"), documentType(t, inv))
	})

	t.Run("a hand-set code is replaced by the scenario", func(t *testing.T) {
		inv := scenarioInvoice(ae, bill.InvoiceTypeStandard, pint.TagOutsideScope)
		inv.Tax = &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyDocumentType: "380"})}
		assert.Equal(t, cbc.Code("480"), documentType(t, inv))
	})

	t.Run("the tag has no effect outside AE", func(t *testing.T) {
		inv := scenarioInvoice(l10n.AU.Tax(), bill.InvoiceTypeStandard, pint.TagOutsideScope)
		assert.Equal(t, cbc.Code("380"), documentType(t, inv))

		inv = scenarioInvoice(l10n.AU.Tax(), bill.InvoiceTypeCreditNote, pint.TagOutsideScope)
		assert.Equal(t, cbc.Code("381"), documentType(t, inv))
	})
}
