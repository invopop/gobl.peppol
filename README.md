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

Pass a context option for self-billing, or the wildcard contexts used for SMP
registration:

```go
import ubl "github.com/invopop/gobl.ubl"

doc, err := peppol.ConvertInvoice(env, ubl.WithContext(peppol.ContextPINTSelfBilled))
```
