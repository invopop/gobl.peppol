// Package cii adds the Peppol BIS CII format on top of the gobl.cii base
// import and export, and registers it with gobl.cii and the GOBL convert
// register. Import it for its side effects:
//
//	import _ "github.com/invopop/gobl.peppol/cii"
package cii

import (
	goblcii "github.com/invopop/gobl.cii"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/schema"
)

// KeyBIS identifies the Peppol BIS 3 CII format.
const KeyBIS cbc.Key = "cii+peppol+bis-v3"

var invoiceSchema = schema.Lookup(bill.Invoice{})

// FormatBIS defines the Peppol BIS Billing 3.0 CII format.
var FormatBIS = goblcii.Format{
	Key:         KeyBIS,
	Name:        i18n.NewString("CII Peppol BIS Billing 3"),
	Import:      []schema.ID{invoiceSchema},
	Export:      []schema.ID{invoiceSchema},
	GuidelineID: goblcii.GuidelineIDEN16931 + "#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0",
	BusinessID:  "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0",
	Version:     goblcii.VersionD16B,
	Addons:      []cbc.Key{en16931.V2017},
	Attachments: true,
	VESID:       "eu.cen.en16931:cii:1.3.16",
	ExportFuncs: []goblcii.ExportFunc{exportBISBilling},
}

func init() {
	goblcii.RegisterFormats(FormatBIS)
}
