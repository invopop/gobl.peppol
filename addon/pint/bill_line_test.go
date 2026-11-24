package pint_test

import (
	"testing"

	"github.com/invopop/gobl.peppol/addon/pint"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillLineRulesAE(t *testing.T) {
	const (
		faultLineVAT         = "GOBL-PEPPOL-PINT-BILL-LINE-02"
		faultLineDescription = "GOBL-PEPPOL-PINT-BILL-LINE-03"
	)

	t.Run("line without a VAT combo is rejected (IBR-145-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Taxes = nil
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultLineVAT)
		assert.Contains(t, out, "$.lines[0].taxes")
	})

	t.Run("item description is required (IBR-125-AE)", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Item.Description = ""
		out := aeFaults(t, inv)
		assert.Contains(t, out, faultLineDescription)
		assert.Contains(t, out, "$.lines[0].item.description")
	})

	t.Run("line rules do not apply outside AE", func(t *testing.T) {
		inv := scenarioInvoice(l10n.AU.Tax(), bill.InvoiceTypeStandard)
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv, tax.AddonContext(pint.V1), tax.RegimeContext(l10n.AU.Tax()))
		if err != nil {
			assert.NotContains(t, err.Error(), faultLineDescription)
		}
	})
}

func TestBillLineRulesAUNZ(t *testing.T) {
	const faultLineGST = "GOBL-PEPPOL-PINT-BILL-LINE-01"
	lineFaults := func(t *testing.T, inv *bill.Invoice) string {
		t.Helper()
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv, tax.AddonContext(pint.V1), tax.RegimeContext(inv.GetRegime()))
		if err == nil {
			return ""
		}
		return err.Error()
	}

	t.Run("line without a GST combo is rejected (ALIGNED-IBRP-050-AUNZ)", func(t *testing.T) {
		inv := scenarioInvoice(l10n.AU.Tax(), bill.InvoiceTypeStandard)
		inv.Lines[0].Taxes = nil
		assert.Contains(t, lineFaults(t, inv), faultLineGST)
	})

	t.Run("line with a GST combo is fine", func(t *testing.T) {
		assert.NotContains(t, lineFaults(t, scenarioInvoice(l10n.AU.Tax(), bill.InvoiceTypeStandard)), faultLineGST)
	})

	t.Run("the GST rule does not apply outside A-NZ", func(t *testing.T) {
		inv := aeDocument(bill.InvoiceTypeStandard)
		assert.NotContains(t, aeFaults(t, inv), faultLineGST)
	})
}

func TestBillGoodsTypeRulesAE(t *testing.T) {
	const (
		faultGoodsType      = "GOBL-PEPPOL-PINT-BILL-LINE-04"
		faultGoodsTypeValue = "GOBL-PEPPOL-PINT-BILL-LINE-05"
		faultGTIN           = "GOBL-PEPPOL-PINT-BILL-LINE-06"
	)
	gtin := func(scheme cbc.Code) []*org.Identity {
		return []*org.Identity{{Code: "4006381333931", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: scheme})}}
	}
	reverseCharge := func(goodsType cbc.Code, ids []*org.Identity) *bill.Invoice {
		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Taxes = tax.Set{lineCombo(tax.KeyReverseCharge)}
		inv.Lines[0].Item.Identities = ids
		if goodsType != "" {
			inv.Lines[0].Item.Ext = tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyGoodsType: goodsType})
		}
		return inv
	}

	t.Run("reverse charge item needs the goods type (IBR-166-AE)", func(t *testing.T) {
		assert.Contains(t, aeFaults(t, reverseCharge("", gtin("0160"))), faultGoodsType)
		out := aeFaults(t, reverseCharge("DL8.48.3.1", gtin("0160")))
		assert.NotContains(t, out, faultGoodsType)
		assert.NotContains(t, out, faultGoodsTypeValue)
		assert.NotContains(t, out, faultGTIN)
	})

	t.Run("goods type must be from the AE list on any line (IBR-006-AE)", func(t *testing.T) {
		assert.NotContains(t, aeFaults(t, reverseCharge("DL8.48.8.3", gtin("0160"))), faultGoodsTypeValue)
		assert.Contains(t, aeFaults(t, reverseCharge("DL8.48.9", gtin("0160"))), faultGoodsTypeValue)

		inv := aeDocument(bill.InvoiceTypeStandard)
		inv.Lines[0].Item.Ext = tax.ExtensionsOf(cbc.CodeMap{pint.ExtKeyGoodsType: "OTHER"})
		assert.Contains(t, aeFaults(t, inv), faultGoodsTypeValue)
	})

	t.Run("reverse charge item needs a GTIN (IBR-174-AE)", func(t *testing.T) {
		assert.Contains(t, aeFaults(t, reverseCharge("DL8.48.3.1", nil)), faultGTIN)
		assert.Contains(t, aeFaults(t, reverseCharge("DL8.48.3.1", gtin("0088"))), faultGTIN)
	})
}
