package ubl_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/aunz"
	"github.com/invopop/gobl.peppol/addon/pint"
	peppolubl "github.com/invopop/gobl.peppol/ubl"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/convert"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormat(t *testing.T) {
	f := convert.FormatFor(peppolubl.KeyAUNZ)
	require.NotNil(t, f)
	assert.Equal(t, cbc.Key("ubl"), f.Syntax)
	assert.ElementsMatch(t, []l10n.Code{l10n.AU, l10n.NZ}, f.Countries)

	base := convert.FormatFor(peppolubl.KeyPINT)
	require.NotNil(t, base)
	assert.Empty(t, base.Countries)
	assert.Contains(t, peppolubl.FormatPINT.Addons, pint.V1)

	for _, country := range []l10n.Code{l10n.AU, l10n.NZ} {
		keys := make([]cbc.Key, 0)
		for _, c := range convert.FormatsFor(country) {
			keys = append(keys, c.Key)
		}
		assert.Contains(t, keys, peppolubl.KeyAUNZ, country)
	}
}

func TestDetectAndImport(t *testing.T) {
	data, err := testLoadXML("aunz/invoice-au.xml")
	require.NoError(t, err)

	f, err := convert.Detect(data)
	require.NoError(t, err)
	assert.Equal(t, peppolubl.KeyAUNZ, f.Key)

	env, err := convert.Import(data)
	require.NoError(t, err)
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	assert.Contains(t, inv.GetAddons(), aunz.V1)
}

func TestExport(t *testing.T) {
	env := loadTestEnvelope(t, "aunz/invoice-au.json")
	out, err := convert.Export(env, peppolubl.KeyAUNZ)
	require.NoError(t, err)
	assert.Equal(t, peppolubl.KeyAUNZ, out.Format.Key)
	assert.Contains(t, string(out.Data), "<cbc:CustomizationID>"+peppolubl.CustomizationAUNZBilling)

	f, err := convert.Detect(out.Data)
	require.NoError(t, err)
	assert.Equal(t, peppolubl.KeyAUNZ, f.Key, "detected again")
}

// TestDetectPINTFallback checks that a PINT jurisdiction with no dedicated
// format falls back to the base billing one, while A-NZ keeps its own.
func TestDetectPINTFallback(t *testing.T) {
	f := ubl.FindFormat(peppolubl.CustomizationPINTBilling+"@jp-1", peppolubl.ProfileBilling)
	require.NotNil(t, f)
	assert.Equal(t, peppolubl.KeyPINT, f.Key)

	f = ubl.FindFormat(peppolubl.CustomizationAUNZBilling, peppolubl.ProfileBilling)
	require.NotNil(t, f)
	assert.Equal(t, peppolubl.KeyAUNZ, f.Key)

	f = ubl.FindFormat(peppolubl.CustomizationAUNZSelfBilling, peppolubl.ProfileSelfBilling)
	require.NotNil(t, f)
	assert.Equal(t, peppolubl.KeyAUNZ, f.Key)
}

// TestSpecificationAUNZSelfBilling checks that self-billed invoices are
// written with the A-NZ self-billing identifiers and validated against its
// rule set.
func TestSpecificationAUNZSelfBilling(t *testing.T) {
	env := loadTestEnvelope(t, "aunz/invoice-au.json")
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)

	f := peppolubl.FormatAUNZ
	assert.Equal(t, peppolubl.VESIDAUNZInvoice, f.VESID(inv))

	inv.SetTags(tax.TagSelfBilled)
	assert.Equal(t, peppolubl.VESIDAUNZInvoiceSelfBilling, f.VESID(inv))
	s := f.SpecificationFor(inv)
	require.NotNil(t, s)
	assert.Equal(t, peppolubl.CustomizationAUNZSelfBilling, s.CustomizationID)
	assert.Equal(t, peppolubl.ProfileSelfBilling, s.ProfileID)
}
