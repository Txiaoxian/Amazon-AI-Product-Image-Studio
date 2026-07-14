package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

var errCaptchaNotFound = errors.New("captcha not found or expired")

type CaptchaStore interface {
	Put(ctx context.Context, captchaID string, record CaptchaRecord, ttl time.Duration) error
	Image(ctx context.Context, captchaID string) ([]byte, error)
	ConsumeDigest(ctx context.Context, captchaID string) (string, error)
}

type CaptchaRecord struct {
	Digest   string `json:"digest"`
	ImagePNG []byte `json:"imagePng"`
}

type redisCaptchaClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	GetDel(ctx context.Context, key string) *redis.StringCmd
}

type RedisCaptchaStore struct {
	client redisCaptchaClient
}

func NewRedisCaptchaStore(queueConfig config.QueueConfig) *RedisCaptchaStore {
	return &RedisCaptchaStore{client: redis.NewClient(&redis.Options{
		Addr:     queueConfig.RedisAddr,
		Password: queueConfig.RedisPassword,
		DB:       queueConfig.RedisDB,
	})}
}

func (s *RedisCaptchaStore) Put(ctx context.Context, captchaID string, record CaptchaRecord, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return errors.New("captcha store is unavailable")
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.client.Set(normalizeContext(ctx), captchaKey(captchaID), payload, ttl).Err()
}

func (s *RedisCaptchaStore) Image(ctx context.Context, captchaID string) ([]byte, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("captcha store is unavailable")
	}
	payload, err := s.client.Get(normalizeContext(ctx), captchaKey(captchaID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, errCaptchaNotFound
	}
	if err != nil {
		return nil, err
	}
	record, err := decodeCaptchaRecord(payload)
	if err != nil {
		return nil, err
	}
	return record.ImagePNG, nil
}

func (s *RedisCaptchaStore) ConsumeDigest(ctx context.Context, captchaID string) (string, error) {
	if s == nil || s.client == nil {
		return "", errors.New("captcha store is unavailable")
	}
	payload, err := s.client.GetDel(normalizeContext(ctx), captchaKey(captchaID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return "", errCaptchaNotFound
	}
	if err != nil {
		return "", err
	}
	record, err := decodeCaptchaRecord(payload)
	if err != nil {
		return "", err
	}
	return record.Digest, nil
}

type memoryCaptchaEntry struct {
	record    CaptchaRecord
	expiresAt time.Time
}

type MemoryCaptchaStore struct {
	mu      sync.Mutex
	entries map[string]memoryCaptchaEntry
	now     func() time.Time
}

func NewMemoryCaptchaStore() *MemoryCaptchaStore {
	return &MemoryCaptchaStore{
		entries: map[string]memoryCaptchaEntry{},
		now:     time.Now,
	}
}

func (s *MemoryCaptchaStore) Put(_ context.Context, captchaID string, record CaptchaRecord, ttl time.Duration) error {
	if s == nil {
		return errors.New("captcha store is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureEntries()
	s.entries[captchaKey(captchaID)] = memoryCaptchaEntry{
		record:    CaptchaRecord{Digest: record.Digest, ImagePNG: append([]byte(nil), record.ImagePNG...)},
		expiresAt: s.currentTime().Add(ttl),
	}
	return nil
}

func (s *MemoryCaptchaStore) Image(_ context.Context, captchaID string) ([]byte, error) {
	if s == nil {
		return nil, errors.New("captcha store is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.validEntry(captchaKey(captchaID))
	if !ok {
		return nil, errCaptchaNotFound
	}
	return append([]byte(nil), entry.record.ImagePNG...), nil
}

func (s *MemoryCaptchaStore) ConsumeDigest(_ context.Context, captchaID string) (string, error) {
	if s == nil {
		return "", errors.New("captcha store is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := captchaKey(captchaID)
	entry, ok := s.validEntry(key)
	delete(s.entries, key)
	if !ok {
		return "", errCaptchaNotFound
	}
	return entry.record.Digest, nil
}

func (s *MemoryCaptchaStore) validEntry(key string) (memoryCaptchaEntry, bool) {
	s.ensureEntries()
	entry, ok := s.entries[key]
	if !ok {
		return memoryCaptchaEntry{}, false
	}
	if !entry.expiresAt.After(s.currentTime()) {
		delete(s.entries, key)
		return memoryCaptchaEntry{}, false
	}
	return entry, true
}

func (s *MemoryCaptchaStore) ensureEntries() {
	if s.entries == nil {
		s.entries = map[string]memoryCaptchaEntry{}
	}
}

func (s *MemoryCaptchaStore) currentTime() time.Time {
	if s.now == nil {
		return time.Now()
	}
	return s.now()
}

func decodeCaptchaRecord(payload []byte) (CaptchaRecord, error) {
	var record CaptchaRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return CaptchaRecord{}, err
	}
	if strings.TrimSpace(record.Digest) == "" || len(record.ImagePNG) == 0 {
		return CaptchaRecord{}, errors.New("captcha record is invalid")
	}
	return record, nil
}

func captchaDigest(code string) string {
	digest := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))))
	return hex.EncodeToString(digest[:])
}

func captchaKey(captchaID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(captchaID)))
	return "auth:captcha:" + hex.EncodeToString(digest[:])
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
