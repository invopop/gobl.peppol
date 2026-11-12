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
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// aeDocument is an uncalculated UAE document of the given type and tags that
// passes every PINT AE rule: AE parties with valid TRNs and full addresses, a
// standard-rated line with a description, payment instructions and a due date.
// Out-of-scope documents get an outside-scope line and a customer legal
// identity. Tests change one thing and check for that rule's fault code.
func aeDocument(typ cbc.Key, tags ...cbc.Key) *bill.Invoice {
	inv := scenarioInvoice(l10n.AE.Tax(), typ, tags...)
	inv.Supplier = &org.Party{
		Name:      "Seller LLC",
		TaxID:     &tax.Identity{Country: "AE", Code: "100000000000003"},
		Endpoints: validEndpoints("100000000000003"),
		Addresses: aeAddress("DXB"),
	}
	inv.Customer = &org.Party{
		Name:      "Buyer LLC",
		TaxID:     &tax.Identity{Country: "AE", Code: "100000000000103"},
		Endpoints: validEndpoints("100000000000103"),
		Addresses: aeAddress("AUH"),
	}
	inv.Lines[0].Item.Description = "Consulting services"
	inv.Lines[0].Taxes = tax.Set{standardVAT()}
	if inv.HasTags(pint.TagOutsideScope) {
		inv.Lines[0].Taxes = tax.Set{{Category: tax.CategoryVAT, Key: tax.KeyOutsideScope}}
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "CN-1234567"}}
	}
	if typ == bill.InvoiceTypeCreditNote {
		inv.Preceding = []*org.DocumentRef{{Code: "INV-001"}}
		inv.Tax = &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyCreditNoteReason: "DL8.61.1.E"})}
	}
	due := cal.MakeDate(2026, 12, 10)
	inv.Payment = &bill.PaymentDetails{
		Instructions: &pay.Instructions{
			Key:            pay.MeansKeyCreditTransfer,
			Ext:            tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "30"}),
			CreditTransfer: []*pay.CreditTransfer{{IBAN: "AE070331234567890123456"}},
		},
		Terms: &pay.Terms{
			DueDates: []*pay.DueDate{{Date: &due, Percent: num.NewPercentage(100, 2)}},
		},
	}
	return inv
}

// aeAddress is a complete UAE address in the given emirate.
func aeAddress(emirate string) []*org.Address {
	return []*org.Address{{
		Street:   "Sheikh Zayed Road",
		Locality: "Dubai",
		Region:   emirate,
		Country:  "AE",
	}}
}

// standardVAT returns a standard-rated VAT combo at 5%, the UAE rate.
func standardVAT() *tax.Combo {
	return &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyStandard, Percent: num.NewPercentage(5, 2)}
}

// lineCombo returns a VAT combo with the given key and no rate, for the
// zero, exempt, outside-scope and reverse charge keys.
func lineCombo(key cbc.Key) *tax.Combo {
	return &tax.Combo{Category: tax.CategoryVAT, Key: key}
}

// aeFaults calculates the document, so the scenarios set its document type, and
// returns the faults raised under the AE regime, or "" if there are none.
func aeFaults(t *testing.T, inv *bill.Invoice) string {
	t.Helper()
	require.NoError(t, inv.Calculate())
	err := rules.Validate(inv, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.AE.Tax()))
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestBillDocumentRulesAE(t *testing.T) {
	const (
		faultSellerTRN       = "GOBL-PEPPOL-PINT-BILL-INVOICE-08"
		faultReverseCharge   = "GOBL-PEPPOL-PINT-BILL-INVOICE-09"
		faultBuyerLegalID    = "GOBL-PEPPOL-PINT-BILL-INVOICE-10"
		faultLineCategory    = "GOBL-PEPPOL-PINT-BILL-INVOICE-11"
		faultDiscountCat     = "GOBL-PEPPOL-PINT-BILL-INVOICE-12"
		faultChargeCategory  = "GOBL-PEPPOL-PINT-BILL-INVOICE-13"
		faultOnlyExempt      = "GOBL-PEPPOL-PINT-BILL-INVOICE-14"
		faultCreditPointDate = "GOBL-PEPPOL-PINT-BILL-INVOICE-15"
		faultPointDate       = "GOBL-PEPPOL-PINT-BILL-INVOICE-16"
		faultDueDate         = "GOBL-PEPPOL-PINT-BILL-INVOICE-17"
	)
	standard := func() *bill.Invoice { return aeDocument(bill.InvoiceTypeStandard) }
	outsideScope := func() *bill.Invoice {
		return aeDocument(bill.InvoiceTypeStandard, pint.TagOutsideScope)
	}
	outsideScopeCredit := func() *bill.Invoice {
		return aeDocument(bill.InvoiceTypeCreditNote, pint.TagOutsideScope)
	}

	t.Run("valid documents raise no PINT faults", func(t *testing.T) {
		for name, inv := range map[string]*bill.Invoice{
			"380": standard(),
			"381": aeDocument(bill.InvoiceTypeCreditNote),
			"480": outsideScope(),
			"81":  outsideScopeCredit(),
		} {
			assert.NotContains(t, aeFaults(t, inv), "GOBL-PEPPOL-PINT", name)
		}
	})

	t.Run("seller TRN is required (IBR-134-AE)", func(t *testing.T) {
		inv := standard()
		inv.Supplier.TaxID = &tax.Identity{Country: "AE"}
		assert.Contains(t, aeFaults(t, inv), faultSellerTRN)
	})

	t.Run("seller TRN is not required on 480 and 81 (IBR-134-AE)", func(t *testing.T) {
		for _, inv := range []*bill.Invoice{outsideScope(), outsideScopeCredit()} {
			inv.Supplier.TaxID = &tax.Identity{Country: "AE"}
			assert.NotContains(t, aeFaults(t, inv), faultSellerTRN)
		}
	})

	t.Run("reverse charge needs a buyer tax registration (IBR-103-AE)", func(t *testing.T) {
		inv := standard()
		inv.Lines[0].Taxes = tax.Set{lineCombo(tax.KeyReverseCharge)}
		inv.Customer.TaxID = nil
		assert.Contains(t, aeFaults(t, inv), faultReverseCharge)
	})

	t.Run("reverse charge accepts a tax scope identity (IBR-103-AE)", func(t *testing.T) {
		inv := standard()
		inv.Lines[0].Taxes = tax.Set{lineCombo(tax.KeyReverseCharge)}
		inv.Customer.TaxID = nil
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeTax, Code: "100000000000103"}}
		assert.NotContains(t, aeFaults(t, inv), faultReverseCharge)
	})

	t.Run("buyer tax registration is not required without reverse charge (IBR-103-AE)", func(t *testing.T) {
		inv := standard()
		inv.Customer.TaxID = nil
		assert.NotContains(t, aeFaults(t, inv), faultReverseCharge)
	})

	t.Run("480 and 81 need a buyer legal identity (IBR-136-AE)", func(t *testing.T) {
		for _, inv := range []*bill.Invoice{outsideScope(), outsideScopeCredit()} {
			inv.Customer.Identities = nil
			assert.Contains(t, aeFaults(t, inv), faultBuyerLegalID)
		}
	})

	t.Run("a tax scope identity is not a legal identity (IBR-136-AE)", func(t *testing.T) {
		inv := outsideScope()
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeTax, Code: "100000000000103"}}
		assert.Contains(t, aeFaults(t, inv), faultBuyerLegalID)
	})

	t.Run("buyer legal identity is not required on 380 (IBR-136-AE)", func(t *testing.T) {
		assert.NotContains(t, aeFaults(t, standard()), faultBuyerLegalID)
	})

	t.Run("480 and 81 accept E, Z and O lines (IBR-122-AE)", func(t *testing.T) {
		for _, key := range []cbc.Key{tax.KeyExempt, tax.KeyZero, tax.KeyOutsideScope} {
			inv := outsideScope()
			inv.Lines[0].Taxes = tax.Set{lineCombo(key)}
			assert.NotContains(t, aeFaults(t, inv), faultLineCategory, key)
		}
	})

	t.Run("480 and 81 reject standard rated lines (IBR-122-AE)", func(t *testing.T) {
		for _, inv := range []*bill.Invoice{outsideScope(), outsideScopeCredit()} {
			inv.Lines[0].Taxes = tax.Set{standardVAT()}
			assert.Contains(t, aeFaults(t, inv), faultLineCategory)
		}
	})

	t.Run("480 rejects standard rated discounts and charges (IBR-122-AE)", func(t *testing.T) {
		inv := outsideScope()
		inv.Discounts = []*bill.Discount{{
			Reason: "Loyalty", Amount: num.MakeAmount(1000, 2),
			Taxes: tax.Set{standardVAT()},
		}}
		inv.Charges = []*bill.Charge{{
			Reason: "Shipping", Amount: num.MakeAmount(1000, 2),
			Taxes: tax.Set{standardVAT()},
		}}
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultDiscountCat)
		assert.Contains(t, out, faultChargeCategory)
	})

	t.Run("380 rejects only exempt and outside scope lines (IBR-151-AE)", func(t *testing.T) {
		inv := standard()
		inv.Lines[0].Taxes = tax.Set{lineCombo(tax.KeyExempt)}
		inv.Lines = append(inv.Lines, &bill.Line{
			Quantity: num.MakeAmount(1, 0),
			Item:     &org.Item{Name: "Other", Description: "Other", Price: num.NewAmount(100, 2)},
			Taxes:    tax.Set{lineCombo(tax.KeyOutsideScope)},
		})
		assert.Contains(t, aeFaults(t, inv), faultOnlyExempt)
	})

	t.Run("380 accepts a zero rated line (IBR-151-AE)", func(t *testing.T) {
		inv := standard()
		inv.Lines[0].Taxes = tax.Set{lineCombo(tax.KeyZero)}
		assert.NotContains(t, aeFaults(t, inv), faultOnlyExempt)
	})

	t.Run("381 and 81 reject a VAT point date (IBR-124-AE)", func(t *testing.T) {
		for _, inv := range []*bill.Invoice{aeDocument(bill.InvoiceTypeCreditNote), outsideScopeCredit()} {
			vd := cal.MakeDate(2026, 11, 1)
			inv.ValueDate = &vd
			assert.Contains(t, aeFaults(t, inv), faultCreditPointDate)
		}
	})

	t.Run("380 accepts a VAT point date before the issue date (IBR-124-AE, IBR-141-AE)", func(t *testing.T) {
		inv := standard()
		vd := cal.MakeDate(2026, 11, 1)
		inv.ValueDate = &vd
		out := aeFaults(t, inv)
		assert.NotContains(t, out, faultCreditPointDate)
		assert.NotContains(t, out, faultPointDate)
	})

	t.Run("VAT point date on or after the issue date is rejected (IBR-141-AE)", func(t *testing.T) {
		for _, day := range []int{10, 20} {
			inv := standard()
			vd := cal.MakeDate(2026, 11, day)
			inv.ValueDate = &vd
			assert.Contains(t, aeFaults(t, inv), faultPointDate, "day %d", day)
		}
	})

	t.Run("due date is required when an amount is due (IBR-127-AE)", func(t *testing.T) {
		inv := standard()
		inv.Payment.Terms = nil
		assert.Contains(t, aeFaults(t, inv), faultDueDate)

		inv = standard()
		inv.Payment = nil
		assert.Contains(t, aeFaults(t, inv), faultDueDate)
	})

	t.Run("due date is not required when nothing is due (IBR-127-AE)", func(t *testing.T) {
		inv := standard()
		inv.Payment.Terms = nil
		inv.Payment.Advances = []*pay.Record{{Description: "Paid in full", Percent: num.NewPercentage(100, 2)}}
		assert.NotContains(t, aeFaults(t, inv), faultDueDate)
	})

	t.Run("due date is not required on 381, 81 and 261 (IBR-127-AE)", func(t *testing.T) {
		for name, inv := range map[string]*bill.Invoice{
			"381": aeDocument(bill.InvoiceTypeCreditNote),
			"81":  outsideScopeCredit(),
			"261": aeDocument(bill.InvoiceTypeCreditNote, tax.TagSelfBilled),
		} {
			inv.Payment.Terms = nil
			assert.NotContains(t, aeFaults(t, inv), faultDueDate, name)
		}
	})

	t.Run("document rules do not apply outside AE", func(t *testing.T) {
		inv := scenarioInvoice(l10n.AU.Tax(), bill.InvoiceTypeStandard)
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.AU.Tax()))
		require.Error(t, err)
		for _, code := range []string{faultSellerTRN, faultDueDate, faultOnlyExempt} {
			assert.NotContains(t, err.Error(), code)
		}
	})
}

func TestBillAddressRulesAE(t *testing.T) {
	const (
		faultSupplierAddress = "GOBL-PEPPOL-PINT-BILL-INVOICE-18"
		faultSupplierEmirate = "GOBL-PEPPOL-PINT-BILL-INVOICE-19"
		faultCustomerAddress = "GOBL-PEPPOL-PINT-BILL-INVOICE-20"
		faultCustomerEmirate = "GOBL-PEPPOL-PINT-BILL-INVOICE-21"
		faultTaxRepEmirate   = "GOBL-PEPPOL-PINT-BILL-INVOICE-22"
	)
	standard := func() *bill.Invoice { return aeDocument(bill.InvoiceTypeStandard) }

	t.Run("supplier address needs street, city and region (IBR-143-AE)", func(t *testing.T) {
		for _, clear := range []func(a *org.Address){
			func(a *org.Address) { a.Street = "" },
			func(a *org.Address) { a.Locality = "" },
			func(a *org.Address) { a.Region = "" },
		} {
			inv := standard()
			clear(inv.Supplier.Addresses[0])
			assert.Contains(t, aeFaults(t, inv), faultSupplierAddress)
		}
	})

	t.Run("customer address needs street, city and region (IBR-144-AE)", func(t *testing.T) {
		inv := standard()
		inv.Customer.Addresses[0].Locality = ""
		assert.Contains(t, aeFaults(t, inv), faultCustomerAddress)
	})

	t.Run("foreign customer address also needs a region (IBR-144-AE)", func(t *testing.T) {
		inv := standard()
		inv.Customer.Addresses = []*org.Address{{Street: "Hauptstr. 1", Locality: "Berlin", Country: "DE"}}
		assert.Contains(t, aeFaults(t, inv), faultCustomerAddress)
	})

	t.Run("only the first address is checked (IBR-143-AE, IBR-128-AE)", func(t *testing.T) {
		inv := standard()
		inv.Supplier.Addresses = append(inv.Supplier.Addresses, &org.Address{Country: "AE", Region: "Dubai"})
		out := aeFaults(t, inv)
		assert.NotContains(t, out, faultSupplierAddress)
		assert.NotContains(t, out, faultSupplierEmirate)
	})

	t.Run("party without addresses is left to EN 16931 (BR-8, BR-10)", func(t *testing.T) {
		inv := standard()
		inv.Customer.Addresses = nil
		out := aeFaults(t, inv)
		assert.Contains(t, out, "BR-10")
		assert.NotContains(t, out, faultCustomerAddress)
		assert.NotContains(t, out, faultCustomerEmirate)
	})

	t.Run("UAE address region must be an emirate code (IBR-128-AE)", func(t *testing.T) {
		inv := standard()
		inv.Supplier.Addresses[0].Region = "Dubai"
		inv.Customer.Addresses[0].Region = "DBX"
		inv.Ordering = &bill.Ordering{Seller: &org.Party{Name: "Tax Rep", Addresses: aeAddress("XXX")}}
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultSupplierEmirate)
		assert.Contains(t, out, faultCustomerEmirate)
		assert.Contains(t, out, faultTaxRepEmirate)
	})

	t.Run("every emirate code is accepted (IBR-128-AE)", func(t *testing.T) {
		for _, code := range []string{"AUH", "DXB", "SHJ", "UAQ", "FUJ", "AJM", "RAK"} {
			inv := standard()
			inv.Supplier.Addresses[0].Region = code
			assert.NotContains(t, aeFaults(t, inv), faultSupplierEmirate, code)
		}
	})

	t.Run("foreign address region is not checked (IBR-128-AE)", func(t *testing.T) {
		inv := standard()
		inv.Customer.Addresses = []*org.Address{{Street: "Hauptstr. 1", Locality: "Berlin", Region: "Berlin", Country: "DE"}}
		assert.NotContains(t, aeFaults(t, inv), faultCustomerEmirate)
	})
}

func uaeEndpoints(code string) []*org.Endpoint {
	return []*org.Endpoint{{URI: cbc.URI("iso6523-actorid-upis::0235:" + code)}}
}

func tradeLicense(code string) *org.Identity {
	return &org.Identity{Key: ae.IdentityKeyTradeLicense, Code: cbc.Code(code), Description: "Dubai Economy and Tourism"}
}

// aeIdentityDocument returns a valid UAE invoice whose parties have UAE Peppol
// endpoints and trade licences, the case the legal registration rules apply to.
func aeIdentityDocument() *bill.Invoice {
	inv := aeDocument(bill.InvoiceTypeStandard)
	inv.Supplier.Endpoints = uaeEndpoints("1357902468")
	inv.Supplier.Identities = []*org.Identity{tradeLicense("112345678900003")}
	inv.Customer.Endpoints = uaeEndpoints("1345678901")
	inv.Customer.Identities = []*org.Identity{tradeLicense("112345679000001")}
	return inv
}

func TestBillIdentityRulesAE(t *testing.T) {
	const (
		faultSupplierLegalRequired = "GOBL-PEPPOL-PINT-BILL-INVOICE-23"
		faultSupplierLegalType     = "GOBL-PEPPOL-PINT-BILL-INVOICE-24"
		faultSupplierPassport      = "GOBL-PEPPOL-PINT-BILL-INVOICE-25"
		faultSupplierAuthority     = "GOBL-PEPPOL-PINT-BILL-INVOICE-26"
		faultSupplierTaxIdentities = "GOBL-PEPPOL-PINT-BILL-INVOICE-27"
		faultSupplierTIN           = "GOBL-PEPPOL-PINT-BILL-INVOICE-28"
		faultCustomerLegalType     = "GOBL-PEPPOL-PINT-BILL-INVOICE-29"
		faultCustomerPassport      = "GOBL-PEPPOL-PINT-BILL-INVOICE-30"
		faultCustomerAuthority     = "GOBL-PEPPOL-PINT-BILL-INVOICE-31"
		faultCustomerTaxRequired   = "GOBL-PEPPOL-PINT-BILL-INVOICE-32"
		faultCustomerTaxCount      = "GOBL-PEPPOL-PINT-BILL-INVOICE-33"
		faultCustomerTaxSupplier   = "GOBL-PEPPOL-PINT-BILL-INVOICE-34"
	)

	t.Run("trade licences with authorities pass", func(t *testing.T) {
		assert.Empty(t, aeFaults(t, aeIdentityDocument()))
	})

	t.Run("supplier with a UAE endpoint needs a legal identity (IBR-150-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = nil
		assert.Contains(t, aeFaults(t, inv), faultSupplierLegalRequired)
	})

	t.Run("supplier without a UAE endpoint needs no legal identity (IBR-150-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = nil
		inv.Supplier.Endpoints = validEndpoints("1357902468")
		assert.NotContains(t, aeFaults(t, inv), faultSupplierLegalRequired)
	})

	t.Run("UAE endpoint in an inbox counts too (IBR-150-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = nil
		inv.Supplier.Endpoints = nil
		inv.Supplier.Inboxes = []*org.Inbox{{Scheme: "0235", Code: "1357902468"}}
		assert.Contains(t, aeFaults(t, inv), faultSupplierLegalRequired)
	})

	t.Run("legal identity needs a PINT AE type with a UAE endpoint (IBR-173-AE, IBR-181-AE, IBR-180-AE, IBR-183-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "CN-1234567"}}
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "CN-7654321"}}
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultSupplierLegalType)
		assert.Contains(t, out, faultCustomerLegalType)
	})

	t.Run("legal identity without a type is fine without a UAE endpoint", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Endpoints = validEndpoints("1357902468")
		inv.Supplier.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "CN-1234567"}}
		inv.Customer.Endpoints = validEndpoints("1345678901")
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "CN-7654321"}}
		out := aeFaults(t, inv)
		assert.NotContains(t, out, faultSupplierLegalType)
		assert.NotContains(t, out, faultCustomerLegalType)
	})

	t.Run("each PINT AE type is accepted (IBR-173-AE, IBR-183-AE)", func(t *testing.T) {
		for _, id := range []*org.Identity{
			{Key: ae.IdentityKeyEmiratesID, Code: "784199012345671"},
			{Key: org.IdentityKeyPassport, Code: "P1234567890", Country: "IN"},
			{Key: ae.IdentityKeyCabinetDecision, Code: "CD-12-2020"},
		} {
			inv := aeIdentityDocument()
			inv.Supplier.Identities = []*org.Identity{id}
			assert.Empty(t, aeFaults(t, inv), id.Key)
		}
	})

	t.Run("only the first legal identity is checked (IBR-173-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = append(inv.Supplier.Identities,
			&org.Identity{Scope: org.IdentityScopeLegal, Code: "CN-1234567"})
		assert.NotContains(t, aeFaults(t, inv), faultSupplierLegalType)
	})

	t.Run("passport needs the issuing country (IBR-012-AE, IBR-010-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = []*org.Identity{{Key: org.IdentityKeyPassport, Code: "P9876543210"}}
		inv.Customer.Identities = []*org.Identity{{Key: org.IdentityKeyPassport, Code: "P1234567890"}}
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultSupplierPassport)
		assert.Contains(t, out, faultCustomerPassport)
	})

	t.Run("trade licence needs the issuing authority (IBR-172-AE, IBR-101-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities[0].Description = ""
		inv.Customer.Identities[0].Description = ""
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultSupplierAuthority)
		assert.Contains(t, out, faultCustomerAuthority)
	})

	t.Run("supplier TIN is accepted, before or after the legal identity (IBR-148-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = append([]*org.Identity{
			{Key: ae.IdentityKeyTIN, Code: "1234567890"},
		}, inv.Supplier.Identities...)
		assert.Empty(t, aeFaults(t, inv))
	})

	t.Run("supplier tax identity must be a TIN starting with 1 (IBR-148-AE)", func(t *testing.T) {
		for _, code := range []cbc.Code{"2234567890", "123456789"} {
			inv := aeIdentityDocument()
			inv.Supplier.Identities = append(inv.Supplier.Identities,
				&org.Identity{Scope: org.IdentityScopeTax, Type: "TIN", Code: code})
			assert.Contains(t, aeFaults(t, inv), faultSupplierTIN, code)
		}
	})

	t.Run("supplier has at most one tax identity (IBR-178-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = append(inv.Supplier.Identities,
			&org.Identity{Key: ae.IdentityKeyTIN, Code: "1234567890"},
			&org.Identity{Scope: org.IdentityScopeTax, Type: "TIN", Code: "1234567891"})
		assert.Contains(t, aeFaults(t, inv), faultSupplierTaxIdentities)
	})

	t.Run("customer with a UAE endpoint not starting with 1 or 9 needs a tax registration (IBR-149-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Customer.TaxID = nil
		inv.Customer.Endpoints = uaeEndpoints("2345678901")
		assert.Contains(t, aeFaults(t, inv), faultCustomerTaxRequired)
	})

	t.Run("customer with a UAE endpoint starting with 1 or 9 needs no tax registration (IBR-149-AE)", func(t *testing.T) {
		for _, code := range []string{"1345678901", "9345678901"} {
			inv := aeIdentityDocument()
			inv.Customer.TaxID = nil
			inv.Customer.Endpoints = uaeEndpoints(code)
			assert.NotContains(t, aeFaults(t, inv), faultCustomerTaxRequired, code)
		}
	})

	t.Run("customer has at most one tax registration (IBR-179-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = append(inv.Supplier.Identities,
			&org.Identity{Key: ae.IdentityKeyTIN, Code: "1234567890"})
		inv.Customer.Identities = append(inv.Customer.Identities,
			&org.Identity{Key: ae.IdentityKeyTIN, Code: "1345678901"})
		assert.Contains(t, aeFaults(t, inv), faultCustomerTaxCount)
	})

	t.Run("customer tax identity needs a supplier TIN (IBR-133-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Customer.TaxID = nil
		inv.Customer.Identities = append(inv.Customer.Identities,
			&org.Identity{Key: ae.IdentityKeyTIN, Code: "1345678901"})
		assert.Contains(t, aeFaults(t, inv), faultCustomerTaxSupplier)

		inv.Supplier.Identities = append(inv.Supplier.Identities,
			&org.Identity{Key: ae.IdentityKeyTIN, Code: "1234567890"})
		assert.NotContains(t, aeFaults(t, inv), faultCustomerTaxSupplier)
	})
}

func TestBillTransactionTypeRulesAE(t *testing.T) {
	const (
		faultPaymentInstructions = "GOBL-PEPPOL-PINT-BILL-INVOICE-07"
		faultDueDate             = "GOBL-PEPPOL-PINT-BILL-INVOICE-17"
		faultTransactionType     = "GOBL-PEPPOL-PINT-BILL-INVOICE-35"
		faultBeneficiary         = "GOBL-PEPPOL-PINT-BILL-INVOICE-36"
		faultSummaryPeriod       = "GOBL-PEPPOL-PINT-BILL-INVOICE-37"
		faultPrincipal           = "GOBL-PEPPOL-PINT-BILL-INVOICE-38"
		faultPrincipalSupplier   = "GOBL-PEPPOL-PINT-BILL-INVOICE-39"
		faultAgentSupplierTax    = "GOBL-PEPPOL-PINT-BILL-INVOICE-40"
		faultECommerceDelivery   = "GOBL-PEPPOL-PINT-BILL-INVOICE-41"
		faultExportDelivery      = "GOBL-PEPPOL-PINT-BILL-INVOICE-42"
		faultOutsideScopeFlags   = "GOBL-PEPPOL-PINT-BILL-INVOICE-43"
		faultCustomerIdentifier  = "GOBL-PEPPOL-PINT-BILL-INVOICE-44"
	)
	setExt := func(inv *bill.Invoice, key cbc.Key, code cbc.Code) {
		if inv.Tax == nil {
			inv.Tax = new(bill.Tax)
		}
		inv.Tax.Ext = inv.Tax.Ext.Set(key, code)
	}
	receiver := func(a *org.Address) *bill.DeliveryDetails {
		return &bill.DeliveryDetails{Receiver: &org.Party{Name: "Receiver", Addresses: []*org.Address{a}}}
	}
	foreign := &org.Address{Street: "Baker Street 221B", Locality: "London", Region: "Greater London", Country: "GB"}

	t.Run("transaction type is required (IBR-154-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		require.NoError(t, inv.Calculate())
		inv.Tax.Ext = inv.Tax.Ext.Delete(pint.ExtKeyTransactionType)
		err := rules.Validate(inv, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.AE.Tax()))
		require.Error(t, err)
		assert.Contains(t, err.Error(), faultTransactionType)
	})

	t.Run("transaction type must be 8 flags (IBR-154-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		setExt(inv, pint.ExtKeyTransactionType, "0000002")
		assert.Contains(t, aeFaults(t, inv), faultTransactionType)
	})

	t.Run("deemed supplies need no payment instructions or due date (IBR-191-AE, IBR-127-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard, pint.TagDeemedSupply)
		inv.Payment = &bill.PaymentDetails{Terms: &pay.Terms{Notes: "No consideration"}}
		out := aeFaults(t, inv)
		assert.NotContains(t, out, faultPaymentInstructions)
		assert.NotContains(t, out, faultDueDate)

		inv = aeDocument(bill.InvoiceTypeStandard)
		inv.Payment = &bill.PaymentDetails{Terms: &pay.Terms{Notes: "No consideration"}}
		out = aeFaults(t, inv)
		assert.Contains(t, out, faultPaymentInstructions)
		assert.Contains(t, out, faultDueDate)
	})

	t.Run("free trade zone needs the beneficiary ID (IBR-007-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard, pint.TagFreeTradeZone)
		assert.Contains(t, aeFaults(t, inv), faultBeneficiary)
		setExt(inv, pint.ExtKeyBeneficiaryID, "189098765401003")
		assert.Empty(t, aeFaults(t, inv))
	})

	t.Run("summary invoice needs the ordering period (IBR-138-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard, pint.TagSummary)
		assert.Contains(t, aeFaults(t, inv), faultSummaryPeriod)
		start, end := cal.MakeDate(2026, 10, 1), cal.MakeDate(2026, 10, 31)
		inv.Ordering = &bill.Ordering{Period: &cal.Period{Start: &start, End: &end}}
		assert.Empty(t, aeFaults(t, inv))
	})

	t.Run("disclosed agent needs the principal ID (IBR-137-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard, pint.TagDisclosedAgent)
		assert.Contains(t, aeFaults(t, inv), faultPrincipal)
		setExt(inv, pint.ExtKeyPrincipalID, "155667745601003")
		assert.Empty(t, aeFaults(t, inv))
	})

	t.Run("principal ID must differ from the supplier TRN (IBR-176-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard, pint.TagDisclosedAgent)
		setExt(inv, pint.ExtKeyPrincipalID, inv.Supplier.TaxID.Code)
		assert.Contains(t, aeFaults(t, inv), faultPrincipalSupplier)
	})

	t.Run("disclosed agent supplier needs a tax registration (IBR-177-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard, pint.TagDisclosedAgent)
		setExt(inv, pint.ExtKeyPrincipalID, "155667745601003")
		inv.Supplier.TaxID = &tax.Identity{Country: "AE"}
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultAgentSupplierTax)
		assert.NotContains(t, out, faultPrincipalSupplier)
	})

	t.Run("e-commerce needs a complete delivery address (IBR-142-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard, pint.TagECommerce)
		assert.Contains(t, aeFaults(t, inv), faultECommerceDelivery)
		inv.Delivery = receiver(&org.Address{Locality: "Dubai", Region: "DXB", Country: "AE"})
		assert.Contains(t, aeFaults(t, inv), faultECommerceDelivery)
		inv.Delivery = receiver(aeAddress("DXB")[0])
		assert.Empty(t, aeFaults(t, inv))
	})

	t.Run("export needs a complete delivery address outside the UAE (IBR-152-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard, tax.TagExport)
		assert.Contains(t, aeFaults(t, inv), faultExportDelivery)
		inv.Delivery = receiver(&org.Address{Locality: "London", Country: "GB"})
		assert.Contains(t, aeFaults(t, inv), faultExportDelivery)
		inv.Delivery = receiver(foreign)
		assert.Empty(t, aeFaults(t, inv))
		inv.Delivery = receiver(&org.Address{Locality: "Dubai", Country: "AE"})
		assert.NotContains(t, aeFaults(t, inv), faultExportDelivery)
	})

	t.Run("outside scope documents can't be summary, deemed supply or margin scheme (IBR-157-AE)", func(t *testing.T) {
		for _, tag := range []cbc.Key{pint.TagSummary, pint.TagDeemedSupply, pint.TagMarginScheme} {
			inv := aeDocument(bill.InvoiceTypeStandard, pint.TagOutsideScope, tag)
			assert.Contains(t, aeFaults(t, inv), faultOutsideScopeFlags, tag)
		}
		inv := aeDocument(bill.InvoiceTypeStandard, pint.TagOutsideScope, pint.TagContinuousSupply)
		assert.NotContains(t, aeFaults(t, inv), faultOutsideScopeFlags)
	})

	t.Run("customer with a UAE endpoint not starting with 1 needs an identifier (IBR-135-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Customer.TaxID = nil
		inv.Customer.Endpoints = uaeEndpoints("9345678901")
		assert.Contains(t, aeFaults(t, inv), faultCustomerIdentifier)

		inv.Customer.Identities = []*org.Identity{{Type: "XYZ", Code: "12345"}}
		assert.NotContains(t, aeFaults(t, inv), faultCustomerIdentifier)
	})

	t.Run("exports and endpoints starting with 1 need no customer identifier (IBR-135-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard, tax.TagExport)
		inv.Customer.TaxID = nil
		inv.Customer.Endpoints = uaeEndpoints("9345678901")
		assert.NotContains(t, aeFaults(t, inv), faultCustomerIdentifier)

		inv = aeDocument(bill.InvoiceTypeStandard)
		inv.Customer.TaxID = nil
		inv.Customer.Endpoints = uaeEndpoints("1345678901")
		assert.NotContains(t, aeFaults(t, inv), faultCustomerIdentifier)
	})
}

func TestBillCreditNoteReasonRulesAE(t *testing.T) {
	const (
		faultReasonRequired   = "GOBL-PEPPOL-PINT-BILL-INVOICE-45"
		faultPrecedingMissing = "GOBL-PEPPOL-PINT-BILL-INVOICE-46"
		faultPrecedingVD      = "GOBL-PEPPOL-PINT-BILL-INVOICE-47"
		faultReasonCode       = "GOBL-PEPPOL-PINT-BILL-INVOICE-48"
	)
	withReason := func(inv *bill.Invoice, code cbc.Code) *bill.Invoice {
		if inv.Tax == nil {
			inv.Tax = new(bill.Tax)
		}
		inv.Tax.Ext = inv.Tax.Ext.Set(pint.ExtKeyCreditNoteReason, code)
		return inv
	}

	t.Run("credit note needs a reason (IBR-158-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeCreditNote)
		inv.Tax.Ext = inv.Tax.Ext.Delete(pint.ExtKeyCreditNoteReason)
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultReasonRequired)
		assert.NotContains(t, out, faultPrecedingMissing)
		assert.NotContains(t, out, faultPrecedingVD)
	})

	t.Run("credit note related to goods and services needs a reason (IBR-055-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeCreditNote, pint.TagOutsideScope)
		inv.Tax.Ext = inv.Tax.Ext.Delete(pint.ExtKeyCreditNoteReason)
		assert.Contains(t, aeFaults(t, inv), faultReasonRequired)
	})

	t.Run("invoices and self-billed credit notes need no reason", func(t *testing.T) {
		assert.NotContains(t, aeFaults(t, aeDocument(bill.InvoiceTypeStandard)), faultReasonRequired)
		inv := aeDocument(bill.InvoiceTypeCreditNote, tax.TagSelfBilled)
		inv.Tax.Ext = inv.Tax.Ext.Delete(pint.ExtKeyCreditNoteReason)
		assert.NotContains(t, aeFaults(t, inv), faultReasonRequired)
	})

	t.Run("credit note needs a preceding invoice, except for volume discounts (IBR-055-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeCreditNote)
		inv.Preceding = nil
		assert.Contains(t, aeFaults(t, inv), faultPrecedingMissing)

		inv = withReason(aeDocument(bill.InvoiceTypeCreditNote), pint.CreditNoteReasonVolumeDiscount)
		inv.Preceding = nil
		assert.Empty(t, aeFaults(t, inv))
	})

	t.Run("volume discount credit note has no preceding invoice (IBR-055-AE)", func(t *testing.T) {
		inv := withReason(aeDocument(bill.InvoiceTypeCreditNote), pint.CreditNoteReasonVolumeDiscount)
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultPrecedingVD)
		assert.NotContains(t, out, faultPrecedingMissing)
	})

	t.Run("reason must be from the code list (IBR-001-AE)", func(t *testing.T) {
		for _, code := range []cbc.Code{"DL8.61.1.A", "DL8.61.1.B", "DL8.61.1.C", "DL8.61.1.D", "DL8.61.1.E"} {
			inv := withReason(aeDocument(bill.InvoiceTypeCreditNote), code)
			assert.Empty(t, aeFaults(t, inv), code)
		}
		inv := withReason(aeDocument(bill.InvoiceTypeCreditNote), "DL8.61.1.F")
		assert.Contains(t, aeFaults(t, inv), faultReasonCode)
	})
}

func TestBillBillingFrequencyRulesAE(t *testing.T) {
	const (
		faultFrequencyCode = "GOBL-PEPPOL-PINT-BILL-INVOICE-49"
		faultTaxPoint      = "GOBL-PEPPOL-PINT-BILL-INVOICE-50"
		faultOthersNote    = "GOBL-PEPPOL-PINT-BILL-INVOICE-51"
	)
	withFrequency := func(code cbc.Code) *bill.Invoice {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Tax = &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyBillingFrequency: code})}
		return inv
	}

	t.Run("frequency must be from the code list (IBR-005-AE)", func(t *testing.T) {
		for _, code := range []cbc.Code{"DLY", "WKY", "Q15", "MTH", "Q45", "Q60", "QTR", "YRL", "HYR"} {
			assert.Empty(t, aeFaults(t, withFrequency(code)), code)
		}
		assert.Contains(t, aeFaults(t, withFrequency("FTN")), faultFrequencyCode)
	})

	t.Run("Others needs a note (IBR-160-AE)", func(t *testing.T) {
		inv := withFrequency(pint.BillingFrequencyOthers)
		assert.Contains(t, aeFaults(t, inv), faultOthersNote)
		inv.Notes = []*org.Note{{Text: "Every 3 weeks"}}
		assert.Empty(t, aeFaults(t, inv))
	})

	t.Run("tax point can't be set, as it would take the frequency's place (IBR-005-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Tax = &bill.Tax{Point: tax.PointIssue}
		assert.Contains(t, aeFaults(t, inv), faultTaxPoint)
	})

	t.Run("tax point is fine outside the UAE", func(t *testing.T) {
		inv := scenarioInvoice(l10n.AU.Tax(), bill.InvoiceTypeStandard)
		inv.Tax = &bill.Tax{Point: tax.PointIssue}
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.AU.Tax()))
		if err != nil {
			assert.NotContains(t, err.Error(), faultTaxPoint)
		}
	})
}

func TestBillExemptionRulesAE(t *testing.T) {
	const (
		faultLineExemption  = "GOBL-PEPPOL-PINT-BILL-LINE-03"
		faultDiscountReason = "GOBL-PEPPOL-PINT-BILL-INVOICE-52"
		faultChargeReason   = "GOBL-PEPPOL-PINT-BILL-INVOICE-53"
	)
	exempt := func(code cbc.Code) *tax.Combo {
		c := lineCombo(tax.KeyExempt)
		if code != "" {
			c.Ext = tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyExemption: code})
		}
		return c
	}

	t.Run("exempt line needs a UAE exemption reason (IBR-167-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Taxes = tax.Set{exempt("")}
		assert.Contains(t, aeFaults(t, inv), faultLineExemption)

		// IBR-167-AE only checks a code is there, so codes outside the AE
		// list, like the official examples' VATEX-AE-EDU, are accepted.
		for _, code := range []cbc.Code{"DL8.46.1", "DL8.46.4", "VATEX-AE-EDU"} {
			inv = aeDocument(bill.InvoiceTypeStandard)
			inv.Lines[0].Taxes = tax.Set{exempt(code)}
			assert.NotContains(t, aeFaults(t, inv), faultLineExemption, code)
		}
	})

	t.Run("other categories need no exemption reason", func(t *testing.T) {
		assert.NotContains(t, aeFaults(t, aeDocument(bill.InvoiceTypeStandard)), faultLineExemption)
	})

	t.Run("exempt discount needs an allowance reason code (IBR-168-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Discounts = []*bill.Discount{{Reason: "Loyalty", Amount: num.MakeAmount(100, 2), Taxes: tax.Set{exempt("DL8.46.1")}}}
		assert.Contains(t, aeFaults(t, inv), faultDiscountReason)

		inv.Discounts[0].Ext = tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyAllowance: "95"})
		assert.NotContains(t, aeFaults(t, inv), faultDiscountReason)
	})

	t.Run("exempt charge needs a charge reason code (IBR-169-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Charges = []*bill.Charge{{Reason: "Handling", Amount: num.MakeAmount(100, 2), Taxes: tax.Set{exempt("DL8.46.1")}}}
		assert.Contains(t, aeFaults(t, inv), faultChargeReason)

		inv.Charges[0].Ext = tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyCharge: "AAT"})
		assert.NotContains(t, aeFaults(t, inv), faultChargeReason)
	})

	t.Run("standard rated discounts and charges need no reason code", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Discounts = []*bill.Discount{{Reason: "Loyalty", Amount: num.MakeAmount(100, 2), Taxes: tax.Set{standardVAT()}}}
		inv.Charges = []*bill.Charge{{Reason: "Handling", Amount: num.MakeAmount(100, 2), Taxes: tax.Set{standardVAT()}}}
		out := aeFaults(t, inv)
		assert.NotContains(t, out, faultDiscountReason)
		assert.NotContains(t, out, faultChargeReason)
	})
}

func TestBillLineRulesAE(t *testing.T) {
	const (
		faultLineVAT         = "GOBL-PEPPOL-PINT-BILL-LINE-01"
		faultLineDescription = "GOBL-PEPPOL-PINT-BILL-LINE-02"
	)

	t.Run("line without a VAT combo is rejected (IBR-145-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Taxes = nil
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultLineVAT)
		assert.Contains(t, out, "$.lines[0].taxes")
	})

	t.Run("item description is required (IBR-125-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Item.Description = ""
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultLineDescription)
		assert.Contains(t, out, "$.lines[0].item.description")
	})

	t.Run("line rules do not apply outside AE", func(t *testing.T) {
		inv := scenarioInvoice(l10n.AU.Tax(), bill.InvoiceTypeStandard)
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.AU.Tax()))
		if err != nil {
			assert.NotContains(t, err.Error(), faultLineDescription)
		}
	})
}
