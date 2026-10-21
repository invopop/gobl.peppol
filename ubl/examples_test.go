package ubl_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/gobl"
	peppolubl "github.com/invopop/gobl.peppol/ubl"
	ubl "github.com/invopop/gobl.ubl"
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
	format ubl.Format
}{
	{"bis", peppolubl.FormatBIS},
	{"pint/aunz", peppolubl.FormatAUNZ},
	{"pint", peppolubl.FormatPINT},
}

// TestConvertExamples converts each GOBL example of a format into UBL and
// compares the result with its expected output.
func TestConvertExamples(t *testing.T) {
	var pc *phorm.Client
	if *validate {
		pc = phormClient(t)
	}
	for _, ec := range exampleFormats {
		t.Run(ec.dir, func(t *testing.T) {
			examples, err := filepath.Glob(filepath.Join("testdata", "convert", ec.dir, "*.json"))
			require.NoError(t, err)
			require.NotEmpty(t, examples)
			for _, example := range examples {
				name := filepath.Base(example)
				t.Run(name, func(t *testing.T) {
					doc, err := testInvoiceFromFormat(filepath.Join(ec.dir, name), ec.format)
					require.NoError(t, err)
					data, err := ubl.Encode(doc)
					require.NoError(t, err)

					outPath := filepath.Join("testdata", "convert", ec.dir, "out", strings.Replace(name, ".json", ".xml", 1))
					if *updateOut {
						require.NoError(t, os.WriteFile(outPath, data, 0644))
					}
					if *validate {
						env, err := loadTestEnvelopeFromPath(example)
						require.NoError(t, err)
						inv, ok := env.Extract().(*bill.Invoice)
						require.True(t, ok, "Document should be an invoice")
						validateXML(t, pc, ec.format.VESID(inv), data)
					}

					output, err := os.ReadFile(outPath)
					require.NoError(t, err)
					assert.Equal(t, string(output), string(data), "Output should match the expected XML. Update with --update flag.")
				})
			}
		})
	}
}

// unvalidatedParseFixtures lists the parse fixtures that break the Peppol
// schematron on purpose, mapped to the reason, as they test the import of
// documents received in the wild.
var unvalidatedParseFixtures = map[string]string{
	"bis/example-encoding.xml":       "broken text encoding, with placeholder GLN endpoints",
	"bis/invoice-peppol.xml":         "placeholder ABN and Swedish organisation numbers",
	"bis/nbio-stuck-ubl.xml":         "a document that once stalled the import, with invalid Belgian enterprise numbers",
	"bis/partial-invoice.xml":        "partial invoice type 326 between non-German parties",
	"bis/proforma-invoice.xml":       "pro forma invoice type, which Peppol BIS does not allow",
	"bis/self-billed-creditnote.xml": "self-billing type code under the Peppol BIS Billing identifiers",
	"bis/self-billed-invoice.xml":    "self-billing type code under the Peppol BIS Billing identifiers",
}

// TestParseExamples parses each UBL example of a format into GOBL and
// compares the invoice with its expected output.
func TestParseExamples(t *testing.T) {
	var pc *phorm.Client
	if *validate {
		pc = phormClient(t)
	}
	for _, ec := range exampleFormats {
		examples, err := filepath.Glob(filepath.Join("testdata", "parse", ec.dir, "*.xml"))
		require.NoError(t, err)
		if len(examples) == 0 {
			continue
		}
		t.Run(ec.dir, func(t *testing.T) {
			for _, example := range examples {
				name := filepath.Base(example)
				t.Run(name, func(t *testing.T) {
					env := parseXMLInvoice(t, filepath.Join(ec.dir, name))
					env.Head.UUID = staticUUID
					inv, ok := env.Extract().(*bill.Invoice)
					require.True(t, ok, "document is not an invoice")
					inv.UUID = staticUUID
					require.NoError(t, env.Calculate())

					// The parse fixtures are held to the same schematron as the
					// documents the converter produces, unless they are inputs kept
					// to exercise the import of non-compliant documents.
					if reason, ok := unvalidatedParseFixtures[filepath.Join(ec.dir, name)]; ok {
						t.Logf("not validated: %s", reason)
					} else if *validate {
						data, err := os.ReadFile(example)
						require.NoError(t, err)
						validateXML(t, pc, ec.format.VESID(inv), data)
					}

					outPath := filepath.Join("testdata", "parse", ec.dir, "out", strings.Replace(name, ".xml", ".json", 1))
					if *updateOut {
						data, err := json.MarshalIndent(env, "", "\t")
						require.NoError(t, err)
						require.NoError(t, os.WriteFile(outPath, data, 0644))
					}

					data, err := json.MarshalIndent(env.Extract(), "", "\t")
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

// testInvoiceFromFormat converts a GOBL example from testdata/convert into
// a UBL invoice using the format.
func testInvoiceFromFormat(name string, f ubl.Format) (*ubl.Invoice, error) {
	env, err := loadTestEnvelopeFromPath(filepath.Join("testdata", "convert", name))
	if err != nil {
		return nil, err
	}
	return ubl.ExportInvoice(env, ubl.WithFormat(f))
}

// loadTestEnvelope loads, calculates, and validates a GOBL example from
// testdata/convert, failing the test on any error.
func loadTestEnvelope(t *testing.T, name string) *gobl.Envelope {
	t.Helper()
	env, err := loadTestEnvelopeFromPath(filepath.Join("testdata", "convert", name))
	require.NoError(t, err)
	return env
}

func loadTestEnvelopeFromPath(path string) (*gobl.Envelope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	env := new(gobl.Envelope)
	if err := json.Unmarshal(data, env); err != nil {
		return nil, err
	}

	// Clear the IDs
	env.Head.UUID = staticUUID
	if inv, ok := env.Extract().(*bill.Invoice); ok {
		inv.UUID = staticUUID
	}
	if err := env.Calculate(); err != nil {
		return nil, err
	}
	if err := env.Validate(); err != nil {
		return nil, err
	}

	if *updateOut {
		data, err := json.MarshalIndent(env, "", "\t")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return nil, err
		}
	}
	return env, nil
}

// testLoadXML provides the raw data of a UBL example from testdata/parse.
func testLoadXML(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join("testdata", "parse", name))
}

// parseXMLInvoice parses a UBL example from testdata/parse into a GOBL
// envelope, failing the test on any error.
func parseXMLInvoice(t *testing.T, name string) *gobl.Envelope {
	t.Helper()
	data, err := testLoadXML(name)
	require.NoError(t, err)
	doc, err := ubl.Decode(data)
	require.NoError(t, err)
	inv, ok := doc.(*ubl.Invoice)
	require.True(t, ok, "document is not an invoice")
	env, err := ubl.Import(inv)
	require.NoError(t, err)
	return env
}
