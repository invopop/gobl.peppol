package pint

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// billInvoiceRules validates invoices on PINT documents.
func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		// PINT has no BR-E-10, BR-O-10 or BR-AE-10; PINT AE asks for the UAE reason per line, discount and charge.
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
		// Electronic addresses and customer identification
		// gobl.ubl writes an email with the scheme EM and leaves web addresses out, which PINT rejects (ibr-cl-25).
		rules.Field("supplier",
			rules.Field("endpoints", rules.Each(
				rules.Field("uri",
					rules.Assert("05", "supplier electronic address must be a Peppol endpoint (IBR-081, ibr-cl-25)",
						cbc.URISchemeIn(iso.ActorIDScheme),
					),
				),
			)),
		),
		rules.Field("customer",
			rules.Field("endpoints", rules.Each(
				rules.Field("uri",
					rules.Assert("06", "customer electronic address must be a Peppol endpoint (IBR-080, ibr-cl-25)",
						cbc.URISchemeIn(iso.ActorIDScheme),
					),
				),
			)),
			rules.AssertIfPresent("07", "customer can have at most one identity besides its first legal one and its tax ones (ibr-sr-16)",
				is.Func("at most one buyer identifier", partyHasOneIdentification),
			),
			// EN 16931 already allows one tax scope identity, so with a tax ID code there must be none.
			rules.When(
				org.PartyHasTaxIDCode(),
				rules.Field("identities",
					rules.Assert("08", "customer must have at most one tax registration (ibr-104, IBR-179-AE)",
						is.Not(is.Func("has tax scope identity", identitiesScopeIn(org.IdentityScopeTax))),
					),
				),
			),
		),
		// Payee
		rules.Field("payment",
			rules.Field("payee",
				rules.Field("name",
					rules.Assert("09", "payee name is required (ibr-017)", is.Present),
				),
			),
		),
		rules.Assert("10", "payee must differ from the supplier by legal identity, identity or name (ibr-017)",
			is.Func("payee is not the supplier", payeeIsNotSupplier),
		),
		// Notes
		rules.Field("notes",
			rules.Assert("11", "invoice can have at most one note (ibr-sr-51)", is.Length(0, 1)),
		),
		rules.Field("lines", rules.Each(
			rules.Field("notes",
				rules.Assert("12", "line can have at most one note (ibr-sr-34)", is.Length(0, 1)),
			),
		)),
		// Due dates
		// Our own limit, not a PINT rule: only the payment due date (IBT-009) is written, with no amount.
		rules.Field("payment",
			rules.Field("terms",
				rules.Field("due_dates",
					rules.Assert("13", "invoice can have at most one due date, as only the payment due date is written (IBT-009)",
						is.Length(0, 1),
					),
				),
			),
		),
		rules.Assert("14", "due date must be for the whole amount due, as the payment due date has no amount (IBT-009)",
			is.Func("due date is for the amount due", dueDatesCoverAmountDue),
		),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			// AE supplier
			rules.Field("supplier",
				rules.Field("addresses",
					rules.Each(
						rules.Field("street",
							rules.Assert("15", "supplier address street is required (IBR-143-AE)", is.Present),
						),
						rules.Field("locality",
							rules.Assert("16", "supplier address city is required (IBR-143-AE)", is.Present),
						),
						rules.Field("region",
							rules.Assert("17", "supplier address region is required (IBR-143-AE)", is.Present),
						),
					),
				),
				// IBR-150-AE only applies with a 0235 endpoint, which every UAE Peppol participant has (UAE PASR).
				rules.Field("identities",
					rules.Assert("18", "supplier legal identity is required (IBR-150-AE)",
						is.Func("has legal scope identity", identitiesScopeIn(org.IdentityScopeLegal)),
					),
				),
				rules.Field("identities", rules.Each(
					rules.When(
						is.Func("tax scope", identityScopeIn(org.IdentityScopeTax)),
						rules.Assert("19", "supplier tax scope identity must be a TIN (IBR-148-AE)",
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
							rules.Assert("20", "customer address street is required (IBR-144-AE)", is.Present),
						),
						rules.Field("locality",
							rules.Assert("21", "customer address city is required (IBR-144-AE)", is.Present),
						),
						rules.Field("region",
							rules.Assert("22", "customer address region is required (IBR-144-AE)", is.Present),
						),
					),
				),
				rules.When(
					is.Func("has UAE Peppol endpoint other than 1XXXXXXXXX or 9XXXXXXXXX", partyHasUAEEndpoint(endpointPattern1or9, false)),
					rules.Field("identities",
						rules.Assert("23", "customer legal identity is required with a UAE Peppol endpoint other than 1XXXXXXXXX or 9XXXXXXXXX (IBR-149-AE)",
							is.Func("has legal scope identity", identitiesScopeIn(org.IdentityScopeLegal)),
						),
					),
				),
			),

			// AE currency
			rules.Assert("24", "exchange_rates must have a rate from the invoice currency to AED (IBR-159-AE)",
				currency.CanConvertTo(currency.AED),
			),
			rules.Assert("25", "exchange rate to AED must have at most 6 decimals (IBR-002-AE)",
				is.Func("AED rate has at most 6 decimals", exchangeRateToAEDHasAtMost6Decimals),
			),

			// AE VAT point date
			rules.Assert("26", "VAT point date must be before the invoice issue date (IBR-141-AE)",
				is.Func("VAT point date before issue date", pointDateBeforeIssueDate),
			),

			// AE payment
			rules.When(
				is.Not(is.Or(
					bill.InvoiceTypeIn(bill.InvoiceTypeCreditNote),
					is.Func("deemed supply", invoiceHasTags(TagDeemedSupply)),
				)),
				rules.Field("payment",
					rules.Assert("27", "payment details required, except on credit notes and deemed supplies (IBR-191-AE)",
						is.Present,
					),
					rules.Field("instructions",
						rules.Assert("28", "payment instructions required, except on credit notes and deemed supplies (IBR-191-AE)",
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
							rules.Assert("29", "payment terms are required when the amount due is positive, except on credit notes and deemed supplies (IBR-127-AE)",
								is.Present,
							),
							rules.Field("due_dates",
								rules.Assert("30", "payment due date is required when the amount due is positive, except on credit notes and deemed supplies (IBR-127-AE)",
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
					rules.Assert("31", "supplier tax ID code is required, except on outside scope documents (IBR-134-AE)",
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
						rules.Assert("32", "customer legal identity is required on outside scope documents or credit notes related to goods and services (IBR-136-AE)",
							is.Func("has legal scope identity", identitiesScopeIn(org.IdentityScopeLegal)),
						),
					),
				),
				rules.Field("lines", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Field("ext",
							rules.Assert("33", "line VAT category code must be E or O on outside scope documents or credit notes related to goods and services (IBR-122-AE)",
								tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
								tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesOutsideScope...),
							),
						),
					)),
				)),
				rules.Field("discounts", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Field("ext",
							rules.Assert("34", "discount VAT category code must be E or O on outside scope documents or credit notes related to goods and services (IBR-122-AE)",
								tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
								tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesOutsideScope...),
							),
						),
					)),
				)),
				rules.Field("charges", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Field("ext",
							rules.Assert("35", "charge VAT category code must be E or O on outside scope documents or credit notes related to goods and services (IBR-122-AE)",
								tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
								tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesOutsideScope...),
							),
						),
					)),
				)),
				rules.Assert("36", "transaction type cannot be summary, deemed supply or margin scheme on outside scope documents or credit notes related to goods and services (IBR-157-AE)",
					is.Not(is.Or(
						is.Func("summary", invoiceHasTags(TagSummary)),
						is.Func("deemed supply", invoiceHasTags(TagDeemedSupply)),
						is.Func("margin scheme", invoiceHasTags(TagMarginScheme)),
					)),
				),
			),

			// AE commercial invoices and credit notes
			rules.When(
				is.Func("document type is 380 or 381",
					invoiceExtIn(untdid.ExtKeyDocumentType, docTypeCommercialInvoice, docTypeCreditNote)),
				rules.Assert("37", "at least one line combo must be something other than E or O, or the invoice needs the outside-tax-scope tag (IBR-151-AE)",
					is.Func("has at least one line combo other than E or O", invoiceHasLineComboOtherThanEorO),
				),
			),

			// AE credit notes
			// IBR-124-AE names 381 and 81, but gobl.ubl writes a credit note's VAT point date where UBL doesn't allow it.
			rules.When(
				bill.InvoiceTypeIn(bill.InvoiceTypeCreditNote),
				rules.Field("value_date",
					rules.Assert("38", "VAT point date must not be there on credit notes (IBR-124-AE)",
						is.Not(is.Present),
					),
				),
			),
			// IBR-158-AE names only 381, but the IBR-055-AE test also fails 81 without a reason.
			rules.When(
				is.Func("document type is credit note or credit note related to goods and services",
					invoiceExtIn(untdid.ExtKeyDocumentType, docTypeCreditNote, docTypeCreditNoteGoodsServices)),
				rules.Field("tax",
					rules.Field("ext",
						rules.Assert("39", "credit note reason is required in the peppol-pint-ae-credit-note-reason extension (IBR-158-AE, IBR-055-AE)",
							tax.ExtensionsRequire(ExtKeyCreditNoteReason),
						),
					),
				),
				rules.When(
					is.Not(is.Func("credit note reason is missing or volume discount",
						invoiceExtIn(ExtKeyCreditNoteReason, cbc.CodeEmpty, CreditNoteReasonVolumeDiscount))),
					rules.Field("preceding",
						rules.Assert("40", "preceding invoice is required on credit notes, except for volume discounts (IBR-055-AE)",
							is.Present,
						),
					),
				),
				rules.When(
					is.Func("credit note reason is volume discount", invoiceExtIn(ExtKeyCreditNoteReason, CreditNoteReasonVolumeDiscount)),
					rules.Field("preceding",
						rules.Assert("41", "preceding invoice must not be there on volume discount credit notes (IBR-055-AE)",
							is.Not(is.Present),
						),
					),
				),
			),

			// AE transaction type
			rules.When(
				is.Func("free trade zone", invoiceHasTags(TagFreeTradeZone)),
				rules.Field("tax",
					rules.Field("ext",
						rules.Assert("42", "beneficiary ID is required on free trade zone supplies (IBR-007-AE)",
							tax.ExtensionsRequire(ExtKeyBeneficiaryID),
						),
					),
				),
			),
			rules.When(
				is.Func("deemed supply", invoiceHasTags(TagDeemedSupply)),
				rules.Field("customer",
					rules.AssertIfPresent("43", "customer electronic address must be the predefined endpoint 0235:9900000097 on deemed supplies (PINT AE BIS 1.5.3, IBT-049)",
						is.Func("has UAE Peppol endpoint 9900000097", partyHasUAEEndpoint(endpointPatternDeemedSupply, true)),
					),
				),
			),
			rules.When(
				is.Func("margin scheme", invoiceHasTags(TagMarginScheme)),
				rules.Field("lines", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.Field("ext",
							rules.Assert("44", "margin scheme invoice lines must be exempt without a UAE exemption reason, mapped to the margin scheme category N (IBR-116-AE)",
								tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, TaxCategoryMarginScheme),
							),
						),
					)),
				)),
				// Discounts and charges need a VAT category (BR-32, BR-37), and it can't be N (IBR-114-AE, IBR-115-AE).
				rules.Field("discounts",
					rules.Assert("45", "margin scheme invoice can't have discounts (IBR-116-AE)", is.Empty),
				),
				rules.Field("charges",
					rules.Assert("46", "margin scheme invoice can't have charges (IBR-116-AE)", is.Empty),
				),
			),
			rules.When(
				is.Func("summary invoice", invoiceHasTags(TagSummary)),
				rules.Field("ordering",
					rules.Assert("47", "ordering is required on summary invoices (IBR-138-AE)",
						is.Present,
					),
					rules.Field("period",
						rules.Assert("48", "ordering period is required on summary invoices (IBR-138-AE)",
							is.Present,
						),
					),
				),
			),
			rules.When(
				is.Func("disclosed agent billing", invoiceHasTags(TagDisclosedAgent)),
				rules.Field("tax",
					rules.Field("ext",
						rules.Assert("49", "principal ID is required on disclosed agent billing (IBR-137-AE)",
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
							rules.Assert("50", "supplier TRN or TIN is required on outside scope documents or credit notes related to goods and services (IBR-177-AE)",
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
					rules.Assert("51", "delivery is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
					rules.Field("receiver",
						rules.Assert("52", "delivery receiver is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
						rules.Field("addresses",
							rules.Assert("53", "delivery address is required on e-commerce supplies and exports (IBR-142-AE, IBR-152-AE)", is.Present),
						),
					),
				),
			),
			rules.When(
				is.Func("e-commerce supply", invoiceHasTags(TagECommerce)),
				rules.Field("delivery",
					rules.Field("receiver",
						rules.Field("addresses",
							rules.Each(
								rules.Field("street",
									rules.Assert("54", "e-commerce delivery address street is required (IBR-142-AE)", is.Present),
								),
								rules.Field("locality",
									rules.Assert("55", "e-commerce delivery address city is required (IBR-142-AE)", is.Present),
								),
								rules.Field("region",
									rules.Assert("56", "e-commerce delivery address region is required (IBR-142-AE)", is.Present),
								),
							),
						),
					),
				),
			),
			// IBR-152-AE's text forbids an AE delivery country on exports, but its test accepts one; we follow the test.
			rules.When(
				is.Func("export", invoiceHasTags(tax.TagExport)),
				rules.Field("delivery",
					rules.Field("receiver",
						rules.Field("addresses",
							rules.Each(
								rules.When(
									is.Not(is.Func("UAE address", addressIsAE)),
									rules.Field("street",
										rules.Assert("57", "export delivery address street is required (IBR-152-AE)", is.Present),
									),
									rules.Field("locality",
										rules.Assert("58", "export delivery address city is required (IBR-152-AE)", is.Present),
									),
									rules.Field("region",
										rules.Assert("59", "export delivery address region is required (IBR-152-AE)", is.Present),
									),
								),
							),
						),
					),
				),
			),
			rules.When(
				is.Not(is.Func("export", invoiceHasTags(tax.TagExport))),
				rules.Field("customer",
					rules.When(
						is.Func("has UAE Peppol endpoint that is not a TIN", partyHasUAEEndpoint(ae.TINPattern, false)),
						// gobl.ubl writes unscoped identities as cac:PartyIdentification.
						rules.When(
							is.Not(org.PartyHasTaxIDCode()),
							rules.Field("identities",
								rules.Assert("60", "customer tax ID, TIN or other identity besides the legal one is required with a UAE Peppol endpoint that is not a TIN, except on exports (IBR-135-AE)",
									is.Func("has tax scope or unscoped identity", identitiesScopeIn(org.IdentityScopeTax, cbc.KeyEmpty)),
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
							rules.Assert("61", "customer tax registration is required with reverse charge (IBR-103-AE)",
								is.Func("has tax scope identity", identitiesScopeIn(org.IdentityScopeTax)),
							),
						),
					),
				),
			),

			// AE tax extensions
			rules.Field("tax",
				rules.Field("ext",
					rules.Assert("62", "credit note reason must be from the Reasons for credit note code list (IBR-001-AE)",
						tax.ExtensionHasValidCode(ExtKeyCreditNoteReason),
					),
					rules.Assert("63", "principal ID must be a TRN of 15 digits starting with 1 and ending with 03 (IBR-132-AE)",
						tax.ExtensionHasValidCode(ExtKeyPrincipalID),
					),
				),
				rules.Field("point",
					rules.Assert("64", "tax point must not be set, as PINT AE uses its element for the billing frequency (IBR-005-AE)",
						is.Not(is.Present),
					),
				),
			),

			// AE document type list
			rules.Field("tax",
				rules.Field("ext",
					rules.Assert("65", "document type must be 380, 480, 381, 81, 389 or 261 (ibr-cl-01)",
						tax.ExtensionsHasCodes(untdid.ExtKeyDocumentType, docTypesAE...),
					),
				),
			),
		),

		// Currency decimals
		// PINT AE validates against PINT 1.1.4, which allows each currency's precision; the others use 1.1.3, which allows 2.
		// TODO: use the currency precision everywhere once the other VESIDs move to PINT 1.1.4.
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.Field("currency",
				rules.Assert("66", "currency must not have more decimals than PINT allows for it (ibr-091, ibr-121, ibr-122, ibr-123, ibr-124, ibr-125)",
					is.Func("currency decimals within the PINT precision", currencyHasPINTDecimals),
				),
			),
		),
		rules.When(
			is.Not(is.InContext(tax.RegimeIn(regimesAE...))),
			rules.Field("currency",
				rules.Assert("66", "currency must have at most 2 decimals (ibr-091, ibr-121, ibr-122, ibr-123, ibr-124, ibr-125)",
					is.Func("currency has at most 2 decimals", currencyHasAtMost2Decimals),
				),
			),
		),

		// Tags
		// GOBL can't limit an addon's tags to a regime, so the PINT AE ones are rejected elsewhere.
		rules.When(
			is.Not(is.InContext(tax.RegimeIn(regimesAE...))),
			rules.Assert("67", "PINT AE tags can only be used on UAE documents",
				is.Func("has no PINT AE tags", invoiceHasNoAETags),
			),
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

// invoiceHasNoAETags reports whether the invoice has none of the tags only the PINT addon defines.
// Tags that the regime, another addon or GOBL also define don't count.
func invoiceHasNoAETags(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	others := bill.DefaultInvoiceTags()
	if r := inv.RegimeDef(); r != nil {
		others = others.Merge(tax.TagSetForSchema(r.Tags, bill.ShortSchemaInvoice))
	}
	for _, a := range inv.AddonDefs() {
		if a.Key != V1 {
			others = others.Merge(tax.TagSetForSchema(a.Tags, bill.ShortSchemaInvoice))
		}
	}
	keys := others.Keys()
	for _, def := range invoiceTags.List {
		if inv.HasTags(def.Key) && !def.Key.In(keys...) {
			return false
		}
	}
	return true
}

// currencyDecimalsPINT are the currencies whose official precision in PINT 1.1.4
// is other than 2 decimals. PINT also gives UYW 4, but GOBL has no UYW.
var currencyDecimalsPINT = map[currency.Code]uint32{
	currency.BHD: 3, currency.IQD: 3, currency.JOD: 3, currency.KWD: 3,
	currency.LYD: 3, currency.OMR: 3, currency.TND: 3,
	currency.CLF: 4,
}

// currencyHasPINTDecimals reports whether GOBL's decimals for the currency fit its PINT 1.1.4 precision.
func currencyHasPINTDecimals(val any) bool {
	code, ok := val.(currency.Code)
	if !ok {
		return false
	}
	def := code.Def()
	if def == nil {
		return false
	}
	limit, ok := currencyDecimalsPINT[code]
	if !ok {
		limit = 2
	}
	return def.Subunits <= limit
}

// currencyHasAtMost2Decimals reports whether GOBL's decimals for the currency are 2 at most (PINT 1.1.3).
func currencyHasAtMost2Decimals(val any) bool {
	code, ok := val.(currency.Code)
	if !ok {
		return false
	}
	def := code.Def()
	return def != nil && def.Subunits <= 2
}

// exchangeRateToAEDHasAtMost6Decimals reports whether the rate to AED, if any, has at most 6 decimals.
func exchangeRateToAEDHasAtMost6Decimals(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	rate := currency.MatchExchangeRate(inv.ExchangeRates, inv.Currency, currency.AED)
	return rate == nil || rate.Amount.Exp() <= 6
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

// dueDatesCoverAmountDue reports whether each due date is for the whole amount due, as 100% or as
// that amount. Due dates with neither pass.
func dueDatesCoverAmountDue(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Payment == nil || inv.Payment.Terms == nil || inv.Totals == nil {
		return true
	}
	due := inv.Totals.Payable
	if inv.Totals.Due != nil {
		due = *inv.Totals.Due
	}
	for _, dd := range inv.Payment.Terms.DueDates {
		switch {
		case dd == nil:
		case dd.Percent != nil:
			if !dd.Percent.Equals(num.MakePercentage(1, 0)) {
				return false
			}
		case dd.Amount != nil && !dd.Amount.Equals(due):
			return false
		}
	}
	return true
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
