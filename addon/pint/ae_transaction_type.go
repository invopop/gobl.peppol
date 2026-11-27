package pint

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
)

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

var transactionTypeFlags = []func(*TransactionType) *bool{
	func(t *TransactionType) *bool { return &t.FreeTradeZone },
	func(t *TransactionType) *bool { return &t.DeemedSupply },
	func(t *TransactionType) *bool { return &t.MarginScheme },
	func(t *TransactionType) *bool { return &t.Summary },
	func(t *TransactionType) *bool { return &t.ContinuousSupply },
	func(t *TransactionType) *bool { return &t.DisclosedAgent },
	func(t *TransactionType) *bool { return &t.ECommerce },
	func(t *TransactionType) *bool { return &t.Export },
}

func normalizeTransactionType(inv *bill.Invoice) {
	if inv == nil {
		return
	}
	if inv.Tax == nil {
		inv.Tax = new(bill.Tax)
	}
	inv.Tax.Ext = inv.Tax.Ext.SetIfEmpty(ExtKeyTransactionType, "00000000")
}

// ParseTransactionType reads the 8 binary flags of a PINT AE transaction type code.
func ParseTransactionType(code cbc.Code) TransactionType {
	var t TransactionType
	s := code.String()
	for i, get := range transactionTypeFlags {
		if len(s) > i && s[i] == '1' {
			*get(&t) = true
		}
	}
	return t
}

// Code returns the 8-character transaction type code.
func (t TransactionType) Code() cbc.Code {
	b := []byte("00000000")
	for i, get := range transactionTypeFlags {
		if *get(&t) {
			b[i] = '1'
		}
	}
	return cbc.Code(b)
}

func invoiceTransactionType(check func(TransactionType) bool) func(any) bool {
	return func(val any) bool {
		inv, ok := val.(*bill.Invoice)
		return ok && inv != nil && check(ParseTransactionType(inv.Tax.GetExt(ExtKeyTransactionType)))
	}
}
