// Package au provides the Australia jurisdiction addon for the Peppol PINT
// billing model.
package au

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
	// Namespace is the rules namespace for the Peppol PINT Australia addon.
	Namespace rules.Code = "PEPPOL-PINT-AU"

	// Key identifies the Peppol PINT Australia addon family.
	Key cbc.Key = "peppol-pint-au"

	// V1 is the first version of the Peppol PINT Australia addon.
	V1 cbc.Key = Key + "-v1"
)

func init() {
	tax.RegisterAddonDef(newV1Addon())
	rules.RegisterWithGuard(
		Key.String(),
		rules.GOBL.Add(Namespace),
		is.InContext(tax.AddonIn(V1)),
		billInvoiceRules(),
	)
}

func newV1Addon() *tax.AddonDef {
	return &tax.AddonDef{
		Key: V1,
		Name: i18n.String{
			i18n.EN: "Peppol PINT Australia",
		},
		Requires: []cbc.Key{
			pint.V1,
		},
		Description: i18n.String{
			i18n.EN: here.Doc(`
				Australia jurisdiction extension of the Peppol PINT billing model.

				It builds on the base PINT addon and adds the requirement that
				Australian parties carry their Australian Business Number (ABN) as a
				legal registration identity.
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("Peppol PINT A-NZ Billing specification"),
				URL:   "https://docs.peppol.eu/pint/pint-aunz/",
			},
		},
	}
}
