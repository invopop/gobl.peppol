package peppol_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/gobl"
	peppol "github.com/invopop/gobl.peppol"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const jsonPattern = "*.json"

func getConvertPath() string {
	return filepath.Join("test", "data", "convert")
}

// convertCase is a GOBL envelope to convert, with the PINT A-NZ XML it should
// produce.
type convertCase struct {
	name   string
	src    string
	golden string
}

// convertCases lists the fixtures under test/data/convert plus the shipped
// examples. The examples are here because TestExamples only checks GOBL-to-GOBL
// normalisation: without this nothing converts them, and an example could ship
// as invalid PINT A-NZ UBL.
func convertCases(t *testing.T) []convertCase {
	t.Helper()
	dirs := []struct{ label, src, golden string }{
		{"convert", getConvertPath(), filepath.Join(getConvertPath(), "out")},
		{"examples", filepath.Join("examples", "out"), filepath.Join("examples", "out")},
	}
	var cases []convertCase
	for _, dir := range dirs {
		found, err := filepath.Glob(filepath.Join(dir.src, jsonPattern))
		require.NoError(t, err)
		require.NotEmpty(t, found, "no envelopes found in %s", dir.src)
		for _, src := range found {
			name := strings.TrimSuffix(filepath.Base(src), ".json")
			cases = append(cases, convertCase{
				name:   dir.label + "/" + name,
				src:    src,
				golden: filepath.Join(dir.golden, name+".xml"),
			})
		}
	}
	return cases
}

// loadTestEnvelope loads a GOBL envelope from a JSON file path.
func loadTestEnvelope(t *testing.T, path string) *gobl.Envelope {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	env := new(gobl.Envelope)
	require.NoError(t, json.Unmarshal(data, env))
	return env
}

// TestConvert converts every fixture and shipped example to a Peppol PINT A-NZ
// UBL document, using gobl.ubl's EN 16931 base with this module's billing
// context, and compares it against its golden XML. Run with -update to
// (re)generate the goldens.
func TestConvert(t *testing.T) {
	for _, example := range convertCases(t) {
		t.Run(example.name, func(t *testing.T) {
			env := loadTestEnvelope(t, example.src)

			doc, err := ubl.ConvertInvoice(env, ubl.WithContext(peppol.ContextPINT))
			require.NoError(t, err)

			data, err := ubl.Bytes(doc)
			require.NoError(t, err)

			if *update {
				require.NoError(t, os.WriteFile(example.golden, data, 0o644))
			}

			output, err := os.ReadFile(example.golden)
			assert.NoError(t, err)
			assert.Equal(t, string(output), string(data), "Output should match the expected XML. Update with --update flag.")
		})
	}
}
