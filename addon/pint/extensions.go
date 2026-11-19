package pint

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pkg/here"
	"github.com/invopop/gobl/regimes/ae"
)

// PINT AE extension keys, for the BTAE fields GOBL has no place for.
const (
	ExtKeyTransactionType  cbc.Key = "peppol-pint-ae-transaction-type"
	ExtKeyBeneficiaryID    cbc.Key = "peppol-pint-ae-beneficiary-id" // free trade zone beneficiary
	ExtKeyPrincipalID      cbc.Key = "peppol-pint-ae-principal-id"   // principal a disclosed agent bills for
	ExtKeyCreditNoteReason cbc.Key = "peppol-pint-ae-credit-note-reason"
	ExtKeyGoodsType        cbc.Key = "peppol-pint-ae-goods-type" // goods or services under reverse charge
)

// CreditNoteReasonVolumeDiscount is the credit note reason for a volume discount.
const CreditNoteReasonVolumeDiscount cbc.Code = "VD"

// principalIDPattern is a UAE TRN ending with 03, as PINT AE asks of the principal ID (IBR-132-AE).
const principalIDPattern = `^1\d{12}03$`

// Usages of the PINT AE extensions, under the AE regime.
var (
	usageAEInvoiceTax = &cbc.Usage{
		Keys:    []cbc.Key{V1},
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{bill.ShortSchemaTax},
	}
	usageAEItem = &cbc.Usage{
		Keys:    []cbc.Key{V1},
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{org.ShortSchemaItem},
	}
)

// extensions are the extensions the PINT addon defines. They are only used
// on UAE documents.
var extensions = []*cbc.Definition{
	{
		Key:   ExtKeyTransactionType,
		Usage: usageAEInvoiceTax,
		Name: i18n.String{
			i18n.EN: "PINT AE: Transaction Type",
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
			i18n.EN: "PINT AE: Beneficiary ID",
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
			i18n.EN: "PINT AE: Principal ID",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Identifier of the principal a disclosed agent bills on behalf of
				(BTAE-14), required with the "disclosed-agent" transaction type
				(IBR-137-AE). It is the principal's TRN: 15 digits starting with 1
				and ending with 03 (IBR-132-AE).
			`),
		},
		Pattern: principalIDPattern,
	},
	{
		Key:   ExtKeyCreditNoteReason,
		Usage: usageAEInvoiceTax,
		Name: i18n.String{
			i18n.EN: "PINT AE: Credit Note Reason",
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
		Key:   ExtKeyGoodsType,
		Usage: usageAEItem,
		Name: i18n.String{
			i18n.EN: "PINT AE: Type of Goods or Services",
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
			{Code: "DL8.48.8.3", Name: i18n.String{i18n.EN: "Metal Scrap Trading"}},
			{Code: "DL8.48.3.1", Name: i18n.String{i18n.EN: "Crude or refined oil"}},
			{Code: "DL8.48.3.2", Name: i18n.String{i18n.EN: "Unprocessed or processed natural gas"}},
			{Code: "DL8.48.3.3", Name: i18n.String{i18n.EN: "Pure hydrocarbons"}},
		},
	},
}
