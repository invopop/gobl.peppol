package pint

import (
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// ItemKeyGoodsServices is the item key for PINT AE's item type B, both goods and services.
const ItemKeyGoodsServices cbc.Key = "goods-services"

// PINT AE item types, written as the item's
// cac:CommodityClassification/cbc:CommodityCode (BTAE-13).
const (
	ItemTypeGoods         cbc.Code = "G"
	ItemTypeServices      cbc.Code = "S"
	ItemTypeGoodsServices cbc.Code = "B"
)

// Labels of the item identities PINT AE uses.
const (
	IdentityLabelHS  = "HS"  // HS item classification
	IdentityLabelSAC = "SAC" // service accounting code
)

// ItemTypeAE returns the PINT AE item type for an item key, or an empty code
// if the key has none.
func ItemTypeAE(key cbc.Key) cbc.Code {
	switch key {
	case org.ItemKeyGoods:
		return ItemTypeGoods
	case org.ItemKeyServices:
		return ItemTypeServices
	case ItemKeyGoodsServices:
		return ItemTypeGoodsServices
	}
	return ""
}

// ItemKeyAE returns the item key for a PINT AE item type, or an empty key if
// the type is unknown.
func ItemKeyAE(typ cbc.Code) cbc.Key {
	switch typ {
	case ItemTypeGoods:
		return org.ItemKeyGoods
	case ItemTypeServices:
		return org.ItemKeyServices
	case ItemTypeGoodsServices:
		return ItemKeyGoodsServices
	}
	return ""
}

// orgItemRules validates items on UAE documents.
func orgItemRules() *rules.Set {
	return rules.For(new(org.Item),
		rules.When(
			is.InContext(tax.RegimeIn(regimesAE...)),
			rules.When(
				is.Func("goods", itemKeyIs(org.ItemKeyGoods)),
				rules.Field("identities",
					rules.Assert("01", "goods item needs an HS item classification (IBR-184-AE)",
						is.Func("has HS", identitiesHaveLabel(IdentityLabelHS)),
					),
				),
			),
			rules.When(
				is.Func("goods and services", itemKeyIs(ItemKeyGoodsServices)),
				rules.Field("identities",
					rules.Assert("02", "goods and services item needs an HS item classification (IBR-186-AE)",
						is.Func("has HS", identitiesHaveLabel(IdentityLabelHS)),
					),
				),
			),
			// IBR-188-AE's text wants every classification to be HS, but its test passes when any one is.
			rules.Field("identities",
				rules.Assert("03", "item classifications must include one from the HS list (IBR-188-AE)",
					is.Func("classifications include HS", identitiesClassifiedWithHS),
				),
			),
		),
	)
}

// itemKeyIs returns a predicate that reports whether an item has the key.
func itemKeyIs(key cbc.Key) func(any) bool {
	return func(val any) bool {
		it, ok := val.(*org.Item)
		return ok && it != nil && it.Key == key
	}
}

// identitiesHaveLabel returns a predicate that reports whether one of the item classifications,
// the identities with a label and no scheme ID, has the label.
func identitiesHaveLabel(label string) func(any) bool {
	return func(val any) bool {
		ids, ok := val.([]*org.Identity)
		if !ok {
			return false
		}
		for _, id := range ids {
			if id != nil && id.Label == label && !id.Ext.Has(iso.ExtKeySchemeID) {
				return true
			}
		}
		return false
	}
}

// identitiesClassifiedWithHS reports whether the item classifications, if any, include an HS one.
// SAC codes don't count.
func identitiesClassifiedWithHS(val any) bool {
	ids, ok := val.([]*org.Identity)
	if !ok {
		return false
	}
	classified := false
	for _, id := range ids {
		if id == nil || id.Label == "" || id.Label == IdentityLabelSAC || id.Ext.Has(iso.ExtKeySchemeID) {
			continue
		}
		if id.Label == IdentityLabelHS {
			return true
		}
		classified = true
	}
	return !classified
}
