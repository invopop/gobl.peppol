package bis

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
)

// billInvoiceRules enforces the Peppol BIS Billing 3.0 document-level rules that
// are not already guaranteed by EN 16931.
func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		rules.Assert("01", "invoice must have a buyer reference or a purchase order reference (PEPPOL-EN16931-R003)",
			is.Func("has buyer reference or purchase order", invoiceHasBuyerRefOrPO),
		),
		rules.Assert("02", "invoice must not have more than one document-level note (PEPPOL-EN16931-R002)",
			is.Func("at most one note", invoiceHasAtMostOneNote),
		),
		rules.Assert("05", "invoice line period must fall within the invoice period (PEPPOL-EN16931-R110, PEPPOL-EN16931-R111)",
			is.Func("line periods within invoice period", invoiceLinePeriodsWithin),
		),
		rules.Field("supplier",
			rules.Field("inboxes",
				rules.Assert("03", "invoice supplier electronic address is required (PEPPOL-EN16931-R020)", is.Present),
			),
		),
		rules.Field("customer",
			rules.Field("inboxes",
				rules.Assert("04", "invoice customer electronic address is required (PEPPOL-EN16931-R010)", is.Present),
			),
		),
	)
}

// invoiceHasBuyerRefOrPO reports whether the invoice carries a buyer reference
// (ordering code, ibt-010) or at least one purchase order reference (ibt-013).
func invoiceHasBuyerRefOrPO(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	if inv.Ordering == nil {
		return false
	}
	return inv.Ordering.Code != "" || len(inv.Ordering.Purchases) > 0
}

// invoiceHasAtMostOneNote reports whether the invoice has no more than one
// document-level note. Peppol relaxes this when both parties are German, which
// is left to a dedicated German jurisdiction addon.
func invoiceHasAtMostOneNote(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	return len(inv.Notes) <= 1
}

// invoiceLinePeriodsWithin reports whether every invoice-line period falls
// within the invoice period. Lines without a period, or an invoice without a
// period, are not constrained.
func invoiceLinePeriodsWithin(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Ordering == nil || inv.Ordering.Period == nil {
		return true
	}
	ip := inv.Ordering.Period
	if !dateSet(ip.Start) || !dateSet(ip.End) {
		return true
	}
	for _, l := range inv.Lines {
		if l == nil || l.Period == nil {
			continue
		}
		if dateSet(l.Period.Start) && l.Period.Start.DaysSince(ip.Start.Date) < 0 {
			return false
		}
		if dateSet(l.Period.End) && ip.End.DaysSince(l.Period.End.Date) < 0 {
			return false
		}
	}
	return true
}

// dateSet reports whether the date is present and not zero.
func dateSet(d *cal.Date) bool {
	return d != nil && !d.IsZero()
}
