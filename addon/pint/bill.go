package pint

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// billInvoiceRules enforces the PINT invoice rules shared by every
// jurisdiction, and the PINT AE rules on UAE documents.
func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		// Supplier
		rules.Field("supplier",
			rules.Assert("01", "invoice supplier must have a tax ID or identity (IBR-CO-26)",
				is.Func("has identifier", partyHasIdentifier),
			),
			rules.Field("endpoints",
				rules.Assert("02", "invoice supplier electronic address is required (IBR-081)", is.Present),
			),
		),
		// Customer
		rules.Field("customer",
			rules.Assert("03", "invoice customer is required (IBR-007)", is.Present),
			rules.Field("endpoints",
				rules.Assert("04", "invoice customer electronic address is required (IBR-080)", is.Present),
			),
		),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			// PINT has no BR-CO-25. PINT AE asks for payment instructions
			// (IBR-191-AE, rules 27 and 28) and a due date (IBR-127-AE, rule 29)
			// instead, except on credit notes and deemed supplies.
			rules.Ignore("GOBL-EU-EN16931-BILL-INVOICE-06"),
			rules.Ignore("GOBL-EU-EN16931-BILL-INVOICE-07"),
			// EN 16931 wants an EU VATEX code or an exemption note for every E, O
			// and AE breakdown (BR-E-10, BR-O-10, BR-AE-10). PINT AE has no such
			// rule: it only asks for the UAE exemption reason on exempt lines
			// (IBR-167-AE), and no official example has a reason on a breakdown.
			rules.Ignore("GOBL-EU-EN16931-BILL-INVOICE-08"),

			// AE supplier
			//
			// IBR-128-AE's text names the seller, buyer, tax representative and
			// delivery subdivisions (IBT-039, IBT-054, IBT-068, IBT-079), but its
			// test never runs on the seller's and buyer's addresses: its context,
			// cac:PostalAddress, comes after their address rules (IBR-143-AE,
			// IBR-144-AE) in the same pattern, where a node only fires the first
			// rule it matches. Rules 06 and 13 follow the text there, as the
			// official examples do, so they check more than the test. Rules 21
			// and 22, and line rule 11, follow the test: gobl.ubl also writes the
			// issuer and the line sellers with a cac:PostalAddress.
			rules.Field("supplier",
				rules.Assert("05", "supplier address must have a street, city and region (IBR-143-AE)",
					is.Func("first address has street, city and region", partyAddressIsComplete),
				),
				rules.Assert("06", "supplier region must be an emirate code when the country is AE (IBR-128-AE)",
					is.Func("first address region is an emirate", partyAddressHasEmirate),
				),
				rules.When(
					is.Func("has UAE Peppol endpoint", partyHasUAEEndpoint),
					rules.Assert("07", "supplier legal identity is required with a UAE Peppol endpoint (IBR-150-AE)",
						is.Func("has legal identity", partyHasLegalIdentity),
					),
					rules.Field("identities", rules.Each(
						rules.When(
							is.Func("legal scope", identityHasLegalScope),
							rules.Assert("08", "supplier legal identity must be a trade license, Emirates ID, passport or Cabinet Decision with a UAE Peppol endpoint (IBR-173-AE, IBR-181-AE)",
								is.Or(org.IdentityKeyIn(org.IdentityKeyPassport), org.IdentityTypeIn(identityTypesAE...)),
							),
						),
					)),
				),
				rules.Field("identities", rules.Each(
					rules.When(
						org.IdentityKeyIn(org.IdentityKeyPassport),
						rules.Field("country",
							rules.Assert("09", "supplier passport must have the issuing country (IBR-012-AE)", is.Present),
						),
					),
					rules.When(
						org.IdentityTypeIn(ae.IdentityTypeTradeLicense),
						rules.Field("description",
							rules.Assert("10", "supplier trade license must have the issuing authority in the description (IBR-172-AE)", is.Present),
						),
					),
					// IBR-178-AE needs no rule: EN 16931 allows one tax scope identity
					// (GOBL-EU-EN16931-ORG-PARTY-03), which, with the TRN, is the two
					// PartyTaxSchemes with one VAT that it accepts.
					rules.When(
						is.Func("tax scope", identityHasTaxScope),
						rules.Field("code",
							rules.Assert("11", "supplier tax scope identity must be a TIN of 10 digits starting with 1 (IBR-148-AE)",
								is.MatchesRegexp(tinRegexp),
							),
						),
					),
				)),
			),

			// AE customer
			rules.Field("customer",
				rules.Assert("12", "customer address must have a street, city and region (IBR-144-AE)",
					is.Func("first address has street, city and region", partyAddressIsComplete),
				),
				rules.Assert("13", "customer region must be an emirate code when the country is AE (IBR-128-AE)",
					is.Func("first address region is an emirate", partyAddressHasEmirate),
				),
				// TODO: ibr-080 is a shared PINT rule, so every PINT jurisdiction
				// needs this: rule 04 only checks that there is an endpoint, but
				// gobl.ubl writes none as cbc:EndpointID for other URI schemes.
				rules.Assert("14", "customer electronic address must be an ISO 6523 or email endpoint, or an inbox (IBR-080)",
					is.Func("electronic address written as endpoint ID", partyHasEndpointID),
				),
				// TODO: ibr-sr-16 is a shared PINT rule, and EN 16931 has it too
				// (UBL-SR-16), so every PINT jurisdiction and BIS need this.
				rules.Assert("15", "customer can have at most one identity besides its first legal one and its tax ones (ibr-sr-16)",
					is.Func("at most one buyer identifier", partyHasOneIdentification),
				),
				// IBR-183-AE's test also accepts CL, which only its rule text's
				// "Commercial/Trade license" explains. FormatAE reads CL as a
				// trade licence on import, but GOBL writes TL, which is valid for
				// both seller and buyer.
				rules.When(
					is.Func("has UAE Peppol endpoint", partyHasUAEEndpoint),
					rules.Field("identities", rules.Each(
						rules.When(
							is.Func("legal scope", identityHasLegalScope),
							rules.Assert("16", "customer legal identity must be a trade license, Emirates ID, passport or Cabinet Decision with a UAE Peppol endpoint (IBR-180-AE, IBR-183-AE)",
								is.Or(org.IdentityKeyIn(org.IdentityKeyPassport), org.IdentityTypeIn(identityTypesAE...)),
							),
						),
					)),
				),
				rules.Field("identities", rules.Each(
					rules.When(
						org.IdentityKeyIn(org.IdentityKeyPassport),
						rules.Field("country",
							rules.Assert("17", "customer passport must have the issuing country (IBR-010-AE)", is.Present),
						),
					),
					rules.When(
						org.IdentityTypeIn(ae.IdentityTypeTradeLicense),
						rules.Field("description",
							rules.Assert("18", "customer trade license must have the issuing authority in the description (IBR-101-AE)", is.Present),
						),
					),
				)),
				// IBR-149-AE's text asks for the buyer legal registration identifier
				// (IBT-047), but its test checks the tax registration,
				// cac:PartyTaxScheme/cbc:CompanyID. We follow the test.
				rules.When(
					is.Func("has UAE Peppol endpoint not starting with 1 or 9", partyHasUAEEndpointNotStartingWith1or9),
					rules.Assert("19", "customer tax registration is required with a UAE Peppol endpoint not starting with 1 or 9 (IBR-149-AE)",
						is.Func("has tax registration", partyHasTaxRegistration),
					),
				),
				rules.Assert("20", "customer must have at most one tax registration (IBR-179-AE)",
					is.Func("at most one tax registration", partyHasAtMostOneTaxRegistration),
				),
			),

			// AE tax representative and issuer
			rules.Field("ordering",
				rules.Field("seller",
					rules.Assert("21", "tax representative region must be an emirate code when the country is AE (IBR-128-AE)",
						is.Func("postal address region is an emirate", partyPostalAddressHasEmirate),
					),
				),
				rules.Field("issuer",
					rules.Assert("22", "issuer region must be an emirate code when the country is AE (IBR-128-AE)",
						is.Func("postal address region is an emirate", partyPostalAddressHasEmirate),
					),
				),
			),

			// AE tax schemes of every party
			//
			// IBR-133-AE's test runs on every cac:TaxScheme/cbc:ID: a scheme
			// other than VAT needs a seller TIN. gobl.ubl writes tax schemes for
			// the customer, tax representative, issuer, payee and line sellers.
			// The supplier's own pass: its tax ID is under VAT, and rule 11 makes
			// any tax scope identity of it a TIN. Every tax category here is VAT.
			rules.Assert("23", "tax registrations other than VAT require a supplier TIN (IBR-133-AE)",
				is.Func("non-VAT tax registrations with supplier TIN", invoiceNonVATTaxRegistrationsHaveSupplierTIN),
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
					is.Func("deemed supply", invoiceIsDeemedSupply),
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
			// Self-billed credit notes (261) are exempt too. The rule text only
			// names 381 and 81, but the Schematron test also exempts 261, and we
			// follow the test.
			rules.When(
				is.Not(is.Or(
					is.Func("document type is 381, 81 or 261",
						invoiceExtIn(untdid.ExtKeyDocumentType, docTypeCreditNote, docTypeCreditNoteGoodsServices, docTypeSelfBilledCreditNote)),
					is.Func("deemed supply", invoiceIsDeemedSupply),
				)),
				rules.When(
					is.Func("amount due is positive", invoiceIsDue),
					rules.Assert("29", "payment due date is required when the amount due is positive, except on credit notes and deemed supplies (IBR-127-AE)",
						is.Func("has due date", invoiceHasDueDate),
					),
				),
			),

			// AE document types: all but outside scope (480, 81)
			rules.When(
				is.Not(is.Func("document type is invoice outside scope or credit note related to goods and services",
					invoiceExtIn(untdid.ExtKeyDocumentType, docTypeInvoiceOutsideScope, docTypeCreditNoteGoodsServices))),
				rules.Field("supplier",
					rules.Assert("30", "supplier tax ID code is required, except on outside scope documents (IBR-134-AE)",
						org.PartyHasTaxIDCode(),
					),
				),
			),

			// AE document types: outside scope (480, 81)
			rules.When(
				is.Func("document type is invoice outside scope or credit note related to goods and services",
					invoiceExtIn(untdid.ExtKeyDocumentType, docTypeInvoiceOutsideScope, docTypeCreditNoteGoodsServices)),
				rules.Field("customer",
					rules.Assert("31", "customer legal identity is required on outside scope documents or credit notes related to goods and services (IBR-136-AE)",
						is.Func("has legal identity", partyHasLegalIdentity),
					),
				),
				rules.Field("lines", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.When(
							is.Func("VAT", comboIsVAT),
							rules.Field("ext",
								rules.Assert("32", "line VAT category code must be an E, Z, or O on outside scope documents or credit notes related to goods and services (IBR-122-AE)",
									tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
									tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesOutsideScope...),
								),
							),
						),
					)),
				)),
				rules.Field("discounts", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.When(
							is.Func("VAT", comboIsVAT),
							rules.Field("ext",
								rules.Assert("33", "discount VAT category code must be an E, Z, or O on outside scope documents or credit notes related to goods and services (IBR-122-AE)",
									tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
									tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesOutsideScope...),
								),
							),
						),
					)),
				)),
				rules.Field("charges", rules.Each(
					rules.Field("taxes", rules.Each(
						rules.When(
							is.Func("VAT", comboIsVAT),
							rules.Field("ext",
								rules.Assert("34", "charge VAT category code must be an E, Z, or O on outside scope documents or credit notes related to goods and services (IBR-122-AE)",
									tax.ExtensionsRequire(untdid.ExtKeyTaxCategory),
									tax.ExtensionsHasCodes(untdid.ExtKeyTaxCategory, taxCategoryCodesOutsideScope...),
								),
							),
						),
					)),
				)),
				rules.Assert("35", "transaction type cannot be summary, deemed supply or margin scheme on outside scope documents or credit notes related to goods and services (IBR-157-AE)",
					is.Not(is.Func("summary, deemed supply or margin scheme", invoiceIsSummaryDeemedOrMargin)),
				),
			),

			// AE document types: commercial invoice and credit note (380, 381)
			rules.When(
				is.Func("document type is 380 or 381",
					invoiceExtIn(untdid.ExtKeyDocumentType, docTypeCommercialInvoice, docTypeCreditNote)),
				rules.Assert("36", "at least one line combo must be something other than E or O (IBR-151-AE)",
					is.Func("has at least one line combo other than E or O", invoiceHasLineComboOtherThanEorO),
				),
			),

			// AE document types: credit notes (381, 81)
			//
			// IBR-158-AE only names 381, but IBR-055-AE's test fails 81 credit
			// notes without a reason too, whether or not they have a preceding
			// invoice, so both need one.
			rules.When(
				is.Func("document type is credit note or credit note related to goods and services",
					invoiceExtIn(untdid.ExtKeyDocumentType, docTypeCreditNote, docTypeCreditNoteGoodsServices)),
				rules.Field("value_date",
					rules.Assert("37", "VAT point date must not be there on credit notes or credit notes related to goods and services (IBR-124-AE)",
						is.Not(is.Present),
					),
				),
				rules.Field("tax",
					rules.Field("ext",
						rules.Assert("38", "credit note reason is required (IBR-158-AE, IBR-055-AE)",
							tax.ExtensionsRequire(ExtKeyCreditNoteReason),
						),
					),
				),
				rules.When(
					// A credit note without a reason is reported by the rule above.
					is.Not(is.Func("credit note reason is missing or volume discount",
						invoiceExtIn(ExtKeyCreditNoteReason, cbc.CodeEmpty, CreditNoteReasonVolumeDiscount))),
					rules.Field("preceding",
						rules.Assert("39", "preceding invoice is required on credit notes, except for volume discounts (IBR-055-AE)",
							is.Present,
						),
					),
				),
				rules.When(
					is.Func("credit note reason is volume discount", invoiceExtIn(ExtKeyCreditNoteReason, CreditNoteReasonVolumeDiscount)),
					rules.Field("preceding",
						rules.Assert("40", "preceding invoice must not be there on volume discount credit notes (IBR-055-AE)",
							is.Not(is.Present),
						),
					),
				),
			),

			// AE transaction type
			rules.Field("tax",
				rules.Field("ext",
					rules.Assert("41", "transaction type is required, as 8 flags of 0 or 1 (IBR-154-AE)",
						tax.ExtensionsRequire(ExtKeyTransactionType),
						tax.ExtensionHasValidCode(ExtKeyTransactionType),
					),
				),
			),
			rules.When(
				is.Func("free trade zone", invoiceIsFreeTradeZone),
				rules.Field("tax",
					rules.Field("ext",
						rules.Assert("42", "beneficiary ID is required on free trade zone supplies (IBR-007-AE)",
							tax.ExtensionsRequire(ExtKeyBeneficiaryID),
						),
					),
				),
			),
			// With the discount and charge rules 02, a margin scheme invoice can
			// have no discounts or charges, as GOBL requires them to have taxes
			// (BR-32, BR-37).
			rules.When(
				is.Func("margin scheme", invoiceIsMarginScheme),
				rules.Assert("43", "margin scheme invoice must only have exempt lines without a UAE exemption reason, mapped to the margin scheme category N, and no other VAT treatments (IBR-116-AE)",
					is.Func("only margin scheme VAT", invoiceTaxesAreMarginScheme),
				),
			),
			rules.When(
				is.Func("summary invoice", invoiceIsSummary),
				rules.Assert("44", "ordering period or billing frequency is required on summary invoices (IBR-138-AE)",
					is.Func("has invoice period", invoiceHasInvoicePeriod),
				),
			),
			rules.When(
				is.Func("disclosed agent billing", invoiceIsDisclosedAgent),
				rules.Field("tax",
					rules.Field("ext",
						rules.Assert("45", "principal ID is required on disclosed agent billing (IBR-137-AE)",
							tax.ExtensionsRequire(ExtKeyPrincipalID),
						),
					),
				),
				rules.Assert("46", "principal ID must differ from the supplier tax ID on disclosed agent billing (IBR-176-AE)",
					is.Func("principal ID is not the supplier tax ID", principalIsNotSupplier),
				),
				rules.Field("supplier",
					rules.Assert("47", "supplier tax registration is required on disclosed agent billing (IBR-177-AE)",
						is.Func("has tax registration", partyHasTaxRegistration),
					),
				),
			),
			rules.When(
				is.Func("e-commerce supply", invoiceIsECommerce),
				rules.Assert("48", "delivery address must have a street, city and region on e-commerce supplies (IBR-142-AE)",
					is.Func("delivery address is complete", invoiceDeliveryAddressIsComplete),
				),
			),
			// IBR-152-AE's text says the delivery country should not be AE on
			// exports, but its test accepts a delivery address in the UAE, and
			// only asks the others for a street, city and region. We follow the
			// test, which fails exports without a delivery address.
			rules.When(
				is.Func("export", invoiceIsExport),
				rules.Assert("49", "delivery address outside the UAE must have a street, city and region on exports (IBR-152-AE)",
					is.Func("delivery address is in the UAE or complete", invoiceDeliveryAddressIsAEOrComplete),
				),
			),
			rules.When(
				is.Not(is.Func("export", invoiceIsExport)),
				rules.Field("customer",
					rules.When(
						is.Func("has UAE Peppol endpoint that is not a TIN", partyHasUAEEndpointNotTIN),
						rules.Assert("50", "customer identity or tax registration is required with a UAE Peppol endpoint that is not a TIN, except on exports (IBR-135-AE)",
							is.Func("has identifier or tax registration", partyHasIdentificationOrTaxRegistration),
						),
					),
				),
			),

			// AE reverse charge
			rules.When(
				is.Func("has reverse charge lines", invoiceHasReverseChargeLine),
				rules.Field("customer",
					rules.Assert("51", "customer tax registration is required with reverse charge (IBR-103-AE)",
						is.Func("has tax registration", partyHasTaxRegistration),
					),
				),
			),

			// AE tax extensions
			rules.Field("tax",
				rules.Field("ext",
					rules.Assert("52", "credit note reason must be from the Reasons for credit note code list (IBR-001-AE)",
						tax.ExtensionHasValidCode(ExtKeyCreditNoteReason),
					),
					rules.Assert("53", "billing frequency must be from the frequency of billing code list (IBR-005-AE)",
						tax.ExtensionHasValidCode(ExtKeyBillingFrequency),
					),
				),
				// PINT AE writes the billing frequency where EN writes the VAT
				// point date code (BT-8), cac:InvoicePeriod/cbc:DescriptionCode,
				// and IBR-005-AE only accepts frequencies there.
				rules.Field("point",
					rules.Assert("54", "tax point must not be set, as its code goes where the billing frequency does (IBR-005-AE)",
						is.Not(is.Present),
					),
				),
			),
			rules.When(
				is.Func("billing frequency is Others", invoiceExtIn(ExtKeyBillingFrequency, BillingFrequencyOthers)),
				rules.Field("notes",
					rules.Assert("55", "a note is required with the billing frequency Others (IBR-160-AE)",
						is.Present,
					),
				),
			),
		),

		// Tags
		//
		// The tags the addon defines are for UAE documents, but GOBL can't limit
		// an addon's tags to a regime. Other documents can only use the ones
		// their regime, another addon or GOBL itself defines too, as before the
		// addon had tags.
		rules.When(
			is.Not(is.InContext(tax.RegimeIn(regimesAE...))),
			rules.Assert("56", "PINT AE tags can only be used on UAE documents",
				is.Func("has no PINT AE tags", invoiceHasNoAETags),
			),
		),
	)
}

// billDiscountRules enforces the PINT AE document level discount rules on UAE
// documents.
func billDiscountRules() *rules.Set {
	return rules.For(new(bill.Discount),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			// IBR-168-AE names the exemption reason code, but its test checks the
			// allowance reason code, cbc:AllowanceChargeReasonCode. We follow the
			// test.
			rules.When(
				is.Func("has exempt VAT", taxesIn(comboIsExemptVAT)),
				rules.Field("ext",
					rules.Assert("01", "exempt discount needs an allowance reason code (IBR-168-AE)",
						tax.ExtensionsRequire(untdid.ExtKeyAllowance),
					),
				),
			),
			rules.Field("taxes", rules.Each(
				rules.When(
					is.Func("VAT", comboIsVAT),
					rules.Field("ext",
						rules.Assert("02", "discount VAT category cannot be margin scheme (IBR-115-AE)",
							tax.ExtensionsExcludeCodes(untdid.ExtKeyTaxCategory, TaxCategoryMarginScheme),
						),
					),
				),
			)),
		),
	)
}

// billChargeRules enforces the PINT AE document level charge rules on UAE
// documents.
func billChargeRules() *rules.Set {
	return rules.For(new(bill.Charge),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			// IBR-169-AE names the exemption reason code, but its test checks the
			// charge reason code, cbc:AllowanceChargeReasonCode. We follow the
			// test.
			rules.When(
				is.Func("has exempt VAT", taxesIn(comboIsExemptVAT)),
				rules.Field("ext",
					rules.Assert("01", "exempt charge needs a charge reason code (IBR-169-AE)",
						tax.ExtensionsRequire(untdid.ExtKeyCharge),
					),
				),
			),
			rules.Field("taxes", rules.Each(
				rules.When(
					is.Func("VAT", comboIsVAT),
					rules.Field("ext",
						rules.Assert("02", "charge VAT category cannot be margin scheme (IBR-114-AE)",
							tax.ExtensionsExcludeCodes(untdid.ExtKeyTaxCategory, TaxCategoryMarginScheme),
						),
					),
				),
			)),
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

// invoiceHasNoAETags reports whether the invoice has none of the tags only
// the PINT addon defines. A tag its regime, another addon or GOBL itself
// defines with the same key, such as the Spanish "summary", is not one.
func invoiceHasNoAETags(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
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

// exchangeRateToAEDHasAtMost6Decimals reports whether the exchange rate from
// the invoice currency to AED has at most 6 decimals, the limit for
// cbc:CalculationRate (IBR-002-AE). It is the only rate written. Invoices
// without it pass: they are in AED, or rule 24 reports them.
func exchangeRateToAEDHasAtMost6Decimals(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	rate := currency.MatchExchangeRate(inv.ExchangeRates, inv.Currency, currency.AED)
	return rate == nil || rate.Amount.Exp() <= 6
}

// invoiceHasReverseChargeLine reports whether any line of the invoice has a
// VAT combo with the reverse charge category AE, the code IBR-103-AE tests.
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
	return inv.IssueDate.DaysSince(inv.ValueDate.Date) > 0
}

// invoiceIsDue reports whether the amount gobl.ubl writes as
// cbc:PayableAmount is greater than zero, as IBR-127-AE tests it: the due
// total when set, the payable total otherwise.
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

// invoiceNonVATTaxRegistrationsHaveSupplierTIN reports whether the supplier
// has a TIN (IBT-032) whenever another party has a tax registration that
// gobl.ubl writes under a tax scheme other than VAT: a tax scope identity
// whose type isn't VAT, or a tax ID from a regime with another scheme, such
// as GST in Australia. PINT AE only allows those when the seller has a TIN
// (IBR-133-AE). The parties are the ones gobl.ubl writes tax schemes for:
// the customer, tax representative, issuer, payee and line sellers. The
// official exports example sends a foreign buyer's tax number as its
// electronic address and legal identifier instead.
func invoiceNonVATTaxRegistrationsHaveSupplierTIN(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	parties := []*org.Party{inv.Customer}
	if inv.Ordering != nil {
		parties = append(parties, inv.Ordering.Seller, inv.Ordering.Issuer)
	}
	if inv.Payment != nil {
		parties = append(parties, inv.Payment.Payee)
	}
	for _, l := range inv.Lines {
		if l != nil {
			parties = append(parties, l.Seller)
		}
	}
	for _, p := range parties {
		if partyHasNonVATTaxRegistration(p) {
			return partyHasTIN(inv.Supplier)
		}
	}
	return true
}

// partyHasNonVATTaxRegistration reports whether gobl.ubl writes a tax scheme
// other than VAT for the party: its tax ID's, or a tax scope identity's,
// which is the identity type, or LOC without one.
func partyHasNonVATTaxRegistration(p *org.Party) bool {
	if p == nil {
		return false
	}
	if partyTaxIDIsNotVAT(p) {
		return true
	}
	for _, id := range p.Identities {
		if identityHasTaxScope(id) && id.Type != tax.CategoryVAT {
			return true
		}
	}
	return false
}

// partyHasTIN reports whether the party has a tax scope identity with the
// code IBR-133-AE looks for in the seller's tax schemes: 10 digits starting
// with 1.
func partyHasTIN(p *org.Party) bool {
	if p == nil {
		return false
	}
	for _, id := range p.Identities {
		if identityHasTaxScope(id) && tinRegexp.MatchString(id.Code.String()) {
			return true
		}
	}
	return false
}

// partyTaxIDIsNotVAT reports whether the party has a tax ID code whose scheme
// is not VAT. gobl.ubl writes a tax ID without a scheme under VAT.
func partyTaxIDIsNotVAT(p *org.Party) bool {
	if p.TaxID == nil || p.TaxID.Code == "" {
		return false
	}
	scheme := p.TaxID.GetScheme()
	return scheme != "" && scheme != tax.CategoryVAT
}

// normalizeRounding gives UAE invoices without a rounding rule the currency
// one. PINT sends each line amount rounded to the currency, and each VAT
// breakdown must equal the sum of its lines (ALIGNED-IBRP-S-08 and the other
// breakdown rules). With the default precise rule, the breakdown is the sum
// of the unrounded lines, which can differ by a few cents.
func normalizeRounding(inv *bill.Invoice) {
	if inv == nil {
		return
	}
	if inv.Tax == nil {
		inv.Tax = new(bill.Tax)
	}
	if inv.Tax.Rounding == "" {
		inv.Tax.Rounding = tax.RoundingRuleCurrency
	}
}

// invoiceHasInvoicePeriod reports whether the invoice has what is written as
// cac:InvoicePeriod: an ordering period or a billing frequency. IBR-138-AE's
// text asks for the invoicing period, but its test only checks the element
// is there, so the frequency alone passes.
func invoiceHasInvoicePeriod(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	return (inv.Ordering != nil && inv.Ordering.Period != nil) ||
		inv.Tax.GetExt(ExtKeyBillingFrequency) != ""
}

// principalIsNotSupplier reports whether the principal ID differs from the
// supplier's tax registrations, the cbc:CompanyID values of its
// PartyTaxSchemes (IBR-176-AE). A missing principal ID or supplier tax
// registration passes, as rules 45 and 47 report those.
func principalIsNotSupplier(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	principal := inv.Tax.GetExt(ExtKeyPrincipalID)
	if principal == "" || inv.Supplier == nil {
		return true
	}
	var codes []cbc.Code
	if inv.Supplier.TaxID != nil && inv.Supplier.TaxID.Code != "" {
		codes = append(codes, inv.Supplier.TaxID.Code)
	}
	for _, id := range inv.Supplier.Identities {
		if identityHasTaxScope(id) {
			codes = append(codes, id.Code)
		}
	}
	if len(codes) == 0 {
		return true
	}
	for _, c := range codes {
		if c != principal {
			return true
		}
	}
	return false
}

// invoiceDeliveryAddress returns the first address of the delivery receiver,
// which gobl.ubl writes as cac:DeliveryLocation/cac:Address, or nil.
func invoiceDeliveryAddress(inv *bill.Invoice) *org.Address {
	if inv == nil || inv.Delivery == nil || inv.Delivery.Receiver == nil {
		return nil
	}
	return partyFirstAddress(inv.Delivery.Receiver)
}

// invoiceDeliveryAddressIsComplete reports whether the invoice has a delivery
// address with a street, city and region (IBR-142-AE).
func invoiceDeliveryAddressIsComplete(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok {
		return false
	}
	a := invoiceDeliveryAddress(inv)
	return a != nil && a.Street != "" && a.Locality != "" && a.Region != ""
}

// invoiceDeliveryAddressIsAEOrComplete reports whether the invoice has a
// delivery address in the UAE, or one with a street, city and region
// (IBR-152-AE).
func invoiceDeliveryAddressIsAEOrComplete(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok {
		return false
	}
	a := invoiceDeliveryAddress(inv)
	return a != nil && (a.Country == l10n.AE.ISO() || (a.Street != "" && a.Locality != "" && a.Region != ""))
}

// taxesIn returns a predicate that reports whether a discount or charge has
// a tax combo that check accepts.
func taxesIn(check func(any) bool) func(any) bool {
	return func(val any) bool {
		var set tax.Set
		switch v := val.(type) {
		case *bill.Discount:
			if v != nil {
				set = v.Taxes
			}
		case *bill.Charge:
			if v != nil {
				set = v.Taxes
			}
		}
		for _, c := range set {
			if check(c) {
				return true
			}
		}
		return false
	}
}

// invoiceTaxesAreMarginScheme reports whether every tax combo of an invoice's
// lines, discounts and charges is VAT under the profit margin scheme.
func invoiceTaxesAreMarginScheme(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	var sets []tax.Set
	for _, l := range inv.Lines {
		if l != nil {
			sets = append(sets, l.Taxes)
		}
	}
	for _, d := range inv.Discounts {
		if d != nil {
			sets = append(sets, d.Taxes)
		}
	}
	for _, c := range inv.Charges {
		if c != nil {
			sets = append(sets, c.Taxes)
		}
	}
	for _, set := range sets {
		for _, c := range set {
			if !comboIsMarginScheme(c) {
				return false
			}
		}
	}
	return true
}
