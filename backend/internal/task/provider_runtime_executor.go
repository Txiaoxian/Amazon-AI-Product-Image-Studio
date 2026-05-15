package task

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provideradapter"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
)

type apiKeyDecrypter interface {
	Decrypt(string) (string, error)
}

type ProviderRuntimeExecutorOptions struct {
	Runtime       provideradapter.Runtime
	Decrypter     apiKeyDecrypter
	URLValidator  *provider.URLValidator
	Provider      config.ProviderConfig
	Storage       config.StorageConfig
	Upload        config.UploadConfig
	Store         storage.ObjectStore
	MaxInputBytes int64
}

type ProviderRuntimeExecutor struct {
	db            *gorm.DB
	runtime       provideradapter.Runtime
	decrypter     apiKeyDecrypter
	validator     *provider.URLValidator
	storage       config.StorageConfig
	store         storage.ObjectStore
	maxInputBytes int64
}

func NewProviderRuntimeExecutor(db *gorm.DB, log *slog.Logger, options ProviderRuntimeExecutorOptions) (*ProviderRuntimeExecutor, error) {
	if log == nil {
		log = slog.Default()
	}
	decrypter := options.Decrypter
	if decrypter == nil {
		cipher, err := provider.NewAPIKeyCipher(options.Provider.APIKeyEncryptionKey, options.Provider.APIKeyEncryptionKeyID)
		if err != nil {
			return nil, err
		}
		decrypter = cipher
	}
	validator := options.URLValidator
	if validator == nil {
		validator = provider.NewURLValidator(nil)
	}
	runtime := options.Runtime
	if runtime == nil {
		httpClient := provider.NewSafeHTTPClient(validator, options.Provider.DefaultTimeout)
		runtime = provideradapter.NewClient(provideradapter.ClientOptions{HTTPClient: httpClient})
	}
	upload := config.NormalizeUploadConfig(options.Upload)
	maxInputBytes := options.MaxInputBytes
	if maxInputBytes <= 0 {
		maxInputBytes = upload.MaxFileSizeBytes
	}
	return &ProviderRuntimeExecutor{
		db:            db,
		runtime:       runtime,
		decrypter:     decrypter,
		validator:     validator,
		storage:       config.NormalizeStorageConfig(options.Storage),
		store:         options.Store,
		maxInputBytes: maxInputBytes,
	}, nil
}

func (e *ProviderRuntimeExecutor) Execute(ctx context.Context, execution ExecutionContext) ExecutionResult {
	if e == nil || e.db == nil || e.runtime == nil {
		return ExecutionResult{ErrorCode: "PROVIDER_RUNTIME_UNAVAILABLE", ErrorMessage: "Provider runtime is unavailable.", Retryable: true}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := e.validator.Validate(ctx, execution.Provider.BaseURL); err != nil {
		return ExecutionResult{ErrorCode: "PROVIDER_BASE_URL_INVALID", ErrorMessage: "Provider base URL failed security validation."}
	}
	apiKey, err := e.decrypter.Decrypt(execution.Provider.EncryptedAPIKey)
	if err != nil {
		return ExecutionResult{ErrorCode: "PROVIDER_CREDENTIAL_UNAVAILABLE", ErrorMessage: "Provider credentials are unavailable."}
	}
	parameters, err := taskParameters(execution.Task)
	if err != nil {
		return ExecutionResult{ErrorCode: "TASK_PARAMETERS_INVALID", ErrorMessage: "Task parameters are invalid."}
	}
	inputImages, err := e.loadInputImages(ctx, execution.Task)
	if err != nil {
		return ExecutionResult{ErrorCode: "TASK_INPUT_UNAVAILABLE", ErrorMessage: "Task input images are unavailable.", Retryable: true}
	}
	operation := provideradapter.OperationGenerate
	if execution.Task.Type == TypeImageEdit {
		operation = provideradapter.OperationEdit
	}

	result, err := e.runtime.Execute(ctx, provideradapter.ImageRequest{
		TenantID:    execution.Task.TenantID,
		ProjectID:   execution.Task.ProjectID,
		TaskID:      execution.Task.ID,
		Operation:   operation,
		Prompt:      execution.Task.Prompt,
		ImageType:   execution.Task.ImageType,
		Provider:    provideradapter.ProviderConfig{ID: execution.Provider.ID, Type: execution.Provider.Type, BaseURL: execution.Provider.BaseURL, APIKey: apiKey, TimeoutSeconds: execution.Provider.TimeoutSeconds},
		Model:       provideradapter.ModelConfig{ID: execution.Model.ID, ModelName: execution.Model.ModelName},
		Parameters:  parameters,
		InputImages: inputImages,
	})
	executionResult := executionResultFromProvider(result)
	if err != nil {
		var providerErr provideradapter.ProviderError
		if errors.As(err, &providerErr) {
			executionResult.ErrorCode = provideradapter.ErrorCode(providerErr.Code, "PROVIDER_CALL_FAILED")
			executionResult.ErrorMessage = provideradapter.SanitizeErrorMessage(providerErr.Message)
			executionResult.Retryable = providerErr.Retryable
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			executionResult.TimedOut = true
			executionResult.ErrorCode = "TASK_TIMED_OUT"
			executionResult.ErrorMessage = "Task execution timed out."
		} else {
			executionResult.ErrorCode = "PROVIDER_CALL_FAILED"
			executionResult.ErrorMessage = provideradapter.SanitizeErrorMessage(err.Error())
			executionResult.Retryable = true
		}
	}
	return executionResult
}

func (e *ProviderRuntimeExecutor) loadInputImages(ctx context.Context, taskRecord database.GenerationTask) ([]provideradapter.InputImage, error) {
	assetIDs, err := taskInputAssetIDs(taskRecord)
	if err != nil {
		return nil, err
	}
	if len(assetIDs) == 0 {
		return nil, nil
	}
	if e.store == nil {
		return nil, storage.ErrUnavailable
	}
	scope, err := tenant.NewScope(taskRecord.TenantID)
	if err != nil {
		return nil, err
	}
	var records []database.ImageAsset
	if err := e.db.WithContext(ctx).Model(&database.ImageAsset{}).
		Where("tenant_id = ? AND project_id = ? AND id IN ? AND deleted_at IS NULL", scope.ID(), taskRecord.ProjectID, assetIDs).
		Find(&records).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]database.ImageAsset, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}
	if len(byID) != len(assetIDs) {
		return nil, ErrValidation
	}
	images := make([]provideradapter.InputImage, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		record := byID[assetID]
		object, err := e.store.GetObject(ctx, bucketForAssetKind(e.storage, record.Kind), record.ObjectKey)
		if err != nil {
			return nil, err
		}
		data, readErr := readInputObject(object.Body, e.maxInputBytes)
		closeErr := object.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		images = append(images, provideradapter.InputImage{
			Data:     data,
			MIMEType: record.MimeType,
			Filename: record.Filename,
		})
	}
	return images, nil
}

func executionResultFromProvider(result provideradapter.ImageResult) ExecutionResult {
	outputs := make([]GeneratedImageOutput, 0, len(result.Images))
	for _, image := range result.Images {
		outputs = append(outputs, GeneratedImageOutput{
			Data:     image.Data,
			MIMEType: image.MIMEType,
			Metadata: provideradapter.SanitizeMetadata(image.Metadata),
		})
	}
	return ExecutionResult{
		Outputs: outputs,
		Usage: UsageResult{
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
			ImageCount:   result.Usage.ImageCount,
			Raw:          provideradapter.SanitizeMetadata(result.Usage.Raw),
		},
		APICall: APICallResult{
			Status:           result.APICall.Status,
			DurationMs:       result.APICall.DurationMs,
			RequestID:        result.APICall.RequestID,
			HTTPStatus:       result.APICall.HTTPStatus,
			ErrorCode:        result.APICall.ErrorCode,
			ErrorMessage:     result.APICall.ErrorMessage,
			RequestMetadata:  provideradapter.SanitizeMetadata(result.APICall.RequestMetadata),
			ResponseMetadata: provideradapter.SanitizeMetadata(result.APICall.ResponseMetadata),
		},
	}
}

func taskParameters(taskRecord database.GenerationTask) (map[string]any, error) {
	parameters := map[string]any{}
	if strings.TrimSpace(taskRecord.ParamsJSON) == "" {
		return parameters, nil
	}
	if err := json.Unmarshal([]byte(taskRecord.ParamsJSON), &parameters); err != nil {
		return nil, err
	}
	if parameters == nil {
		parameters = map[string]any{}
	}
	return parameters, nil
}

func taskInputAssetIDs(taskRecord database.GenerationTask) ([]string, error) {
	var assetIDs []string
	if strings.TrimSpace(taskRecord.InputAssetIDsJSON) == "" {
		return []string{}, nil
	}
	if err := json.Unmarshal([]byte(taskRecord.InputAssetIDsJSON), &assetIDs); err != nil {
		return nil, err
	}
	return uniqueStrings(assetIDs), nil
}

func readInputObject(reader io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, ErrValidation
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrValidation
	}
	return data, nil
}

func bucketForAssetKind(storageConfig config.StorageConfig, kind string) string {
	storageConfig = config.NormalizeStorageConfig(storageConfig)
	switch kind {
	case "GENERATED", "EDITED":
		return storageConfig.BucketGenerated
	default:
		return storageConfig.BucketOriginals
	}
}
