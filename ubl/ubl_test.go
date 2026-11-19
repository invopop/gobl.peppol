package ubl_test

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	peppolubl "github.com/invopop/gobl.peppol/ubl"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
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

// importFaultsAE are the faults the official PINT AE examples give once
// imported. They pass the PINT AE Schematron, but their data is made up: the
// TRNs and TINs fail the check digit the Federal Tax Authority defines (the
// supplier gets no TIN from such a TRN), and some email addresses are
// placeholders. Any other fault is a defect in the
// import. margin-scheme-credit-note.xml is the official margin scheme
// example turned into a credit note, as there is no official one.
var importFaultsAE = map[string][]string{
	"allowance-exempt.xml":           {"GOBL-AE-TAX-IDENTITY-02"},
	"allowance-reverse-charge.xml":   {"GOBL-AE-TAX-IDENTITY-02"},
	"buyer-pas-identifier.xml":       {"GOBL-AE-TAX-IDENTITY-02"},
	"deemed-supply.xml":              {"GOBL-AE-TAX-IDENTITY-02", "GOBL-ORG-EMAIL-02"},
	"margin-scheme.xml":              {"GOBL-AE-TAX-IDENTITY-02", "GOBL-ORG-EMAIL-02"},
	"margin-scheme-credit-note.xml":  {"GOBL-AE-TAX-IDENTITY-02", "GOBL-ORG-EMAIL-02"},
	"reverse-charge.xml":             {"GOBL-AE-TAX-IDENTITY-02", "GOBL-ORG-EMAIL-02"},
	"seller-pas-identifier.xml":      {"GOBL-AE-TAX-IDENTITY-02"},
	"seller-tin-identifier.xml":      {"GOBL-AE-ORG-IDENTITY-02", "GOBL-AE-TAX-IDENTITY-02"},
	"seller-trn-tin-identifiers.xml": {"GOBL-AE-ORG-IDENTITY-02", "GOBL-AE-TAX-IDENTITY-02"},
	"standard-invoice.xml":           {"GOBL-AE-TAX-IDENTITY-02"},
	"supply-free-trade-zone.xml":     {"GOBL-AE-TAX-IDENTITY-02", "GOBL-ORG-EMAIL-02"},
}

// faultCodePattern matches the fault codes in a validation error.
var faultCodePattern = regexp.MustCompile(`\[(GOBL-[A-Z0-9-]+)\]`)

// TestImportAEFaults checks that the official PINT AE examples import with
// no faults beyond the ones their made-up data gives.
func TestImportAEFaults(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join("testdata", "parse", "pint", "ae", "*.xml"))
	require.NoError(t, err)
	require.Len(t, examples, len(importFaultsAE))
	for _, example := range examples {
		name := filepath.Base(example)
		t.Run(name, func(t *testing.T) {
			env := parseXMLInvoice(t, filepath.Join("pint", "ae", name))
			require.NoError(t, env.Calculate())
			err := env.Validate()
			require.Error(t, err)
			var got []string
			for _, m := range faultCodePattern.FindAllStringSubmatch(err.Error(), -1) {
				got = append(got, m[1])
			}
			assert.ElementsMatch(t, importFaultsAE[name], got)
		})
	}
}

// TestRoundTripAE checks that each AE convert example gives the same XML
// after an import and a second export, so whatever FormatAE writes it also
// reads back, and that the imported invoice is the one exported, but for the
// known differences roundTripKnownDifferences lists.
func TestRoundTripAE(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join("testdata", "convert", "pint", "ae", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, examples)
	for _, example := range examples {
		name := filepath.Base(example)
		t.Run(name, func(t *testing.T) {
			env := loadTestEnvelope(t, filepath.Join("pint", "ae", name))
			doc, err := ubl.ExportInvoice(env, ubl.WithFormat(peppolubl.FormatAE))
			require.NoError(t, err)
			first, err := ubl.Encode(doc)
			require.NoError(t, err)

			in, err := ubl.Decode(first)
			require.NoError(t, err)
			out, err := ubl.Import(in)
			require.NoError(t, err)
			require.NoError(t, out.Calculate())
			require.NoError(t, out.Validate())

			sent, ok := env.Extract().(*bill.Invoice)
			require.True(t, ok)
			got, ok := out.Extract().(*bill.Invoice)
			require.True(t, ok)
			want := cloneInvoice(t, sent)
			got = cloneInvoice(t, got)
			roundTripKnownDifferences(want, got)
			wantJSON, err := json.Marshal(want)
			require.NoError(t, err)
			gotJSON, err := json.Marshal(got)
			require.NoError(t, err)
			assert.JSONEq(t, string(wantJSON), string(gotJSON))

			doc, err = ubl.ExportInvoice(out, ubl.WithFormat(peppolubl.FormatAE))
			require.NoError(t, err)
			second, err := ubl.Encode(doc)
			require.NoError(t, err)

			assert.Equal(t, string(first), string(second))
		})
	}
}

// cloneInvoice provides a deep copy of an invoice.
func cloneInvoice(t *testing.T, inv *bill.Invoice) *bill.Invoice {
	t.Helper()
	data, err := json.Marshal(inv)
	require.NoError(t, err)
	out := new(bill.Invoice)
	require.NoError(t, json.Unmarshal(data, out))
	return out
}

// roundTripKnownDifferences evens out what a GOBL invoice can't get back from
// its UBL, in the invoice sent (want) or the one imported (got):
//   - UBL has a single document number, so the series is joined to the code;
//   - gobl.ubl imports invoices with currency rounding, which the amounts
//     written already follow;
//   - gobl.ubl gives an imported tax registration the party's country;
//   - PINT AE needs a gross price on every line, so an item without a list
//     price comes back with its price as one, and with the default unit code;
//   - the base of a line charge or discount, which GOBL computes, comes back
//     set;
//   - a note's key isn't written, only the UNTDID text subject code the
//     EN 16931 addon gives it;
//   - tags and identities come back in the order UBL holds them.
func roundTripKnownDifferences(want, got *bill.Invoice) {
	joinSeries := func(series, code cbc.Code) cbc.Code {
		if series == "" {
			return code
		}
		return series + "-" + code
	}
	want.Code = joinSeries(want.Series, want.Code)
	want.Series = ""
	for _, ref := range want.Preceding {
		ref.Code = joinSeries(ref.Series, ref.Code)
		ref.Series = ""
	}
	if want.Tax != nil {
		want.Tax.Rounding = tax.RoundingRuleCurrency
	}

	for _, pair := range [][2]*org.Party{{want.Supplier, got.Supplier}, {want.Customer, got.Customer}} {
		if pair[0] == nil || pair[1] == nil {
			continue
		}
		for _, g := range pair[1].Identities {
			for _, w := range pair[0].Identities {
				if g.Code == w.Code && w.Country == "" {
					g.Country = ""
				}
			}
		}
		sortIdentities(pair[0].Identities)
		sortIdentities(pair[1].Identities)
	}

	for i, gl := range got.Lines {
		if i >= len(want.Lines) || gl.Item == nil || want.Lines[i].Item == nil {
			continue
		}
		wi, gi := want.Lines[i].Item, gl.Item
		if wi.List == nil && gi.List != nil && gi.Price != nil && gi.List.Equals(*gi.Price) {
			gi.List = nil
		}
		if !wi.Ext.Has(untdid.ExtKeyUnit) && gi.Ext.Get(untdid.ExtKeyUnit) == "C62" {
			gi.Ext = gi.Ext.Delete(untdid.ExtKeyUnit)
		}
		sortIdentities(wi.Identities)
		sortIdentities(gi.Identities)
		for j, c := range gl.Charges {
			if j < len(want.Lines[i].Charges) && want.Lines[i].Charges[j].Base == nil {
				c.Base = nil
			}
		}
		for j, d := range gl.Discounts {
			if j < len(want.Lines[i].Discounts) && want.Lines[i].Discounts[j].Base == nil {
				d.Base = nil
			}
		}
	}

	clearNoteKeys(want.Notes)
	for _, l := range want.Lines {
		clearNoteKeys(l.Notes)
	}

	for _, inv := range []*bill.Invoice{want, got} {
		tags := slices.Clone(inv.GetTags())
		slices.Sort(tags)
		inv.SetTags(tags...)
	}
}

// clearNoteKeys removes the keys of notes that have a text subject code.
func clearNoteKeys(notes []*org.Note) {
	for _, n := range notes {
		if n.Ext.Has(untdid.ExtKeyTextSubject) {
			n.Key = ""
		}
	}
}

// sortIdentities orders identities by code.
func sortIdentities(ids []*org.Identity) {
	slices.SortFunc(ids, func(a, b *org.Identity) int {
		return strings.Compare(a.Code.String(), b.Code.String())
	})
}

// TestImportAESellerTIN checks that a seller with a TIN and no TRN, as out of
// scope documents allow, keeps the TIN as a tax registration when imported,
// instead of getting it as its tax ID.
func TestImportAESellerTIN(t *testing.T) {
	env := loadTestEnvelope(t, "pint/ae/invoice-ae-outside-scope.json")
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	// The fixture's supplier has its TIN as a tax identity; without the TRN,
	// the TIN is its only tax registration.
	inv.Supplier.TaxID = &tax.Identity{Country: "AE"}
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
		if id.Type == ae.IdentityTypeTIN {
			tin = id
		}
	}
	require.NotNil(t, tin)
	assert.Equal(t, org.IdentityScopeTax, tin.Scope)
	assert.Equal(t, cbc.Code("1357902467"), tin.Code)
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
