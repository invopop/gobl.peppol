package cii_test

import (
	"testing"

	goblcii "github.com/invopop/gobl.cii"
	peppolcii "github.com/invopop/gobl.peppol/cii"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportEndpoints checks that the Peppol CII format adds the seller's and
// buyer's electronic address as an endpoint, whatever the EAS scheme, while
// keeping the inbox the base import reads it into.
func TestImportEndpoints(t *testing.T) {
	uc := func(scheme, value string) *goblcii.URIUniversalCommunication {
		return &goblcii.URIUniversalCommunication{ID: &goblcii.PartyID{SchemeID: scheme, Value: value}}
	}
	cases := map[string]struct {
		seller *goblcii.URIUniversalCommunication
		buyer  *goblcii.URIUniversalCommunication
		want   [2]cbc.URI
	}{
		"0002 and 0225": {
			seller: uc("0002", "123456789"),
			buyer:  uc("0225", "987654321"),
			want:   [2]cbc.URI{"iso6523-actorid-upis::0002:123456789", "iso6523-actorid-upis::0225:987654321"},
		},
		"0088": {
			seller: uc("0088", "5790000000001"),
			buyer:  uc("0088", "5790000000002"),
			want:   [2]cbc.URI{"iso6523-actorid-upis::0088:5790000000001", "iso6523-actorid-upis::0088:5790000000002"},
		},
		"email": {
			seller: uc(goblcii.SchemeIDEmail, "seller@example.com"),
			buyer:  uc(goblcii.SchemeIDEmail, "buyer@example.com"),
			want:   [2]cbc.URI{"mailto:seller@example.com", "mailto:buyer@example.com"},
		},
		"empty scheme": {
			seller: uc("", "123456789"),
			buyer:  uc("", "987654321"),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			doc, err := goblcii.ExportInvoice(loadTestEnvelope(t, "bis/invoice-minimal.json"), goblcii.WithFormat(peppolcii.FormatBIS))
			require.NoError(t, err)
			doc.Transaction.Agreement.Seller.URIUniversalCommunication = tc.seller
			doc.Transaction.Agreement.Buyer.URIUniversalCommunication = tc.buyer

			env, err := goblcii.Import(doc, goblcii.WithFormat(peppolcii.FormatBIS))
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
}
