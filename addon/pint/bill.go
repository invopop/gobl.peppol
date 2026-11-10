package pint

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		rules.Field("supplier",
			rules.Assert("01", "invoice supplier must have a tax ID or identity (IBR-CO-26)",
				is.Func("has identifier", partyHasIdentifier),
			),
			rules.Field("endpoints",
				rules.Assert("02", "invoice supplier electronic address is required (IBR-081)", is.Present),
			),
		),
		rules.Field("customer",
			rules.Assert("03", "invoice customer is required (IBR-007)", is.Present),
			rules.Field("endpoints",
				rules.Assert("04", "invoice customer electronic address is required (IBR-080)", is.Present),
			),
		),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.Assert("05", "exchange_rates must have a rate from the invoice currency to AED (IBR-159-AE)",
				is.Func("has AED exchange rate", exchangeRateHasAED),
			),
			rules.Field("exchange_rates",
				rules.Each(rules.Field("amount",
					rules.Assert("06", "exchange rate must have at most 6 decimals (IBR-002-AE)",
						is.Func("has at most 6 decimals", amountHasAtMost6Decimals),
					),
				)),
			),
			// TODO: IBR-191-AE also exempts deemed supplies (transaction type
			// BTAE-02 X1XXXXXX). Add the exception with the BTAE extensions (task J).
			rules.When(
				is.Not(is.Func("credit note", invoiceIsCreditNote)),
				rules.Assert("07", "payment instructions required, except on credit notes (IBR-191-AE)",
					is.Func("has payment instructions", invoiceHasPaymentInstructions),
				),
			),
		),
	)
}

// partyHasIdentifier reports whether the party can be identified through a tax
// identity code or at least one organisation identity.
func partyHasIdentifier(val any) bool {
	p, ok := val.(*org.Party)
	return ok && p != nil && ((p.TaxID != nil && p.TaxID.Code != "") || len(p.Identities) > 0)
}

// exchangeRateHasAED reports whether an invoice not in AED has an exchange
// rate from its currency to AED, the rate written as cbc:CalculationRate in the
// tax exchange rate (IBR-159-AE). AED invoices need no rate.
func exchangeRateHasAED(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	return inv.Currency == currency.AED ||
		currency.MatchExchangeRate(inv.ExchangeRates, inv.Currency, currency.AED) != nil
}

// amountHasAtMost6Decimals reports whether an exchange rate amount has at
// most 6 decimals, the limit for cbc:CalculationRate (IBR-002-AE). The rules
// engine passes struct fields as pointers, hence *num.Amount.
func amountHasAtMost6Decimals(val any) bool {
	a, ok := val.(*num.Amount)
	return ok && a != nil && a.Exp() <= 6
}

// invoiceIsCreditNote reports whether the invoice is a credit note. The EN
// 16931 scenarios map this type to the document type codes 381, and 261 when
// self-billed, the codes IBR-191-AE exempts.
func invoiceIsCreditNote(val any) bool {
	inv, ok := val.(*bill.Invoice)
	return ok && inv != nil && inv.Type == bill.InvoiceTypeCreditNote
}

// invoiceHasPaymentInstructions reports whether the invoice has payment
// instructions. It is checked on the invoice rather than with nested fields,
// because the rules engine skips the fields of a nil payment.
func invoiceHasPaymentInstructions(val any) bool {
	inv, ok := val.(*bill.Invoice)
	return ok && inv != nil && inv.Payment != nil && inv.Payment.Instructions != nil
}
