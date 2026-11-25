// Package data embeds the static data files shipped with gobl.peppol.
package data

import "embed"

// Content contains the embedded catalogue definitions.
//
//go:embed catalogues/*.json
var Content embed.FS
