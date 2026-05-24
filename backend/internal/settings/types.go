package settings

import (
	"errors"
	"fmt"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
)

const (
	KeyUploadPolicy    = "upload_policy"
	KeyTaskDefaults    = "task_defaults"
	KeyTaskConcurrency = "task_concurrency"

	PermissionManage = "system:settings:manage"

	ActionUpdateSystemSettings = "system_settings.update"
)

var (
	ErrValidation                   = errors.New("invalid system settings request")
	ErrStoredTaskDefaultsInvalid    = fmt.Errorf("%w: invalid stored task defaults", ErrValidation)
	ErrStoredTaskConcurrencyInvalid = fmt.Errorf("%w: invalid stored task concurrency", ErrValidation)
	ErrForbidden                    = errors.New("system settings access forbidden")
)

type Response struct {
	UploadPolicy    UploadPolicy    `json:"uploadPolicy"`
	TaskDefaults    TaskDefaults    `json:"taskDefaults"`
	TaskConcurrency TaskConcurrency `json:"taskConcurrency"`
}

type UploadPolicy struct {
	MaxFileSizeBytes int64 `json:"maxFileSizeBytes"`
	MaxWidth         int   `json:"maxWidth"`
	MaxHeight        int   `json:"maxHeight"`
	MaxPixels        int64 `json:"maxPixels"`
}

type UploadPolicyPatch struct {
	MaxFileSizeBytes *int64
	MaxWidth         *int64
	MaxHeight        *int64
	MaxPixels        *int64
}

type TaskDefaults struct {
	DefaultProviderID *string `json:"defaultProviderId"`
	DefaultModelID    *string `json:"defaultModelId"`
}

type TaskDefaultsPatch struct {
	DefaultProviderID *string
	DefaultModelID    *string
}

type TaskConcurrency struct {
	TenantLimit   int `json:"tenantLimit"`
	UserLimit     int `json:"userLimit"`
	ProviderLimit int `json:"providerLimit"`
	ModelLimit    int `json:"modelLimit"`
}

type TaskConcurrencyPatch struct {
	TenantLimit   *int64
	UserLimit     *int64
	ProviderLimit *int64
	ModelLimit    *int64
}

type PatchRequest struct {
	UploadPolicy    *UploadPolicyPatch
	TaskDefaults    *TaskDefaultsPatch
	TaskConcurrency *TaskConcurrencyPatch
}

func uploadPolicyFromConfig(upload config.UploadConfig) UploadPolicy {
	upload = config.NormalizeUploadConfig(upload)
	return UploadPolicy{
		MaxFileSizeBytes: upload.MaxFileSizeBytes,
		MaxWidth:         upload.MaxWidth,
		MaxHeight:        upload.MaxHeight,
		MaxPixels:        upload.MaxPixels,
	}
}

func uploadConfigFromPolicy(base config.UploadConfig, policy UploadPolicy) config.UploadConfig {
	base = config.NormalizeUploadConfig(base)
	base.MaxFileSizeBytes = policy.MaxFileSizeBytes
	base.MaxWidth = policy.MaxWidth
	base.MaxHeight = policy.MaxHeight
	base.MaxPixels = policy.MaxPixels
	return base
}

func taskConcurrencyFromQueueConfig(queueConfig config.QueueConfig) TaskConcurrency {
	return normalizeTaskConcurrencyHardCap(TaskConcurrency{
		TenantLimit:   queueConfig.TenantConcurrency,
		UserLimit:     queueConfig.UserConcurrency,
		ProviderLimit: queueConfig.ProviderConcurrency,
		ModelLimit:    queueConfig.ModelConcurrency,
	})
}

func normalizeTaskConcurrencyHardCap(policy TaskConcurrency) TaskConcurrency {
	if policy.TenantLimit <= 0 {
		policy.TenantLimit = 1
	}
	if policy.UserLimit <= 0 {
		policy.UserLimit = 1
	}
	if policy.ProviderLimit <= 0 {
		policy.ProviderLimit = 1
	}
	if policy.ModelLimit <= 0 {
		policy.ModelLimit = 1
	}
	return policy
}
