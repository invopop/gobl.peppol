package ubl

import (
	"strings"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.peppol/addon/pint"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/tax"
)

// UNTDID 5305 VAT category codes the AE format writes rates and amounts for.
const (
	taxCategoryStandard      = "S"
	taxCategoryExempt        = "E"
	taxCategoryReverseCharge = "AE"
)

// aedTotalDocumentType is the document type code PINT AE gives the additional
// document reference that carries the total with VAT in AED (BTAE-20).
const aedTotalDocumentType = "aedtotal-incl-vat"

// aeReverseChargeRate is the rate PINT AE expects on reverse charge lines and
// breakdowns. GOBL keeps reverse charge without a percent, as no VAT is
// charged, but the PINT AE examples and the 5% standard rate (IBR-190-AE)
// write 5.
const aeReverseChargeRate = "5"

// taxSchemeTIN is the tax scheme PINT AE writes the seller TIN under.
const taxSchemeTIN = "TIN"

// exportAE applies the Peppol PINT AE rules to an exported invoice. Each step
// only uses data GOBL already has; the AE amounts and the BTAE extensions are
// added by later steps.
func exportAE(_ *goblubl.Format, env *gobl.Envelope, doc goblubl.Document) error {
	out, ok := doc.(*goblubl.Invoice)
	inv, ok2 := env.Extract().(*bill.Invoice)
	if !ok || !ok2 {
		return nil
	}
	exportAEUUID(out, inv)
	exportAELines(out.InvoiceLines, inv)
	exportAELines(out.CreditNoteLines, inv)
	for _, p := range goblubl.InvoiceParties(inv, out) {
		exportAEPartyTaxID(p.UBL, p.GOBL.TaxID)
	}
	exportAEPartyIdentities(out.AccountingSupplierParty.Party, inv.Supplier)
	exportAEPartyIdentities(out.AccountingCustomerParty.Party, inv.Customer)
	for i := range out.TaxTotal {
		exportAETaxTotal(&out.TaxTotal[i])
	}
	exportAEExchangeRate(out, inv)
	return nil
}

// importAE applies the Peppol PINT AE rules to an imported invoice.
func importAE(_ *goblubl.Format, doc goblubl.Document, env *gobl.Envelope) error {
	in, ok := doc.(*goblubl.Invoice)
	inv, ok2 := env.Extract().(*bill.Invoice)
	if !ok || !ok2 {
		return nil
	}
	importAEOutsideScope(in, inv)
	importAEPartyIdentities(in.AccountingSupplierParty.Party, inv.Supplier, true)
	importAEPartyIdentities(in.AccountingCustomerParty.Party, inv.Customer, false)
	return nil
}

// importAEOutsideScope reads the PINT AE out-of-scope document types, which
// gobl.ubl does not know and imports as "other": 480 becomes a standard
// invoice and 81 a credit note, both with the outside-scope tag. The PINT
// scenarios then set the type code back when the document is calculated.
func importAEOutsideScope(in *goblubl.Invoice, inv *bill.Invoice) {
	code := in.InvoiceTypeCode
	if code == nil {
		code = in.CreditNoteTypeCode
	}
	if code == nil {
		return
	}
	switch code.Value {
	case "480":
		inv.Type = bill.InvoiceTypeStandard
	case "81":
		inv.Type = bill.InvoiceTypeCreditNote
	default:
		return
	}
	if !inv.HasTags(pint.TagOutsideScope) {
		inv.Tags.List = append(inv.Tags.List, pint.TagOutsideScope)
	}
}

// exportAEUUID writes the invoice's UUID as cbc:UUID, the unique identifier
// number BTAE-07 (IBR-193-AE).
func exportAEUUID(out *goblubl.Invoice, inv *bill.Invoice) {
	if !inv.UUID.IsZero() {
		out.UUID = inv.UUID.String()
	}
}

// exportAELines completes the UBL lines, which gobl.ubl writes in the same
// order as the invoice's lines.
func exportAELines(lines []goblubl.InvoiceLine, inv *bill.Invoice) {
	for i := range lines {
		exportAELine(&lines[i])
		if len(lines) == len(inv.Lines) {
			exportAELineAmounts(&lines[i], inv.Lines[i], inv.Currency)
		}
	}
}

// exportAELine completes a line's price details and VAT rate.
func exportAELine(line *goblubl.InvoiceLine) {
	exportAEPrice(line)
	if line.Item != nil && line.Item.ClassifiedTaxCategory != nil {
		c := line.Item.ClassifiedTaxCategory
		c.Percent = aeRate(c.ID, c.Percent)
	}
}

// exportAELineAmounts writes the line amount payable (BTAE-10) and the line VAT
// amount (BTAE-08) in cac:ItemPriceExtension, which every line needs
// (IBR-194-AE, IBR-104-AE). Despite their names, the official examples write
// both in the document currency, not in AED. GOBL keeps no VAT per line, so it
// is the line total at the line's rate, rounded to the currency. Exempt lines
// get no VAT amount (IBR-163-AE), and zero rated and reverse charge lines a
// zero one (IBR-165-AE, IBR-162-AE).
func exportAELineAmounts(line *goblubl.InvoiceLine, l *bill.Line, cur currency.Code) {
	if l == nil || l.Total == nil {
		return
	}
	def := cur.Def()
	if def == nil {
		return
	}
	total := def.Rescale(*l.Total)
	ext := &goblubl.ItemPriceExtension{Amount: aeAmount(total, cur)}
	if vat, ok := aeLineVAT(line, l); ok {
		vat = def.Rescale(vat)
		ext.Amount = aeAmount(total.Add(vat), cur)
		ext.TaxTotal = []goblubl.TaxTotal{{TaxAmount: aeAmount(vat, cur)}}
	}
	line.ItemPriceExtension = ext
}

// aeLineVAT provides the VAT charged on a line, and false for exempt lines,
// which must not have one. Only standard rated lines charge VAT: zero rated,
// reverse charge and outside scope lines charge none.
func aeLineVAT(line *goblubl.InvoiceLine, l *bill.Line) (num.Amount, bool) {
	zero := num.MakeAmount(0, l.Total.Exp())
	if line.Item == nil || line.Item.ClassifiedTaxCategory == nil || line.Item.ClassifiedTaxCategory.ID == nil {
		return zero, true
	}
	switch line.Item.ClassifiedTaxCategory.ID.Value {
	case taxCategoryExempt:
		return zero, false
	case taxCategoryStandard:
		if c := l.Taxes.Get(tax.CategoryVAT); c != nil && c.Percent != nil {
			return c.Percent.Of(*l.Total), true
		}
	}
	return zero, true
}

// exportAEExchangeRate writes, for invoices not in AED, the exchange rate to
// AED (BTAE-04, IBR-159-AE) as cac:TaxExchangeRate from the document currency
// to the tax currency (IBR-153-AE), and the total with VAT in AED (BTAE-20,
// IBR-175-AE). gobl.ubl already writes the tax currency AED and the VAT total
// in AED (ibt-006, ibt-111) when there is a rate. The PINT addon requires the
// rate (GOBL-PEPPOL-PINT-BILL-INVOICE-05) with at most 6 decimals (-06).
func exportAEExchangeRate(out *goblubl.Invoice, inv *bill.Invoice) {
	if inv.Currency == currency.AED {
		return
	}
	rate := currency.MatchExchangeRate(inv.ExchangeRates, inv.Currency, currency.AED)
	if rate == nil {
		return
	}
	source, target, calc := inv.Currency.String(), currency.AED.String(), rate.Amount.String()
	out.TaxExchangeRate = &goblubl.ExchangeRate{
		SourceCurrencyCode: &source,
		TargetCurrencyCode: &target,
		CalculationRate:    &calc,
	}
	if inv.Totals != nil {
		total := rate.Convert(inv.Totals.TotalWithTax)
		out.AdditionalDocumentReference = append(out.AdditionalDocumentReference, goblubl.Reference{
			ID:                  goblubl.IDType{Value: currency.AED.String()},
			DocumentTypeCode:    aedTotalDocumentType,
			DocumentDescription: currency.AED.String() + " " + total.String(),
		})
	}
}

// aeAmount provides a UBL amount in the currency.
func aeAmount(a num.Amount, cur currency.Code) goblubl.Amount {
	code := cur.String()
	return goblubl.Amount{CurrencyID: &code, Value: a.String()}
}

// exportAEPrice adds the price base quantity (ibt-149) and the gross price
// (ibt-148) PINT AE requires on every line (IBR-126-AE). GOBL prices are per
// unit, so the base quantity is 1 in the line's unit. Without a list price,
// gobl.ubl writes no price allowance, so the gross price is the net price with
// a zero discount. The base quantity also lets IBR-147-AE compute the line net
// amount.
func exportAEPrice(line *goblubl.InvoiceLine) {
	price := line.Price
	if price == nil {
		return
	}
	if price.BaseQuantity == nil {
		price.BaseQuantity = &goblubl.Quantity{UnitCode: lineUnitCode(line), Value: "1"}
	}
	if price.AllowanceCharge == nil {
		gross := price.PriceAmount
		price.AllowanceCharge = &goblubl.AllowanceCharge{
			ChargeIndicator: false,
			Amount:          goblubl.Amount{CurrencyID: price.PriceAmount.CurrencyID, Value: "0"},
			BaseAmount:      &gross,
		}
	}
}

// lineUnitCode provides the unit code of the line's invoiced or credited
// quantity.
func lineUnitCode(line *goblubl.InvoiceLine) string {
	if line.InvoicedQuantity != nil {
		return line.InvoicedQuantity.UnitCode
	}
	if line.CreditedQuantity != nil {
		return line.CreditedQuantity.UnitCode
	}
	return ""
}

// aeRate provides the rate PINT AE expects for a line or breakdown VAT
// category. gobl.ubl writes 0 when a combo has no percent, which is right for
// A-NZ but not for AE: exempt must have no rate (aligned-ibrp-e-05,
// IBR-121-AE), and reverse charge must have one (aligned-ibrp-ae-05-ae,
// IBR-119-AE). Document allowances and charges keep gobl.ubl's 0, which is
// what PINT AE wants there for both (aligned-ibrp-e-06/07, ae-06/07).
func aeRate(id *goblubl.IDType, percent *string) *string {
	if id == nil {
		return percent
	}
	switch id.Value {
	case taxCategoryExempt:
		return nil
	case taxCategoryReverseCharge:
		rate := aeReverseChargeRate
		return &rate
	}
	return percent
}

// exportAEPartyTaxID writes a UAE TRN without the AE prefix gobl.ubl adds,
// the bare 15 digits IBR-132-AE checks on the seller, buyer and tax
// representative. Tax IDs from other countries keep their prefix, the usual
// VAT number format.
func exportAEPartyTaxID(out *goblubl.Party, tID *tax.Identity) {
	if out == nil || tID == nil || tID.Code == "" || tID.Country.Code() != l10n.AE {
		return
	}
	prefixed := tID.String()
	for i := range out.PartyTaxScheme {
		id := out.PartyTaxScheme[i].CompanyID
		if id != nil && id.Value == prefixed {
			id.Value = tID.Code.String()
		}
	}
}

// exportAEPartyIdentities writes the PINT AE details of the seller and buyer
// identities that gobl.ubl has no place for.
func exportAEPartyIdentities(out *goblubl.Party, p *org.Party) {
	if out == nil || p == nil {
		return
	}
	exportAELegalIdentity(out, p)
	exportAETINScheme(out, p)
}

// exportAELegalIdentity writes the legal registration identifier type of the
// party's legal identity as @schemeAgencyID (BTAE-15, BTAE-16), and, as
// @schemeAgencyName, the issuing country of a passport (BTAE-18, BTAE-19) or
// the issuing authority of a trade licence, taken from its description
// (BTAE-11, BTAE-12; IBR-101-AE, IBR-172-AE). The syntax binding also allows
// an authority on Emirates IDs and Cabinet Decisions, but no rule or text asks
// for one, so none is written. gobl.ubl writes the first legal identity as
// cac:PartyLegalEntity/cbc:CompanyID, so that is the one described here.
func exportAELegalIdentity(out *goblubl.Party, p *org.Party) {
	if out.PartyLegalEntity == nil || out.PartyLegalEntity.CompanyID == nil {
		return
	}
	var id *org.Identity
	for _, i := range p.Identities {
		if i != nil && i.Scope == org.IdentityScopeLegal {
			id = i
			break
		}
	}
	if id == nil {
		return
	}
	typ := pint.IdentityTypeAE(id.Key)
	if typ == "" {
		return
	}
	cid := out.PartyLegalEntity.CompanyID
	agencyID := typ.String()
	cid.SchemeAgencyID = &agencyID
	switch typ {
	case pint.IdentityTypePassport:
		if id.Country != "" {
			name := id.Country.String()
			cid.SchemeAgencyName = &name
		}
	case pint.IdentityTypeTradeLicense:
		if id.Description != "" {
			name := id.Description
			cid.SchemeAgencyName = &name
		}
	}
}

// importAEPartyIdentities reads back what exportAEPartyIdentities writes:
// the legal identity's key from @schemeAgencyID, with @schemeAgencyName as
// the passport's country or the trade licence's description, and, for the
// supplier, the TIN key on the tax registration that is not VAT, as PINT AE
// defines IBT-032 as the seller TIN.
func importAEPartyIdentities(in *goblubl.Party, p *org.Party, supplier bool) {
	if in == nil || p == nil {
		return
	}
	if le := in.PartyLegalEntity; le != nil && le.CompanyID != nil && le.CompanyID.SchemeAgencyID != nil {
		cid := le.CompanyID
		for _, id := range p.Identities {
			if id == nil || id.Scope != org.IdentityScopeLegal || id.Code.String() != cid.Value {
				continue
			}
			importAELegalIdentity(id, cid)
			break
		}
	}
	if supplier {
		for _, id := range p.Identities {
			if id != nil && id.Scope == org.IdentityScopeTax && id.Key == "" && id.Type == "" {
				id.Key = ae.IdentityKeyTIN
			}
		}
	}
}

// importAELegalIdentity sets the identity's key and, from @schemeAgencyName,
// its issuing country or description.
func importAELegalIdentity(id *org.Identity, cid *goblubl.IDType) {
	key := pint.IdentityKeyAE(cbc.Code(strings.TrimSpace(*cid.SchemeAgencyID)))
	if key == "" {
		return
	}
	id.Key = key
	if cid.SchemeAgencyName == nil {
		return
	}
	name := strings.TrimSpace(*cid.SchemeAgencyName)
	switch key {
	case org.IdentityKeyPassport:
		id.Country = l10n.ISOCountryCode(name)
	case ae.IdentityKeyTradeLicense:
		id.Description = name
	}
}

// exportAETINScheme writes the TIN tax scheme on the party tax schemes that
// gobl.ubl writes for the party's TIN identities, which it gives the generic
// LOC scheme.
//
// No rule asks for TIN: the BIS only says the scheme of the seller TIN
// (IBT-032) must not be VAT, the syntax binding gives no code list, and
// IBR-133-AE and IBR-178-AE only check for "not VAT", so LOC would pass too.
// TIN is the value the only official example with a seller TIN uses
// (Seller-TIN-identifier.xml), so receivers are most likely to expect it.
// It is not the TIN of UNCL 1153, which is a transport instruction number.
func exportAETINScheme(out *goblubl.Party, p *org.Party) {
	for _, id := range p.Identities {
		if id == nil || id.Key != ae.IdentityKeyTIN || id.Scope != org.IdentityScopeTax {
			continue
		}
		for i := range out.PartyTaxScheme {
			pts := &out.PartyTaxScheme[i]
			if pts.CompanyID != nil && pts.CompanyID.Value == id.Code.String() &&
				pts.TaxScheme != nil && pts.TaxScheme.ID.Value != goblubl.TaxSchemeVAT {
				pts.TaxScheme.ID.Value = taxSchemeTIN
			}
		}
	}
}

// exportAETaxTotal fixes the rates of the VAT breakdowns, and merges the ones
// left with the same category and rate.
func exportAETaxTotal(tt *goblubl.TaxTotal) {
	for i := range tt.TaxSubtotal {
		c := &tt.TaxSubtotal[i].TaxCategory
		c.Percent = aeRate(c.ID, c.Percent)
	}
	tt.TaxSubtotal = mergeAETaxSubtotals(tt.TaxSubtotal)
}

// mergeAETaxSubtotals merges the VAT breakdowns with the same category and
// rate. GOBL makes a breakdown per exemption code, so two exempt lines with
// different codes give two E breakdowns, but PINT AE wants exactly one per
// category (aligned-ibrp-e-01, o-01, z-01) with the taxable amount of all its
// lines (aligned-ibrp-e-08, o-08, ae-08-ae). Exemption codes and reasons that
// differ are dropped from the merged breakdown; the line keeps its own.
func mergeAETaxSubtotals(subtotals []goblubl.TaxSubtotal) []goblubl.TaxSubtotal {
	var merged []goblubl.TaxSubtotal
	for _, st := range subtotals {
		i := indexAETaxSubtotal(merged, st.TaxCategory)
		if i < 0 {
			merged = append(merged, st)
			continue
		}
		m := &merged[i]
		m.TaxableAmount.Value = addAmounts(m.TaxableAmount.Value, st.TaxableAmount.Value)
		m.TaxAmount.Value = addAmounts(m.TaxAmount.Value, st.TaxAmount.Value)
		if !sameString(m.TaxCategory.TaxExemptionReasonCode, st.TaxCategory.TaxExemptionReasonCode) {
			m.TaxCategory.TaxExemptionReasonCode = nil
		}
		if !sameString(m.TaxCategory.TaxExemptionReason, st.TaxCategory.TaxExemptionReason) {
			m.TaxCategory.TaxExemptionReason = nil
		}
	}
	return merged
}

// indexAETaxSubtotal finds the breakdown with the same category ID, rate and
// tax scheme, or -1.
func indexAETaxSubtotal(list []goblubl.TaxSubtotal, c goblubl.TaxCategory) int {
	for i, st := range list {
		if sameID(st.TaxCategory.ID, c.ID) &&
			sameString(st.TaxCategory.Percent, c.Percent) &&
			sameScheme(st.TaxCategory.TaxScheme, c.TaxScheme) {
			return i
		}
	}
	return -1
}

func sameID(a, b *goblubl.IDType) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Value == b.Value
}

func sameString(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameScheme(a, b *goblubl.TaxScheme) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ID.Value == b.ID.Value
}

// addAmounts adds two decimal amounts, keeping the larger number of decimals.
func addAmounts(a, b string) string {
	x, err := num.AmountFromString(a)
	if err != nil {
		return a
	}
	y, err := num.AmountFromString(b)
	if err != nil {
		return a
	}
	return x.MatchPrecision(y).Add(y).String()
}
