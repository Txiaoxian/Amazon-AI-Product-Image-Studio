package model

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
)

const (
	StatusEnabled  = "ENABLED"
	StatusDisabled = "DISABLED"

	PermissionRead   = "model:read"
	PermissionManage = "model:manage"

	maxModelNameRunes        = 255
	maxDisplayNameRunes      = 255
	maxProviderIDRunes       = 128
	maxListItems             = 64
	maxListItemRunes         = 128
	maxPricingUnitPrices     = 20
	maxPricingUnitNameRunes  = 64
	maxOutputCount           = 16
	defaultPageNum           = 1
	defaultPageSize          = 20
	maxPageSize              = 100
	defaultPricingCurrency   = "USD"
	capabilityGenerateFilter = "generate"
	capabilityEditFilter     = "edit"
)

var (
	ErrValidation         = errors.New("invalid model request")
	ErrForbidden          = errors.New("model access forbidden")
	ErrNotFound           = errors.New("model not found")
	ErrDuplicateModelName = errors.New("model name already exists for provider")
)

type ListQuery struct {
	PageNum    int
	PageSize   int
	ProviderID string
	Status     string
	Capability string
}

type ListOptions struct {
	PageNum    int
	PageSize   int
	ProviderID string
	Status     string
	Capability string
}

type Page struct {
	Records  []Response `json:"records"`
	Total    int64      `json:"total"`
	PageNum  int        `json:"pageNum"`
	PageSize int        `json:"pageSize"`
}

type Pricing struct {
	Currency   string             `json:"currency"`
	UnitPrices map[string]float64 `json:"unitPrices"`
}

type Response struct {
	ID                     string   `json:"id"`
	TenantID               string   `json:"tenantId"`
	ProviderID             string   `json:"providerId"`
	ProviderName           string   `json:"providerName"`
	ModelName              string   `json:"modelName"`
	DisplayName            string   `json:"displayName"`
	SupportsGenerate       bool     `json:"supportsGenerate"`
	SupportsEdit           bool     `json:"supportsEdit"`
	SupportsMultiReference bool     `json:"supportsMultiReference"`
	SupportsN              bool     `json:"supportsN"`
	MaxOutputCount         int      `json:"maxOutputCount"`
	SupportedSizes         []string `json:"supportedSizes"`
	SupportedQualities     []string `json:"supportedQualities"`
	SupportedOutputFormats []string `json:"supportedOutputFormats"`
	Pricing                Pricing  `json:"pricing"`
	Status                 string   `json:"status"`
	CreatedAt              string   `json:"createdAt"`
	UpdatedAt              string   `json:"updatedAt"`
}

type CreateInput struct {
	ProviderID                 string
	ModelName                  string
	DisplayName                string
	SupportsGenerate           bool
	SupportsEdit               bool
	SupportsMultiReference     bool
	SupportsN                  bool
	MaxOutputCount             int
	SupportedSizesJSON         string
	SupportedQualitiesJSON     string
	SupportedOutputFormatsJSON string
	PricingJSON                string
	Status                     string
}

type UpdateInput struct {
	ProviderID                 *string
	ModelName                  *string
	DisplayName                *string
	SupportsGenerate           *bool
	SupportsEdit               *bool
	SupportsMultiReference     *bool
	SupportsN                  *bool
	MaxOutputCount             *int
	SupportedSizesJSON         *string
	SupportedQualitiesJSON     *string
	SupportedOutputFormatsJSON *string
	PricingJSON                *string
	Status                     *string
}

func responseFromRecord(record database.AIModel, providerName string) (Response, error) {
	sizes, err := decodeStringList(record.SupportedSizesJSON)
	if err != nil {
		return Response{}, err
	}
	qualities, err := decodeStringList(record.SupportedQualitiesJSON)
	if err != nil {
		return Response{}, err
	}
	formats, err := decodeStringList(record.SupportedOutputFormatsJSON)
	if err != nil {
		return Response{}, err
	}
	pricing, err := decodePricing(record.PricingJSON)
	if err != nil {
		return Response{}, err
	}

	return Response{
		ID:                     record.ID,
		TenantID:               record.TenantID,
		ProviderID:             record.ProviderID,
		ProviderName:           providerName,
		ModelName:              record.ModelName,
		DisplayName:            record.DisplayName,
		SupportsGenerate:       record.SupportsGenerate,
		SupportsEdit:           record.SupportsEdit,
		SupportsMultiReference: record.SupportsMultiReference,
		SupportsN:              record.SupportsN,
		MaxOutputCount:         record.MaxOutputCount,
		SupportedSizes:         sizes,
		SupportedQualities:     qualities,
		SupportedOutputFormats: formats,
		Pricing:                pricing,
		Status:                 record.Status,
		CreatedAt:              formatTime(record.CreatedAt),
		UpdatedAt:              formatTime(record.UpdatedAt),
	}, nil
}

func encodeJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func decodeStringList(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	if values == nil {
		values = []string{}
	}
	return values, nil
}

func decodePricing(raw string) (Pricing, error) {
	if strings.TrimSpace(raw) == "" {
		return Pricing{Currency: defaultPricingCurrency, UnitPrices: map[string]float64{}}, nil
	}
	var pricing Pricing
	if err := json.Unmarshal([]byte(raw), &pricing); err != nil {
		return Pricing{}, err
	}
	if pricing.UnitPrices == nil {
		pricing.UnitPrices = map[string]float64{}
	}
	return pricing, nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
