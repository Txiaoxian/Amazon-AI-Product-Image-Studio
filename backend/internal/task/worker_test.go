package task

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	modelpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	providerpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provideradapter"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/queue"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestWorkerProcessorCompletesQueuedTaskWithProgressAndWakeups(t *testing.T) {
	db := newWorkerTestDB(t)
	seedWorkerBase(t, db)
	taskID := seedWorkerTask(t, db, workerTaskSeed{ID: "task-complete", Status: StatusQueued})
	publisher := &recordingPublisher{}

	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{
		EventPublisher: publisher,
		Executor: executorFunc(func(context.Context, ExecutionContext) ExecutionResult {
			return ExecutionResult{Progress: []ProgressUpdate{{Percent: 75, Message: "Halfway without Authorization Cookie base64 secret"}}}
		}),
	})

	result, err := processor.Process(context.Background(), queue.TaskClaim{TaskID: taskID, DeliveryCount: 1})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result.Action != claimActionAck {
		t.Fatalf("Process action = %v, want ack", result.Action)
	}

	record := loadWorkerTask(t, db, taskID)
	if record.Status != StatusSucceeded || record.StartedAt == nil || record.FinishedAt == nil {
		t.Fatalf("completed task record = %#v, want succeeded with timestamps", record)
	}
	assertWorkerEvents(t, db, taskID, []string{EventTaskStarted, EventTaskProgress, EventTaskProgress, EventTaskCompleted})
	if len(publisher.events) != 4 {
		t.Fatalf("published events = %d, want 4", len(publisher.events))
	}
	for _, event := range publisher.events {
		if event.Sequence == 0 {
			t.Fatalf("published event missing sequence: %#v", event)
		}
	}
	assertWorkerNoOutputsOrUsage(t, db)
	assertWorkerEventsSanitized(t, db)
}

func TestWorkerProcessorStatusMatrixAndDuplicateTerminalIdempotency(t *testing.T) {
	cases := []struct {
		name         string
		status       string
		wantStatus   string
		wantEvents   []string
		wantAction   claimAction
		executor     Executor
		disableRetry bool
		wantAttempt  int
	}{
		{
			name:       "queued succeeds",
			status:     StatusQueued,
			wantStatus: StatusSucceeded,
			wantEvents: []string{EventTaskStarted, EventTaskProgress, EventTaskCompleted},
			wantAction: claimActionAck,
		},
		{
			name:       "retrying succeeds",
			status:     StatusRetrying,
			wantStatus: StatusSucceeded,
			wantEvents: []string{EventTaskStarted, EventTaskProgress, EventTaskCompleted},
			wantAction: claimActionAck,
		},
		{
			name:       "running duplicate is acked",
			status:     StatusRunning,
			wantStatus: StatusRunning,
			wantEvents: nil,
			wantAction: claimActionAck,
		},
		{
			name:       "succeeded terminal is acked",
			status:     StatusSucceeded,
			wantStatus: StatusSucceeded,
			wantEvents: nil,
			wantAction: claimActionAck,
		},
		{
			name:       "failed terminal is acked",
			status:     StatusFailed,
			wantStatus: StatusFailed,
			wantEvents: nil,
			wantAction: claimActionAck,
		},
		{
			name:       "cancelled terminal is acked",
			status:     StatusCancelled,
			wantStatus: StatusCancelled,
			wantEvents: nil,
			wantAction: claimActionAck,
		},
		{
			name:       "timed out terminal is acked",
			status:     StatusTimedOut,
			wantStatus: StatusTimedOut,
			wantEvents: nil,
			wantAction: claimActionAck,
		},
		{
			name:       "failed execution becomes failed",
			status:     StatusQueued,
			wantStatus: StatusFailed,
			wantEvents: []string{EventTaskStarted, EventTaskProgress, EventTaskFailed},
			wantAction: claimActionAck,
			executor: executorFunc(func(context.Context, ExecutionContext) ExecutionResult {
				return ExecutionResult{ErrorCode: "fake_failed", ErrorMessage: "Fake failure with Authorization Cookie base64 secret"}
			}),
			disableRetry: true,
		},
		{
			name:       "retryable execution becomes queued retry",
			status:     StatusQueued,
			wantStatus: StatusQueued,
			wantEvents: []string{EventTaskStarted, EventTaskProgress, EventTaskRetried, EventTaskQueued},
			wantAction: claimActionRetry,
			executor: executorFunc(func(context.Context, ExecutionContext) ExecutionResult {
				return ExecutionResult{ErrorCode: "temporary", ErrorMessage: "Temporary failure", Retryable: true}
			}),
			wantAttempt: 2,
		},
		{
			name:       "timeout execution becomes timed out",
			status:     StatusQueued,
			wantStatus: StatusTimedOut,
			wantEvents: []string{EventTaskStarted, EventTaskProgress, EventTaskTimedOut},
			wantAction: claimActionAck,
			executor: executorFunc(func(context.Context, ExecutionContext) ExecutionResult {
				return ExecutionResult{TimedOut: true}
			}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newWorkerTestDB(t)
			seedWorkerBase(t, db)
			taskID := seedWorkerTask(t, db, workerTaskSeed{ID: "task-" + strings.ReplaceAll(tc.name, " ", "-"), Status: tc.status})
			options := WorkerProcessorOptions{
				Executor:         tc.executor,
				DisableAutoRetry: tc.disableRetry,
			}
			if options.Executor == nil {
				options.Executor = StubExecutor{}
			}
			processor := newWorkerTestProcessor(db, options)

			result, err := processor.Process(context.Background(), queue.TaskClaim{TaskID: taskID, DeliveryCount: 1})
			if err != nil {
				t.Fatalf("Process returned error: %v", err)
			}
			if result.Action != tc.wantAction {
				t.Fatalf("Process action = %v, want %v", result.Action, tc.wantAction)
			}
			record := loadWorkerTask(t, db, taskID)
			if record.Status != tc.wantStatus {
				t.Fatalf("task status = %q, want %q", record.Status, tc.wantStatus)
			}
			if tc.wantAttempt != 0 && record.Attempt != tc.wantAttempt {
				t.Fatalf("task attempt = %d, want %d", record.Attempt, tc.wantAttempt)
			}
			assertWorkerEvents(t, db, taskID, tc.wantEvents)
			assertWorkerEventsSanitized(t, db)
		})
	}
}

func TestWorkerProcessorDuplicateClaimAfterCompletionDoesNotDuplicateTerminalEvent(t *testing.T) {
	db := newWorkerTestDB(t)
	seedWorkerBase(t, db)
	taskID := seedWorkerTask(t, db, workerTaskSeed{ID: "task-duplicate", Status: StatusQueued})
	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{})

	for i := 0; i < 2; i++ {
		result, err := processor.Process(context.Background(), queue.TaskClaim{TaskID: taskID, DeliveryCount: int64(i + 1)})
		if err != nil {
			t.Fatalf("Process %d returned error: %v", i, err)
		}
		if result.Action != claimActionAck {
			t.Fatalf("Process %d action = %v, want ack", i, result.Action)
		}
	}

	assertWorkerEvents(t, db, taskID, []string{EventTaskStarted, EventTaskProgress, EventTaskCompleted})
}

func TestWorkerProcessorPersistsProviderOutputsUsageAndAPICallIdempotently(t *testing.T) {
	db := newWorkerTestDB(t)
	seedWorkerBase(t, db)
	taskID := seedWorkerTask(t, db, workerTaskSeed{ID: "task-provider-success", Status: StatusQueued})
	store := newMemoryObjectStore()
	pngBytes := workerTinyPNG(t)
	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{
		Store:         store,
		StorageConfig: config.StorageConfig{BucketGenerated: "generated-assets", BucketOriginals: "original-assets"},
		Executor: executorFunc(func(context.Context, ExecutionContext) ExecutionResult {
			httpStatus := 200
			return ExecutionResult{
				Outputs: []GeneratedImageOutput{{
					Data:     pngBytes,
					MIMEType: "image/png",
					Metadata: map[string]any{"providerOutputIndex": 0, "b64_json": "must-redact"},
				}},
				Usage: UsageResult{
					InputTokens:  11,
					OutputTokens: 4,
					ImageCount:   1,
					Raw:          map[string]any{"input_tokens": 11, "b64_json": "must-redact"},
				},
				APICall: APICallResult{
					Status:     provideradapter.APICallStatusSuccess,
					DurationMs: 123,
					RequestID:  "req-success",
					HTTPStatus: &httpStatus,
					RequestMetadata: map[string]any{
						"operation":     "generate",
						"Authorization": "Bearer sk-secret",
					},
					ResponseMetadata: map[string]any{
						"outputCount": 1,
						"b64_json":    "must-redact",
					},
				},
			}
		}),
	})

	for i := 0; i < 2; i++ {
		result, err := processor.Process(context.Background(), queue.TaskClaim{TaskID: taskID, DeliveryCount: int64(i + 1)})
		if err != nil {
			t.Fatalf("Process %d returned error: %v", i, err)
		}
		if result.Action != claimActionAck {
			t.Fatalf("Process %d action = %v, want ack", i, result.Action)
		}
	}

	record := loadWorkerTask(t, db, taskID)
	if record.Status != StatusSucceeded {
		t.Fatalf("task status = %q, want SUCCEEDED", record.Status)
	}
	assertWorkerEvents(t, db, taskID, []string{EventTaskStarted, EventTaskProgress, EventImageOutput, EventUsageRecorded, EventTaskCompleted})
	assertTableCount(t, db, &database.ImageAsset{}, 1)
	assertTableCount(t, db, &database.TaskOutput{}, 1)
	assertTableCount(t, db, &database.UsageRecord{}, 1)
	assertTableCount(t, db, &database.APICallLog{}, 1)
	if len(store.objects["generated-assets"]) != 1 {
		t.Fatalf("generated objects = %#v, want one object in generated bucket", store.objects)
	}
	assertWorkerRuntimeMetadataSanitized(t, db)
}

func TestWorkerProcessorFailureRecordsSanitizedAPICallWithoutAssets(t *testing.T) {
	db := newWorkerTestDB(t)
	seedWorkerBase(t, db)
	taskID := seedWorkerTask(t, db, workerTaskSeed{ID: "task-provider-failure", Status: StatusQueued})
	httpStatus := 500
	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{
		Store: newMemoryObjectStore(),
		Executor: executorFunc(func(context.Context, ExecutionContext) ExecutionResult {
			return ExecutionResult{
				APICall: APICallResult{
					Status:       provideradapter.APICallStatusFailure,
					DurationMs:   20,
					HTTPStatus:   &httpStatus,
					ErrorCode:    "provider_http_error",
					ErrorMessage: "Authorization Bearer sk-secret base64 AAAA",
					RequestMetadata: map[string]any{
						"cookie": "session=secret",
					},
				},
				ErrorCode:    "provider_http_error",
				ErrorMessage: "Authorization Bearer sk-secret base64 AAAA",
			}
		}),
	})

	result, err := processor.Process(context.Background(), queue.TaskClaim{TaskID: taskID, DeliveryCount: 1})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result.Action != claimActionAck {
		t.Fatalf("Process action = %v, want ack", result.Action)
	}
	record := loadWorkerTask(t, db, taskID)
	if record.Status != StatusFailed || record.ErrorMessage != "Task execution message redacted." {
		t.Fatalf("failed task = %#v, want sanitized failure", record)
	}
	assertWorkerEvents(t, db, taskID, []string{EventTaskStarted, EventTaskProgress, EventTaskFailed})
	assertTableCount(t, db, &database.ImageAsset{}, 0)
	assertTableCount(t, db, &database.TaskOutput{}, 0)
	assertTableCount(t, db, &database.UsageRecord{}, 0)
	assertTableCount(t, db, &database.APICallLog{}, 1)
	assertWorkerRuntimeMetadataSanitized(t, db)
}

func TestProviderRuntimeExecutorRedactsCurrentAPIKeyBeforeWorkerPersistsAPICall(t *testing.T) {
	db := newWorkerTestDB(t)
	seedWorkerBase(t, db)
	taskID := seedWorkerTask(t, db, workerTaskSeed{ID: "task-runtime-redacts-current-key", Status: StatusQueued})
	apiKey := "relay_live_1234567890abcdef"
	httpStatus := 502
	runtimeExecutor, err := NewProviderRuntimeExecutor(db, nil, ProviderRuntimeExecutorOptions{
		Runtime: fakeProviderRuntime(func(_ context.Context, req provideradapter.ImageRequest) (provideradapter.ImageResult, error) {
			if req.Provider.APIKey != apiKey {
				t.Fatalf("runtime API key = %q, want decrypted key", req.Provider.APIKey)
			}
			return provideradapter.ImageResult{
					APICall: provideradapter.APICall{
						Status:       provideradapter.APICallStatusFailure,
						DurationMs:   17,
						HTTPStatus:   &httpStatus,
						ErrorCode:    "provider_http_error",
						ErrorMessage: "provider echoed " + apiKey,
						RequestMetadata: map[string]any{
							"message":       "request had " + apiKey,
							"Authorization": "Bearer " + apiKey,
						},
						ResponseMetadata: map[string]any{
							"nested": map[string]any{
								"message": "response had " + apiKey,
								"Cookie":  "session=" + apiKey,
							},
						},
					},
				}, provideradapter.ProviderError{
					Code:       "PROVIDER_HTTP_ERROR",
					Message:    "provider error included " + apiKey,
					HTTPStatus: &httpStatus,
				}
		}),
		Decrypter:    staticAPIKeyDecrypter(apiKey),
		URLValidator: providerpkg.NewURLValidator(staticIPResolver{ip: net.ParseIP("8.8.8.8")}),
	})
	if err != nil {
		t.Fatalf("create runtime executor: %v", err)
	}
	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{Executor: runtimeExecutor})

	result, err := processor.Process(context.Background(), queue.TaskClaim{TaskID: taskID, DeliveryCount: 1})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result.Action != claimActionAck {
		t.Fatalf("Process action = %v, want ack", result.Action)
	}
	record := loadWorkerTask(t, db, taskID)
	if record.Status != StatusFailed {
		t.Fatalf("task status = %q, want FAILED", record.Status)
	}
	assertTableCount(t, db, &database.APICallLog{}, 1)
	assertNoWorkerPersistedText(t, db, apiKey)
	for _, forbidden := range []string{"authorization", "cookie"} {
		assertNoWorkerPersistedText(t, db, forbidden)
	}
}

func TestExecutionResultFromProviderRedactsCurrentAPIKeyInOutputsAndUsage(t *testing.T) {
	apiKey := "relay_live_abcdef1234567890"
	httpStatus := 200

	result := executionResultFromProvider(provideradapter.ImageResult{
		Images: []provideradapter.Image{{
			Data:     workerTinyPNG(t),
			MIMEType: "image/png",
			Metadata: map[string]any{
				"message": "output included " + apiKey,
			},
		}},
		Usage: provideradapter.Usage{
			InputTokens: 1,
			ImageCount:  1,
			Raw: map[string]any{
				"providerNote": "usage included " + apiKey,
			},
		},
		APICall: provideradapter.APICall{
			Status:       provideradapter.APICallStatusSuccess,
			HTTPStatus:   &httpStatus,
			ErrorMessage: "call included " + apiKey,
			RequestMetadata: map[string]any{
				"message": "request included " + apiKey,
			},
			ResponseMetadata: map[string]any{
				"message": "response included " + apiKey,
			},
		},
	}, provideradapter.NewRedactor(apiKey))

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal execution result: %v", err)
	}
	text := string(encoded)
	if strings.Contains(text, apiKey) {
		t.Fatalf("execution result leaked API key: %s", text)
	}
	if !strings.Contains(text, "[REDACTED]") {
		t.Fatalf("execution result did not contain redacted marker: %s", text)
	}
}

func TestWorkerProcessorDropsProviderOutputsWhenTaskCancelledAfterUpload(t *testing.T) {
	db := newWorkerTestDB(t)
	seedWorkerBase(t, db)
	taskID := seedWorkerTask(t, db, workerTaskSeed{ID: "task-cancel-after-upload", Status: StatusQueued})
	store := newMemoryObjectStore()
	store.onPut = func() {
		now := time.Now().UTC()
		finishedAt := now
		if err := db.Model(&database.GenerationTask{}).
			Where("tenant_id = ? AND id = ?", "tenant-worker", taskID).
			Updates(map[string]any{
				"status":      StatusCancelled,
				"finished_at": &finishedAt,
				"updated_at":  now,
			}).Error; err != nil {
			t.Fatalf("cancel task after upload: %v", err)
		}
	}
	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{
		Store:         store,
		StorageConfig: config.StorageConfig{BucketGenerated: "generated-assets", BucketOriginals: "original-assets"},
		Executor: executorFunc(func(context.Context, ExecutionContext) ExecutionResult {
			return ExecutionResult{Outputs: []GeneratedImageOutput{{Data: workerTinyPNG(t), MIMEType: "image/png"}}}
		}),
	})

	result, err := processor.Process(context.Background(), queue.TaskClaim{TaskID: taskID, DeliveryCount: 1})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result.Action != claimActionAck {
		t.Fatalf("Process action = %v, want ack", result.Action)
	}
	assertTableCount(t, db, &database.ImageAsset{}, 0)
	assertTableCount(t, db, &database.TaskOutput{}, 0)
	if len(store.objects["generated-assets"]) != 0 {
		t.Fatalf("unpersisted generated object was not cleaned up: %#v", store.objects)
	}
}

func TestWorkerProcessorHonorsCancellationBeforeCompletion(t *testing.T) {
	db := newWorkerTestDB(t)
	seedWorkerBase(t, db)
	taskID := seedWorkerTask(t, db, workerTaskSeed{ID: "task-cancel-during-worker", Status: StatusQueued})
	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{
		Executor: executorFunc(func(ctx context.Context, execution ExecutionContext) ExecutionResult {
			scope, err := tenant.NewScope(execution.Task.TenantID)
			if err != nil {
				t.Fatalf("tenant scope: %v", err)
			}
			repo := NewRepository(db)
			now := time.Now().UTC()
			finishedAt := now
			cancelled, err := repo.UpdateTask(ctx, scope, execution.Task.ID, []string{StatusRunning}, map[string]any{
				"status":      StatusCancelled,
				"finished_at": &finishedAt,
				"updated_at":  now,
			})
			if err != nil {
				t.Fatalf("cancel running task: %v", err)
			}
			if _, err := writeTaskEvent(ctx, repo, scope, cancelled, EventTaskCancelled, map[string]any{"finishedAt": formatTime(finishedAt)}, now); err != nil {
				t.Fatalf("write cancel event: %v", err)
			}
			return ExecutionResult{}
		}),
	})

	result, err := processor.Process(context.Background(), queue.TaskClaim{TaskID: taskID, DeliveryCount: 1})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result.Action != claimActionAck {
		t.Fatalf("Process action = %v, want ack", result.Action)
	}
	record := loadWorkerTask(t, db, taskID)
	if record.Status != StatusCancelled {
		t.Fatalf("task status = %q, want CANCELLED", record.Status)
	}
	assertWorkerEvents(t, db, taskID, []string{EventTaskStarted, EventTaskProgress, EventTaskCancelled})
}

func TestWorkerProcessorContextCanceledDoesNotCompleteTask(t *testing.T) {
	db := newWorkerTestDB(t)
	seedWorkerBase(t, db)
	taskID := seedWorkerTask(t, db, workerTaskSeed{ID: "task-context-canceled", Status: StatusQueued})
	var cancel context.CancelFunc
	ctx, cancel := context.WithCancel(context.Background())
	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{
		Executor: executorFunc(func(context.Context, ExecutionContext) ExecutionResult {
			cancel()
			return ExecutionResult{}
		}),
	})

	result, err := processor.Process(ctx, queue.TaskClaim{TaskID: taskID, DeliveryCount: 1})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result.Action != claimActionNone {
		t.Fatalf("Process action = %v, want none", result.Action)
	}
	record := loadWorkerTask(t, db, taskID)
	if record.Status == StatusSucceeded {
		t.Fatalf("task status = %q, must not succeed after context cancellation", record.Status)
	}
	if record.Status != StatusRunning {
		t.Fatalf("task status = %q, want RUNNING for recovery/timeout", record.Status)
	}
	assertWorkerEvents(t, db, taskID, []string{EventTaskStarted, EventTaskProgress})
}

func TestWorkerProcessorDeadlineExceededStillTimesOutTask(t *testing.T) {
	db := newWorkerTestDB(t)
	seedWorkerBase(t, db)
	taskID := seedWorkerTask(t, db, workerTaskSeed{
		ID:        "task-deadline-exceeded",
		Status:    StatusQueued,
		TimeoutAt: time.Now().UTC().Add(-time.Second),
	})
	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{
		Executor: executorFunc(func(ctx context.Context, _ ExecutionContext) ExecutionResult {
			<-ctx.Done()
			return ExecutionResult{}
		}),
	})

	result, err := processor.Process(context.Background(), queue.TaskClaim{TaskID: taskID, DeliveryCount: 1})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result.Action != claimActionAck {
		t.Fatalf("Process action = %v, want ack", result.Action)
	}
	record := loadWorkerTask(t, db, taskID)
	if record.Status != StatusTimedOut {
		t.Fatalf("task status = %q, want TIMED_OUT", record.Status)
	}
	assertWorkerEvents(t, db, taskID, []string{EventTaskStarted, EventTaskProgress, EventTaskTimedOut})
}

func TestWorkerProcessorConcurrencyLimitsAllDimensions(t *testing.T) {
	db := newWorkerTestDB(t)
	seedWorkerBase(t, db)
	taskID := seedWorkerTask(t, db, workerTaskSeed{ID: "task-concurrency", Status: StatusQueued, ProviderLimit: 1})
	limiter := &recordingLimiter{blocked: true}
	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{
		Limiter:             limiter,
		GlobalConcurrency:   9,
		TenantConcurrency:   8,
		UserConcurrency:     7,
		ProviderConcurrency: 6,
		ModelConcurrency:    5,
		ConcurrencyLeaseTTL: time.Minute,
	})

	result, err := processor.Process(context.Background(), queue.TaskClaim{TaskID: taskID, DeliveryCount: 1})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result.Action != claimActionRetry {
		t.Fatalf("Process action = %v, want retry", result.Action)
	}
	record := loadWorkerTask(t, db, taskID)
	if record.Status != StatusQueued {
		t.Fatalf("task status = %q, want QUEUED when concurrency blocked", record.Status)
	}
	wantDimensions := []queue.ConcurrencyDimension{
		{Name: "global", Value: "all", Limit: 9},
		{Name: "tenant", Value: "tenant-worker", Limit: 8},
		{Name: "user", Value: "user-worker", Limit: 7},
		{Name: "provider", Value: "provider-worker", Limit: 1},
		{Name: "model", Value: "model-worker", Limit: 5},
	}
	if !reflect.DeepEqual(limiter.lastDimensions, wantDimensions) {
		t.Fatalf("concurrency dimensions = %#v, want %#v", limiter.lastDimensions, wantDimensions)
	}
	assertWorkerEvents(t, db, taskID, nil)
}

func TestWorkerRecoveryTimesOutRunningTaskAndReapsStaleLocks(t *testing.T) {
	db := newWorkerTestDB(t)
	seedWorkerBase(t, db)
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	taskID := seedWorkerTask(t, db, workerTaskSeed{
		ID:        "task-recovery-timeout",
		Status:    StatusRunning,
		TimeoutAt: now.Add(-time.Second),
	})
	limiter := &recordingLimiter{}
	taskQueue := &recordingReliableQueue{}
	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{
		Limiter: limiter,
		Now: func() time.Time {
			return now
		},
	})

	if err := processor.Recover(context.Background(), taskQueue, 10); err != nil {
		t.Fatalf("Recover returned error: %v", err)
	}
	if !limiter.reaped {
		t.Fatal("Recover did not reap stale concurrency locks")
	}
	if !taskQueue.promoted || !taskQueue.recovered {
		t.Fatalf("Recover did not run queue delayed/stale recovery: %#v", taskQueue)
	}
	record := loadWorkerTask(t, db, taskID)
	if record.Status != StatusTimedOut {
		t.Fatalf("task status = %q, want TIMED_OUT", record.Status)
	}
	assertWorkerEvents(t, db, taskID, []string{EventTaskTimedOut})
}

func TestWorkerDeadLetterMarksTaskFailed(t *testing.T) {
	db := newWorkerTestDB(t)
	seedWorkerBase(t, db)
	taskID := seedWorkerTask(t, db, workerTaskSeed{ID: "task-dead-letter", Status: StatusQueued})
	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{})

	if err := processor.MarkDeadLettered(context.Background(), taskID); err != nil {
		t.Fatalf("MarkDeadLettered returned error: %v", err)
	}
	record := loadWorkerTask(t, db, taskID)
	if record.Status != StatusFailed || record.ErrorCode != "QUEUE_DEAD_LETTERED" {
		t.Fatalf("dead-lettered task = %#v, want failed dead letter", record)
	}
	assertWorkerEvents(t, db, taskID, []string{EventTaskFailed})
}

type executorFunc func(context.Context, ExecutionContext) ExecutionResult

func (f executorFunc) Execute(ctx context.Context, execution ExecutionContext) ExecutionResult {
	return f(ctx, execution)
}

type recordingPublisher struct {
	events []database.TaskEvent
}

func (p *recordingPublisher) PublishTaskEvent(_ context.Context, event database.TaskEvent) {
	p.events = append(p.events, event)
}

type recordingLimiter struct {
	blocked        bool
	lastDimensions []queue.ConcurrencyDimension
	released       bool
	reaped         bool
}

func (l *recordingLimiter) Acquire(_ context.Context, dimensions []queue.ConcurrencyDimension, _ time.Duration, now time.Time) (queue.ConcurrencyLease, error) {
	l.lastDimensions = append([]queue.ConcurrencyDimension(nil), dimensions...)
	if l.blocked {
		return queue.ConcurrencyLease{}, queue.ErrConcurrencyLimited
	}
	return queue.ConcurrencyLease{ID: "lease-test", ExpiresAt: now.Add(time.Minute), Keys: []string{"global", "tenant", "user", "provider", "model"}}, nil
}

func (l *recordingLimiter) Release(context.Context, queue.ConcurrencyLease) error {
	l.released = true
	return nil
}

func (l *recordingLimiter) ReapStale(context.Context, time.Time) error {
	l.reaped = true
	return nil
}

type recordingReliableQueue struct {
	promoted  bool
	recovered bool
}

func (q *recordingReliableQueue) EnqueueTask(context.Context, string) error {
	return nil
}

func (q *recordingReliableQueue) Claim(context.Context) (queue.TaskClaim, error) {
	return queue.TaskClaim{}, queue.ErrNoTask
}

func (q *recordingReliableQueue) Ack(context.Context, queue.TaskClaim) error {
	return nil
}

func (q *recordingReliableQueue) Retry(context.Context, queue.TaskClaim, time.Duration) error {
	return nil
}

func (q *recordingReliableQueue) DeadLetter(context.Context, queue.TaskClaim, string) error {
	return nil
}

func (q *recordingReliableQueue) RecoverStale(context.Context, time.Time, int) ([]string, error) {
	q.recovered = true
	return nil, nil
}

func (q *recordingReliableQueue) PromoteDue(context.Context, time.Time, int) ([]string, error) {
	q.promoted = true
	return nil, nil
}

type fakeProviderRuntime func(context.Context, provideradapter.ImageRequest) (provideradapter.ImageResult, error)

func (f fakeProviderRuntime) Execute(ctx context.Context, request provideradapter.ImageRequest) (provideradapter.ImageResult, error) {
	return f(ctx, request)
}

type staticAPIKeyDecrypter string

func (d staticAPIKeyDecrypter) Decrypt(string) (string, error) {
	return string(d), nil
}

type staticIPResolver struct {
	ip net.IP
}

func (r staticIPResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	if r.ip == nil {
		return nil, errors.New("missing static test IP")
	}
	return []net.IPAddr{{IP: r.ip}}, nil
}

type memoryObjectStore struct {
	objects map[string]map[string][]byte
	onPut   func()
}

func newMemoryObjectStore() *memoryObjectStore {
	return &memoryObjectStore{objects: map[string]map[string][]byte{}}
}

func (s *memoryObjectStore) PutObject(_ context.Context, bucket string, key string, body io.Reader, _ int64, _ string) error {
	if s.objects[bucket] == nil {
		s.objects[bucket] = map[string][]byte{}
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.objects[bucket][key] = data
	if s.onPut != nil {
		s.onPut()
	}
	return nil
}

func (s *memoryObjectStore) GetObject(_ context.Context, bucket string, key string) (storage.Object, error) {
	data, ok := s.objects[bucket][key]
	if !ok {
		return storage.Object{}, storage.ErrNotFound
	}
	return storage.Object{Body: io.NopCloser(bytes.NewReader(data)), Size: int64(len(data)), ContentType: "image/png"}, nil
}

func (s *memoryObjectStore) RemoveObject(_ context.Context, bucket string, key string) error {
	if s.objects[bucket] != nil {
		delete(s.objects[bucket], key)
	}
	return nil
}

type workerTaskSeed struct {
	ID            string
	Status        string
	ProviderLimit int
	TimeoutAt     time.Time
}

func newWorkerTestProcessor(db *gorm.DB, options WorkerProcessorOptions) *WorkerProcessor {
	if options.Now == nil {
		options.Now = func() time.Time {
			return time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC)
		}
	}
	return NewWorkerProcessor(db, nil, options)
}

func newWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsnName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dsn := "file:" + dsnName + "_" + time.Now().UTC().Format("20060102150405.000000000") + "?mode=memory&cache=shared&_loc=auto&parseTime=true"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access sqlite test database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	schema := []string{
		`CREATE TABLE tenants (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			email TEXT NOT NULL,
			display_name TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			status TEXT NOT NULL,
			last_login_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE projects (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			name TEXT NOT NULL,
			brand TEXT NOT NULL DEFAULT '',
			asin TEXT NOT NULL DEFAULT '',
			site TEXT NOT NULL DEFAULT '',
			notes TEXT NULL,
			status TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			deleted_at TIMESTAMP NULL
		)`,
		`CREATE TABLE image_assets (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			category TEXT NOT NULL,
			filename TEXT NOT NULL,
			object_key TEXT NOT NULL,
			thumbnail_object_key TEXT NULL,
			mime_type TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			width INTEGER NOT NULL,
			height INTEGER NOT NULL,
			sha256 TEXT NOT NULL,
			is_favorite BOOLEAN NOT NULL,
			source_task_id TEXT NULL,
			created_by TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			deleted_at TIMESTAMP NULL
		)`,
		`CREATE TABLE ai_providers (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			base_url TEXT NOT NULL,
			encrypted_api_key TEXT NOT NULL,
			api_key_hint TEXT NOT NULL,
			api_key_updated_at TIMESTAMP NULL,
			status TEXT NOT NULL,
			timeout_seconds INTEGER NOT NULL,
			concurrency_limit INTEGER NOT NULL,
			last_test_status TEXT NOT NULL DEFAULT '',
			last_tested_at TIMESTAMP NULL,
			last_test_error TEXT NOT NULL DEFAULT '',
			created_by TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			deleted_at TIMESTAMP NULL
		)`,
		`CREATE TABLE ai_models (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			model_name TEXT NOT NULL,
			display_name TEXT NOT NULL,
			supports_generate BOOLEAN NOT NULL,
			supports_edit BOOLEAN NOT NULL,
			supports_multi_reference BOOLEAN NOT NULL,
			supports_n BOOLEAN NOT NULL,
			max_output_count INTEGER NOT NULL,
			supported_sizes_json TEXT NOT NULL,
			supported_qualities_json TEXT NOT NULL,
			supported_output_formats_json TEXT NOT NULL,
			pricing_json TEXT NOT NULL,
			status TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			deleted_at TIMESTAMP NULL
		)`,
		`CREATE TABLE generation_tasks (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		project_id TEXT NOT NULL,
		type TEXT NOT NULL,
		provider_id TEXT NOT NULL,
		model_id TEXT NOT NULL,
		status TEXT NOT NULL,
		prompt TEXT NOT NULL,
		image_type TEXT NOT NULL,
		params_json TEXT NOT NULL,
		input_asset_ids_json TEXT NOT NULL,
		attempt INTEGER NOT NULL,
		max_attempts INTEGER NOT NULL,
		queued_at TIMESTAMP NULL,
		started_at TIMESTAMP NULL,
		finished_at TIMESTAMP NULL,
		timeout_at TIMESTAMP NULL,
		created_by TEXT NOT NULL,
		error_code TEXT NOT NULL,
		error_message TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`,
		`CREATE TABLE task_events (
		sequence INTEGER PRIMARY KEY AUTOINCREMENT,
		id TEXT NOT NULL UNIQUE,
		tenant_id TEXT NOT NULL,
		task_id TEXT NOT NULL,
		project_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		event_payload_json TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL
	)`,
		`CREATE TABLE task_outputs (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		task_id TEXT NOT NULL,
		asset_id TEXT NOT NULL,
		output_index INTEGER NOT NULL,
		created_at TIMESTAMP NOT NULL
	)`,
		`CREATE TABLE api_call_logs (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		task_id TEXT NOT NULL,
		provider_id TEXT NOT NULL,
		model_id TEXT NOT NULL,
		status TEXT NOT NULL,
		duration_ms INTEGER NOT NULL,
		request_id TEXT NOT NULL,
		http_status INTEGER NULL,
		error_code TEXT NOT NULL,
		error_message TEXT NOT NULL,
		redacted_request_json TEXT NULL,
		redacted_response_json TEXT NULL,
		created_at TIMESTAMP NOT NULL
	)`,
		`CREATE TABLE usage_records (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		task_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		project_id TEXT NOT NULL,
		provider_id TEXT NOT NULL,
		model_id TEXT NOT NULL,
		input_tokens INTEGER NOT NULL,
		output_tokens INTEGER NOT NULL,
		image_count INTEGER NOT NULL,
		estimated_cost TEXT NOT NULL,
		currency TEXT NOT NULL,
		raw_usage_json TEXT NULL,
		created_at TIMESTAMP NOT NULL
	)`,
	}
	for _, statement := range schema {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("migrate worker test schema: %v", err)
		}
	}
	return db
}

func seedWorkerBase(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&database.Tenant{ID: "tenant-worker", Name: "Tenant Worker", Status: auth.TenantStatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := db.Create(&database.User{ID: "user-worker", TenantID: "tenant-worker", Email: "worker@example.com", DisplayName: "Worker User", PasswordHash: "hash", Status: auth.UserStatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.Create(&database.Project{ID: "project-worker", TenantID: "tenant-worker", Name: "Worker Project", Status: project.StatusActive, CreatedBy: "user-worker", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := db.Create(&database.AIProvider{
		ID:               "provider-worker",
		TenantID:         "tenant-worker",
		Type:             providerpkg.TypeOpenAICompatible,
		Name:             "Worker Provider",
		BaseURL:          "https://api.openai.com/v1",
		EncryptedAPIKey:  "encrypted",
		APIKeyHint:       "****test",
		Status:           providerpkg.StatusEnabled,
		TimeoutSeconds:   10,
		ConcurrencyLimit: 3,
		CreatedBy:        "user-worker",
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	if err := db.Create(&database.AIModel{
		ID:                         "model-worker",
		TenantID:                   "tenant-worker",
		ProviderID:                 "provider-worker",
		ModelName:                  "worker-model",
		DisplayName:                "Worker Model",
		SupportsGenerate:           true,
		MaxOutputCount:             1,
		SupportedSizesJSON:         `["1024x1024"]`,
		SupportedQualitiesJSON:     `["high"]`,
		SupportedOutputFormatsJSON: `["png"]`,
		PricingJSON:                `{"currency":"USD","unitPrices":{}}`,
		Status:                     modelpkg.StatusEnabled,
		CreatedBy:                  "user-worker",
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}).Error; err != nil {
		t.Fatalf("seed model: %v", err)
	}
}

func seedWorkerTask(t *testing.T, db *gorm.DB, seed workerTaskSeed) string {
	t.Helper()
	now := time.Now().UTC()
	taskID := seed.ID
	if taskID == "" {
		taskID = "task-worker"
	}
	status := seed.Status
	if status == "" {
		status = StatusQueued
	}
	timeoutAt := seed.TimeoutAt
	if timeoutAt.IsZero() {
		timeoutAt = now.Add(30 * time.Minute)
	}
	if seed.ProviderLimit > 0 {
		if err := db.Model(&database.AIProvider{}).Where("tenant_id = ? AND id = ?", "tenant-worker", "provider-worker").Update("concurrency_limit", seed.ProviderLimit).Error; err != nil {
			t.Fatalf("update provider limit: %v", err)
		}
	}
	var startedAt *time.Time
	if status == StatusRunning {
		started := now.Add(-time.Minute)
		startedAt = &started
	}
	record := database.GenerationTask{
		ID:                taskID,
		TenantID:          "tenant-worker",
		ProjectID:         "project-worker",
		Type:              TypeImageGeneration,
		ProviderID:        "provider-worker",
		ModelID:           "model-worker",
		Status:            status,
		Prompt:            "worker prompt",
		ParamsJSON:        `{}`,
		InputAssetIDsJSON: `[]`,
		Attempt:           1,
		MaxAttempts:       3,
		QueuedAt:          &now,
		StartedAt:         startedAt,
		TimeoutAt:         &timeoutAt,
		CreatedBy:         "user-worker",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if terminalStatus(status) {
		finishedAt := now
		record.FinishedAt = &finishedAt
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed task %s: %v", taskID, err)
	}
	return taskID
}

func loadWorkerTask(t *testing.T, db *gorm.DB, taskID string) database.GenerationTask {
	t.Helper()
	var record database.GenerationTask
	if err := db.Where("tenant_id = ? AND id = ?", "tenant-worker", taskID).First(&record).Error; err != nil {
		t.Fatalf("load task %s: %v", taskID, err)
	}
	return record
}

func assertWorkerEvents(t *testing.T, db *gorm.DB, taskID string, expected []string) {
	t.Helper()
	var events []database.TaskEvent
	if err := db.Where("tenant_id = ? AND task_id = ?", "tenant-worker", taskID).Order("sequence ASC").Find(&events).Error; err != nil {
		t.Fatalf("load events: %v", err)
	}
	if len(events) != len(expected) {
		t.Fatalf("event count = %d, want %d: %#v", len(events), len(expected), events)
	}
	for index, event := range events {
		if event.EventType != expected[index] {
			t.Fatalf("event %d type = %q, want %q; events=%#v", index, event.EventType, expected[index], events)
		}
		if event.Sequence == 0 || event.ID != EventIDFromSequence(event.Sequence) {
			t.Fatalf("event %d has unstable replay cursor: %#v", index, event)
		}
	}
}

func assertWorkerNoOutputsOrUsage(t *testing.T, db *gorm.DB) {
	t.Helper()
	var outputCount int64
	if err := db.Model(&database.TaskOutput{}).Count(&outputCount).Error; err != nil {
		t.Fatalf("count task outputs: %v", err)
	}
	var usageCount int64
	if err := db.Model(&database.UsageRecord{}).Count(&usageCount).Error; err != nil {
		t.Fatalf("count usage records: %v", err)
	}
	if outputCount != 0 || usageCount != 0 {
		t.Fatalf("stub worker created outputs=%d usage=%d, want zero", outputCount, usageCount)
	}
}

func assertWorkerEventsSanitized(t *testing.T, db *gorm.DB) {
	t.Helper()
	var events []database.TaskEvent
	if err := db.Find(&events).Error; err != nil {
		t.Fatalf("load all events: %v", err)
	}
	for _, event := range events {
		lower := strings.ToLower(event.EventPayloadJSON)
		for _, forbidden := range []string{"authorization", "cookie", "api_key", "apikey", "secret", "base64"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("event payload leaked %q: %s", forbidden, event.EventPayloadJSON)
			}
		}
	}
}

func assertWorkerRuntimeMetadataSanitized(t *testing.T, db *gorm.DB) {
	t.Helper()
	var apiLogs []database.APICallLog
	if err := db.Find(&apiLogs).Error; err != nil {
		t.Fatalf("load api logs: %v", err)
	}
	var usageRecords []database.UsageRecord
	if err := db.Find(&usageRecords).Error; err != nil {
		t.Fatalf("load usage records: %v", err)
	}
	combined := ""
	for _, record := range apiLogs {
		combined += record.ErrorMessage + record.RedactedRequestJSON + record.RedactedResponseJSON
	}
	for _, record := range usageRecords {
		combined += record.RawUsageJSON
	}
	lower := strings.ToLower(combined)
	for _, forbidden := range []string{"sk-secret", "authorization", "cookie", "bearer", "base64", "b64_json", "must-redact"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("runtime metadata leaked %q: %s", forbidden, combined)
		}
	}
}

func assertNoWorkerPersistedText(t *testing.T, db *gorm.DB, value string) {
	t.Helper()
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return
	}
	var apiLogs []database.APICallLog
	if err := db.Find(&apiLogs).Error; err != nil {
		t.Fatalf("load api logs: %v", err)
	}
	var taskEvents []database.TaskEvent
	if err := db.Find(&taskEvents).Error; err != nil {
		t.Fatalf("load task events: %v", err)
	}
	var tasks []database.GenerationTask
	if err := db.Find(&tasks).Error; err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	combined := ""
	for _, record := range apiLogs {
		combined += record.ErrorMessage + record.RedactedRequestJSON + record.RedactedResponseJSON
	}
	for _, event := range taskEvents {
		combined += event.EventPayloadJSON
	}
	for _, taskRecord := range tasks {
		combined += taskRecord.ErrorMessage
	}
	if strings.Contains(strings.ToLower(combined), value) {
		t.Fatalf("persisted worker data leaked %q: %s", value, combined)
	}
}

func assertTableCount(t *testing.T, db *gorm.DB, model any, expected int64) {
	t.Helper()
	var count int64
	if err := db.Model(model).Count(&count).Error; err != nil {
		t.Fatalf("count table: %v", err)
	}
	if count != expected {
		t.Fatalf("table count = %d, want %d", count, expected)
	}
}

func workerTinyPNG(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC")
	if err != nil {
		t.Fatalf("decode tiny png: %v", err)
	}
	return data
}

var _ queue.ConcurrencyLimiter = (*recordingLimiter)(nil)
var _ queue.ReliableTaskQueue = (*recordingReliableQueue)(nil)
var _ storage.ObjectStore = (*memoryObjectStore)(nil)
var _ Executor = executorFunc(nil)
var _ EventPublisher = (*recordingPublisher)(nil)

func TestWorkerProcessorMissingTaskIsAcked(t *testing.T) {
	db := newWorkerTestDB(t)
	processor := newWorkerTestProcessor(db, WorkerProcessorOptions{})
	result, err := processor.Process(context.Background(), queue.TaskClaim{TaskID: "missing", DeliveryCount: 1})
	if err != nil && !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing task error = %v", err)
	}
	if result.Action != claimActionAck {
		t.Fatalf("missing task action = %v, want ack", result.Action)
	}
}
