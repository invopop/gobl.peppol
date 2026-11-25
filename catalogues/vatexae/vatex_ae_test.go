package vatexae_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/catalogues/vatexae"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	ed := tax.ExtensionForKey(vatexae.ExtKeyVATEX)
	require.NotNil(t, ed)
	assert.Equal(t, vatexae.ExtKeyVATEX, ed.Key)
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
}
