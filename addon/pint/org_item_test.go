package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
)

func TestItemTypeAE(t *testing.T) {
	tests := map[cbc.Key]cbc.Code{
		org.ItemKeyGoods:          pint.ItemTypeGoods,
		org.ItemKeyServices:       pint.ItemTypeServices,
		pint.ItemKeyGoodsServices: pint.ItemTypeGoodsServices,
		"other":                   "",
		cbc.KeyEmpty:              "",
	}
	for key, want := range tests {
		assert.Equal(t, want, pint.ItemTypeAE(key), key)
	}
}

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
