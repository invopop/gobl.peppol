package ubl_test

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	peppolubl "github.com/invopop/gobl.peppol/ubl"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/convert"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/ae"
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
	data, err := testLoadXML("pint/aunz/invoice-au.xml")
	require.NoError(t, err)

	f, err := convert.Detect(data)
	require.NoError(t, err)
	assert.Equal(t, peppolubl.KeyAUNZ, f.Key)

	res, err := convert.Import(data)
	require.NoError(t, err)
	assert.Equal(t, peppolubl.KeyAUNZ, res.Format.Key)
	inv, ok := res.Envelope.Extract().(*bill.Invoice)
	require.True(t, ok)
	assert.Contains(t, inv.GetAddons(), pint.V1)
}

func TestExport(t *testing.T) {
	env := loadTestEnvelope(t, "pint/aunz/invoice-au.json")
	out, err := convert.Export(env, peppolubl.KeyAUNZ)
	require.NoError(t, err)
	assert.Equal(t, peppolubl.KeyAUNZ, out.Format.Key)
	assert.Contains(t, string(out.Data), "<cbc:CustomizationID>"+peppolubl.FormatAUNZ.CustomizationID)

	f, err := convert.Detect(out.Data)
	require.NoError(t, err)
	assert.Equal(t, peppolubl.KeyAUNZ, f.Key, "detected again")
}

// TestDetectPINTFallback checks that a PINT jurisdiction with no dedicated
// format falls back to the base billing one, while A-NZ and AE keep their own.
func TestDetectPINTFallback(t *testing.T) {
	f := ubl.FindFormat(peppolubl.FormatPINT.CustomizationID+"@jp-1", peppolubl.FormatPINT.ProfileID)
	require.NotNil(t, f)
	assert.Equal(t, peppolubl.KeyPINT, f.Key)

	f = ubl.FindFormat(peppolubl.FormatAUNZ.CustomizationID, peppolubl.FormatAUNZ.ProfileID)
	require.NotNil(t, f)
	assert.Equal(t, peppolubl.KeyAUNZ, f.Key)

	f = ubl.FindFormat(peppolubl.SpecificationAUNZSelfBilling.CustomizationID, peppolubl.SpecificationAUNZSelfBilling.ProfileID)
	require.NotNil(t, f)
	assert.Equal(t, peppolubl.KeyAUNZ, f.Key)
	// AE has its own format now, so the fallback no longer claims it.
	f = ubl.FindFormat(peppolubl.FormatPINT.CustomizationID+"@ae-1", peppolubl.FormatPINT.ProfileID)
	require.NotNil(t, f)
	assert.Equal(t, peppolubl.KeyAE, f.Key)
}

// TestSpecificationAUNZSelfBilling checks that self-billed invoices are
// written with the A-NZ self-billing identifiers and validated against its
// rule set.
func TestSpecificationAUNZSelfBilling(t *testing.T) {
	env := loadTestEnvelope(t, "pint/aunz/invoice-au.json")
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)

	f := peppolubl.FormatAUNZ
	assert.Equal(t, peppolubl.FormatAUNZ.VESIDs.Invoice, f.VESID(inv))

	inv.SetTags(tax.TagSelfBilled)
	assert.Equal(t, peppolubl.SpecificationAUNZSelfBilling.VESIDs.Invoice, f.VESID(inv))
	s := f.SpecificationFor(inv)
	require.NotNil(t, s)
	assert.Equal(t, peppolubl.SpecificationAUNZSelfBilling.CustomizationID, s.CustomizationID)
	assert.Equal(t, peppolubl.SpecificationAUNZSelfBilling.ProfileID, s.ProfileID)
}

// TestFormatAE checks that the PINT AE format is registered for the UAE and
// claims its own documents, both billing and self-billing.
func TestFormatAE(t *testing.T) {
	f := convert.FormatFor(peppolubl.KeyAE)
	require.NotNil(t, f)
	assert.Equal(t, []l10n.Code{l10n.AE}, f.Countries)

	keys := make([]cbc.Key, 0)
	for _, c := range convert.FormatsFor(l10n.AE) {
		keys = append(keys, c.Key)
	}
	assert.Contains(t, keys, peppolubl.KeyAE)

	for _, id := range []string{peppolubl.FormatAE.CustomizationID, peppolubl.SpecificationAESelfBilling.CustomizationID} {
		found := ubl.FindFormat(id, "")
		require.NotNil(t, found, id)
		assert.Equal(t, peppolubl.KeyAE, found.Key, id)
	}
}

// TestDetectAndImportAE checks that an official PINT AE example is detected
// as PINT AE and imported with the PINT addon.
func TestDetectAndImportAE(t *testing.T) {
	data, err := testLoadXML("pint/ae/standard-invoice.xml")
	require.NoError(t, err)

	f, err := convert.Detect(data)
	require.NoError(t, err)
	assert.Equal(t, peppolubl.KeyAE, f.Key)

	res, err := convert.Import(data)
	require.NoError(t, err)
	inv, ok := res.Envelope.Extract().(*bill.Invoice)
	require.True(t, ok)
	assert.Contains(t, inv.GetAddons(), pint.V1)
	assert.Equal(t, l10n.TaxCountryCode("AE"), inv.GetRegime())
}

// invalidImportsAE are the official PINT AE examples that pass the PINT AE
// Schematron but don't import as valid GOBL documents, with the faults they
// give: placeholder email addresses, and, on the deemed supply, no payment
// terms, which EN 16931 requires (BR-CO-25).
var invalidImportsAE = map[string][]string{
	"deemed-supply.xml":          {"GOBL-EU-EN16931-BILL-INVOICE-07", "GOBL-ORG-EMAIL-02"},
	"margin-scheme.xml":          {"GOBL-ORG-EMAIL-02"},
	"reverse-charge.xml":         {"GOBL-ORG-EMAIL-02"},
	"supply-free-trade-zone.xml": {"GOBL-ORG-EMAIL-02"},
}

// faultCodePattern matches the fault codes in a validation error.
var faultCodePattern = regexp.MustCompile(`\[(GOBL-[A-Z0-9-]+)\]`)

// TestImportAEValid checks that the official PINT AE examples import as valid
// GOBL documents.
func TestImportAEValid(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join("testdata", "parse", "pint", "ae", "*.xml"))
	require.NoError(t, err)
	require.NotEmpty(t, examples)
	for _, example := range examples {
		name := filepath.Base(example)
		t.Run(name, func(t *testing.T) {
			env := parseXMLInvoice(t, filepath.Join("pint", "ae", name))
			require.NoError(t, env.Calculate())
			err := env.Validate()
			want, ok := invalidImportsAE[name]
			if !ok {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			var got []string
			for _, m := range faultCodePattern.FindAllStringSubmatch(err.Error(), -1) {
				got = append(got, m[1])
			}
			assert.ElementsMatch(t, want, got)
		})
	}
}

// TestImportAESellerTIN checks that a seller with a TIN and no TRN, as out of
// scope documents allow, keeps the TIN as a tax registration when imported,
// instead of getting it as its tax ID.
func TestImportAESellerTIN(t *testing.T) {
	env := loadTestEnvelope(t, "pint/ae/invoice-ae-outside-scope.json")
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	inv.Supplier.TaxID = &tax.Identity{Country: "AE"}
	inv.Supplier.Identities = append(inv.Supplier.Identities, &org.Identity{Key: ae.IdentityKeyTIN, Code: "1357902468"})
	require.NoError(t, env.Calculate())
	require.NoError(t, env.Validate())

	doc, err := ubl.ExportInvoice(env, ubl.WithFormat(peppolubl.FormatAE))
	require.NoError(t, err)
	out, err := ubl.Import(doc)
	require.NoError(t, err)
	supplier := out.Extract().(*bill.Invoice).Supplier
	assert.Nil(t, supplier.TaxID)
	var tin *org.Identity
	for _, id := range supplier.Identities {
		if id.Key == ae.IdentityKeyTIN {
			tin = id
		}
	}
	require.NotNil(t, tin)
	assert.Equal(t, org.IdentityScopeTax, tin.Scope)
	assert.Equal(t, cbc.Code("1357902468"), tin.Code)
}

// TestExportAE checks that a UAE invoice is exported with the PINT AE
// identifiers and detected as PINT AE again.
func TestExportAE(t *testing.T) {
	env := loadTestEnvelope(t, "pint/ae/invoice-ae.json")
	out, err := convert.Export(env, peppolubl.KeyAE)
	require.NoError(t, err)
	assert.Equal(t, peppolubl.KeyAE, out.Format.Key)
	assert.Contains(t, string(out.Data), "<cbc:CustomizationID>"+peppolubl.FormatAE.CustomizationID)

	f, err := convert.Detect(out.Data)
	require.NoError(t, err)
	assert.Equal(t, peppolubl.KeyAE, f.Key, "detected again")
}

// TestSpecificationAESelfBilling checks that self-billed invoices are written
// with the AE self-billing identifiers and validated against its rule set.
func TestSpecificationAESelfBilling(t *testing.T) {
	env := loadTestEnvelope(t, "pint/ae/invoice-ae.json")
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)

	f := peppolubl.FormatAE
	assert.Equal(t, peppolubl.FormatAE.VESIDs.Invoice, f.VESID(inv))

	inv.SetTags(tax.TagSelfBilled)
	assert.Equal(t, peppolubl.SpecificationAESelfBilling.VESIDs.Invoice, f.VESID(inv))
	s := f.SpecificationFor(inv)
	require.NotNil(t, s)
	assert.Equal(t, peppolubl.SpecificationAESelfBilling.CustomizationID, s.CustomizationID)
	assert.Equal(t, peppolubl.SpecificationAESelfBilling.ProfileID, s.ProfileID)

	inv.Type = bill.InvoiceTypeCreditNote
	assert.Equal(t, peppolubl.SpecificationAESelfBilling.VESIDs.CreditNote, f.VESID(inv))
}
