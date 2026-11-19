package cii

import (
	"testing"

	goblcii "github.com/invopop/gobl.cii"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportPartyEndpoint(t *testing.T) {
	t.Run("keeps an endpoint for the same URI scheme", func(t *testing.T) {
		party := &org.Party{Endpoints: []*org.Endpoint{{URI: "iso6523-actorid-upis::0088:5790000000001"}}}
		importPartyEndpoint(party, &goblcii.URIUniversalCommunication{ID: &goblcii.PartyID{SchemeID: "0002", Value: "123456789"}})
		require.Len(t, party.Endpoints, 1)
		assert.Equal(t, cbc.URI("iso6523-actorid-upis::0088:5790000000001"), party.Endpoints[0].URI)
	})

	t.Run("skips a missing or empty address", func(t *testing.T) {
		party := new(org.Party)
		importPartyEndpoint(party, nil)
		importPartyEndpoint(party, &goblcii.URIUniversalCommunication{})
		importPartyEndpoint(party, &goblcii.URIUniversalCommunication{ID: &goblcii.PartyID{SchemeID: "0002"}})
		assert.Empty(t, party.Endpoints)
	})
}
