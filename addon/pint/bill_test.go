package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/norm"
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

// faults validates the invoice with the PINT addon, in the given contexts too,
// and returns the faults raised, or "" if there are none.
func faults(t *testing.T, inv *bill.Invoice, ctx ...rules.WithContext) string {
	t.Helper()
	err := rules.Validate(inv, append([]rules.WithContext{tax.AddonContext(pint.V1)}, ctx...)...)
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestBillInvoiceRules(t *testing.T) {
	validate := func(inv *bill.Invoice) error {
		return rules.Validate(inv, tax.AddonContext(pint.V1))
	}

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

func TestBillPartyRules(t *testing.T) {
	const (
		faultCustomer    = "GOBL-PEPPOL-PINT-BILL-INVOICE-03"
		faultCustomerIDs = "GOBL-PEPPOL-PINT-BILL-INVOICE-05"
		faultPayeeName   = "GOBL-PEPPOL-PINT-BILL-INVOICE-06"
	)

	t.Run("customer has at most one identifier besides its legal one (ibr-sr-16)", func(t *testing.T) {
		legal := &org.Identity{Scope: org.IdentityScopeLegal, Code: "53004085616"}
		gln := &org.Identity{Code: "5790000435975"}
		duns := &org.Identity{Code: "123456789"}

		inv := baseInvoice()
		inv.Customer.Identities = []*org.Identity{legal, gln}
		assert.NotContains(t, faults(t, inv), faultCustomerIDs)

		inv.Customer.Identities = []*org.Identity{legal, gln, duns}
		assert.Contains(t, faults(t, inv), faultCustomerIDs)
	})

	t.Run("a missing customer raises no customer faults but its own (IBR-007)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Customer = nil
		out := faults(t, inv)
		assert.Contains(t, out, faultCustomer)
		assert.NotContains(t, out, faultCustomerIDs)
	})

	t.Run("payee needs a name (ibr-017)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Payment = &bill.PaymentDetails{Payee: &org.Party{Alias: "Factor"}}
		assert.Contains(t, faults(t, inv), faultPayeeName)

		inv.Payment.Payee.Name = "Factor Pty Ltd"
		assert.NotContains(t, faults(t, inv), faultPayeeName)
	})
}

func TestBillNoteRules(t *testing.T) {
	const (
		faultInvoiceNotes = "GOBL-PEPPOL-PINT-BILL-INVOICE-07"
		faultLineNotes    = "GOBL-PEPPOL-PINT-BILL-INVOICE-08"
	)
	note := func(text string) *org.Note { return &org.Note{Text: text} }

	t.Run("one note on the invoice and on each line is fine", func(t *testing.T) {
		inv := baseInvoice()
		inv.Notes = []*org.Note{note("First")}
		inv.Lines = []*bill.Line{{Notes: []*org.Note{note("First")}}}
		out := faults(t, inv)
		assert.NotContains(t, out, faultInvoiceNotes)
		assert.NotContains(t, out, faultLineNotes)
	})

	t.Run("more notes are rejected, not joined (ibr-sr-51, ibr-sr-34)", func(t *testing.T) {
		inv := baseInvoice()
		inv.Notes = []*org.Note{note("First"), note("Second")}
		inv.Lines = []*bill.Line{{Notes: []*org.Note{note("First"), note("Second")}}}
		out := faults(t, inv)
		assert.Contains(t, out, faultInvoiceNotes)
		assert.Contains(t, out, faultLineNotes)
	})
}

func TestBillDueDateRules(t *testing.T) {
	const (
		faultDueDates = "GOBL-PEPPOL-PINT-BILL-INVOICE-09"
	)
	withDueDates := func(dds ...*pay.DueDate) *bill.Invoice {
		inv := baseInvoice()
		inv.Totals = &bill.Totals{Payable: num.MakeAmount(100000, 2)}
		inv.Payment = &bill.PaymentDetails{Terms: &pay.Terms{DueDates: dds}}
		return inv
	}
	date := cal.NewDate(2026, 2, 1)
	percent := func(p int64) *num.Percentage { v := num.MakePercentage(p, 2); return &v }

	t.Run("one due date is fine", func(t *testing.T) {
		inv := withDueDates(&pay.DueDate{Date: date, Percent: percent(100)})
		assert.NotContains(t, faults(t, inv), faultDueDates)
	})

	t.Run("more due dates are rejected", func(t *testing.T) {
		inv := withDueDates(&pay.DueDate{Date: date, Percent: percent(50)}, &pay.DueDate{Date: date, Percent: percent(50)})
		assert.Contains(t, faults(t, inv), faultDueDates)
	})
}

func TestBillExemptionNoteRules(t *testing.T) {
	t.Run("VAT breakdowns outside the scope need no exemption note (BR-O-10 is not in PINT)", func(t *testing.T) {
		inv := scenarioInvoice(l10n.GB.Tax(), bill.InvoiceTypeStandard)
		inv.Currency = currency.GBP
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv, tax.AddonContext(en16931.V2017, pint.V1), tax.RegimeContext(l10n.GB.Tax()))
		if err != nil {
			assert.NotContains(t, err.Error(), "GOBL-EU-EN16931-BILL-INVOICE-08")
		}
	})
}

func TestNormalizeExchangeRateAE(t *testing.T) {
	normalize := func(regime l10n.TaxCountryCode, inv *bill.Invoice) {
		norm.Normalize(inv, tax.AddonContext(en16931.V2017, pint.V1), tax.RegimeContext(regime))
	}

	t.Run("rounds rate to AED with more than 6 decimals down to 6 (IBR-002-AE)", func(t *testing.T) {
		inv := &bill.Invoice{
			Currency: currency.USD,
			ExchangeRates: []*currency.ExchangeRate{
				{From: currency.USD, To: currency.AED, Amount: num.MakeAmount(36725126, 7)},
			},
		}
		normalize(l10n.AE.Tax(), inv)
		assert.Equal(t, "3.672513", inv.ExchangeRates[0].Amount.String())
	})

	t.Run("keeps rate to AED with at most 6 decimals untouched", func(t *testing.T) {
		inv := &bill.Invoice{
			Currency: currency.USD,
			ExchangeRates: []*currency.ExchangeRate{
				{From: currency.USD, To: currency.AED, Amount: num.MakeAmount(367285, 5)},
			},
		}
		normalize(l10n.AE.Tax(), inv)
		assert.Equal(t, "3.67285", inv.ExchangeRates[0].Amount.String())
	})

	t.Run("leaves other rates and AED invoices untouched", func(t *testing.T) {
		inv := &bill.Invoice{
			Currency: currency.USD,
			ExchangeRates: []*currency.ExchangeRate{
				{From: currency.USD, To: currency.AED, Amount: num.MakeAmount(36725000, 7)},
				{From: currency.USD, To: currency.EUR, Amount: num.MakeAmount(9212345, 7)},
			},
		}
		normalize(l10n.AE.Tax(), inv)
		assert.Equal(t, "3.672500", inv.ExchangeRates[0].Amount.String())
		assert.Equal(t, "0.9212345", inv.ExchangeRates[1].Amount.String())

		aedInv := &bill.Invoice{
			Currency: currency.AED,
			ExchangeRates: []*currency.ExchangeRate{
				{From: currency.AED, To: currency.EUR, Amount: num.MakeAmount(2512345, 7)},
			},
		}
		normalize(l10n.AE.Tax(), aedInv)
		assert.Equal(t, "0.2512345", aedInv.ExchangeRates[0].Amount.String())
	})

	t.Run("only runs under the AE regime", func(t *testing.T) {
		inv := &bill.Invoice{
			Currency: currency.USD,
			ExchangeRates: []*currency.ExchangeRate{
				{From: currency.USD, To: currency.AED, Amount: num.MakeAmount(36725126, 7)},
			},
		}
		normalize(l10n.AU.Tax(), inv)
		assert.Equal(t, "3.6725126", inv.ExchangeRates[0].Amount.String())
	})
}

func TestBillInvoiceRulesAE(t *testing.T) {
	const (
		faultRate         = "GOBL-PEPPOL-PINT-BILL-INVOICE-18"
		faultPayment      = "GOBL-PEPPOL-PINT-BILL-INVOICE-20"
		faultInstructions = "GOBL-PEPPOL-PINT-BILL-INVOICE-21"
	)
	inAE := tax.RegimeContext(l10n.AE.Tax())
	standard := func() *bill.Invoice { return aeDocument(bill.InvoiceTypeStandard) }
	usdToAED := func(amount num.Amount) []*currency.ExchangeRate {
		return []*currency.ExchangeRate{{From: currency.USD, To: currency.AED, Amount: amount}}
	}

	t.Run("AED invoice needs no exchange rate (IBR-159-AE)", func(t *testing.T) {
		inv := standard()
		inv.ExchangeRates = nil
		assert.NotContains(t, faults(t, inv, inAE), faultRate)
	})

	t.Run("non-AED invoice with a rate to AED is accepted (IBR-159-AE)", func(t *testing.T) {
		inv := standard()
		inv.Currency = currency.USD
		inv.ExchangeRates = usdToAED(num.MakeAmount(3672500, 6))
		assert.NotContains(t, faults(t, inv, inAE), faultRate)
	})

	t.Run("non-AED invoice without a rate is rejected (IBR-159-AE)", func(t *testing.T) {
		inv := standard()
		inv.Currency = currency.USD
		assert.Contains(t, faults(t, inv, inAE), faultRate)
	})

	t.Run("non-AED invoice with a rate to another currency is rejected (IBR-159-AE)", func(t *testing.T) {
		inv := standard()
		inv.Currency = currency.USD
		inv.ExchangeRates = []*currency.ExchangeRate{
			{From: currency.USD, To: currency.EUR, Amount: num.MakeAmount(92, 2)},
		}
		assert.Contains(t, faults(t, inv, inAE), faultRate)
	})

	t.Run("invoice without payment is rejected (IBR-191-AE)", func(t *testing.T) {
		inv := standard()
		inv.Payment = nil
		assert.Contains(t, faults(t, inv, inAE), faultPayment)
	})

	t.Run("invoice with payment but no instructions is rejected (IBR-191-AE)", func(t *testing.T) {
		inv := standard()
		inv.Payment = &bill.PaymentDetails{}
		out := faults(t, inv, inAE)
		assert.NotContains(t, out, faultPayment)
		assert.Contains(t, out, faultInstructions)
	})

	t.Run("credit note without payment is accepted (IBR-191-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeCreditNote)
		inv.Payment = nil
		assert.NotContains(t, faults(t, inv, inAE), faultPayment)
	})

	t.Run("AE invoice rules do not apply outside AE", func(t *testing.T) {
		inv := standard()
		inv.Currency = currency.USD
		inv.Payment = nil
		for _, regime := range []l10n.TaxCountryCode{l10n.SG.Tax(), l10n.AU.Tax()} {
			inv.Regime = tax.WithRegime(regime)
			out := faults(t, inv, tax.RegimeContext(regime))
			assert.NotContains(t, out, faultRate, "regime %s", regime)
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
		Name:       "Seller LLC",
		TaxID:      &tax.Identity{Country: "AE", Code: "100000001600003"},
		Identities: []*org.Identity{tradeLicense("112345678900003")},
		Endpoints:  validEndpoints("100000001600003"),
		Addresses:  aeAddress("DXB"),
	}
	inv.Customer = &org.Party{
		Name:      "Buyer LLC",
		TaxID:     &tax.Identity{Country: "AE", Code: "100000002400003"},
		Endpoints: validEndpoints("100000002400003"),
		Addresses: aeAddress("AUH"),
	}
	inv.Lines[0].Item.Description = "Consulting services"
	inv.Lines[0].Taxes = tax.Set{standardVAT()}
	if typ == bill.InvoiceTypeCreditNote {
		inv.Preceding = []*org.DocumentRef{{Code: "INV-001"}}
		inv.Tax = &bill.Tax{
			Ext: tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyCreditNoteReason: "DL8.61.1.E"}),
		}
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
	return faults(t, inv, tax.RegimeContext(l10n.AE.Tax()))
}

// aeOutsideScopeDocument returns a valid UAE document whose lines are all outside the scope of VAT,
// which the scenarios send as 480 or 81.
func aeOutsideScopeDocument(typ cbc.Key, tags ...cbc.Key) *bill.Invoice {
	inv := aeDocument(typ, tags...)
	inv.Lines[0].Taxes = tax.Set{lineCombo(tax.KeyOutsideScope)}
	inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "CN-1234567"}}
	return inv
}

func TestBillDocumentRulesAE(t *testing.T) {
	const (
		faultSellerTRN       = "GOBL-PEPPOL-PINT-BILL-INVOICE-24"
		faultReverseCharge   = "GOBL-PEPPOL-PINT-BILL-INVOICE-46"
		faultBuyerLegalID    = "GOBL-PEPPOL-PINT-BILL-INVOICE-25"
		faultCreditPointDate = "GOBL-PEPPOL-PINT-BILL-INVOICE-26"
		faultPointDate       = "GOBL-PEPPOL-PINT-BILL-INVOICE-19"
		faultTaxPoint        = "GOBL-PEPPOL-PINT-BILL-INVOICE-47"
		faultDueTerms        = "GOBL-PEPPOL-PINT-BILL-INVOICE-22"
		faultDueDate         = "GOBL-PEPPOL-PINT-BILL-INVOICE-23"
		faultDocumentType    = "GOBL-PEPPOL-PINT-BILL-INVOICE-48"
	)
	standard := func() *bill.Invoice { return aeDocument(bill.InvoiceTypeStandard) }
	outsideScope := func() *bill.Invoice {
		return aeOutsideScopeDocument(bill.InvoiceTypeStandard)
	}
	outsideScopeCredit := func() *bill.Invoice {
		return aeOutsideScopeDocument(bill.InvoiceTypeCreditNote)
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
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeTax, Code: "100000002400003"}}
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
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeTax, Code: "100000002400003"}}
		assert.Contains(t, aeFaults(t, inv), faultBuyerLegalID)
	})

	t.Run("buyer legal identity is not required on 380 (IBR-136-AE)", func(t *testing.T) {
		assert.NotContains(t, aeFaults(t, standard()), faultBuyerLegalID)
	})

	t.Run("credit notes reject a VAT point date (IBR-124-AE)", func(t *testing.T) {
		// 261 too: gobl.ubl writes a credit note's VAT point date where UBL doesn't allow it.
		for _, inv := range []*bill.Invoice{aeDocument(bill.InvoiceTypeCreditNote), outsideScopeCredit(), aeDocument(bill.InvoiceTypeCreditNote, tax.TagSelfBilled)} {
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
		inv := standard()
		vd := cal.MakeDate(2026, 11, 10)
		inv.ValueDate = &vd
		assert.Contains(t, aeFaults(t, inv), faultPointDate)
	})

	t.Run("tax point is rejected, as its element holds the billing frequency (IBR-005-AE)", func(t *testing.T) {
		inv := standard()
		inv.Tax = &bill.Tax{Point: tax.PointDelivery}
		assert.Contains(t, aeFaults(t, inv), faultTaxPoint)
	})

	t.Run("due date is required when an amount is due (IBR-127-AE)", func(t *testing.T) {
		inv := standard()
		inv.Payment.Terms.DueDates = nil
		assert.Contains(t, aeFaults(t, inv), faultDueDate)

		inv = standard()
		inv.Payment.Terms = nil
		assert.Contains(t, aeFaults(t, inv), faultDueTerms)
	})

	t.Run("due date is not required when nothing is due (IBR-127-AE)", func(t *testing.T) {
		inv := standard()
		inv.Payment.Terms = nil
		inv.Payment.Advances = []*pay.Record{{Description: "Paid in full", Percent: num.NewPercentage(100, 2)}}
		assert.NotContains(t, aeFaults(t, inv), faultDueTerms)
	})

	t.Run("due date is not required when the amount due is negative (IBR-127-AE)", func(t *testing.T) {
		inv := standard()
		inv.Payment.Terms = nil
		inv.Payment.Advances = []*pay.Record{{Description: "Overpaid", Percent: num.NewPercentage(150, 2)}}
		out := aeFaults(t, inv)
		require.True(t, inv.Totals.Due.IsNegative())
		assert.NotContains(t, out, faultDueTerms)
	})

	t.Run("due date is not required on 381, 81 and 261 (IBR-127-AE)", func(t *testing.T) {
		for name, inv := range map[string]*bill.Invoice{
			"381": aeDocument(bill.InvoiceTypeCreditNote),
			"81":  outsideScopeCredit(),
			"261": aeDocument(bill.InvoiceTypeCreditNote, tax.TagSelfBilled),
		} {
			inv.Payment.Terms = nil
			assert.NotContains(t, aeFaults(t, inv), faultDueTerms, name)
		}
	})

	t.Run("document type must be in the PINT AE list (ibr-cl-01)", func(t *testing.T) {
		// 380, 381, 480 and 81 are already checked in "valid documents raise no PINT faults".
		for name, inv := range map[string]*bill.Invoice{
			"389": aeDocument(bill.InvoiceTypeStandard, tax.TagSelfBilled),
			"261": aeDocument(bill.InvoiceTypeCreditNote, tax.TagSelfBilled),
		} {
			assert.NotContains(t, aeFaults(t, inv), faultDocumentType, name)
		}
		for name, inv := range map[string]*bill.Invoice{
			"325 pro forma":   aeDocument(bill.InvoiceTypeProforma),
			"384 corrective":  aeDocument(bill.InvoiceTypeCorrective),
			"383 debit note":  aeDocument(bill.InvoiceTypeDebitNote),
			"326 partial":     aeDocument(bill.InvoiceTypeStandard, tax.TagPartial),
			"386 prepayment":  aeDocument(bill.InvoiceTypeStandard, tax.TagPrepayment),
			"393 factoring":   aeDocument(bill.InvoiceTypeStandard, tax.TagFactoring),
			"396 credit note": aeDocument(bill.InvoiceTypeCreditNote, tax.TagFactoring),
		} {
			assert.Contains(t, aeFaults(t, inv), faultDocumentType, name)
		}
	})
}

func TestBillAddressRulesAE(t *testing.T) {
	const (
		faultSupplierStreet = "GOBL-PEPPOL-PINT-BILL-INVOICE-10"
		faultSupplierCity   = "GOBL-PEPPOL-PINT-BILL-INVOICE-11"
		faultSupplierRegion = "GOBL-PEPPOL-PINT-BILL-INVOICE-12"
		faultCustomerStreet = "GOBL-PEPPOL-PINT-BILL-INVOICE-15"
		faultCustomerCity   = "GOBL-PEPPOL-PINT-BILL-INVOICE-16"
		faultCustomerRegion = "GOBL-PEPPOL-PINT-BILL-INVOICE-17"
	)
	standard := func() *bill.Invoice { return aeDocument(bill.InvoiceTypeStandard) }

	t.Run("supplier address needs street, city and region (IBR-143-AE)", func(t *testing.T) {
		for fault, clear := range map[string]func(a *org.Address){
			faultSupplierStreet: func(a *org.Address) { a.Street = "" },
			faultSupplierCity:   func(a *org.Address) { a.Locality = "" },
			faultSupplierRegion: func(a *org.Address) { a.Region = "" },
		} {
			inv := standard()
			clear(inv.Supplier.Addresses[0])
			assert.Contains(t, aeFaults(t, inv), fault)
		}
	})

	t.Run("customer address needs street, city and region (IBR-144-AE)", func(t *testing.T) {
		inv := standard()
		inv.Customer.Addresses[0].Locality = ""
		assert.Contains(t, aeFaults(t, inv), faultCustomerCity)
	})

	t.Run("foreign customer address also needs a region (IBR-144-AE)", func(t *testing.T) {
		inv := standard()
		inv.Customer.Addresses = []*org.Address{{Street: "Hauptstr. 1", Locality: "Berlin", Country: "DE"}}
		assert.Contains(t, aeFaults(t, inv), faultCustomerRegion)
	})

	t.Run("every address is checked (IBR-143-AE)", func(t *testing.T) {
		inv := standard()
		inv.Supplier.Addresses = append(inv.Supplier.Addresses, &org.Address{Country: "AE", Region: "DXB"})
		assert.Contains(t, aeFaults(t, inv), faultSupplierStreet)
	})

	t.Run("party without addresses is left to EN 16931 (BR-8, BR-10)", func(t *testing.T) {
		inv := standard()
		inv.Customer.Addresses = nil
		inv.Customer.TaxID = nil
		out := aeFaults(t, inv)
		for _, fault := range []string{faultCustomerStreet, faultCustomerCity, faultCustomerRegion} {
			assert.NotContains(t, out, fault)
		}
	})
}

func uaeEndpoints(code string) []*org.Endpoint {
	return []*org.Endpoint{{URI: cbc.URI("iso6523-actorid-upis::0235:" + code)}}
}

func tradeLicense(code string) *org.Identity {
	return &org.Identity{Type: ae.IdentityTypeTradeLicense, Code: cbc.Code(code), Description: "Dubai Economy and Tourism"}
}

// aeIdentityDocument returns a valid UAE invoice whose parties have UAE Peppol
// endpoints and trade licences, the case the legal registration rules apply to.
func aeIdentityDocument() *bill.Invoice {
	inv := aeDocument(bill.InvoiceTypeStandard)
	inv.Supplier.Endpoints = uaeEndpoints("1357902467")
	inv.Supplier.Identities = []*org.Identity{tradeLicense("112345678900003")}
	inv.Customer.Endpoints = uaeEndpoints("1345678906")
	inv.Customer.Identities = []*org.Identity{tradeLicense("112345679000001")}
	return inv
}

func TestBillIdentityRulesAE(t *testing.T) {
	const (
		faultSupplierLegalRequired = "GOBL-PEPPOL-PINT-BILL-INVOICE-13"
		faultSupplierTIN           = "GOBL-PEPPOL-PINT-BILL-INVOICE-14"
	)

	t.Run("supplier needs a legal identity (IBR-150-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = nil
		assert.Contains(t, aeFaults(t, inv), faultSupplierLegalRequired)
	})

	t.Run("supplier TIN before the legal identity is accepted (IBR-148-AE)", func(t *testing.T) {
		inv := aeIdentityDocument()
		inv.Supplier.Identities = append([]*org.Identity{
			{Type: ae.IdentityTypeTIN, Code: "1234567897"},
		}, inv.Supplier.Identities...)
		assert.Empty(t, aeFaults(t, inv))
	})

	t.Run("supplier tax identity must be a TIN (IBR-148-AE)", func(t *testing.T) {
		for _, typ := range []cbc.Code{"", "TAX"} {
			inv := aeIdentityDocument()
			inv.Supplier.Identities = append(inv.Supplier.Identities,
				&org.Identity{Scope: org.IdentityScopeTax, Type: typ, Code: "1234567897"})
			assert.Contains(t, aeFaults(t, inv), faultSupplierTIN, typ)
		}
	})

	t.Run("the supplier TIN's format is left to the AE regime (IBR-148-AE)", func(t *testing.T) {
		for _, code := range []cbc.Code{"2234567890", "123456789"} {
			inv := aeIdentityDocument()
			inv.Supplier.Identities = append(inv.Supplier.Identities,
				&org.Identity{Scope: org.IdentityScopeTax, Type: ae.IdentityTypeTIN, Code: code})
			assert.NotContains(t, aeFaults(t, inv), faultSupplierTIN, code)
		}
	})
}

func setAETaxExt(inv *bill.Invoice, key cbc.Key, code cbc.Code) {
	if inv.Tax == nil {
		inv.Tax = new(bill.Tax)
	}
	inv.Tax.Ext = inv.Tax.Ext.Set(key, code)
}

func TestBillTransactionTypeRulesAE(t *testing.T) {
	const (
		faultPayment             = "GOBL-PEPPOL-PINT-BILL-INVOICE-20"
		faultPaymentInstructions = "GOBL-PEPPOL-PINT-BILL-INVOICE-21"
		faultDueTerms            = "GOBL-PEPPOL-PINT-BILL-INVOICE-22"
		faultDueDate             = "GOBL-PEPPOL-PINT-BILL-INVOICE-23"
		faultSellerTRN           = "GOBL-PEPPOL-PINT-BILL-INVOICE-24"
		faultBeneficiary         = "GOBL-PEPPOL-PINT-BILL-INVOICE-31"
		faultSummaryOrdering     = "GOBL-PEPPOL-PINT-BILL-INVOICE-35"
		faultSummaryPeriod       = "GOBL-PEPPOL-PINT-BILL-INVOICE-36"
		faultPrincipal           = "GOBL-PEPPOL-PINT-BILL-INVOICE-37"
		faultSupplierTax         = "GOBL-PEPPOL-PINT-BILL-INVOICE-38"
		faultDelivery            = "GOBL-PEPPOL-PINT-BILL-INVOICE-39"
		faultDeliveryAddress     = "GOBL-PEPPOL-PINT-BILL-INVOICE-41"
		faultDeliveryStreet      = "GOBL-PEPPOL-PINT-BILL-INVOICE-42"
		faultDeliveryRegion      = "GOBL-PEPPOL-PINT-BILL-INVOICE-44"
		faultExportCountry       = "GOBL-PEPPOL-PINT-BILL-INVOICE-45"
		faultTxTypeFormat        = "GOBL-PEPPOL-PINT-BILL-INVOICE-49"
		faultPrincipalFormat     = "GOBL-PEPPOL-PINT-BILL-INVOICE-51"
	)
	receiver := func(a *org.Address) *bill.DeliveryDetails {
		return &bill.DeliveryDetails{Receiver: &org.Party{Name: "Receiver", Addresses: []*org.Address{a}}}
	}
	foreign := &org.Address{Street: "Baker Street 221B", Locality: "London", Region: "Greater London", Country: "GB"}
	withTxType := func(code cbc.Code) *bill.Invoice {
		inv := aeDocument(bill.InvoiceTypeStandard)
		setAETaxExt(inv, pint.ExtKeyTransactionType, code)
		return inv
	}

	t.Run("transaction type must be an 8-digit binary string (IBR-154-AE)", func(t *testing.T) {
		for _, code := range []cbc.Code{"00000002", "101", "000000001", "abcdefgh"} {
			assert.Contains(t, aeFaults(t, withTxType(code)), faultTxTypeFormat, code)
		}
		assert.NotContains(t, aeFaults(t, withTxType("00001000")), faultTxTypeFormat)
	})

	t.Run("deemed supplies need no payment details, instructions or due date (IBR-191-AE, IBR-127-AE)", func(t *testing.T) {
		inv := withTxType("01000000")
		inv.Payment = nil
		out := aeFaults(t, inv)
		assert.NotContains(t, out, faultPayment)
		assert.NotContains(t, out, faultPaymentInstructions)
		assert.NotContains(t, out, faultDueTerms)
		assert.NotContains(t, out, faultDueDate)

		inv = withTxType("01000000")
		inv.Payment = &bill.PaymentDetails{Terms: &pay.Terms{Notes: "No consideration"}}
		out = aeFaults(t, inv)
		assert.NotContains(t, out, faultPaymentInstructions)
		assert.NotContains(t, out, faultDueDate)
	})

	t.Run("free trade zone needs pint-ae-beneficiary-id (IBR-007-AE)", func(t *testing.T) {
		inv := withTxType("10000000")
		assert.Contains(t, aeFaults(t, inv), faultBeneficiary)
		setAETaxExt(inv, pint.ExtKeyBeneficiaryID, "189098765401003")
		assert.Empty(t, aeFaults(t, inv))
	})

	t.Run("summary invoice needs the ordering period (IBR-138-AE)", func(t *testing.T) {
		inv := withTxType("00010000")
		assert.Contains(t, aeFaults(t, inv), faultSummaryOrdering)
		inv.Ordering = &bill.Ordering{Code: "PO-1"}
		assert.Contains(t, aeFaults(t, inv), faultSummaryPeriod)
		start, end := cal.MakeDate(2026, 10, 1), cal.MakeDate(2026, 10, 31)
		inv.Ordering.Period = &cal.Period{Start: &start, End: &end}
		assert.Empty(t, aeFaults(t, inv))
	})

	t.Run("disclosed agent needs pint-ae-principal-id matching UAE TRN ending with 03 (IBR-137-AE, IBR-132-AE)", func(t *testing.T) {
		inv := withTxType("00000100")
		assert.Contains(t, aeFaults(t, inv), faultPrincipal)
		setAETaxExt(inv, pint.ExtKeyPrincipalID, "155667745601003")
		assert.Empty(t, aeFaults(t, inv))

		// Format is validated whenever present, even without the disclosed agent flag.
		for _, code := range []cbc.Code{"255667745601003", "155667745601004"} {
			inv = aeDocument(bill.InvoiceTypeStandard)
			setAETaxExt(inv, pint.ExtKeyPrincipalID, code)
			assert.Contains(t, aeFaults(t, inv), faultPrincipalFormat, code)
		}
	})

	t.Run("supplier needs a TRN or TIN on outside scope documents (IBR-177-AE)", func(t *testing.T) {
		// Elsewhere the TRN is already required (IBR-134-AE), so only that
		// rule fails.
		inv := withTxType("00000100")
		setAETaxExt(inv, pint.ExtKeyPrincipalID, "155667745601003")
		inv.Supplier.TaxID = &tax.Identity{Country: "AE"}
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultSellerTRN)
		assert.NotContains(t, out, faultSupplierTax)

		for _, typ := range []cbc.Key{bill.InvoiceTypeStandard, bill.InvoiceTypeCreditNote} {
			inv = aeOutsideScopeDocument(typ)
			inv.Supplier.TaxID = &tax.Identity{Country: "AE"}
			assert.Contains(t, aeFaults(t, inv), faultSupplierTax, typ)

			// The TIN is enough on 480 and 81 (IBR-134-AE).
			inv.Supplier.Identities = append(inv.Supplier.Identities,
				&org.Identity{Type: ae.IdentityTypeTIN, Code: "1234567897"})
			assert.Empty(t, aeFaults(t, inv), typ)
		}
	})

	t.Run("e-commerce needs a complete delivery address (IBR-142-AE)", func(t *testing.T) {
		inv := withTxType("00000010")
		assert.Contains(t, aeFaults(t, inv), faultDelivery)
		inv.Delivery = &bill.DeliveryDetails{Receiver: &org.Party{Name: "Warehouse"}}
		assert.Contains(t, aeFaults(t, inv), faultDeliveryAddress)
		inv.Delivery = receiver(&org.Address{Locality: "Dubai", Region: "DXB", Country: "AE"})
		assert.Contains(t, aeFaults(t, inv), faultDeliveryStreet)
		inv.Delivery = receiver(aeAddress("DXB")[0])
		assert.Empty(t, aeFaults(t, inv))
	})

	t.Run("export needs a complete delivery address outside the UAE (IBR-152-AE)", func(t *testing.T) {
		inv := withTxType("00000001")
		assert.Contains(t, aeFaults(t, inv), faultDelivery)
		inv.Delivery = receiver(&org.Address{Locality: "London", Country: "GB"})
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultDeliveryStreet)
		assert.Contains(t, out, faultDeliveryRegion)
		assert.NotContains(t, out, faultExportCountry)

		inv.Delivery = receiver(foreign)
		assert.Empty(t, aeFaults(t, inv))

		inv.Delivery = receiver(aeAddress("DXB")[0])
		out = aeFaults(t, inv)
		assert.Contains(t, out, faultExportCountry)
	})
}

func TestBillCreditNoteReasonRulesAE(t *testing.T) {
	const (
		faultReasonForbidden  = "GOBL-PEPPOL-PINT-BILL-INVOICE-27"
		faultReasonRequired   = "GOBL-PEPPOL-PINT-BILL-INVOICE-28"
		faultPrecedingVD      = "GOBL-PEPPOL-PINT-BILL-INVOICE-29"
		faultPrecedingMissing = "GOBL-PEPPOL-PINT-BILL-INVOICE-30"
		faultReasonCode       = "GOBL-PEPPOL-PINT-BILL-INVOICE-50"
	)

	t.Run("credit note reason is forbidden on non-credit-notes (380, 480, 389)", func(t *testing.T) {
		for name, inv := range map[string]*bill.Invoice{
			"380": aeDocument(bill.InvoiceTypeStandard),
			"480": aeOutsideScopeDocument(bill.InvoiceTypeStandard),
			"389": aeDocument(bill.InvoiceTypeStandard, tax.TagSelfBilled),
		} {
			setAETaxExt(inv, pint.ExtKeyCreditNoteReason, "DL8.61.1.E")
			assert.Contains(t, aeFaults(t, inv), faultReasonForbidden, name)
		}
	})

	t.Run("credit note needs pint-ae-credit-note-reason (IBR-158-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeCreditNote)
		inv.Tax = nil
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultReasonRequired)
		assert.NotContains(t, out, faultPrecedingMissing)
		assert.NotContains(t, out, faultPrecedingVD)
		assert.NotContains(t, out, faultReasonCode)
	})

	t.Run("credit note related to goods and services (81) needs a reason (IBR-055-AE)", func(t *testing.T) {
		inv := aeOutsideScopeDocument(bill.InvoiceTypeCreditNote)
		inv.Tax = nil
		assert.Contains(t, aeFaults(t, inv), faultReasonRequired)
	})

	t.Run("self-billed credit notes (261) allow optional reason and validate if present (IBR-001-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeCreditNote, tax.TagSelfBilled)
		inv.Tax = nil
		out := aeFaults(t, inv)
		assert.NotContains(t, out, faultReasonRequired)
		assert.NotContains(t, out, faultReasonCode)

		inv = aeDocument(bill.InvoiceTypeCreditNote, tax.TagSelfBilled)
		setAETaxExt(inv, pint.ExtKeyCreditNoteReason, "DL8.61.1.D")
		assert.Empty(t, aeFaults(t, inv))

		// VD is allowed on 261, and the 381/81 preceding-invoice rules do not apply to 261.
		inv = aeDocument(bill.InvoiceTypeCreditNote, tax.TagSelfBilled)
		setAETaxExt(inv, pint.ExtKeyCreditNoteReason, pint.CreditNoteReasonVolumeDiscount)
		assert.Empty(t, aeFaults(t, inv))

		inv = aeDocument(bill.InvoiceTypeCreditNote, tax.TagSelfBilled)
		inv.Preceding = nil
		setAETaxExt(inv, pint.ExtKeyCreditNoteReason, pint.CreditNoteReasonVolumeDiscount)
		assert.Empty(t, aeFaults(t, inv))

		inv = aeDocument(bill.InvoiceTypeCreditNote, tax.TagSelfBilled)
		setAETaxExt(inv, pint.ExtKeyCreditNoteReason, "Free text reason")
		assert.Contains(t, aeFaults(t, inv), faultReasonCode)
	})

	t.Run("credit note needs a preceding invoice, except for volume discounts (IBR-055-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeCreditNote)
		inv.Preceding = nil
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultPrecedingMissing)
		assert.NotContains(t, out, faultReasonRequired)

		inv = aeDocument(bill.InvoiceTypeCreditNote)
		inv.Preceding = nil
		setAETaxExt(inv, pint.ExtKeyCreditNoteReason, pint.CreditNoteReasonVolumeDiscount)
		assert.Empty(t, aeFaults(t, inv))
	})

	t.Run("volume discount credit note has no preceding invoice (IBR-055-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeCreditNote)
		setAETaxExt(inv, pint.ExtKeyCreditNoteReason, pint.CreditNoteReasonVolumeDiscount)
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultPrecedingVD)
		assert.NotContains(t, out, faultPrecedingMissing)
	})

	t.Run("reason must be from the code list (IBR-001-AE)", func(t *testing.T) {
		for _, code := range []cbc.Code{"DL8.61.1.A", "DL8.61.1.B", "DL8.61.1.C", "DL8.61.1.D", "DL8.61.1.E"} {
			inv := aeDocument(bill.InvoiceTypeCreditNote)
			setAETaxExt(inv, pint.ExtKeyCreditNoteReason, code)
			assert.Empty(t, aeFaults(t, inv), code)
		}
		for _, code := range []cbc.Code{"DL8.61.1.F", "Other reason"} {
			inv := aeDocument(bill.InvoiceTypeCreditNote)
			setAETaxExt(inv, pint.ExtKeyCreditNoteReason, code)
			assert.Contains(t, aeFaults(t, inv), faultReasonCode, code)
		}
	})
}

func TestBillExemptionRulesAE(t *testing.T) {
	const (
		faultExemptionCode     = "GOBL-PEPPOL-PINT-TAX-COMBO-08"
		faultLineExemption     = "GOBL-PEPPOL-PINT-BILL-INVOICE-52"
		faultDiscountExemption = "GOBL-PEPPOL-PINT-BILL-INVOICE-53"
		faultChargeExemption   = "GOBL-PEPPOL-PINT-BILL-INVOICE-54"
	)
	exempt := func(code cbc.Code) *tax.Combo {
		c := lineCombo(tax.KeyExempt)
		if code != "" {
			c.Ext = tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyExemption: code})
		}
		return c
	}

	t.Run("exemption reason must be a UAE one", func(t *testing.T) {
		for _, code := range []cbc.Code{"DL8.46.5", "VATEX-EU-132"} {
			inv := aeDocument(bill.InvoiceTypeStandard)
			inv.Lines[0].Taxes = tax.Set{exempt(code)}
			assert.Contains(t, aeFaults(t, inv), faultExemptionCode, code)
		}
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Taxes = tax.Set{exempt("DL8.46.2")}
		assert.NotContains(t, aeFaults(t, inv), faultExemptionCode)
	})

	t.Run("exempt lines, discounts and charges require pint-ae-exemption on non-margin-scheme invoices (IBR-167-AE, IBR-168-AE, IBR-169-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Taxes = tax.Set{exempt("")}
		assert.Contains(t, aeFaults(t, inv), faultLineExemption)

		inv = aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Taxes = tax.Set{exempt("DL8.46.1")}
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "CN-1234567"}}
		assert.Empty(t, aeFaults(t, inv))

		inv = aeDocument(bill.InvoiceTypeStandard)
		inv.Discounts = []*bill.Discount{{Reason: "Loyalty", Amount: num.MakeAmount(100, 2), Taxes: tax.Set{exempt("")}}}
		assert.Contains(t, aeFaults(t, inv), faultDiscountExemption)

		inv = aeDocument(bill.InvoiceTypeStandard)
		inv.Discounts = []*bill.Discount{{Reason: "Loyalty", Amount: num.MakeAmount(100, 2), Taxes: tax.Set{exempt("DL8.46.1")}}}
		assert.NotContains(t, aeFaults(t, inv), faultDiscountExemption)

		inv = aeDocument(bill.InvoiceTypeStandard)
		inv.Charges = []*bill.Charge{{Reason: "Delivery", Amount: num.MakeAmount(100, 2), Taxes: tax.Set{exempt("")}}}
		assert.Contains(t, aeFaults(t, inv), faultChargeExemption)

		inv = aeDocument(bill.InvoiceTypeStandard)
		inv.Charges = []*bill.Charge{{Reason: "Delivery", Amount: num.MakeAmount(100, 2), Taxes: tax.Set{exempt("DL8.46.4")}}}
		assert.NotContains(t, aeFaults(t, inv), faultChargeExemption)
	})
}

func TestBillMarginSchemeRulesAE(t *testing.T) {
	const (
		faultMarginLine     = "GOBL-PEPPOL-PINT-BILL-INVOICE-32"
		faultMarginDiscount = "GOBL-PEPPOL-PINT-BILL-INVOICE-33"
		faultMarginCharge   = "GOBL-PEPPOL-PINT-BILL-INVOICE-34"
	)
	margin := func() *bill.Invoice {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Taxes = tax.Set{lineCombo(tax.KeyExempt)}
		setAETaxExt(inv, pint.ExtKeyTransactionType, "00100000")
		return inv
	}

	t.Run("exempt lines stay E without VAT, keep document type 380, and no automatic scenario note is added", func(t *testing.T) {
		inv := margin()
		assert.Empty(t, aeFaults(t, inv))
		c := inv.Lines[0].Taxes[0]
		assert.Equal(t, en16931.TaxCategoryExempt, c.Ext.Get(untdid.ExtKeyTaxCategory))
		assert.Nil(t, c.Percent)
		assert.Equal(t, cbc.Code("00100000"), inv.Tax.GetExt(pint.ExtKeyTransactionType))
		assert.Equal(t, cbc.Code("380"), inv.Tax.GetExt(untdid.ExtKeyDocumentType))
		assert.Empty(t, inv.Tax.Notes)
	})

	t.Run("margin scheme invoices only have exempt lines without a UAE exemption reason (IBR-116-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		setAETaxExt(inv, pint.ExtKeyTransactionType, "00100000")
		assert.Contains(t, aeFaults(t, inv), faultMarginLine)

		inv = margin()
		inv.Lines = append(inv.Lines, &bill.Line{
			Quantity: num.MakeAmount(1, 0),
			Item:     &org.Item{Name: "Watch strap", Description: "Leather strap", Price: num.NewAmount(1000, 2)},
			Taxes:    tax.Set{standardVAT()},
		})
		assert.Contains(t, aeFaults(t, inv), faultMarginLine)

		// A line with a UAE exemption reason is a real exempt supply, not a
		// margin scheme one.
		inv = margin()
		inv.Lines[0].Taxes[0].Ext = tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyExemption: "DL8.46.1"})
		assert.Contains(t, aeFaults(t, inv), faultMarginLine)
		assert.Equal(t, en16931.TaxCategoryExempt, inv.Lines[0].Taxes[0].Ext.Get(untdid.ExtKeyTaxCategory))

		inv = margin()
		inv.Discounts = []*bill.Discount{{Reason: "Loyalty", Amount: num.MakeAmount(100, 2), Taxes: tax.Set{lineCombo(tax.KeyExempt)}}}
		inv.Charges = []*bill.Charge{{Reason: "Delivery", Amount: num.MakeAmount(100, 2), Taxes: tax.Set{lineCombo(tax.KeyExempt)}}}
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultMarginDiscount)
		assert.Contains(t, out, faultMarginCharge)
	})
}

func TestBillDiscountChargeRules(t *testing.T) {
	const (
		faultCharge     = "GOBL-PEPPOL-PINT-BILL-CHARGE-01"
		faultENDiscount = "GOBL-EU-EN16931-BILL-DISCOUNT-01"
	)
	ctx := []rules.WithContext{tax.AddonContext(pint.V1, en16931.V2017), tax.RegimeContext(l10n.AU.Tax())}
	gst := func() tax.Set {
		return tax.Set{{
			Category: tax.CategoryGST,
			Key:      tax.KeyStandard,
			Percent:  num.NewPercentage(10, 2),
			Ext:      tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: "S"}),
		}}
	}
	errText := func(err error) string {
		if err == nil {
			return ""
		}
		return err.Error()
	}

	t.Run("GST discounts and charges pass, as PINT takes any tax scheme", func(t *testing.T) {
		d := &bill.Discount{Reason: "Loyalty", Amount: num.MakeAmount(100, 2), Taxes: gst()}
		c := &bill.Charge{Reason: "Freight", Amount: num.MakeAmount(100, 2), Taxes: gst()}
		out := errText(rules.Validate(d, ctx...)) + errText(rules.Validate(c, ctx...))
		for _, code := range []string{faultCharge, faultENDiscount} {
			assert.NotContains(t, out, code)
		}
	})

	t.Run("discounts and charges need taxes (BR-32, aligned-ibrp-037)", func(t *testing.T) {
		d := &bill.Discount{Reason: "Loyalty", Amount: num.MakeAmount(100, 2)}
		c := &bill.Charge{Reason: "Freight", Amount: num.MakeAmount(100, 2)}
		assert.Contains(t, errText(rules.Validate(d, ctx...)), faultENDiscount)
		assert.Contains(t, errText(rules.Validate(c, ctx...)), faultCharge)
	})
}
