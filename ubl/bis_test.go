package ubl_test

import (
	"testing"

	"github.com/invopop/gobl"
	peppolubl "github.com/invopop/gobl.peppol/ubl"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/convert"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatBIS(t *testing.T) {
	t.Run("billing", func(t *testing.T) {
		env := loadTestEnvelope(t, "bis/invoice-minimal.json")
		doc, err := ubl.ExportInvoice(env, ubl.WithFormat(peppolubl.FormatBIS))
		require.NoError(t, err)
		assert.Equal(t, peppolubl.FormatBIS.CustomizationID, doc.CustomizationID)
		require.NotNil(t, doc.ProfileID)
		assert.Equal(t, peppolubl.FormatBIS.ProfileID, doc.ProfileID.Value)
	})

	t.Run("self-billing", func(t *testing.T) {
		env := loadTestEnvelope(t, "bis/self-billed-invoice.json")
		doc, err := ubl.ExportInvoice(env, ubl.WithFormat(peppolubl.FormatBIS))
		require.NoError(t, err)
		assert.Equal(t, peppolubl.SpecificationBISSelfBilling.CustomizationID, doc.CustomizationID)
		require.NotNil(t, doc.ProfileID)
		assert.Equal(t, peppolubl.SpecificationBISSelfBilling.ProfileID, doc.ProfileID.Value)
	})

	t.Run("notes are joined into one", func(t *testing.T) {
		env := loadTestEnvelope(t, "bis/invoice-minimal.json")
		inv := env.Extract().(*bill.Invoice)
		inv.Notes = []*org.Note{{Text: "First"}, {Text: "Second"}}
		doc, err := ubl.ExportInvoice(env, ubl.WithFormat(peppolubl.FormatBIS))
		require.NoError(t, err)
		assert.Equal(t, []string{"First\n\nSecond"}, doc.Note)
	})

	t.Run("VESIDs", func(t *testing.T) {
		f := peppolubl.FormatBIS
		assert.Equal(t, peppolubl.FormatBIS.VESIDs.Invoice, f.VESID(loadTestEnvelope(t, "bis/invoice-minimal.json").Extract()))
		assert.Equal(t, peppolubl.FormatBIS.VESIDs.CreditNote, f.VESID(loadTestEnvelope(t, "bis/credit-note.json").Extract()))
		assert.Equal(t, peppolubl.SpecificationBISSelfBilling.VESIDs.Invoice, f.VESID(loadTestEnvelope(t, "bis/self-billed-invoice.json").Extract()))
		assert.Equal(t, peppolubl.SpecificationBISSelfBilling.VESIDs.CreditNote, f.VESID(loadTestEnvelope(t, "bis/credit-note-self-billed.json").Extract()))
		assert.Equal(t, peppolubl.SpecificationBISInvoiceResponse.VESIDs.Status, f.VESID(baseStatus()))
		assert.Empty(t, f.VESID(&bill.Payment{}))
	})

	t.Run("find format", func(t *testing.T) {
		for name, ids := range map[string][2]string{
			"billing":          {peppolubl.FormatBIS.CustomizationID, peppolubl.FormatBIS.ProfileID},
			"self-billing":     {peppolubl.SpecificationBISSelfBilling.CustomizationID, peppolubl.SpecificationBISSelfBilling.ProfileID},
			"invoice response": {peppolubl.SpecificationBISInvoiceResponse.CustomizationID, peppolubl.SpecificationBISInvoiceResponse.ProfileID},
		} {
			t.Run(name, func(t *testing.T) {
				f := ubl.FindFormat(ids[0], ids[1])
				require.NotNil(t, f)
				assert.Equal(t, peppolubl.KeyBIS, f.Key)
			})
		}
	})
}

func TestFormatBISConvert(t *testing.T) {
	statusEnv := func(t *testing.T) *gobl.Envelope {
		t.Helper()
		env, err := gobl.Envelop(baseStatus())
		require.NoError(t, err)
		return env
	}
	for name, tc := range map[string]struct {
		env    *gobl.Envelope
		spec   string
		addons bool
	}{
		"invoice":             {loadTestEnvelope(t, "bis/invoice-minimal.json"), peppolubl.FormatBIS.CustomizationID, true},
		"self-billed invoice": {loadTestEnvelope(t, "bis/self-billed-invoice.json"), peppolubl.SpecificationBISSelfBilling.CustomizationID, true},
		"status":              {statusEnv(t), peppolubl.SpecificationBISInvoiceResponse.CustomizationID, false},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := convert.Export(tc.env, peppolubl.KeyBIS)
			require.NoError(t, err)
			assert.Contains(t, string(out.Data), "<cbc:CustomizationID>"+tc.spec+"</cbc:CustomizationID>")

			f, err := convert.Detect(out.Data)
			require.NoError(t, err)
			assert.Equal(t, peppolubl.KeyBIS, f.Key)

			env, err := convert.Import(out.Data)
			require.NoError(t, err)
			doc := env.Extract().(interface{ GetAddons() []cbc.Key })
			if tc.addons {
				assert.Contains(t, doc.GetAddons(), en16931.V2017)
			} else {
				assert.Empty(t, doc.GetAddons())
			}
		})
	}
}

// baseStatus provides a minimal response status with a single line.
func baseStatus() *bill.Status {
	return &bill.Status{
		Type:      bill.StatusTypeResponse,
		Code:      "RESP-RT",
		IssueDate: cal.MakeDate(2026, 5, 29),
		Supplier:  &org.Party{Name: "Seller Co"},
		Customer:  &org.Party{Name: "Buyer Co"},
		Lines:     []*bill.StatusLine{{Index: 1, Doc: &org.DocumentRef{Code: "INV-RT"}}},
	}
}

// exportInvoiceResponse converts a status into a Peppol Invoice Response.
func exportInvoiceResponse(t *testing.T, st *bill.Status) *ubl.ApplicationResponse {
	t.Helper()
	env, err := gobl.Envelop(st)
	require.NoError(t, err)
	doc, err := ubl.Export(env, ubl.WithFormat(peppolubl.FormatBIS))
	require.NoError(t, err)
	ar, ok := doc.(*ubl.ApplicationResponse)
	require.True(t, ok)
	return ar
}

// importInvoiceResponse parses a Peppol Invoice Response into a status.
func importInvoiceResponse(t *testing.T, data []byte) *bill.Status {
	t.Helper()
	doc, err := ubl.Decode(data)
	require.NoError(t, err)
	ar, ok := doc.(*ubl.ApplicationResponse)
	require.True(t, ok)
	env, err := ubl.Import(ar)
	require.NoError(t, err)
	st, ok := env.Extract().(*bill.Status)
	require.True(t, ok)
	return st
}

// roundTripInvoiceResponse converts a status into a Peppol Invoice Response,
// encodes it, and parses it back.
func roundTripInvoiceResponse(t *testing.T, st *bill.Status) *bill.Status {
	t.Helper()
	data, err := ubl.Encode(exportInvoiceResponse(t, st))
	require.NoError(t, err)
	return importInvoiceResponse(t, data)
}

func TestExportInvoiceResponse(t *testing.T) {
	t.Run("codes and clarifications", func(t *testing.T) {
		st := baseStatus()
		st.Lines[0].Key = bill.StatusLineRejected
		st.Lines[0].Reasons = []*bill.Reason{{Key: bill.ReasonKeyReferences, Description: "missing PO"}}
		st.Lines[0].Actions = []*bill.Action{{Key: bill.ActionKeyReissue, Description: "please reissue"}}
		ar := exportInvoiceResponse(t, st)

		assert.Equal(t, peppolubl.SpecificationBISInvoiceResponse.CustomizationID, ar.CustomizationID)
		require.NotNil(t, ar.ProfileID)
		assert.Equal(t, peppolubl.SpecificationBISInvoiceResponse.ProfileID, ar.ProfileID.Value)
		assert.Empty(t, ar.UBLVersionID)

		require.Len(t, ar.DocumentResponse, 1)
		dr := ar.DocumentResponse[0]
		resp := dr.Response
		require.NotNil(t, resp.ResponseCode)
		assert.Equal(t, "RE", resp.ResponseCode.Value)
		assert.Empty(t, resp.ReferenceID)
		assert.Empty(t, resp.Description)

		require.Len(t, resp.Status, 2)
		require.NotNil(t, resp.Status[0].StatusReasonCode.ListID)
		assert.Equal(t, "OPStatusReason", *resp.Status[0].StatusReasonCode.ListID)
		assert.Equal(t, "REF", resp.Status[0].StatusReasonCode.Value)
		assert.Equal(t, []string{"missing PO"}, resp.Status[0].StatusReason)
		assert.Equal(t, "OPStatusAction", *resp.Status[1].StatusReasonCode.ListID)
		assert.Equal(t, "NIN", resp.Status[1].StatusReasonCode.Value)
		assert.Equal(t, []string{"please reissue"}, resp.Status[1].StatusReason)

		require.NotNil(t, dr.DocumentReference.DocumentTypeCode)
		require.NotNil(t, dr.DocumentReference.DocumentTypeCode.ListID)
		assert.Equal(t, "UNCL1001", *dr.DocumentReference.DocumentTypeCode.ListID)
		assert.Equal(t, "380", dr.DocumentReference.DocumentTypeCode.Value)
	})

	t.Run("parties are trimmed", func(t *testing.T) {
		st := baseStatus()
		st.Customer.Addresses = []*org.Address{{Street: "Main St", Locality: "Madrid", Country: "ES"}}
		ar := exportInvoiceResponse(t, st)
		require.NotNil(t, ar.SenderParty)
		assert.Nil(t, ar.SenderParty.PartyName)
		assert.Nil(t, ar.SenderParty.PostalAddress)
		assert.Nil(t, ar.SenderParty.PartyTaxScheme)
	})

	t.Run("error maps to rejected", func(t *testing.T) {
		st := baseStatus()
		st.Lines[0].Key = bill.StatusLineError
		st.Lines[0].Reasons = []*bill.Reason{{Key: bill.ReasonKeyOther, Description: "system failure"}}
		ar := exportInvoiceResponse(t, st)
		require.Len(t, ar.DocumentResponse, 1)
		require.NotNil(t, ar.DocumentResponse[0].Response.ResponseCode)
		assert.Equal(t, "RE", ar.DocumentResponse[0].Response.ResponseCode.Value)
	})

	t.Run("validates against the schematron", func(t *testing.T) {
		pc := phormClient(t)
		st := baseStatus()
		st.Supplier.Inboxes = []*org.Inbox{{Scheme: "9920", Code: "B98602642"}}
		st.Customer.Inboxes = []*org.Inbox{{Scheme: "9920", Code: "A39200019"}}
		st.Lines[0].Key = bill.StatusLineRejected
		st.Lines[0].Reasons = []*bill.Reason{{Key: bill.ReasonKeyReferences, Description: "missing PO"}}
		data, err := ubl.Encode(exportInvoiceResponse(t, st))
		require.NoError(t, err)
		validateXML(t, pc, peppolubl.SpecificationBISInvoiceResponse.VESIDs.Status, data)
	})
}

const sampleInvoiceResponse = `<?xml version="1.0" encoding="UTF-8"?>
<ApplicationResponse xmlns="urn:oasis:names:specification:ubl:schema:xsd:ApplicationResponse-2" xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2" xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
  <cbc:CustomizationID>urn:fdc:peppol.eu:poacc:trns:invoice_response:3</cbc:CustomizationID>
  <cbc:ProfileID>urn:fdc:peppol.eu:poacc:bis:invoice_response:3</cbc:ProfileID>
  <cbc:ID>RESP-9</cbc:ID>
  <cbc:IssueDate>2026-05-29</cbc:IssueDate>
  <cac:SenderParty><cac:PartyName><cbc:Name>Buyer Co</cbc:Name></cac:PartyName></cac:SenderParty>
  <cac:ReceiverParty><cac:PartyName><cbc:Name>Seller Co</cbc:Name></cac:PartyName></cac:ReceiverParty>
  <cac:DocumentResponse>
    <cac:Response>
      <cbc:ResponseCode>RE</cbc:ResponseCode>
      <cac:Status>
        <cbc:StatusReasonCode listID="OPStatusReason">REF</cbc:StatusReasonCode>
        <cbc:StatusReason>missing PO</cbc:StatusReason>
      </cac:Status>
      <cac:Status>
        <cbc:StatusReasonCode listID="OPStatusAction">NIN</cbc:StatusReasonCode>
        <cbc:StatusReason>please reissue</cbc:StatusReason>
      </cac:Status>
    </cac:Response>
    <cac:DocumentReference><cbc:ID>INV-9</cbc:ID></cac:DocumentReference>
  </cac:DocumentResponse>
</ApplicationResponse>`

const sampleConditionallyAccepted = `<?xml version="1.0" encoding="UTF-8"?>
<ApplicationResponse xmlns="urn:oasis:names:specification:ubl:schema:xsd:ApplicationResponse-2" xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2" xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
  <cbc:CustomizationID>urn:fdc:peppol.eu:poacc:trns:invoice_response:3</cbc:CustomizationID>
  <cbc:ProfileID>urn:fdc:peppol.eu:poacc:bis:invoice_response:3</cbc:ProfileID>
  <cbc:ID>RESP-CA</cbc:ID>
  <cbc:IssueDate>2026-05-29</cbc:IssueDate>
  <cac:SenderParty><cac:PartyName><cbc:Name>Buyer Co</cbc:Name></cac:PartyName></cac:SenderParty>
  <cac:ReceiverParty><cac:PartyName><cbc:Name>Seller Co</cbc:Name></cac:PartyName></cac:ReceiverParty>
  <cac:DocumentResponse>
    <cac:Response>
      <cbc:ResponseCode>CA</cbc:ResponseCode>
      <cac:Status>
        <cbc:StatusReasonCode listID="OPStatusReason">PRI</cbc:StatusReasonCode>
        <cbc:StatusReason>price to be confirmed</cbc:StatusReason>
      </cac:Status>
    </cac:Response>
    <cac:DocumentReference><cbc:ID>INV-CA</cbc:ID></cac:DocumentReference>
  </cac:DocumentResponse>
</ApplicationResponse>`

func TestImportInvoiceResponse(t *testing.T) {
	t.Run("codes and clarifications", func(t *testing.T) {
		st := importInvoiceResponse(t, []byte(sampleInvoiceResponse))
		require.Len(t, st.Lines, 1)
		assert.Equal(t, bill.StatusLineRejected, st.Lines[0].Key)
		require.Len(t, st.Lines[0].Reasons, 1)
		assert.Equal(t, bill.ReasonKeyReferences, st.Lines[0].Reasons[0].Key)
		assert.Equal(t, "missing PO", st.Lines[0].Reasons[0].Description)
		require.Len(t, st.Lines[0].Actions, 1)
		assert.Equal(t, bill.ActionKeyReissue, st.Lines[0].Actions[0].Key)
		assert.Equal(t, "please reissue", st.Lines[0].Actions[0].Description)
	})

	t.Run("conditionally accepted", func(t *testing.T) {
		st := importInvoiceResponse(t, []byte(sampleConditionallyAccepted))
		require.Len(t, st.Lines, 1)
		assert.Equal(t, bill.StatusLineAccepted, st.Lines[0].Key)
		require.Len(t, st.Lines[0].Reasons, 1)
		assert.Equal(t, bill.ReasonKeyPrices, st.Lines[0].Reasons[0].Key)
		assert.Equal(t, "price to be confirmed", st.Lines[0].Reasons[0].Description)
	})
}

func TestInvoiceResponseRoundTrip(t *testing.T) {
	t.Run("response codes", func(t *testing.T) {
		for _, ev := range []cbc.Key{
			bill.StatusLineAcknowledged,
			bill.StatusLineProcessing,
			bill.StatusLineQuerying,
			bill.StatusLineRejected,
			bill.StatusLineAccepted,
			bill.StatusLinePaid,
		} {
			t.Run(ev.String(), func(t *testing.T) {
				st := baseStatus()
				st.Lines[0].Key = ev
				st.Lines[0].Reasons = []*bill.Reason{{Key: bill.ReasonKeyOther, Description: "x"}}
				out := roundTripInvoiceResponse(t, st)
				require.Len(t, out.Lines, 1)
				assert.Equal(t, ev, out.Lines[0].Key)
			})
		}
	})

	t.Run("status reasons", func(t *testing.T) {
		for _, rk := range []cbc.Key{
			bill.ReasonKeyNone,
			bill.ReasonKeyReferences,
			bill.ReasonKeyLegal,
			bill.ReasonKeyUnknownReceiver,
			bill.ReasonKeyQuality,
			bill.ReasonKeyDelivery,
			bill.ReasonKeyPrices,
			bill.ReasonKeyQuantity,
			bill.ReasonKeyItems,
			bill.ReasonKeyPaymentTerms,
			bill.ReasonKeyNotRecognized,
			bill.ReasonKeyFinanceTerms,
			bill.ReasonKeyPartial,
			bill.ReasonKeyOther,
		} {
			t.Run(rk.String(), func(t *testing.T) {
				st := baseStatus()
				st.Lines[0].Key = bill.StatusLineRejected
				st.Lines[0].Reasons = []*bill.Reason{{Key: rk, Description: "d"}}
				out := roundTripInvoiceResponse(t, st)
				require.Len(t, out.Lines, 1)
				require.Len(t, out.Lines[0].Reasons, 1)
				assert.Equal(t, rk, out.Lines[0].Reasons[0].Key)
			})
		}
	})

	t.Run("status actions", func(t *testing.T) {
		for _, ak := range []cbc.Key{
			bill.ActionKeyNone,
			bill.ActionKeyProvide,
			bill.ActionKeyReissue,
			bill.ActionKeyCreditFull,
			bill.ActionKeyCreditPartial,
			bill.ActionKeyCreditAmount,
			bill.ActionKeyOther,
		} {
			t.Run(ak.String(), func(t *testing.T) {
				st := baseStatus()
				st.Lines[0].Key = bill.StatusLineRejected
				st.Lines[0].Actions = []*bill.Action{{Key: ak, Description: "d"}}
				out := roundTripInvoiceResponse(t, st)
				require.Len(t, out.Lines, 1)
				require.Len(t, out.Lines[0].Actions, 1)
				assert.Equal(t, ak, out.Lines[0].Actions[0].Key)
			})
		}
	})

	t.Run("credit note reference", func(t *testing.T) {
		st := baseStatus()
		st.Lines[0].Key = bill.StatusLineRejected
		st.Lines[0].Doc.Type = bill.InvoiceTypeCreditNote
		st.Lines[0].Reasons = []*bill.Reason{{Key: bill.ReasonKeyPrices, Description: "price off"}}
		st.Lines[0].Actions = []*bill.Action{{Key: bill.ActionKeyReissue, Description: "redo"}}
		out := roundTripInvoiceResponse(t, st)

		require.Len(t, out.Lines, 1)
		l := out.Lines[0]
		assert.Equal(t, bill.StatusLineRejected, l.Key)
		require.NotNil(t, l.Doc)
		assert.Equal(t, bill.InvoiceTypeCreditNote, l.Doc.Type)
		require.Len(t, l.Reasons, 1)
		assert.Equal(t, "price off", l.Reasons[0].Description)
		require.Len(t, l.Actions, 1)
		assert.Equal(t, "redo", l.Actions[0].Description)
	})
}

// TestFormatBISSelfBilledTag checks the self-billing specification follows
// the invoice's self-billed tag.
func TestFormatBISSelfBilledTag(t *testing.T) {
	env := loadTestEnvelope(t, "bis/invoice-minimal.json")
	inv := env.Extract().(*bill.Invoice)
	assert.Equal(t, peppolubl.FormatBIS.CustomizationID, peppolubl.FormatBIS.SpecificationFor(inv).CustomizationID)
	inv.SetTags(tax.TagSelfBilled)
	assert.Equal(t, peppolubl.SpecificationBISSelfBilling.CustomizationID, peppolubl.FormatBIS.SpecificationFor(inv).CustomizationID)
}
