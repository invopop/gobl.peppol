package uae_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/catalogues/uae"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	t.Run("vatex-ae", func(t *testing.T) {
		ed := tax.ExtensionForKey(uae.ExtKeyVATEX)
		require.NotNil(t, ed)
		assert.Equal(t, uae.ExtKeyVATEX, ed.Key)
		assert.Len(t, ed.Values, 4)

		for _, code := range []cbc.Code{"DL8.46.1", "DL8.46.2", "DL8.46.3", "DL8.46.4"} {
			cd := ed.CodeDef(code)
			require.NotNil(t, cd, code.String())
			assert.NotEmpty(t, cd.Name.In("en"), code.String())
			assert.NotEmpty(t, cd.Desc.In("en"), code.String())
		}

		for _, code := range []cbc.Code{"DL8.46.5", "VATEX-EU-132", ""} {
			assert.Nil(t, ed.CodeDef(code), code.String())
		}
	})

	t.Run("ae-goods-type", func(t *testing.T) {
		ed := tax.ExtensionForKey(uae.ExtKeyGoodsType)
		require.NotNil(t, ed)
		assert.Equal(t, uae.ExtKeyGoodsType, ed.Key)
		assert.Len(t, ed.Values, 6)

		for _, code := range []cbc.Code{"DL8.48.8.2", "DL8.48.8.1", "DL8.48.8.3", "DL8.48.3.1", "DL8.48.3.2", "DL8.48.3.3"} {
			cd := ed.CodeDef(code)
			require.NotNil(t, cd, code.String())
			assert.NotEmpty(t, cd.Name.In("en"), code.String())
		}

		for _, code := range []cbc.Code{"DL8.48.9", "OTHER", ""} {
			assert.Nil(t, ed.CodeDef(code), code.String())
		}
	})

	t.Run("extensions", func(t *testing.T) {
		exts := uae.Extensions()
		require.Len(t, exts, 2)
		assert.Same(t, tax.ExtensionForKey(uae.ExtKeyVATEX), exts[0])
		assert.Same(t, tax.ExtensionForKey(uae.ExtKeyGoodsType), exts[1])
	})
}
