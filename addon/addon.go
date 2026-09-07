// Package addon is a convenience aggregator that registers all of the Peppol
// addons:
//
//   - peppol-pint-v1     (base Peppol International billing model)
//   - peppol-pint-au-v1  (Australia jurisdiction)
//
// Import it for its side effects to make all of them available, or import the
// individual subpackages directly. Each document must declare the addon that
// applies to it.
package addon

import (
	_ "github.com/invopop/gobl.peppol/addon/au"
	_ "github.com/invopop/gobl.peppol/addon/pint"
)
