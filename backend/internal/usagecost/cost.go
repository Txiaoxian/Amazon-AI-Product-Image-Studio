package usagecost

import (
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

const (
	defaultCurrency = "USD"
	zeroCost        = "0.00000000"
)

var maxUnitPrice = big.NewRat(1000000, 1)

type Usage struct {
	InputTokens  int64
	OutputTokens int64
	ImageCount   int
}

type Result struct {
	Currency      string
	EstimatedCost string
}

type pricingConfig struct {
	Currency   string                     `json:"currency"`
	UnitPrices map[string]json.RawMessage `json:"unitPrices"`
}

func Estimate(pricingJSON string, usage Usage) Result {
	config, ok := decodePricing(pricingJSON)
	currency := defaultCurrency
	if ok {
		currency = normalizeCurrency(config.Currency)
	}
	if !ok || config.UnitPrices == nil {
		return Result{Currency: currency, EstimatedCost: zeroCost}
	}

	prices, ok := parseUnitPrices(config.UnitPrices)
	if !ok {
		return Result{Currency: currency, EstimatedCost: zeroCost}
	}

	inputPrice, ok := requiredPrice(nonNegativeInt64(usage.InputTokens) > 0, prices, "inputToken", "input_token", "inputTokens", "input_tokens")
	if !ok {
		return Result{Currency: currency, EstimatedCost: zeroCost}
	}
	outputPrice, ok := requiredPrice(nonNegativeInt64(usage.OutputTokens) > 0, prices, "outputToken", "output_token", "outputTokens", "output_tokens")
	if !ok {
		return Result{Currency: currency, EstimatedCost: zeroCost}
	}
	imagePrice, ok := requiredPrice(nonNegativeInt(usage.ImageCount) > 0, prices, "image", "images", "outputImage", "output_image")
	if !ok {
		return Result{Currency: currency, EstimatedCost: zeroCost}
	}

	total := new(big.Rat)
	addCost(total, nonNegativeInt64(usage.InputTokens), inputPrice)
	addCost(total, nonNegativeInt64(usage.OutputTokens), outputPrice)
	addCost(total, int64(nonNegativeInt(usage.ImageCount)), imagePrice)
	if total.Sign() < 0 {
		return Result{Currency: currency, EstimatedCost: zeroCost}
	}
	return Result{Currency: currency, EstimatedCost: total.FloatString(8)}
}

func decodePricing(raw string) (pricingConfig, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return pricingConfig{}, false
	}
	var config pricingConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return pricingConfig{}, false
	}
	return config, true
}

func parseUnitPrices(values map[string]json.RawMessage) (map[string]*big.Rat, bool) {
	prices := make(map[string]*big.Rat, len(values))
	for key, raw := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, false
		}
		price, ok := parseNonNegativeDecimal(string(raw))
		if !ok {
			return nil, false
		}
		prices[key] = price
	}
	return prices, true
}

func requiredPrice(required bool, prices map[string]*big.Rat, keys ...string) (*big.Rat, bool) {
	if !required {
		return new(big.Rat), true
	}
	for _, key := range keys {
		if price, ok := prices[key]; ok {
			return price, true
		}
	}
	return nil, false
}

func addCost(total *big.Rat, quantity int64, unitPrice *big.Rat) {
	if quantity <= 0 || unitPrice == nil {
		return
	}
	term := new(big.Rat).Mul(new(big.Rat).SetInt64(quantity), unitPrice)
	total.Add(total, term)
}

func normalizeCurrency(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != 3 {
		return defaultCurrency
	}
	for _, r := range value {
		if !unicode.IsUpper(r) || r < 'A' || r > 'Z' {
			return defaultCurrency
		}
	}
	return value
}

func parseNonNegativeDecimal(raw string) (*big.Rat, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}

	mantissa := raw
	exponent := 0
	if index := strings.IndexAny(raw, "eE"); index >= 0 {
		mantissa = raw[:index]
		parsedExponent, err := strconv.Atoi(raw[index+1:])
		if err != nil || parsedExponent < -64 || parsedExponent > 64 {
			return nil, false
		}
		exponent = parsedExponent
	}

	negative := false
	if strings.HasPrefix(mantissa, "-") {
		negative = true
		mantissa = strings.TrimPrefix(mantissa, "-")
	} else if strings.HasPrefix(mantissa, "+") {
		return nil, false
	}

	parts := strings.Split(mantissa, ".")
	if len(parts) > 2 {
		return nil, false
	}
	integerPart := parts[0]
	fractionPart := ""
	if len(parts) == 2 {
		fractionPart = parts[1]
	}
	digits := integerPart + fractionPart
	if digits == "" || len(digits) > 64 {
		return nil, false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return nil, false
		}
	}

	numerator := new(big.Int)
	if _, ok := numerator.SetString(digits, 10); !ok {
		return nil, false
	}
	if negative && numerator.Sign() != 0 {
		return nil, false
	}

	scale := len(fractionPart) - exponent
	if scale < -64 || scale > 64 {
		return nil, false
	}
	if scale < 0 {
		numerator.Mul(numerator, pow10(-scale))
		scale = 0
	}
	denominator := pow10(scale)
	value := new(big.Rat).SetFrac(numerator, denominator)
	if value.Cmp(maxUnitPrice) > 0 {
		return nil, false
	}
	return value, true
}

func pow10(exponent int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil)
}

func nonNegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func nonNegativeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
