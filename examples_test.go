package peppol_test

import (
	"flag"
	"path/filepath"
	"testing"

	// Register the Peppol addons so example documents declaring the peppol-pint-*
	// addons normalize and validate.
	_ "github.com/invopop/gobl.peppol/addon"

	"github.com/invopop/gobl/pkg/examples"
)

var update = flag.Bool("update", false, "update the example golden files")

// TestExamples converts every document under test/data/convert/ to a
// calculated, validated JSON envelope and compares it against its golden
// output, using the shared GOBL example helpers. Run with -update to
// (re)generate the goldens.
func TestExamples(t *testing.T) {
	examples.Run(t, filepath.Join("test", "data", "convert"), *update)
}
