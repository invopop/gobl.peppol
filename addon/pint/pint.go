// Package pint provides the base addon for the Peppol International (PINT)
// billing model.
package pint

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/norm"
	"github.com/invopop/gobl/pkg/here"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

const (
	// Namespace is the rules namespace for the Peppol PINT addon.
	Namespace rules.Code = "PEPPOL-PINT"

	// Key identifies the Peppol PINT addon family.
	Key cbc.Key = "peppol-pint"

	// V1 is the first version of the Peppol PINT addon.
	V1 cbc.Key = Key + "-v1"
)

// regimesAUNZ lists the tax regimes the PINT A-NZ jurisdiction rules apply to.
var regimesAUNZ = []l10n.TaxCountryCode{l10n.AU.Tax(), l10n.NZ.Tax()}

func init() {
	tax.RegisterAddonDef(newV1Addon())
	rules.RegisterWithGuard(
		Key.String(),
		rules.GOBL.Add(Namespace),
		is.InContext(tax.AddonIn(V1)),
		billInvoiceRules(),
		taxComboRules(),
		taxCategoryTotalRules(),
		payInstructionsRules(),
		creditTransferRules(),
	)
	norm.RegisterWithGuard(
		is.InContext(tax.AddonIn(V1)),
		norm.For(normalizeTaxCombo),
	)
}

func newV1Addon() *tax.AddonDef {
	return &tax.AddonDef{
		Key: V1,
		Name: i18n.String{
			i18n.EN: "Peppol PINT",
		},
		Requires: []cbc.Key{
			en16931.V2017,
		},
		Description: i18n.String{
			i18n.EN: here.Doc(`
				Support for the Peppol International (PINT) billing model, the common
				layer shared by the country-specific PINT specifications.

				PINT builds on EN 16931 but, unlike the EU CIUS, recognises indirect
				tax schemes beyond VAT. This addon re-maps the
				UNTDID 5305 tax category code from the GOBL tax key so that, for
				example, standard-rated GST is reported as "S" (standard rated) instead
				of the EN 16931 default of "O" (outside scope).

				Jurisdiction-specific requirements are applied according to the
				document's tax regime: Australian and New Zealand documents are also
				validated against the PINT A-NZ rules.
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("Peppol PINT specifications"),
				URL:   "https://docs.peppol.eu/poac/pint/",
			},
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
