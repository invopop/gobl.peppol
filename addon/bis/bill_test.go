package bis_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/bis"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func inboxes() []*org.Inbox {
	return []*org.Inbox{{Key: "peppol", Scheme: "0088", Code: "1234567890128"}}
}

func baseInvoice() *bill.Invoice {
	return &bill.Invoice{
		Ordering: &bill.Ordering{Code: "PO-123"},
		Supplier: &org.Party{Name: "Seller", Inboxes: inboxes()},
		Customer: &org.Party{Name: "Buyer", Inboxes: inboxes()},
	}
}

func TestBillInvoiceRules(t *testing.T) {
	validate := func(inv *bill.Invoice) error {
		return rules.Validate(inv, tax.AddonContext(bis.V1))
	}

	t.Run("valid invoice raises no BIS faults", func(t *testing.T) {
		if err := validate(baseInvoice()); err != nil {
			assert.NotContains(t, err.Error(), "buyer reference")
			assert.NotContains(t, err.Error(), "electronic address")
			assert.NotContains(t, err.Error(), "document-level note")
			assert.NotContains(t, err.Error(), "line period")
		}
	})

	t.Run("buyer reference or purchase order required (R003)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Ordering = nil
		assert.ErrorContains(t, validate(inv), "buyer reference or a purchase order reference")
	})

	t.Run("supplier electronic address required (R020)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Supplier.Inboxes = nil
		assert.ErrorContains(t, validate(inv), "supplier electronic address is required")
	})

	t.Run("customer electronic address required (R010)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Customer.Inboxes = nil
		assert.ErrorContains(t, validate(inv), "customer electronic address is required")
	})

	t.Run("at most one document-level note (R002)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Notes = []*org.Note{{Text: "first"}, {Text: "second"}}
		assert.ErrorContains(t, validate(inv), "more than one document-level note")
	})

	t.Run("line period outside invoice period is rejected (R110/R111)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Ordering.Period = &cal.Period{Start: cal.NewDate(2024, 1, 1), End: cal.NewDate(2024, 1, 31)}
		inv.Lines = []*bill.Line{{Period: &cal.Period{Start: cal.NewDate(2024, 2, 1), End: cal.NewDate(2024, 2, 5)}}}
		assert.ErrorContains(t, validate(inv), "line period must fall within the invoice period")
	})

	t.Run("line period within invoice period is accepted (R110/R111)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Ordering.Period = &cal.Period{Start: cal.NewDate(2024, 1, 1), End: cal.NewDate(2024, 1, 31)}
		inv.Lines = []*bill.Line{{Period: &cal.Period{Start: cal.NewDate(2024, 1, 5), End: cal.NewDate(2024, 1, 20)}}}
		if err := validate(inv); err != nil {
			assert.NotContains(t, err.Error(), "line period must fall within")
		}
	})

	t.Run("open-ended line period is checked on its start only (R110)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Ordering.Period = &cal.Period{Start: cal.NewDate(2024, 1, 1), End: cal.NewDate(2024, 1, 31)}
		inv.Lines = []*bill.Line{{Period: &cal.Period{Start: cal.NewDate(2023, 12, 20)}}}
		assert.ErrorContains(t, validate(inv), "line period must fall within the invoice period")
	})
}
