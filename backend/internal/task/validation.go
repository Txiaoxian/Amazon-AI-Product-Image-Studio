package task

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	assetpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/gin-gonic/gin"
)

type createRequest struct {
	Type              string         `json:"type"`
	Prompt            string         `json:"prompt"`
	ProviderID        string         `json:"providerId"`
	ModelID           string         `json:"modelId"`
	ImageType         string         `json:"imageType"`
	ReferenceAssetIDs []string       `json:"referenceAssetIds"`
	EditSourceAssetID string         `json:"editSourceAssetId"`
	Parameters        map[string]any `json:"parameters"`
}

var allowedCreateFields = map[string]bool{
	"type":              true,
	"prompt":            true,
	"providerId":        true,
	"modelId":           true,
	"imageType":         true,
	"referenceAssetIds": true,
	"editSourceAssetId": true,
	"parameters":        true,
}

var serverOwnedCreateFields = map[string]bool{
	"tenantId":       true,
	"tenant_id":      true,
	"createdBy":      true,
	"created_by":     true,
	"createdAt":      true,
	"created_at":     true,
	"updatedAt":      true,
	"updated_at":     true,
	"status":         true,
	"attempt":        true,
	"maxAttempts":    true,
	"max_attempts":   true,
	"queuedAt":       true,
	"queued_at":      true,
	"startedAt":      true,
	"started_at":     true,
	"finishedAt":     true,
	"finished_at":    true,
	"timeoutAt":      true,
	"timeout_at":     true,
	"errorCode":      true,
	"error_code":     true,
	"errorMessage":   true,
	"error_message":  true,
	"outputAssetIds": true,
	"cost":           true,
	"usage":          true,
}

func bindCreateRequest(c *gin.Context) (createRequest, error) {
	raw, err := c.GetRawData()
	if err != nil || len(raw) == 0 {
		return createRequest{}, ErrMalformedRequest
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return createRequest{}, ErrMalformedRequest
	}
	for key := range fields {
		if serverOwnedCreateFields[key] || !allowedCreateFields[key] {
			return createRequest{}, ErrValidation
		}
	}

	var request createRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return createRequest{}, ErrMalformedRequest
	}
	return request, nil
}

func normalizeCreateRequest(request createRequest, model database.AIModel, assets map[string]database.ImageAsset) (CreateInput, error) {
	taskType := cleanEnum(request.Type)
	if !validType(taskType) {
		return CreateInput{}, ErrValidation
	}
	prompt, err := cleanRequired(request.Prompt, maxPromptRunes)
	if err != nil {
		return CreateInput{}, err
	}
	providerID, err := cleanRequired(request.ProviderID, 128)
	if err != nil {
		return CreateInput{}, err
	}
	modelID, err := cleanRequired(request.ModelID, 128)
	if err != nil {
		return CreateInput{}, err
	}

	imageType, err := cleanImageType(request.ImageType)
	if err != nil {
		return CreateInput{}, err
	}

	inputAssetIDs, err := normalizeInputAssetIDs(taskType, request.ReferenceAssetIDs, request.EditSourceAssetID)
	if err != nil {
		return CreateInput{}, err
	}
	if err := validateModelCapability(taskType, model, inputAssetIDs); err != nil {
		return CreateInput{}, err
	}
	if err := validateInputAssets(taskType, request.ReferenceAssetIDs, request.EditSourceAssetID, assets); err != nil {
		return CreateInput{}, err
	}

	parameters, imageTypeFromParams, err := normalizeParameters(request.Parameters, model)
	if err != nil {
		return CreateInput{}, err
	}
	if imageTypeFromParams != "" {
		if imageType != "" && imageType != imageTypeFromParams {
			return CreateInput{}, ErrValidation
		}
		imageType = imageTypeFromParams
	}

	return CreateInput{
		Type:          taskType,
		Prompt:        prompt,
		ProviderID:    providerID,
		ModelID:       modelID,
		ImageType:     imageType,
		InputAssetIDs: inputAssetIDs,
		Parameters:    parameters,
	}, nil
}

func parseListQuery(c *gin.Context) (ListQuery, error) {
	pageNum := parsePositiveInt(c.Query("pageNum"), defaultPageNum)
	pageSize := parsePositiveInt(c.Query("pageSize"), defaultPageSize)
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	status := cleanEnum(c.Query("status"))
	if status != "" && !validStatus(status) {
		return ListQuery{}, ErrValidation
	}
	taskType := cleanEnum(c.Query("type"))
	if taskType != "" && !validType(taskType) {
		return ListQuery{}, ErrValidation
	}
	return ListQuery{PageNum: pageNum, PageSize: pageSize, Status: status, Type: taskType}, nil
}

func parseHistoryQuery(c *gin.Context) (HistoryQuery, error) {
	pageNum, err := parseStrictPositiveInt(c.Query("pageNum"), defaultPageNum)
	if err != nil {
		return HistoryQuery{}, err
	}
	pageSize, err := parseStrictPositiveInt(c.Query("pageSize"), defaultPageSize)
	if err != nil {
		return HistoryQuery{}, err
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	kind := cleanEnum(c.Query("kind"))
	if kind != "" && !validHistoryKind(kind) {
		return HistoryQuery{}, ErrValidation
	}
	return HistoryQuery{PageNum: pageNum, PageSize: pageSize, Kind: kind}, nil
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

func parseStrictPositiveInt(raw string, fallback int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, ErrValidation
	}
	return value, nil
}

func validHistoryKind(kind string) bool {
	switch kind {
	case assetpkg.KindGenerated, assetpkg.KindEdited:
		return true
	default:
		return false
	}
}

func normalizeInputAssetIDs(taskType string, referenceAssetIDs []string, editSourceAssetID string) ([]string, error) {
	if len(referenceAssetIDs) > maxReferenceAssetIDs {
		return nil, ErrValidation
	}
	ids := make([]string, 0, len(referenceAssetIDs)+1)
	if taskType == TypeImageEdit {
		editSourceAssetID = strings.TrimSpace(editSourceAssetID)
		if editSourceAssetID == "" || utf8.RuneCountInString(editSourceAssetID) > 128 {
			return nil, ErrValidation
		}
		ids = append(ids, editSourceAssetID)
	}
	for _, assetID := range referenceAssetIDs {
		assetID = strings.TrimSpace(assetID)
		if assetID == "" || utf8.RuneCountInString(assetID) > 128 {
			return nil, ErrValidation
		}
		ids = append(ids, assetID)
	}
	return uniqueStrings(ids), nil
}

func validateModelCapability(taskType string, model database.AIModel, inputAssetIDs []string) error {
	switch taskType {
	case TypeImageGeneration:
		if !model.SupportsGenerate {
			return ErrValidation
		}
	case TypeImageEdit:
		if !model.SupportsEdit {
			return ErrValidation
		}
	default:
		return ErrValidation
	}
	if len(inputAssetIDs) > 1 && !model.SupportsMultiReference {
		return ErrValidation
	}
	return nil
}

func validateInputAssets(taskType string, referenceAssetIDs []string, editSourceAssetID string, assets map[string]database.ImageAsset) error {
	for _, assetID := range referenceAssetIDs {
		assetID = strings.TrimSpace(assetID)
		record, ok := assets[assetID]
		if !ok || record.ID == "" {
			return ErrValidation
		}
		if record.Kind != assetpkg.KindReference {
			return ErrValidation
		}
	}
	if taskType != TypeImageEdit {
		return nil
	}
	editSourceAssetID = strings.TrimSpace(editSourceAssetID)
	record, ok := assets[editSourceAssetID]
	if !ok || record.ID == "" {
		return ErrValidation
	}
	if !validEditSourceKind(record.Kind) {
		return ErrValidation
	}
	return nil
}

func validEditSourceKind(kind string) bool {
	switch kind {
	case assetpkg.KindReference, assetpkg.KindGenerated, assetpkg.KindEdited:
		return true
	default:
		return false
	}
}

func normalizeParameters(raw map[string]any, model database.AIModel) (map[string]any, string, error) {
	if raw == nil {
		raw = map[string]any{}
	}
	sizes, err := decodeStringList(model.SupportedSizesJSON)
	if err != nil {
		return nil, "", ErrValidation
	}
	qualities, err := decodeStringList(model.SupportedQualitiesJSON)
	if err != nil {
		return nil, "", ErrValidation
	}
	formats, err := decodeStringList(model.SupportedOutputFormatsJSON)
	if err != nil {
		return nil, "", ErrValidation
	}

	clean := make(map[string]any, len(raw)+1)
	var imageType string
	var outputCount *int
	for key, value := range raw {
		if utf8.RuneCountInString(key) > maxParameterKeyRunes {
			return nil, "", ErrValidation
		}
		switch key {
		case "size":
			size, err := cleanParameterString(value, strings.ToLower)
			if err != nil || !containsString(sizes, size) {
				return nil, "", ErrValidation
			}
			clean["size"] = size
		case "quality":
			quality, err := cleanParameterString(value, strings.ToLower)
			if err != nil || len(qualities) == 0 || !containsString(qualities, quality) {
				return nil, "", ErrValidation
			}
			clean["quality"] = quality
		case "outputFormat":
			format, err := cleanParameterString(value, strings.ToLower)
			if err != nil {
				return nil, "", ErrValidation
			}
			if format == "jpg" {
				format = "jpeg"
			}
			if !containsString(formats, format) {
				return nil, "", ErrValidation
			}
			clean["outputFormat"] = format
		case "outputCount", "n":
			count, err := cleanPositiveInteger(value)
			if err != nil {
				return nil, "", ErrValidation
			}
			if outputCount != nil && *outputCount != count {
				return nil, "", ErrValidation
			}
			outputCount = &count
		case "aspectRatio":
			aspectRatio, err := cleanParameterString(value, strings.ToLower)
			if err != nil {
				return nil, "", ErrValidation
			}
			clean["aspectRatio"] = aspectRatio
		case "imageType":
			value, err := cleanParameterString(value, strings.ToUpper)
			if err != nil {
				return nil, "", ErrValidation
			}
			imageType, err = cleanImageType(value)
			if err != nil {
				return nil, "", err
			}
		default:
			return nil, "", ErrValidation
		}
	}

	count := 1
	if outputCount != nil {
		count = *outputCount
	}
	if count < 1 || count > model.MaxOutputCount {
		return nil, "", ErrValidation
	}
	if count > 1 && !model.SupportsN {
		return nil, "", ErrValidation
	}
	clean["outputCount"] = count

	return clean, imageType, nil
}

func cleanParameterString(value any, transform func(string) string) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", ErrValidation
	}
	text = strings.TrimSpace(text)
	if transform != nil {
		text = transform(text)
	}
	if text == "" || utf8.RuneCountInString(text) > maxParameterValueRunes {
		return "", ErrValidation
	}
	for _, r := range text {
		if r <= 31 || r == 127 {
			return "", ErrValidation
		}
	}
	return text, nil
}

func cleanPositiveInteger(value any) (int, error) {
	number, ok := value.(float64)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, ErrValidation
	}
	integer := int(number)
	if number != float64(integer) || integer < 1 {
		return 0, ErrValidation
	}
	return integer, nil
}

func cleanImageType(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	if utf8.RuneCountInString(value) > maxImageTypeRunes {
		return "", ErrValidation
	}
	switch value {
	case "MAIN", "A_PLUS", "SCENE", "DETAIL", "DIMENSION", "SELLING_POINT", "COMPARISON":
		return value, nil
	default:
		return "", ErrValidation
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

func validType(value string) bool {
	switch value {
	case TypeImageGeneration, TypeImageEdit:
		return true
	default:
		return false
	}
}

func validStatus(value string) bool {
	switch value {
	case StatusQueued, StatusRunning, StatusSucceeded, StatusFailed, StatusCancelled, StatusRetrying, StatusTimedOut:
		return true
	default:
		return false
	}
}

func terminalStatus(status string) bool {
	switch status {
	case StatusSucceeded, StatusFailed, StatusCancelled, StatusTimedOut:
		return true
	default:
		return false
	}
}

func retryableStatus(status string) bool {
	switch status {
	case StatusFailed, StatusCancelled, StatusTimedOut:
		return true
	default:
		return false
	}
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
		return []string{}, nil
	}
	return values, nil
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	unique := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}
