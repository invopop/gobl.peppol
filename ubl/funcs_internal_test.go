package ubl

import (
	"testing"

	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportPartyTaxID(t *testing.T) {
	party := func(companyID string) *goblubl.Party {
		return &goblubl.Party{
			PostalAddress: &goblubl.PostalAddress{Country: &goblubl.Country{IdentificationCode: "AU"}},
			PartyTaxScheme: []goblubl.PartyTaxScheme{{
				CompanyID: &goblubl.IDType{Value: companyID},
				TaxScheme: &goblubl.TaxScheme{ID: goblubl.IDType{Value: "GST"}},
			}},
		}
	}

	t.Run("strips the country prefix", func(t *testing.T) {
		out := &org.Party{Identities: []*org.Identity{
			{Scope: org.IdentityScopeTax, Code: "AU51824753556"},
		}}
		importPartyTaxID(party("AU51824753556"), out)
		require.NotNil(t, out.TaxID)
		assert.Equal(t, "AU", out.TaxID.Country.String())
		assert.Equal(t, cbc.Code("51824753556"), out.TaxID.Code)
		assert.Empty(t, out.Identities, "tax identity promoted")
	})

	t.Run("keeps a code without a prefix", func(t *testing.T) {
		out := new(org.Party)
		importPartyTaxID(party("51824753556"), out)
		require.NotNil(t, out.TaxID)
		assert.Equal(t, cbc.Code("51824753556"), out.TaxID.Code)
	})
}

func TestSetEndpoint(t *testing.T) {
	t.Run("keeps an endpoint for the same URI scheme", func(t *testing.T) {
		party := &org.Party{Endpoints: []*org.Endpoint{{URI: "iso6523-actorid-upis::0088:5790000000001"}}}
		setEndpoint(party, &goblubl.EndpointID{SchemeID: "0002", Value: "123456789"})
		require.Len(t, party.Endpoints, 1)
		assert.Equal(t, cbc.URI("iso6523-actorid-upis::0088:5790000000001"), party.Endpoints[0].URI)
	})

	t.Run("adds a mailto next to an ISO 6523 endpoint", func(t *testing.T) {
		party := &org.Party{Endpoints: []*org.Endpoint{{URI: "iso6523-actorid-upis::0088:5790000000001"}}}
		setEndpoint(party, &goblubl.EndpointID{SchemeID: goblubl.SchemeIDEmail, Value: "seller@example.com"})
		require.Len(t, party.Endpoints, 2)
		assert.Equal(t, cbc.URI("mailto:seller@example.com"), party.Endpoints[1].URI)
	})

	t.Run("skips a missing or empty address", func(t *testing.T) {
		party := new(org.Party)
		setEndpoint(party, nil)
		setEndpoint(party, &goblubl.EndpointID{SchemeID: "0002"})
		assert.Empty(t, party.Endpoints)
	})
}
