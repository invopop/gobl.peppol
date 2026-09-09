// Package aunz provides the Australia/New Zealand (A-NZ) jurisdiction addon for
// the Peppol PINT billing model.
package aunz

import (
	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

const (
	// Namespace is the rules namespace for the Peppol PINT A-NZ addon.
	Namespace rules.Code = "PEPPOL-PINT-AUNZ"

	// Key identifies the Peppol PINT A-NZ addon family.
	Key cbc.Key = "peppol-pint-aunz"

	// V1 is the first version of the Peppol PINT A-NZ addon.
	V1 cbc.Key = Key + "-v1"
)

func init() {
	tax.RegisterAddonDef(newV1Addon())
	rules.RegisterWithGuard(
		Key.String(),
		rules.GOBL.Add(Namespace),
		is.InContext(tax.AddonIn(V1)),
		payInstructionsRules(),
		creditTransferRules(),
		taxComboRules(),
		taxCategoryTotalRules(),
	)
}

func newV1Addon() *tax.AddonDef {
	return &tax.AddonDef{
		Key: V1,
		Name: i18n.String{
			i18n.EN: "Peppol PINT A-NZ",
		},
		Requires: []cbc.Key{
			pint.V1,
		},
		Description: i18n.String{
			i18n.EN: here.Doc(`
				Support for Australia and New Zealand (AU-NZ) jurisdiction extension of
				the Peppol PINT billing model.

				This addon extends Peppol PINT with AU-NZ fields and validations.
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("Peppol PINT A-NZ Billing specification"),
				URL:   "https://docs.peppol.eu/poac/aunz/pint-aunz/bis/",
			},
			{
				Title: i18n.NewString("PINT A-NZ jurisdiction-aligned rules"),
				URL:   "https://docs.peppol.eu/poac/aunz/pint-aunz/trn-invoice/rule/PINT-jurisdiction-aligned-rules/",
			},
		},
	}
}
