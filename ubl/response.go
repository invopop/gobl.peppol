package ubl

import (
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
)

// Peppol Invoice Response code-list identifiers for the StatusReasonCode.
const (
	statusReasonListID = "OPStatusReason"
	statusActionListID = "OPStatusAction"
)

// Peppol Invoice Response document-type codes (UNCL1001) for the referenced
// document, which T111 makes mandatory.
const (
	documentTypeCodeListID     = "UNCL1001"
	documentTypeCodeInvoice    = "380"
	documentTypeCodeCreditNote = "381"
)

// responseCodes maps GOBL status events to the Peppol Invoice Response status
// codes (UNCL4343 subset, transaction T111). A technical error belongs in the
// Message Level Response, but the Invoice Response has no code for it, so it
// falls back to RE with the detail in the status clarification.
var responseCodes = map[cbc.Key]string{
	bill.StatusLineAcknowledged: "AB",
	bill.StatusLineProcessing:   "IP",
	bill.StatusLineQuerying:     "UQ",
	bill.StatusLineRejected:     "RE",
	bill.StatusLineAccepted:     "AP",
	bill.StatusLinePaid:         "PD",
	bill.StatusLineError:        "RE",
}

// responseEvents reverses responseCodes for imports, mapping RE back to the
// business rejection and the conditional acceptance CA to accepted, with its
// conditions carried in the status line's reasons and actions.
var responseEvents = map[string]cbc.Key{
	"AB": bill.StatusLineAcknowledged,
	"IP": bill.StatusLineProcessing,
	"UQ": bill.StatusLineQuerying,
	"RE": bill.StatusLineRejected,
	"AP": bill.StatusLineAccepted,
	"PD": bill.StatusLinePaid,
	"CA": bill.StatusLineAccepted,
}

// statusReasonCodes maps GOBL reason keys to OPStatusReason codes.
var statusReasonCodes = map[cbc.Key]string{
	bill.ReasonKeyNone:            "NON",
	bill.ReasonKeyReferences:      "REF",
	bill.ReasonKeyLegal:           "LEG",
	bill.ReasonKeyUnknownReceiver: "REC",
	bill.ReasonKeyQuality:         "QUA",
	bill.ReasonKeyDelivery:        "DEL",
	bill.ReasonKeyPrices:          "PRI",
	bill.ReasonKeyQuantity:        "QTY",
	bill.ReasonKeyItems:           "ITM",
	bill.ReasonKeyPaymentTerms:    "PAY",
	bill.ReasonKeyNotRecognized:   "UNR",
	bill.ReasonKeyFinanceTerms:    "FIN",
	bill.ReasonKeyPartial:         "PPD",
	bill.ReasonKeyOther:           "OTH",
}

// statusActionCodes maps GOBL action keys to OPStatusAction codes.
var statusActionCodes = map[cbc.Key]string{
	bill.ActionKeyNone:          "NOA",
	bill.ActionKeyProvide:       "PIN",
	bill.ActionKeyReissue:       "NIN",
	bill.ActionKeyCreditFull:    "CNF",
	bill.ActionKeyCreditPartial: "CNP",
	bill.ActionKeyCreditAmount:  "CNA",
	bill.ActionKeyOther:         "OTH",
}

// exportDocumentResponse stamps the UNCL4343 response code onto a document
// response, along with one status per reason and action carrying its
// OPStatusReason or OPStatusAction clarification, and the referenced
// document's type.
func exportDocumentResponse(dr *goblubl.DocumentResponse, line *bill.StatusLine) {
	resp := dr.Response
	if code := responseCodes[line.Key]; code != "" {
		resp.ResponseCode = &goblubl.IDType{Value: code}
	}
	for _, r := range line.Reasons {
		if r == nil {
			continue
		}
		if rc := statusReasonCodes[r.Key]; rc != "" {
			resp.Status = append(resp.Status, newStatus(statusReasonListID, rc, r.Description))
		}
	}
	for _, a := range line.Actions {
		if a == nil {
			continue
		}
		if ac := statusActionCodes[a.Key]; ac != "" {
			resp.Status = append(resp.Status, newStatus(statusActionListID, ac, a.Description))
		}
	}
	if dr.DocumentReference != nil {
		docType := documentTypeCodeInvoice
		if line.Doc != nil && line.Doc.Type == bill.InvoiceTypeCreditNote {
			docType = documentTypeCodeCreditNote
		}
		listID := documentTypeCodeListID
		dr.DocumentReference.DocumentTypeCode = &goblubl.IDType{ListID: &listID, Value: docType}
	}
}

// importStatusLine maps the Peppol Invoice Response codes of a document
// response back to the status line's event, reasons, actions and referenced
// document type.
func importStatusLine(line *bill.StatusLine, dr *goblubl.DocumentResponse) {
	r := dr.Response
	if r == nil {
		return
	}
	if r.ResponseCode != nil {
		line.Key = responseEvents[r.ResponseCode.Value]
	}
	for _, s := range r.Status {
		if s == nil || s.StatusReasonCode == nil {
			continue
		}
		listID := ""
		if s.StatusReasonCode.ListID != nil {
			listID = *s.StatusReasonCode.ListID
		}
		desc := ""
		if len(s.StatusReason) > 0 {
			desc = s.StatusReason[0]
		}
		switch listID {
		case statusReasonListID:
			if key := keyForCode(statusReasonCodes, s.StatusReasonCode.Value); key != "" {
				line.Reasons = append(line.Reasons, &bill.Reason{Key: key, Description: desc})
			}
		case statusActionListID:
			if key := keyForCode(statusActionCodes, s.StatusReasonCode.Value); key != "" {
				line.Actions = append(line.Actions, &bill.Action{Key: key, Description: desc})
			}
		}
	}
	if ref := dr.DocumentReference; ref != nil && line.Doc != nil &&
		ref.DocumentTypeCode != nil && ref.DocumentTypeCode.Value == documentTypeCodeCreditNote {
		line.Doc.Type = bill.InvoiceTypeCreditNote
	}
}

// trimToResponseParty reduces a party to the elements the Peppol Invoice
// Response (T111) permits on the sender and receiver: EndpointID,
// PartyIdentification, PartyLegalEntity and Contact.
func trimToResponseParty(p *goblubl.Party) {
	if p == nil {
		return
	}
	p.PartyName = nil
	p.PostalAddress = nil
	p.PartyTaxScheme = nil
}

// newStatus builds a status clarification with the code from the list and an
// optional reason.
func newStatus(listID, code, reason string) *goblubl.Status {
	s := &goblubl.Status{StatusReasonCode: &goblubl.IDType{ListID: &listID, Value: code}}
	if reason != "" {
		s.StatusReason = []string{reason}
	}
	return s
}

// keyForCode provides the GOBL key mapped to the code, or empty if none is.
func keyForCode(m map[cbc.Key]string, code string) cbc.Key {
	for k, v := range m {
		if v == code {
			return k
		}
	}
	return ""
}
