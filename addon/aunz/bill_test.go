package aunz_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/aunz"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func legalIdentity(scheme, code string) *org.Identity {
	return &org.Identity{
		Scope: org.IdentityScopeLegal,
		Code:  cbc.Code(code),
		Ext:   tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: cbc.Code(scheme)}),
	}
}

func auParty(name, code string) *org.Party {
	return &org.Party{
		Name:       name,
		TaxID:      &tax.Identity{Country: "AU", Code: cbc.Code(code)},
		Identities: []*org.Identity{legalIdentity("0151", code)},
		Addresses:  []*org.Address{{Country: "AU"}},
	}
}

func nzParty(name, businessNo string) *org.Party {
	return &org.Party{
		Name:       name,
		TaxID:      &tax.Identity{Country: "NZ", Code: "49091850"},
		Identities: []*org.Identity{legalIdentity("0088", businessNo)},
		Addresses:  []*org.Address{{Country: "NZ"}},
	}
}

func TestBillInvoiceRules(t *testing.T) {
	validate := func(inv *bill.Invoice) error {
		return rules.Validate(inv, tax.AddonContext(aunz.V1))
	}

	t.Run("AU parties with ABN raise no jurisdiction faults", func(t *testing.T) {
		inv := &bill.Invoice{
			Supplier: auParty("Seller Pty Ltd", "51824753556"),
			Customer: auParty("Buyer Pty Ltd", "53004085616"),
		}
		if err := validate(inv); err != nil {
			assert.NotContains(t, err.Error(), "ABN legal identity")
			assert.NotContains(t, err.Error(), "NZBN legal identity")
		}
	})

	t.Run("NZ parties with NZBN raise no jurisdiction faults", func(t *testing.T) {
		inv := &bill.Invoice{
			Supplier: nzParty("Seller Limited", "9429000000000"),
			Customer: nzParty("Buyer Limited", "9429000000001"),
		}
		if err := validate(inv); err != nil {
			assert.NotContains(t, err.Error(), "ABN legal identity")
			assert.NotContains(t, err.Error(), "NZBN legal identity")
		}
	})

	t.Run("AU supplier without ABN is rejected (IBR-001)", func(t *testing.T) {
		inv := &bill.Invoice{Supplier: auParty("Seller Pty Ltd", "51824753556")}
		inv.Supplier.Identities = nil
		err := validate(inv)
		assert.ErrorContains(t, err, "supplier established in Australia must have an ABN legal identity")
	})

	t.Run("NZ supplier without NZBN is rejected (IBR-002)", func(t *testing.T) {
		inv := &bill.Invoice{Supplier: nzParty("Seller Limited", "9429000000000")}
		inv.Supplier.Identities = nil
		err := validate(inv)
		assert.ErrorContains(t, err, "supplier established in New Zealand must have an NZBN legal identity")
	})

	t.Run("AU customer without ABN is rejected (IBR-004)", func(t *testing.T) {
		inv := &bill.Invoice{
			Supplier: auParty("Seller Pty Ltd", "51824753556"),
			Customer: auParty("Buyer Pty Ltd", "53004085616"),
		}
		inv.Customer.Identities = nil
		err := validate(inv)
		assert.ErrorContains(t, err, "customer established in Australia must have an ABN legal identity")
	})

	t.Run("NZ customer without NZBN is rejected (IBR-005)", func(t *testing.T) {
		inv := &bill.Invoice{
			Supplier: nzParty("Seller Limited", "9429000000000"),
			Customer: nzParty("Buyer Limited", "9429000000001"),
		}
		inv.Customer.Identities = nil
		err := validate(inv)
		assert.ErrorContains(t, err, "customer established in New Zealand must have an NZBN legal identity")
	})

	t.Run("non-A-NZ supplier is exempt from the business-number rules", func(t *testing.T) {
		inv := &bill.Invoice{Supplier: &org.Party{
			Name:      "Overseas Inc",
			TaxID:     &tax.Identity{Country: "US"},
			Addresses: []*org.Address{{Country: "US"}},
		}}
		if err := validate(inv); err != nil {
			assert.NotContains(t, err.Error(), "must have an ABN legal identity")
			assert.NotContains(t, err.Error(), "must have an NZBN legal identity")
		}
	})
}
