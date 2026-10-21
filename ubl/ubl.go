// Package ubl adds the Peppol BIS and PINT UBL formats on top of the gobl.ubl
// base import and export, and registers them with gobl.ubl and the GOBL
// convert register. Import it for its side effects:
//
//	import _ "github.com/invopop/gobl.peppol/ubl"
package ubl

import (
	"strings"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl.peppol/addon/pint/aunz"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/schema"
	"github.com/invopop/gobl/tax"
)

const (
	// KeyBIS identifies the Peppol BIS 3 format.
	KeyBIS cbc.Key = "ubl+peppol+bis-v3"
	// KeyPINT identifies the jurisdiction-neutral Peppol PINT billing format.
	KeyPINT cbc.Key = "ubl+peppol+pint-v1"
	// KeyAUNZ identifies the Peppol PINT A-NZ format.
	KeyAUNZ cbc.Key = "ubl+peppol+pint+aunz-v1"
)

var (
	invoiceSchema = schema.Lookup(bill.Invoice{})
	statusSchema  = schema.Lookup(bill.Status{})
)

// FormatBIS defines the Peppol BIS 3 format.
var FormatBIS = goblubl.Format{
	Key:             KeyBIS,
	Name:            i18n.NewString("UBL Peppol BIS 3"),
	Import:          []schema.ID{invoiceSchema, statusSchema},
	Export:          []schema.ID{invoiceSchema, statusSchema},
	CustomizationID: "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0",
	ProfileID:       "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0",
	Addons:          []cbc.Key{en16931.V2017},
	VESIDs: goblubl.VESIDMapping{
		Invoice:    "eu.peppol.bis3:invoice:2026.5",
		CreditNote: "eu.peppol.bis3:creditnote:2026.5",
	},
	Specifications: []goblubl.Specification{
		SpecificationBISSelfBilling,
		SpecificationBISInvoiceResponse,
	},
	ExportFuncs: []goblubl.ExportFunc{exportBISBilling, exportInvoiceResponse},
	ImportFuncs: []goblubl.ImportFunc{importInvoiceResponse},
}

// SpecificationBISSelfBilling is Peppol BIS Self-Billing 3.0, used for
// self-billed invoices.
var SpecificationBISSelfBilling = goblubl.Specification{
	CustomizationID: "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:selfbilling:3.0",
	ProfileID:       "urn:fdc:peppol.eu:2017:poacc:selfbilling:01:1.0",
	Addons:          []cbc.Key{en16931.V2017},
	VESIDs: goblubl.VESIDMapping{
		Invoice:    "eu.peppol.bis3:invoice-self-billing:2026.5",
		CreditNote: "eu.peppol.bis3:creditnote-self-billing:2026.5",
	},
	Applies: isSelfBilled,
}

// SpecificationBISInvoiceResponse is Peppol Invoice Response 3, used for
// statuses.
var SpecificationBISInvoiceResponse = goblubl.Specification{
	CustomizationID: "urn:fdc:peppol.eu:poacc:trns:invoice_response:3",
	ProfileID:       "urn:fdc:peppol.eu:poacc:bis:invoice_response:3",
	VESIDs: goblubl.VESIDMapping{
		Status: "eu.peppol.bis3:invoice-message-response:2026.5",
	},
	Applies: func(doc any) bool {
		_, ok := doc.(*bill.Status)
		return ok
	},
}

// FormatPINT defines the jurisdiction-neutral Peppol PINT billing format that
// the jurisdiction specializations extend.
var FormatPINT = goblubl.Format{
	Key:             KeyPINT,
	Name:            i18n.NewString("UBL Peppol PINT Billing 1"),
	Import:          []schema.ID{invoiceSchema},
	Export:          []schema.ID{invoiceSchema},
	CustomizationID: "urn:peppol:pint:billing-1",
	ProfileID:       "urn:peppol:bis:billing",
	Addons:          []cbc.Key{pint.V1},
	VESIDs: goblubl.VESIDMapping{
		Invoice:    "org.peppol.pint:invoice:1.1.2",
		CreditNote: "org.peppol.pint:credit-note:1.1.2",
	},
	ImportFuncs: []goblubl.ImportFunc{importPINT},
	// A jurisdiction specialization no dedicated format claimed still follows
	// the base billing model its CustomizationID is suffixed from.
	Fallback: matchCustomization("urn:peppol:pint:billing-1@"),
}

// FormatAUNZ defines the Peppol PINT A-NZ format. Invoices are written with
// PINT A-NZ Billing, and self-billed invoices with PINT A-NZ Self-Billing.
var FormatAUNZ = goblubl.Format{
	Key:             KeyAUNZ,
	Name:            i18n.NewString("UBL Peppol PINT A-NZ 1"),
	Countries:       []l10n.Code{l10n.AU, l10n.NZ},
	Import:          []schema.ID{invoiceSchema},
	Export:          []schema.ID{invoiceSchema},
	CustomizationID: "urn:peppol:pint:billing-1@aunz-1",
	ProfileID:       "urn:peppol:bis:billing",
	Addons:          []cbc.Key{aunz.V1},
	VESIDs: goblubl.VESIDMapping{
		Invoice:    "org.peppol.pint.aunz:invoice:1.1.2",
		CreditNote: "org.peppol.pint.aunz:creditnote:1.1.2",
	},
	Specifications: []goblubl.Specification{
		SpecificationAUNZSelfBilling,
	},
	ImportFuncs: []goblubl.ImportFunc{importPINT},
	Match:       matchCustomization("urn:peppol:pint:billing-1@aunz-1", "urn:peppol:pint:selfbilling-1@aunz-1"),
}

// SpecificationAUNZSelfBilling is Peppol PINT A-NZ Self-Billing, used for
// self-billed invoices.
var SpecificationAUNZSelfBilling = goblubl.Specification{
	CustomizationID: "urn:peppol:pint:selfbilling-1@aunz-1",
	ProfileID:       "urn:peppol:bis:selfbilling",
	Addons:          []cbc.Key{aunz.V1},
	VESIDs: goblubl.VESIDMapping{
		Invoice:    "org.peppol.pint.aunz:invoice-self-billing:1.1.2",
		CreditNote: "org.peppol.pint.aunz:creditnote-self-billing:1.1.2",
	},
	Applies: isSelfBilled,
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

// isSelfBilled reports whether the document is a self-billed invoice.
func isSelfBilled(doc any) bool {
	inv, ok := doc.(*bill.Invoice)
	return ok && inv.HasTags(tax.TagSelfBilled)
}

func init() {
	goblubl.RegisterFormats(FormatBIS, FormatPINT, FormatAUNZ)
}
