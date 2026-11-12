package pint

import (
	"regexp"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/tax"
)

// Invoice tags for the PINT AE transaction type flags (BTAE-02). Exports use
// GOBL's own "export" tag.
const (
	// TagFreeTradeZone marks a supply of goods or services in a free trade zone.
	TagFreeTradeZone cbc.Key = "free-trade-zone"
	// TagDeemedSupply marks a supply of goods or services without
	// consideration.
	TagDeemedSupply cbc.Key = "deemed-supply"
	// TagMarginScheme marks a supply of goods under the profit margin scheme.
	TagMarginScheme cbc.Key = "margin-scheme"
	// TagSummary marks a summary invoice: more than one supply to the same
	// person in the same calendar month.
	TagSummary cbc.Key = "summary"
	// TagContinuousSupply marks a supply made on a recurrent basis.
	TagContinuousSupply cbc.Key = "continuous-supply"
	// TagDisclosedAgent marks a supply an agent who is a registrant makes on
	// behalf of a principal.
	TagDisclosedAgent cbc.Key = "disclosed-agent"
	// TagECommerce marks a supply of goods or services through e-commerce.
	TagECommerce cbc.Key = "e-commerce"
)

// transactionTypePattern is the format of the transaction type: 8 flags of
// 0 or 1 (IBR-154-AE).
const transactionTypePattern = `^[01]{8}$`

var transactionTypeRegexp = regexp.MustCompile(transactionTypePattern)

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

// ParseTransactionType reads the flags of a transaction type code. A flag is
// set when its position holds a 1; a missing or malformed code gives no
// flags, as IBR-154-AE checks the format.
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

// invoiceTransactionType returns the parsed transaction type of an invoice.
func invoiceTransactionType(val any) TransactionType {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return TransactionType{}
	}
	return ParseTransactionType(inv.Tax.GetExt(ExtKeyTransactionType))
}

// normalizeTransactionType sets the transaction type of a UAE invoice from its
// flag tags, "00000000" without any. Every PINT AE document needs one
// (IBR-154-AE). The tags are the only source: a code from an earlier
// calculation can't be told apart from one set directly, so keeping it would
// leave the flags of removed tags in place.
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
		Key:  TagMarginScheme,
		Name: i18n.String{i18n.EN: "Profit Margin Scheme"},
		Desc: i18n.String{i18n.EN: "PINT AE: supply of goods under the profit margin scheme."},
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

// transactionTypeIs returns a predicate that reports whether an invoice's
// transaction type has the flag that check reads.
func transactionTypeIs(check func(TransactionType) bool) func(any) bool {
	return func(val any) bool {
		return check(invoiceTransactionType(val))
	}
}

var (
	invoiceIsFreeTradeZone  = transactionTypeIs(func(t TransactionType) bool { return t.FreeTradeZone })
	invoiceIsDeemedSupply   = transactionTypeIs(func(t TransactionType) bool { return t.DeemedSupply })
	invoiceIsSummary        = transactionTypeIs(func(t TransactionType) bool { return t.Summary })
	invoiceIsDisclosedAgent = transactionTypeIs(func(t TransactionType) bool { return t.DisclosedAgent })
	invoiceIsECommerce      = transactionTypeIs(func(t TransactionType) bool { return t.ECommerce })
	invoiceIsExport         = transactionTypeIs(func(t TransactionType) bool { return t.Export })
	invoiceIsMarginScheme   = transactionTypeIs(func(t TransactionType) bool { return t.MarginScheme })
	// invoiceIsSummaryDeemedOrMargin reports the flags outside scope documents
	// and credit notes related to goods and services can't have (IBR-157-AE).
	invoiceIsSummaryDeemedOrMargin = transactionTypeIs(func(t TransactionType) bool {
		return t.Summary || t.DeemedSupply || t.MarginScheme
	})
)
