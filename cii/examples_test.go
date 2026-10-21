package cii_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/gobl"
	goblcii "github.com/invopop/gobl.cii"
	peppolcii "github.com/invopop/gobl.peppol/cii"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/uuid"
	"github.com/invopop/phorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const staticUUID uuid.UUID = "0195ce71-dc9c-72c8-bf2c-9890a4a9f0a2"

var updateOut = flag.Bool("update", false, "Update the example files in testdata")

// validate is a flag that enables schematron validation against phorm
var validate = flag.Bool("validate", false, "Run phorm schematron validation on generated XML")

// exampleFormats lists the example directories and the format each one
// is converted with.
var exampleFormats = []struct {
	dir    string
	format goblcii.Format
}{
	{"bis", peppolcii.FormatBIS},
}

// TestConvertExamples converts each GOBL example of a format into CII and
// compares the result with its expected output.
func TestConvertExamples(t *testing.T) {
	var pc *phorm.Client
	if *validate {
		pc = phormClient(t)
	}
	for _, ef := range exampleFormats {
		t.Run(ef.dir, func(t *testing.T) {
			examples, err := filepath.Glob(filepath.Join("testdata", "convert", ef.dir, "*.json"))
			require.NoError(t, err)
			require.NotEmpty(t, examples)
			for _, example := range examples {
				name := filepath.Base(example)
				t.Run(name, func(t *testing.T) {
					env := loadTestEnvelope(t, filepath.Join(ef.dir, name))
					doc, err := goblcii.Export(env, goblcii.WithFormat(ef.format))
					require.NoError(t, err)
					data, err := goblcii.Encode(doc)
					require.NoError(t, err)

					outPath := filepath.Join("testdata", "convert", ef.dir, "out", strings.Replace(name, ".json", ".xml", 1))
					if *updateOut {
						require.NoError(t, os.WriteFile(outPath, data, 0644))
					}
					if *validate {
						validateXML(t, pc, ef.format.VESID, data)
					}

					output, err := os.ReadFile(outPath)
					require.NoError(t, err)
					assert.Equal(t, string(output), string(data), "Output should match the expected XML. Update with --update flag.")
				})
			}
		})
	}
}

// TestParseExamples parses each CII example of a format into GOBL and
// compares the invoice with its expected output.
func TestParseExamples(t *testing.T) {
	for _, ef := range exampleFormats {
		examples, err := filepath.Glob(filepath.Join("testdata", "parse", ef.dir, "*.xml"))
		require.NoError(t, err)
		if len(examples) == 0 {
			continue
		}
		t.Run(ef.dir, func(t *testing.T) {
			for _, example := range examples {
				name := filepath.Base(example)
				t.Run(name, func(t *testing.T) {
					data, err := os.ReadFile(example)
					require.NoError(t, err)
					doc, err := goblcii.Decode(data)
					require.NoError(t, err)
					env, err := goblcii.Import(doc)
					require.NoError(t, err)
					env.Head.UUID = staticUUID
					inv, ok := env.Extract().(*bill.Invoice)
					require.True(t, ok, "document is not an invoice")
					inv.UUID = staticUUID
					require.NoError(t, env.Calculate())

					outPath := filepath.Join("testdata", "parse", ef.dir, "out", strings.Replace(name, ".xml", ".json", 1))
					if *updateOut {
						data, err := json.MarshalIndent(env, "", "\t")
						require.NoError(t, err)
						require.NoError(t, os.WriteFile(outPath, data, 0644))
					}

					data, err = json.MarshalIndent(env.Extract(), "", "\t")
					require.NoError(t, err)
					output, err := os.ReadFile(outPath)
					require.NoError(t, err)
					expected := new(gobl.Envelope)
					require.NoError(t, json.Unmarshal(output, expected))
					expectedData, err := json.MarshalIndent(expected.Extract(), "", "\t")
					require.NoError(t, err)
					assert.JSONEq(t, string(expectedData), string(data), "Invoice should match the expected JSON. Update with --update flag.")
				})
			}
		})
	}
}

// loadTestEnvelope loads, calculates, and validates a GOBL example from
// testdata/convert, failing the test on any error.
func loadTestEnvelope(t *testing.T, name string) *gobl.Envelope {
	t.Helper()
	path := filepath.Join("testdata", "convert", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	env := new(gobl.Envelope)
	require.NoError(t, json.Unmarshal(data, env))

	// Clear the IDs
	env.Head.UUID = staticUUID
	if inv, ok := env.Extract().(*bill.Invoice); ok {
		inv.UUID = staticUUID
	}
	require.NoError(t, env.Calculate())
	require.NoError(t, env.Validate())

	if *updateOut {
		data, err := json.MarshalIndent(env, "", "\t")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, data, 0644))
	}
	return env
}
