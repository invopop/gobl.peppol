package peppol

import (
	"github.com/invopop/gobl"
	ubl "github.com/invopop/gobl.ubl"
)

// Peppol PINT A-NZ customization and business-process  identifiers.
const (
	CustomizationBilling     = "urn:peppol:pint:billing-1@aunz-1"
	CustomizationSelfBilling = "urn:peppol:pint:selfbilling-1@aunz-1"
	ProfileBilling           = "urn:peppol:bis:billing"
	ProfileSelfBilling       = "urn:peppol:bis:selfbilling"
)

// Peppol PINT A-NZ validation exchange specification identifiers (VESIDs),
// used to select the phive rule set the document is validated against.
const (
	VESIDInvoice               = "org.peppol.pint.aunz:invoice:1.1.2"
	VESIDCreditNote            = "org.peppol.pint.aunz:creditnote:1.1.2"
	VESIDInvoiceSelfBilling    = "org.peppol.pint.aunz:invoice-self-billing:1.1.2"
	VESIDCreditNoteSelfBilling = "org.peppol.pint.aunz:creditnote-self-billing:1.1.2"
)

// ContextPINT is the Peppol PINT A-NZ billing context.
var ContextPINT = ubl.Context{
	CustomizationID: CustomizationBilling,
	ProfileID:       ProfileBilling,
	// Addons:          []cbc.Key{aunz.V1},
	VESIDs: ubl.VESIDMapping{Invoice: VESIDInvoice, CreditNote: VESIDCreditNote},
}

// ContextPINTSelfBilled is the Peppol PINT A-NZ self-billing context.
var ContextPINTSelfBilled = ubl.Context{
	CustomizationID: CustomizationSelfBilling,
	ProfileID:       ProfileSelfBilling,
	// Addons:          []cbc.Key{aunz.V1},
	VESIDs: ubl.VESIDMapping{Invoice: VESIDInvoiceSelfBilling, CreditNote: VESIDCreditNoteSelfBilling},
}

// ContextPINTWildcard identifies the A-NZ billing context by its wildcard
// customization ID.
var ContextPINTWildcard = ubl.Context{
	CustomizationID:       CustomizationBilling + "*",
	OutputCustomizationID: CustomizationBilling,
	ProfileID:             ProfileBilling,
	// Addons:                []cbc.Key{aunz.V1},
	VESIDs: ubl.VESIDMapping{Invoice: VESIDInvoice, CreditNote: VESIDCreditNote},
}

// ContextPINTSelfBilledWildcard identifies the A-NZ self-billing context by its
// wildcard customization ID.
var ContextPINTSelfBilledWildcard = ubl.Context{
	CustomizationID:       CustomizationSelfBilling + "*",
	OutputCustomizationID: CustomizationSelfBilling,
	ProfileID:             ProfileSelfBilling,
	// Addons:                []cbc.Key{aunz.V1},
	VESIDs: ubl.VESIDMapping{Invoice: VESIDInvoiceSelfBilling, CreditNote: VESIDCreditNoteSelfBilling},
}

// Context is a UBL conversion context, re-exported so callers can hold the
// A-NZ contexts (ContextPINT and friends) without importing gobl.ubl directly.
type Context = ubl.Context

// Option is a UBL conversion option, re-exported alongside WithContext so
// callers can pass a context to Convert / ConvertInvoice without importing
// gobl.ubl directly.
type Option = ubl.Option

// UBLVersion is the UBL version generated documents declare.
const UBLVersion = ubl.Version

// WithContext returns an Option that converts using the given context,
// overriding the default A-NZ billing context.
func WithContext(ctx Context) Option {
	return ubl.WithContext(ctx)
}

// Convert turns a GOBL envelope into a Peppol PINT A-NZ UBL document using the
// billing context.
func Convert(env *gobl.Envelope, opts ...Option) (any, error) {
	return ubl.Convert(env, withDefaultContext(opts)...)
}

// ConvertInvoice is Convert for callers that already know the document is an
// invoice.
func ConvertInvoice(env *gobl.Envelope, opts ...Option) (*ubl.Invoice, error) {
	return ubl.ConvertInvoice(env, withDefaultContext(opts)...)
}

// withDefaultContext prepends the A-NZ billing context so it applies unless the
// caller supplies their own WithContext option (later options win).
func withDefaultContext(opts []Option) []Option {
	return append([]Option{ubl.WithContext(ContextPINT)}, opts...)
}

// Bytes renders a converted document as indented XML.
func Bytes(in any) ([]byte, error) {
	return ubl.Bytes(in)
}

// BytesCompact renders a converted document as XML without indentation.
func BytesCompact(in any) ([]byte, error) {
	return ubl.BytesCompact(in)
}
