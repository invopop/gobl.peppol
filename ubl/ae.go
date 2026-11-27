package ubl

import (
	"regexp"
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

const (
	taxCategoryStandard      = "S"
	taxCategoryExempt        = "E"
	taxCategoryReverseCharge = "AE"
	taxCategoryMarginScheme  = "N"

	aedTotalDocumentType         = "aedtotal-incl-vat" // BTAE-20
	deliveryTermsSchemeIncoterms = "Incoterms"         // BTAE-22

	identityTypePassport cbc.Code = "PAS"  // BTAE-15, BTAE-16
	tinScheme            cbc.Code = "0235" // ISO 6523 ICD for UAE TIN
	identitySchemeGTIN   cbc.Code = "0160" // ISO 6523 ICD for GTIN

	identityLabelHS  = "HS"
	identityLabelSAC = "SAC"
)

var identityTypesAE = []cbc.Code{
	ae.IdentityTypeTradeLicense,
	ae.IdentityTypeEmiratesID,
	ae.IdentityTypeCabinetDecision,
}

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
	exportAECharges(out, inv, aeStandardRate(inv))
	exportAEMarginScheme(out, inv)
	exportAELines(out.InvoiceLines, inv)
	exportAELines(out.CreditNoteLines, inv)
	for _, p := range goblubl.InvoiceParties(inv, out) {
		exportAEPartyTaxID(p.UBL, p.GOBL.TaxID)
		exportAETINIdentification(p.UBL)
	}
	exportAEPartyIdentities(out.AccountingSupplierParty.Party, inv.Supplier)
	exportAEPartyIdentities(out.AccountingCustomerParty.Party, inv.Customer)
	if len(out.TaxTotal) > 0 {
		exportAETaxSubtotalExemptions(&out.TaxTotal[0], inv)
	}
	for i := range out.TaxTotal {
		exportAETaxTotal(&out.TaxTotal[i], aeStandardRate(inv))
	}
	exportAEExchangeRate(out, inv)
	return nil
}

func importAE(_ *goblubl.Format, doc goblubl.Document, env *gobl.Envelope) error {
	in, ok := doc.(*goblubl.Invoice)
	inv, ok2 := env.Extract().(*bill.Invoice)
	if !ok || !ok2 {
		return nil
	}
	importAEUUID(in, inv)
	importAEOutsideTaxScope(in, inv)
	importAETransactionType(in, inv)
	importAEMarginScheme(inv)
	importAECreditNoteReason(in, inv)
	importAEIncoterms(in, inv)
	importAEDeliveryPartyID(in, inv)
	importAEChargeExemptions(in, inv)
	importAELineExemptions(in.InvoiceLines, inv)
	importAELineExemptions(in.CreditNoteLines, inv)
	clearAECEFVATEX(inv)
	importAELineGoods(in.InvoiceLines, inv)
	importAELineGoods(in.CreditNoteLines, inv)
	importAELineItems(in.InvoiceLines, inv)
	importAELineItems(in.CreditNoteLines, inv)
	importAEPartyIdentities(in.AccountingSupplierParty.Party, inv.Supplier, true)
	importAEPartyIdentities(in.AccountingCustomerParty.Party, inv.Customer, false)
	for _, p := range goblubl.InvoiceParties(inv, in) {
		importAETINIdentification(p.GOBL)
	}
	return nil
}

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
	}
}

// Must run before exportAELines and exportAETaxTotal so rates and line VAT are written for N.
func exportAEMarginScheme(out *goblubl.Invoice, inv *bill.Invoice) {
	if !pint.ParseTransactionType(inv.Tax.GetExt(pint.ExtKeyTransactionType)).MarginScheme {
		return
	}
	isVAT := func(s *goblubl.TaxScheme) bool {
		return s != nil && s.ID.Value == tax.CategoryVAT.String()
	}
	rewriteLines := func(lines []goblubl.InvoiceLine) {
		for i := range lines {
			if lines[i].Item == nil || lines[i].Item.ClassifiedTaxCategory == nil {
				continue
			}
			c := lines[i].Item.ClassifiedTaxCategory
			if isVAT(c.TaxScheme) && c.ID != nil && c.ID.Value == taxCategoryExempt {
				c.ID.Value = taxCategoryMarginScheme
			}
		}
	}
	rewriteLines(out.InvoiceLines)
	rewriteLines(out.CreditNoteLines)
	for i := range out.TaxTotal {
		for j := range out.TaxTotal[i].TaxSubtotal {
			c := &out.TaxTotal[i].TaxSubtotal[j].TaxCategory
			if isVAT(c.TaxScheme) && c.ID != nil && c.ID.Value == taxCategoryExempt {
				c.ID.Value = taxCategoryMarginScheme
			}
		}
	}
}

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
			if c.Category == tax.CategoryVAT && c.Ext.Get(untdid.ExtKeyTaxCategory) == taxCategoryMarginScheme {
				c.Key = tax.KeyExempt
				c.Percent = nil
				c.Ext = c.Ext.Set(untdid.ExtKeyTaxCategory, taxCategoryExempt)
				found = true
			}
		}
	}
	if inv.Tax != nil {
		for _, n := range inv.Tax.Notes {
			if n != nil && n.Category == tax.CategoryVAT && n.Ext.Get(untdid.ExtKeyTaxCategory) == taxCategoryMarginScheme {
				n.Ext = n.Ext.Set(untdid.ExtKeyTaxCategory, taxCategoryExempt)
			}
		}
	}
	if found {
		cur := inv.Tax.GetExt(pint.ExtKeyTransactionType)
		if cur == "" || transactionTypeRegexp.MatchString(cur.String()) {
			tt := pint.ParseTransactionType(cur)
			tt.MarginScheme = true
			setAEExt(inv, pint.ExtKeyTransactionType, tt.Code())
		}
	}
}

func exportAEUUID(out *goblubl.Invoice, inv *bill.Invoice) {
	if !inv.UUID.IsZero() {
		out.UUID = inv.UUID.String()
	}
}

func exportAETransactionType(out *goblubl.Invoice, inv *bill.Invoice) {
	out.ProfileExecutionID = inv.Tax.GetExt(pint.ExtKeyTransactionType).String()
	if id := inv.Tax.GetExt(pint.ExtKeyBeneficiaryID); id != "" {
		out.BuyerCustomerParty = &goblubl.CustomerParty{Party: identifiedParty(id)}
	}
	if id := inv.Tax.GetExt(pint.ExtKeyPrincipalID); id != "" {
		out.SellerSupplierParty = &goblubl.SupplierParty{Party: identifiedParty(id)}
	}
}

func importAEUUID(in *goblubl.Invoice, inv *bill.Invoice) {
	if id, err := uuid.Parse(strings.TrimSpace(in.UUID)); err == nil {
		inv.UUID = id
	}
}

func exportAEIssueTime(out *goblubl.Invoice, inv *bill.Invoice) {
	if inv.IssueTime != nil {
		out.IssueTime = inv.IssueTime.String()
	}
}

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

func exportAEPayment(out *goblubl.Invoice, inv *bill.Invoice) {
	if len(out.PaymentMeans) == 0 || inv.Payment == nil {
		return
	}
	if instr := inv.Payment.Instructions; instr != nil && len(instr.CreditTransfer) > 0 {
		exportAECreditTransfer(out.PaymentMeans[0].PayeeFinancialAccount, instr.CreditTransfer[0])
	}
}

func exportAECreditTransfer(acc *goblubl.FinancialAccount, ct *pay.CreditTransfer) {
	if acc == nil || ct == nil {
		return
	}
	// TODO: FormatPINT should also write the branch address (IBG-34).
	if addr := aeAddress(ct.Branch); addr != nil {
		if acc.FinancialInstitutionBranch == nil {
			acc.FinancialInstitutionBranch = new(goblubl.Branch)
		}
		acc.FinancialInstitutionBranch.Address = addr
	}
}

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

func exportAECharges(out *goblubl.Invoice, inv *bill.Invoice, standard string) {
	for _, ac := range out.AllowanceCharge {
		for _, c := range ac.TaxCategory {
			if c != nil && c.ID != nil && c.ID.Value == taxCategoryReverseCharge {
				rate := standard
				c.Percent = &rate
			}
		}
	}
	if len(out.AllowanceCharge) != len(inv.Charges)+len(inv.Discounts) {
		return
	}
	for i, ch := range inv.Charges {
		if ch != nil {
			exportAETaxCategoryExemption(out.AllowanceCharge[i].TaxCategory, ch.Taxes)
		}
	}
	for i, d := range inv.Discounts {
		if d != nil {
			exportAETaxCategoryExemption(out.AllowanceCharge[len(inv.Charges)+i].TaxCategory, d.Taxes)
		}
	}
}

func exportAETaxCategoryExemption(cats []*goblubl.TaxCategory, taxes tax.Set) {
	c := taxes.Get(tax.CategoryVAT)
	if c == nil {
		return
	}
	code := c.Ext.Get(pint.ExtKeyExemption)
	if code == "" {
		return
	}
	for _, tc := range cats {
		if tc != nil && tc.TaxScheme != nil && tc.TaxScheme.ID.Value == tax.CategoryVAT.String() {
			v := code.String()
			tc.TaxExemptionReasonCode = &v
			return
		}
	}
}

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

// Keeps UAE exemptions strictly under pint-ae-exemption rather than cef-vatex.
func clearAECEFVATEX(inv *bill.Invoice) {
	clearSet := func(set tax.Set) {
		for _, c := range set {
			if c != nil {
				c.Ext = c.Ext.Delete(cef.ExtKeyVATEX)
			}
		}
	}
	for _, l := range inv.Lines {
		if l != nil {
			clearSet(l.Taxes)
		}
	}
	for _, d := range inv.Discounts {
		if d != nil {
			clearSet(d.Taxes)
		}
	}
	for _, ch := range inv.Charges {
		if ch != nil {
			clearSet(ch.Taxes)
		}
	}
	if inv.Totals != nil && inv.Totals.Taxes != nil {
		for _, cat := range inv.Totals.Taxes.Categories {
			for _, r := range cat.Rates {
				r.Ext = r.Ext.Delete(cef.ExtKeyVATEX)
			}
		}
	}
}

func identifiedParty(id cbc.Code) *goblubl.Party {
	return &goblubl.Party{PartyIdentification: []goblubl.Identification{
		{ID: &goblubl.IDType{Value: id.String()}},
	}}
}

func importAETransactionType(in *goblubl.Invoice, inv *bill.Invoice) {
	setAEExt(inv, pint.ExtKeyTransactionType, cbc.Code(strings.TrimSpace(in.ProfileExecutionID)))
	if in.BuyerCustomerParty != nil {
		setAEExt(inv, pint.ExtKeyBeneficiaryID, partyIdentificationID(in.BuyerCustomerParty.Party))
	}
	if in.SellerSupplierParty != nil {
		setAEExt(inv, pint.ExtKeyPrincipalID, partyIdentificationID(in.SellerSupplierParty.Party))
	}
}

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

func setAEExt(inv *bill.Invoice, key cbc.Key, code cbc.Code) {
	if code == "" {
		return
	}
	if inv.Tax == nil {
		inv.Tax = new(bill.Tax)
	}
	inv.Tax.Ext = inv.Tax.Ext.Set(key, code)
}

func exportAECreditNoteReason(out *goblubl.Invoice, inv *bill.Invoice) {
	if out.CreditNoteTypeCode == nil {
		return
	}
	reason := inv.Tax.GetExt(pint.ExtKeyCreditNoteReason)
	if reason == "" {
		return
	}
	out.DiscrepancyResponse = []goblubl.Response{
		{ResponseCode: &goblubl.IDType{Value: reason.String()}},
	}
}

func importAECreditNoteReason(in *goblubl.Invoice, inv *bill.Invoice) {
	for _, r := range in.DiscrepancyResponse {
		if r.ResponseCode == nil {
			continue
		}
		if code := cbc.Code(strings.TrimSpace(r.ResponseCode.Value)); code != "" {
			setAEExt(inv, pint.ExtKeyCreditNoteReason, code)
			return
		}
	}
}

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

// Rebuilds line item classifications, SAC, and buyer ID so HSN is prepended under listID HS.
func exportAELineIdentities(line *goblubl.InvoiceLine, l *bill.Line) {
	if l == nil || l.Item == nil || line.Item == nil {
		return
	}
	var hsClassifications []goblubl.CommodityClassification
	var otherClassifications []goblubl.CommodityClassification
	var additional []goblubl.ItemIdentification
	var buyerID *goblubl.ItemIdentification

	for _, id := range l.Item.Identities {
		if id == nil || id.Code == "" {
			continue
		}
		hasScheme := id.Ext.Has(iso.ExtKeySchemeID)
		switch {
		case id.Key == org.IdentityKeyHSN && !hasScheme:
			listID := identityLabelHS
			hsClassifications = append(hsClassifications, goblubl.CommodityClassification{
				ItemClassificationCode: &goblubl.IDType{
					Value:  id.Code.String(),
					ListID: &listID,
				},
			})
		case id.Label == identityLabelSAC && !hasScheme:
			scheme := identityLabelSAC
			additional = append(additional, goblubl.ItemIdentification{
				ID: &goblubl.IDType{
					SchemeID: &scheme,
					Value:    id.Code.String(),
				},
			})
		case id.Label != "" && !hasScheme:
			listID := id.Label
			otherClassifications = append(otherClassifications, goblubl.CommodityClassification{
				ItemClassificationCode: &goblubl.IDType{
					Value:  id.Code.String(),
					ListID: &listID,
				},
			})
		case !hasScheme:
			if buyerID == nil {
				buyerID = &goblubl.ItemIdentification{
					ID: &goblubl.IDType{
						Value: id.Code.String(),
					},
				}
			}
		}
	}

	classifications := append(hsClassifications, otherClassifications...)
	if len(classifications) > 0 {
		line.Item.CommodityClassification = &classifications
	} else {
		line.Item.CommodityClassification = nil
	}
	if len(additional) > 0 {
		line.Item.AdditionalItemIdentification = append(line.Item.AdditionalItemIdentification, additional...)
	}
	line.Item.BuyersItemIdentification = buyerID
}

func gtinIdentity(ids []*org.Identity) *org.Identity {
	for _, id := range ids {
		if id != nil && id.Ext.Get(iso.ExtKeySchemeID) == identitySchemeGTIN {
			return id
		}
	}
	return nil
}

func exportAELineGoods(line *goblubl.InvoiceLine, l *bill.Line) {
	if l == nil || l.Item == nil || line.Item == nil {
		return
	}
	if code := l.Item.Ext.Get(pint.ExtKeyGoodsType); code != "" {
		v := code.String()
		aeCommodityClassification(line.Item).NatureCode = &v
	}
	if id := gtinIdentity(l.Item.Identities); id != nil {
		scheme := identitySchemeGTIN.String()
		line.Item.StandardItemIdentification = &goblubl.ItemIdentification{
			ID: &goblubl.IDType{SchemeID: &scheme, Value: id.Code.String()},
		}
	}
}

func aeCommodityClassification(it *goblubl.Item) *goblubl.CommodityClassification {
	if it.CommodityClassification == nil || len(*it.CommodityClassification) == 0 {
		it.CommodityClassification = &[]goblubl.CommodityClassification{{}}
	}
	return &(*it.CommodityClassification)[0]
}

func exportAELineItemType(line *goblubl.InvoiceLine, l *bill.Line) {
	if l == nil || l.Item == nil || line.Item == nil {
		return
	}
	if typ := l.Item.Ext.Get(pint.ExtKeyItemType); typ != "" {
		v := typ.String()
		aeCommodityClassification(line.Item).CommodityCode = &v
	}
}

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

func importAELineItems(lines []goblubl.InvoiceLine, inv *bill.Invoice) {
	if len(lines) == 0 || len(lines) != len(inv.Lines) {
		return
	}
	for i, line := range lines {
		it := inv.Lines[i].Item
		if line.Item == nil || it == nil {
			continue
		}
		for _, id := range it.Identities {
			if id != nil && id.Label == identityLabelHS && !id.Ext.Has(iso.ExtKeySchemeID) {
				id.Key = org.IdentityKeyHSN
				id.Label = ""
			}
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
			typ := cbc.Code(strings.TrimSpace(*cc.CommodityCode))
			if typ == "" {
				continue
			}
			it.Ext = it.Ext.Set(pint.ExtKeyItemType, typ)
			switch typ {
			case pint.ItemTypeGoods:
				it.Key = org.ItemKeyGoods
			case pint.ItemTypeServices:
				it.Key = org.ItemKeyServices
			}
			break
		}
	}
}

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

var (
	trnRegexp             = regexp.MustCompile("^1[0-9]{14}$")
	transactionTypeRegexp = regexp.MustCompile("^[01]{8}$")
)

func extensionHasCode(key cbc.Key, code cbc.Code) bool {
	def := tax.ExtensionForKey(key)
	return def != nil && def.HasCode(code)
}

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
			exportAELineIdentities(&lines[i], inv.Lines[i])
			exportAELineGoods(&lines[i], inv.Lines[i])
			exportAELineItemType(&lines[i], inv.Lines[i])
		}
	}
}

func exportAELine(line *goblubl.InvoiceLine, standard string) {
	exportAEPrice(line)
	if line.Item != nil && line.Item.ClassifiedTaxCategory != nil {
		c := line.Item.ClassifiedTaxCategory
		c.Percent = aeRate(c.ID, c.Percent, standard)
	}
}

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

func aeAmount(a num.Amount, cur currency.Code) goblubl.Amount {
	code := cur.String()
	return goblubl.Amount{CurrencyID: &code, Value: a.String()}
}

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

func lineUnitCode(line *goblubl.InvoiceLine) string {
	if line.InvoicedQuantity != nil {
		return line.InvoicedQuantity.UnitCode
	}
	if line.CreditedQuantity != nil {
		return line.CreditedQuantity.UnitCode
	}
	return ""
}

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

func exportAEPartyIdentities(out *goblubl.Party, p *org.Party) {
	if out == nil || p == nil {
		return
	}
	exportAELegalIdentity(out, p)
}

func exportAETINIdentification(out *goblubl.Party) {
	for i := range out.PartyIdentification {
		id := out.PartyIdentification[i].ID
		if id != nil && id.SchemeID != nil && *id.SchemeID == ae.IdentityTypeTIN.String() {
			scheme := tinScheme.String()
			id.SchemeID = &scheme
		}
	}
}

func identityTypeAE(id *org.Identity) cbc.Code {
	switch {
	case id == nil:
		return ""
	case id.Key == org.IdentityKeyPassport:
		return identityTypePassport
	case id.Type.In(identityTypesAE...):
		return id.Type
	}
	return ""
}

func setIdentityTypeAE(id *org.Identity, typ cbc.Code) bool {
	switch {
	case typ == identityTypePassport:
		id.Key = org.IdentityKeyPassport
	case typ.In(identityTypesAE...):
		id.Type = typ
	default:
		return false
	}
	return true
}

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
	typ := identityTypeAE(id)
	if typ == "" {
		return
	}
	cid := out.PartyLegalEntity.CompanyID
	agencyID := typ.String()
	cid.SchemeAgencyID = &agencyID
	switch typ {
	case identityTypePassport:
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

// TODO: seller TIN scheme has no code list (examples use TIN and TAX), so any non-VAT scheme is read as TIN.
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

func importAETINIdentification(p *org.Party) {
	for _, id := range p.Identities {
		if id != nil && id.Scope == "" && id.Ext.Get(iso.ExtKeySchemeID) == tinScheme {
			id.Type = ae.IdentityTypeTIN
			id.Ext = id.Ext.Delete(iso.ExtKeySchemeID)
		}
	}
}

func importAELegalIdentity(id *org.Identity, cid *goblubl.IDType) {
	if !setIdentityTypeAE(id, cbc.Code(strings.TrimSpace(*cid.SchemeAgencyID))) {
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

func exportAETaxSubtotalExemptions(tt *goblubl.TaxTotal, inv *bill.Invoice) {
	if inv == nil || inv.Totals == nil || inv.Totals.Taxes == nil {
		return
	}
	var codes []cbc.Code
	for _, cat := range inv.Totals.Taxes.Categories {
		for _, r := range cat.Rates {
			codes = append(codes, r.Ext.Get(pint.ExtKeyExemption))
		}
	}
	if len(codes) != len(tt.TaxSubtotal) {
		return
	}
	for i, code := range codes {
		if code != "" {
			v := code.String()
			tt.TaxSubtotal[i].TaxCategory.TaxExemptionReasonCode = &v
		}
	}
}

func exportAETaxTotal(tt *goblubl.TaxTotal, standard string) {
	for i := range tt.TaxSubtotal {
		c := &tt.TaxSubtotal[i].TaxCategory
		c.Percent = aeRate(c.ID, c.Percent, standard)
	}
	tt.TaxSubtotal = mergeAETaxSubtotals(tt.TaxSubtotal)
	if len(tt.TaxSubtotal) > 0 {
		included := false
		tt.TaxIncludedIndicator = &included
	}
}

// Merges breakdowns sharing category and rate, dropping differing exemption codes or reasons.
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
