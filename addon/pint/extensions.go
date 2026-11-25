package pint

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
	"github.com/invopop/gobl/regimes/ae"
)

// PINT AE extension keys, for the BTAE fields GOBL has no place for.
const (
	ExtKeyTransactionType cbc.Key = "peppol-pint-ae-transaction-type"
)

// Usages of the PINT AE extensions, under the AE regime.
var (
	usageAEInvoiceTax = &cbc.Usage{
		Keys:    []cbc.Key{V1},
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{bill.ShortSchemaTax},
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
}
