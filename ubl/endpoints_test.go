package ubl_test

import (
	"testing"

	peppolubl "github.com/invopop/gobl.peppol/ubl"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportEndpoints checks that every Peppol UBL format adds the seller's
// and buyer's electronic address as an endpoint, whatever the EAS scheme,
// while keeping the inbox the base import reads it into.
func TestImportEndpoints(t *testing.T) {
	formats := map[string]struct {
		format  ubl.Format
		example string
	}{
		"BIS":       {peppolubl.FormatBIS, "bis/invoice-minimal.json"},
		"PINT":      {peppolubl.FormatPINT, "pint/invoice-au.json"},
		"PINT A-NZ": {peppolubl.FormatAUNZ, "pint/aunz/invoice-au.json"},
	}
	cases := map[string]struct {
		supplier *ubl.EndpointID
		customer *ubl.EndpointID
		want     [2]cbc.URI
	}{
		"0002 and 0225": {
			supplier: &ubl.EndpointID{SchemeID: "0002", Value: "123456789"},
			customer: &ubl.EndpointID{SchemeID: "0225", Value: "987654321"},
			want:     [2]cbc.URI{"iso6523-actorid-upis::0002:123456789", "iso6523-actorid-upis::0225:987654321"},
		},
		"0088": {
			supplier: &ubl.EndpointID{SchemeID: "0088", Value: "5790000000001"},
			customer: &ubl.EndpointID{SchemeID: "0088", Value: "5790000000002"},
			want:     [2]cbc.URI{"iso6523-actorid-upis::0088:5790000000001", "iso6523-actorid-upis::0088:5790000000002"},
		},
		"empty scheme": {
			supplier: &ubl.EndpointID{Value: "123456789"},
			customer: &ubl.EndpointID{Value: "987654321"},
		},
	}
	for fname, fc := range formats {
		t.Run(fname, func(t *testing.T) {
			for name, tc := range cases {
				t.Run(name, func(t *testing.T) {
					doc, err := ubl.ExportInvoice(loadTestEnvelope(t, fc.example), ubl.WithFormat(fc.format))
					require.NoError(t, err)
					doc.AccountingSupplierParty.Party.EndpointID = tc.supplier
					doc.AccountingCustomerParty.Party.EndpointID = tc.customer

					env, err := ubl.Import(doc, ubl.WithFormat(fc.format))
					require.NoError(t, err)
					inv, ok := env.Extract().(*bill.Invoice)
					require.True(t, ok)

					for i, p := range []*org.Party{inv.Supplier, inv.Customer} {
						if tc.want[i] == "" {
							assert.Empty(t, p.Endpoints)
							continue
						}
						require.Len(t, p.Endpoints, 1)
						assert.Equal(t, tc.want[i], p.Endpoints[0].URI)
						assert.Len(t, p.Inboxes, 1, "base inbox kept")
					}
				})
			}
		})
	}
}
