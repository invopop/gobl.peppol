package bis

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/org"
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
		rules.Field("notes",
			rules.Assert("02", "invoice must not have more than one document-level note (PEPPOL-EN16931-R002)", is.Length(0, 1)),
		),
		rules.Assert("03", "invoice line period must fall within the invoice period (PEPPOL-EN16931-R110, PEPPOL-EN16931-R111)",
			is.Func("line periods within invoice period", invoiceLinePeriodsWithin),
		),
		rules.Field("ordering",
			rules.Field("projects",
				rules.Assert("04", "invoice must not have more than one project reference (PEPPOL-EN16931-R080)", is.Length(0, 1)),
			),
		),
		rules.Field("supplier",
			rules.Assert("05", "invoice supplier electronic address is required (PEPPOL-EN16931-R020)",
				is.Func("has Peppol endpoint", partyHasPeppolEndpoint),
			),
		),
		rules.Field("customer",
			rules.Assert("06", "invoice customer electronic address is required (PEPPOL-EN16931-R010)",
				is.Func("has Peppol endpoint", partyHasPeppolEndpoint),
			),
		),
	)
}

// invoiceHasBuyerRefOrPO reports whether the invoice carries a buyer reference
// (ordering code, ibt-010) or at least one purchase order reference (ibt-013).
func invoiceHasBuyerRefOrPO(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Ordering == nil {
		return false
	}
	return inv.Ordering.Code != "" || len(inv.Ordering.Purchases) > 0
}

// partyHasPeppolEndpoint reports whether the party carries an ISO 6523 endpoint,
// which EN 16931 derives from a Peppol inbox during normalization.
func partyHasPeppolEndpoint(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return false
	}
	return p.Endpoint(iso.ActorIDScheme) != nil
}

// invoiceLinePeriodsWithin reports whether every invoice-line period falls
// within the invoice period. Invoices without a period are not constrained.
func invoiceLinePeriodsWithin(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	if inv.Ordering == nil || inv.Ordering.Period == nil {
		return true
	}
	ip := inv.Ordering.Period
	for _, l := range inv.Lines {
		if l == nil || l.Period == nil {
			continue
		}
		if isSetDate(ip.Start) && isSetDate(l.Period.Start) && l.Period.Start.DaysSince(ip.Start.Date) < 0 {
			return false
		}
		if isSetDate(ip.End) && isSetDate(l.Period.End) && ip.End.DaysSince(l.Period.End.Date) < 0 {
			return false
		}
	}
	return true
}

// isSetDate reports whether an optional date is present and not zero.
func isSetDate(d *cal.Date) bool {
	return d != nil && !d.IsZero()
}
