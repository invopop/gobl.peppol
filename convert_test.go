package peppol_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/gobl"
	peppol "github.com/invopop/gobl.peppol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadEnvelope reads a calculated GOBL envelope from a JSON file.
func loadEnvelope(t *testing.T, path string) *gobl.Envelope {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	env := new(gobl.Envelope)
	require.NoError(t, json.Unmarshal(data, env))
	return env
}

// TestConvert converts each shipped example envelope to Peppol PINT A-NZ UBL and
// compares it against its golden XML. Run with -update to regenerate the goldens.
func TestConvert(t *testing.T) {
	srcs, err := filepath.Glob(filepath.Join("examples", "out", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, srcs)
	for _, src := range srcs {
		name := strings.TrimSuffix(filepath.Base(src), ".json")
		t.Run(name, func(t *testing.T) {
			env := loadEnvelope(t, src)

			doc, err := peppol.ConvertInvoice(env)
			require.NoError(t, err)

			out, err := peppol.Bytes(doc)
			require.NoError(t, err)

			golden := filepath.Join("examples", "out", name+".xml")
			if *update {
				require.NoError(t, os.WriteFile(golden, out, 0o644))
			}
			want, err := os.ReadFile(golden)
			require.NoError(t, err)
			assert.Equal(t, string(want), string(out), "converted XML should match the golden; regenerate with -update")
		})
	}
}

// TestConvertBillingContext confirms the billing context stamps the A-NZ
// specification identifier onto the generated document.
func TestConvertBillingContext(t *testing.T) {
	env := loadEnvelope(t, filepath.Join("examples", "out", "invoice-au.json"))

	inv, err := peppol.ConvertInvoice(env)
	require.NoError(t, err)
	assert.Equal(t, peppol.CustomizationBilling, inv.CustomizationID)
}
