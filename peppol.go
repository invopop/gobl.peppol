package peppol

import (
	"github.com/invopop/gobl"
	ubl "github.com/invopop/gobl.ubl"
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
// select the phive rule set the document is validated against.
const (
	VESIDPINTInvoice               = "org.peppol.pint:invoice:1.1.2"
	VESIDPINTCreditNote            = "org.peppol.pint:credit-note:1.1.2"
	VESIDAUNZInvoice               = "org.peppol.pint.aunz:invoice:1.1.2"
	VESIDAUNZCreditNote            = "org.peppol.pint.aunz:creditnote:1.1.2"
	VESIDAUNZInvoiceSelfBilling    = "org.peppol.pint.aunz:invoice-self-billing:1.1.2"
	VESIDAUNZCreditNoteSelfBilling = "org.peppol.pint.aunz:creditnote-self-billing:1.1.2"
)

// ContextPINT is the jurisdiction-neutral Peppol PINT billing context that the
// jurisdiction specializations extend.
var ContextPINT = ubl.Context{
	CustomizationID:       CustomizationPINTBilling + "*",
	OutputCustomizationID: CustomizationAUNZBilling,
	ProfileID:             ProfileBilling,
	// Addons:                []cbc.Key{pint.V1},
	VESIDs: ubl.VESIDMapping{Invoice: VESIDAUNZInvoice, CreditNote: VESIDAUNZCreditNote},
}

// ContextAUNZ is the Peppol PINT A-NZ billing context.
var ContextAUNZ = ubl.Context{
	CustomizationID: CustomizationAUNZBilling,
	ProfileID:       ProfileBilling,
	// Addons:          []cbc.Key{aunz.V1},
	VESIDs: ubl.VESIDMapping{Invoice: VESIDAUNZInvoice, CreditNote: VESIDAUNZCreditNote},
}

// ContextAUNZSelfBilled is the Peppol PINT A-NZ self-billing context.
var ContextAUNZSelfBilled = ubl.Context{
	CustomizationID: CustomizationAUNZSelfBilling,
	ProfileID:       ProfileSelfBilling,
	// Addons:          []cbc.Key{aunz.V1},
	VESIDs: ubl.VESIDMapping{Invoice: VESIDAUNZInvoiceSelfBilling, CreditNote: VESIDAUNZCreditNoteSelfBilling},
}

// ContextAUNZWildcard identifies the A-NZ billing context by its wildcard
// customization ID.
var ContextAUNZWildcard = ubl.Context{
	CustomizationID:       CustomizationAUNZBilling + "*",
	OutputCustomizationID: CustomizationAUNZBilling,
	ProfileID:             ProfileBilling,
	// Addons:                []cbc.Key{aunz.V1},
	VESIDs: ubl.VESIDMapping{Invoice: VESIDAUNZInvoice, CreditNote: VESIDAUNZCreditNote},
}

// ContextAUNZSelfBilledWildcard identifies the A-NZ self-billing context by its
// wildcard customization ID.
var ContextAUNZSelfBilledWildcard = ubl.Context{
	CustomizationID:       CustomizationAUNZSelfBilling + "*",
	OutputCustomizationID: CustomizationAUNZSelfBilling,
	ProfileID:             ProfileSelfBilling,
	// Addons:                []cbc.Key{aunz.V1},
	VESIDs: ubl.VESIDMapping{Invoice: VESIDAUNZInvoiceSelfBilling, CreditNote: VESIDAUNZCreditNoteSelfBilling},
}

// Context is a UBL conversion context, re-exported so callers can hold the PINT
// contexts (ContextPINT, ContextAUNZ and friends) without importing gobl.ubl
// directly.
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
// A-NZ billing context, the module default.
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
	return append([]Option{ubl.WithContext(ContextAUNZ)}, opts...)
}

// Bytes renders a converted document as indented XML.
func Bytes(in any) ([]byte, error) {
	return ubl.Bytes(in)
}

// BytesCompact renders a converted document as XML without indentation.
func BytesCompact(in any) ([]byte, error) {
	return ubl.BytesCompact(in)
}
