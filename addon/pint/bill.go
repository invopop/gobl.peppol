package pint

import (
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

// normalizeExchangeRateAE caps the AED exchange rate at 6 decimals (IBR-002-AE).
func normalizeExchangeRateAE(inv *bill.Invoice) {
	if inv == nil || inv.Currency == currency.AED {
		return
	}
	if rate := currency.MatchExchangeRate(inv.ExchangeRates, inv.Currency, currency.AED); rate != nil {
		rate.Amount = rate.Amount.RescaleDown(6)
	}
}

func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		// PINT has no BR-E-10, BR-O-10 or BR-AE-10; PINT AE replaces it with
		// GOBL-PEPPOL-PINT-BILL-INVOICE-52/53/54 (IBR-167-AE, IBR-168-AE, IBR-169-AE) per line, discount and charge.
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
			),

			// AE currency
			rules.Assert("18", "invoice exchange_rates must have a rate from the invoice currency to AED (IBR-159-AE)",
				currency.CanConvertTo(currency.AED),
			),

			// AE VAT point date
			rules.Assert("19", "invoice VAT point date must be before the invoice issue date (IBR-141-AE)",
				is.Func("VAT point date before issue date", pointDateBeforeIssueDate),
			),

			// AE payment
			rules.When(
				is.Not(is.Or(
					bill.InvoiceTypeIn(bill.InvoiceTypeCreditNote),
					is.Func("deemed supply", invoiceTransactionType(func(t TransactionType) bool { return t.DeemedSupply })),
				)),
				rules.Field("payment",
					rules.Assert("20", "invoice payment details required, except on credit notes and deemed supplies (IBR-191-AE)",
						is.Present,
					),
					rules.Field("instructions",
						rules.Assert("21", "invoice payment instructions required, except on credit notes and deemed supplies (IBR-191-AE)",
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
					is.Func("deemed supply", invoiceTransactionType(func(t TransactionType) bool { return t.DeemedSupply })),
				)),
				rules.When(
					is.Func("amount due is positive", invoiceIsDue),
					rules.Field("payment",
						rules.Field("terms",
							rules.Assert("22", "invoice payment terms are required when the amount due is positive, except on credit notes and deemed supplies (IBR-127-AE)",
								is.Present,
							),
							rules.Field("due_dates",
								rules.Assert("23", "invoice payment due date is required when the amount due is positive, except on credit notes and deemed supplies (IBR-127-AE)",
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
					rules.Assert("24", "invoice supplier tax ID code is required, except on outside scope documents (IBR-134-AE)",
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
						rules.Assert("25", "invoice customer legal identity is required on outside scope documents or credit notes related to goods and services (IBR-136-AE)",
							is.Func("has legal scope identity", identitiesScopeIn(org.IdentityScopeLegal)),
						),
					),
				),
			),

			// AE credit notes
			// Credit notes must not specify a value date (IBR-124-AE).
			rules.When(
				bill.InvoiceTypeIn(bill.InvoiceTypeCreditNote),
				rules.Field("value_date",
					rules.Assert("26", "invoice VAT point date must not be set on credit notes (IBR-124-AE)",
						is.Not(is.Present),
					),
				),
			),
			// Our own limit, not a PINT rule: BTAE-03 is cac:DiscrepancyResponse, which UBL only has on credit notes.
			rules.When(
				is.Not(bill.InvoiceTypeIn(bill.InvoiceTypeCreditNote)),
				rules.Field("tax",
					rules.Field("ext",
						rules.Assert("27", "invoice credit note reason must only be set on credit notes (BTAE-03)",
							tax.ExtensionsExclude(ExtKeyCreditNoteReason),
						),
					),
				),
			),
			// IBR-158-AE names only 381, but the IBR-055-AE test also fails 81 without a reason.
			rules.When(
				is.Func("document type is credit note or credit note related to goods and services",
					invoiceExtIn(untdid.ExtKeyDocumentType, docTypeCreditNote, docTypeCreditNoteGoodsServices)),
				rules.Field("tax",
					rules.Field("ext",
						rules.Assert("28", "invoice credit note reason is required in pint-ae-credit-note-reason (IBR-158-AE)",
							tax.ExtensionsRequire(ExtKeyCreditNoteReason),
						),
					),
				),
				rules.When(
					is.Func("volume discount", invoiceExtIn(ExtKeyCreditNoteReason, CreditNoteReasonVolumeDiscount)),
					rules.Field("preceding",
						rules.Assert("29", "invoice preceding reference must not be set on volume discount credit notes (IBR-055-AE)",
							is.Not(is.Present),
						),
					),
				),
				rules.When(
					is.Not(is.Func("volume discount", invoiceExtIn(ExtKeyCreditNoteReason, CreditNoteReasonVolumeDiscount))),
					rules.Field("preceding",
						rules.Assert("30", "invoice preceding reference is required on credit notes, except for volume discounts (IBR-055-AE)",
							is.Present,
						),
					),
				),
			),

			// AE transaction type
			rules.When(
				is.Func("free trade zone", invoiceTransactionType(func(t TransactionType) bool { return t.FreeTradeZone })),
				rules.Field("tax",
					rules.Field("ext",
						rules.Assert("31", "invoice beneficiary ID is required in pint-ae-beneficiary-id on free trade zone supplies (IBR-007-AE)",
							tax.ExtensionsRequire(ExtKeyBeneficiaryID),
						),
					),
				),
			),
			rules.When(
				is.Func("margin scheme", invoiceTransactionType(func(t TransactionType) bool { return t.MarginScheme })),
				rules.Field("lines", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Field("ext",
							rules.Assert("32", "invoice margin scheme lines must be exempt without a UAE exemption reason (IBR-116-AE)",
								is.AllOf(
									tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
									tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, en16931.TaxCategoryExempt),
									tax.ExtensionsExclude(ExtKeyExemption),
								),
							),
						),
					)),
				)),
				// Discounts and charges need a VAT category (BR-32, BR-37), and it can't be N (IBR-114-AE, IBR-115-AE).
				rules.Field("discounts",
					rules.Assert("33", "invoice margin scheme cannot have discounts (IBR-116-AE)", is.Empty),
				),
				rules.Field("charges",
					rules.Assert("34", "invoice margin scheme cannot have charges (IBR-116-AE)", is.Empty),
				),
			),
			rules.When(
				is.Func("summary invoice", invoiceTransactionType(func(t TransactionType) bool { return t.Summary })),
				rules.Field("ordering",
					rules.Assert("35", "invoice ordering is required on summary invoices (IBR-138-AE)",
						is.Present,
					),
					rules.Field("period",
						rules.Assert("36", "invoice ordering period is required on summary invoices (IBR-138-AE)",
							is.Present,
						),
					),
				),
			),
			rules.When(
				is.Func("disclosed agent billing", invoiceTransactionType(func(t TransactionType) bool { return t.DisclosedAgent })),
				rules.Field("tax",
					rules.Field("ext",
						rules.Assert("37", "invoice principal ID is required in pint-ae-principal-id on disclosed agent billing (IBR-137-AE)",
							tax.ExtensionsRequire(ExtKeyPrincipalID),
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
							rules.Assert("38", "invoice supplier TRN or TIN is required on outside scope documents or credit notes related to goods and services (IBR-177-AE)",
								is.Func("has tax scope identity", identitiesScopeIn(org.IdentityScopeTax)),
							),
						),
					),
				),
			),
			rules.When(
				is.Func("e-commerce supply or export", invoiceTransactionType(func(t TransactionType) bool { return t.ECommerce || t.Export })),
				rules.Field("delivery",
					rules.Assert("39", "invoice delivery is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
					rules.Field("receiver",
						rules.Assert("40", "invoice delivery receiver is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
						rules.Field("addresses",
							rules.Assert("41", "invoice delivery address is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
							rules.Each(
								rules.Field("street",
									rules.Assert("42", "invoice delivery address street is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
								),
								rules.Field("locality",
									rules.Assert("43", "invoice delivery address city is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
								),
								rules.Field("region",
									rules.Assert("44", "invoice delivery address region is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
								),
							),
						),
					),
				),
			),
			// Follows the IBR-152-AE rule text, which forbids an AE delivery country on exports (stricter than the .sch test).
			rules.When(
				is.Func("export", invoiceTransactionType(func(t TransactionType) bool { return t.Export })),
				rules.Field("delivery",
					rules.Field("receiver",
						rules.Field("addresses",
							rules.Each(
								rules.Assert("45", "invoice export delivery address country must not be AE (IBR-152-AE)",
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
							rules.Assert("46", "invoice customer tax registration is required with reverse charge (IBR-103-AE)",
								is.Func("has tax scope identity", identitiesScopeIn(org.IdentityScopeTax)),
							),
						),
					),
				),
			),

			// AE tax extensions and document type list
			rules.Field("tax",
				rules.Field("point",
					rules.Assert("47", "invoice tax point must not be set, as PINT AE uses its element for the billing frequency (IBR-005-AE)",
						is.Not(is.Present),
					),
				),
				rules.Field("ext",
					rules.Assert("48", "invoice document type must be 380, 480, 381, 81, 389 or 261 (ibr-cl-01)",
						tax.ExtensionsHasCodes(untdid.ExtKeyDocumentType, docTypesAE...),
					),
					rules.Assert("49", "invoice transaction type must be an 8-digit binary string (IBR-154-AE)",
						tax.ExtensionHasValidCode(ExtKeyTransactionType),
					),
					rules.Assert("50", "invoice credit note reason must be from the Reasons for credit note code list (IBR-001-AE)",
						tax.ExtensionHasValidCode(ExtKeyCreditNoteReason),
					),
					rules.Assert("51", "invoice principal ID must be a TRN of 15 digits starting with 1 and ending with 03 (IBR-132-AE)",
						tax.ExtensionHasValidCode(ExtKeyPrincipalID),
					),
				),
			),

			// AE exempt VAT reason (not applicable to margin scheme invoices, whose exempt lines export as category N)
			rules.When(
				is.Not(is.Func("margin scheme", invoiceTransactionType(func(t TransactionType) bool { return t.MarginScheme }))),
				rules.Field("lines", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Field("ext",
							rules.When(
								is.AllOf(
									tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
									tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, en16931.TaxCategoryExempt),
								),
								rules.Assert("52", "exempt VAT line needs a pint-ae-exemption reason: DL8.46.1, DL8.46.2, DL8.46.3 or DL8.46.4 (IBR-167-AE, IBR-168-AE, IBR-169-AE)",
									tax.ExtensionsRequire(ExtKeyExemption),
								),
							),
						),
					)),
				)),
				rules.Field("discounts", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Field("ext",
							rules.When(
								is.AllOf(
									tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
									tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, en16931.TaxCategoryExempt),
								),
								rules.Assert("53", "exempt VAT discount needs a pint-ae-exemption reason: DL8.46.1, DL8.46.2, DL8.46.3 or DL8.46.4 (IBR-167-AE, IBR-168-AE, IBR-169-AE)",
									tax.ExtensionsRequire(ExtKeyExemption),
								),
							),
						),
					)),
				)),
				rules.Field("charges", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Field("ext",
							rules.When(
								is.AllOf(
									tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
									tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, en16931.TaxCategoryExempt),
								),
								rules.Assert("54", "exempt VAT charge needs a pint-ae-exemption reason: DL8.46.1, DL8.46.2, DL8.46.3 or DL8.46.4 (IBR-167-AE, IBR-168-AE, IBR-169-AE)",
									tax.ExtensionsRequire(ExtKeyExemption),
								),
							),
						),
					)),
				)),
			),
		),
	)
}

func billChargeRules() *rules.Set {
	return rules.For(new(bill.Charge),
		rules.Field("taxes",
			rules.Assert("01", "charge taxes are required (aligned-ibrp-037)", is.Present),
		),
	)
}

func invoiceExtIn(key cbc.Key, codes ...cbc.Code) func(any) bool {
	return func(val any) bool {
		inv, ok := val.(*bill.Invoice)
		return ok && inv != nil && inv.Tax.GetExt(key).In(codes...)
	}
}

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

func partyHasOneIdentification(val any) bool {
	p, ok := val.(*org.Party)
	return ok && p != nil && len(partyIdentifications(p)) <= 1
}

func partyIdentifications(p *org.Party) []*org.Identity {
	legal := partyFirstLegalIdentity(p)
	var ids []*org.Identity
	for _, id := range p.Identities {
		if id != nil && id != legal && id.Scope != org.IdentityScopeTax {
			ids = append(ids, id)
		}
	}
	return ids
}

func partyFirstLegalIdentity(p *org.Party) *org.Identity {
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeLegal {
			return id
		}
	}
	return nil
}
