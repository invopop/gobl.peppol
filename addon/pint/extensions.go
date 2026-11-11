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
)

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
}
