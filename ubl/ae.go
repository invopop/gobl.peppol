package ubl

import (
	"regexp"
	"slices"
	"strings"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.peppol/addon/pint"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/cef"
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

// accountSchemeIBAN is the scheme PINT AE gives the ID of an IBAN account
// (IBT-084-1), as the official examples do.
const accountSchemeIBAN = "IBAN"

// deliveryTermsSchemeIncoterms is the scheme of the Incoterms code (BTAE-22),
// which the syntax binding requires and the official examples write.
const deliveryTermsSchemeIncoterms = "Incoterms"

// exportAE applies the Peppol PINT AE rules to an exported invoice.
func exportAE(_ *goblubl.Format, env *gobl.Envelope, doc goblubl.Document) error {
	out, ok := doc.(*goblubl.Invoice)
	inv, ok2 := env.Extract().(*bill.Invoice)
	if !ok || !ok2 {
		return nil
	}
	exportAEUUID(out, inv)
	exportAEIssueTime(out, inv)
	exportAETransactionType(out, inv)
	exportAECreditNoteReason(out, inv)
	exportAEIncoterms(out, inv)
	exportAEDeliveryPartyID(out, inv)
	exportAEContractValue(out, inv)
	exportAEPayment(out, inv)
	exportAEChargeRates(out, aeStandardRate(inv))
	exportAELines(out.InvoiceLines, inv)
	exportAELines(out.CreditNoteLines, inv)
	for _, p := range goblubl.InvoiceParties(inv, out) {
		exportAEPartyTaxID(p.UBL, p.GOBL.TaxID)
	}
	exportAEPartyIdentities(out.AccountingSupplierParty.Party, inv.Supplier)
	exportAEPartyIdentities(out.AccountingCustomerParty.Party, inv.Customer)
	for i := range out.TaxTotal {
		exportAETaxTotal(&out.TaxTotal[i], aeStandardRate(inv))
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
	importAEIncoterms(in, inv)
	importAEDeliveryPartyID(in, inv)
	importAEChargeExemptions(in, inv)
	importAELineExemptions(in.InvoiceLines, inv)
	importAELineExemptions(in.CreditNoteLines, inv)
	importAELineGoods(in.InvoiceLines, inv)
	importAELineGoods(in.CreditNoteLines, inv)
	importAELineItems(in.InvoiceLines, inv)
	importAELineItems(in.CreditNoteLines, inv)
	importAEPartyIdentities(in.AccountingSupplierParty.Party, inv.Supplier, true)
	importAEPartyIdentities(in.AccountingCustomerParty.Party, inv.Customer, false)
	importAEPartyEndpoint(in.AccountingSupplierParty.Party, inv.Supplier)
	importAEPartyEndpoint(in.AccountingCustomerParty.Party, inv.Customer)
	return nil
}

// importAEOutsideTaxScope reads 480 and 81, which gobl.ubl imports as "other", as a standard invoice
// and a credit note with the outside-tax-scope tag.
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

// importAEMarginScheme reads N VAT combos as exempt with the margin-scheme tag, as a user writes them;
// without a key EN 16931 would make them standard rated.
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
// number (BTAE-07).
func exportAEUUID(out *goblubl.Invoice, inv *bill.Invoice) {
	if !inv.UUID.IsZero() {
		out.UUID = inv.UUID.String()
	}
}

// exportAETransactionType writes the transaction type (BTAE-02), and the beneficiary (BTAE-01) and
// principal (BTAE-14) IDs as the buyer and seller party identifications.
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

// exportAEContractValue writes each contract's description, PINT AE's contract value (BTAE-05), as its
// cac:ContractDocumentReference/cbc:DocumentDescription.
func exportAEContractValue(out *goblubl.Invoice, inv *bill.Invoice) {
	if inv.Ordering == nil || len(inv.Ordering.Contracts) != len(out.ContractDocumentReference) {
		return
	}
	for i, c := range inv.Ordering.Contracts {
		if c == nil {
			continue
		}
		out.ContractDocumentReference[i].DocumentDescription = c.Description
	}
}

// exportAEPayment completes the payee account gobl.ubl writes from the first credit transfer.
func exportAEPayment(out *goblubl.Invoice, inv *bill.Invoice) {
	if len(out.PaymentMeans) == 0 || inv.Payment == nil {
		return
	}
	if instr := inv.Payment.Instructions; instr != nil && len(instr.CreditTransfer) > 0 {
		exportAECreditTransfer(out.PaymentMeans[0].PayeeFinancialAccount, instr.CreditTransfer[0])
	}
}

// exportAECreditTransfer writes the IBAN scheme of the payee account (IBT-084-1) and the address of
// its branch (IBT-169 to IBT-175), which gobl.ubl already reads back.
func exportAECreditTransfer(acc *goblubl.FinancialAccount, ct *pay.CreditTransfer) {
	if acc == nil || ct == nil {
		return
	}
	if acc.ID != nil && ct.IBAN != "" && acc.ID.Value == ct.IBAN.String() {
		scheme := accountSchemeIBAN
		acc.ID.SchemeID = &scheme
	}
	// TODO: FormatPINT should write the branch address too (IBG-34); gobl.ubl can't (UBL-CR-432).
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

// exportAEChargeRates writes the standard rate on reverse charge allowances and charges, where gobl.ubl
// writes 0 and PINT AE wants one above 0.
func exportAEChargeRates(out *goblubl.Invoice, standard string) {
	for _, ac := range out.AllowanceCharge {
		for _, c := range ac.TaxCategory {
			if c != nil && c.ID != nil && c.ID.Value == taxCategoryReverseCharge {
				rate := standard
				c.Percent = &rate
			}
		}
	}
}

// importAEChargeExemptions reads charge and discount exemption reasons from their own cac:TaxCategory;
// gobl.ubl takes them from the breakdown, which keeps only shared ones.
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
				c.Ext = c.Ext.Set(cef.ExtKeyVATEX, code)
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

// importAETransactionType reads back what exportAETransactionType writes, and adds the tags of the
// transaction type's flags.
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
// cac:DiscrepancyResponse with a code, the only one PINT AE allows.
func importAECreditNoteReason(in *goblubl.Invoice, inv *bill.Invoice) {
	for _, r := range in.DiscrepancyResponse {
		if r.ResponseCode != nil && strings.TrimSpace(r.ResponseCode.Value) != "" {
			setAEExt(inv, pint.ExtKeyCreditNoteReason, cbc.Code(strings.TrimSpace(r.ResponseCode.Value)))
			return
		}
	}
}

// exportAELineExemption writes the line's VATEX code (IBT-186) and, except on standard rated lines,
// its category's tax note as the reason (IBT-185).
func exportAELineExemption(line *goblubl.InvoiceLine, l *bill.Line, tx *bill.Tax) {
	if l == nil || line.Item == nil || line.Item.ClassifiedTaxCategory == nil {
		return
	}
	c := l.Taxes.Get(tax.CategoryVAT)
	if c == nil {
		return
	}
	ctc := line.Item.ClassifiedTaxCategory
	if code := c.Ext.Get(cef.ExtKeyVATEX); code != "" {
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

// importAELineExemptions reads the line VATEX codes back, and a reason text as the category's tax
// note when the breakdown gave none.
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
				c.Ext = c.Ext.Set(cef.ExtKeyVATEX, code)
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

// exportAELineGoods writes the goods type (BTAE-09) as cbc:NatureCode and the GTIN as the standard
// identifier, where gobl.ubl may write another identity.
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

// aeCommodityClassification returns the item's first commodity classification, adding one if it has
// none. The PINT AE item codes go there with the item classification code, as in the official examples.
func aeCommodityClassification(it *goblubl.Item) *goblubl.CommodityClassification {
	if it.CommodityClassification == nil || len(*it.CommodityClassification) == 0 {
		it.CommodityClassification = &[]goblubl.CommodityClassification{{}}
	}
	return &(*it.CommodityClassification)[0]
}

// exportAELineSAC moves the SAC item classifications gobl.ubl writes to cac:AdditionalItemIdentification
// with the SAC scheme (BTAE-17).
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

// importAELineGoods reads the goods type back onto the line items. gobl.ubl already reads the GTIN.
func importAELineGoods(lines []goblubl.InvoiceLine, inv *bill.Invoice) {
	if len(lines) == 0 || len(lines) != len(inv.Lines) {
		return
	}
	for i, line := range lines {
		if line.Item == nil || inv.Lines[i].Item == nil {
			continue
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

// importAELineItems reads back what exportAELineSAC and exportAELineItemType
// write: the service accounting codes as identities with their scheme as
// label, and the item type as the item's key.
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
			it.Identities = append(it.Identities, id)
		}
		if line.Item.CommodityClassification == nil {
			continue
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

// exportAEDeliveryPartyID writes the receiver's UAE TRN, or else TIN, as the Deliver to Party ID
// (BTAE-23, implementation guide 2.3.11).
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
	standard := aeStandardRate(inv)
	for i := range lines {
		exportAELine(&lines[i], standard)
		if len(lines) == len(inv.Lines) {
			exportAELineAmounts(&lines[i], inv.Lines[i], inv.Currency, rate)
			exportAELineExemption(&lines[i], inv.Lines[i], inv.Tax)
			exportAELineSAC(&lines[i])
			exportAELineGoods(&lines[i], inv.Lines[i])
			exportAELineItemType(&lines[i], inv.Lines[i])
		}
	}
}

// exportAELine completes a line's price details and VAT rate.
func exportAELine(line *goblubl.InvoiceLine, standard string) {
	exportAEPrice(line)
	if line.Item != nil && line.Item.ClassifiedTaxCategory != nil {
		c := line.Item.ClassifiedTaxCategory
		c.Percent = aeRate(c.ID, c.Percent, standard)
	}
}

// exportAELineAmounts writes the line amount payable (BTAE-10) and VAT (BTAE-08), in AED as the
// implementation guide (3.1.1) asks. GOBL keeps no VAT per line, so it is the line total at the line's rate.
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

// aeLineVAT provides the VAT charged on a line, which only standard rated lines have, and false for
// exempt lines, which must not have one.
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

// exportAEExchangeRate writes, for invoices not in AED, the rate to AED (BTAE-04) as cac:TaxExchangeRate
// and the total with VAT in AED (BTAE-20).
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

// exportAEPrice adds the base quantity 1 and the gross price PINT AE requires on every line,
// with a zero price discount when gobl.ubl writes none.
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

// aeStandardRate provides the AE regime's standard VAT rate on the invoice's tax date, as UBL writes it.
func aeStandardRate(inv *bill.Invoice) string {
	date := inv.IssueDate
	if inv.ValueDate != nil {
		date = *inv.ValueDate
	}
	rd := tax.RegimeDefFor(l10n.AE).CategoryDef(tax.CategoryVAT).RateDef(tax.KeyStandard, tax.RateGeneral)
	if rd == nil {
		return ""
	}
	if v := rd.Value(date, tax.Extensions{}); v != nil {
		return v.Percent.StringWithoutSymbol()
	}
	return ""
}

// aeRate provides the rate PINT AE expects for a VAT category where gobl.ubl writes 0: none for exempt,
// the standard rate for reverse charge and margin scheme.
func aeRate(id *goblubl.IDType, percent *string, standard string) *string {
	if id == nil {
		return percent
	}
	switch id.Value {
	case taxCategoryExempt:
		return nil
	case taxCategoryReverseCharge, taxCategoryMarginScheme:
		rate := standard
		return &rate
	}
	return percent
}

// exportAEPartyTaxID writes UAE TRNs without the AE prefix gobl.ubl adds.
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

// exportAETINIdentification writes the scheme of a TIN identity in cac:PartyIdentification as its
// ISO 6523 ICD, 0235, where gobl.ubl writes its type, TIN, which is not an ICD.
func exportAETINIdentification(out *goblubl.Party) {
	for i := range out.PartyIdentification {
		id := out.PartyIdentification[i].ID
		if id != nil && id.SchemeID != nil && *id.SchemeID == ae.IdentityTypeTIN.String() {
			scheme := ae.TINScheme.String()
			id.SchemeID = &scheme
		}
	}
}

// exportAELegalIdentity writes the PINT AE type of the first legal identity as @schemeAgencyID (BTAE-15/16),
// and a passport's country or a trade licence's authority as @schemeAgencyName.
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

// importAEPartyIdentities reads back what exportAEPartyIdentities writes, and the supplier's non-VAT
// tax registration as its TIN (IBT-032).
//
// TODO: the seller TIN scheme has no code list (examples use TIN and TAX), so any scheme is read as TIN.
// TODO: gobl.ubl gives imported tax registrations the address's country; fix there.
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
	for _, id := range p.Identities {
		if id != nil && id.Scope == "" && id.Ext.Get(iso.ExtKeySchemeID) == ae.TINScheme {
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

// importAEPartyEndpoint reads cbc:EndpointID as an endpoint, replacing the inbox
// gobl.ubl imports it as.
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

// exportAETaxTotal fixes the breakdown rates, merges breakdowns left with the same category and rate,
// and says prices exclude VAT (IBT-200).
func exportAETaxTotal(tt *goblubl.TaxTotal, standard string) {
	for i := range tt.TaxSubtotal {
		c := &tt.TaxSubtotal[i].TaxCategory
		c.Percent = aeRate(c.ID, c.Percent, standard)
	}
	tt.TaxSubtotal = mergeAETaxSubtotals(tt.TaxSubtotal)
	if len(tt.TaxSubtotal) > 0 {
		// gobl.ubl takes the taxes out of prices that include them.
		included := false
		tt.TaxIncludedIndicator = &included
	}
}

// mergeAETaxSubtotals merges breakdowns with the same category and rate: GOBL makes one per exemption
// code, PINT AE wants one per category. Codes and reasons that differ are dropped.
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

// samePercent compares rates by value, so "0" and "0.0" match.
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
