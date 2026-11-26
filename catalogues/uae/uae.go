// Package uae provides the UAE Federal Tax Authority code lists (Federal
// Decree-Law No. 8 of 2017) used by Peppol PINT AE.
package uae

import (
	"encoding/json"
	"slices"

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

var catalogue *tax.CatalogueDef

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
	catalogue = cat
	tax.RegisterExtension(cat.Extensions...)
}

// Extensions returns a copy of the extension definitions registered by the UAE catalogue.
func Extensions() []*cbc.Definition {
	return slices.Clone(catalogue.Extensions)
}
