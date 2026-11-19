package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func TestItemKeyAE(t *testing.T) {
	tests := map[cbc.Code]cbc.Key{
		"G": org.ItemKeyGoods,
		"S": org.ItemKeyServices,
		"B": pint.ItemKeyGoodsServices,
		"X": "",
		"":  "",
	}
	for typ, want := range tests {
		assert.Equal(t, want, pint.ItemKeyAE(typ), typ)
	}
}

func TestOrgItemRulesAE(t *testing.T) {
	const (
		faultGoodsHS        = "GOBL-PEPPOL-PINT-ORG-ITEM-01"
		faultGoodsServices  = "GOBL-PEPPOL-PINT-ORG-ITEM-02"
		faultClassification = "GOBL-PEPPOL-PINT-ORG-ITEM-03"
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
