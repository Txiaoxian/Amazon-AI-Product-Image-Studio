package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/queue"
	"github.com/gin-gonic/gin"
)

func TestNewRouterServesBaseAndVersionedHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(RouterOptions{
		Config: config.Config{},
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	})

	for _, path := range []string{"/healthz", "/api/v1/healthz"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, response.Code, http.StatusOK)
		}
	}
}

func TestNewRouterAppliesConfiguredCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(RouterOptions{
		Config: config.Config{
			API: config.APIConfig{
				CORSAllowedOrigins: []string{"https://studio.example.com"},
			},
		},
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	request.Header.Set("Origin", "https://studio.example.com")
	router.ServeHTTP(response, request)

	if response.Header().Get("Access-Control-Allow-Origin") != "https://studio.example.com" {
		t.Fatalf("CORS origin header = %q, want https://studio.example.com", response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestNewRouterStartsRedisTaskEventBridgeWithLifecycleContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lifecycleCtx, cancel := context.WithCancel(context.Background())
	subscriber := newRouterTaskEventSubscriber(nil)

	router := NewRouter(RouterOptions{
		Config:              config.Config{AppEnv: "production"},
		Logger:              slog.New(slog.NewJSONHandler(io.Discard, nil)),
		LifecycleContext:    lifecycleCtx,
		TaskEventSubscriber: subscriber,
	})
	if router == nil {
		t.Fatal("NewRouter returned nil")
	}

	startedCtx := waitForRouterTaskEventSubscriberStart(t, subscriber.started)
	select {
	case <-startedCtx.Done():
		t.Fatal("subscriber context was cancelled before lifecycle cancellation")
	default:
	}

	cancel()
	waitForRouterTaskEventSubscriberDone(t, subscriber.done)
	if !errors.Is(startedCtx.Err(), context.Canceled) {
		t.Fatalf("subscriber context error = %v, want context.Canceled", startedCtx.Err())
	}
}

func TestNewRouterDoesNotStartRedisTaskEventBridgeInTestEnvByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	subscriber := newRouterTaskEventSubscriber(nil)

	router := NewRouter(RouterOptions{
		Config:              config.Config{AppEnv: "test"},
		Logger:              slog.New(slog.NewJSONHandler(io.Discard, nil)),
		LifecycleContext:    context.Background(),
		TaskEventSubscriber: subscriber,
	})
	if router == nil {
		t.Fatal("NewRouter returned nil")
	}

	select {
	case <-subscriber.started:
		t.Fatal("test environment unexpectedly started task event subscriber")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestNewRouterLogsTaskEventSubscriberErrorWithoutCrashing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expectedErr := errors.New("subscriber boom")
	subscriber := newRouterTaskEventSubscriber(expectedErr)
	var logs lockedLogBuffer

	router := NewRouter(RouterOptions{
		Config:              config.Config{AppEnv: "development"},
		Logger:              slog.New(slog.NewJSONHandler(&logs, nil)),
		LifecycleContext:    context.Background(),
		TaskEventSubscriber: subscriber,
	})
	if router == nil {
		t.Fatal("NewRouter returned nil")
	}
	waitForRouterTaskEventSubscriberDone(t, subscriber.done)
	waitForRouterLogContains(t, &logs, "task event wakeup subscriber stopped")

	if !strings.Contains(logs.String(), "task event wakeup subscriber stopped") || !strings.Contains(logs.String(), expectedErr.Error()) {
		t.Fatalf("subscriber error was not logged: %s", logs.String())
	}
}

type routerTaskEventSubscriber struct {
	err     error
	started chan context.Context
	done    chan struct{}
}

func newRouterTaskEventSubscriber(err error) *routerTaskEventSubscriber {
	return &routerTaskEventSubscriber{
		err:     err,
		started: make(chan context.Context, 1),
		done:    make(chan struct{}),
	}
}

func (s *routerTaskEventSubscriber) Run(ctx context.Context, _ queue.TaskEventSink, _ *slog.Logger) error {
	defer close(s.done)
	s.started <- ctx
	if s.err != nil {
		return s.err
	}
	<-ctx.Done()
	return ctx.Err()
}

func waitForRouterTaskEventSubscriberStart(t *testing.T, started <-chan context.Context) context.Context {
	t.Helper()
	select {
	case ctx := <-started:
		return ctx
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for task event subscriber to start")
		return nil
	}
}

func waitForRouterTaskEventSubscriberDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for task event subscriber to stop")
	}
}

type lockedLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitForRouterLogContains(t *testing.T, logs *lockedLogBuffer, needle string) {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if strings.Contains(logs.String(), needle) {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for log %q; logs: %s", needle, logs.String())
		case <-ticker.C:
		}
	}
}
