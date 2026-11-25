// Package uae provides the UAE Federal Tax Authority code lists (Federal
// Decree-Law No. 8 of 2017) used by Peppol PINT AE.
package uae

import (
	"encoding/json"

	"github.com/invopop/gobl.peppol/data"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
)

const (
	// ExtKeyVATEX identifies the UAE VAT exemption reason code extension.
	ExtKeyVATEX cbc.Key = "vatex-ae"

	// ExtKeyGoodsType identifies the UAE reverse-charge goods/services type extension.
	ExtKeyGoodsType cbc.Key = "ae-goods-type"
)

func init() {
	registerCatalogue()
}

func registerCatalogue() {
	raw, err := data.Content.ReadFile("catalogues/uae.json")
	if err != nil {
		panic(err)
	}
	cat := new(tax.CatalogueDef)
	if err := json.Unmarshal(raw, cat); err != nil {
		panic(err)
	}
	tax.RegisterExtension(cat.Extensions...)
}
