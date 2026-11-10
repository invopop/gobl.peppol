package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
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

// aeInvoice is a skeleton UAE invoice in AED with payment instructions, which
// passes the AE invoice rules. Like baseInvoice, it trips unrelated core rules,
// so the tests check for specific fault codes.
func aeInvoice() *bill.Invoice {
	inv := baseInvoice()
	inv.Type = bill.InvoiceTypeStandard
	inv.Currency = currency.AED
	inv.Payment = &bill.PaymentDetails{
		Instructions: &pay.Instructions{Key: pay.MeansKeyCreditTransfer},
	}
	return inv
}

func usdToAED(amount num.Amount) []*currency.ExchangeRate {
	return []*currency.ExchangeRate{{From: currency.USD, To: currency.AED, Amount: amount}}
}

func TestBillInvoiceRulesAE(t *testing.T) {
	const (
		faultRate     = "GOBL-PEPPOL-PINT-BILL-INVOICE-05"
		faultDecimals = "GOBL-PEPPOL-PINT-BILL-INVOICE-06"
		faultPayment  = "GOBL-PEPPOL-PINT-BILL-INVOICE-07"
	)
	validateIn := func(inv *bill.Invoice, regime l10n.TaxCountryCode) string {
		err := rules.Validate(inv, tax.AddonContext(pint.V1), tax.RegimeContext(regime))
		if err == nil {
			return ""
		}
		return err.Error()
	}
	validate := func(inv *bill.Invoice) string {
		return validateIn(inv, l10n.AE.Tax())
	}

	t.Run("valid AE invoice raises no AE invoice faults", func(t *testing.T) {
		out := validate(aeInvoice())
		assert.NotContains(t, out, faultRate)
		assert.NotContains(t, out, faultDecimals)
		assert.NotContains(t, out, faultPayment)
	})

	t.Run("AED invoice needs no exchange rate (IBR-159-AE)", func(t *testing.T) {
		inv := aeInvoice()
		inv.ExchangeRates = nil
		assert.NotContains(t, validate(inv), faultRate)
	})

	t.Run("non-AED invoice with a rate to AED is accepted (IBR-159-AE)", func(t *testing.T) {
		inv := aeInvoice()
		inv.Currency = currency.USD
		inv.ExchangeRates = usdToAED(num.MakeAmount(3672500, 6))
		assert.NotContains(t, validate(inv), faultRate)
	})

	t.Run("non-AED invoice without a rate is rejected (IBR-159-AE)", func(t *testing.T) {
		inv := aeInvoice()
		inv.Currency = currency.USD
		assert.Contains(t, validate(inv), faultRate)
	})

	t.Run("non-AED invoice with a rate to another currency is rejected (IBR-159-AE)", func(t *testing.T) {
		inv := aeInvoice()
		inv.Currency = currency.USD
		inv.ExchangeRates = []*currency.ExchangeRate{
			{From: currency.USD, To: currency.EUR, Amount: num.MakeAmount(92, 2)},
		}
		assert.Contains(t, validate(inv), faultRate)
	})

	t.Run("exchange rate with 6 decimals is accepted (IBR-002-AE)", func(t *testing.T) {
		inv := aeInvoice()
		inv.Currency = currency.USD
		inv.ExchangeRates = usdToAED(num.MakeAmount(3672500, 6))
		assert.NotContains(t, validate(inv), faultDecimals)
	})

	t.Run("exchange rate with 7 decimals is rejected (IBR-002-AE)", func(t *testing.T) {
		inv := aeInvoice()
		inv.Currency = currency.USD
		inv.ExchangeRates = usdToAED(num.MakeAmount(36725000, 7))
		assert.Contains(t, validate(inv), faultDecimals)
	})

	t.Run("invoice without payment is rejected (IBR-191-AE)", func(t *testing.T) {
		inv := aeInvoice()
		inv.Payment = nil
		assert.Contains(t, validate(inv), faultPayment)
	})

	t.Run("invoice with payment but no instructions is rejected (IBR-191-AE)", func(t *testing.T) {
		inv := aeInvoice()
		inv.Payment = &bill.PaymentDetails{}
		assert.Contains(t, validate(inv), faultPayment)
	})

	t.Run("credit note without payment is accepted (IBR-191-AE)", func(t *testing.T) {
		inv := aeInvoice()
		inv.Type = bill.InvoiceTypeCreditNote
		inv.Payment = nil
		assert.NotContains(t, validate(inv), faultPayment)
	})

	t.Run("AE invoice rules do not apply outside AE", func(t *testing.T) {
		inv := aeInvoice()
		inv.Currency = currency.USD
		inv.ExchangeRates = usdToAED(num.MakeAmount(36725000, 7))
		inv.Payment = nil
		for _, regime := range []l10n.TaxCountryCode{l10n.SG.Tax(), l10n.AU.Tax()} {
			out := validateIn(inv, regime)
			assert.NotContains(t, out, faultRate, "regime %s", regime)
			assert.NotContains(t, out, faultDecimals, "regime %s", regime)
			assert.NotContains(t, out, faultPayment, "regime %s", regime)
		}
	})
}
