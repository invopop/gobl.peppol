package pint

import (
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
