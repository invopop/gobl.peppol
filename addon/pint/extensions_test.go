package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl.peppol/catalogues/uae"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddonExtensions(t *testing.T) {
	ad := tax.AddonForKey(pint.V1)
	require.NotNil(t, ad)

	txType := tax.ExtensionForKey(pint.ExtKeyTransactionType)
	vatex := tax.ExtensionForKey(uae.ExtKeyVATEX)
	goodsType := tax.ExtensionForKey(uae.ExtKeyGoodsType)
	require.NotNil(t, txType)
	require.NotNil(t, vatex)
	require.NotNil(t, goodsType)

	assert.Contains(t, ad.Extensions, txType)
	assert.Contains(t, ad.Extensions, vatex)
	assert.Contains(t, ad.Extensions, goodsType)

	aeExts := tax.FilterExtensionDefs(nil, []cbc.Code{"AE"})
	assert.Contains(t, aeExts, txType)
	assert.Contains(t, aeExts, vatex)
	assert.Contains(t, aeExts, goodsType)

	auExts := tax.FilterExtensionDefs([]cbc.Key{pint.V1}, []cbc.Code{"AU"})
	assert.NotContains(t, auExts, txType)
	assert.NotContains(t, auExts, vatex)
	assert.NotContains(t, auExts, goodsType)
}
