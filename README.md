# GOBL ➡️ Peppol

Peppol PINT support for [GOBL](https://github.com/invopop/gobl): the tax addons
plus a UBL converter built on [`gobl.ubl`](https://github.com/invopop/gobl.ubl)'s
EN 16931 base.

Released under the Apache 2.0 [LICENSE](https://github.com/invopop/gobl.peppol/blob/main/LICENSE), Copyright 2026 [Invopop S.L.](https://invopop.com).

[![Lint](https://github.com/invopop/gobl.peppol/actions/workflows/lint.yaml/badge.svg)](https://github.com/invopop/gobl.peppol/actions/workflows/lint.yaml)
[![Test Go](https://github.com/invopop/gobl.peppol/actions/workflows/test.yaml/badge.svg)](https://github.com/invopop/gobl.peppol/actions/workflows/test.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/invopop/gobl.peppol)](https://goreportcard.com/report/github.com/invopop/gobl.peppol)
[![GoDoc](https://godoc.org/github.com/invopop/gobl.peppol?status.svg)](https://godoc.org/github.com/invopop/gobl.peppol)
![Latest Tag](https://img.shields.io/github/v/tag/invopop/gobl.peppol)

This module implements the [Peppol International (PINT)](https://docs.peppol.eu/poac/pint/)
billing model as a set of GOBL tax addons:

- **PINT** (`peppol-pint-v1`) — the base Peppol International billing model, the
  common layer shared by the country-specific PINT specifications. It builds on
  EN 16931 but recognises indirect tax schemes beyond VAT (such as GST).
- **A-NZ** (`peppol-pint-aunz-v1`) — the Australia/New Zealand jurisdiction
  extension of PINT, adding the requirement that Australian parties carry their
  Australian Business Number (ABN) and New Zealand parties their New Zealand
  Business Number (NZBN) as a legal registration identity.

The country-specific addons require the base PINT addon; a document must declare
the specific addon that applies to it.

## Usage

Import the aggregator package for its side effects to register every addon:

```go
import _ "github.com/invopop/gobl.peppol/addon"
```

Or import an individual addon directly:

```go
import _ "github.com/invopop/gobl.peppol/addon/aunz"
```

Then declare the addon on the document:

```yaml
$addons:
  - "peppol-pint-aunz-v1"
```

See the [`examples`](./examples) directory for complete invoices.

## Conversion

The root package converts a GOBL envelope to a Peppol PINT A-NZ UBL document,
reusing `gobl.ubl`'s EN 16931 conversion and stamping the A-NZ customization,
profile and VESID identifiers:

```go
import peppol "github.com/invopop/gobl.peppol"

doc, err := peppol.ConvertInvoice(env)            // billing context
data, err := peppol.Bytes(doc)                    // indented UBL XML
```

Pass a context option for another customization, or the wildcard contexts used
for SMP registration:

```go
import ubl "github.com/invopop/gobl.ubl"

doc, err := peppol.ConvertInvoice(env, ubl.WithContext(peppol.ContextAUNZSelfBilled))
```

| Context | CustomizationID (`ibt-024`) | ProfileID (`ibt-023`) |
| --- | --- | --- |
| `ContextPINT` | `urn:peppol:pint:billing-1` | `urn:peppol:bis:billing` |
| `ContextAUNZ` | `urn:peppol:pint:billing-1@aunz-1` | `urn:peppol:bis:billing` |
| `ContextAUNZSelfBilled` | `urn:peppol:pint:selfbilling-1@aunz-1` | `urn:peppol:bis:selfbilling` |

Each one's Peppol document type identifier is
`urn:oasis:names:specification:ubl:schema:xsd:Invoice-2::Invoice##<CustomizationID>::2.1`,
with `CreditNote-2::CreditNote` replacing `Invoice-2::Invoice` for credit notes.
`ContextPINT` is the jurisdiction-neutral base: it stamps the plain PINT
customization and validates against the base PINT rule set rather than the A-NZ
one, which stays the module default.

## Validation

A converted document can be checked against the Peppol PINT A-NZ schematron.
The schematron is XSLT 2.0, so validation runs in a [phive](https://github.com/phax/phive)
rule engine fronted by a [phorm](https://github.com/invopop/phorm) HTTP service
rather than in-process:

```go
v := peppol.NewValidator("http://phorm:8080", "", nil)      // token "" => default
vesID, _ := peppol.VESIDForDocument(data)                   // pick VESID from the XML
findings, err := v.Validate(ctx, vesID, data)               // err only if it couldn't run
for _, f := range findings {                                // each is a schematron failure
	log.Printf("%s", f)
}
```

`Validate` returns the failed schematron assertions; `err` is non-nil only when
validation could not be run (service unreachable or an unexpected response).

The `TestSchematron` test validates every document under `test/data/convert/out`
and `test/data/parse` against the schematron. Like `-update`, it is gated behind
a flag and off by default, so a plain `go test ./...` needs no phorm service:

```sh
go test -validate ./...            # posts to $PHORM_URL, default http://localhost:8080
```

CI runs it against a `phelger/phorm:latest` service container (see
`.github/workflows/test.yaml`).
