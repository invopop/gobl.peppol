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
	ExtKeyTransactionType cbc.Key = "peppol-pint-ae-transaction-type"
	ExtKeyGoodsType       cbc.Key = "peppol-pint-ae-goods-type" // goods or services under reverse charge
)

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
