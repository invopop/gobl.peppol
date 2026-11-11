package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/stretchr/testify/assert"
)

func TestIdentityTypeAE(t *testing.T) {
	tests := map[cbc.Key]cbc.Code{
		ae.IdentityKeyTradeLicense:    pint.IdentityTypeTradeLicense,
		ae.IdentityKeyEmiratesID:      pint.IdentityTypeEmiratesID,
		org.IdentityKeyPassport:       pint.IdentityTypePassport,
		ae.IdentityKeyCabinetDecision: pint.IdentityTypeCabinetDecision,
		ae.IdentityKeyTIN:             "",
		cbc.KeyEmpty:                  "",
	}
	for key, want := range tests {
		assert.Equal(t, want, pint.IdentityTypeAE(key), key)
	}
}

func TestIdentityKeyAE(t *testing.T) {
	tests := map[cbc.Code]cbc.Key{
		"TL":  ae.IdentityKeyTradeLicense,
		"CL":  ae.IdentityKeyTradeLicense,
		"EID": ae.IdentityKeyEmiratesID,
		"PAS": org.IdentityKeyPassport,
		"CD":  ae.IdentityKeyCabinetDecision,
		"XYZ": "",
		"":    "",
	}
	for typ, want := range tests {
		assert.Equal(t, want, pint.IdentityKeyAE(typ), typ)
	}
}
