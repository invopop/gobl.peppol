// Package bis provides the addon for the Peppol BIS Billing 3.0 profile.
package bis

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

const (
	// Namespace is the rules namespace for the Peppol BIS addon.
	Namespace rules.Code = "PEPPOL-BIS"

	// Key identifies the Peppol BIS Billing addon family.
	Key cbc.Key = "peppol-bis"

	// V1 is the first version of the Peppol BIS Billing 3.0 addon.
	V1 cbc.Key = Key + "-v1"
)

func init() {
	tax.RegisterAddonDef(newV1Addon())
	rules.RegisterWithGuard(
		Key.String(),
		rules.GOBL.Add(Namespace),
		is.InContext(tax.AddonIn(V1)),
		billInvoiceRules(),
		taxComboRules(),
		payInstructionsRules(),
	)
}

func newV1Addon() *tax.AddonDef {
	return &tax.AddonDef{
		Key: V1,
		Name: i18n.String{
			i18n.EN: "Peppol BIS Billing 3.0",
		},
		Requires: []cbc.Key{
			en16931.V2017,
		},
		Description: i18n.String{
			i18n.EN: here.Doc(`
				Support for the Peppol BIS Billing 3.0 profile, the pan-European Peppol
				specification built on EN 16931.

				This addon layers the Peppol-specific business rules
				(PEPPOL-EN16931-*) on top of EN 16931. Rules already guaranteed by
				EN 16931 or GOBL's calculation engine are not duplicated here, and
				country-specific CIUS rules are handled by dedicated jurisdiction
				addons.
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("Peppol BIS Billing 3.0"),
				URL:   "https://docs.peppol.eu/poacc/billing/3.0/",
			},
			{
				Title: i18n.NewString("Peppol BIS Billing 3.0 UBL business rules"),
				URL:   "https://docs.peppol.eu/poacc/billing/3.0/rules/ubl-peppol/",
			},
		},
	}
}
