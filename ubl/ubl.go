// Package ubl adds the Peppol PINT UBL formats on top of the gobl.ubl base
// import and export, and registers them with gobl.ubl and the GOBL convert
// register. Import it for its side effects:
//
//	import _ "github.com/invopop/gobl.peppol/ubl"
package ubl

import (
	"strings"

	"github.com/invopop/gobl.peppol/addon/aunz"
	"github.com/invopop/gobl.peppol/addon/pint"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/schema"
	"github.com/invopop/gobl/tax"
)

// Keys identifying the Peppol PINT formats, with one layer for each
// specification they build on.
const (
	// KeyPINT identifies the jurisdiction-neutral Peppol PINT billing format.
	KeyPINT cbc.Key = "ubl+peppol+pint"
	// KeyAUNZ identifies the Peppol PINT A-NZ format.
	KeyAUNZ cbc.Key = "ubl+peppol+pint+aunz"
)

// Peppol PINT customization and business-process identifiers, both the
// jurisdiction-neutral base ones and the A-NZ specializations built on them.
const (
	CustomizationPINTBilling     = "urn:peppol:pint:billing-1"
	CustomizationAUNZBilling     = "urn:peppol:pint:billing-1@aunz-1"
	CustomizationAUNZSelfBilling = "urn:peppol:pint:selfbilling-1@aunz-1"
	ProfileBilling               = "urn:peppol:bis:billing"
	ProfileSelfBilling           = "urn:peppol:bis:selfbilling"
)

// Peppol PINT validation exchange specification identifiers (VESIDs), used to
// select the phive rule set a document is validated against.
const (
	VESIDPINTInvoice               = "org.peppol.pint:invoice:1.1.2"
	VESIDPINTCreditNote            = "org.peppol.pint:credit-note:1.1.2"
	VESIDAUNZInvoice               = "org.peppol.pint.aunz:invoice:1.1.2"
	VESIDAUNZCreditNote            = "org.peppol.pint.aunz:creditnote:1.1.2"
	VESIDAUNZInvoiceSelfBilling    = "org.peppol.pint.aunz:invoice-self-billing:1.1.2"
	VESIDAUNZCreditNoteSelfBilling = "org.peppol.pint.aunz:creditnote-self-billing:1.1.2"
)

var invoiceSchema = schema.Lookup(bill.Invoice{})

// FormatPINT defines the jurisdiction-neutral Peppol PINT billing format that
// the jurisdiction specializations extend.
var FormatPINT = goblubl.Format{
	Key:             KeyPINT,
	Name:            i18n.NewString("UBL Peppol PINT Billing"),
	Import:          []schema.ID{invoiceSchema},
	Export:          []schema.ID{invoiceSchema},
	CustomizationID: CustomizationPINTBilling,
	ProfileID:       ProfileBilling,
	Addons:          []cbc.Key{pint.V1},
	VESIDs: goblubl.VESIDMapping{
		Invoice:    VESIDPINTInvoice,
		CreditNote: VESIDPINTCreditNote,
	},
	ImportFuncs: []goblubl.ImportFunc{importPINT},
	// A jurisdiction specialization no dedicated format claimed still follows
	// the base billing model its CustomizationID is suffixed from.
	Fallback: matchCustomization(CustomizationPINTBilling + "@"),
}

// FormatAUNZ defines the Peppol PINT A-NZ format. Invoices are written with
// PINT A-NZ Billing, and self-billed invoices with PINT A-NZ Self-Billing.
var FormatAUNZ = goblubl.Format{
	Key:             KeyAUNZ,
	Name:            i18n.NewString("UBL Peppol PINT A-NZ"),
	Countries:       []l10n.Code{l10n.AU, l10n.NZ},
	Import:          []schema.ID{invoiceSchema},
	Export:          []schema.ID{invoiceSchema},
	CustomizationID: CustomizationAUNZBilling,
	ProfileID:       ProfileBilling,
	Addons:          []cbc.Key{aunz.V1},
	VESIDs: goblubl.VESIDMapping{
		Invoice:    VESIDAUNZInvoice,
		CreditNote: VESIDAUNZCreditNote,
	},
	Specifications: []goblubl.Specification{
		SpecificationAUNZSelfBilling,
	},
	ImportFuncs: []goblubl.ImportFunc{importPINT},
	Match:       matchCustomization(CustomizationAUNZBilling, CustomizationAUNZSelfBilling),
}

// SpecificationAUNZSelfBilling is Peppol PINT A-NZ Self-Billing, used for
// self-billed invoices.
var SpecificationAUNZSelfBilling = goblubl.Specification{
	CustomizationID: CustomizationAUNZSelfBilling,
	ProfileID:       ProfileSelfBilling,
	Addons:          []cbc.Key{aunz.V1},
	VESIDs: goblubl.VESIDMapping{
		Invoice:    VESIDAUNZInvoiceSelfBilling,
		CreditNote: VESIDAUNZCreditNoteSelfBilling,
	},
	Applies: func(doc any) bool {
		inv, ok := doc.(*bill.Invoice)
		return ok && inv.HasTags(tax.TagSelfBilled)
	},
}

// matchCustomization claims every document whose CustomizationID starts with
// one of the given ones, the wildcard the Peppol SMP registrations are made
// under.
func matchCustomization(prefixes ...string) func(string, string) bool {
	return func(customizationID, _ string) bool {
		for _, prefix := range prefixes {
			if strings.HasPrefix(customizationID, prefix) {
				return true
			}
		}
		return false
	}
}

func init() {
	goblubl.RegisterFormats(FormatPINT, FormatAUNZ)
}
