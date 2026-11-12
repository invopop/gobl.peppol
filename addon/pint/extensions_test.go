package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtensionsUsage(t *testing.T) {
	addon := tax.AddonForKey(pint.V1)
	require.NotNil(t, addon)
	require.NotEmpty(t, addon.Extensions)
	for _, ext := range addon.Extensions {
		require.NotNil(t, ext.Usage, ext.Key)
		assert.Contains(t, ext.Usage.Keys, pint.V1, ext.Key)
		assert.Contains(t, ext.Usage.Codes, cbc.Code(ae.CountryCode), ext.Key)
		assert.Len(t, ext.Usage.Schemas, 1, ext.Key)
		assert.NoError(t, rules.Validate(ext.Usage), ext.Key)
	}
}
