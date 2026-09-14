package peppol_test

import (
	"context"
	"flag"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	peppol "github.com/invopop/gobl.peppol"
	"github.com/stretchr/testify/require"
)

// validate gates the schematron test: like -update, it is off by default so a
// plain `go test ./...` needs no phorm service, and passed explicitly
// (`go test -validate`) to run the documents through the PINT A-NZ schematron.
var validate = flag.Bool("validate", false, "run schematron validation against a phorm service")

// phormBaseURL is the phorm validation service the schematron test posts to,
// from PHORM_URL or a local default.
func phormBaseURL() string {
	if u := os.Getenv("PHORM_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

// requirePhorm skips the test unless -validate is set, then returns a phorm
// base URL, failing if it cannot be reached (the flag is an explicit request
// to validate, so an unreachable service is an error rather than a skip).
func requirePhorm(t *testing.T) string {
	t.Helper()
	if !*validate {
		t.Skip("pass -validate to run schematron validation")
	}
	base := phormBaseURL()
	u, err := url.Parse(base)
	require.NoError(t, err)
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	require.NoErrorf(t, err, "phorm not reachable at %s (required by -validate)", base)
	_ = conn.Close()
	return base
}

// TestSchematron validates every Peppol PINT A-NZ UBL document under
// test/data/convert/out (converter output) and test/data/parse (parser input)
// against the PINT A-NZ schematron via phorm, failing on any schematron error.
// Run with -validate.
func TestSchematron(t *testing.T) {
	base := requirePhorm(t)
	v := peppol.NewValidator(base, "", &http.Client{Timeout: 3 * time.Minute})

	dirs := []string{
		filepath.Join("test", "data", "convert", "out"),
		filepath.Join("test", "data", "parse"),
	}
	var files []string
	for _, dir := range dirs {
		found, err := filepath.Glob(filepath.Join(dir, "*.xml"))
		require.NoError(t, err)
		files = append(files, found...)
	}
	require.NotEmpty(t, files, "no PINT A-NZ documents found to validate")

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			doc, err := os.ReadFile(file)
			require.NoError(t, err)

			vesID, err := peppol.VESIDForDocument(doc)
			require.NoError(t, err)

			findings, err := v.Validate(context.Background(), vesID, doc)
			require.NoError(t, err, "validation could not be run")

			for _, f := range findings {
				t.Errorf("%s: %s", vesID, f.Error())
			}
		})
	}
}
