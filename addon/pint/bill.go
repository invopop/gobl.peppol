package pint

import (
	"slices"

	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// creditNoteReasonsAE are the PINT AE credit note reason codes (BTAE-03) for
// credit notes referencing a preceding invoice (all codes except VD, which is
// represented by TagVolumeDiscount).
var creditNoteReasonsAE = []string{
	"DL8.61.1.A",
	"DL8.61.1.B",
	"DL8.61.1.C",
	"DL8.61.1.D",
	"DL8.61.1.E",
}

// normalizeExchangeRateAE rounds the exchange rate from the invoice currency to
// AED down to at most 6 decimals (IBR-002-AE). Other exchange rates are left untouched.
func normalizeExchangeRateAE(inv *bill.Invoice) {
	if inv == nil || inv.Currency == currency.AED {
		return
	}
	if rate := currency.MatchExchangeRate(inv.ExchangeRates, inv.Currency, currency.AED); rate != nil {
		rate.Amount = rate.Amount.RescaleDown(6)
	}
}

// billInvoiceRules validates invoices on PINT documents.
func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		// PINT has no BR-E-10, BR-O-10 or BR-AE-10; PINT AE replaces it with
		// GOBL-PEPPOL-PINT-TAX-COMBO-08 (IBR-167-AE, IBR-168-AE, IBR-169-AE) per line, discount and charge.
		rules.Ignore("GOBL-EU-EN16931-BILL-INVOICE-08"),

		// Supplier and customer
		rules.Field("supplier",
			rules.When(
				is.Not(org.PartyHasTaxIDCode()),
				rules.Field("identities",
					rules.Assert("01", "invoice supplier must have a tax ID or identity (IBR-CO-26)", is.Present),
				),
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
		// Customer identification
		rules.Field("customer",
			rules.AssertIfPresent("05", "invoice customer must have at most one identity besides its first legal one and its tax ones (ibr-sr-16)",
				is.Func("at most one buyer identifier", partyHasOneIdentification),
			),
		),
		// Payee
		rules.Field("payment",
			rules.Field("payee",
				rules.Field("name",
					rules.Assert("06", "invoice payee name is required (ibr-017)", is.Present),
				),
			),
		),
		// Notes
		rules.Field("notes",
			rules.Assert("07", "invoice can have at most one note (ibr-sr-51)", is.Length(0, 1)),
		),
		rules.Field("lines", rules.Each(
			rules.Field("notes",
				rules.Assert("08", "invoice line can have at most one note (ibr-sr-34)", is.Length(0, 1)),
			),
		)),
		// Due dates
		// Our own limit, not a PINT rule: only the payment due date (IBT-009) is written.
		rules.Field("payment",
			rules.Field("terms",
				rules.Field("due_dates",
					rules.Assert("09", "invoice can have at most one due date (IBT-009)",
						is.Length(0, 1),
					),
				),
			),
		),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			// AE supplier
			rules.Field("supplier",
				rules.Field("addresses",
					rules.Each(
						rules.Field("street",
							rules.Assert("10", "invoice supplier address street is required (IBR-143-AE)", is.Present),
						),
						rules.Field("locality",
							rules.Assert("11", "invoice supplier address city is required (IBR-143-AE)", is.Present),
						),
						rules.Field("region",
							rules.Assert("12", "invoice supplier address region is required (IBR-143-AE)", is.Present),
						),
					),
				),
				// IBR-150-AE only applies with a 0235 endpoint, which every UAE Peppol participant has (UAE PASR).
				rules.Field("identities",
					rules.Assert("13", "invoice supplier legal identity is required (IBR-150-AE)",
						is.Func("has legal scope identity", identitiesScopeIn(org.IdentityScopeLegal)),
					),
				),
				rules.Field("identities", rules.Each(
					rules.When(
						is.Func("tax scope", identityScopeIn(org.IdentityScopeTax)),
						rules.Assert("14", "invoice supplier tax scope identity must be a TIN (IBR-148-AE)",
							org.IdentityTypeIn(ae.IdentityTypeTIN),
						),
					),
				)),
			),

			// AE customer
			rules.Field("customer",
				rules.Field("addresses",
					rules.Each(
						rules.Field("street",
							rules.Assert("15", "invoice customer address street is required (IBR-144-AE)", is.Present),
						),
						rules.Field("locality",
							rules.Assert("16", "invoice customer address city is required (IBR-144-AE)", is.Present),
						),
						rules.Field("region",
							rules.Assert("17", "invoice customer address region is required (IBR-144-AE)", is.Present),
						),
					),
				),
				rules.When(
					is.Func("has UAE Peppol endpoint other than 1XXXXXXXXX or 9XXXXXXXXX", partyHasUAEEndpoint(endpointPattern1or9, false)),
					rules.Field("identities",
						rules.Assert("18", "invoice customer legal identity is required with a UAE Peppol endpoint other than 1XXXXXXXXX or 9XXXXXXXXX (IBR-149-AE)",
							is.Func("has legal scope identity", identitiesScopeIn(org.IdentityScopeLegal)),
						),
					),
				),
			),

			// AE currency
			rules.Assert("19", "invoice exchange_rates must have a rate from the invoice currency to AED (IBR-159-AE)",
				currency.CanConvertTo(currency.AED),
			),

			// AE VAT point date
			rules.Assert("20", "invoice VAT point date must be before the invoice issue date (IBR-141-AE)",
				is.Func("VAT point date before issue date", pointDateBeforeIssueDate),
			),

			// AE payment
			rules.When(
				is.Not(is.Or(
					bill.InvoiceTypeIn(bill.InvoiceTypeCreditNote),
					is.Func("deemed supply", invoiceHasTags(TagDeemedSupply)),
				)),
				rules.Field("payment",
					rules.Assert("21", "invoice payment details required, except on credit notes and deemed supplies (IBR-191-AE)",
						is.Present,
					),
					rules.Field("instructions",
						rules.Assert("22", "invoice payment instructions required, except on credit notes and deemed supplies (IBR-191-AE)",
							is.Present,
						),
					),
				),
			),
			// The IBR-127-AE test also exempts 261, which its text doesn't name; we follow the test.
			rules.When(
				is.Not(is.Or(
					is.Func("document type is 381, 81 or 261",
						invoiceExtIn(untdid.ExtKeyDocumentType, docTypeCreditNote, docTypeCreditNoteGoodsServices, docTypeSelfBilledCreditNote)),
					is.Func("deemed supply", invoiceHasTags(TagDeemedSupply)),
				)),
				rules.When(
					is.Func("amount due is positive", invoiceIsDue),
					rules.Field("payment",
						rules.Field("terms",
							rules.Assert("23", "invoice payment terms are required when the amount due is positive, except on credit notes and deemed supplies (IBR-127-AE)",
								is.Present,
							),
							rules.Field("due_dates",
								rules.Assert("24", "invoice payment due date is required when the amount due is positive, except on credit notes and deemed supplies (IBR-127-AE)",
									is.Present,
								),
							),
						),
					),
				),
			),

			// AE document types
			rules.When(
				is.Not(is.Func("document type is invoice outside scope or credit note related to goods and services",
					invoiceExtIn(untdid.ExtKeyDocumentType, docTypeInvoiceOutsideScope, docTypeCreditNoteGoodsServices))),
				rules.Field("supplier",
					rules.Assert("25", "invoice supplier tax ID code is required, except on outside scope documents (IBR-134-AE)",
						org.PartyHasTaxIDCode(),
					),
				),
			),

			// AE outside scope documents
			rules.When(
				is.Func("document type is invoice outside scope or credit note related to goods and services",
					invoiceExtIn(untdid.ExtKeyDocumentType, docTypeInvoiceOutsideScope, docTypeCreditNoteGoodsServices)),
				rules.Field("customer",
					rules.Field("identities",
						rules.Assert("26", "invoice customer legal identity is required on outside scope documents or credit notes related to goods and services (IBR-136-AE)",
							is.Func("has legal scope identity", identitiesScopeIn(org.IdentityScopeLegal)),
						),
					),
				),
				// The lines are always E or O here: the scenarios only give 480 and 81 to documents with such lines.
				rules.Field("discounts", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Field("ext",
							rules.Assert("27", "invoice discount VAT category code must be E or O on outside scope documents or credit notes related to goods and services (IBR-122-AE)",
								tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
								tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesOutsideScope...),
							),
						),
					)),
				)),
				rules.Field("charges", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Field("ext",
							rules.Assert("28", "invoice charge VAT category code must be E or O on outside scope documents or credit notes related to goods and services (IBR-122-AE)",
								tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
								tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesOutsideScope...),
							),
						),
					)),
				)),
				rules.Field("tax",
					rules.Field("ext",
						rules.Assert("29", "invoice transaction type cannot be summary, deemed supply or margin scheme on outside scope documents or credit notes related to goods and services (IBR-157-AE)",
							tax.ExtensionsHasCodes(ExtKeyTransactionType, transactionTypeCodesOutsideScope...),
						),
					),
				),
			),

			// AE credit notes
			// IBR-124-AE names 381 and 81, but gobl.ubl writes a credit note's VAT point date where UBL doesn't allow it.
			// exportAECreditNoteReason writes preceding[0].reason on any credit note (including 261), so IBR-001-AE applies to all of them.
			rules.When(
				bill.InvoiceTypeIn(bill.InvoiceTypeCreditNote),
				rules.Field("value_date",
					rules.Assert("30", "invoice VAT point date must not be set on credit notes (IBR-124-AE)",
						is.Not(is.Present),
					),
				),
				rules.Field("preceding",
					rules.AssertIfPresent("31", `invoice credit note reason must be from the Reasons for credit note code list; use the "volume-discount" tag for VD (IBR-001-AE)`,
						is.Func("first preceding reference reason is valid", precedingHasValidReasonAE),
					),
				),
			),
			// IBR-158-AE names only 381, but the IBR-055-AE test also fails 81 without a reason.
			rules.When(
				is.Func("document type is credit note or credit note related to goods and services",
					invoiceExtIn(untdid.ExtKeyDocumentType, docTypeCreditNote, docTypeCreditNoteGoodsServices)),
				rules.When(
					is.Func("volume discount", invoiceHasTags(TagVolumeDiscount)),
					rules.Field("preceding",
						rules.Assert("32", "invoice preceding reference must not be set on volume discount credit notes (IBR-055-AE)",
							is.Not(is.Present),
						),
					),
				),
				rules.When(
					is.Not(is.Func("volume discount", invoiceHasTags(TagVolumeDiscount))),
					rules.Field("preceding",
						rules.Assert("33", "invoice preceding reference is required on credit notes, except for volume discounts (IBR-055-AE)",
							is.Present,
						),
						rules.AssertIfPresent("34", "invoice credit note reason is required in the first preceding reference (IBR-158-AE)",
							is.Func("first preceding reference has reason", precedingHasReason),
						),
					),
				),
			),

			// AE transaction type
			rules.When(
				is.Func("free trade zone", invoiceHasTags(TagFreeTradeZone)),
				rules.Field("customer",
					rules.Field("identities",
						rules.Assert("35", "invoice customer must have a beneficiary identity on free trade zone supplies (IBR-007-AE)",
							org.IdentitiesKeyIn(IdentityKeyBeneficiary),
						),
					),
				),
			),
			rules.When(
				is.Func("margin scheme", invoiceHasTags(TagMarginScheme)),
				rules.Field("lines", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Field("ext",
							rules.Assert("36", "invoice margin scheme lines must be exempt without a UAE exemption reason, mapped to the margin scheme category N (IBR-116-AE)",
								tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, TaxCategoryMarginScheme),
							),
						),
					)),
				)),
				// Discounts and charges need a VAT category (BR-32, BR-37), and it can't be N (IBR-114-AE, IBR-115-AE).
				rules.Field("discounts",
					rules.Assert("37", "invoice margin scheme cannot have discounts (IBR-116-AE)", is.Empty),
				),
				rules.Field("charges",
					rules.Assert("38", "invoice margin scheme cannot have charges (IBR-116-AE)", is.Empty),
				),
			),
			rules.When(
				is.Func("summary invoice", invoiceHasTags(TagSummary)),
				rules.Field("ordering",
					rules.Assert("39", "invoice ordering is required on summary invoices (IBR-138-AE)",
						is.Present,
					),
					rules.Field("period",
						rules.Assert("40", "invoice ordering period is required on summary invoices (IBR-138-AE)",
							is.Present,
						),
					),
				),
			),
			rules.When(
				is.Func("disclosed agent billing", invoiceHasTags(TagDisclosedAgent)),
				rules.Field("supplier",
					rules.Field("identities",
						rules.Assert("41", "invoice supplier must have a principal identity on disclosed agent billing (IBR-137-AE)",
							org.IdentitiesKeyIn(IdentityKeyPrincipal),
						),
					),
				),
			),
			// IBR-134-AE already requires the TRN elsewhere, so IBR-177-AE is only checked on 480 and 81.
			rules.When(
				is.Func("document type is invoice outside scope or credit note related to goods and services",
					invoiceExtIn(untdid.ExtKeyDocumentType, docTypeInvoiceOutsideScope, docTypeCreditNoteGoodsServices)),
				rules.Field("supplier",
					rules.When(
						is.Not(org.PartyHasTaxIDCode()),
						rules.Field("identities",
							rules.Assert("42", "invoice supplier TRN or TIN is required on outside scope documents or credit notes related to goods and services (IBR-177-AE)",
								is.Func("has tax scope identity", identitiesScopeIn(org.IdentityScopeTax)),
							),
						),
					),
				),
			),
			rules.When(
				is.Or(
					is.Func("e-commerce supply", invoiceHasTags(TagECommerce)),
					is.Func("export", invoiceHasTags(tax.TagExport)),
				),
				rules.Field("delivery",
					rules.Assert("43", "invoice delivery is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
					rules.Field("receiver",
						rules.Assert("44", "invoice delivery receiver is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
						rules.Field("addresses",
							rules.Assert("45", "invoice delivery address is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
							rules.Each(
								rules.Field("street",
									rules.Assert("46", "invoice delivery address street is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
								),
								rules.Field("locality",
									rules.Assert("47", "invoice delivery address city is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
								),
								rules.Field("region",
									rules.Assert("48", "invoice delivery address region is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
								),
							),
						),
					),
				),
			),
			// Follows the IBR-152-AE rule text, which forbids an AE delivery country on exports (stricter than the .sch test).
			rules.When(
				is.Func("export", invoiceHasTags(tax.TagExport)),
				rules.Field("delivery",
					rules.Field("receiver",
						rules.Field("addresses",
							rules.Each(
								rules.Assert("49", "invoice export delivery address country must not be AE (IBR-152-AE)",
									is.Not(is.Func("UAE address", addressIsAE)),
								),
							),
						),
					),
				),
			),

			// AE reverse charge
			rules.When(
				is.Func("has reverse charge lines", invoiceHasReverseChargeLine),
				rules.Field("customer",
					rules.When(
						is.Not(org.PartyHasTaxIDCode()),
						rules.Field("identities",
							rules.Assert("50", "invoice customer tax registration is required with reverse charge (IBR-103-AE)",
								is.Func("has tax scope identity", identitiesScopeIn(org.IdentityScopeTax)),
							),
						),
					),
				),
			),

			// AE tax extensions and document type list
			rules.Field("tax",
				rules.Field("point",
					rules.Assert("51", "invoice tax point must not be set, as PINT AE uses its element for the billing frequency (IBR-005-AE)",
						is.Not(is.Present),
					),
				),
				rules.Field("ext",
					rules.Assert("52", "invoice document type must be 380, 480, 381, 81, 389 or 261 (ibr-cl-01)",
						tax.ExtensionsHasCodes(untdid.ExtKeyDocumentType, docTypesAE...),
					),
				),
			),
		),
	)
}

// billChargeRules validates document level charges on PINT documents.
func billChargeRules() *rules.Set {
	return rules.For(new(bill.Charge),
		rules.Field("taxes",
			rules.Assert("01", "charge taxes are required (aligned-ibrp-037)", is.Present),
		),
	)
}

// invoiceExtIn returns a predicate that reports whether the invoice's tax
// extension for the key is one of the codes. A missing extension is the empty
// code.
func invoiceExtIn(key cbc.Key, codes ...cbc.Code) func(any) bool {
	return func(val any) bool {
		inv, ok := val.(*bill.Invoice)
		return ok && inv != nil && inv.Tax.GetExt(key).In(codes...)
	}
}

// precedingHasReason reports whether the first preceding reference has a reason.
func precedingHasReason(val any) bool {
	refs, ok := val.([]*org.DocumentRef)
	return ok && len(refs) > 0 && refs[0] != nil && refs[0].Reason != ""
}

// precedingHasValidReasonAE reports whether the first preceding reference's
// reason, when present, is one of the PINT AE credit note reason codes.
func precedingHasValidReasonAE(val any) bool {
	refs, ok := val.([]*org.DocumentRef)
	if !ok || len(refs) == 0 || refs[0] == nil || refs[0].Reason == "" {
		return true
	}
	return slices.Contains(creditNoteReasonsAE, refs[0].Reason)
}

// invoiceHasReverseChargeLine reports whether any line of the invoice has a
// VAT combo with the reverse charge category AE.
func invoiceHasReverseChargeLine(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	for _, l := range inv.Lines {
		if lineHasReverseChargeVAT(l) {
			return true
		}
	}
	return false
}

// invoiceHasLineComboOtherThanEorO reports whether any line has a VAT category other than E or O.
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

// pointDateBeforeIssueDate reports whether the value date, if any, is before the issue date.
func pointDateBeforeIssueDate(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	if inv.ValueDate == nil {
		return true
	}
	return inv.IssueDate.DaysSince(inv.ValueDate.Date) > 0
}

// invoiceIsDue reports whether the amount gobl.ubl writes as cbc:PayableAmount, the due total
// when set or the payable total otherwise, is greater than zero.
func invoiceIsDue(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Totals == nil {
		return false
	}
	if inv.Totals.Due != nil {
		return inv.Totals.Due.IsPositive()
	}
	return inv.Totals.Payable.IsPositive()
}
