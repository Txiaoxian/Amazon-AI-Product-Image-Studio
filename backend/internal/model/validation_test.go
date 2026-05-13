package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestNormalizeCreateRequestValidatesCapabilitiesAndPricing(t *testing.T) {
	input, err := normalizeCreateRequest(createRequest{
		ProviderID:             "provider-1",
		ModelName:              "gpt-image",
		DisplayName:            "GPT Image",
		SupportsGenerate:       true,
		SupportsEdit:           true,
		SupportsMultiReference: true,
		SupportsN:              true,
		MaxOutputCount:         4,
		SupportedSizes:         []string{"1024x1024", "1024x1024", "1536x1024"},
		SupportedQualities:     []string{"standard", "hd"},
		SupportedOutputFormats: []string{"png", "jpeg"},
		Pricing: Pricing{
			Currency: "usd",
			UnitPrices: map[string]float64{
				"image": 0.04,
			},
		},
	})
	if err != nil {
		t.Fatalf("normalize create request: %v", err)
	}
	if input.Status != StatusEnabled {
		t.Fatalf("default status = %q, want ENABLED", input.Status)
	}
	if input.MaxOutputCount != 4 {
		t.Fatalf("max output count = %d, want 4", input.MaxOutputCount)
	}

	var sizes []string
	if err := json.Unmarshal([]byte(input.SupportedSizesJSON), &sizes); err != nil {
		t.Fatalf("decode sizes: %v", err)
	}
	if len(sizes) != 2 {
		t.Fatalf("deduped sizes = %#v, want two values", sizes)
	}
	var pricing Pricing
	if err := json.Unmarshal([]byte(input.PricingJSON), &pricing); err != nil {
		t.Fatalf("decode pricing: %v", err)
	}
	if pricing.Currency != "USD" || pricing.UnitPrices["image"] != 0.04 {
		t.Fatalf("normalized pricing = %#v", pricing)
	}
}

func TestNormalizeCreateRequestRejectsInvalidCapabilityCombinations(t *testing.T) {
	base := createRequest{
		ProviderID:             "provider-1",
		ModelName:              "gpt-image",
		DisplayName:            "GPT Image",
		SupportsGenerate:       true,
		MaxOutputCount:         1,
		SupportedSizes:         []string{"1024x1024"},
		SupportedQualities:     []string{"standard"},
		SupportedOutputFormats: []string{"png"},
	}

	noCapability := base
	noCapability.SupportsGenerate = false
	noCapability.SupportsEdit = false
	if _, err := normalizeCreateRequest(noCapability); !errors.Is(err, ErrValidation) {
		t.Fatalf("no capability err = %v, want ErrValidation", err)
	}

	invalidN := base
	invalidN.SupportsN = false
	invalidN.MaxOutputCount = 2
	if _, err := normalizeCreateRequest(invalidN); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid supportsN err = %v, want ErrValidation", err)
	}
}

func TestNormalizePricingRejectsUnboundedOrInvalidJSONShape(t *testing.T) {
	tooManyPrices := map[string]float64{}
	for i := 0; i < maxPricingUnitPrices+1; i++ {
		tooManyPrices[fmt.Sprintf("unit-%02d", i)] = 0.01
	}
	if _, err := normalizePricingJSON(Pricing{Currency: "USD", UnitPrices: tooManyPrices}); !errors.Is(err, ErrValidation) {
		t.Fatalf("too many prices err = %v, want ErrValidation", err)
	}
	if _, err := normalizePricingJSON(Pricing{Currency: "US", UnitPrices: map[string]float64{"image": 0.01}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid currency err = %v, want ErrValidation", err)
	}
	if _, err := normalizePricingJSON(Pricing{Currency: "USD", UnitPrices: map[string]float64{"image": -0.01}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("negative price err = %v, want ErrValidation", err)
	}
}

func TestNormalizeCapabilityListsRejectInvalidValues(t *testing.T) {
	if _, err := normalizeSupportedSizesJSON(nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("empty size list err = %v, want ErrValidation", err)
	}
	if _, err := normalizeSupportedSizesJSON([]string{"localhost"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid size err = %v, want ErrValidation", err)
	}
	if _, err := normalizeSupportedQualitiesJSON([]string{"ultra-premium"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid quality err = %v, want ErrValidation", err)
	}
	formatsJSON, err := normalizeSupportedOutputFormatsJSON([]string{"jpg", "webp"})
	if err != nil {
		t.Fatalf("normalize formats: %v", err)
	}
	var formats []string
	if err := json.Unmarshal([]byte(formatsJSON), &formats); err != nil {
		t.Fatalf("decode formats: %v", err)
	}
	if len(formats) != 2 || formats[0] != "jpeg" || formats[1] != "webp" {
		t.Fatalf("normalized formats = %#v", formats)
	}
	if _, err := normalizeSupportedOutputFormatsJSON(nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("empty output format list err = %v, want ErrValidation", err)
	}
	if _, err := normalizeSupportedOutputFormatsJSON([]string{"gif"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid output format err = %v, want ErrValidation", err)
	}
}
