package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillLineRulesAE(t *testing.T) {
	const (
		faultLineVAT         = "GOBL-PEPPOL-PINT-BILL-LINE-01"
		faultLineDescription = "GOBL-PEPPOL-PINT-BILL-LINE-02"
	)

	t.Run("line without a VAT combo is rejected (IBR-145-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Taxes = nil
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultLineVAT)
		assert.Contains(t, out, "$.lines[0].taxes")
	})

	t.Run("item description is required (IBR-125-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Item.Description = ""
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultLineDescription)
		assert.Contains(t, out, "$.lines[0].item.description")
	})

	t.Run("line rules do not apply outside AE", func(t *testing.T) {
		inv := scenarioInvoice(l10n.AU.Tax(), bill.InvoiceTypeStandard)
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.AU.Tax()))
		if err != nil {
			assert.NotContains(t, err.Error(), faultLineDescription)
		}
	})
}

func TestBillGoodsTypeRulesAE(t *testing.T) {
	const (
		faultGoodsType      = "GOBL-PEPPOL-PINT-BILL-LINE-04"
		faultGoodsTypeValue = "GOBL-PEPPOL-PINT-BILL-LINE-05"
		faultGTIN           = "GOBL-PEPPOL-PINT-BILL-LINE-06"
	)
	gtin := func(scheme cbc.Code) []*org.Identity {
		return []*org.Identity{{Code: "4006381333931", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: scheme})}}
	}
	reverseCharge := func(goodsType cbc.Code, ids []*org.Identity) *bill.Invoice {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Taxes = tax.Set{lineCombo(tax.KeyReverseCharge)}
		inv.Lines[0].Item.Identities = ids
		if goodsType != "" {
			inv.Lines[0].Item.Ext = tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyGoodsType: goodsType})
		}
		return inv
	}

	t.Run("reverse charge item needs the goods type (IBR-166-AE)", func(t *testing.T) {
		assert.Contains(t, aeFaults(t, reverseCharge("", gtin("0160"))), faultGoodsType)
		out := aeFaults(t, reverseCharge("DL8.48.3.1", gtin("0160")))
		assert.NotContains(t, out, faultGoodsType)
		assert.NotContains(t, out, faultGoodsTypeValue)
		assert.NotContains(t, out, faultGTIN)
	})

	t.Run("goods type must be from the AE list on any line (IBR-006-AE)", func(t *testing.T) {
		assert.NotContains(t, aeFaults(t, reverseCharge("DL8.48.8.3", gtin("0160"))), faultGoodsTypeValue)
		assert.Contains(t, aeFaults(t, reverseCharge("DL8.48.9", gtin("0160"))), faultGoodsTypeValue)

		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Item.Ext = tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyGoodsType: "OTHER"})
		assert.Contains(t, aeFaults(t, inv), faultGoodsTypeValue)
	})

	t.Run("reverse charge item needs a GTIN (IBR-174-AE)", func(t *testing.T) {
		assert.Contains(t, aeFaults(t, reverseCharge("DL8.48.3.1", nil)), faultGTIN)
		assert.Contains(t, aeFaults(t, reverseCharge("DL8.48.3.1", gtin("0088"))), faultGTIN)
	})

	t.Run("other lines need neither", func(t *testing.T) {
		out := aeFaults(t, aeDocument(bill.InvoiceTypeStandard))
		assert.NotContains(t, out, faultGoodsType)
		assert.NotContains(t, out, faultGTIN)
	})
}

func TestBillItemTypeRulesAE(t *testing.T) {
	const (
		faultGoodsHS        = "GOBL-PEPPOL-PINT-BILL-LINE-07"
		faultGoodsServices  = "GOBL-PEPPOL-PINT-BILL-LINE-08"
		faultClassification = "GOBL-PEPPOL-PINT-BILL-LINE-09"
	)
	hs := &org.Identity{Label: pint.IdentityLabelHS, Code: "8471"}
	sac := &org.Identity{Label: pint.IdentityLabelSAC, Code: "998713"}
	item := func(key cbc.Key, ids ...*org.Identity) *bill.Invoice {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Item.Key = key
		inv.Lines[0].Item.Identities = ids
		return inv
	}

	t.Run("goods need an HS classification (IBR-184-AE)", func(t *testing.T) {
		assert.Contains(t, aeFaults(t, item(org.ItemKeyGoods)), faultGoodsHS)
		assert.Contains(t, aeFaults(t, item(org.ItemKeyGoods, sac)), faultGoodsHS)
		assert.NotContains(t, aeFaults(t, item(org.ItemKeyGoods, hs)), faultGoodsHS)

		// An identity with a scheme ID isn't written as a classification.
		withScheme := &org.Identity{Label: pint.IdentityLabelHS, Code: "8471", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: "0088"})}
		assert.Contains(t, aeFaults(t, item(org.ItemKeyGoods, withScheme)), faultGoodsHS)
	})

	t.Run("services need no service accounting code", func(t *testing.T) {
		assert.Empty(t, aeFaults(t, item(org.ItemKeyServices)))
	})

	t.Run("goods and services need an HS classification (IBR-186-AE)", func(t *testing.T) {
		assert.Contains(t, aeFaults(t, item(pint.ItemKeyGoodsServices)), faultGoodsServices)
		assert.Contains(t, aeFaults(t, item(pint.ItemKeyGoodsServices, sac)), faultGoodsServices)
		assert.NotContains(t, aeFaults(t, item(pint.ItemKeyGoodsServices, hs)), faultGoodsServices)
	})

	t.Run("no item type needs neither", func(t *testing.T) {
		out := aeFaults(t, item(""))
		assert.NotContains(t, out, faultGoodsHS)
		assert.NotContains(t, out, faultGoodsServices)
	})

	t.Run("item classifications must include an HS one (IBR-188-AE)", func(t *testing.T) {
		cpv := &org.Identity{Label: "CPV", Code: "30200000"}
		assert.Contains(t, aeFaults(t, item("", cpv)), faultClassification)
		assert.Contains(t, aeFaults(t, item("", cpv, sac)), faultClassification)
		assert.NotContains(t, aeFaults(t, item("", hs, cpv)), faultClassification)
		assert.NotContains(t, aeFaults(t, item("", hs, sac)), faultClassification)
		assert.NotContains(t, aeFaults(t, item("", sac)), faultClassification)

		// Identities with a scheme ID aren't classifications.
		gtin := &org.Identity{Label: "GTIN", Code: "4006381333931", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: "0160"})}
		assert.NotContains(t, aeFaults(t, item("", gtin)), faultClassification)
	})
}
