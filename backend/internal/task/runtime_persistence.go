package task

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provideradapter"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/thumbnail"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/usagecost"
	"golang.org/x/image/webp"
	"gorm.io/gorm"
)

type validatedOutputImage struct {
	Data      []byte
	MIMEType  string
	Ext       string
	SizeBytes int64
	Width     int
	Height    int
	SHA256    string
	Metadata  map[string]any
}

type uploadedOutput struct {
	Index              int
	AssetID            string
	ObjectKey          string
	ThumbnailObjectKey string
	Image              validatedOutputImage
	Thumbnail          thumbnail.Image
}

func hasAPICall(call APICallResult) bool {
	return call.Status != "" || call.DurationMs > 0 || call.RequestID != "" || call.HTTPStatus != nil || call.ErrorCode != "" || call.ErrorMessage != "" || len(call.RequestMetadata) > 0 || len(call.ResponseMetadata) > 0
}

func (p *WorkerProcessor) recordAPICall(ctx context.Context, scope tenant.Scope, taskRecord database.GenerationTask, call APICallResult) error {
	if p == nil || p.db == nil {
		return database.ErrNilDB
	}
	status := strings.ToUpper(strings.TrimSpace(call.Status))
	if status == "" {
		status = provideradapter.APICallStatusFailure
	}
	now := p.now()
	record := database.APICallLog{
		ID:                   idgen.New(),
		TenantID:             scope.ID(),
		TaskID:               taskRecord.ID,
		ProviderID:           taskRecord.ProviderID,
		ModelID:              taskRecord.ModelID,
		Status:               status,
		DurationMs:           nonNegativeInt64(call.DurationMs),
		RequestID:            cleanWorkerMessage(call.RequestID),
		HTTPStatus:           call.HTTPStatus,
		ErrorCode:            cleanWorkerCode(call.ErrorCode, ""),
		ErrorMessage:         cleanWorkerMessage(provideradapter.SanitizeErrorMessage(call.ErrorMessage)),
		RedactedRequestJSON:  provideradapter.JSONString(call.RequestMetadata),
		RedactedResponseJSON: provideradapter.JSONString(call.ResponseMetadata),
		CreatedAt:            now,
	}
	if record.ErrorMessage == "Provider request failed." && status == provideradapter.APICallStatusSuccess {
		record.ErrorMessage = ""
	}
	return p.db.WithContext(ctx).Create(&record).Error
}

func (p *WorkerProcessor) persistSuccessfulResult(ctx context.Context, scope tenant.Scope, taskID string, modelRecord database.AIModel, result ExecutionResult) error {
	if len(result.Outputs) == 0 && result.Usage.ImageCount == 0 && result.Usage.InputTokens == 0 && result.Usage.OutputTokens == 0 && len(result.Usage.Raw) == 0 {
		return nil
	}
	current, err := p.repo.FindTask(ctx, scope, taskID)
	if err != nil {
		return err
	}
	if current.Status != StatusRunning {
		return nil
	}
	if len(result.Outputs) > 0 && p.store == nil {
		return storage.ErrUnavailable
	}

	uploaded := make([]uploadedOutput, 0, len(result.Outputs))
	pending := make([]uploadedOutput, 0, len(result.Outputs))
	var pendingBytes int64
	for index, output := range result.Outputs {
		exists, err := p.taskOutputExists(ctx, scope, current.ID, index)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		validated, err := p.validateOutputImage(output)
		if err != nil {
			p.log.Warn("provider output image validation failed", "task_id", current.ID, "output_index", index)
			return ErrValidation
		}
		assetID := idgen.New()
		objectKey := outputObjectKey(scope.ID(), current.ProjectID, assetID, validated.Ext)
		thumbnailImage, err := thumbnail.Generate(validated.Data, validated.MIMEType)
		if err != nil {
			p.log.Warn("provider output thumbnail generation failed", "task_id", current.ID, "output_index", index)
			return ErrValidation
		}
		pending = append(pending, uploadedOutput{
			Index:              index,
			AssetID:            assetID,
			ObjectKey:          objectKey,
			ThumbnailObjectKey: thumbnail.ObjectKey(scope.ID(), current.ProjectID, assetID, thumbnailImage.Ext),
			Image:              validated,
			Thumbnail:          thumbnailImage,
		})
		pendingBytes += validated.SizeBytes
	}
	if pendingBytes > 0 {
		if err := settings.CheckStorageQuota(ctx, settings.NewRepository(p.db), scope, pendingBytes); err != nil {
			return err
		}
	}
	for _, output := range pending {
		if err := p.store.PutObject(ctx, p.storage.BucketGenerated, output.ObjectKey, bytes.NewReader(output.Image.Data), output.Image.SizeBytes, output.Image.MIMEType); err != nil {
			p.cleanupUploadedOutputs(ctx, uploaded)
			return err
		}
		uploaded = append(uploaded, output)
		if err := p.store.PutObject(ctx, p.storage.BucketThumbnails, output.ThumbnailObjectKey, bytes.NewReader(output.Thumbnail.Data), output.Thumbnail.SizeBytes, output.Thumbnail.MIMEType); err != nil {
			p.cleanupUploadedOutputs(ctx, uploaded)
			return err
		}
	}

	var events []database.TaskEvent
	persistedObjects := map[string]bool{}
	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := p.repo.withDB(tx)
		taskRecord, err := repo.FindTask(ctx, scope, current.ID)
		if err != nil {
			return err
		}
		if taskRecord.Status != StatusRunning {
			return nil
		}
		now := p.now()
		for _, output := range uploaded {
			exists, err := p.taskOutputExistsWithDB(ctx, tx, scope, taskRecord.ID, output.Index)
			if err != nil {
				return err
			}
			if exists {
				continue
			}
			sourceTaskID := taskRecord.ID
			kind := asset.KindGenerated
			if taskRecord.Type == TypeImageEdit {
				kind = asset.KindEdited
			}
			imageAsset := database.ImageAsset{
				ID:                 output.AssetID,
				TenantID:           scope.ID(),
				ProjectID:          taskRecord.ProjectID,
				Kind:               kind,
				Category:           taskRecord.ImageType,
				Filename:           outputFilename(kind, output.Index, output.Image.Ext),
				ObjectKey:          output.ObjectKey,
				ThumbnailObjectKey: &output.ThumbnailObjectKey,
				MimeType:           output.Image.MIMEType,
				SizeBytes:          output.Image.SizeBytes,
				Width:              output.Image.Width,
				Height:             output.Image.Height,
				SHA256:             output.Image.SHA256,
				IsFavorite:         false,
				SourceTaskID:       &sourceTaskID,
				CreatedBy:          taskRecord.CreatedBy,
				CreatedAt:          now,
				UpdatedAt:          now,
			}
			if err := tx.Create(&imageAsset).Error; err != nil {
				return err
			}
			if err := tx.Create(&database.TaskOutput{
				ID:          idgen.New(),
				TenantID:    scope.ID(),
				TaskID:      taskRecord.ID,
				AssetID:     imageAsset.ID,
				OutputIndex: output.Index,
				CreatedAt:   now,
			}).Error; err != nil {
				return err
			}
			persistedObjects[output.ObjectKey] = true
			event, err := writeTaskEvent(ctx, repo, scope, taskRecord, EventImageOutput, map[string]any{
				"assetId":       imageAsset.ID,
				"previewUrl":    "/api/v1/assets/" + imageAsset.ID + "/download",
				"thumbnailUrl":  thumbnail.URL(imageAsset.ID),
				"width":         imageAsset.Width,
				"height":        imageAsset.Height,
				"mimeType":      imageAsset.MimeType,
				"outputIndex":   output.Index,
				"sourceTaskId":  taskRecord.ID,
				"assetKind":     imageAsset.Kind,
				"sizeBytes":     imageAsset.SizeBytes,
				"providerIndex": output.Image.Metadata["providerOutputIndex"],
			}, now)
			if err != nil {
				return err
			}
			events = append(events, event)
		}
		usage := result.Usage
		if usage.ImageCount == 0 && len(result.Outputs) > 0 {
			usage.ImageCount = len(result.Outputs)
		}
		if shouldCreateUsage(usage) {
			created, err := p.createUsageIfAbsent(ctx, tx, scope, taskRecord, modelRecord, usage, now)
			if err != nil {
				return err
			}
			if created.ID != "" {
				event, err := writeTaskEvent(ctx, repo, scope, taskRecord, EventUsageRecorded, map[string]any{
					"usageRecordId": created.ID,
					"inputTokens":   created.InputTokens,
					"outputTokens":  created.OutputTokens,
					"imageCount":    created.ImageCount,
					"estimatedCost": created.EstimatedCost,
					"currency":      created.Currency,
				}, now)
				if err != nil {
					return err
				}
				events = append(events, event)
			}
		}
		return nil
	})
	if err != nil {
		p.cleanupUploadedOutputs(ctx, uploaded)
		return err
	}
	p.cleanupUnpersistedOutputs(ctx, uploaded, persistedObjects)
	p.publishEvents(ctx, events)
	return nil
}

func (p *WorkerProcessor) taskOutputExists(ctx context.Context, scope tenant.Scope, taskID string, outputIndex int) (bool, error) {
	return p.taskOutputExistsWithDB(ctx, p.db, scope, taskID, outputIndex)
}

func (p *WorkerProcessor) taskOutputExistsWithDB(ctx context.Context, db *gorm.DB, scope tenant.Scope, taskID string, outputIndex int) (bool, error) {
	var count int64
	err := db.WithContext(ctx).Model(&database.TaskOutput{}).
		Where("tenant_id = ? AND task_id = ? AND output_index = ?", scope.ID(), taskID, outputIndex).
		Count(&count).Error
	return count > 0, err
}

func (p *WorkerProcessor) validateOutputImage(output GeneratedImageOutput) (validatedOutputImage, error) {
	data := output.Data
	if len(data) == 0 || int64(len(data)) > p.upload.MaxFileSizeBytes {
		return validatedOutputImage{}, ErrValidation
	}
	mimeType, ext, err := detectOutputImageType(data)
	if err != nil {
		return validatedOutputImage{}, err
	}
	if !uploadMIMEAllowed(p.upload.AllowedMIMETypes, mimeType) {
		return validatedOutputImage{}, ErrValidation
	}
	width, height, err := decodeOutputDimensions(mimeType, data)
	if err != nil {
		return validatedOutputImage{}, ErrValidation
	}
	if width <= 0 || height <= 0 || width > p.upload.MaxWidth || height > p.upload.MaxHeight || int64(width)*int64(height) > p.upload.MaxPixels {
		return validatedOutputImage{}, ErrValidation
	}
	sum := sha256.Sum256(data)
	return validatedOutputImage{
		Data:      data,
		MIMEType:  mimeType,
		Ext:       ext,
		SizeBytes: int64(len(data)),
		Width:     width,
		Height:    height,
		SHA256:    hex.EncodeToString(sum[:]),
		Metadata:  provideradapter.SanitizeMetadata(output.Metadata),
	}, nil
}

func (p *WorkerProcessor) createUsageIfAbsent(ctx context.Context, tx *gorm.DB, scope tenant.Scope, taskRecord database.GenerationTask, modelRecord database.AIModel, usage UsageResult, now time.Time) (database.UsageRecord, error) {
	var count int64
	if err := tx.WithContext(ctx).Model(&database.UsageRecord{}).
		Where("tenant_id = ? AND task_id = ?", scope.ID(), taskRecord.ID).
		Count(&count).Error; err != nil {
		return database.UsageRecord{}, err
	}
	if count > 0 {
		return database.UsageRecord{}, nil
	}
	cost := usagecost.Estimate(modelRecord.PricingJSON, usagecost.Usage{
		InputTokens:  nonNegativeInt64(usage.InputTokens),
		OutputTokens: nonNegativeInt64(usage.OutputTokens),
		ImageCount:   nonNegativeInt(usage.ImageCount),
	})
	record := database.UsageRecord{
		ID:            idgen.New(),
		TenantID:      scope.ID(),
		TaskID:        taskRecord.ID,
		UserID:        taskRecord.CreatedBy,
		ProjectID:     taskRecord.ProjectID,
		ProviderID:    taskRecord.ProviderID,
		ModelID:       taskRecord.ModelID,
		InputTokens:   nonNegativeInt64(usage.InputTokens),
		OutputTokens:  nonNegativeInt64(usage.OutputTokens),
		ImageCount:    nonNegativeInt(usage.ImageCount),
		EstimatedCost: cost.EstimatedCost,
		Currency:      cost.Currency,
		RawUsageJSON:  provideradapter.JSONString(usage.Raw),
		CreatedAt:     now,
	}
	if err := tx.WithContext(ctx).Create(&record).Error; err != nil {
		return database.UsageRecord{}, err
	}
	return record, nil
}

func shouldCreateUsage(usage UsageResult) bool {
	return usage.ImageCount > 0 || usage.InputTokens > 0 || usage.OutputTokens > 0 || len(usage.Raw) > 0
}

func (p *WorkerProcessor) cleanupUploadedOutputs(ctx context.Context, outputs []uploadedOutput) {
	if p == nil || p.store == nil {
		return
	}
	for _, output := range outputs {
		if strings.TrimSpace(output.ObjectKey) == "" {
			continue
		}
		if err := p.store.RemoveObject(ctx, p.storage.BucketGenerated, output.ObjectKey); err != nil {
			p.log.Warn("generated output cleanup failed", "asset_id", output.AssetID, "error_kind", safeWorkerCleanupErrorKind(err))
		}
		if strings.TrimSpace(output.ThumbnailObjectKey) != "" {
			if err := p.store.RemoveObject(ctx, p.storage.BucketThumbnails, output.ThumbnailObjectKey); err != nil {
				p.log.Warn("generated thumbnail cleanup failed", "asset_id", output.AssetID, "error_kind", safeWorkerCleanupErrorKind(err))
			}
		}
	}
}

func (p *WorkerProcessor) cleanupUnpersistedOutputs(ctx context.Context, outputs []uploadedOutput, persisted map[string]bool) {
	if p == nil || p.store == nil {
		return
	}
	for _, output := range outputs {
		if strings.TrimSpace(output.ObjectKey) == "" || persisted[output.ObjectKey] {
			continue
		}
		if err := p.store.RemoveObject(ctx, p.storage.BucketGenerated, output.ObjectKey); err != nil {
			p.log.Warn("unpersisted generated output cleanup failed", "asset_id", output.AssetID, "error_kind", safeWorkerCleanupErrorKind(err))
		}
		if strings.TrimSpace(output.ThumbnailObjectKey) != "" {
			if err := p.store.RemoveObject(ctx, p.storage.BucketThumbnails, output.ThumbnailObjectKey); err != nil {
				p.log.Warn("unpersisted generated thumbnail cleanup failed", "asset_id", output.AssetID, "error_kind", safeWorkerCleanupErrorKind(err))
			}
		}
	}
}

func safeWorkerCleanupErrorKind(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "context_deadline_exceeded"
	case errors.Is(err, storage.ErrNotFound):
		return "storage_not_found"
	case errors.Is(err, storage.ErrUnavailable):
		return "storage_unavailable"
	default:
		return "internal_error"
	}
}

func detectOutputImageType(data []byte) (string, string, error) {
	switch {
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg", "jpg", nil
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return "image/png", "png", nil
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp", "webp", nil
	default:
		return "", "", ErrValidation
	}
}

func decodeOutputDimensions(mimeType string, data []byte) (int, int, error) {
	switch mimeType {
	case "image/jpeg":
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
		return cfg.Width, cfg.Height, err
	case "image/png":
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		return cfg.Width, cfg.Height, err
	case "image/webp":
		cfg, err := webp.DecodeConfig(bytes.NewReader(data))
		return cfg.Width, cfg.Height, err
	default:
		return 0, 0, ErrValidation
	}
}

func uploadMIMEAllowed(values []string, mimeType string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), mimeType) {
			return true
		}
	}
	return false
}

func outputObjectKey(tenantID string, projectID string, assetID string, ext string) string {
	return "tenants/" + tenantID + "/projects/" + projectID + "/assets/" + assetID + "/original." + ext
}

func outputFilename(kind string, index int, ext string) string {
	prefix := "generated"
	if kind == asset.KindEdited {
		prefix = "edited"
	}
	return fmt.Sprintf("%s-%d.%s", prefix, index+1, ext)
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
