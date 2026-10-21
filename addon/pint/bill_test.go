package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func validEndpoints(code string) []*org.Endpoint {
	return []*org.Endpoint{{URI: cbc.URI("iso6523-actorid-upis::0151:" + code)}}
}

func baseInvoice() *bill.Invoice {
	return &bill.Invoice{
		Supplier: &org.Party{
			Name:      "Seller Pty Ltd",
			TaxID:     &tax.Identity{Country: "AU", Code: "51824753556"},
			Endpoints: validEndpoints("51824753556"),
		},
		Customer: &org.Party{
			Name:      "Buyer Pty Ltd",
			TaxID:     &tax.Identity{Country: "AU", Code: "53004085616"},
			Endpoints: validEndpoints("53004085616"),
		},
	}
}

func TestBillInvoiceRules(t *testing.T) {
	validate := func(inv *bill.Invoice) error {
		return rules.Validate(inv, tax.AddonContext(pint.V1))
	}

	t.Run("valid parties raise no PINT party faults", func(t *testing.T) {
		// The skeleton invoice trips unrelated core rules; assert only that
		// none of the PINT party rules fired. The full happy path is covered
		// by the golden example invoices.
		err := validate(baseInvoice())
		if err != nil {
			assert.NotContains(t, err.Error(), "customer is required")
			assert.NotContains(t, err.Error(), "electronic address is required")
			assert.NotContains(t, err.Error(), "tax ID or identity")
		}
	})

	t.Run("customer is required (ibr-007)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Customer = nil
		err := validate(inv)
		assert.ErrorContains(t, err, "customer is required")
	})

	t.Run("supplier electronic address is required (ibr-081)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Supplier.Endpoints = nil
		err := validate(inv)
		assert.ErrorContains(t, err, "supplier electronic address is required")
	})

	t.Run("customer electronic address is required (ibr-080)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Customer.Endpoints = nil
		err := validate(inv)
		assert.ErrorContains(t, err, "customer electronic address is required")
	})

	t.Run("supplier must be identifiable (ibr-co-26)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Supplier.TaxID = nil
		inv.Supplier.Identities = nil
		err := validate(inv)
		assert.ErrorContains(t, err, "supplier must have a tax ID or identity")
	})

	t.Run("supplier identity alone is enough (ibr-co-26)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Supplier.TaxID = nil
		inv.Supplier.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "51824753556"}}
		err := validate(inv)
		if err != nil {
			assert.NotContains(t, err.Error(), "tax ID or identity")
		}
	})
}
