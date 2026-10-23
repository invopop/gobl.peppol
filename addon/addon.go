// Package addon is a convenience aggregator that registers all of the Peppol
// addons:
//
//   - peppol-bis-v3   (Peppol BIS Billing 3.0 profile)
//   - peppol-pint-v1  (Peppol International billing model, with the
//     Australia/New Zealand rules applied to AU and NZ documents)
//
// Import it for its side effects to make all of them available, or import the
// individual subpackages directly. Each document must declare the addon that
// applies to it.
package addon

import (
	_ "github.com/invopop/gobl.peppol/addon/bis"
	_ "github.com/invopop/gobl.peppol/addon/pint"
)
