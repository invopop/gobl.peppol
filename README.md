# GOBL ➡️ Peppol

Peppol BIS and PINT support for [GOBL](https://github.com/invopop/gobl): the tax addons
plus the UBL and CII formats built on [`gobl.ubl`](https://github.com/invopop/gobl.ubl)'s
and [`gobl.cii`](https://github.com/invopop/gobl.cii)'s base import and export.

Released under the Apache 2.0 [LICENSE](https://github.com/invopop/gobl.peppol/blob/main/LICENSE), Copyright 2026 [Invopop S.L.](https://invopop.com).

[![Lint](https://github.com/invopop/gobl.peppol/actions/workflows/lint.yaml/badge.svg)](https://github.com/invopop/gobl.peppol/actions/workflows/lint.yaml)
[![Test Go](https://github.com/invopop/gobl.peppol/actions/workflows/test.yaml/badge.svg)](https://github.com/invopop/gobl.peppol/actions/workflows/test.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/invopop/gobl.peppol)](https://goreportcard.com/report/github.com/invopop/gobl.peppol)
[![GoDoc](https://godoc.org/github.com/invopop/gobl.peppol?status.svg)](https://godoc.org/github.com/invopop/gobl.peppol)
![Latest Tag](https://img.shields.io/github/v/tag/invopop/gobl.peppol)

This module implements the Peppol billing models as a set of GOBL tax addons:

- **BIS** (`peppol-bis-v1`) — the [Peppol BIS Billing 3.0](https://docs.peppol.eu/poacc/billing/3.0/)
  profile, layering the Peppol business rules (`PEPPOL-EN16931-*`) on top of
  EN 16931. Rules already guaranteed by EN 16931 or GOBL's calculation engine
  are not duplicated; country-specific CIUS rules belong in dedicated
  jurisdiction addons.
- **PINT** (`peppol-pint-v1`) — the base [Peppol International (PINT)](https://docs.peppol.eu/poac/pint/)
  billing model, the common layer shared by the country-specific PINT
  specifications. It builds on EN 16931 but recognises indirect tax schemes
  beyond VAT (such as GST).
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
import _ "github.com/invopop/gobl.peppol/addon/pint/aunz"
```

Then declare the addon on the document:

```yaml
$addons:
  - "peppol-pint-aunz-v1"
```

See the [`examples`](./examples) directory for complete invoices.

## Layout

- `addon/` — the GOBL addons: normalizers, scenarios and validation rules that
  register into GOBL on import. These packages are kept dependency-light so
  importing them never pulls in conversion tooling.
- `ubl/` — the Peppol BIS and PINT UBL formats, registered with `gobl.ubl` and
  GOBL's `convert` register.
- `cii/` — the Peppol BIS CII format, registered with `gobl.cii` and GOBL's
  `convert` register.

## Conversion

Blank-import the `ubl` package to register the Peppol formats with both `gobl.ubl`
and GOBL's central [`convert`](https://github.com/invopop/gobl/tree/main/convert)
register, then convert through either:

```go
import (
	"github.com/invopop/gobl/convert"
	_ "github.com/invopop/gobl.peppol/ubl"
)

out, err := convert.Export(env, "ubl+peppol+pint+aunz")   // GOBL -> UBL
env, err := convert.Import(data)                          // UBL -> GOBL
```

The format is picked from the document's `CustomizationID` and `ProfileID` on
import, and the exported XML carries them back:

| Format | Key | CustomizationID (`ibt-024`) | ProfileID (`ibt-023`) |
| --- | --- | --- | --- |
| `FormatBIS` | `ubl+peppol+bis-v3` | `urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0` | `urn:fdc:peppol.eu:2017:poacc:billing:01:1.0` |
| `FormatPINT` | `ubl+peppol+pint` | `urn:peppol:pint:billing-1` | `urn:peppol:bis:billing` |
| `FormatAUNZ` | `ubl+peppol+pint+aunz` | `urn:peppol:pint:billing-1@aunz-1` | `urn:peppol:bis:billing` |

`FormatBIS` writes self-billed invoices with `SpecificationBISSelfBilling`
(Peppol BIS Self-Billing 3.0) and statuses (`bill.Status`) as Peppol Invoice
Response 3 application responses with `SpecificationBISInvoiceResponse`,
mapping the status events, reasons and actions to the UNCL4343,
OPStatusReason and OPStatusAction codes and back. It joins the notes into one,
as Peppol BIS allows a single note.

`FormatAUNZ` writes self-billed invoices (those tagged `self-billed`) with its
`SpecificationAUNZSelfBilling` instead: `urn:peppol:pint:selfbilling-1@aunz-1`
and `urn:peppol:bis:selfbilling`. Documents carrying either are imported with
`FormatAUNZ`.

Each one's Peppol document type identifier is
`urn:oasis:names:specification:ubl:schema:xsd:Invoice-2::Invoice##<CustomizationID>::2.1`,
with `CreditNote-2::CreditNote` replacing `Invoice-2::Invoice` for credit notes.
The A-NZ formats also claim the wildcard customizations the SMP registrations
are made under, and `FormatPINT` is the jurisdiction-neutral base: it takes any
other `urn:peppol:pint:billing-1@…` specialization and validates it against the
base PINT rule set.

To hold a format directly, use `gobl.ubl` itself:

```go
import (
	peppolubl "github.com/invopop/gobl.peppol/ubl"
	ubl "github.com/invopop/gobl.ubl"
)

doc, err := ubl.ExportInvoice(env, ubl.WithFormat(peppolubl.FormatAUNZ))
data, err := ubl.Encode(doc)
```

### CII

Peppol BIS Billing 3.0 also allows the UN/CEFACT CII syntax. Blank-import the
`cii` package to register it:

```go
import (
	"github.com/invopop/gobl/convert"
	_ "github.com/invopop/gobl.peppol/cii"
)

out, err := convert.Export(env, "cii+peppol+bis-v3")   // GOBL -> CII
```

| Format | Key | GuidelineID (`BT-24`) | BusinessProcess (`BT-23`) |
| --- | --- | --- | --- |
| `FormatBIS` | `cii+peppol+bis-v3` | `urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0` | `urn:fdc:peppol.eu:2017:poacc:billing:01:1.0` |

Like its UBL counterpart, it joins the notes into one. PINT, self-billing and
the Invoice Response are UBL only. phive carries no Peppol CII rule set, so
CII documents are validated against the EN 16931 CII one.

## Validation

Converted documents are checked against the Peppol BIS and PINT schematrons. The
schematron is XSLT 2.0, so validation runs in a [phive](https://github.com/phax/phive)
rule engine fronted by a [phorm](https://github.com/invopop/phorm) HTTP service
rather than in-process. Each format and specification carries the VESIDs of its
rule sets, which `Format.VESID` picks for the document.

`TestConvertExamples` and `TestParseExamples` validate every document under
`ubl/testdata` and `cii/testdata` when `-validate` is passed. Like `-update` it is off by default,
so a plain `go test ./...` needs no phorm service:

```sh
go test ./ubl/ ./cii/ -args -validate=true   # posts to $PHORM_URL, default http://localhost:8080
```

CI runs it against a `phelger/phorm:latest` service container (see
`.github/workflows/test.yaml`).
