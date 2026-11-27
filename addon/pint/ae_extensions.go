package pint

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pkg/here"
	"github.com/invopop/gobl/regimes/ae"
	"github.com/invopop/gobl/tax"
)

// PINT AE extension keys, for the BTAE fields and UAE statutory code lists.
const (
	ExtKeyTransactionType  cbc.Key = "pint-ae-transaction-type"
	ExtKeyCreditNoteReason cbc.Key = "pint-ae-credit-note-reason"
	ExtKeyBeneficiaryID    cbc.Key = "pint-ae-beneficiary-id"
	ExtKeyPrincipalID      cbc.Key = "pint-ae-principal-id"
	ExtKeyExemption        cbc.Key = "pint-ae-exemption"
	ExtKeyGoodsType        cbc.Key = "pint-ae-goods-type"
	ExtKeyItemType         cbc.Key = "pint-ae-item-type"
)

// CreditNoteReasonVolumeDiscount is the PINT AE credit note reason code (BTAE-03) for a volume discount.
const CreditNoteReasonVolumeDiscount cbc.Code = "VD"

// PINT AE item type codes for ExtKeyItemType (BTAE-13).
const (
	ItemTypeGoods         cbc.Code = "G"
	ItemTypeServices      cbc.Code = "S"
	ItemTypeGoodsServices cbc.Code = "B"
)

var (
	usageAEInvoiceTax = &cbc.Usage{
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{bill.ShortSchemaTax},
	}
	usageAETaxCombo = &cbc.Usage{
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{tax.ShortSchemaCombo},
	}
	usageAEItem = &cbc.Usage{
		Codes:   []cbc.Code{ae.CountryCode},
		Schemas: []string{org.ShortSchemaItem},
	}
)

var extensions = []*cbc.Definition{
	{
		Key:   ExtKeyTransactionType,
		Usage: usageAEInvoiceTax,
		Name: i18n.String{
			i18n.EN: "PINT AE: Transaction Type",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				PINT AE invoice transaction type (BTAE-02): 8 binary flags of 0 or 1, in
				this order: free trade zone, deemed supply, profit margin scheme, summary
				invoice, continuous supply, disclosed agent billing, supply through
				e-commerce, and exports. Defaults to "00000000" when unset.
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("Peppol PINT AE Billing specification (1.0.5)"),
				URL:   "https://docs.peppol.eu/poac/ae/v1.0.5/pint-ae/bis/",
			},
		},
		Pattern: "^[01]{8}$",
	},
	{
		Key:   ExtKeyCreditNoteReason,
		Usage: usageAEInvoiceTax,
		Name: i18n.String{
			i18n.EN: "PINT AE: Credit Note Reason",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Reason for a PINT AE credit note (BTAE-03), from the Reasons for credit
				note code list. Required on credit notes (IBR-158-AE). Volume discounts
				("VD") have no preceding invoice; every other reason needs one
				(IBR-055-AE).
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("PINT AE Reasons for credit note"),
				URL:   "https://docs.peppol.eu/poac/ae/v1.0.5/pint-ae/trn-creditnote/codelist/CreditReason/",
			},
		},
		Values: []*cbc.Definition{
			{
				Code: "DL8.61.1.A",
				Name: i18n.String{i18n.EN: "If the supply was cancelled."},
			},
			{
				Code: "DL8.61.1.B",
				Name: i18n.String{i18n.EN: "If the tax treatment of the supply has changed due to a change in the nature of the supply."},
			},
			{
				Code: "DL8.61.1.C",
				Name: i18n.String{i18n.EN: "If the previously agreed consideration for the supply was altered for any reason (i.e. bad debt relief)."},
			},
			{
				Code: "DL8.61.1.D",
				Name: i18n.String{i18n.EN: "If the recipient of goods or recipient of services returned them to the registrant in full or in part and the Consideration was returned in full or in part."},
			},
			{
				Code: "DL8.61.1.E",
				Name: i18n.String{i18n.EN: "If the tax was charged or tax treatment was applied in error."},
			},
			{
				Code: CreditNoteReasonVolumeDiscount,
				Name: i18n.String{i18n.EN: "Volume Discount."},
			},
		},
	},
	{
		Key:   ExtKeyBeneficiaryID,
		Usage: usageAEInvoiceTax,
		Name: i18n.String{
			i18n.EN: "PINT AE: Beneficiary ID",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Identifier of the beneficiary of a supply in a free trade zone (BTAE-01),
				required when the free trade zone flag (position 1, "10000000", of
				pint-ae-transaction-type) is set (IBR-007-AE).
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("Peppol PINT AE Billing specification (1.0.5)"),
				URL:   "https://docs.peppol.eu/poac/ae/v1.0.5/pint-ae/bis/",
			},
		},
	},
	{
		Key:   ExtKeyPrincipalID,
		Usage: usageAEInvoiceTax,
		Name: i18n.String{
			i18n.EN: "PINT AE: Principal ID",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Identifier of the principal a disclosed agent bills on behalf of
				(BTAE-14), required when the disclosed agent billing flag (position 6,
				"00000100", of pint-ae-transaction-type) is set (IBR-137-AE). It is the
				principal's TRN: 15 digits starting with 1 and ending with 03
				(IBR-132-AE).
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("Peppol PINT AE Billing specification (1.0.5)"),
				URL:   "https://docs.peppol.eu/poac/ae/v1.0.5/pint-ae/bis/",
			},
		},
		Pattern: "^1[0-9]{12}03$",
	},
	{
		Key:   ExtKeyExemption,
		Usage: usageAETaxCombo,
		Name: i18n.String{
			i18n.EN: "PINT AE: VAT Exemption Reason",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Reasons for exemption from UAE VAT under Article 46 of Federal Decree-Law
				No. 8 of 2017 (PINT AE Aligned-TaxExemptionCodes).
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("PINT AE Aligned Tax Exemption Codes"),
				URL:   "https://docs.peppol.eu/poac/ae/v1.0.5/pint-ae/trn-invoice/codelist/Aligned-TaxExemptionCodes/",
			},
		},
		Values: []*cbc.Definition{
			{
				Code: "DL8.46.1",
				Name: i18n.String{i18n.EN: "Financial services"},
				Desc: i18n.String{i18n.EN: "Exempt supply of certain financial services under Article 46(1) of UAE Federal Decree-Law No. 8 of 2017."},
			},
			{
				Code: "DL8.46.2",
				Name: i18n.String{i18n.EN: "Residential buildings"},
				Desc: i18n.String{i18n.EN: "Exempt supply of residential buildings through sale or lease under Article 46(2) of UAE Federal Decree-Law No. 8 of 2017."},
			},
			{
				Code: "DL8.46.3",
				Name: i18n.String{i18n.EN: "Bare land"},
				Desc: i18n.String{i18n.EN: "Exempt supply of bare land under Article 46(3) of UAE Federal Decree-Law No. 8 of 2017."},
			},
			{
				Code: "DL8.46.4",
				Name: i18n.String{i18n.EN: "Local passenger transport"},
				Desc: i18n.String{i18n.EN: "Exempt supply of local passenger transport under Article 46(4) of UAE Federal Decree-Law No. 8 of 2017."},
			},
		},
	},
	{
		Key:   ExtKeyGoodsType,
		Usage: usageAEItem,
		Name: i18n.String{
			i18n.EN: "PINT AE: Type of Goods or Services",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Type of goods or services subject to the UAE domestic reverse charge
				mechanism under Article 48 of Federal Decree-Law No. 8 of 2017 (PINT AE
				BTAE-09, GoodsType code list). Required on the items of reverse charge
				lines (IBR-166-AE).
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("PINT AE Type of goods or services subject to RCM"),
				URL:   "https://docs.peppol.eu/poac/ae/v1.0.5/pint-ae/trn-invoice/codelist/GoodsType/",
			},
		},
		Values: []*cbc.Definition{
			{Code: "DL8.48.8.2", Name: i18n.String{i18n.EN: "Electronic Devices"}},
			{Code: "DL8.48.8.1", Name: i18n.String{i18n.EN: "Gold and Diamonds"}},
			{Code: "DL8.48.8.3", Name: i18n.String{i18n.EN: "Metal Scrap Trading"}},
			{Code: "DL8.48.3.1", Name: i18n.String{i18n.EN: "Crude or refined oil"}},
			{Code: "DL8.48.3.2", Name: i18n.String{i18n.EN: "Unprocessed or processed natural gas"}},
			{Code: "DL8.48.3.3", Name: i18n.String{i18n.EN: "Pure hydrocarbons"}},
		},
	},
	{
		Key:   ExtKeyItemType,
		Usage: usageAEItem,
		Name: i18n.String{
			i18n.EN: "PINT AE: Item Type",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				PINT AE item type (BTAE-13): goods ("G"), services ("S"), or both goods
				and services ("B"). Derived automatically from the item's key ("goods" ->
				"G", "services" -> "S") unless set to "B" explicitly.
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.NewString("Peppol PINT AE Billing specification (1.0.5)"),
				URL:   "https://docs.peppol.eu/poac/ae/v1.0.5/pint-ae/bis/",
			},
		},
		Values: []*cbc.Definition{
			{Code: ItemTypeGoods, Name: i18n.String{i18n.EN: "Goods"}},
			{Code: ItemTypeServices, Name: i18n.String{i18n.EN: "Services"}},
			{Code: ItemTypeGoodsServices, Name: i18n.String{i18n.EN: "Both goods and services"}},
		},
	},
}
