package pint

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/pkg/here"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/tax"
)

// PINT AE extension keys, for the BTAE fields GOBL has no place for.
const (
	// ExtKeyTransactionType is the PINT AE invoice transaction type (BTAE-02).
	ExtKeyTransactionType cbc.Key = "peppol-pint-ae-transaction-type"
	// ExtKeyBeneficiaryID is the ID of the beneficiary of a supply in a free
	// trade zone (BTAE-01).
	ExtKeyBeneficiaryID cbc.Key = "peppol-pint-ae-beneficiary-id"
	// ExtKeyPrincipalID is the ID of the principal a disclosed agent bills for
	// (BTAE-14).
	ExtKeyPrincipalID cbc.Key = "peppol-pint-ae-principal-id"
	// ExtKeyCreditNoteReason is the reason for a PINT AE credit note (BTAE-03).
	ExtKeyCreditNoteReason cbc.Key = "peppol-pint-ae-credit-note-reason"
	// ExtKeyBillingFrequency is the frequency of billing (BTAE-06).
	ExtKeyBillingFrequency cbc.Key = "peppol-pint-ae-billing-frequency"
	// ExtKeyExemption is the UAE VAT exemption reason of an exempt tax combo
	// (IBT-186 on lines).
	ExtKeyExemption cbc.Key = "peppol-pint-ae-exemption"
	// ExtKeyGoodsType is the type of goods or services subject to the reverse
	// charge mechanism, on the item (BTAE-09).
	ExtKeyGoodsType cbc.Key = "peppol-pint-ae-goods-type"
	// ExtKeyCustomsReference is the customs reference number of an export or
	// import (BTAE-21).
	ExtKeyCustomsReference cbc.Key = "peppol-pint-ae-customs-reference"
	// ExtKeyBatchNumber is the production batch or lot the items come from,
	// on the item (BTAE-24).
	ExtKeyBatchNumber cbc.Key = "peppol-pint-ae-batch-number"
	// ExtKeySchemeVersion is the version of the scheme an item identity's code
	// is from, such as an HS or SAC code (IBT-158-2, BTAE-17-2).
	ExtKeySchemeVersion cbc.Key = "peppol-pint-ae-scheme-version"
	// ExtKeyContractValue is the value of a contract, on its reference
	// (BTAE-05).
	ExtKeyContractValue cbc.Key = "peppol-pint-ae-contract-value"
	// ExtKeyPurchaseOrder is the purchase order a line belongs to (IBT-183).
	ExtKeyPurchaseOrder cbc.Key = "peppol-pint-ae-purchase-order"
	// ExtKeyDespatchAdvice is the despatch advice a line was sent with
	// (IBT-184).
	ExtKeyDespatchAdvice cbc.Key = "peppol-pint-ae-despatch-advice"
	// ExtKeyRemittanceScheme is the scheme of the payment instructions'
	// remittance reference, such as "ABA" (IBT-083-1).
	ExtKeyRemittanceScheme cbc.Key = "peppol-pint-ae-remittance-scheme"
)

// CreditNoteReasonVolumeDiscount is the credit note reason for a volume
// discount, the only one that needs no preceding invoice (IBR-055-AE).
const CreditNoteReasonVolumeDiscount cbc.Code = "VD"

// BillingFrequencyOthers is the billing frequency "Others", whose value goes
// in an invoice note (IBR-160-AE).
const BillingFrequencyOthers cbc.Code = "OTH"

// Usages of the PINT AE extensions: offered by the PINT addon under the AE
// regime, on the object each one goes on.
var (
	usageAEInvoiceTax = &cbc.Usage{
		Keys:    []cbc.Key{V1},
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{bill.ShortSchemaTax},
	}
	usageAETaxCombo = &cbc.Usage{
		Keys:    []cbc.Key{V1},
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{tax.ShortSchemaCombo},
	}
	usageAEItem = &cbc.Usage{
		Keys:    []cbc.Key{V1},
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{org.ShortSchemaItem},
	}
	usageAEIdentity = &cbc.Usage{
		Keys:    []cbc.Key{V1},
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{org.ShortSchemaIdentity},
	}
	usageAEDocumentRef = &cbc.Usage{
		Keys:    []cbc.Key{V1},
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{org.ShortSchemaDocumentRef},
	}
	usageAELine = &cbc.Usage{
		Keys:    []cbc.Key{V1},
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{bill.ShortSchemaLine},
	}
	usageAEInstructions = &cbc.Usage{
		Keys:    []cbc.Key{V1},
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{pay.ShortSchemaInstructions},
	}
)

// extensions are the extensions the PINT addon defines. They are only used
// on UAE documents.
var extensions = []*cbc.Definition{
	{
		Key:   ExtKeyTransactionType,
		Usage: usageAEInvoiceTax,
		Name: i18n.String{
			i18n.EN: "PINT AE Transaction Type",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				PINT AE invoice transaction type (BTAE-02): 8 flags of 0 or 1, in this
				order: free trade zone, deemed supply, profit margin scheme, summary
				invoice, continuous supply, disclosed agent billing, supply through
				e-commerce, and exports. "00000000" when none applies.

				It is built from the invoice tags with the same names
				("free-trade-zone", "deemed-supply", "margin-scheme", "summary",
				"continuous-supply", "disclosed-agent", "e-commerce" and "export").
				A code set directly is replaced.
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("Peppol PINT AE Billing specification"),
				URL:   "https://docs.peppol.eu/poac/ae/pint-ae/bis/",
			},
		},
		Pattern: transactionTypePattern,
	},
	{
		Key:   ExtKeyBeneficiaryID,
		Usage: usageAEInvoiceTax,
		Name: i18n.String{
			i18n.EN: "PINT AE Beneficiary ID",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Identifier of the beneficiary of a supply in a free trade zone (BTAE-01),
				required with the "free-trade-zone" transaction type (IBR-007-AE).
			`),
		},
	},
	{
		Key:   ExtKeyPrincipalID,
		Usage: usageAEInvoiceTax,
		Name: i18n.String{
			i18n.EN: "PINT AE Principal ID",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Identifier of the principal a disclosed agent bills on behalf of
				(BTAE-14), required with the "disclosed-agent" transaction type
				(IBR-137-AE). It must differ from the supplier's tax ID (IBR-176-AE).
			`),
		},
	},
	{
		Key:   ExtKeyCreditNoteReason,
		Usage: usageAEInvoiceTax,
		Name: i18n.String{
			i18n.EN: "PINT AE Credit Note Reason",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Reason for a PINT AE credit note (BTAE-03), from the Reasons for credit
				note code list. Required on credit notes (IBR-158-AE). Volume discounts
				("VD") have no preceding invoice; every other reason needs one
				(IBR-055-AE).
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("PINT AE Reasons for credit note"),
				URL:   "https://docs.peppol.eu/poac/ae/pint-ae/trn-creditnote/codelist/CreditReason/",
			},
		},
		Values: []*cbc.Definition{
			{
				Code: "DL8.61.1.A",
				Name: i18n.String{i18n.EN: "If the supply was cancelled."},
			},
			{
				Code: "DL8.61.1.B",
				Name: i18n.String{i18n.EN: "If the tax treatment of the supply has changed due to a change in the nature of the supply."},
			},
			{
				Code: "DL8.61.1.C",
				Name: i18n.String{i18n.EN: "If the previously agreed consideration for the supply was altered for any reason (i.e. bad debt relief)."},
			},
			{
				Code: "DL8.61.1.D",
				Name: i18n.String{i18n.EN: "If the recipient of goods or recipient of services returned them to the registrant in full or in part and the Consideration was returned in full or in part."},
			},
			{
				Code: "DL8.61.1.E",
				Name: i18n.String{i18n.EN: "If the tax was charged or tax treatment was applied in error."},
			},
			{
				Code: CreditNoteReasonVolumeDiscount,
				Name: i18n.String{i18n.EN: "Volume Discount."},
			},
		},
	},
	{
		Key:   ExtKeyBillingFrequency,
		Usage: usageAEInvoiceTax,
		Name: i18n.String{
			i18n.EN: "PINT AE Frequency of Billing",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Frequency of billing (BTAE-06), from the frequency of billing code list
				(IBR-005-AE). With "OTH" (Others), the frequency is given in an invoice
				note (IBR-160-AE).
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("PINT AE Frequency of billing codes"),
				URL:   "https://docs.peppol.eu/poac/ae/pint-ae/trn-invoice/codelist/FreqBilling/",
			},
		},
		Values: []*cbc.Definition{
			{Code: "DLY", Name: i18n.String{i18n.EN: "Daily"}},
			{Code: "WKY", Name: i18n.String{i18n.EN: "Weekly"}},
			{Code: "Q15", Name: i18n.String{i18n.EN: "Once in 15 days"}},
			{Code: "MTH", Name: i18n.String{i18n.EN: "Monthly"}},
			{Code: "Q45", Name: i18n.String{i18n.EN: "Once in 45 days"}},
			{Code: "Q60", Name: i18n.String{i18n.EN: "Once in 60 days"}},
			{Code: "QTR", Name: i18n.String{i18n.EN: "Quarterly"}},
			{Code: "YRL", Name: i18n.String{i18n.EN: "Yearly"}},
			{Code: "HYR", Name: i18n.String{i18n.EN: "Half-Yearly"}},
			{Code: BillingFrequencyOthers, Name: i18n.String{i18n.EN: "Others"}},
		},
	},
	{
		Key:   ExtKeyExemption,
		Usage: usageAETaxCombo,
		Name: i18n.String{
			i18n.EN: "PINT AE VAT Exemption Reason",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Reason a supply is exempt from UAE VAT, such as one of the PINT AE
				"Reasons for exemption from tax" codes (DL8.46.1 to DL8.46.4).
				Required on exempt lines (IBR-167-AE), where it is written as the
				line's exemption reason code.
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("PINT AE Reasons for exemption from tax"),
				URL:   "https://docs.peppol.eu/poac/ae/pint-ae/trn-invoice/codelist/Aligned-TaxExemptionCodes/",
			},
		},
	},
	{
		Key:   ExtKeyGoodsType,
		Usage: usageAEItem,
		Name: i18n.String{
			i18n.EN: "PINT AE Type of Goods or Services",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Type of goods or services subject to the reverse charge mechanism
				(BTAE-09), from the PINT AE code list (IBR-006-AE). Required on the
				items of reverse charge lines (IBR-166-AE).
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("PINT AE Type of goods or services subject to RCM"),
				URL:   "https://docs.peppol.eu/poac/ae/pint-ae/trn-invoice/codelist/GoodsType/",
			},
		},
		Values: []*cbc.Definition{
			{Code: "DL8.48.8.2", Name: i18n.String{i18n.EN: "Electronic Devices"}},
			{Code: "DL8.48.8.1", Name: i18n.String{i18n.EN: "Gold and Diamonds"}},
			{Code: "DL8.48.3.1", Name: i18n.String{i18n.EN: "Crude or refined oil"}},
			{Code: "DL8.48.3.2", Name: i18n.String{i18n.EN: "Unprocessed or processed natural gas"}},
			{Code: "DL8.48.3.3", Name: i18n.String{i18n.EN: "Pure hydrocarbons"}},
		},
	},
	{
		Key:   ExtKeyCustomsReference,
		Usage: usageAEInvoiceTax,
		Name: i18n.String{
			i18n.EN: "PINT AE Customs Reference Number",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Customs reference number of an export or import transaction
				(BTAE-21).
			`),
		},
	},
	{
		Key:   ExtKeyBatchNumber,
		Usage: usageAEItem,
		Name: i18n.String{
			i18n.EN: "PINT AE Batch Number",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Production batch or lot the items come from (BTAE-24).
			`),
		},
	},
	{
		Key:   ExtKeySchemeVersion,
		Usage: usageAEIdentity,
		Name: i18n.String{
			i18n.EN: "PINT AE Scheme Version",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Version of the scheme the code of an item identity is from, for the
				item classifications and service accounting codes of an item
				(IBT-158-2, BTAE-17-2).
			`),
		},
	},
	{
		Key:   ExtKeyContractValue,
		Usage: usageAEDocumentRef,
		Name: i18n.String{
			i18n.EN: "PINT AE Contract Value",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Value of a contract, on its reference in the invoice's ordering
				details, such as "AED 200000" (BTAE-05).
			`),
		},
	},
	{
		Key:   ExtKeyPurchaseOrder,
		Usage: usageAELine,
		Name: i18n.String{
			i18n.EN: "PINT AE Line Purchase Order Reference",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Identifier of the purchase order a line belongs to, issued by the
				buyer (IBT-183). The line's "order" is the line of that order.
			`),
		},
	},
	{
		Key:   ExtKeyDespatchAdvice,
		Usage: usageAELine,
		Name: i18n.String{
			i18n.EN: "PINT AE Line Despatch Advice Reference",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Identifier of the despatch advice a line was sent with (IBT-184).
			`),
		},
	},
	{
		Key:   ExtKeyRemittanceScheme,
		Usage: usageAEInstructions,
		Name: i18n.String{
			i18n.EN: "PINT AE Remittance Reference Scheme",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Scheme of the payment instructions' reference, the remittance
				information, such as "ABA" (IBT-083-1).
			`),
		},
	},
}
