package settings

import (
	"errors"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
)

const (
	KeyUploadPolicy = "upload_policy"

	PermissionManage = "system:settings:manage"

	ActionUpdateSystemSettings = "system_settings.update"
)

var (
	ErrValidation = errors.New("invalid system settings request")
	ErrForbidden  = errors.New("system settings access forbidden")
)

type Response struct {
	UploadPolicy UploadPolicy `json:"uploadPolicy"`
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
