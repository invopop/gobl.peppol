package pint

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// taxCategoryCodesOutsideScope are the VAT category codes allowed on 480 and
// 81 documents: exempt (E), zero rated (Z) and not subject to VAT (O)
// (IBR-122-AE).
var taxCategoryCodesOutsideScope = []cbc.Code{
	en16931.TaxCategoryExempt,
	en16931.TaxCategoryZero,
	en16931.TaxCategoryOutsideScope,
}

// emirateCodes are the country subdivision codes PINT AE accepts for UAE
// addresses (IBR-128-AE). They are PINT AE's own list, not ISO 3166-2.
var emirateCodes = []cbc.Code{
	"AUH", "DXB", "SHJ", "UAQ", "FUJ", "AJM", "RAK",
}

// billInvoiceRules enforces the PINT invoice rules shared by every
// jurisdiction, and the PINT AE rules on UAE documents.
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
			rules.When(
				is.Not(is.Func("document type is invoice outside scope or credit note related to goods and services",
					documentTypeIn(docTypeInvoiceOutsideScope, docTypeCreditNoteGoodsServices))),
				rules.Field("supplier",
					rules.Assert("08", "supplier tax ID code is required, except on outside scope documents (IBR-134-AE)",
						is.Func("has tax ID code", partyHasTaxIDCode),
					),
				),
			),
			rules.When(
				is.Func("has reverse charge lines", invoiceHasReverseChargeLine),
				rules.Field("customer",
					rules.Assert("09", "customer tax registration is required with reverse charge (IBR-103-AE)",
						is.Func("has tax registration", partyHasTaxRegistration),
					),
				),
			),
			rules.When(
				is.Func("document type is invoice outside scope or credit note related to goods and services",
					documentTypeIn(docTypeInvoiceOutsideScope, docTypeCreditNoteGoodsServices)),
				rules.Field("customer",
					rules.Assert("10", "customer legal identity is required on outside scope documents or credit notes related to goods and services (IBR-136-AE)",
						is.Func("has legal identity", partyHasLegalIdentity),
					),
				),
				rules.Field("lines", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Assert("11", "line VAT category code must be an E, Z, or O on outside scope documents or credit notes related to goods and services (IBR-122-AE)",
							is.Func("is E, O or Z", comboCategoryIn(taxCategoryCodesOutsideScope...)),
						),
					)),
				)),
				rules.Field("discounts", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Assert("12", "discount VAT category code must be an E, Z, or O on outside scope documents or credit notes related to goods and services (IBR-122-AE)",
							is.Func("is E, O or Z", comboCategoryIn(taxCategoryCodesOutsideScope...)),
						),
					)),
				)),
				rules.Field("charges", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Assert("13", "charge VAT category code must be an E, Z, or O on outside scope documents or credit notes related to goods and services (IBR-122-AE)",
							is.Func("is E, O or Z", comboCategoryIn(taxCategoryCodesOutsideScope...)),
						),
					)),
				)),
			),
			rules.When(
				is.Func("document type is 380 or 381",
					documentTypeIn(docTypeCommercialInvoice, docTypeCreditNote)),
				rules.Assert("14", "at least one line combo must be something other than E or O (IBR-151-AE)",
					is.Func("has at least one line combo other than E or O", invoiceHasLineComboOtherThanEorO),
				),
			),
			rules.When(
				is.Func("document type is credit note or credit note related to goods and services",
					documentTypeIn(docTypeCreditNote, docTypeCreditNoteGoodsServices)),
				rules.Field("value_date",
					rules.Assert("15", "VAT point date must not be there on credit notes or credit notes related to goods and services (IBR-124-AE)",
						is.Not(is.Present),
					),
				),
			),
			rules.Assert("16", "VAT point date must be before the invoice issue date (IBR-141-AE)",
				is.Func("VAT point date before issue date", pointDateBeforeIssueDate),
			),
			// TODO: IBR-127-AE also exempts deemed supplies: invoices whose
			// transaction type (BTAE-02, written as cbc:ProfileExecutionID) matches
			// X1XXXXXX, a 1 in the second of its 8 flags. GOBL has no field for
			// BTAE-02 yet, so add the exception with the BTAE extensions (task J).
			// Self-billed credit notes (261) are exempt too. The rule text only
			// names 381 and 81, but the Schematron test also exempts 261, and we
			// follow the test.
			rules.When(
				is.Not(is.Func("document type is 381, 81 or 261",
					documentTypeIn(docTypeCreditNote, docTypeCreditNoteGoodsServices, docTypeSelfBilledCreditNote))),
				rules.When(
					is.Func("amount is due", invoiceIsDue),
					rules.Assert("17", "payment due date is required when an amount is due (IBR-127-AE)",
						is.Func("has due date", invoiceHasDueDate),
					),
				),
			),
			rules.Field("supplier",
				rules.Assert("18", "supplier address must have a street, city and region (IBR-143-AE)",
					is.Func("first address has street, city and region", partyAddressIsComplete),
				),
				rules.Assert("19", "supplier region must be an emirate code when the country is AE (IBR-128-AE)",
					is.Func("first address region is an emirate", partyAddressHasEmirate),
				),
			),
			rules.Field("customer",
				rules.Assert("20", "customer address must have a street, city and region (IBR-144-AE)",
					is.Func("first address has street, city and region", partyAddressIsComplete),
				),
				rules.Assert("21", "customer region must be an emirate code when the country is AE (IBR-128-AE)",
					is.Func("first address region is an emirate", partyAddressHasEmirate),
				),
			),
			rules.Field("ordering",
				rules.Field("seller",
					rules.Assert("22", "tax representative region must be an emirate code when the country is AE (IBR-128-AE)",
						is.Func("first address region is an emirate", partyAddressHasEmirate),
					),
				),
			),
		),
	)
}

// billLineRules enforces the PINT AE line rules on UAE documents.
func billLineRules() *rules.Set {
	return rules.For(new(bill.Line),
		// TODO: IBR-145-AE matches ALIGNED-IBRP-050/059-AUNZ (GST). Extend to
		// A-NZ after discussing it with colleagues.
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.Field("taxes",
				rules.Assert("01", "line must have a VAT tax combo (IBR-145-AE)",
					is.Func("has VAT combo", taxesHaveVAT),
				),
			),
			rules.Field("item",
				rules.Field("description",
					rules.Assert("02", "item description is required (IBR-125-AE)", is.Present),
				),
			),
		),
	)
}

// documentType returns the UNTDID 1001 document type code (ibt-003) that the
// scenarios set on the invoice, or an empty code if none is set.
func documentType(inv *bill.Invoice) cbc.Code {
	return inv.Tax.GetExt(untdid.ExtKeyDocumentType)
}

// documentTypeIn returns a predicate that reports whether the invoice's
// document type is one of the given codes.
func documentTypeIn(codes ...cbc.Code) func(any) bool {
	return func(val any) bool {
		inv, ok := val.(*bill.Invoice)
		return ok && inv != nil && documentType(inv).In(codes...)
	}
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

// partyHasTaxIDCode reports whether the party has a tax ID with a code, the
// TRN written as the seller's PartyTaxScheme/CompanyID (IBR-134-AE).
func partyHasTaxIDCode(val any) bool {
	p, ok := val.(*org.Party)
	return ok && p != nil && p.TaxID != nil && p.TaxID.Code != ""
}

// invoiceHasReverseChargeLine reports whether any line of the invoice has a
// VAT combo with the reverse charge key, category AE (IBR-103-AE).
func invoiceHasReverseChargeLine(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	for _, l := range inv.Lines {
		if l == nil {
			continue
		}
		if c := l.Taxes.Get(tax.CategoryVAT); c != nil && c.Key == tax.KeyReverseCharge {
			return true
		}
	}
	return false
}

// partyHasTaxRegistration reports whether the party has a tax registration:
// a tax ID code or an identity with the tax scope. IBR-103-AE accepts any
// buyer PartyTaxScheme/CompanyID, and gobl.ubl writes either one there.
func partyHasTaxRegistration(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	if p.TaxID != nil && p.TaxID.Code != "" {
		return true
	}
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeTax {
			return true
		}
	}
	return false
}

// partyHasLegalIdentity reports whether the party has an identity with the
// legal scope. gobl.ubl writes the first one as
// PartyLegalEntity/CompanyID, the buyer legal registration identifier
// IBR-136-AE requires on 480 and 81.
func partyHasLegalIdentity(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeLegal {
			return true
		}
	}
	return false
}

// comboCategoryIn returns a predicate that reports whether a VAT combo's
// UNTDID 5305 category code is one of the given codes. Combos of other taxes
// pass, and any value that is not a combo fails, so a misplaced rule cannot
// pass silently.
func comboCategoryIn(codes ...cbc.Code) func(any) bool {
	return func(val any) bool {
		c, ok := val.(*tax.Combo)
		return ok && c != nil &&
			(c.Category != tax.CategoryVAT || c.Ext.Get(untdid.ExtKeyTaxCategory).In(codes...))
	}
}

// invoiceHasLineComboOtherThanEorO reports whether at least one line has a VAT
// combo whose category is neither exempt (E) nor not subject to VAT (O)
// (IBR-151-AE). Combos of other taxes are ignored, as they carry no VAT
// category.
func invoiceHasLineComboOtherThanEorO(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	for _, l := range inv.Lines {
		if l == nil {
			continue
		}
		for _, c := range l.Taxes {
			if c == nil || c.Category != tax.CategoryVAT {
				continue
			}
			if cat := c.Ext.Get(untdid.ExtKeyTaxCategory); cat != en16931.TaxCategoryExempt && cat != en16931.TaxCategoryOutsideScope {
				return true
			}
		}
	}
	return false
}

// pointDateBeforeIssueDate reports whether the invoice's value date, written as
// the VAT point date cbc:TaxPointDate, is strictly before the issue date
// (IBR-141-AE). Invoices without a value date pass.
func pointDateBeforeIssueDate(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.ValueDate == nil {
		return true
	}
	return inv.IssueDate.Date.DaysSince(inv.ValueDate.Date) > 0
}

// invoiceIsDue reports whether the invoice has an amount due for payment: the
// due total when set, the payable total otherwise. It follows isDue in the
// EN 16931 addon, which is not exported.
func invoiceIsDue(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	return inv.Totals != nil &&
		((inv.Totals.Due != nil && !inv.Totals.Due.IsZero()) ||
			(inv.Totals.Due == nil && !inv.Totals.Payable.IsZero()))
}

// invoiceHasDueDate reports whether the invoice's payment terms have a due date,
// which gobl.ubl writes as cbc:DueDate (IBR-127-AE). Missing payment details or
// terms fail.
func invoiceHasDueDate(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Payment == nil || inv.Payment.Terms == nil {
		return false
	}

	dueDates := inv.Payment.Terms.DueDates

	for _, dd := range dueDates {
		if dd != nil && dd.Date != nil {
			return true
		}
	}
	return false
}

// partyFirstAddress returns the party's first address, the only one gobl.ubl
// writes as cac:PostalAddress, or nil if there is none.
func partyFirstAddress(val any) *org.Address {
	p, ok := val.(*org.Party)
	if !ok || p == nil || len(p.Addresses) == 0 {
		return nil
	}
	return p.Addresses[0]
}

// partyAddressIsComplete reports whether the party's first address has a
// street, city and region, written as cbc:StreetName, cbc:CityName and
// cbc:CountrySubentity (IBR-143-AE, IBR-144-AE). A party without an address
// passes: EN 16931 already requires supplier and customer addresses (BR-8,
// BR-10).
func partyAddressIsComplete(val any) bool {
	a := partyFirstAddress(val)
	return a == nil || (a.Street != "" && a.Locality != "" && a.Region != "")
}

// partyAddressHasEmirate reports whether the party's first address, when in
// the UAE, has an emirate code as its region (IBR-128-AE). Addresses in other
// countries pass.
func partyAddressHasEmirate(val any) bool {
	a := partyFirstAddress(val)
	return a == nil || a.Country != l10n.AE.ISO() || cbc.Code(a.Region).In(emirateCodes...)
}

// taxesHaveVAT reports whether a line's taxes include a VAT combo (IBR-145-AE).
// It fails on any other value, so a type mismatch can't pass silently.
func taxesHaveVAT(val any) bool {
	set, ok := val.(tax.Set)
	return ok && set.Get(tax.CategoryVAT) != nil
}
