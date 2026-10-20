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
