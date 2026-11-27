package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddonExtensions(t *testing.T) {
	ad := tax.AddonForKey(pint.V1)
	require.NotNil(t, ad)
	assert.Empty(t, ad.Tags)

	keys := []cbc.Key{
		pint.ExtKeyTransactionType,
		pint.ExtKeyCreditNoteReason,
		pint.ExtKeyBeneficiaryID,
		pint.ExtKeyPrincipalID,
		pint.ExtKeyExemption,
		pint.ExtKeyGoodsType,
		pint.ExtKeyItemType,
	}

	aeExts := tax.FilterExtensionDefs(nil, []cbc.Code{"AE"})
	auExts := tax.FilterExtensionDefs([]cbc.Key{pint.V1}, []cbc.Code{"AU"})

	for _, key := range keys {
		def := tax.ExtensionForKey(key)
		require.NotNil(t, def, key)
		assert.Contains(t, ad.Extensions, def, key)
		assert.Contains(t, aeExts, def, key)
		assert.NotContains(t, auExts, def, key)
	}
}
