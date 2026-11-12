package pint

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
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
)

// CreditNoteReasonVolumeDiscount is the credit note reason for a volume
// discount, the only one that needs no preceding invoice (IBR-055-AE).
const CreditNoteReasonVolumeDiscount cbc.Code = "VD"

// BillingFrequencyOthers is the billing frequency "Others", whose value goes
// in an invoice note (IBR-160-AE).
const BillingFrequencyOthers cbc.Code = "OTH"

// extensions are the extensions the PINT addon defines. They are only used
// on UAE documents.
var extensions = []*cbc.Definition{
	{
		Key: ExtKeyTransactionType,
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
				Without any of those tags, a code set directly is kept.
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
		Key: ExtKeyBeneficiaryID,
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
		Key: ExtKeyPrincipalID,
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
		Key: ExtKeyCreditNoteReason,
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
		Key: ExtKeyBillingFrequency,
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
		Key: ExtKeyExemption,
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
}
