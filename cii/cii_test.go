package cii_test

import (
	"os"
	"path/filepath"
	"testing"

	goblcii "github.com/invopop/gobl.cii"
	peppolcii "github.com/invopop/gobl.peppol/cii"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/convert"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatBIS(t *testing.T) {
	t.Run("context", func(t *testing.T) {
		env := loadTestEnvelope(t, "bis/invoice-minimal.json")
		doc, err := goblcii.ExportInvoice(env, goblcii.WithFormat(peppolcii.FormatBIS))
		require.NoError(t, err)
		assert.Equal(t, "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0", doc.ExchangedContext.GuidelineContext.ID)
		require.NotNil(t, doc.ExchangedContext.BusinessContext)
		assert.Equal(t, "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0", doc.ExchangedContext.BusinessContext.ID)
	})

	t.Run("notes are joined into one", func(t *testing.T) {
		env := loadTestEnvelope(t, "bis/invoice-minimal.json")
		inv := env.Extract().(*bill.Invoice)
		inv.Notes = []*org.Note{{Text: "First"}, {Text: "Second"}}
		doc, err := goblcii.ExportInvoice(env, goblcii.WithFormat(peppolcii.FormatBIS))
		require.NoError(t, err)
		require.Len(t, doc.ExchangedDocument.IncludedNote, 1)
		assert.Equal(t, "First\n\nSecond", doc.ExchangedDocument.IncludedNote[0].Content)
	})

	t.Run("find format", func(t *testing.T) {
		f := goblcii.FindFormat(peppolcii.FormatBIS.GuidelineID, peppolcii.FormatBIS.BusinessID)
		require.NotNil(t, f)
		assert.Equal(t, peppolcii.KeyBIS, f.Key)

		assert.Nil(t, goblcii.FindFormat(peppolcii.FormatBIS.GuidelineID, "other"))
	})
}

func TestConvertRegister(t *testing.T) {
	t.Run("format", func(t *testing.T) {
		f := convert.FormatFor(peppolcii.KeyBIS)
		require.NotNil(t, f)
		assert.Equal(t, cbc.Key("cii"), f.Syntax)
		assert.True(t, f.Attachments)
	})

	t.Run("export and detect again", func(t *testing.T) {
		env := loadTestEnvelope(t, "bis/invoice-complete.json")
		out, err := convert.Export(env, peppolcii.KeyBIS)
		require.NoError(t, err)
		assert.Equal(t, peppolcii.KeyBIS, out.Format.Key)

		f, err := convert.Detect(out.Data)
		require.NoError(t, err)
		assert.Equal(t, peppolcii.KeyBIS, f.Key)
	})

	t.Run("detect and import", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join("testdata", "parse", "bis", "CII-IN_SE-R-003.xml"))
		require.NoError(t, err)
		res, err := convert.Import(data)
		require.NoError(t, err)
		assert.Equal(t, peppolcii.KeyBIS, res.Format.Key)
		inv, ok := res.Envelope.Extract().(*bill.Invoice)
		require.True(t, ok)
		assert.Contains(t, inv.GetAddons(), en16931.V2017)
	})
}
