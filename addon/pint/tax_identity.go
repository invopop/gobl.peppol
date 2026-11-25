package pint

import "github.com/invopop/gobl/cbc"

// TINScheme is the ISO 6523 ICD the UAE Federal Tax Authority registered for the TIN, which is also the
// scheme of UAE Peppol endpoints (0235:<TIN>).
const TINScheme cbc.Code = "0235"

// UAE number formats PINT AE reads.
const (
	// TINPattern is the UAE TIN format: 10 digits starting with 1.
	TINPattern = `^1\d{9}$`
	// TRNPattern is the UAE TRN format: 15 digits starting with its TIN.
	TRNPattern = `^1\d{14}$`
)
