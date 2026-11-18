package ubl

import (
	"regexp"
	"slices"
	"strings"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.peppol/addon/pint"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/icc"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/tax"
	"github.com/invopop/gobl/uuid"
)

// UNTDID 5305 VAT category codes the AE format writes rates and amounts for.
const (
	taxCategoryStandard      = "S"
	taxCategoryExempt        = "E"
	taxCategoryReverseCharge = "AE"
	taxCategoryMarginScheme  = "N"
)

// aedTotalDocumentType is the document type code PINT AE gives the additional
// document reference that carries the total with VAT in AED (BTAE-20).
const aedTotalDocumentType = "aedtotal-incl-vat"

// aeStandardRate is the rate PINT AE expects on reverse charge and margin
// scheme lines and breakdowns, and on reverse charge allowances and charges.
// GOBL keeps them without a percent, as the invoice charges no VAT, but the
// PINT AE examples and the 5% standard rate (IBR-190-AE) write 5, margin
// scheme lines need a rate (IBR-111-AE), and reverse charge allowances and
// charges one above 0 (aligned-ibrp-ae-06, aligned-ibrp-ae-07).
const aeStandardRate = "5"


// accountSchemeIBAN is the scheme PINT AE gives the ID of an IBAN account
// (IBT-084-1), as the official examples do.
const accountSchemeIBAN = "IBAN"

// deliveryTermsSchemeIncoterms is the scheme of the Incoterms code (BTAE-22),
// which the syntax binding requires and the official examples write.
const deliveryTermsSchemeIncoterms = "Incoterms"

// aePaymentMeansID identifies the invoice's payment instructions (IBT-178), so
// that its payment terms can refer to them (IBT-187). GOBL has a single set of
// instructions, so it needs no ID of its own.
const aePaymentMeansID = "1"

// exportAE applies the Peppol PINT AE rules to an exported invoice.
func exportAE(_ *goblubl.Format, env *gobl.Envelope, doc goblubl.Document) error {
	out, ok := doc.(*goblubl.Invoice)
	inv, ok2 := env.Extract().(*bill.Invoice)
	if !ok || !ok2 {
		return nil
	}
	exportAEUUID(out, inv)
	exportAEIssueTime(out, inv)
	out.Note = aeJoinNotes(out.Note)
	exportAETransactionType(out, inv)
	exportAECreditNoteReason(out, inv)
	exportAEBillingFrequency(out, inv)
	exportAEIncoterms(out, inv)
	exportAEDeliveryPartyID(out, inv)
	exportAECustomsReference(out, inv)
	exportAEContractValue(out, inv)
	exportAEPayment(out, inv)
	exportAEChargeExemptions(out, inv)
	exportAEChargeRates(out)
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
	importAEUUID(in, inv)
	importAEOutsideTaxScope(in, inv)
	importAEMarginScheme(inv)
	importAETransactionType(in, inv)
	importAECreditNoteReason(in, inv)
	importAEBillingFrequency(in, inv)
	importAEIncoterms(in, inv)
	importAEDeliveryPartyID(in, inv)
	importAECustomsReference(in, inv)
	importAEContractValue(in, inv)
	importAERemittanceScheme(in, inv)
	importAEChargeExemptions(in, inv)
	importAELineExemptions(in.InvoiceLines, inv)
	importAELineExemptions(in.CreditNoteLines, inv)
	importAELineGoods(in.InvoiceLines, inv)
	importAELineGoods(in.CreditNoteLines, inv)
	importAELineItems(in.InvoiceLines, inv)
	importAELineItems(in.CreditNoteLines, inv)
	importAELineReferences(in.InvoiceLines, inv)
	importAELineReferences(in.CreditNoteLines, inv)
	importAEPartyIdentities(in.AccountingSupplierParty.Party, inv.Supplier, true)
	importAEPartyIdentities(in.AccountingCustomerParty.Party, inv.Customer, false)
	importAEPartyEndpoint(in.AccountingSupplierParty.Party, inv.Supplier)
	importAEPartyEndpoint(in.AccountingCustomerParty.Party, inv.Customer)
	return nil
}

// importAEOutsideTaxScope reads the PINT AE out-of-scope document types, which
// gobl.ubl does not know and imports as "other": 480 becomes a standard
// invoice and 81 a credit note, both with the outside-tax-scope tag. The PINT
// scenarios then set the type code back when the document is calculated.
func importAEOutsideTaxScope(in *goblubl.Invoice, inv *bill.Invoice) {
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
	if !inv.HasTags(pint.TagOutsideTaxScope) {
		inv.Tags.List = append(inv.Tags.List, pint.TagOutsideTaxScope)
	}
}

// importAEMarginScheme reads the combos gobl.ubl imports with the N category
// and a 5% rate as exempt, with the margin-scheme tag, the way a user writes a
// margin scheme supply. Without a key, EN 16931 would read them as standard
// rated, which charges VAT the invoice doesn't. The tag normally comes from
// the transaction type, but keeps N on the lines when the flag is missing.
func importAEMarginScheme(inv *bill.Invoice) {
	var sets []tax.Set
	for _, l := range inv.Lines {
		sets = append(sets, l.Taxes)
	}
	for _, d := range inv.Discounts {
		sets = append(sets, d.Taxes)
	}
	for _, c := range inv.Charges {
		sets = append(sets, c.Taxes)
	}
	found := false
	for _, set := range sets {
		for _, c := range set {
			if c.Category == tax.CategoryVAT && c.Ext.Get(untdid.ExtKeyTaxCategory) == pint.TaxCategoryMarginScheme {
				c.Key = tax.KeyExempt
				c.Percent = nil
				found = true
			}
		}
	}
	if found && !inv.HasTags(pint.TagMarginScheme) {
		inv.Tags.List = append(inv.Tags.List, pint.TagMarginScheme)
	}
}

// exportAEUUID writes the invoice's UUID as cbc:UUID, the unique identifier
// number BTAE-07 (IBR-193-AE).
func exportAEUUID(out *goblubl.Invoice, inv *bill.Invoice) {
	if !inv.UUID.IsZero() {
		out.UUID = inv.UUID.String()
	}
}

// exportAETransactionType writes the transaction type as
// cbc:ProfileExecutionID (BTAE-02), and the beneficiary (BTAE-01) and
// principal (BTAE-14) IDs as the only identification of the buyer and seller
// parties, as the PINT AE examples do.
func exportAETransactionType(out *goblubl.Invoice, inv *bill.Invoice) {
	out.ProfileExecutionID = inv.Tax.GetExt(pint.ExtKeyTransactionType).String()
	if id := inv.Tax.GetExt(pint.ExtKeyBeneficiaryID); id != "" {
		out.BuyerCustomerParty = &goblubl.CustomerParty{Party: identifiedParty(id)}
	}
	if id := inv.Tax.GetExt(pint.ExtKeyPrincipalID); id != "" {
		out.SellerSupplierParty = &goblubl.SupplierParty{Party: identifiedParty(id)}
	}
}

// importAEUUID reads the invoice's UUID back from cbc:UUID (BTAE-07). A value
// that is not a UUID is left out.
func importAEUUID(in *goblubl.Invoice, inv *bill.Invoice) {
	if id, err := uuid.Parse(strings.TrimSpace(in.UUID)); err == nil {
		inv.UUID = id
	}
}

// exportAEIssueTime writes the invoice's issue time as cbc:IssueTime
// (IBT-168), which gobl.ubl reads but doesn't write.
func exportAEIssueTime(out *goblubl.Invoice, inv *bill.Invoice) {
	if inv.IssueTime != nil {
		out.IssueTime = inv.IssueTime.String()
	}
}

// exportAECustomsReference writes the customs reference number as
// cac:StatementDocumentReference/cbc:ID (BTAE-21).
func exportAECustomsReference(out *goblubl.Invoice, inv *bill.Invoice) {
	if ref := inv.Tax.GetExt(pint.ExtKeyCustomsReference); ref != "" {
		out.StatementDocumentReference = []goblubl.Reference{
			{ID: goblubl.IDType{Value: ref.String()}},
		}
	}
}

// importAECustomsReference reads the first customs reference number back.
func importAECustomsReference(in *goblubl.Invoice, inv *bill.Invoice) {
	for _, r := range in.StatementDocumentReference {
		if ref := cbc.Code(strings.TrimSpace(r.ID.Value)); ref != "" {
			setAEExt(inv, pint.ExtKeyCustomsReference, ref)
			return
		}
	}
}

// exportAEContractValue writes the value of each contract as
// cac:ContractDocumentReference/cbc:DocumentDescription (BTAE-05), on the
// references gobl.ubl writes in the same order as the invoice's contracts.
func exportAEContractValue(out *goblubl.Invoice, inv *bill.Invoice) {
	if inv.Ordering == nil || len(inv.Ordering.Contracts) != len(out.ContractDocumentReference) {
		return
	}
	for i, c := range inv.Ordering.Contracts {
		if c == nil {
			continue
		}
		if v := c.Ext.Get(pint.ExtKeyContractValue); v != "" {
			out.ContractDocumentReference[i].DocumentDescription = v.String()
		}
	}
}

// importAEContractValue reads the value of each contract back, which gobl.ubl
// imports as the description of the contract.
func importAEContractValue(in *goblubl.Invoice, inv *bill.Invoice) {
	if inv.Ordering == nil || len(inv.Ordering.Contracts) != len(in.ContractDocumentReference) {
		return
	}
	for i, r := range in.ContractDocumentReference {
		c := inv.Ordering.Contracts[i]
		if v := cbc.Code(strings.TrimSpace(r.DocumentDescription)); c != nil && v != "" {
			c.Ext = c.Ext.Set(pint.ExtKeyContractValue, v)
			c.Description = ""
		}
	}
}

// exportAEPayment completes the payment instructions gobl.ubl writes, which
// are the invoice's only ones, and the payment terms that go with them.
func exportAEPayment(out *goblubl.Invoice, inv *bill.Invoice) {
	if len(out.PaymentMeans) == 0 || inv.Payment == nil {
		return
	}
	pm := &out.PaymentMeans[0]
	if instr := inv.Payment.Instructions; instr != nil {
		if scheme := instr.Ext.Get(pint.ExtKeyRemittanceScheme).String(); scheme != "" && pm.PaymentID != nil {
			pm.PaymentID.SchemeID = &scheme
		}
		if len(instr.CreditTransfer) > 0 {
			exportAECreditTransfer(pm.PayeeFinancialAccount, instr.CreditTransfer[0])
		}
	}
	exportAEPaymentTerms(out, pm, inv)
}

// importAERemittanceScheme reads the scheme of the remittance reference back
// (IBT-083-1).
func importAERemittanceScheme(in *goblubl.Invoice, inv *bill.Invoice) {
	if len(in.PaymentMeans) == 0 || inv.Payment == nil || inv.Payment.Instructions == nil {
		return
	}
	id := in.PaymentMeans[0].PaymentID
	if id == nil || id.SchemeID == nil {
		return
	}
	if scheme := cbc.Code(strings.TrimSpace(*id.SchemeID)); scheme != "" {
		instr := inv.Payment.Instructions
		instr.Ext = instr.Ext.Set(pint.ExtKeyRemittanceScheme, scheme)
	}
}

// exportAECreditTransfer writes the IBAN scheme of the payee account
// (IBT-084-1) and the address of its branch (IBT-169 to IBT-175).
// gobl.ubl already reads the address back.
func exportAECreditTransfer(acc *goblubl.FinancialAccount, ct *pay.CreditTransfer) {
	if acc == nil || ct == nil {
		return
	}
	if acc.ID != nil && ct.IBAN != "" && acc.ID.Value == ct.IBAN.String() {
		scheme := accountSchemeIBAN
		acc.ID.SchemeID = &scheme
	}
	// TODO: gobl.ubl reads the branch address for every UBL format, but only
	// FormatAE writes it, so BIS, PINT and A-NZ drop it on export. Move this
	// export into gobl.ubl, reusing its address builder, and remove aeAddress.
	if addr := aeAddress(ct.Branch); addr != nil {
		if acc.FinancialInstitutionBranch == nil {
			acc.FinancialInstitutionBranch = new(goblubl.Branch)
		}
		acc.FinancialInstitutionBranch.Address = addr
	}
}

// aeAddress provides the UBL address of a GOBL one, with the lines gobl.ubl
// writes for a party's address.
func aeAddress(a *org.Address) *goblubl.PostalAddress {
	if a == nil {
		return nil
	}
	addr := new(goblubl.PostalAddress)
	set := func(field **string, v string) {
		if v != "" {
			*field = &v
		}
	}
	set(&addr.StreetName, a.LineOne())
	set(&addr.AdditionalStreetName, a.LineTwo())
	set(&addr.CityName, a.Locality)
	set(&addr.PostalZone, a.Code.String())
	set(&addr.CountrySubentity, a.Region)
	if a.Country != "" {
		addr.Country = &goblubl.Country{IdentificationCode: a.Country.String()}
	}
	return addr
}

// exportAEPaymentTerms writes the amount of the first due date and its date
// as the amount and installment due date of the payment terms (IBT-176,
// IBT-177), which refer to the payment instructions by their ID (IBT-187,
// IBT-178). gobl.ubl writes a single cac:PaymentTerms, so later due dates are
// left out, as they are from cbc:DueDate. A due date in another currency has
// no amount to write, as every amount is in the invoice's (IBR-126).
func exportAEPaymentTerms(out *goblubl.Invoice, pm *goblubl.PaymentMeans, inv *bill.Invoice) {
	terms := inv.Payment.Terms
	if terms == nil || len(terms.DueDates) == 0 {
		return
	}
	dd := terms.DueDates[0]
	def := inv.Currency.Def()
	if dd == nil || dd.Date == nil || dd.Amount == nil || def == nil ||
		(dd.Currency != "" && dd.Currency != inv.Currency) {
		return
	}
	if out.PaymentTerms == nil {
		out.PaymentTerms = new(goblubl.PaymentTerms)
	}
	id := aePaymentMeansID
	pm.ID = &id
	amount := aeAmount(def.Rescale(*dd.Amount), inv.Currency)
	out.PaymentTerms.PaymentMeansID = id
	out.PaymentTerms.Amount = &amount
	out.PaymentTerms.InstallmentDueDate = dd.Date.String()
}

// aeChargeTaxes provides the taxes of the invoice's charges and discounts, in
// the order gobl.ubl writes their cac:AllowanceCharge.
func aeChargeTaxes(inv *bill.Invoice) []tax.Set {
	sets := make([]tax.Set, 0, len(inv.Charges)+len(inv.Discounts))
	for _, c := range inv.Charges {
		sets = append(sets, c.Taxes)
	}
	for _, d := range inv.Discounts {
		sets = append(sets, d.Taxes)
	}
	return sets
}

// exportAEChargeExemptions writes the UAE VAT exemption reason of the exempt
// charges and discounts as their cac:TaxCategory/cbc:TaxExemptionReasonCode
// (IBT-196, IBT-198).
func exportAEChargeExemptions(out *goblubl.Invoice, inv *bill.Invoice) {
	sets := aeChargeTaxes(inv)
	if len(sets) != len(out.AllowanceCharge) {
		return
	}
	for i, set := range sets {
		cats := out.AllowanceCharge[i].TaxCategory
		if len(cats) != len(set) {
			continue
		}
		for j, c := range set {
			if code := c.Ext.Get(pint.ExtKeyExemption); code != "" && c.Category == tax.CategoryVAT {
				v := code.String()
				cats[j].TaxExemptionReasonCode = &v
			}
		}
	}
}

// exportAEChargeRates writes the rate of the reverse charge allowances and
// charges at 5, as PINT AE wants one above 0 (aligned-ibrp-ae-06,
// aligned-ibrp-ae-07) and gobl.ubl writes 0. Exempt ones keep the 0 PINT AE
// wants there (aligned-ibrp-e-06, aligned-ibrp-e-07), and the margin scheme
// isn't allowed on them (IBR-114-AE, IBR-115-AE).
func exportAEChargeRates(out *goblubl.Invoice) {
	for _, ac := range out.AllowanceCharge {
		for _, c := range ac.TaxCategory {
			if c != nil && c.ID != nil && c.ID.Value == taxCategoryReverseCharge {
				rate := aeStandardRate
				c.Percent = &rate
			}
		}
	}
}

// importAEChargeExemptions reads the exemption reasons of the charges and
// discounts back onto their VAT combos. gobl.ubl imports the charges and the
// discounts apart, each in the order of the document.
func importAEChargeExemptions(in *goblubl.Invoice, inv *bill.Invoice) {
	if len(in.AllowanceCharge) != len(inv.Charges)+len(inv.Discounts) {
		return
	}
	var charges, discounts int
	for _, ac := range in.AllowanceCharge {
		var set tax.Set
		if ac.ChargeIndicator {
			set = inv.Charges[charges].Taxes
			charges++
		} else {
			set = inv.Discounts[discounts].Taxes
			discounts++
		}
		c := set.Get(tax.CategoryVAT)
		if c == nil {
			continue
		}
		for _, cat := range ac.TaxCategory {
			if cat == nil || cat.TaxExemptionReasonCode == nil {
				continue
			}
			if code := cbc.Code(strings.TrimSpace(*cat.TaxExemptionReasonCode)); code != "" {
				c.Ext = c.Ext.Set(pint.ExtKeyExemption, code)
				break
			}
		}
	}
}

// identifiedParty returns a party with only a cac:PartyIdentification.
func identifiedParty(id cbc.Code) *goblubl.Party {
	return &goblubl.Party{PartyIdentification: []goblubl.Identification{
		{ID: &goblubl.IDType{Value: id.String()}},
	}}
}

// importAETransactionType reads back what exportAETransactionType writes. The
// transaction type also sets its flags' tags, so the invoice reads the same
// way a user would write it.
func importAETransactionType(in *goblubl.Invoice, inv *bill.Invoice) {
	code := cbc.Code(strings.TrimSpace(in.ProfileExecutionID))
	if code != "" {
		setAEExt(inv, pint.ExtKeyTransactionType, code)
		for _, tag := range pint.ParseTransactionType(code).Tags() {
			if !inv.HasTags(tag) {
				inv.Tags.List = append(inv.Tags.List, tag)
			}
		}
	}
	if in.BuyerCustomerParty != nil {
		setAEExt(inv, pint.ExtKeyBeneficiaryID, partyIdentificationID(in.BuyerCustomerParty.Party))
	}
	if in.SellerSupplierParty != nil {
		setAEExt(inv, pint.ExtKeyPrincipalID, partyIdentificationID(in.SellerSupplierParty.Party))
	}
}

// partyIdentificationID returns the party's first cac:PartyIdentification ID.
func partyIdentificationID(p *goblubl.Party) cbc.Code {
	if p == nil {
		return ""
	}
	for _, pi := range p.PartyIdentification {
		if pi.ID != nil && strings.TrimSpace(pi.ID.Value) != "" {
			return cbc.Code(strings.TrimSpace(pi.ID.Value))
		}
	}
	return ""
}

// setAEExt sets a tax extension on the invoice, when the value isn't empty.
func setAEExt(inv *bill.Invoice, key cbc.Key, code cbc.Code) {
	if code == "" {
		return
	}
	if inv.Tax == nil {
		inv.Tax = new(bill.Tax)
	}
	inv.Tax.Ext = inv.Tax.Ext.Set(key, code)
}

// exportAECreditNoteReason writes the credit note reason as
// cac:DiscrepancyResponse/cbc:ResponseCode (BTAE-03). UBL 2.1 only allows it
// on credit notes.
func exportAECreditNoteReason(out *goblubl.Invoice, inv *bill.Invoice) {
	reason := inv.Tax.GetExt(pint.ExtKeyCreditNoteReason)
	if reason == "" || out.CreditNoteTypeCode == nil {
		return
	}
	out.DiscrepancyResponse = []goblubl.Response{
		{ResponseCode: &goblubl.IDType{Value: reason.String()}},
	}
}

// importAECreditNoteReason reads the credit note reason back from the first
// cac:DiscrepancyResponse with a code, the only one PINT AE allows
// (ibr-198-ae).
func importAECreditNoteReason(in *goblubl.Invoice, inv *bill.Invoice) {
	for _, r := range in.DiscrepancyResponse {
		if r.ResponseCode != nil && strings.TrimSpace(r.ResponseCode.Value) != "" {
			setAEExt(inv, pint.ExtKeyCreditNoteReason, cbc.Code(strings.TrimSpace(r.ResponseCode.Value)))
			return
		}
	}
}

// exportAELineExemption writes the UAE VAT exemption reason of an exempt line
// as its cac:ClassifiedTaxCategory/cbc:TaxExemptionReasonCode (IBT-186,
// IBR-167-AE), and the text of the invoice's tax note for the line's VAT
// category as its cbc:TaxExemptionReason (IBT-185), the same note gobl.ubl
// writes on the category's VAT breakdown. Standard rated lines have no reason.
func exportAELineExemption(line *goblubl.InvoiceLine, l *bill.Line, tx *bill.Tax) {
	if l == nil || line.Item == nil || line.Item.ClassifiedTaxCategory == nil {
		return
	}
	c := l.Taxes.Get(tax.CategoryVAT)
	if c == nil {
		return
	}
	ctc := line.Item.ClassifiedTaxCategory
	if code := c.Ext.Get(pint.ExtKeyExemption); code != "" {
		v := code.String()
		ctc.TaxExemptionReasonCode = &v
	}
	if tx == nil || c.Ext.Get(untdid.ExtKeyTaxCategory) == taxCategoryStandard {
		return
	}
	if note := aeTaxNote(tx.Notes, c); note != nil {
		ctc.TaxExemptionReason = &note.Text
	}
}

// aeTaxNote finds the tax note for a combo's category, the way gobl.ubl does
// for the VAT breakdowns: by tax category and UNTDID 5305 category code.
func aeTaxNote(notes []*tax.Note, c *tax.Combo) *tax.Note {
	code := c.Ext.Get(untdid.ExtKeyTaxCategory)
	if code == "" {
		return nil
	}
	for _, n := range notes {
		if n != nil && n.Category == c.Category && n.Ext.Get(untdid.ExtKeyTaxCategory) == code {
			return n
		}
	}
	return nil
}

// importAELineExemptions reads the line exemption reasons back onto the VAT
// combos of the invoice's lines, which gobl.ubl imports in the same order. A
// reason text becomes the tax note of the line's VAT category, unless the VAT
// breakdown already gave it one.
func importAELineExemptions(lines []goblubl.InvoiceLine, inv *bill.Invoice) {
	if len(lines) == 0 || len(lines) != len(inv.Lines) {
		return
	}
	for i, line := range lines {
		c := inv.Lines[i].Taxes.Get(tax.CategoryVAT)
		if line.Item == nil || line.Item.ClassifiedTaxCategory == nil || c == nil {
			continue
		}
		ctc := line.Item.ClassifiedTaxCategory
		if ctc.TaxExemptionReasonCode != nil {
			if code := cbc.Code(strings.TrimSpace(*ctc.TaxExemptionReasonCode)); code != "" {
				c.Ext = c.Ext.Set(pint.ExtKeyExemption, code)
			}
		}
		if ctc.TaxExemptionReason == nil || strings.TrimSpace(*ctc.TaxExemptionReason) == "" {
			continue
		}
		code := c.Ext.Get(untdid.ExtKeyTaxCategory)
		if code == "" || (inv.Tax != nil && aeTaxNote(inv.Tax.Notes, c) != nil) {
			continue
		}
		inv.Tax = inv.Tax.MergeNotes(&tax.Note{
			Category: c.Category,
			Text:     strings.TrimSpace(*ctc.TaxExemptionReason),
			Ext:      tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: code}),
		})
	}
}

// exportAELineGoods writes the type of goods or services (BTAE-09) as the
// item's cac:CommodityClassification/cbc:NatureCode, and the item's GTIN as
// its standard identifier (IBR-174-AE): gobl.ubl writes the first identity
// with a scheme there, which may not be the GTIN.
func exportAELineGoods(line *goblubl.InvoiceLine, l *bill.Line) {
	if l == nil || l.Item == nil || line.Item == nil {
		return
	}
	if code := l.Item.Ext.Get(pint.ExtKeyGoodsType); code != "" {
		v := code.String()
		aeCommodityClassification(line.Item).NatureCode = &v
	}
	if id := pint.GTINIdentity(l.Item.Identities); id != nil {
		scheme := pint.IdentitySchemeGTIN.String()
		line.Item.StandardItemIdentification = &goblubl.ItemIdentification{
			ID: &goblubl.IDType{SchemeID: &scheme, Value: id.Code.String()},
		}
	}
}

// aeCommodityClassification returns the item's first commodity
// classification, adding one if it has none, where the PINT AE item codes go
// alongside the item classification code, as in the official examples.
func aeCommodityClassification(it *goblubl.Item) *goblubl.CommodityClassification {
	if it.CommodityClassification == nil || len(*it.CommodityClassification) == 0 {
		it.CommodityClassification = &[]goblubl.CommodityClassification{{}}
	}
	return &(*it.CommodityClassification)[0]
}

// exportAELineSAC moves the service accounting codes (BTAE-17), which
// gobl.ubl writes as item classifications listed "SAC" since their
// identities have that label, to cac:AdditionalItemIdentification with the
// SAC scheme (IBR-189-AE).
func exportAELineSAC(line *goblubl.InvoiceLine) {
	if line.Item == nil || line.Item.CommodityClassification == nil {
		return
	}
	var kept []goblubl.CommodityClassification
	for _, cc := range *line.Item.CommodityClassification {
		code := cc.ItemClassificationCode
		if code == nil || code.ListID == nil || *code.ListID != pint.IdentityLabelSAC {
			kept = append(kept, cc)
			continue
		}
		scheme := pint.IdentityLabelSAC
		line.Item.AdditionalItemIdentification = append(line.Item.AdditionalItemIdentification,
			goblubl.ItemIdentification{ID: &goblubl.IDType{SchemeID: &scheme, Value: code.Value}})
	}
	if len(kept) == 0 {
		line.Item.CommodityClassification = nil
		return
	}
	line.Item.CommodityClassification = &kept
}

// exportAELineItemType writes the item type (BTAE-13) of the item's key as
// its cac:CommodityClassification/cbc:CommodityCode.
func exportAELineItemType(line *goblubl.InvoiceLine, l *bill.Line) {
	if l == nil || l.Item == nil || line.Item == nil {
		return
	}
	if typ := pint.ItemTypeAE(l.Item.Key); typ != "" {
		v := typ.String()
		aeCommodityClassification(line.Item).CommodityCode = &v
	}
}

// exportAELineSchemeVersions writes the scheme version of the item's
// identities that have one: as @listVersionID on their item classification
// code (IBT-158-2), or as @schemeVersionID on their additional item
// identification (BTAE-17-2), which is where the service accounting codes are
// by now.
func exportAELineSchemeVersions(line *goblubl.InvoiceLine, l *bill.Line) {
	if l == nil || l.Item == nil || line.Item == nil {
		return
	}
	for _, id := range l.Item.Identities {
		if id == nil || id.Label == "" || id.Ext.Get(pint.ExtKeySchemeVersion) == "" {
			continue
		}
		version := id.Ext.Get(pint.ExtKeySchemeVersion).String()
		if line.Item.CommodityClassification != nil {
			for _, cc := range *line.Item.CommodityClassification {
				if c := cc.ItemClassificationCode; c != nil && c.ListID != nil && *c.ListID == id.Label && c.Value == id.Code.String() {
					c.ListVersionID = &version
				}
			}
		}
		for _, ai := range line.Item.AdditionalItemIdentification {
			if ai.ID != nil && ai.ID.SchemeID != nil && *ai.ID.SchemeID == id.Label && ai.ID.Value == id.Code.String() {
				ai.ID.SchemeVersionID = &version
			}
		}
	}
}

// exportAELineBatch writes the item's batch number as its
// cac:ItemInstance/cac:LotIdentification/cbc:LotNumberID (BTAE-24).
func exportAELineBatch(line *goblubl.InvoiceLine, l *bill.Line) {
	if l == nil || l.Item == nil || line.Item == nil {
		return
	}
	if batch := l.Item.Ext.Get(pint.ExtKeyBatchNumber); batch != "" {
		line.Item.ItemInstance = []goblubl.ItemInstance{
			{LotIdentification: &goblubl.LotIdentification{LotNumberID: batch.String()}},
		}
	}
}

// aeNoLineID stands for the line of an order or despatch advice when only
// the document is known, as UBL needs a line ID in a line reference.
const aeNoLineID = "NA"

// exportAELineReferences writes the purchase order of a line as
// cac:OrderLineReference/cac:OrderReference/cbc:ID (IBT-183), next to the
// order line gobl.ubl writes, and its despatch advice as
// cac:DespatchLineReference/cac:DocumentReference/cbc:ID (IBT-184).
func exportAELineReferences(line *goblubl.InvoiceLine, l *bill.Line) {
	if l == nil {
		return
	}
	if po := l.Ext.Get(pint.ExtKeyPurchaseOrder); po != "" {
		if line.OrderLineReference == nil {
			line.OrderLineReference = &goblubl.OrderLineReference{LineID: aeNoLineID}
		}
		line.OrderLineReference.OrderReference = &goblubl.OrderReference{ID: po.String()}
	}
	if da := l.Ext.Get(pint.ExtKeyDespatchAdvice); da != "" {
		line.DespatchLineReference = &goblubl.LineReference{
			LineID:            aeNoLineID,
			DocumentReference: &goblubl.Reference{ID: goblubl.IDType{Value: da.String()}},
		}
	}
}

// importAELineReferences reads the purchase order and despatch advice of each
// line back, on the lines gobl.ubl imports in the same order. An order line
// that only stands in for a missing one is dropped.
func importAELineReferences(lines []goblubl.InvoiceLine, inv *bill.Invoice) {
	if len(lines) != len(inv.Lines) {
		return
	}
	for i, line := range lines {
		l := inv.Lines[i]
		if ref := line.OrderLineReference; ref != nil && ref.OrderReference != nil {
			if po := cbc.Code(strings.TrimSpace(ref.OrderReference.ID)); po != "" {
				l.Ext = l.Ext.Set(pint.ExtKeyPurchaseOrder, po)
				if l.Order == aeNoLineID {
					l.Order = ""
				}
			}
		}
		if ref := line.DespatchLineReference; ref != nil && ref.DocumentReference != nil {
			if da := cbc.Code(strings.TrimSpace(ref.DocumentReference.ID.Value)); da != "" {
				l.Ext = l.Ext.Set(pint.ExtKeyDespatchAdvice, da)
			}
		}
	}
}

// importAELineGoods reads the type of goods or services and the batch number
// back onto the items of the invoice's lines, which gobl.ubl imports in the
// same order. gobl.ubl already reads the GTIN as an identity.
func importAELineGoods(lines []goblubl.InvoiceLine, inv *bill.Invoice) {
	if len(lines) == 0 || len(lines) != len(inv.Lines) {
		return
	}
	for i, line := range lines {
		if line.Item == nil || inv.Lines[i].Item == nil {
			continue
		}
		for _, ii := range line.Item.ItemInstance {
			if ii.LotIdentification == nil {
				continue
			}
			if batch := cbc.Code(strings.TrimSpace(ii.LotIdentification.LotNumberID)); batch != "" {
				inv.Lines[i].Item.Ext = inv.Lines[i].Item.Ext.Set(pint.ExtKeyBatchNumber, batch)
				break
			}
		}
		if line.Item.CommodityClassification == nil {
			continue
		}
		for _, cc := range *line.Item.CommodityClassification {
			if cc.NatureCode == nil {
				continue
			}
			if code := cbc.Code(strings.TrimSpace(*cc.NatureCode)); code != "" {
				inv.Lines[i].Item.Ext = inv.Lines[i].Item.Ext.Set(pint.ExtKeyGoodsType, code)
				break
			}
		}
	}
}

// importAELineItems reads back what exportAELineSAC, exportAELineItemType and
// exportAELineSchemeVersions write: the service accounting codes as
// identities with their scheme as label, the item type as the item's key, and
// the scheme versions on the identities of their codes.
func importAELineItems(lines []goblubl.InvoiceLine, inv *bill.Invoice) {
	if len(lines) == 0 || len(lines) != len(inv.Lines) {
		return
	}
	for i, line := range lines {
		it := inv.Lines[i].Item
		if line.Item == nil || it == nil {
			continue
		}
		for _, ai := range line.Item.AdditionalItemIdentification {
			if ai.ID == nil || strings.TrimSpace(ai.ID.Value) == "" {
				continue
			}
			id := &org.Identity{Code: cbc.Code(strings.TrimSpace(ai.ID.Value))}
			if ai.ID.SchemeID != nil {
				id.Label = strings.TrimSpace(*ai.ID.SchemeID)
			}
			setAESchemeVersion(id, ai.ID.SchemeVersionID)
			it.Identities = append(it.Identities, id)
		}
		if line.Item.CommodityClassification == nil {
			continue
		}
		for _, cc := range *line.Item.CommodityClassification {
			c := cc.ItemClassificationCode
			if c == nil || c.ListID == nil {
				continue
			}
			for _, id := range it.Identities {
				if id != nil && id.Label == strings.TrimSpace(*c.ListID) && id.Code.String() == c.Value {
					setAESchemeVersion(id, c.ListVersionID)
				}
			}
		}
		for _, cc := range *line.Item.CommodityClassification {
			if cc.CommodityCode == nil {
				continue
			}
			if key := pint.ItemKeyAE(cbc.Code(strings.TrimSpace(*cc.CommodityCode))); key != "" {
				it.Key = key
				break
			}
		}
	}
}

// setAESchemeVersion sets the scheme version of an item identity, when there
// is one.
func setAESchemeVersion(id *org.Identity, version *string) {
	if version == nil || strings.TrimSpace(*version) == "" {
		return
	}
	id.Ext = id.Ext.Set(pint.ExtKeySchemeVersion, cbc.Code(strings.TrimSpace(*version)))
}

// exportAEBillingFrequency writes the billing frequency as
// cac:InvoicePeriod/cbc:DescriptionCode (IBT-008), on the period gobl.ubl
// writes from the ordering period, or on its own: IBR-CO-19 accepts a period
// with only a description code.
func exportAEBillingFrequency(out *goblubl.Invoice, inv *bill.Invoice) {
	freq := inv.Tax.GetExt(pint.ExtKeyBillingFrequency)
	if freq == "" {
		return
	}
	if len(out.InvoicePeriod) == 0 {
		out.InvoicePeriod = []goblubl.Period{{}}
	}
	out.InvoicePeriod[0].DescriptionCode = freq.String()
}

// importAEBillingFrequency reads the billing frequency back. The frequency
// codes don't overlap the VAT point date codes gobl.ubl reads from the same
// element, so it never sets a tax point from them.
func importAEBillingFrequency(in *goblubl.Invoice, inv *bill.Invoice) {
	if len(in.InvoicePeriod) == 0 {
		return
	}
	setAEExt(inv, pint.ExtKeyBillingFrequency, cbc.Code(strings.TrimSpace(in.InvoicePeriod[0].DescriptionCode)))
}

// exportAEIncoterms writes the delivery's Incoterms rule code, from the ICC
// Incoterms 2020 or 2010 extension, as cac:Delivery/cac:DeliveryTerms/cbc:ID
// (BTAE-22) under the Incoterms scheme.
func exportAEIncoterms(out *goblubl.Invoice, inv *bill.Invoice) {
	if inv.Delivery == nil {
		return
	}
	code := inv.Delivery.Ext.Get(icc.ExtKeyIncoterms2020)
	if code == "" {
		code = inv.Delivery.Ext.Get(icc.ExtKeyIncoterms2010)
	}
	if code == "" {
		return
	}
	if len(out.Delivery) == 0 {
		out.Delivery = []*goblubl.Delivery{{}}
	}
	scheme := deliveryTermsSchemeIncoterms
	out.Delivery[0].DeliveryTerms = []goblubl.DeliveryTerms{
		{ID: goblubl.IDType{SchemeID: &scheme, Value: code.String()}},
	}
}

// importAEIncoterms reads the delivery terms back as the ICC Incoterms 2020
// extension, the current edition, as PINT AE doesn't say which one a code is
// from. A code only the 2010 edition has is read as that one.
func importAEIncoterms(in *goblubl.Invoice, inv *bill.Invoice) {
	if len(in.Delivery) == 0 || in.Delivery[0] == nil || len(in.Delivery[0].DeliveryTerms) == 0 {
		return
	}
	code := cbc.Code(strings.TrimSpace(in.Delivery[0].DeliveryTerms[0].ID.Value))
	if code == "" {
		return
	}
	if inv.Delivery == nil {
		inv.Delivery = new(bill.DeliveryDetails)
	}
	key := icc.ExtKeyIncoterms2020
	if !extensionHasCode(key, code) && extensionHasCode(icc.ExtKeyIncoterms2010, code) {
		key = icc.ExtKeyIncoterms2010
	}
	inv.Delivery.Ext = inv.Delivery.Ext.Set(key, code)
}

// exportAEDeliveryPartyID writes the delivery receiver's UAE TRN, or its TIN
// when it has no TRN, as cac:DeliveryParty/cac:PartyIdentification/cbc:ID:
// the Deliver to Party ID (BTAE-23) the implementation guide (section 2.3.11)
// asks for when the goods go to someone other than the buyer. The PINT AE
// syntax has the element, the generic PINT one doesn't, but neither the
// syntax nor the semantic model give it a business term, and no rule checks
// it. gobl.ubl writes the delivery party with only its name and contact.
func exportAEDeliveryPartyID(out *goblubl.Invoice, inv *bill.Invoice) {
	if inv.Delivery == nil {
		return
	}
	id := aeReceiverID(inv.Delivery.Receiver)
	if id == "" {
		return
	}
	if len(out.Delivery) == 0 {
		out.Delivery = []*goblubl.Delivery{{}}
	}
	d := out.Delivery[0]
	if d.DeliveryParty == nil {
		d.DeliveryParty = new(goblubl.Party)
	}
	d.DeliveryParty.PartyIdentification = []goblubl.Identification{
		{ID: &goblubl.IDType{Value: id.String()}},
	}
}

// aeReceiverID provides the receiver's UAE TRN, or else its TIN.
func aeReceiverID(p *org.Party) cbc.Code {
	if p == nil {
		return ""
	}
	if p.TaxID != nil && p.TaxID.Country.Code() == l10n.AE && p.TaxID.Code != "" {
		return p.TaxID.Code
	}
	for _, id := range p.Identities {
		if id != nil && id.Type == ae.IdentityTypeTIN && id.Code != "" {
			return id.Code
		}
	}
	return ""
}

// importAEDeliveryPartyID reads back what exportAEDeliveryPartyID writes: a
// TRN, 15 digits starting with 1, as the receiver's UAE tax ID, and anything
// else as a TIN identity, the only other ID the implementation guide allows.
func importAEDeliveryPartyID(in *goblubl.Invoice, inv *bill.Invoice) {
	if len(in.Delivery) == 0 || in.Delivery[0] == nil {
		return
	}
	code := partyIdentificationID(in.Delivery[0].DeliveryParty)
	if code == "" {
		return
	}
	if inv.Delivery == nil {
		inv.Delivery = new(bill.DeliveryDetails)
	}
	if inv.Delivery.Receiver == nil {
		inv.Delivery.Receiver = new(org.Party)
	}
	r := inv.Delivery.Receiver
	if trnRegexp.MatchString(code.String()) {
		r.TaxID = &tax.Identity{Country: l10n.AE.Tax(), Code: code}
		return
	}
	r.Identities = append(r.Identities, &org.Identity{Type: ae.IdentityTypeTIN, Code: code})
}

// trnRegexp matches a UAE TRN.
var trnRegexp = regexp.MustCompile(ae.TRNPattern)

// extensionHasCode reports whether the extension lists the code.
func extensionHasCode(key cbc.Key, code cbc.Code) bool {
	def := tax.ExtensionForKey(key)
	return def != nil && def.HasCode(code)
}

// exportAELines completes the UBL lines, which gobl.ubl writes in the same
// order as the invoice's lines.
func exportAELines(lines []goblubl.InvoiceLine, inv *bill.Invoice) {
	var rate *currency.ExchangeRate
	if inv.Currency != currency.AED {
		rate = currency.MatchExchangeRate(inv.ExchangeRates, inv.Currency, currency.AED)
	}
	for i := range lines {
		exportAELine(&lines[i])
		if len(lines) == len(inv.Lines) {
			exportAELineAmounts(&lines[i], inv.Lines[i], inv.Currency, rate)
			exportAELineExemption(&lines[i], inv.Lines[i], inv.Tax)
			exportAELineSAC(&lines[i])
			exportAELineGoods(&lines[i], inv.Lines[i])
			exportAELineItemType(&lines[i], inv.Lines[i])
			exportAELineSchemeVersions(&lines[i], inv.Lines[i])
			exportAELineBatch(&lines[i], inv.Lines[i])
			exportAELineReferences(&lines[i], inv.Lines[i])
		}
	}
}

// aeJoinNotes joins the notes into one, separated by a blank line, as PINT
// allows a single note on the document and on each line (ibr-sr-51,
// ibr-sr-34). A joined note is imported back as a single note.
//
// TODO: these are shared PINT rules, and EN 16931 has the line one too
// (UBL-SR-34), so FormatPINT and FormatAUNZ need the same, and FormatBIS on
// its lines.
func aeJoinNotes(notes []string) []string {
	if len(notes) < 2 {
		return notes
	}
	return []string{strings.Join(notes, "\n\n")}
}

// exportAELine completes a line's notes, price details and VAT rate.
func exportAELine(line *goblubl.InvoiceLine) {
	line.Note = aeJoinNotes(line.Note)
	exportAEPrice(line)
	if line.Item != nil && line.Item.ClassifiedTaxCategory != nil {
		c := line.Item.ClassifiedTaxCategory
		c.Percent = aeRate(c.ID, c.Percent)
	}
}

// exportAELineAmounts writes the line amount payable (BTAE-10) and the line VAT
// amount (BTAE-08) in cac:ItemPriceExtension, which every line needs
// (IBR-194-AE, IBR-104-AE). Both are in AED, as their names and IBR-104-AE
// say, so for invoices in another currency they are converted with the rate
// to AED. The implementation guide (section 3.1.1) asks for AED whatever the
// invoice currency, and IBR-126 leaves cac:ItemPriceExtension out of the
// document currency check as amounts in the tax currency, which IBR-140-AE
// fixes to AED. No test checks their currency, and the official exports in
// USD write them in AED too. Without a rate, which the PINT addon requires
// (GOBL-PEPPOL-PINT-BILL-INVOICE-28), they stay in the document currency.
//
// GOBL keeps no VAT per line, so it is the line total at the line's rate,
// rounded to the document currency, then converted. Exempt lines get no VAT
// amount (IBR-163-AE), and zero rated, reverse charge and margin scheme lines
// a zero one (IBR-165-AE, IBR-162-AE, as the official margin scheme example).
func exportAELineAmounts(line *goblubl.InvoiceLine, l *bill.Line, cur currency.Code, rate *currency.ExchangeRate) {
	if l == nil || l.Total == nil {
		return
	}
	def := cur.Def()
	if def == nil {
		return
	}
	out := cur
	convert := func(a num.Amount) num.Amount { return a }
	if rate != nil {
		out = currency.AED
		convert = rate.Convert
	}
	total := convert(def.Rescale(*l.Total))
	ext := &goblubl.ItemPriceExtension{Amount: aeAmount(total, out)}
	if vat, ok := aeLineVAT(line, l); ok {
		vat = convert(def.Rescale(vat))
		ext.Amount = aeAmount(total.Add(vat), out)
		ext.TaxTotal = []goblubl.TaxTotal{{TaxAmount: aeAmount(vat, out)}}
	}
	line.ItemPriceExtension = ext
}

// aeLineVAT provides the VAT charged on a line, and false for exempt lines,
// which must not have one. Only standard rated lines charge VAT: zero rated,
// reverse charge, margin scheme and outside scope lines charge none.
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
// rate (GOBL-PEPPOL-PINT-BILL-INVOICE-28) with at most 6 decimals (-29).
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
			DocumentDescription: total.String(),
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
// a zero discount, written with the decimals of the price as gobl.ubl does for
// a list price. The base quantity also lets IBR-147-AE compute the line net
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
		net, _ := num.AmountFromString(gross.Value)
		price.AllowanceCharge = &goblubl.AllowanceCharge{
			ChargeIndicator: false,
			Amount:          goblubl.Amount{CurrencyID: gross.CurrencyID, Value: num.AmountZero.MatchPrecision(net).String()},
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
// IBR-121-AE), and reverse charge and margin scheme must have one
// (aligned-ibrp-ae-05-ae, IBR-119-AE, IBR-111-AE). Document allowances and
// charges are set by exportAEChargeRates.
func aeRate(id *goblubl.IDType, percent *string) *string {
	if id == nil {
		return percent
	}
	switch id.Value {
	case taxCategoryExempt:
		return nil
	case taxCategoryReverseCharge, taxCategoryMarginScheme:
		rate := aeStandardRate
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
	exportAETINIdentification(out)
}

// identitySchemeTIN is the ISO 6523 ICD of the UAE TIN, the scheme of UAE
// Peppol endpoints too.
const identitySchemeTIN = "0235"

// exportAETINIdentification writes the scheme of a TIN identity in
// cac:PartyIdentification as its ISO 6523 ICD, 0235. gobl.ubl writes the
// identity's type, TIN, which is not an ICD (ibr-cl-10).
func exportAETINIdentification(out *goblubl.Party) {
	for i := range out.PartyIdentification {
		id := out.PartyIdentification[i].ID
		if id != nil && id.SchemeID != nil && *id.SchemeID == ae.IdentityTypeTIN.String() {
			scheme := identitySchemeTIN
			id.SchemeID = &scheme
		}
	}
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
	typ := pint.IdentityTypeAE(id)
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
	case ae.IdentityTypeTradeLicense:
		if id.Description != "" {
			name := id.Description
			cid.SchemeAgencyName = &name
		}
	}
}

// importAEPartyIdentities reads back what exportAEPartyIdentities writes:
// the legal identity's key or type from @schemeAgencyID, with
// @schemeAgencyName as the passport's country or the trade licence's
// description, and a TIN identification from its 0235 scheme. For the
// supplier, it also reads every tax registration that
// is not VAT as a TIN, as PINT AE defines IBT-032 as the seller TIN.
//
// TODO: the BIS gives the seller TIN's scheme no code list, and the official
// examples use TIN and TAX. A scheme like TAX is read as a TIN, and written
// back as TIN.
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
	// A cac:PartyIdentification with the scheme 0235 is a TIN, which export
	// writes with that scheme.
	for _, id := range p.Identities {
		if id != nil && id.Scope == "" && id.Ext.Get(iso.ExtKeySchemeID) == identitySchemeTIN {
			id.Type = ae.IdentityTypeTIN
			id.Ext = id.Ext.Delete(iso.ExtKeySchemeID)
		}
	}
	if !supplier {
		return
	}
	for _, pts := range in.PartyTaxScheme {
		if pts.CompanyID == nil || pts.TaxScheme == nil ||
			strings.EqualFold(strings.TrimSpace(pts.TaxScheme.ID.Value), goblubl.TaxSchemeVAT) {
			continue
		}
		for _, id := range p.Identities {
			if id != nil && id.Scope == org.IdentityScopeTax && id.Code.String() == pts.CompanyID.Value {
				id.Type = ae.IdentityTypeTIN
				break
			}
		}
	}
}

// importAEPartyEndpoint reads the party's electronic address, cbc:EndpointID,
// as an endpoint, which the PINT addon requires on the supplier and customer
// (IBR-080, IBR-081). gobl.ubl imports it as an inbox, which the endpoint
// replaces.
func importAEPartyEndpoint(in *goblubl.Party, p *org.Party) {
	if in == nil || p == nil || in.EndpointID == nil || in.EndpointID.Value == "" {
		return
	}
	scheme, value := in.EndpointID.SchemeID, in.EndpointID.Value
	uri := cbc.URI(iso.ActorIDScheme + "::" + scheme + ":" + value)
	if scheme == goblubl.SchemeIDEmail {
		uri = cbc.URI("mailto:" + value)
	}
	p.Inboxes = slices.DeleteFunc(p.Inboxes, func(ib *org.Inbox) bool {
		return ib.Email == value || (ib.Scheme.String() == scheme && ib.Code.String() == value)
	})
	p.Endpoints = append(p.Endpoints, &org.Endpoint{URI: uri})
}

// importAELegalIdentity sets the identity's key or type and, from
// @schemeAgencyName, its issuing country or description.
func importAELegalIdentity(id *org.Identity, cid *goblubl.IDType) {
	if !pint.SetIdentityTypeAE(id, cbc.Code(strings.TrimSpace(*cid.SchemeAgencyID))) {
		return
	}
	if cid.SchemeAgencyName == nil {
		return
	}
	name := strings.TrimSpace(*cid.SchemeAgencyName)
	switch {
	case id.Key == org.IdentityKeyPassport:
		id.Country = l10n.ISOCountryCode(name)
	case id.Type == ae.IdentityTypeTradeLicense:
		id.Description = name
	}
}

// exportAETaxTotal fixes the rates of the VAT breakdowns, and merges the ones
// left with the same category and rate. The total with breakdowns, which is
// the one in the document currency, also says its prices don't include VAT
// (IBT-200).
func exportAETaxTotal(tt *goblubl.TaxTotal) {
	for i := range tt.TaxSubtotal {
		c := &tt.TaxSubtotal[i].TaxCategory
		c.Percent = aeRate(c.ID, c.Percent)
	}
	tt.TaxSubtotal = mergeAETaxSubtotals(tt.TaxSubtotal)
	if len(tt.TaxSubtotal) > 0 {
		// gobl.ubl takes the taxes out of prices that include them.
		included := false
		tt.TaxIncludedIndicator = &included
	}
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
			samePercent(st.TaxCategory.Percent, c.Percent) &&
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

// samePercent compares two rates by value, so that "0" and "0.0" are the
// same rate and their breakdowns are merged (ALIGNED-IBRP-Z-01 and the other
// one-breakdown-per-category rules).
func samePercent(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	x, errA := num.AmountFromString(*a)
	y, errB := num.AmountFromString(*b)
	if errA != nil || errB != nil {
		return *a == *b
	}
	return x.Equals(y)
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
