package pint

import (
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// ItemKeyGoodsServices is the item key for an item that is both goods and
// services, PINT AE's item type "B". GOBL's own "goods" and "services" keys
// give the other two types.
const ItemKeyGoodsServices cbc.Key = "goods-services"

// PINT AE item types, written as the item's
// cac:CommodityClassification/cbc:CommodityCode (BTAE-13).
const (
	ItemTypeGoods         cbc.Code = "G"
	ItemTypeServices      cbc.Code = "S"
	ItemTypeGoodsServices cbc.Code = "B"
)

// Labels of the item identities PINT AE uses: the HS item classification
// (IBR-188-AE) and the service accounting code (BTAE-17, IBR-189-AE).
const (
	IdentityLabelHS  = "HS"
	IdentityLabelSAC = "SAC"
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

// itemKeyIs returns a predicate that reports whether an item has the key.
func itemKeyIs(key cbc.Key) func(any) bool {
	return func(val any) bool {
		it, ok := val.(*org.Item)
		return ok && it != nil && it.Key == key
	}
}

// identitiesHaveLabel returns a predicate that reports whether an item has
// identities with all the labels.
func identitiesHaveLabel(labels ...string) func(any) bool {
	return func(val any) bool {
		ids, ok := val.([]*org.Identity)
		if !ok {
			return false
		}
		for _, label := range labels {
			if !identitiesHaveLabelOf(ids, label) {
				return false
			}
		}
		return true
	}
}

// identitiesHaveLabelOf reports whether one of the identities gobl.ubl writes
// as item classifications, those with a label and no scheme ID, has the label.
func identitiesHaveLabelOf(ids []*org.Identity, label string) bool {
	for _, id := range ids {
		if id != nil && id.Label == label && !id.Ext.Has(iso.ExtKeySchemeID) {
			return true
		}
	}
	return false
}

// identitiesClassifiedWithHS reports whether the identities gobl.ubl writes
// as item classifications, those with a label and no scheme ID, include an HS
// one. Items without classifications pass, and the service accounting codes
// don't count as one.
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
