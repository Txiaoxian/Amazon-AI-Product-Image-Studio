package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func TestHandlerReturnsHealthyStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(httpx.RequestID())
	router.GET("/healthz", Handler("api"))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body struct {
		Data Status `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Data.Status != "ok" {
		t.Fatalf("status = %q, want ok", body.Data.Status)
	}
	if body.Data.Service != "api" {
		t.Fatalf("service = %q, want api", body.Data.Service)
	}
}

func TestHandlerReturnsUnavailableWhenDependencyFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(httpx.RequestID())
	router.GET("/healthz", Handler("api", failingChecker{}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}

	var body struct {
		Data Status `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Data.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", body.Data.Status)
	}
	if body.Data.Dependencies["database"] != "unhealthy" {
		t.Fatalf("database dependency = %q, want unhealthy", body.Data.Dependencies["database"])
	}
}

func TestRedisCheckerPingsRedis(t *testing.T) {
	checker := NewRedisCheckerWithClient(fakeRedisPinger{})

	if err := checker.Check(context.Background()); err != nil {
		t.Fatalf("RedisChecker.Check returned error: %v", err)
	}
	if checker.Name() != "redis" {
		t.Fatalf("RedisChecker.Name() = %q, want redis", checker.Name())
	}
}

func TestRedisCheckerReportsPingFailure(t *testing.T) {
	checker := NewRedisCheckerWithClient(fakeRedisPinger{err: errors.New("redis down")})

	if err := checker.Check(context.Background()); err == nil {
		t.Fatal("RedisChecker.Check returned nil error for failed ping")
	}
}

func TestMinIOCheckerRequiresConfiguredBuckets(t *testing.T) {
	store := &fakeMinIOBucketChecker{
		buckets: map[string]bool{
			"product-originals": true,
			"product-generated": true,
		},
	}
	checker := NewMinIOCheckerWithClient(store, "product-originals", "product-generated", "product-originals", "  ")

	if err := checker.Check(context.Background()); err != nil {
		t.Fatalf("MinIOChecker.Check returned error: %v", err)
	}
	if checker.Name() != "minio" {
		t.Fatalf("MinIOChecker.Name() = %q, want minio", checker.Name())
	}
	if len(store.checked) != 2 {
		t.Fatalf("checked buckets = %v, want two unique bucket checks", store.checked)
	}
}

func TestMinIOCheckerReportsMissingBucket(t *testing.T) {
	checker := NewMinIOCheckerWithClient(&fakeMinIOBucketChecker{
		buckets: map[string]bool{"product-originals": true},
	}, "product-originals", "product-generated")

	if err := checker.Check(context.Background()); err == nil {
		t.Fatal("MinIOChecker.Check returned nil error for missing bucket")
	}
}

type failingChecker struct{}

func (failingChecker) Name() string {
	return "database"
}

func (failingChecker) Check(context.Context) error {
	return errors.New("dependency unavailable")
}

type fakeRedisPinger struct {
	err error
}

func (f fakeRedisPinger) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	if f.err != nil {
		cmd.SetErr(f.err)
		return cmd
	}
	cmd.SetVal("PONG")
	return cmd
}

type fakeMinIOBucketChecker struct {
	buckets map[string]bool
	checked []string
	err     error
}

func (f *fakeMinIOBucketChecker) BucketExists(_ context.Context, bucket string) (bool, error) {
	f.checked = append(f.checked, bucket)
	if f.err != nil {
		return false, f.err
	}
	return f.buckets[bucket], nil
}
