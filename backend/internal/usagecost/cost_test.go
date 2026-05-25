package usagecost

import "testing"

func TestEstimateUsesDeterministicEightDecimalCostAndPricingAliases(t *testing.T) {
	result := Estimate(`{
		"currency":"eur",
		"unitPrices":{
			"input_token":0.1,
			"outputTokens":0.2,
			"output_image":0.3
		}
	}`, Usage{
		InputTokens:  3,
		OutputTokens: 2,
		ImageCount:   1,
	})

	if result.Currency != "EUR" {
		t.Fatalf("currency = %q, want EUR", result.Currency)
	}
	if result.EstimatedCost != "1.00000000" {
		t.Fatalf("estimated cost = %q, want 1.00000000", result.EstimatedCost)
	}
}

func TestEstimateRoundsDeterministicallyToEightDecimals(t *testing.T) {
	result := Estimate(`{"currency":"USD","unitPrices":{"inputToken":0.000000005}}`, Usage{InputTokens: 1})

	if result.EstimatedCost != "0.00000001" {
		t.Fatalf("estimated cost = %q, want 0.00000001", result.EstimatedCost)
	}
}

func TestEstimateDefaultsInvalidCurrencyWithoutDiscardingValidPricing(t *testing.T) {
	result := Estimate(`{"currency":"US","unitPrices":{"image":0.01}}`, Usage{ImageCount: 1})

	if result.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", result.Currency)
	}
	if result.EstimatedCost != "0.01000000" {
		t.Fatalf("estimated cost = %q, want 0.01000000", result.EstimatedCost)
	}
}

func TestEstimateReturnsZeroForMissingInvalidNegativeOrIncompletePricing(t *testing.T) {
	tests := []struct {
		name         string
		pricingJSON  string
		usage        Usage
		wantCurrency string
	}{
		{
			name:         "missing pricing",
			pricingJSON:  "",
			usage:        Usage{InputTokens: 1},
			wantCurrency: "USD",
		},
		{
			name:         "invalid json",
			pricingJSON:  `{"currency":"JPY","unitPrices":`,
			usage:        Usage{InputTokens: 1},
			wantCurrency: "USD",
		},
		{
			name:         "missing unitPrices",
			pricingJSON:  `{"currency":"EUR"}`,
			usage:        Usage{InputTokens: 1},
			wantCurrency: "EUR",
		},
		{
			name:         "invalid unit price type",
			pricingJSON:  `{"currency":"USD","unitPrices":{"inputToken":"0.01"}}`,
			usage:        Usage{InputTokens: 1},
			wantCurrency: "USD",
		},
		{
			name:         "negative unit price",
			pricingJSON:  `{"currency":"USD","unitPrices":{"inputToken":-0.01}}`,
			usage:        Usage{InputTokens: 1},
			wantCurrency: "USD",
		},
		{
			name:         "incomplete for used dimension",
			pricingJSON:  `{"currency":"USD","unitPrices":{"inputToken":0.01}}`,
			usage:        Usage{InputTokens: 1, OutputTokens: 1},
			wantCurrency: "USD",
		},
		{
			name:         "negative usage is ignored",
			pricingJSON:  `{"currency":"USD","unitPrices":{"inputToken":0.01}}`,
			usage:        Usage{InputTokens: -1},
			wantCurrency: "USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Estimate(tt.pricingJSON, tt.usage)
			if result.Currency != tt.wantCurrency {
				t.Fatalf("currency = %q, want %q", result.Currency, tt.wantCurrency)
			}
			if result.EstimatedCost != "0.00000000" {
				t.Fatalf("estimated cost = %q, want 0.00000000", result.EstimatedCost)
			}
		})
	}
}
