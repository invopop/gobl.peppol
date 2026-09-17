package peppol

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	ubl "github.com/invopop/gobl.ubl"
)

// PhormDefaultToken is phorm's stock X-Token, used when NewValidator is given
// an empty token. phorm always requires a matching token but, reachable only
// inside a trusted network, treats it as a shared secret rather than a security
// boundary.
const PhormDefaultToken = "phorm-dev-token"

// ValidationError is a single schematron assertion a document failed.
type ValidationError struct {
	Rule    string
	Field   string
	Message string
}

// Error renders the failed assertion as a single line.
func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Rule, e.Message, e.Field)
	}
	return fmt.Sprintf("%s: %s", e.Rule, e.Message)
}

// Validator runs Peppol PINT A-NZ schematron validation by posting documents to
// a phorm service, which wraps the phive rule engine.
type Validator struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewValidator returns a Validator that posts documents to the phorm service at
// baseURL. An empty token falls back to PhormDefaultToken; a nil client uses
// http.DefaultClient.
func NewValidator(baseURL, token string, client *http.Client) *Validator {
	if token == "" {
		token = PhormDefaultToken
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Validator{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  client,
	}
}

// Validate runs the schematron identified by vesID (one of the VESID*
// constants) over the UBL document and returns every schematron error it fails.
func (v *Validator) Validate(ctx context.Context, vesID string, doc []byte) ([]ValidationError, error) {
	endpoint := v.baseURL + "/api/validate/" + url.PathEscape(vesID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(doc))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Token", v.token)

	res, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("peppol: phorm request: %w", err)
	}
	defer res.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("peppol: reading phorm response: %w", err)
	}

	// phorm answers 200 for a clean document and 400 when the schematron
	// reports errors.
	var result phiveResult
	if jerr := json.Unmarshal(body, &result); jerr != nil {
		return nil, fmt.Errorf("peppol: phorm %s: %s", res.Status, strings.TrimSpace(string(body)))
	}

	var errs []ValidationError
	for _, layer := range result.Results {
		for _, it := range layer.Items {
			if strings.EqualFold(it.ErrorLevel, "ERROR") {
				errs = append(errs, ValidationError{
					Rule:    it.ErrorID,
					Field:   it.ErrorFieldName,
					Message: it.ErrorText,
				})
			}
		}
	}
	return errs, nil
}

// phiveResult is the subset of phive's JSON validation result we consume.
type phiveResult struct {
	Success bool `json:"success"`
	Results []struct {
		Items []struct {
			ErrorLevel     string `json:"errorLevel"`
			ErrorID        string `json:"errorID"`
			ErrorFieldName string `json:"errorFieldName"`
			ErrorText      string `json:"errorText"`
		} `json:"items"`
	} `json:"results"`
}

// VESIDForDocument returns the validation exchange specification id (VESID) for
// a Peppol PINT UBL document.
func VESIDForDocument(doc []byte) (string, error) {
	var probe struct {
		XMLName         xml.Name
		CustomizationID string `xml:"CustomizationID"`
	}
	if err := xml.Unmarshal(doc, &probe); err != nil {
		return "", fmt.Errorf("peppol: parsing document for VESID: %w", err)
	}
	vesIDs := vesIDsForCustomization(probe.CustomizationID)
	if probe.XMLName.Local == "CreditNote" {
		return vesIDs.CreditNote, nil
	}
	return vesIDs.Invoice, nil
}

// vesIDsForCustomization maps a CustomizationID to the rule sets of the context
// that produces it.
func vesIDsForCustomization(customizationID string) ubl.VESIDMapping {
	switch {
	case strings.HasPrefix(customizationID, CustomizationAUNZSelfBilling):
		return ContextAUNZSelfBilled.VESIDs
	case strings.HasPrefix(customizationID, CustomizationAUNZBilling):
		return ContextAUNZ.VESIDs
	case strings.HasPrefix(customizationID, CustomizationPINTBilling):
		return ContextPINT.VESIDs
	default:
		return ContextAUNZ.VESIDs
	}
}
