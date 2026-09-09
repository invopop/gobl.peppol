package peppol_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getParsePath() string {
	return filepath.Join("test", "data", "parse")
}

// normalizeJSONField overwrites a top-level JSON field with a fixed
// placeholder, so a non-reproducible value doesn't break a golden diff.
func normalizeJSONField(t *testing.T, data []byte, field string) []byte {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	if _, ok := m[field]; ok {
		m[field] = "00000000-0000-0000-0000-000000000000"
	}
	out, err := json.MarshalIndent(m, "", "\t")
	require.NoError(t, err)
	return out
}

// TestParseInvoice parses every Peppol PINT A-NZ UBL document under
// test/data/parse back into GOBL and compares the resulting invoice against its
// golden JSON. Run with -update to (re)generate the goldens.
func TestParseInvoice(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join(getParsePath(), "*.xml"))
	require.NoError(t, err)
	require.NotEmpty(t, examples, "no PINT A-NZ examples found")

	for _, example := range examples {
		inName := filepath.Base(example)
		outName := strings.Replace(inName, ".xml", ".json", 1)

		t.Run(inName, func(t *testing.T) {
			data, err := os.ReadFile(example)
			require.NoError(t, err)

			doc, err := ubl.Parse(data)
			require.NoError(t, err)
			inv, ok := doc.(*ubl.Invoice)
			require.True(t, ok, "document should be an invoice")

			env, err := inv.Convert()
			require.NoError(t, err)

			// Compare the invoice content only: the envelope wrapper (head.uuid,
			// its digest) is freshly minted on every parse and never reproducible.
			out, err := json.MarshalIndent(env.Extract(), "", "\t")
			require.NoError(t, err)
			// No wire UUID means a fresh one gets minted every parse; normalize it away.
			if inv.UUID == "" {
				out = normalizeJSONField(t, out, "uuid")
			}
			out = append(out, '\n')

			outPath := filepath.Join(getParsePath(), "out", outName)
			if *update {
				require.NoError(t, os.WriteFile(outPath, out, 0o644))
			}

			expected, err := os.ReadFile(outPath)
			assert.NoError(t, err)
			assert.JSONEq(t, string(expected), string(out), "Output should match the expected GOBL JSON. Update with --update flag.")
		})
	}
}
