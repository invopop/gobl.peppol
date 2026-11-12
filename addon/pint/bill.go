package pint

import (
	"strings"

	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
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

// endpointSchemeUAE is the Peppol participant identifier scheme for UAE
// endpoints, which the PINT AE legal registration rules depend on.
const endpointSchemeUAE cbc.Code = "0235"

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
			// EN 16931 wants an EU VATEX code or an exemption note for every E, O
			// and AE breakdown (BR-E-10, BR-O-10, BR-AE-10). PINT AE has no such
			// rule: it only asks for the UAE exemption reason on exempt lines
			// (IBR-167-AE), and no official example has a reason on a breakdown.
			rules.Ignore("GOBL-EU-EN16931-BILL-INVOICE-08"),
			rules.Assert("05", "exchange_rates must have a rate from the invoice currency to AED (IBR-159-AE)",
				is.Func("has AED exchange rate", exchangeRateHasAED),
			),
			rules.Assert("06", "exchange rate to AED must have at most 6 decimals (IBR-002-AE)",
				is.Func("AED rate has at most 6 decimals", exchangeRateToAEDHasAtMost6Decimals),
			),
			rules.When(
				is.Not(is.Or(
					is.Func("credit note", invoiceIsCreditNote),
					is.Func("deemed supply", invoiceIsDeemedSupply),
				)),
				rules.Assert("07", "payment instructions required, except on credit notes and deemed supplies (IBR-191-AE)",
					is.Func("has payment instructions", invoiceHasPaymentInstructions),
				),
			),
			rules.When(
				is.Not(is.Func("document type is invoice outside scope or credit note related to goods and services",
					documentTypeIn(docTypeInvoiceOutsideScope, docTypeCreditNoteGoodsServices))),
				rules.Field("supplier",
					rules.Assert("08", "supplier tax ID code is required, except on outside scope documents (IBR-134-AE)",
						org.PartyHasTaxIDCode(),
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
			// Self-billed credit notes (261) are exempt too. The rule text only
			// names 381 and 81, but the Schematron test also exempts 261, and we
			// follow the test.
			rules.When(
				is.Not(is.Or(
					is.Func("document type is 381, 81 or 261",
						documentTypeIn(docTypeCreditNote, docTypeCreditNoteGoodsServices, docTypeSelfBilledCreditNote)),
					is.Func("deemed supply", invoiceIsDeemedSupply),
				)),
				rules.When(
					is.Func("amount due is positive", invoiceIsDue),
					rules.Assert("17", "payment due date is required when the amount due is positive, except on credit notes and deemed supplies (IBR-127-AE)",
						is.Func("has due date", invoiceHasDueDate),
					),
				),
			),
			// IBR-128-AE's text names the seller, buyer, tax representative and
			// delivery subdivisions (IBT-039, IBT-054, IBT-068, IBT-079), but its
			// test never runs on the seller's and buyer's addresses: its context,
			// cac:PostalAddress, comes after their address rules (IBR-143-AE,
			// IBR-144-AE) in the same pattern, where a node only fires the first
			// rule it matches. Rules 19 and 21 follow the text there, as the
			// official examples do, so they check more than the test. Rules 22
			// and 23, and line rule 11, follow the test: gobl.ubl also writes the
			// issuer and the line sellers with a cac:PostalAddress.
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
				rules.Field("issuer",
					rules.Assert("23", "issuer region must be an emirate code when the country is AE (IBR-128-AE)",
						is.Func("first address region is an emirate", partyAddressHasEmirate),
					),
				),
			),
			rules.Field("supplier",
				rules.When(
					is.Func("has UAE Peppol endpoint", partyHasUAEEndpoint),
					rules.Assert("24", "supplier legal identity is required with a UAE Peppol endpoint (IBR-150-AE)",
						is.Func("has legal identity", partyHasLegalIdentity),
					),
					rules.Assert("25", "supplier legal identity must be a trade license, Emirates ID, passport or Cabinet Decision with a UAE Peppol endpoint (IBR-173-AE, IBR-181-AE)",
						is.Func("legal identity has a PINT AE type", partyLegalIdentityHasTypeAE),
					),
				),
				rules.Assert("26", "supplier passport must have the issuing country (IBR-012-AE)",
					is.Func("passport has country", partyPassportHasCountry),
				),
				rules.Assert("27", "supplier trade license must have the issuing authority in the description (IBR-172-AE)",
					is.Func("trade license has authority", partyTradeLicenseHasAuthority),
				),
				// IBR-178-AE needs no rule: EN 16931 allows one tax scope identity
				// (GOBL-EU-EN16931-ORG-PARTY-03), which, with the TRN, is the two
				// PartyTaxSchemes with one VAT that it accepts.
				rules.Field("identities", rules.Each(
					rules.When(
						is.Func("tax scope", identityHasTaxScope),
						rules.Field("code",
							rules.Assert("28", "supplier tax scope identity must be a TIN of 10 digits starting with 1 (IBR-148-AE)",
								is.MatchesRegexp(tinRegexp),
							),
						),
					),
				)),
			),
			rules.Field("customer",
				// IBR-183-AE's test also accepts CL, which only its rule text's
				// "Commercial/Trade license" explains. FormatAE reads CL as a
				// trade licence on import, but GOBL writes TL, which is valid for
				// both seller and buyer.
				rules.When(
					is.Func("has UAE Peppol endpoint", partyHasUAEEndpoint),
					rules.Assert("29", "customer legal identity must be a trade license, Emirates ID, passport or Cabinet Decision with a UAE Peppol endpoint (IBR-180-AE, IBR-183-AE)",
						is.Func("legal identity has a PINT AE type", partyLegalIdentityHasTypeAE),
					),
				),
				rules.Assert("30", "customer passport must have the issuing country (IBR-010-AE)",
					is.Func("passport has country", partyPassportHasCountry),
				),
				rules.Assert("31", "customer trade license must have the issuing authority in the description (IBR-101-AE)",
					is.Func("trade license has authority", partyTradeLicenseHasAuthority),
				),
				// IBR-149-AE's text asks for the buyer legal registration identifier
				// (IBT-047), but its test checks the tax registration,
				// cac:PartyTaxScheme/cbc:CompanyID. We follow the test.
				rules.When(
					is.Func("has UAE Peppol endpoint not starting with 1 or 9", partyHasUAEEndpointNotStartingWith1or9),
					rules.Assert("32", "customer tax registration is required with a UAE Peppol endpoint not starting with 1 or 9 (IBR-149-AE)",
						is.Func("has tax registration", partyHasTaxRegistration),
					),
				),
				rules.Assert("33", "customer must have at most one tax registration (IBR-179-AE)",
					is.Func("at most one tax registration", partyHasAtMostOneTaxRegistration),
				),
			),
			rules.Assert("34", "customer tax scope identities require a supplier TIN (IBR-133-AE)",
				is.Func("customer tax scope identities with supplier TIN", customerTaxIdentitiesHaveSupplierTIN),
			),
			rules.Assert("35", "transaction type is required, as 8 flags of 0 or 1 (IBR-154-AE)",
				is.Func("has a valid transaction type", invoiceHasTransactionType),
			),
			rules.When(
				is.Func("free trade zone", invoiceIsFreeTradeZone),
				rules.Assert("36", "beneficiary ID is required on free trade zone supplies (IBR-007-AE)",
					is.Func("has beneficiary ID", invoiceHasExt(ExtKeyBeneficiaryID)),
				),
			),
			rules.When(
				is.Func("summary invoice", invoiceIsSummary),
				rules.Assert("37", "ordering period or billing frequency is required on summary invoices (IBR-138-AE)",
					is.Func("has invoice period", invoiceHasInvoicePeriod),
				),
			),
			rules.When(
				is.Func("disclosed agent billing", invoiceIsDisclosedAgent),
				rules.Assert("38", "principal ID is required on disclosed agent billing (IBR-137-AE)",
					is.Func("has principal ID", invoiceHasExt(ExtKeyPrincipalID)),
				),
				rules.Assert("39", "principal ID must differ from the supplier tax ID on disclosed agent billing (IBR-176-AE)",
					is.Func("principal ID is not the supplier tax ID", principalIsNotSupplier),
				),
				rules.Field("supplier",
					rules.Assert("40", "supplier tax registration is required on disclosed agent billing (IBR-177-AE)",
						is.Func("has tax registration", partyHasTaxRegistration),
					),
				),
			),
			rules.When(
				is.Func("e-commerce supply", invoiceIsECommerce),
				rules.Assert("41", "delivery address must have a street, city and region on e-commerce supplies (IBR-142-AE)",
					is.Func("delivery address is complete", invoiceDeliveryAddressIsComplete),
				),
			),
			// IBR-152-AE's text says the delivery country should not be AE on
			// exports, but its test accepts a delivery address in the UAE, and
			// only asks the others for a street, city and region. We follow the
			// test, which fails exports without a delivery address.
			rules.When(
				is.Func("export", invoiceIsExport),
				rules.Assert("42", "delivery address outside the UAE must have a street, city and region on exports (IBR-152-AE)",
					is.Func("delivery address is in the UAE or complete", invoiceDeliveryAddressIsAEOrComplete),
				),
			),
			rules.When(
				is.Func("document type is invoice outside scope or credit note related to goods and services",
					documentTypeIn(docTypeInvoiceOutsideScope, docTypeCreditNoteGoodsServices)),
				rules.Assert("43", "transaction type cannot be summary, deemed supply or margin scheme on outside scope documents or credit notes related to goods and services (IBR-157-AE)",
					is.Not(is.Func("summary, deemed supply or margin scheme", invoiceIsSummaryDeemedOrMargin)),
				),
			),
			rules.When(
				is.Not(is.Func("export", invoiceIsExport)),
				rules.Field("customer",
					rules.When(
						is.Func("has UAE Peppol endpoint that is not a TIN", partyHasUAEEndpointNotTIN),
						rules.Assert("44", "customer identity or tax registration is required with a UAE Peppol endpoint that is not a TIN, except on exports (IBR-135-AE)",
							is.Func("has identifier or tax registration", partyHasIdentificationOrTaxRegistration),
						),
					),
				),
			),
			// IBR-158-AE only names 381, but IBR-055-AE's test fails 81 credit
			// notes without a reason too, whether or not they have a preceding
			// invoice, so both need one.
			rules.When(
				is.Func("document type is credit note or credit note related to goods and services",
					documentTypeIn(docTypeCreditNote, docTypeCreditNoteGoodsServices)),
				rules.Assert("45", "credit note reason is required (IBR-158-AE, IBR-055-AE)",
					is.Func("has credit note reason", invoiceHasExt(ExtKeyCreditNoteReason)),
				),
				rules.When(
					is.Func("credit note reason is not volume discount", creditNoteReasonIsNotVolumeDiscount),
					rules.Field("preceding",
						rules.Assert("46", "preceding invoice is required on credit notes, except for volume discounts (IBR-055-AE)",
							is.Present,
						),
					),
				),
				rules.When(
					is.Func("credit note reason is volume discount", creditNoteReasonIs(CreditNoteReasonVolumeDiscount)),
					rules.Field("preceding",
						rules.Assert("47", "preceding invoice must not be there on volume discount credit notes (IBR-055-AE)",
							is.Not(is.Present),
						),
					),
				),
			),
			rules.Field("tax",
				rules.Field("ext",
					rules.Assert("48", "credit note reason must be from the Reasons for credit note code list (IBR-001-AE)",
						tax.ExtensionHasValidCode(ExtKeyCreditNoteReason),
					),
					rules.Assert("49", "billing frequency must be from the frequency of billing code list (IBR-005-AE)",
						tax.ExtensionHasValidCode(ExtKeyBillingFrequency),
					),
				),
				// PINT AE writes the billing frequency where EN writes the VAT
				// point date code (BT-8), cac:InvoicePeriod/cbc:DescriptionCode,
				// and IBR-005-AE only accepts frequencies there.
				rules.Field("point",
					rules.Assert("50", "tax point must not be set, as its code goes where the billing frequency does (IBR-005-AE)",
						is.Not(is.Present),
					),
				),
			),
			rules.When(
				is.Func("billing frequency is Others", billingFrequencyIs(BillingFrequencyOthers)),
				rules.Field("notes",
					rules.Assert("51", "a note is required with the billing frequency Others (IBR-160-AE)",
						is.Present,
					),
				),
			),
			// IBR-168-AE and IBR-169-AE name the exemption reason code (IBT-196,
			// IBT-198), but their tests check the allowance or charge reason code,
			// cbc:AllowanceChargeReasonCode. We follow the tests.
			rules.Field("discounts", rules.Each(
				rules.When(
					is.Func("has exempt VAT", taxesIn(comboIsExemptVAT)),
					rules.Field("ext",
						rules.Assert("52", "exempt discount needs an allowance reason code (IBR-168-AE)",
							tax.ExtensionsRequire(untdid.ExtKeyAllowance),
						),
					),
				),
			)),
			rules.Field("charges", rules.Each(
				rules.When(
					is.Func("has exempt VAT", taxesIn(comboIsExemptVAT)),
					rules.Field("ext",
						rules.Assert("53", "exempt charge needs a charge reason code (IBR-169-AE)",
							tax.ExtensionsRequire(untdid.ExtKeyCharge),
						),
					),
				),
			)),
			rules.Field("discounts", rules.Each(
				rules.Assert("54", "discount VAT category cannot be margin scheme (IBR-115-AE)",
					is.Not(is.Func("has margin scheme VAT", taxesIn(comboIsMarginScheme))),
				),
			)),
			rules.Field("charges", rules.Each(
				rules.Assert("55", "charge VAT category cannot be margin scheme (IBR-114-AE)",
					is.Not(is.Func("has margin scheme VAT", taxesIn(comboIsMarginScheme))),
				),
			)),
			// With rules 54 and 55, a margin scheme invoice can have no discounts
			// or charges, as GOBL requires them to have taxes (BR-32, BR-37).
			rules.When(
				is.Func("margin scheme", invoiceIsMarginScheme),
				rules.Assert("56", "margin scheme invoice must only use the margin scheme VAT key (IBR-116-AE)",
					is.Func("only margin scheme VAT", invoiceTaxesAreMarginScheme),
				),
			),
		),
		// The tags the addon defines are for UAE documents, but GOBL can't limit
		// an addon's tags to a regime. Other documents can only use the ones
		// their regime, another addon or GOBL itself defines too, as before the
		// addon had tags.
		rules.When(
			is.Not(is.InContext(tax.RegimeIn(regimesAE...))),
			rules.Assert("57", "PINT AE tags can only be used on UAE documents",
				is.Func("has no PINT AE tags", invoiceHasNoAETags),
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
			rules.Field("taxes", rules.Each(
				rules.When(
					is.Func("exempt VAT", comboIsExemptVAT),
					rules.Field("ext",
						rules.Assert("03", "exempt line needs a UAE VAT exemption reason (IBR-167-AE)",
							tax.ExtensionsRequire(ExtKeyExemption),
						),
					),
				),
			)),
			rules.When(
				is.Func("reverse charge VAT", lineHasReverseChargeVAT),
				rules.Field("item",
					rules.Field("ext",
						rules.Assert("04", "reverse charge line item needs the type of goods or services (IBR-166-AE)",
							tax.ExtensionsRequire(ExtKeyGoodsType),
						),
					),
				),
			),
			// The IBR-006-AE test checks every goods type, not only those of
			// reverse charge lines as its text says.
			rules.Field("item",
				rules.Field("ext",
					rules.Assert("05", "type of goods or services must be from the PINT AE code list (IBR-006-AE)",
						tax.ExtensionHasValidCode(ExtKeyGoodsType),
					),
				),
			),
			rules.When(
				is.Func("reverse charge VAT", lineHasReverseChargeVAT),
				rules.Field("item",
					rules.Field("identities",
						rules.Assert("06", "reverse charge line item needs a GTIN identity, scheme 0160 (IBR-174-AE)",
							is.Func("has GTIN", identitiesHaveGTIN),
						),
					),
				),
			),
			rules.Field("item",
				rules.When(
					is.Func("goods", itemKeyIs(org.ItemKeyGoods)),
					rules.Field("identities",
						rules.Assert("07", "goods item needs an HS item classification (IBR-184-AE)",
							is.Func("has HS", identitiesHaveLabel(IdentityLabelHS)),
						),
					),
				),
				rules.When(
					is.Func("services", itemKeyIs(org.ItemKeyServices)),
					rules.Field("identities",
						rules.Assert("08", "services item needs a service accounting code (IBR-185-AE)",
							is.Func("has SAC", identitiesHaveLabel(IdentityLabelSAC)),
						),
					),
				),
				rules.When(
					is.Func("goods and services", itemKeyIs(ItemKeyGoodsServices)),
					rules.Field("identities",
						rules.Assert("09", "goods and services item needs an HS item classification and a service accounting code (IBR-186-AE)",
							is.Func("has HS and SAC", identitiesHaveLabel(IdentityLabelHS, IdentityLabelSAC)),
						),
					),
				),
				// gobl.ubl writes identities with a label and no scheme ID as item
				// classifications, with the label as their list. FormatAE writes the
				// SAC ones as service accounting codes instead (IBR-189-AE).
				// IBR-188-AE's text asks for HS on every classification, but its
				// test passes when any one is HS.
				rules.Field("identities",
					rules.Assert("10", "item classifications must include one from the HS list (IBR-188-AE)",
						is.Func("classifications include HS", identitiesClassifiedWithHS),
					),
				),
			),
			rules.Field("seller",
				rules.Assert("11", "line seller region must be an emirate code when the country is AE (IBR-128-AE)",
					is.Func("first address region is an emirate", partyAddressHasEmirate),
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

// exchangeRateToAEDHasAtMost6Decimals reports whether the exchange rate from
// the invoice currency to AED has at most 6 decimals, the limit for
// cbc:CalculationRate (IBR-002-AE). It is the only rate written. Invoices
// without it pass: they are in AED, or rule 05 reports them.
func exchangeRateToAEDHasAtMost6Decimals(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	rate := currency.MatchExchangeRate(inv.ExchangeRates, inv.Currency, currency.AED)
	return rate == nil || rate.Amount.Exp() <= 6
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

// partyEndpointID returns the scheme and code of the party's electronic
// address, chosen as gobl.ubl does for cbc:EndpointID: the first ISO 6523
// endpoint, or else the first inbox with a scheme.
func partyEndpointID(p *org.Party) (cbc.Code, string) {
	if e := p.Endpoint(iso.ActorIDScheme); e != nil {
		// An ISO 6523 URI reads "iso6523-actorid-upis::<scheme>:<code>".
		scheme, code, ok := strings.Cut(strings.TrimPrefix(e.URI.Opaque(), ":"), ":")
		if ok && scheme != "" && code != "" {
			return cbc.Code(scheme), code
		}
	}
	for _, ib := range p.Inboxes {
		if ib != nil && ib.Email == "" && ib.Scheme != "" {
			return ib.Scheme, ib.Code.String()
		}
	}
	return "", ""
}

// partyHasUAEEndpoint reports whether the party's electronic address uses the
// UAE Peppol scheme 0235, the condition of the legal registration rules.
func partyHasUAEEndpoint(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	scheme, _ := partyEndpointID(p)
	return scheme == endpointSchemeUAE
}

// partyHasUAEEndpointNotStartingWith1or9 reports whether the party's
// electronic address uses the UAE Peppol scheme 0235 with a code that doesn't
// start with 1 or 9 (IBR-149-AE).
func partyHasUAEEndpointNotStartingWith1or9(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	scheme, code := partyEndpointID(p)
	return scheme == endpointSchemeUAE && !strings.HasPrefix(code, "1") && !strings.HasPrefix(code, "9")
}

// partyFirstLegalIdentity returns the party's first identity with the legal
// scope, the one gobl.ubl writes as cac:PartyLegalEntity/cbc:CompanyID, or nil
// if there is none.
func partyFirstLegalIdentity(val any) *org.Identity {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return nil
	}
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeLegal {
			return id
		}
	}
	return nil
}

// partyLegalIdentityHasTypeAE reports whether the party's legal identity has
// a PINT AE legal registration identifier type: trade licence, Emirates ID,
// passport or Cabinet Decision. A party without a legal identity passes.
func partyLegalIdentityHasTypeAE(val any) bool {
	id := partyFirstLegalIdentity(val)
	return id == nil || IdentityTypeAE(id.Key) != ""
}

// partyPassportHasCountry reports whether the party's legal identity, when a
// passport, has the issuing country, written as @schemeAgencyName (BTAE-18,
// BTAE-19). GOBL already checks the country is an ISO 3166-1 code
// (IBR-011-AE, IBR-013-AE).
func partyPassportHasCountry(val any) bool {
	id := partyFirstLegalIdentity(val)
	return id == nil || id.Key != org.IdentityKeyPassport || id.Country != ""
}

// partyTradeLicenseHasAuthority reports whether the party's legal identity,
// when a trade licence, has the issuing authority in its description, written
// as @schemeAgencyName (BTAE-11, BTAE-12).
func partyTradeLicenseHasAuthority(val any) bool {
	id := partyFirstLegalIdentity(val)
	return id == nil || id.Key != ae.IdentityKeyTradeLicense || id.Description != ""
}

// partyTaxIdentityCount returns how many of the party's identities have the
// tax scope. gobl.ubl writes each one as a cac:PartyTaxScheme.
func partyTaxIdentityCount(p *org.Party) int {
	n := 0
	for _, id := range p.Identities {
		if identityHasTaxScope(id) {
			n++
		}
	}
	return n
}

// identityHasTaxScope reports whether an identity has the tax scope.
func identityHasTaxScope(val any) bool {
	id, ok := val.(*org.Identity)
	return ok && id != nil && id.Scope == org.IdentityScopeTax
}

// partyHasAtMostOneTaxRegistration reports whether the party has at most one
// tax registration, counting the tax ID code and the tax scope identities, as
// the buyer can only have one PartyTaxScheme/CompanyID (IBR-179-AE).
func partyHasAtMostOneTaxRegistration(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	n := partyTaxIdentityCount(p)
	if p.TaxID != nil && p.TaxID.Code != "" {
		n++
	}
	return n <= 1
}

// customerTaxIdentitiesHaveSupplierTIN reports whether the supplier has a tax
// scope identity, the TIN (IBT-032), whenever the customer has one. gobl.ubl
// writes a customer tax scope identity under a tax scheme other than VAT,
// which PINT AE only allows when the seller has a TIN (IBR-133-AE).
func customerTaxIdentitiesHaveSupplierTIN(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	if inv.Customer == nil || partyTaxIdentityCount(inv.Customer) == 0 {
		return true
	}
	return inv.Supplier != nil && partyTaxIdentityCount(inv.Supplier) > 0
}

// invoiceHasTransactionType reports whether the invoice has a transaction
// type of 8 flags of 0 or 1, written as cbc:ProfileExecutionID (IBR-154-AE).
// GOBL doesn't check an extension's pattern on its own.
func invoiceHasTransactionType(val any) bool {
	inv, ok := val.(*bill.Invoice)
	return ok && inv != nil && transactionTypeRegexp.MatchString(inv.Tax.GetExt(ExtKeyTransactionType).String())
}

// invoiceHasExt returns a predicate that reports whether the invoice's tax
// extensions have a value for the key.
func invoiceHasExt(key cbc.Key) func(any) bool {
	return func(val any) bool {
		inv, ok := val.(*bill.Invoice)
		return ok && inv != nil && inv.Tax.GetExt(key) != ""
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
// registration passes, as rules 38 and 40 report those.
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

// partyHasUAEEndpointNotTIN reports whether the party's electronic address
// uses the UAE Peppol scheme 0235 with a code that is not a TIN, 1XXXXXXXXX
// (IBR-135-AE).
func partyHasUAEEndpointNotTIN(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	scheme, code := partyEndpointID(p)
	return scheme == endpointSchemeUAE && !tinRegexp.MatchString(code)
}

// partyHasIdentificationOrTaxRegistration reports whether the party has an
// identity gobl.ubl writes as cac:PartyIdentification (any identity that is
// neither a tax registration nor the first legal one) or a tax registration
// (IBR-135-AE).
func partyHasIdentificationOrTaxRegistration(val any) bool {
	if partyHasTaxRegistration(val) {
		return true
	}
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	legal := partyFirstLegalIdentity(p)
	for _, id := range p.Identities {
		if id != nil && id != legal && id.Scope != org.IdentityScopeTax {
			return true
		}
	}
	return false
}

// creditNoteReasonIsNotVolumeDiscount reports whether the invoice has a
// credit note reason other than volume discount, the reasons that need a
// preceding invoice (IBR-055-AE). Invoices without a reason fail, as rule 45
// reports them.
func creditNoteReasonIsNotVolumeDiscount(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	reason := inv.Tax.GetExt(ExtKeyCreditNoteReason)
	return reason != "" && reason != CreditNoteReasonVolumeDiscount
}

// creditNoteReasonIs returns a predicate that reports whether the invoice's
// credit note reason is the given code.
func creditNoteReasonIs(code cbc.Code) func(any) bool {
	return func(val any) bool {
		inv, ok := val.(*bill.Invoice)
		return ok && inv != nil && inv.Tax.GetExt(ExtKeyCreditNoteReason) == code
	}
}

// billingFrequencyIs returns a predicate that reports whether the invoice's
// billing frequency is the given code.
func billingFrequencyIs(code cbc.Code) func(any) bool {
	return func(val any) bool {
		inv, ok := val.(*bill.Invoice)
		return ok && inv != nil && inv.Tax.GetExt(ExtKeyBillingFrequency) == code
	}
}

// comboIsExemptVAT reports whether a tax combo is VAT with the exempt
// category code E.
func comboIsExemptVAT(val any) bool {
	c, ok := val.(*tax.Combo)
	return ok && c != nil && c.Category == tax.CategoryVAT &&
		c.Ext.Get(untdid.ExtKeyTaxCategory) == en16931.TaxCategoryExempt
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

// taxesHaveVAT reports whether a line's taxes include a VAT combo (IBR-145-AE).
// It fails on any other value, so a type mismatch can't pass silently.
func taxesHaveVAT(val any) bool {
	set, ok := val.(tax.Set)
	return ok && set.Get(tax.CategoryVAT) != nil
}

// lineHasReverseChargeVAT reports whether a line's VAT combo is reverse
// charge.
func lineHasReverseChargeVAT(val any) bool {
	l, ok := val.(*bill.Line)
	if !ok || l == nil {
		return false
	}
	c := l.Taxes.Get(tax.CategoryVAT)
	return c != nil && c.Ext.Get(untdid.ExtKeyTaxCategory) == en16931.TaxCategoryReverseCharge
}

// identitiesHaveGTIN reports whether an item has an identity with the GTIN
// scheme, 0160, the one gobl.ubl writes as the standard item identifier.
func identitiesHaveGTIN(val any) bool {
	ids, ok := val.([]*org.Identity)
	if !ok {
		return false
	}
	return GTINIdentity(ids) != nil
}

// itemKeyIs returns a predicate that reports whether an item has the key.
func itemKeyIs(key cbc.Key) func(any) bool {
	return func(val any) bool {
		it, ok := val.(*org.Item)
		return ok && it != nil && it.Key == key
	}
}

// identitiesHaveLabel returns a predicate that reports whether an item has
// identities with all the labels.
func identitiesHaveLabel(labels ...string) func(any) bool {
	return func(val any) bool {
		ids, ok := val.([]*org.Identity)
		if !ok {
			return false
		}
		for _, label := range labels {
			if !identitiesHaveLabelOf(ids, label) {
				return false
			}
		}
		return true
	}
}

func identitiesHaveLabelOf(ids []*org.Identity, label string) bool {
	for _, id := range ids {
		if id != nil && id.Label == label {
			return true
		}
	}
	return false
}

// identitiesClassifiedWithHS reports whether the identities gobl.ubl writes
// as item classifications, those with a label and no scheme ID, include an HS
// one. Items without classifications pass, and the service accounting codes
// don't count as one.
func identitiesClassifiedWithHS(val any) bool {
	ids, ok := val.([]*org.Identity)
	if !ok {
		return false
	}
	classified := false
	for _, id := range ids {
		if id == nil || id.Label == "" || id.Label == IdentityLabelSAC || id.Ext.Has(iso.ExtKeySchemeID) {
			continue
		}
		if id.Label == IdentityLabelHS {
			return true
		}
		classified = true
	}
	return !classified
}

// comboIsMarginScheme reports whether a tax combo is VAT under the profit
// margin scheme.
func comboIsMarginScheme(val any) bool {
	c, ok := val.(*tax.Combo)
	return ok && c != nil && c.Category == tax.CategoryVAT &&
		c.Ext.Get(untdid.ExtKeyTaxCategory) == TaxCategoryMarginScheme
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
