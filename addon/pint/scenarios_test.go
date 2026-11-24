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
	standardLines := func(inv *bill.Invoice) *bill.Invoice {
		inv.Lines[0].Taxes = tax.Set{{Category: tax.CategoryVAT, Key: tax.KeyStandard}}
		return inv
	}

	t.Run("AE invoice with only out of scope lines is 480", func(t *testing.T) {
		assert.Equal(t, cbc.Code("480"), documentType(t, scenarioInvoice(ae, bill.InvoiceTypeStandard)))
	})

	t.Run("AE credit note with only out of scope lines is 81", func(t *testing.T) {
		assert.Equal(t, cbc.Code("81"), documentType(t, scenarioInvoice(ae, bill.InvoiceTypeCreditNote)))
	})

	t.Run("AE documents with only exempt lines are 480 and 81", func(t *testing.T) {
		exemptLines := func(inv *bill.Invoice) *bill.Invoice {
			inv.Lines[0].Taxes = tax.Set{{Category: tax.CategoryVAT, Key: tax.KeyExempt}}
			return inv
		}
		assert.Equal(t, cbc.Code("480"), documentType(t, exemptLines(scenarioInvoice(ae, bill.InvoiceTypeStandard))))
		assert.Equal(t, cbc.Code("81"), documentType(t, exemptLines(scenarioInvoice(ae, bill.InvoiceTypeCreditNote))))
	})

	t.Run("a standard or zero rated line next to an out of scope one keeps 380 and 381", func(t *testing.T) {
		for _, key := range []cbc.Key{tax.KeyStandard, tax.KeyZero} {
			for typ, code := range map[cbc.Key]cbc.Code{bill.InvoiceTypeStandard: "380", bill.InvoiceTypeCreditNote: "381"} {
				inv := scenarioInvoice(ae, typ)
				inv.Lines = append(inv.Lines, &bill.Line{
					Quantity: num.MakeAmount(1, 0),
					Item:     &org.Item{Name: "Other", Price: num.NewAmount(100, 2)},
					Taxes:    tax.Set{{Category: tax.CategoryVAT, Key: key}},
				})
				assert.Equal(t, code, documentType(t, inv), "%s %s", key, typ)
			}
		}
	})

	t.Run("a standard rated line keeps 380 and 381", func(t *testing.T) {
		assert.Equal(t, cbc.Code("380"), documentType(t, standardLines(scenarioInvoice(ae, bill.InvoiceTypeStandard))))
		assert.Equal(t, cbc.Code("381"), documentType(t, standardLines(scenarioInvoice(ae, bill.InvoiceTypeCreditNote))))
	})

	t.Run("a hand-set code is replaced by the one the lines give", func(t *testing.T) {
		inv := standardLines(scenarioInvoice(ae, bill.InvoiceTypeStandard))
		inv.Tax = &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyDocumentType: "480"})}
		assert.Equal(t, cbc.Code("380"), documentType(t, inv))
	})

	t.Run("self-billed documents keep 389 and 261", func(t *testing.T) {
		assert.Equal(t, cbc.Code("389"), documentType(t, scenarioInvoice(ae, bill.InvoiceTypeStandard, tax.TagSelfBilled)))
		assert.Equal(t, cbc.Code("261"), documentType(t, scenarioInvoice(ae, bill.InvoiceTypeCreditNote, tax.TagSelfBilled)))
	})

	t.Run("documents outside AE keep 380 and 381", func(t *testing.T) {
		assert.Equal(t, cbc.Code("380"), documentType(t, scenarioInvoice(l10n.AU.Tax(), bill.InvoiceTypeStandard)))
		assert.Equal(t, cbc.Code("381"), documentType(t, scenarioInvoice(l10n.AU.Tax(), bill.InvoiceTypeCreditNote)))
	})
}
