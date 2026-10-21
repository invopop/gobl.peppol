package bis_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/bis"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/norm"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func endpoints() []*org.Endpoint {
	return []*org.Endpoint{{URI: "iso6523-actorid-upis::0088:1234567890128"}}
}

func baseInvoice() *bill.Invoice {
	return &bill.Invoice{
		Ordering: &bill.Ordering{Code: "PO-123"},
		Supplier: &org.Party{Name: "Seller", Endpoints: endpoints()},
		Customer: &org.Party{Name: "Buyer", Endpoints: endpoints()},
	}
}

func TestBillInvoiceRules(t *testing.T) {
	validate := func(inv *bill.Invoice) error {
		return rules.Validate(inv, tax.AddonContext(bis.V3))
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
		inv.Supplier.Endpoints = nil
		assert.ErrorContains(t, validate(inv), "supplier electronic address is required")
	})

	t.Run("customer electronic address required (R010)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Customer.Endpoints = nil
		assert.ErrorContains(t, validate(inv), "customer electronic address is required")
	})

	t.Run("non-Peppol endpoint does not satisfy the electronic address (R010)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Customer.Endpoints = []*org.Endpoint{{URI: "mailto:billing@example.com"}}
		assert.ErrorContains(t, validate(inv), "customer electronic address is required")
	})

	t.Run("Peppol inbox is normalized into the electronic address (R010/R020)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Supplier.Endpoints = nil
		inv.Supplier.Inboxes = []*org.Inbox{{Key: "peppol", Scheme: "0088", Code: "1234567890128"}}
		norm.Normalize(inv.Supplier, tax.AddonContext(en16931.V2017))
		if err := validate(inv); err != nil {
			assert.NotContains(t, err.Error(), "supplier electronic address")
		}
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

	t.Run("open-ended periods are compared on the bounds both provide (R110/R111)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Ordering.Period = &cal.Period{Start: cal.NewDate(2024, 1, 1)}
		inv.Lines = []*bill.Line{{Period: &cal.Period{End: cal.NewDate(2024, 2, 5)}}}
		if err := validate(inv); err != nil {
			assert.NotContains(t, err.Error(), "line period must fall within")
		}
		inv.Lines = []*bill.Line{{Period: &cal.Period{Start: cal.NewDate(2023, 12, 1)}}}
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

	t.Run("more than one project reference is rejected (R080)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Ordering.Projects = []*org.DocumentRef{{Code: "P1"}, {Code: "P2"}}
		assert.ErrorContains(t, validate(inv), "more than one project reference")
	})

	t.Run("a single project reference is accepted (R080)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Ordering.Projects = []*org.DocumentRef{{Code: "P1"}}
		if err := validate(inv); err != nil {
			assert.NotContains(t, err.Error(), "project reference")
		}
	})
}
