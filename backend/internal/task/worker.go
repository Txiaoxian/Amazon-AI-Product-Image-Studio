package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	modelpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	providerpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/queue"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
)

const (
	claimActionAck claimAction = iota
	claimActionRetry
	claimActionNone

	defaultRecoveryBatchSize = 100
	defaultWorkerConcurrency = 1
	maxWorkerConcurrency     = 256
)

var errTaskConcurrencyPolicyUnavailable = errors.New("task concurrency policy unavailable")

type claimAction int

type Worker struct {
	queue     queue.ReliableTaskQueue
	processor *WorkerProcessor
	log       *slog.Logger
	options   WorkerOptions
}

type WorkerOptions struct {
	RetryBackoff     time.Duration
	RecoveryInterval time.Duration
	RecoveryBatch    int
	Concurrency      int
}

type WorkerProcessorOptions struct {
	Limiter             queue.ConcurrencyLimiter
	EventPublisher      EventPublisher
	Executor            Executor
	ConcurrencyLeaseTTL time.Duration
	GlobalConcurrency   int
	TenantConcurrency   int
	UserConcurrency     int
	ProviderConcurrency int
	ModelConcurrency    int
	RetryBackoff        time.Duration
	DisableAutoRetry    bool
	RecoveryBatch       int
	Store               storage.ObjectStore
	StorageConfig       config.StorageConfig
	UploadConfig        config.UploadConfig
	Now                 func() time.Time
}

type ProcessResult struct {
	Action     claimAction
	RetryDelay time.Duration
}

type ExecutionContext struct {
	Task     database.GenerationTask
	Tenant   database.Tenant
	Project  database.Project
	Provider database.AIProvider
	Model    database.AIModel
}

type ProgressUpdate struct {
	Percent int
	Message string
}

type ExecutionResult struct {
	Progress     []ProgressUpdate
	Outputs      []GeneratedImageOutput
	Usage        UsageResult
	APICall      APICallResult
	ErrorCode    string
	ErrorMessage string
	Retryable    bool
	TimedOut     bool
}

type Executor interface {
	Execute(ctx context.Context, execution ExecutionContext) ExecutionResult
}

type StubExecutor struct{}

type WorkerProcessor struct {
	db        *gorm.DB
	repo      Repository
	log       *slog.Logger
	options   WorkerProcessorOptions
	publisher EventPublisher
	limiter   queue.ConcurrencyLimiter
	executor  Executor
	store     storage.ObjectStore
	storage   config.StorageConfig
	upload    config.UploadConfig
	now       func() time.Time
}

func NewWorker(taskQueue queue.ReliableTaskQueue, processor *WorkerProcessor, log *slog.Logger, options WorkerOptions) *Worker {
	if log == nil {
		log = slog.Default()
	}
	if options.RecoveryInterval <= 0 {
		options.RecoveryInterval = 30 * time.Second
	}
	if options.RetryBackoff <= 0 {
		options.RetryBackoff = 5 * time.Second
	}
	if options.RecoveryBatch <= 0 {
		options.RecoveryBatch = defaultRecoveryBatchSize
	}
	if options.Concurrency <= 0 {
		options.Concurrency = defaultWorkerConcurrency
	}
	if options.Concurrency > maxWorkerConcurrency {
		options.Concurrency = maxWorkerConcurrency
	}
	return &Worker{queue: taskQueue, processor: processor, log: log, options: options}
}

func NewWorkerProcessor(db *gorm.DB, log *slog.Logger, options WorkerProcessorOptions) *WorkerProcessor {
	if log == nil {
		log = slog.Default()
	}
	if options.Executor == nil {
		options.Executor = StubExecutor{}
	}
	if options.ConcurrencyLeaseTTL <= 0 {
		options.ConcurrencyLeaseTTL = 10 * time.Minute
	}
	if options.RetryBackoff <= 0 {
		options.RetryBackoff = 5 * time.Second
	}
	if options.GlobalConcurrency <= 0 {
		options.GlobalConcurrency = 1
	}
	if options.TenantConcurrency <= 0 {
		options.TenantConcurrency = 1
	}
	if options.UserConcurrency <= 0 {
		options.UserConcurrency = 1
	}
	if options.ProviderConcurrency <= 0 {
		options.ProviderConcurrency = 1
	}
	if options.ModelConcurrency <= 0 {
		options.ModelConcurrency = 1
	}
	if options.RecoveryBatch <= 0 {
		options.RecoveryBatch = defaultRecoveryBatchSize
	}
	now := options.Now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &WorkerProcessor{
		db:        db,
		repo:      NewRepository(db),
		log:       log,
		options:   options,
		publisher: options.EventPublisher,
		limiter:   options.Limiter,
		executor:  options.Executor,
		store:     options.Store,
		storage:   config.NormalizeStorageConfig(options.StorageConfig),
		upload:    config.NormalizeUploadConfig(options.UploadConfig),
		now:       now,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	if w == nil || w.queue == nil || w.processor == nil {
		return ErrQueueUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	loopCount := w.options.Concurrency
	workerCount := loopCount + 1
	errCh := make(chan error, workerCount)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- w.runRecoveryLoop(runCtx)
	}()

	for loopID := 1; loopID <= loopCount; loopID++ {
		loopID := loopID
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- w.runProcessingLoop(runCtx, loopID)
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	var runErr error
	for remaining := workerCount; remaining > 0; remaining-- {
		select {
		case err := <-errCh:
			if err != nil && runErr == nil {
				runErr = err
			}
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				cancel()
				<-done
				return err
			}
		case <-ctx.Done():
			cancel()
			<-done
			return ctx.Err()
		}
	}

	<-done
	if runErr != nil {
		return runErr
	}
	return ctx.Err()
}

func (w *Worker) runRecoveryLoop(ctx context.Context) error {
	recoveryTicker := time.NewTicker(w.options.RecoveryInterval)
	defer recoveryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-recoveryTicker.C:
			if err := w.processor.Recover(ctx, w.queue, w.options.RecoveryBatch); err != nil && !errors.Is(err, context.Canceled) {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				w.log.Warn("worker recovery failed", slog.String("error", err.Error()))
			}
		}
	}
}

func (w *Worker) runProcessingLoop(ctx context.Context, loopID int) error {
	for {
		claim, err := w.queue.Claim(ctx)
		if errors.Is(err, queue.ErrNoTask) {
			continue
		}
		if errors.Is(err, queue.ErrDeadLettered) {
			if markErr := w.processor.MarkDeadLettered(ctx, claim.TaskID); markErr != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				w.log.Warn("dead-letter task mark failed", slog.Int("worker_loop", loopID), slog.String("task_id", claim.TaskID), slog.String("error", markErr.Error()))
			}
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.log.Warn("task claim failed", slog.Int("worker_loop", loopID), slog.String("error", err.Error()))
			continue
		}

		result, err := w.processor.Process(ctx, claim)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.log.Warn("task processing failed", slog.Int("worker_loop", loopID), slog.String("task_id", claim.TaskID), slog.String("error", err.Error()))
			result = ProcessResult{Action: claimActionRetry, RetryDelay: w.options.RetryBackoff}
		}
		if err := w.applyResult(ctx, claim, result); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.log.Warn("task claim finalization failed", slog.Int("worker_loop", loopID), slog.String("task_id", claim.TaskID), slog.String("error", err.Error()))
		}
	}
}

func (w *Worker) applyResult(ctx context.Context, claim queue.TaskClaim, result ProcessResult) error {
	switch result.Action {
	case claimActionAck:
		return w.queue.Ack(ctx, claim)
	case claimActionRetry:
		delay := result.RetryDelay
		if delay <= 0 {
			delay = w.options.RetryBackoff
		}
		if err := w.queue.Retry(ctx, claim, delay); err != nil {
			_ = w.processor.MarkQueueFailure(ctx, claim.TaskID)
			return err
		}
		return nil
	default:
		return nil
	}
}

func (p *WorkerProcessor) Process(ctx context.Context, claim queue.TaskClaim) (ProcessResult, error) {
	if p == nil || p.db == nil {
		return ProcessResult{Action: claimActionRetry}, database.ErrNilDB
	}
	taskID := strings.TrimSpace(claim.TaskID)
	if taskID == "" {
		return ProcessResult{Action: claimActionAck}, queue.ErrInvalidClaim
	}
	snapshot, scope, err := p.loadExecutionContext(ctx, taskID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ProcessResult{Action: claimActionAck}, nil
		}
		return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, err
	}
	if terminalStatus(snapshot.Task.Status) || snapshot.Task.Status == StatusRunning {
		return ProcessResult{Action: claimActionAck}, nil
	}
	if snapshot.Task.Status != StatusQueued && snapshot.Task.Status != StatusRetrying {
		return ProcessResult{Action: claimActionAck}, nil
	}
	if err := validateExecutionContext(snapshot); err != nil {
		if failErr := p.failEligibleTask(ctx, scope, snapshot.Task.ID, "TASK_CONFIGURATION_INVALID", "Task configuration is no longer available."); failErr != nil {
			return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, failErr
		}
		return ProcessResult{Action: claimActionAck}, nil
	}

	lease, err := p.acquireConcurrency(ctx, scope, snapshot)
	if errors.Is(err, settings.ErrStoredTaskConcurrencyInvalid) {
		if failErr := p.failEligibleTask(ctx, scope, snapshot.Task.ID, "TASK_CONFIGURATION_INVALID", "Task configuration is no longer available."); failErr != nil {
			return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, failErr
		}
		return ProcessResult{Action: claimActionAck}, nil
	}
	if errors.Is(err, queue.ErrConcurrencyLimited) {
		return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, nil
	}
	if err != nil {
		return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, err
	}
	defer func() {
		if p.limiter != nil {
			_ = p.limiter.Release(context.Background(), lease)
		}
	}()

	running, claimed, err := p.claimRunning(ctx, scope, snapshot.Task.ID)
	if err != nil {
		return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, err
	}
	if !claimed {
		return ProcessResult{Action: claimActionAck}, nil
	}
	snapshot.Task = running
	if err := p.writeProgress(ctx, scope, running, ProgressUpdate{Percent: 50, Message: "Task execution started."}); err != nil {
		return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, err
	}

	execCtx := ctx
	var cancel context.CancelFunc
	if running.TimeoutAt != nil {
		execCtx, cancel = context.WithDeadline(ctx, running.TimeoutAt.UTC())
	} else {
		execCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	result := p.executor.Execute(execCtx, snapshot)
	if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
		if err := p.timeoutTask(ctx, scope, running.ID); err != nil {
			return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, err
		}
		return ProcessResult{Action: claimActionAck}, nil
	}
	if errors.Is(execCtx.Err(), context.Canceled) {
		return ProcessResult{Action: claimActionNone}, nil
	}
	for _, progress := range result.Progress {
		if err := p.writeProgress(ctx, scope, running, progress); err != nil {
			return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, err
		}
	}
	if hasAPICall(result.APICall) {
		if err := p.recordAPICall(ctx, scope, running, result.APICall); err != nil {
			return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, err
		}
	}

	if result.TimedOut {
		if err := p.timeoutTask(ctx, scope, running.ID); err != nil {
			return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, err
		}
		return ProcessResult{Action: claimActionAck}, nil
	}
	if result.ErrorCode != "" || result.ErrorMessage != "" {
		if result.Retryable && !p.options.DisableAutoRetry && running.Attempt < running.MaxAttempts {
			if err := p.scheduleRetry(ctx, scope, running.ID, result); err != nil {
				return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, err
			}
			return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, nil
		}
		if err := p.failRunningTask(ctx, scope, running.ID, result); err != nil {
			return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, err
		}
		return ProcessResult{Action: claimActionAck}, nil
	}

	if err := p.persistSuccessfulResult(ctx, scope, running.ID, snapshot.Model, result); err != nil {
		return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, err
	}
	if err := p.completeTask(ctx, scope, running.ID); err != nil {
		return ProcessResult{Action: claimActionRetry, RetryDelay: p.options.RetryBackoff}, err
	}
	return ProcessResult{Action: claimActionAck}, nil
}

func (p *WorkerProcessor) Recover(ctx context.Context, taskQueue queue.ReliableTaskQueue, batch int) error {
	now := p.now()
	if p.limiter != nil {
		if err := p.limiter.ReapStale(ctx, now); err != nil {
			return err
		}
	}
	if taskQueue != nil {
		if _, err := taskQueue.PromoteDue(ctx, now, batch); err != nil {
			return err
		}
		if _, err := taskQueue.RecoverStale(ctx, now, batch); err != nil {
			return err
		}
	}
	return p.RecoverTimedOut(ctx, batch)
}

func (p *WorkerProcessor) RecoverTimedOut(ctx context.Context, batch int) error {
	records, err := p.repo.ListRunningTimedOutTasks(ctx, p.now(), batch)
	if err != nil {
		return err
	}
	for _, record := range records {
		scope, err := tenant.NewScope(record.TenantID)
		if err != nil {
			continue
		}
		if err := p.timeoutTask(ctx, scope, record.ID); err != nil && !errors.Is(err, ErrInvalidTransition) {
			return err
		}
	}
	return nil
}

func (p *WorkerProcessor) MarkDeadLettered(ctx context.Context, taskID string) error {
	scope, err := p.scopeForTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	return p.failEligibleTask(ctx, scope, taskID, "QUEUE_DEAD_LETTERED", "Task exceeded queue delivery attempts.")
}

func (p *WorkerProcessor) MarkQueueFailure(ctx context.Context, taskID string) error {
	scope, err := p.scopeForTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	return p.failEligibleTask(ctx, scope, taskID, "QUEUE_RETRY_FAILED", "Task could not be requeued.")
}

func (p *WorkerProcessor) loadExecutionContext(ctx context.Context, taskID string) (ExecutionContext, tenant.Scope, error) {
	scope, err := p.scopeForTask(ctx, taskID)
	if err != nil {
		return ExecutionContext{}, tenant.Scope{}, err
	}
	record, err := p.repo.FindTask(ctx, scope, taskID)
	if err != nil {
		return ExecutionContext{}, tenant.Scope{}, err
	}
	if terminalStatus(record.Status) || record.Status == StatusRunning || (record.Status != StatusQueued && record.Status != StatusRetrying) {
		return ExecutionContext{Task: record}, scope, nil
	}
	tenantRecord, err := p.repo.FindTenant(ctx, scope.ID())
	if err != nil {
		return ExecutionContext{}, tenant.Scope{}, err
	}
	projectRecord, err := p.repo.FindProject(ctx, scope, record.ProjectID)
	if err != nil {
		return ExecutionContext{}, tenant.Scope{}, err
	}
	providerRecord, err := p.repo.FindProvider(ctx, scope, record.ProviderID)
	if err != nil {
		return ExecutionContext{}, tenant.Scope{}, err
	}
	modelRecord, err := p.repo.FindModel(ctx, scope, record.ModelID)
	if err != nil {
		return ExecutionContext{}, tenant.Scope{}, err
	}
	return ExecutionContext{
		Task:     record,
		Tenant:   tenantRecord,
		Project:  projectRecord,
		Provider: providerRecord,
		Model:    modelRecord,
	}, scope, nil
}

func (p *WorkerProcessor) scopeForTask(ctx context.Context, taskID string) (tenant.Scope, error) {
	tenantID, err := p.repo.ResolveTaskTenantID(ctx, taskID)
	if err != nil {
		return tenant.Scope{}, err
	}
	return tenant.NewScope(tenantID)
}

func (p *WorkerProcessor) acquireConcurrency(ctx context.Context, scope tenant.Scope, snapshot ExecutionContext) (queue.ConcurrencyLease, error) {
	if p.limiter == nil {
		return queue.ConcurrencyLease{}, nil
	}
	policy, err := settings.LoadTaskConcurrency(ctx, settings.NewRepository(p.db), scope, settings.TaskConcurrency{
		TenantLimit:   p.options.TenantConcurrency,
		UserLimit:     p.options.UserConcurrency,
		ProviderLimit: p.options.ProviderConcurrency,
		ModelLimit:    p.options.ModelConcurrency,
	})
	if err != nil {
		if !errors.Is(err, settings.ErrStoredTaskConcurrencyInvalid) {
			return queue.ConcurrencyLease{}, errTaskConcurrencyPolicyUnavailable
		}
		return queue.ConcurrencyLease{}, err
	}
	providerLimit := policy.ProviderLimit
	if snapshot.Provider.ConcurrencyLimit > 0 && snapshot.Provider.ConcurrencyLimit < providerLimit {
		providerLimit = snapshot.Provider.ConcurrencyLimit
	}
	dimensions := []queue.ConcurrencyDimension{
		{Name: "global", Value: "all", Limit: p.options.GlobalConcurrency},
		{Name: "tenant", Value: snapshot.Task.TenantID, Limit: policy.TenantLimit},
		{Name: "user", Value: snapshot.Task.CreatedBy, Limit: policy.UserLimit},
		{Name: "provider", Value: snapshot.Task.ProviderID, Limit: providerLimit},
		{Name: "model", Value: snapshot.Task.ModelID, Limit: policy.ModelLimit},
	}
	return p.limiter.Acquire(ctx, dimensions, p.options.ConcurrencyLeaseTTL, p.now())
}

func (p *WorkerProcessor) claimRunning(ctx context.Context, scope tenant.Scope, taskID string) (database.GenerationTask, bool, error) {
	var running database.GenerationTask
	var events []database.TaskEvent
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := p.repo.withDB(tx)
		now := p.now()
		startedAt := now
		var err error
		running, err = repo.UpdateTask(ctx, scope, taskID, []string{StatusQueued, StatusRetrying}, map[string]any{
			"status":        StatusRunning,
			"started_at":    &startedAt,
			"finished_at":   nil,
			"error_code":    "",
			"error_message": "",
			"updated_at":    now,
		})
		if errors.Is(err, ErrInvalidTransition) {
			return nil
		}
		if err != nil {
			return err
		}
		event, err := writeTaskEvent(ctx, repo, scope, running, EventTaskStarted, map[string]any{
			"startedAt": formatTime(startedAt),
		}, now)
		if err != nil {
			return err
		}
		events = append(events, event)
		return nil
	})
	if err != nil {
		return database.GenerationTask{}, false, err
	}
	if running.ID == "" {
		return database.GenerationTask{}, false, nil
	}
	p.publishEvents(ctx, events)
	return running, true, nil
}

func (p *WorkerProcessor) writeProgress(ctx context.Context, scope tenant.Scope, record database.GenerationTask, progress ProgressUpdate) error {
	current, err := p.repo.FindTask(ctx, scope, record.ID)
	if err != nil {
		return err
	}
	if current.Status != StatusRunning {
		return nil
	}
	record = current
	if progress.Percent < 0 {
		progress.Percent = 0
	}
	if progress.Percent > 100 {
		progress.Percent = 100
	}
	now := p.now()
	event, err := writeTaskEvent(ctx, p.repo, scope, record, EventTaskProgress, map[string]any{
		"progress": progress.Percent,
		"message":  cleanWorkerMessage(progress.Message),
	}, now)
	if err != nil {
		return err
	}
	p.publishEvents(ctx, []database.TaskEvent{event})
	return nil
}

func (p *WorkerProcessor) completeTask(ctx context.Context, scope tenant.Scope, taskID string) error {
	var events []database.TaskEvent
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := p.repo.withDB(tx)
		now := p.now()
		finishedAt := now
		updated, err := repo.UpdateTask(ctx, scope, taskID, []string{StatusRunning}, map[string]any{
			"status":        StatusSucceeded,
			"finished_at":   &finishedAt,
			"error_code":    "",
			"error_message": "",
			"updated_at":    now,
		})
		if errors.Is(err, ErrInvalidTransition) {
			return nil
		}
		if err != nil {
			return err
		}
		event, err := writeTaskEvent(ctx, repo, scope, updated, EventTaskCompleted, map[string]any{
			"finishedAt": formatTime(finishedAt),
		}, now)
		if err != nil {
			return err
		}
		events = append(events, event)
		return nil
	})
	if err != nil {
		return err
	}
	p.publishEvents(ctx, events)
	return nil
}

func (p *WorkerProcessor) failRunningTask(ctx context.Context, scope tenant.Scope, taskID string, result ExecutionResult) error {
	return p.failTask(ctx, scope, taskID, []string{StatusRunning}, resultCode(result, "EXECUTION_FAILED"), resultMessage(result, "Task execution failed."))
}

func (p *WorkerProcessor) failEligibleTask(ctx context.Context, scope tenant.Scope, taskID string, code string, message string) error {
	return p.failTask(ctx, scope, taskID, []string{StatusQueued, StatusRetrying, StatusRunning}, code, message)
}

func (p *WorkerProcessor) failTask(ctx context.Context, scope tenant.Scope, taskID string, allowed []string, code string, message string) error {
	var events []database.TaskEvent
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := p.repo.withDB(tx)
		now := p.now()
		finishedAt := now
		updated, err := repo.UpdateTask(ctx, scope, taskID, allowed, map[string]any{
			"status":        StatusFailed,
			"finished_at":   &finishedAt,
			"error_code":    cleanWorkerCode(code, "EXECUTION_FAILED"),
			"error_message": cleanWorkerMessage(message),
			"updated_at":    now,
		})
		if errors.Is(err, ErrInvalidTransition) {
			return nil
		}
		if err != nil {
			return err
		}
		event, err := writeTaskEvent(ctx, repo, scope, updated, EventTaskFailed, map[string]any{
			"errorCode": cleanWorkerCode(code, "EXECUTION_FAILED"),
			"message":   cleanWorkerMessage(message),
		}, now)
		if err != nil {
			return err
		}
		events = append(events, event)
		return nil
	})
	if err != nil {
		return err
	}
	p.publishEvents(ctx, events)
	return nil
}

func (p *WorkerProcessor) timeoutTask(ctx context.Context, scope tenant.Scope, taskID string) error {
	var events []database.TaskEvent
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := p.repo.withDB(tx)
		now := p.now()
		finishedAt := now
		updated, err := repo.UpdateTask(ctx, scope, taskID, []string{StatusRunning}, map[string]any{
			"status":        StatusTimedOut,
			"finished_at":   &finishedAt,
			"error_code":    "TASK_TIMED_OUT",
			"error_message": "Task execution timed out.",
			"updated_at":    now,
		})
		if errors.Is(err, ErrInvalidTransition) {
			return nil
		}
		if err != nil {
			return err
		}
		event, err := writeTaskEvent(ctx, repo, scope, updated, EventTaskTimedOut, map[string]any{
			"finishedAt": formatTime(finishedAt),
			"errorCode":  "TASK_TIMED_OUT",
			"message":    "Task execution timed out.",
		}, now)
		if err != nil {
			return err
		}
		events = append(events, event)
		return nil
	})
	if err != nil {
		return err
	}
	p.publishEvents(ctx, events)
	return nil
}

func (p *WorkerProcessor) scheduleRetry(ctx context.Context, scope tenant.Scope, taskID string, result ExecutionResult) error {
	var events []database.TaskEvent
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := p.repo.withDB(tx)
		current, err := repo.FindTask(ctx, scope, taskID)
		if err != nil {
			return err
		}
		if current.Status != StatusRunning || current.Attempt >= current.MaxAttempts {
			return ErrInvalidTransition
		}
		now := p.now()
		timeoutAt := now.Add(defaultTaskTimeout)
		retrying, err := repo.UpdateTask(ctx, scope, taskID, []string{StatusRunning}, map[string]any{
			"status":        StatusRetrying,
			"attempt":       current.Attempt + 1,
			"queued_at":     nil,
			"started_at":    nil,
			"finished_at":   nil,
			"timeout_at":    &timeoutAt,
			"error_code":    cleanWorkerCode(result.ErrorCode, "EXECUTION_RETRYABLE"),
			"error_message": cleanWorkerMessage(result.ErrorMessage),
			"updated_at":    now,
		})
		if err != nil {
			return err
		}
		event, err := writeTaskEvent(ctx, repo, scope, retrying, EventTaskRetried, map[string]any{
			"errorCode": cleanWorkerCode(result.ErrorCode, "EXECUTION_RETRYABLE"),
			"message":   cleanWorkerMessage(result.ErrorMessage),
		}, now)
		if err != nil {
			return err
		}
		events = append(events, event)

		queuedAt := now
		queued, err := repo.UpdateTask(ctx, scope, taskID, []string{StatusRetrying}, map[string]any{
			"status":        StatusQueued,
			"queued_at":     &queuedAt,
			"error_code":    "",
			"error_message": "",
			"updated_at":    now,
		})
		if err != nil {
			return err
		}
		event, err = writeTaskEvent(ctx, repo, scope, queued, EventTaskQueued, map[string]any{
			"queuedAt": formatTime(queuedAt),
		}, now)
		if err != nil {
			return err
		}
		events = append(events, event)
		return nil
	})
	if err != nil {
		return err
	}
	p.publishEvents(ctx, events)
	return nil
}

func (p *WorkerProcessor) publishEvents(ctx context.Context, events []database.TaskEvent) {
	if p.publisher == nil {
		return
	}
	for _, event := range events {
		p.publisher.PublishTaskEvent(ctx, event)
	}
}

func (StubExecutor) Execute(ctx context.Context, _ ExecutionContext) ExecutionResult {
	select {
	case <-ctx.Done():
		return ExecutionResult{TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded)}
	default:
		return ExecutionResult{}
	}
}

func validateExecutionContext(snapshot ExecutionContext) error {
	if snapshot.Tenant.Status != auth.TenantStatusActive {
		return fmt.Errorf("%w: inactive tenant", ErrValidation)
	}
	if snapshot.Project.Status != project.StatusActive {
		return fmt.Errorf("%w: inactive project", ErrValidation)
	}
	if snapshot.Provider.Status != providerpkg.StatusEnabled {
		return fmt.Errorf("%w: disabled provider", ErrValidation)
	}
	if snapshot.Model.ProviderID != snapshot.Provider.ID || snapshot.Model.Status != modelpkg.StatusEnabled {
		return fmt.Errorf("%w: disabled model", ErrValidation)
	}
	inputAssetIDs, err := taskInputAssetIDs(snapshot.Task)
	if err != nil {
		return fmt.Errorf("%w: invalid task inputs", ErrValidation)
	}
	if err := validateModelCapability(snapshot.Task.Type, snapshot.Model, inputAssetIDs); err != nil {
		return fmt.Errorf("%w: model capability mismatch", ErrValidation)
	}
	parameters, err := taskParameters(snapshot.Task)
	if err != nil {
		return fmt.Errorf("%w: invalid task parameters", ErrValidation)
	}
	if _, _, err := normalizeParameters(parameters, snapshot.Model); err != nil {
		return fmt.Errorf("%w: model parameter mismatch", ErrValidation)
	}
	return nil
}

func resultCode(result ExecutionResult, fallback string) string {
	return cleanWorkerCode(result.ErrorCode, fallback)
}

func resultMessage(result ExecutionResult, fallback string) string {
	message := cleanWorkerMessage(result.ErrorMessage)
	if message == "" {
		return fallback
	}
	return message
}

func cleanWorkerCode(value string, fallback string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		value = fallback
	}
	if len(value) > 128 {
		value = value[:128]
	}
	return value
}

func cleanWorkerMessage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{"authorization", "cookie", "api_key", "apikey", "secret", "base64", "bearer"} {
		if strings.Contains(lower, marker) {
			return "Task execution message redacted."
		}
	}
	if len(value) > 512 {
		value = value[:512]
	}
	return value
}
