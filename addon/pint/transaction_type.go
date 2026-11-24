package pint

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/tax"
)

// Invoice tags for the PINT AE transaction type flags (BTAE-02). Exports use
// GOBL's own "export" tag.
const (
	TagFreeTradeZone    cbc.Key = "free-trade-zone"
	TagDeemedSupply     cbc.Key = "deemed-supply"
	TagMarginScheme             = ae.TagMarginScheme
	TagSummary          cbc.Key = "summary"
	TagContinuousSupply cbc.Key = "continuous-supply"
	TagDisclosedAgent   cbc.Key = "disclosed-agent"
	TagECommerce        cbc.Key = "e-commerce"
)

// transactionTypePattern is the format of the transaction type: 8 flags of 0 or 1.
const transactionTypePattern = `^[01]{8}$`

// TransactionType is a parsed PINT AE invoice transaction type (BTAE-02).
type TransactionType struct {
	FreeTradeZone    bool
	DeemedSupply     bool
	MarginScheme     bool
	Summary          bool
	ContinuousSupply bool
	DisclosedAgent   bool
	ECommerce        bool
	Export           bool
}

// transactionTypeFlags lists the flags in their order in the code, with the
// tag that sets each one.
var transactionTypeFlags = []struct {
	tag cbc.Key
	get func(*TransactionType) *bool
}{
	{TagFreeTradeZone, func(t *TransactionType) *bool { return &t.FreeTradeZone }},
	{TagDeemedSupply, func(t *TransactionType) *bool { return &t.DeemedSupply }},
	{TagMarginScheme, func(t *TransactionType) *bool { return &t.MarginScheme }},
	{TagSummary, func(t *TransactionType) *bool { return &t.Summary }},
	{TagContinuousSupply, func(t *TransactionType) *bool { return &t.ContinuousSupply }},
	{TagDisclosedAgent, func(t *TransactionType) *bool { return &t.DisclosedAgent }},
	{TagECommerce, func(t *TransactionType) *bool { return &t.ECommerce }},
	{tax.TagExport, func(t *TransactionType) *bool { return &t.Export }},
}

// ParseTransactionType reads the flags of a transaction type code: each position holding a 1 sets
// its flag.
func ParseTransactionType(code cbc.Code) TransactionType {
	var t TransactionType
	s := code.String()
	for i, f := range transactionTypeFlags {
		if len(s) > i && s[i] == '1' {
			*f.get(&t) = true
		}
	}
	return t
}

// Code returns the 8-character transaction type code.
func (t TransactionType) Code() cbc.Code {
	b := []byte("00000000")
	for i, f := range transactionTypeFlags {
		if *f.get(&t) {
			b[i] = '1'
		}
	}
	return cbc.Code(b)
}

// transactionTypeCodesOutsideScope are the transaction types allowed on 480 and 81: those without
// the deemed supply, margin scheme or summary flags (IBR-157-AE).
var transactionTypeCodesOutsideScope = transactionTypeCodes(func(t TransactionType) bool {
	return !t.DeemedSupply && !t.MarginScheme && !t.Summary
})

// transactionTypeCodes lists every transaction type code whose flags pass keep.
func transactionTypeCodes(keep func(TransactionType) bool) []cbc.Code {
	var codes []cbc.Code
	for n := 0; n < 1<<len(transactionTypeFlags); n++ {
		var t TransactionType
		for i, f := range transactionTypeFlags {
			*f.get(&t) = n&(1<<i) != 0
		}
		if keep(t) {
			codes = append(codes, t.Code())
		}
	}
	return codes
}

// Tags returns the invoice tags of the flags that are set.
func (t TransactionType) Tags() []cbc.Key {
	var tags []cbc.Key
	for _, f := range transactionTypeFlags {
		if *f.get(&t) {
			tags = append(tags, f.tag)
		}
	}
	return tags
}

// transactionTypeFromTags builds the transaction type from the invoice tags.
func transactionTypeFromTags(inv *bill.Invoice) TransactionType {
	var t TransactionType
	for _, f := range transactionTypeFlags {
		if inv.HasTags(f.tag) {
			*f.get(&t) = true
		}
	}
	return t
}

// normalizeTransactionType sets the transaction type from the flag tags only: a code from an earlier
// calculation can't be told apart from one set directly.
func normalizeTransactionType(inv *bill.Invoice) {
	if inv == nil {
		return
	}
	if inv.Tax == nil {
		inv.Tax = new(bill.Tax)
	}
	inv.Tax.Ext = inv.Tax.Ext.Set(ExtKeyTransactionType, transactionTypeFromTags(inv).Code())
}

// transactionTypeTags are the invoice tags for the transaction type flags.
var transactionTypeTags = []*cbc.Definition{
	{
		Key:  TagFreeTradeZone,
		Name: i18n.String{i18n.EN: "Free Trade Zone"},
		Desc: i18n.String{i18n.EN: "PINT AE: supply of goods or services in a free trade zone. Needs the beneficiary ID."},
	},
	{
		Key:  TagDeemedSupply,
		Name: i18n.String{i18n.EN: "Deemed Supply"},
		Desc: i18n.String{i18n.EN: "PINT AE: supply of goods or services without consideration."},
	},
	{
		Key:  TagSummary,
		Name: i18n.String{i18n.EN: "Summary Invoice"},
		Desc: i18n.String{i18n.EN: "PINT AE: more than one supply of goods or services to the same person in the same calendar month. Needs the invoicing period."},
	},
	{
		Key:  TagContinuousSupply,
		Name: i18n.String{i18n.EN: "Continuous Supply"},
		Desc: i18n.String{i18n.EN: "PINT AE: supply of goods or services made on a recurrent basis."},
	},
	{
		Key:  TagDisclosedAgent,
		Name: i18n.String{i18n.EN: "Disclosed Agent Billing"},
		Desc: i18n.String{i18n.EN: "PINT AE: an agent who is a registrant makes a supply on behalf of the principal. Needs the principal ID."},
	},
	{
		Key:  TagECommerce,
		Name: i18n.String{i18n.EN: "E-commerce"},
		Desc: i18n.String{i18n.EN: "PINT AE: supply of goods or services through e-commerce. Needs a complete delivery address."},
	},
	{
		Key:  tax.TagExport,
		Name: i18n.String{i18n.EN: "Export"},
		Desc: i18n.String{i18n.EN: "PINT AE: supply of goods or services outside the country. Needs a complete delivery address outside the UAE."},
	},
}

// invoiceHasTags returns a predicate that reports whether an invoice has all the tags.
func invoiceHasTags(keys ...cbc.Key) func(any) bool {
	return func(val any) bool {
		inv, ok := val.(*bill.Invoice)
		return ok && inv != nil && inv.HasTags(keys...)
	}
}
