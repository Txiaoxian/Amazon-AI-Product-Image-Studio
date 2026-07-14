package model

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/gin-gonic/gin"
)

type createRequest struct {
	ProviderID             string   `json:"providerId"`
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
}

type updateRequest struct {
	ProviderID             *string   `json:"providerId"`
	ModelName              *string   `json:"modelName"`
	DisplayName            *string   `json:"displayName"`
	SupportsGenerate       *bool     `json:"supportsGenerate"`
	SupportsEdit           *bool     `json:"supportsEdit"`
	SupportsMultiReference *bool     `json:"supportsMultiReference"`
	SupportsN              *bool     `json:"supportsN"`
	MaxOutputCount         *int      `json:"maxOutputCount"`
	SupportedSizes         *[]string `json:"supportedSizes"`
	SupportedQualities     *[]string `json:"supportedQualities"`
	SupportedOutputFormats *[]string `json:"supportedOutputFormats"`
	Pricing                *Pricing  `json:"pricing"`
	Status                 *string   `json:"status"`
}

type capabilityState struct {
	SupportsGenerate       bool
	SupportsEdit           bool
	SupportsMultiReference bool
	SupportsN              bool
	MaxOutputCount         int
}

func normalizeCreateRequest(request createRequest) (CreateInput, error) {
	providerID, err := cleanRequired(request.ProviderID, maxProviderIDRunes)
	if err != nil {
		return CreateInput{}, err
	}
	modelName, err := cleanRequired(request.ModelName, maxModelNameRunes)
	if err != nil {
		return CreateInput{}, err
	}
	displayName, err := cleanRequired(request.DisplayName, maxDisplayNameRunes)
	if err != nil {
		return CreateInput{}, err
	}
	status, err := normalizeStatus(request.Status, StatusEnabled)
	if err != nil {
		return CreateInput{}, err
	}
	maxOutputs, err := normalizeMaxOutputCount(request.MaxOutputCount)
	if err != nil {
		return CreateInput{}, err
	}
	if err := validateCapabilityState(capabilityState{
		SupportsGenerate:       request.SupportsGenerate,
		SupportsEdit:           request.SupportsEdit,
		SupportsMultiReference: request.SupportsMultiReference,
		SupportsN:              request.SupportsN,
		MaxOutputCount:         maxOutputs,
	}); err != nil {
		return CreateInput{}, err
	}

	sizesJSON, err := normalizeSupportedSizesJSON(request.SupportedSizes)
	if err != nil {
		return CreateInput{}, err
	}
	qualitiesJSON, err := normalizeSupportedQualitiesJSON(request.SupportedQualities)
	if err != nil {
		return CreateInput{}, err
	}
	formatsJSON, err := normalizeSupportedOutputFormatsJSON(request.SupportedOutputFormats)
	if err != nil {
		return CreateInput{}, err
	}
	pricingJSON, err := normalizePricingJSON(request.Pricing)
	if err != nil {
		return CreateInput{}, err
	}

	return CreateInput{
		ProviderID:                 providerID,
		ModelName:                  modelName,
		DisplayName:                displayName,
		SupportsGenerate:           request.SupportsGenerate,
		SupportsEdit:               request.SupportsEdit,
		SupportsMultiReference:     request.SupportsMultiReference,
		SupportsN:                  request.SupportsN,
		MaxOutputCount:             maxOutputs,
		SupportedSizesJSON:         sizesJSON,
		SupportedQualitiesJSON:     qualitiesJSON,
		SupportedOutputFormatsJSON: formatsJSON,
		PricingJSON:                pricingJSON,
		Status:                     status,
	}, nil
}

func normalizeUpdateRequest(request updateRequest) (UpdateInput, []string, error) {
	input := UpdateInput{}
	changedFields := make([]string, 0, 12)

	if request.ProviderID != nil {
		value, err := cleanRequired(*request.ProviderID, maxProviderIDRunes)
		if err != nil {
			return UpdateInput{}, nil, err
		}
		input.ProviderID = &value
		changedFields = append(changedFields, "providerId")
	}
	if request.ModelName != nil {
		value, err := cleanRequired(*request.ModelName, maxModelNameRunes)
		if err != nil {
			return UpdateInput{}, nil, err
		}
		input.ModelName = &value
		changedFields = append(changedFields, "modelName")
	}
	if request.DisplayName != nil {
		value, err := cleanRequired(*request.DisplayName, maxDisplayNameRunes)
		if err != nil {
			return UpdateInput{}, nil, err
		}
		input.DisplayName = &value
		changedFields = append(changedFields, "displayName")
	}
	if request.SupportsGenerate != nil {
		input.SupportsGenerate = request.SupportsGenerate
		changedFields = append(changedFields, "supportsGenerate")
	}
	if request.SupportsEdit != nil {
		input.SupportsEdit = request.SupportsEdit
		changedFields = append(changedFields, "supportsEdit")
	}
	if request.SupportsMultiReference != nil {
		input.SupportsMultiReference = request.SupportsMultiReference
		changedFields = append(changedFields, "supportsMultiReference")
	}
	if request.SupportsN != nil {
		input.SupportsN = request.SupportsN
		changedFields = append(changedFields, "supportsN")
	}
	if request.MaxOutputCount != nil {
		value, err := normalizeMaxOutputCount(*request.MaxOutputCount)
		if err != nil {
			return UpdateInput{}, nil, err
		}
		input.MaxOutputCount = &value
		changedFields = append(changedFields, "maxOutputCount")
	}
	if request.SupportedSizes != nil {
		value, err := normalizeSupportedSizesJSON(*request.SupportedSizes)
		if err != nil {
			return UpdateInput{}, nil, err
		}
		input.SupportedSizesJSON = &value
		changedFields = append(changedFields, "supportedSizes")
	}
	if request.SupportedQualities != nil {
		value, err := normalizeSupportedQualitiesJSON(*request.SupportedQualities)
		if err != nil {
			return UpdateInput{}, nil, err
		}
		input.SupportedQualitiesJSON = &value
		changedFields = append(changedFields, "supportedQualities")
	}
	if request.SupportedOutputFormats != nil {
		value, err := normalizeSupportedOutputFormatsJSON(*request.SupportedOutputFormats)
		if err != nil {
			return UpdateInput{}, nil, err
		}
		input.SupportedOutputFormatsJSON = &value
		changedFields = append(changedFields, "supportedOutputFormats")
	}
	if request.Pricing != nil {
		value, err := normalizePricingJSON(*request.Pricing)
		if err != nil {
			return UpdateInput{}, nil, err
		}
		input.PricingJSON = &value
		changedFields = append(changedFields, "pricing")
	}
	if request.Status != nil {
		value, err := normalizeStatus(*request.Status, "")
		if err != nil {
			return UpdateInput{}, nil, err
		}
		input.Status = &value
		changedFields = append(changedFields, "status")
	}
	if len(changedFields) == 0 {
		return UpdateInput{}, nil, ErrValidation
	}
	return input, changedFields, nil
}

func parseListQuery(c *gin.Context) (ListQuery, error) {
	pageNum := parsePositiveInt(c.Query("pageNum"), defaultPageNum)
	pageSize := parsePositiveInt(c.Query("pageSize"), defaultPageSize)
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	providerID := strings.TrimSpace(c.Query("providerId"))
	if utf8.RuneCountInString(providerID) > maxProviderIDRunes {
		return ListQuery{}, ErrValidation
	}
	status := cleanEnum(c.Query("status"))
	if status == "" && strings.EqualFold(strings.TrimSpace(c.Query("enabled")), "true") {
		status = StatusEnabled
	}
	if status != "" && !validStatus(status) {
		return ListQuery{}, ErrValidation
	}
	capability := strings.ToLower(strings.TrimSpace(c.Query("capability")))
	if capability != "" && capability != capabilityGenerateFilter && capability != capabilityEditFilter {
		return ListQuery{}, ErrValidation
	}
	return ListQuery{PageNum: pageNum, PageSize: pageSize, ProviderID: providerID, Status: status, Capability: capability}, nil
}

func parsePositiveInt(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func normalizeStatus(status string, defaultStatus string) (string, error) {
	status = cleanEnum(status)
	if status == "" {
		status = defaultStatus
	}
	if !validStatus(status) {
		return "", ErrValidation
	}
	return status, nil
}

func normalizeMaxOutputCount(value int) (int, error) {
	if value == 0 {
		return 1, nil
	}
	if value < 1 || value > maxOutputCount {
		return 0, ErrValidation
	}
	return value, nil
}

func validateCapabilityState(state capabilityState) error {
	if !state.SupportsGenerate && !state.SupportsEdit {
		return ErrValidation
	}
	if state.MaxOutputCount < 1 || state.MaxOutputCount > maxOutputCount {
		return ErrValidation
	}
	if !state.SupportsN && state.MaxOutputCount > 1 {
		return ErrValidation
	}
	return nil
}

func stateFromRecord(record database.AIModel) capabilityState {
	return capabilityState{
		SupportsGenerate:       record.SupportsGenerate,
		SupportsEdit:           record.SupportsEdit,
		SupportsMultiReference: record.SupportsMultiReference,
		SupportsN:              record.SupportsN,
		MaxOutputCount:         record.MaxOutputCount,
	}
}

func applyStateUpdate(state capabilityState, input UpdateInput) capabilityState {
	if input.SupportsGenerate != nil {
		state.SupportsGenerate = *input.SupportsGenerate
	}
	if input.SupportsEdit != nil {
		state.SupportsEdit = *input.SupportsEdit
	}
	if input.SupportsMultiReference != nil {
		state.SupportsMultiReference = *input.SupportsMultiReference
	}
	if input.SupportsN != nil {
		state.SupportsN = *input.SupportsN
	}
	if input.MaxOutputCount != nil {
		state.MaxOutputCount = *input.MaxOutputCount
	}
	return state
}

func normalizeSupportedSizesJSON(values []string) (string, error) {
	return normalizeStringListJSON(values, cleanSupportedSize, true)
}

func normalizeSupportedQualitiesJSON(values []string) (string, error) {
	return normalizeStringListJSON(values, cleanSupportedQuality, false)
}

func normalizeSupportedOutputFormatsJSON(values []string) (string, error) {
	return normalizeStringListJSON(values, cleanSupportedOutputFormat, true)
}

func normalizeStringListJSON(values []string, cleanItem func(string) (string, error), requireNonEmpty bool) (string, error) {
	if len(values) > maxListItems {
		return "", ErrValidation
	}
	clean := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value, err := cleanItem(value)
		if err != nil {
			return "", ErrValidation
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		clean = append(clean, value)
	}
	if requireNonEmpty && len(clean) == 0 {
		return "", ErrValidation
	}
	return encodeJSON(clean)
}

func cleanSupportedSize(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || utf8.RuneCountInString(value) > maxListItemRunes {
		return "", ErrValidation
	}
	if value == "auto" {
		return value, nil
	}
	if isSupportedAspectRatio(value) {
		return value, nil
	}
	widthRaw, heightRaw, ok := strings.Cut(value, "x")
	if !ok || widthRaw == "" || heightRaw == "" {
		return "", ErrValidation
	}
	width, err := strconv.Atoi(widthRaw)
	if err != nil || width < 1 || width > 8192 {
		return "", ErrValidation
	}
	height, err := strconv.Atoi(heightRaw)
	if err != nil || height < 1 || height > 8192 {
		return "", ErrValidation
	}
	return strconv.Itoa(width) + "x" + strconv.Itoa(height), nil
}

func isSupportedAspectRatio(value string) bool {
	switch value {
	case "1:1", "1.62:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9":
		return true
	default:
		return false
	}
}

func cleanSupportedQuality(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || utf8.RuneCountInString(value) > maxListItemRunes {
		return "", ErrValidation
	}
	switch value {
	case "auto", "1k", "2k", "4k", "low", "medium", "standard", "high", "hd":
		return value, nil
	default:
		return "", ErrValidation
	}
}

func cleanSupportedOutputFormat(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || utf8.RuneCountInString(value) > maxListItemRunes {
		return "", ErrValidation
	}
	switch value {
	case "png", "jpeg", "webp":
		return value, nil
	case "jpg":
		return "jpeg", nil
	default:
		return "", ErrValidation
	}
}

func normalizePricingJSON(pricing Pricing) (string, error) {
	currency := strings.ToUpper(strings.TrimSpace(pricing.Currency))
	if currency == "" {
		currency = defaultPricingCurrency
	}
	if !validCurrency(currency) {
		return "", ErrValidation
	}
	if len(pricing.UnitPrices) > maxPricingUnitPrices {
		return "", ErrValidation
	}

	unitPrices := make(map[string]float64, len(pricing.UnitPrices))
	for key, value := range pricing.UnitPrices {
		key = strings.TrimSpace(key)
		if key == "" || utf8.RuneCountInString(key) > maxPricingUnitNameRunes {
			return "", ErrValidation
		}
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1000000 {
			return "", ErrValidation
		}
		unitPrices[key] = value
	}
	return encodeJSON(Pricing{Currency: currency, UnitPrices: unitPrices})
}

func validCurrency(currency string) bool {
	if utf8.RuneCountInString(currency) != 3 {
		return false
	}
	for _, r := range currency {
		if !unicode.IsUpper(r) || r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func validStatus(status string) bool {
	switch status {
	case StatusEnabled, StatusDisabled:
		return true
	default:
		return false
	}
}

func cleanRequired(value string, limit int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > limit {
		return "", ErrValidation
	}
	return value, nil
}

func cleanEnum(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
