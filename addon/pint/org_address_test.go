package pint_test

import (
	"testing"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/stretchr/testify/assert"
)

func TestOrgAddressRulesAE(t *testing.T) {
	const fault = "GOBL-PEPPOL-PINT-ORG-ADDRESS-01"
	standard := func() *bill.Invoice { return aeDocument(bill.InvoiceTypeStandard) }

	t.Run("UAE address region must be an emirate code (IBR-128-AE)", func(t *testing.T) {
		for _, region := range []string{"Dubai", "DBX", ""} {
			inv := standard()
			inv.Customer.Addresses[0].Region = region
			assert.Contains(t, aeFaults(t, inv), fault, region)
		}
	})

	t.Run("every emirate code is accepted (IBR-128-AE)", func(t *testing.T) {
		for _, code := range []string{"AUH", "DXB", "SHJ", "UAQ", "FUJ", "AJM", "RAK"} {
			inv := standard()
			inv.Supplier.Addresses[0].Region = code
			assert.NotContains(t, aeFaults(t, inv), fault, code)
		}
	})

	t.Run("every party's UAE address is checked (IBR-128-AE)", func(t *testing.T) {
		for name, set := range map[string]func(inv *bill.Invoice, p *org.Party){
			"tax representative": func(inv *bill.Invoice, p *org.Party) { inv.Ordering = &bill.Ordering{Seller: p} },
			"issuer":             func(inv *bill.Invoice, p *org.Party) { inv.Ordering = &bill.Ordering{Issuer: p} },
			"line seller":        func(inv *bill.Invoice, p *org.Party) { inv.Lines[0].Seller = p },
			"delivery receiver":  func(inv *bill.Invoice, p *org.Party) { inv.Delivery = &bill.DeliveryDetails{Receiver: p} },
			"payee":              func(inv *bill.Invoice, p *org.Party) { inv.Payment = &bill.PaymentDetails{Payee: p} },
		} {
			inv := standard()
			set(inv, &org.Party{Name: "Party", Addresses: aeAddress("Dubai")})
			assert.Contains(t, aeFaults(t, inv), fault, name)

			inv = standard()
			set(inv, &org.Party{Name: "Party", Addresses: aeAddress("DXB")})
			assert.NotContains(t, aeFaults(t, inv), fault, name)
		}
	})

	t.Run("bank branch UAE address is checked too (IBR-128-AE)", func(t *testing.T) {
		inv := standard()
		inv.Payment = &bill.PaymentDetails{Instructions: &pay.Instructions{
			Key: pay.MeansKeyCreditTransfer,
			CreditTransfer: []*pay.CreditTransfer{{
				IBAN:   "AE070331234567890123456",
				Branch: &org.Address{Locality: "Dubai", Region: "Dubai", Country: "AE"},
			}},
		}}
		assert.Contains(t, aeFaults(t, inv), fault)
	})

	t.Run("foreign address region is not checked (IBR-128-AE)", func(t *testing.T) {
		inv := standard()
		inv.Customer.TaxID = nil
		inv.Customer.Addresses = []*org.Address{{Street: "Hauptstr. 1", Locality: "Berlin", Region: "Berlin", Country: "DE"}}
		assert.NotContains(t, aeFaults(t, inv), fault)
	})
}
