package au_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/au"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func abnIdentity(code string) *org.Identity {
	return &org.Identity{
		Scope: org.IdentityScopeLegal,
		Code:  cbc.Code(code),
		Ext:   tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: "0151"}),
	}
}

func baseInvoice() *bill.Invoice {
	return &bill.Invoice{
		Supplier: &org.Party{
			Name:       "Seller Pty Ltd",
			TaxID:      &tax.Identity{Country: "AU", Code: "51824753556"},
			Identities: []*org.Identity{abnIdentity("51824753556")},
		},
		Customer: &org.Party{
			Name:       "Buyer Pty Ltd",
			TaxID:      &tax.Identity{Country: "AU", Code: "53004085616"},
			Identities: []*org.Identity{abnIdentity("53004085616")},
		},
	}
}

func TestBillInvoiceRules(t *testing.T) {
	validate := func(inv *bill.Invoice) error {
		return rules.Validate(inv, tax.AddonContext(au.V1))
	}

	t.Run("australian parties with ABN raise no AU faults", func(t *testing.T) {
		err := validate(baseInvoice())
		if err != nil {
			assert.NotContains(t, err.Error(), "ABN legal identity")
		}
	})

	t.Run("supplier without ABN legal identity is rejected", func(t *testing.T) {
		inv := baseInvoice()
		inv.Supplier.Identities = nil
		err := validate(inv)
		assert.ErrorContains(t, err, "supplier established in Australia must have an ABN legal identity")
	})

	t.Run("customer without ABN legal identity is rejected", func(t *testing.T) {
		inv := baseInvoice()
		inv.Customer.Identities = nil
		err := validate(inv)
		assert.ErrorContains(t, err, "customer established in Australia must have an ABN legal identity")
	})

	t.Run("non-australian supplier is exempt from the ABN rule", func(t *testing.T) {
		inv := baseInvoice()
		inv.Supplier.TaxID = &tax.Identity{Country: "NZ", Code: "9429036558796"}
		inv.Supplier.Identities = nil
		err := validate(inv)
		if err != nil {
			assert.NotContains(t, err.Error(), "supplier established in Australia must have an ABN legal identity")
		}
	})
}
