package pint

import (
	"regexp"

	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/ae"
)

// tinRegexp matches a UAE TIN as PINT AE expects it: 10 digits starting with
// 1 (IBR-148-AE). The AE regime only checks the 10 digits; the leading 1 is
// PINT AE's. UAE Peppol endpoints (0235) are TINs too, so it also tells
// whether an endpoint is one (IBR-135-AE).
var tinRegexp = regexp.MustCompile(`^1\d{9}$`)

// PINT AE legal registration identifier types, written as @schemeAgencyID on
// the seller and buyer cbc:CompanyID (BTAE-15, BTAE-16). The Ministry of
// Finance lists the same four types of commercial registration number.
const (
	// IdentityTypeTradeLicense is a commercial or trade licence.
	IdentityTypeTradeLicense cbc.Code = "TL"
	// IdentityTypeEmiratesID is an Emirates ID.
	IdentityTypeEmiratesID cbc.Code = "EID"
	// IdentityTypePassport is a passport.
	IdentityTypePassport cbc.Code = "PAS"
	// IdentityTypeCabinetDecision is a Cabinet Decision.
	IdentityTypeCabinetDecision cbc.Code = "CD"
	// identityTypeCommercialLicense is only accepted by IBR-183-AE's test, for
	// the buyer. No other source lists it; its rule text names
	// "Commercial/Trade license", so it is read as a trade licence and never
	// written: TL is valid for both seller and buyer.
	identityTypeCommercialLicense cbc.Code = "CL"
)

// IdentityTypeAE returns the PINT AE legal registration identifier type for an
// identity key, or an empty code if the key has none.
func IdentityTypeAE(key cbc.Key) cbc.Code {
	switch key {
	case ae.IdentityKeyTradeLicense:
		return IdentityTypeTradeLicense
	case ae.IdentityKeyEmiratesID:
		return IdentityTypeEmiratesID
	case org.IdentityKeyPassport:
		return IdentityTypePassport
	case ae.IdentityKeyCabinetDecision:
		return IdentityTypeCabinetDecision
	}
	return ""
}

// IdentityKeyAE returns the identity key for a PINT AE legal registration
// identifier type, or an empty key if the type is unknown.
func IdentityKeyAE(typ cbc.Code) cbc.Key {
	switch typ {
	case IdentityTypeTradeLicense, identityTypeCommercialLicense:
		return ae.IdentityKeyTradeLicense
	case IdentityTypeEmiratesID:
		return ae.IdentityKeyEmiratesID
	case IdentityTypePassport:
		return org.IdentityKeyPassport
	case IdentityTypeCabinetDecision:
		return ae.IdentityKeyCabinetDecision
	}
	return ""
}

// IdentitySchemeGTIN is the ISO 6523 scheme of a GTIN, the item standard
// identifier reverse charge lines need (IBR-174-AE).
const IdentitySchemeGTIN cbc.Code = "0160"

// GTINIdentity returns the first identity with the GTIN scheme, or nil.
func GTINIdentity(ids []*org.Identity) *org.Identity {
	for _, id := range ids {
		if id != nil && id.Ext.Get(iso.ExtKeySchemeID) == IdentitySchemeGTIN {
			return id
		}
	}
	return nil
}

// IdentityLabelIncoterms is the label of the delivery identity that holds
// the Incoterms code (BTAE-22), such as "CIF".
const IdentityLabelIncoterms = "Incoterms"
