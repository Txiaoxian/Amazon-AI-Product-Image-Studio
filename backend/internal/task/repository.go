package task

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

type EventFilter struct {
	ProjectID string
	TaskID    string
}

type HistoryPair struct {
	AssetID string
	TaskID  string
}

func NewRepository(db *gorm.DB) Repository {
	return Repository{db: db}
}

func (r Repository) withDB(db *gorm.DB) Repository {
	return Repository{db: db}
}

func (r Repository) base(ctx context.Context, scope tenant.Scope) (*gorm.DB, error) {
	if r.db == nil {
		return nil, database.ErrNilDB
	}
	if !scope.Valid() {
		return nil, tenant.ErrMissingTenantID
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return r.db.WithContext(ctx), nil
}

func (r Repository) ListTasks(ctx context.Context, scope tenant.Scope, projectID string, options ListOptions) ([]database.GenerationTask, int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, 0, err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, 0, ErrValidation
	}

	query := db.Model(&database.GenerationTask{}).
		Where("tenant_id = ? AND project_id = ?", scope.ID(), projectID)
	if options.Status != "" {
		query = query.Where("status = ?", options.Status)
	}
	if options.Type != "" {
		query = query.Where("type = ?", options.Type)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []database.GenerationTask
	offset := (options.PageNum - 1) * options.PageSize
	if err := query.
		Order("created_at DESC, id DESC").
		Limit(options.PageSize).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r Repository) ListHistoryPairs(ctx context.Context, scope tenant.Scope, projectID string, options HistoryOptions) ([]HistoryPair, int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, 0, err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, 0, ErrValidation
	}

	kinds := []string{"GENERATED", "EDITED"}
	if options.Kind != "" {
		kinds = []string{options.Kind}
	}

	query := db.Table("task_outputs AS task_outputs").
		Joins("JOIN image_assets AS image_assets ON image_assets.tenant_id = task_outputs.tenant_id AND image_assets.id = task_outputs.asset_id").
		Joins("JOIN generation_tasks AS generation_tasks ON generation_tasks.tenant_id = task_outputs.tenant_id AND generation_tasks.id = task_outputs.task_id").
		Where("task_outputs.tenant_id = ? AND image_assets.tenant_id = ? AND generation_tasks.tenant_id = ?", scope.ID(), scope.ID(), scope.ID()).
		Where("image_assets.project_id = ? AND generation_tasks.project_id = ?", projectID, projectID).
		Where("image_assets.deleted_at IS NULL").
		Where("image_assets.kind IN ?", kinds)
	if options.ImageType != "" {
		query = query.Where("generation_tasks.image_type = ?", options.ImageType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var pairs []HistoryPair
	offset := (options.PageNum - 1) * options.PageSize
	if err := query.
		Select("image_assets.id AS asset_id, generation_tasks.id AS task_id").
		Order("image_assets.created_at DESC, image_assets.id DESC, task_outputs.output_index ASC, task_outputs.id DESC").
		Limit(options.PageSize).
		Offset(offset).
		Scan(&pairs).Error; err != nil {
		return nil, 0, err
	}
	return pairs, total, nil
}

func (r Repository) FindHistoryAssets(ctx context.Context, scope tenant.Scope, projectID string, assetIDs []string) (map[string]database.ImageAsset, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	projectID = strings.TrimSpace(projectID)
	assetIDs = uniqueStrings(assetIDs)
	if projectID == "" {
		return nil, ErrValidation
	}
	if len(assetIDs) == 0 {
		return map[string]database.ImageAsset{}, nil
	}

	var records []database.ImageAsset
	if err := db.Model(&database.ImageAsset{}).
		Where("tenant_id = ? AND project_id = ? AND id IN ? AND deleted_at IS NULL", scope.ID(), projectID, assetIDs).
		Where("kind IN ?", []string{"GENERATED", "EDITED"}).
		Find(&records).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]database.ImageAsset, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}
	return byID, nil
}

func (r Repository) FindHistoryTasks(ctx context.Context, scope tenant.Scope, projectID string, taskIDs []string) (map[string]database.GenerationTask, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	projectID = strings.TrimSpace(projectID)
	taskIDs = uniqueStrings(taskIDs)
	if projectID == "" {
		return nil, ErrValidation
	}
	if len(taskIDs) == 0 {
		return map[string]database.GenerationTask{}, nil
	}

	var records []database.GenerationTask
	if err := db.Model(&database.GenerationTask{}).
		Where("tenant_id = ? AND project_id = ? AND id IN ?", scope.ID(), projectID, taskIDs).
		Find(&records).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]database.GenerationTask, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}
	return byID, nil
}

func (r Repository) FindTask(ctx context.Context, scope tenant.Scope, taskID string) (database.GenerationTask, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.GenerationTask{}, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return database.GenerationTask{}, ErrValidation
	}

	var record database.GenerationTask
	err = db.Model(&database.GenerationTask{}).
		Where("tenant_id = ? AND id = ?", scope.ID(), taskID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.GenerationTask{}, ErrNotFound
	}
	return record, err
}

func (r Repository) ResolveTaskTenantID(ctx context.Context, taskID string) (string, error) {
	if r.db == nil {
		return "", database.ErrNilDB
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", ErrValidation
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Queue payloads intentionally carry only task IDs; resolve tenant_id first,
	// then perform all full task reads and mutations through tenant-scoped paths.
	var record struct {
		TenantID string
	}
	err := r.db.WithContext(ctx).
		Model(&database.GenerationTask{}).
		Select("tenant_id").
		Where("id = ?", taskID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(record.TenantID) == "" {
		return "", ErrNotFound
	}
	return record.TenantID, nil
}

func (r Repository) CreateTask(ctx context.Context, scope tenant.Scope, record *database.GenerationTask) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	record.TenantID = scope.ID()
	return db.Create(record).Error
}

func (r Repository) UpdateTask(ctx context.Context, scope tenant.Scope, taskID string, allowedStatuses []string, updates map[string]any) (database.GenerationTask, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.GenerationTask{}, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || len(allowedStatuses) == 0 || len(updates) == 0 {
		return database.GenerationTask{}, ErrValidation
	}

	result := db.Model(&database.GenerationTask{}).
		Where("tenant_id = ? AND id = ? AND status IN ?", scope.ID(), taskID, allowedStatuses).
		Updates(updates)
	if result.Error != nil {
		return database.GenerationTask{}, result.Error
	}
	if result.RowsAffected == 0 {
		return database.GenerationTask{}, ErrInvalidTransition
	}

	return r.FindTask(ctx, scope, taskID)
}

func (r Repository) FindTenant(ctx context.Context, tenantID string) (database.Tenant, error) {
	if r.db == nil {
		return database.Tenant{}, database.ErrNilDB
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return database.Tenant{}, tenant.ErrMissingTenantID
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var record database.Tenant
	err := r.db.WithContext(ctx).
		Model(&database.Tenant{}).
		Where("id = ?", tenantID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.Tenant{}, ErrNotFound
	}
	return record, err
}

func (r Repository) FindProject(ctx context.Context, scope tenant.Scope, projectID string) (database.Project, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.Project{}, err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return database.Project{}, ErrValidation
	}

	var record database.Project
	err = db.Model(&database.Project{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), projectID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.Project{}, ErrNotFound
	}
	return record, err
}

func (r Repository) CreateEvent(ctx context.Context, scope tenant.Scope, record *database.TaskEvent) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	record.TenantID = scope.ID()
	record.ID = pendingTaskEventID()
	record.Sequence = 0
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		if record.Sequence == 0 {
			return ErrValidation
		}

		stableID := EventIDFromSequence(record.Sequence)
		result := tx.Model(&database.TaskEvent{}).
			Where("tenant_id = ? AND sequence = ?", scope.ID(), record.Sequence).
			Update("id", stableID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		record.ID = stableID
		return nil
	})
}

func (r Repository) ListRunningTimedOutTasks(ctx context.Context, now time.Time, limit int) ([]database.GenerationTask, error) {
	if r.db == nil {
		return nil, database.ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 100
	}

	// Recovery scans global overdue candidates, then each timeout transition is
	// applied through tenant-scoped UpdateTask in the worker processor.
	var records []database.GenerationTask
	if err := r.db.WithContext(ctx).
		Model(&database.GenerationTask{}).
		Where("status = ? AND timeout_at IS NOT NULL AND timeout_at <= ?", StatusRunning, now.UTC()).
		Order("timeout_at ASC, id ASC").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (r Repository) ListDeliveryRepairCandidates(ctx context.Context, afterID string, limit int) ([]database.GenerationTask, error) {
	if r.db == nil {
		return nil, database.ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 100
	}

	// This is an explicit global worker recovery boundary. The worker only uses
	// candidate IDs to repair temporary Redis delivery state; MySQL remains the
	// authoritative source for every tenant-scoped task execution transition.
	query := r.db.WithContext(ctx).
		Model(&database.GenerationTask{}).
		Where("status IN ?", []string{StatusQueued, StatusRetrying})
	if afterID = strings.TrimSpace(afterID); afterID != "" {
		query = query.Where("id > ?", afterID)
	}

	var records []database.GenerationTask
	if err := query.
		Order("id ASC").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (r Repository) ListEventsAfter(ctx context.Context, scope tenant.Scope, cursor uint64, filter EventFilter, limit int) ([]database.TaskEvent, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}

	query := db.Model(&database.TaskEvent{}).
		Where("tenant_id = ? AND sequence > ?", scope.ID(), cursor)
	if projectID := strings.TrimSpace(filter.ProjectID); projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	if taskID := strings.TrimSpace(filter.TaskID); taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	var events []database.TaskEvent
	if err := query.Order("sequence ASC").Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (r Repository) OutputAssetIDs(ctx context.Context, scope tenant.Scope, taskID string) ([]string, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, ErrValidation
	}

	var records []database.TaskOutput
	if err := db.Model(&database.TaskOutput{}).
		Where("tenant_id = ? AND task_id = ?", scope.ID(), taskID).
		Order("output_index ASC").
		Find(&records).Error; err != nil {
		return nil, err
	}
	outputs := make([]string, 0, len(records))
	for _, record := range records {
		outputs = append(outputs, record.AssetID)
	}
	return outputs, nil
}

func (r Repository) VisibleHistoryOutputAssetIDsByTask(ctx context.Context, scope tenant.Scope, projectID string, taskIDs []string) (map[string][]string, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	projectID = strings.TrimSpace(projectID)
	taskIDs = uniqueStrings(taskIDs)
	if projectID == "" {
		return nil, ErrValidation
	}
	outputs := make(map[string][]string, len(taskIDs))
	for _, taskID := range taskIDs {
		outputs[taskID] = []string{}
	}
	if len(taskIDs) == 0 {
		return outputs, nil
	}

	var rows []struct {
		TaskID  string
		AssetID string
	}
	if err := db.Table("task_outputs AS task_outputs").
		Select("task_outputs.task_id AS task_id, image_assets.id AS asset_id").
		Joins("JOIN image_assets AS image_assets ON image_assets.tenant_id = task_outputs.tenant_id AND image_assets.id = task_outputs.asset_id").
		Joins("JOIN generation_tasks AS generation_tasks ON generation_tasks.tenant_id = task_outputs.tenant_id AND generation_tasks.id = task_outputs.task_id").
		Where("task_outputs.tenant_id = ? AND image_assets.tenant_id = ? AND generation_tasks.tenant_id = ?", scope.ID(), scope.ID(), scope.ID()).
		Where("generation_tasks.project_id = ? AND image_assets.project_id = ?", projectID, projectID).
		Where("task_outputs.task_id IN ?", taskIDs).
		Where("image_assets.deleted_at IS NULL").
		Where("image_assets.kind IN ?", []string{"GENERATED", "EDITED"}).
		Order("task_outputs.task_id ASC, task_outputs.output_index ASC, image_assets.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		outputs[row.TaskID] = append(outputs[row.TaskID], row.AssetID)
	}
	return outputs, nil
}

func (r Repository) FindProvider(ctx context.Context, scope tenant.Scope, providerID string) (database.AIProvider, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.AIProvider{}, err
	}
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return database.AIProvider{}, ErrValidation
	}

	var record database.AIProvider
	err = db.Model(&database.AIProvider{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), providerID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.AIProvider{}, ErrValidation
	}
	return record, err
}

func (r Repository) FindModel(ctx context.Context, scope tenant.Scope, modelID string) (database.AIModel, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.AIModel{}, err
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return database.AIModel{}, ErrValidation
	}

	var record database.AIModel
	err = db.Model(&database.AIModel{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), modelID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.AIModel{}, ErrValidation
	}
	return record, err
}

func (r Repository) UserCanAccessModel(ctx context.Context, scope tenant.Scope, userID string, modelID string) (bool, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return false, err
	}
	userID = strings.TrimSpace(userID)
	modelID = strings.TrimSpace(modelID)
	if userID == "" || modelID == "" {
		return false, ErrValidation
	}
	var count int64
	if err := db.Model(&database.UserModelAccessGrant{}).
		Where("tenant_id = ? AND user_id = ? AND model_id = ?", scope.ID(), userID, modelID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r Repository) FindInputAssets(ctx context.Context, scope tenant.Scope, projectID string, assetIDs []string) (map[string]database.ImageAsset, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, ErrValidation
	}
	assetIDs = uniqueStrings(assetIDs)
	if len(assetIDs) == 0 {
		return map[string]database.ImageAsset{}, nil
	}

	var records []database.ImageAsset
	if err := db.Model(&database.ImageAsset{}).
		Where("tenant_id = ? AND project_id = ? AND id IN ? AND deleted_at IS NULL", scope.ID(), projectID, assetIDs).
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
	return byID, nil
}
