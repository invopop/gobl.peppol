// Package ubl adds the Peppol BIS and PINT UBL formats on top of the gobl.ubl
// base import and export, and registers them with gobl.ubl and the GOBL
// convert register. Import it for its side effects:
//
//	import _ "github.com/invopop/gobl.peppol/ubl"
package ubl

import (
	"strings"

	"github.com/invopop/gobl.peppol/addon/aunz"
	"github.com/invopop/gobl.peppol/addon/pint"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/schema"
	"github.com/invopop/gobl/tax"
)

// Keys identifying the Peppol formats, with one layer for each specification
// they build on.
const (
	// KeyBIS identifies the Peppol BIS 3 format.
	KeyBIS cbc.Key = "ubl+peppol+bis-v3"
	// KeyPINT identifies the jurisdiction-neutral Peppol PINT billing format.
	KeyPINT cbc.Key = "ubl+peppol+pint"
	// KeyAUNZ identifies the Peppol PINT A-NZ format.
	KeyAUNZ cbc.Key = "ubl+peppol+pint+aunz"
)

// Peppol BIS 3 customization and business-process identifiers, for billing,
// self-billing and the Invoice Response.
const (
	CustomizationBISBilling         = "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0"
	CustomizationBISSelfBilling     = "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:selfbilling:3.0"
	CustomizationBISInvoiceResponse = "urn:fdc:peppol.eu:poacc:trns:invoice_response:3"
	ProfileBISBilling               = "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0"
	ProfileBISSelfBilling           = "urn:fdc:peppol.eu:2017:poacc:selfbilling:01:1.0"
	ProfileBISInvoiceResponse       = "urn:fdc:peppol.eu:poacc:bis:invoice_response:3"
)

// Peppol BIS 3 validation exchange specification identifiers (VESIDs). The
// self-billing rule sets are versioned independently from the billing ones.
const (
	VESIDBISInvoice                = "eu.peppol.bis3:invoice:2026.5"
	VESIDBISCreditNote             = "eu.peppol.bis3:creditnote:2026.5"
	VESIDBISInvoiceSelfBilling     = "eu.peppol.bis3:invoice-self-billing:2026.5"
	VESIDBISCreditNoteSelfBilling  = "eu.peppol.bis3:creditnote-self-billing:2026.5"
	VESIDBISInvoiceMessageResponse = "eu.peppol.bis3:invoice-message-response:2026.5"
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

var (
	invoiceSchema = schema.Lookup(bill.Invoice{})
	statusSchema  = schema.Lookup(bill.Status{})
)

// FormatBIS defines the Peppol BIS 3 format. Invoices are written with Peppol
// BIS Billing 3.0, self-billed invoices with Peppol BIS Self-Billing 3.0, and
// statuses with Peppol Invoice Response 3.
var FormatBIS = goblubl.Format{
	Key:             KeyBIS,
	Name:            i18n.NewString("UBL Peppol BIS 3"),
	Import:          []schema.ID{invoiceSchema, statusSchema},
	Export:          []schema.ID{invoiceSchema, statusSchema},
	CustomizationID: CustomizationBISBilling,
	ProfileID:       ProfileBISBilling,
	Addons:          []cbc.Key{en16931.V2017},
	VESIDs: goblubl.VESIDMapping{
		Invoice:    VESIDBISInvoice,
		CreditNote: VESIDBISCreditNote,
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
	CustomizationID: CustomizationBISSelfBilling,
	ProfileID:       ProfileBISSelfBilling,
	Addons:          []cbc.Key{en16931.V2017},
	VESIDs: goblubl.VESIDMapping{
		Invoice:    VESIDBISInvoiceSelfBilling,
		CreditNote: VESIDBISCreditNoteSelfBilling,
	},
	Applies: isSelfBilled,
}

// SpecificationBISInvoiceResponse is Peppol Invoice Response 3, used for
// statuses.
var SpecificationBISInvoiceResponse = goblubl.Specification{
	CustomizationID: CustomizationBISInvoiceResponse,
	ProfileID:       ProfileBISInvoiceResponse,
	VESIDs: goblubl.VESIDMapping{
		Status: VESIDBISInvoiceMessageResponse,
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
