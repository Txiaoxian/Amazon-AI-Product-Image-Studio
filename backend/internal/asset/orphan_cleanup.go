package asset

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
)

const (
	defaultOrphanBatchLimit       = 100
	maxOrphanBatchLimit           = 1000
	defaultOrphanGracePeriodHours = 24
	maxOrphanGracePeriodHours     = 24 * 365
	orphanCleanupConfirm          = "DELETE_ORPHANS"
)

type orphanOperationRequest struct {
	DryRun           *bool  `json:"dryRun"`
	Confirm          string `json:"confirm"`
	BatchLimit       int    `json:"batchLimit"`
	GracePeriodHours int    `json:"gracePeriodHours"`
	Cursor           string `json:"cursor"`
}

type orphanOperationOptions struct {
	DryRun           bool
	Confirm          string
	BatchLimit       int
	GracePeriodHours int
	Cursor           string
}

type OrphanOperationResponse struct {
	DryRun           bool           `json:"dryRun"`
	BatchLimit       int            `json:"batchLimit"`
	GracePeriodHours int            `json:"gracePeriodHours"`
	Scanned          int            `json:"scanned"`
	Candidates       int            `json:"candidates"`
	Deleted          int            `json:"deleted"`
	HasMore          bool           `json:"hasMore"`
	NextCursor       string         `json:"nextCursor,omitempty"`
	CandidateIDs     []string       `json:"candidateIds"`
	Skipped          map[string]int `json:"skipped"`
	Errors           map[string]int `json:"errors"`
}

type backendObjectKey struct {
	TenantID  string
	ProjectID string
	AssetID   string
	FileKind  string
}

type orphanObjectClass struct {
	Name   string
	Bucket string
}

type orphanCursorState struct {
	TenantID      string `json:"tenantId"`
	ClassIndex    int    `json:"classIndex"`
	ObjectCursor  string `json:"objectCursor"`
	StorageConfig string `json:"storageConfig"`
}

func (s *Service) ScanStorageOrphans(c *gin.Context) {
	principal, ok := requireStorageOrphanAdmin(c)
	if !ok {
		return
	}
	options, err := parseOrphanOperationRequest(c.Request.Body)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}
	if options.DryRun != nil && !*options.DryRun {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.inspectStorageOrphans(c.Request.Context(), principal, orphanOperationOptions{
		DryRun:           true,
		BatchLimit:       options.BatchLimit,
		GracePeriodHours: options.GracePeriodHours,
		Cursor:           options.Cursor,
	})
	if err != nil {
		s.respondOrphanError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) CleanupStorageOrphans(c *gin.Context) {
	principal, ok := requireStorageOrphanAdmin(c)
	if !ok {
		return
	}
	request, err := parseOrphanOperationRequest(c.Request.Body)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}
	dryRun := true
	if request.DryRun != nil {
		dryRun = *request.DryRun
	}
	if !dryRun && strings.TrimSpace(request.Confirm) != orphanCleanupConfirm {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.inspectStorageOrphans(c.Request.Context(), principal, orphanOperationOptions{
		DryRun:           dryRun,
		Confirm:          request.Confirm,
		BatchLimit:       request.BatchLimit,
		GracePeriodHours: request.GracePeriodHours,
		Cursor:           request.Cursor,
	})
	if err != nil {
		s.respondOrphanError(c, err)
		return
	}
	if err := s.recordOrphanCleanupAudit(c.Request.Context(), principal, response, c.ClientIP(), c.Request.UserAgent()); err != nil {
		s.respondOrphanError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) inspectStorageOrphans(ctx context.Context, principal auth.Principal, options orphanOperationOptions) (OrphanOperationResponse, error) {
	if s.db == nil {
		return OrphanOperationResponse{}, database.ErrNilDB
	}
	if s.store == nil {
		return OrphanOperationResponse{}, ErrStorageUnavailable
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return OrphanOperationResponse{}, err
	}

	limit := normalizeOrphanBatchLimit(options.BatchLimit)
	gracePeriodHours := normalizeOrphanGracePeriodHours(options.GracePeriodHours)
	classes := orphanObjectClasses(s.storageConfig)
	cursorState, err := s.decodeOrphanCursor(scope.ID(), options.Cursor, len(classes))
	if err != nil {
		return OrphanOperationResponse{}, err
	}
	cutoff := s.now().Add(-time.Duration(gracePeriodHours) * time.Hour)
	response := OrphanOperationResponse{
		DryRun:           options.DryRun,
		BatchLimit:       limit,
		GracePeriodHours: gracePeriodHours,
		CandidateIDs:     []string{},
		Skipped:          map[string]int{},
		Errors:           map[string]int{},
	}

	seenObjects := map[string]struct{}{}
	for classIndex := cursorState.ClassIndex; classIndex < len(classes); classIndex++ {
		if response.Scanned >= limit {
			break
		}
		class := classes[classIndex]
		objectCursor := ""
		if classIndex == cursorState.ClassIndex {
			objectCursor = cursorState.ObjectCursor
		}
		listResult, err := s.store.ListObjects(ctx, storage.ListObjectsInput{
			Bucket: class.Bucket,
			Prefix: "tenants/" + scope.ID() + "/",
			Cursor: objectCursor,
			Limit:  limit - response.Scanned,
		})
		if err != nil {
			return OrphanOperationResponse{}, err
		}
		var lastProcessedCursor string
		for _, object := range listResult.Objects {
			if response.Scanned >= limit {
				if lastProcessedCursor != "" {
					if err := s.setOrphanNextCursor(scope.ID(), classIndex, lastProcessedCursor, &response); err != nil {
						return OrphanOperationResponse{}, err
					}
				}
				return response, nil
			}
			lastProcessedCursor = object.Key
			response.Scanned++
			if err := s.inspectStorageObject(ctx, scope, class, object, cutoff, seenObjects, &response); err != nil {
				return OrphanOperationResponse{}, err
			}
		}
		if listResult.NextCursor != "" {
			if err := s.setOrphanNextCursor(scope.ID(), classIndex, listResult.NextCursor, &response); err != nil {
				return OrphanOperationResponse{}, err
			}
			return response, nil
		}
		if response.Scanned >= limit && classIndex+1 < len(classes) {
			if err := s.setOrphanNextCursor(scope.ID(), classIndex+1, "", &response); err != nil {
				return OrphanOperationResponse{}, err
			}
			return response, nil
		}
	}

	return response, nil
}

func (s *Service) inspectStorageObject(ctx context.Context, scope tenant.Scope, class orphanObjectClass, object storage.ListedObject, cutoff time.Time, seenObjects map[string]struct{}, response *OrphanOperationResponse) error {
	parsed, ok := parseBackendObjectKey(object.Key)
	if !ok {
		response.Skipped["unrecognized_pattern"]++
		return nil
	}
	if parsed.TenantID != scope.ID() {
		response.Skipped["tenant_mismatch"]++
		return nil
	}
	if !objectClassMatches(class.Name, parsed.FileKind) {
		response.Skipped["bucket_pattern_mismatch"]++
		return nil
	}
	seenKey := class.Bucket + "\x00" + object.Key
	if _, ok := seenObjects[seenKey]; ok {
		response.Skipped["duplicate_listing"]++
		return nil
	}
	seenObjects[seenKey] = struct{}{}
	if object.LastModified.IsZero() {
		response.Skipped["missing_last_modified"]++
		return nil
	}
	if !object.LastModified.Before(cutoff) {
		response.Skipped["too_new"]++
		return nil
	}

	referenced, err := s.repo.ObjectKeyReferenced(ctx, scope, object.Key)
	if err != nil {
		return err
	}
	if referenced {
		response.Skipped["metadata_referenced"]++
		return nil
	}

	response.Candidates++
	candidateID := storageOrphanCandidateID(class.Name, object.Key)
	response.CandidateIDs = append(response.CandidateIDs, candidateID)
	if response.DryRun {
		return nil
	}

	if err := removeObjectIfPresent(ctx, s.store, class.Bucket, object.Key); err != nil {
		errorKind := safeCleanupErrorKind(err)
		response.Errors[errorKind]++
		s.log.Warn("storage orphan delete failed", slog.String("tenant_id", scope.ID()), slog.String("candidate_id", candidateID), slog.String("object_class", class.Name), slog.String("error_kind", errorKind))
		return nil
	}
	response.Deleted++
	return nil
}

func (s *Service) setOrphanNextCursor(tenantID string, classIndex int, objectCursor string, response *OrphanOperationResponse) error {
	cursor, err := s.encodeOrphanCursor(orphanCursorState{
		TenantID:      tenantID,
		ClassIndex:    classIndex,
		ObjectCursor:  objectCursor,
		StorageConfig: orphanCursorConfigFingerprint(s.storageConfig),
	})
	if err != nil {
		return err
	}
	response.HasMore = true
	response.NextCursor = cursor
	return nil
}

func (s *Service) recordOrphanCleanupAudit(ctx context.Context, principal auth.Principal, response OrphanOperationResponse, ip string, userAgent string) error {
	return audit.NewRecorder(s.db).Record(ctx, audit.Event{
		TenantID:     principal.TenantID,
		ActorUserID:  &principal.UserID,
		Action:       "storage.orphan_cleanup",
		ResourceType: "storage",
		ResourceID:   "orphans",
		IP:           ip,
		UserAgent:    userAgent,
		Metadata: map[string]any{
			"dryRun":           response.DryRun,
			"batchLimit":       response.BatchLimit,
			"gracePeriodHours": response.GracePeriodHours,
			"scanned":          response.Scanned,
			"candidates":       response.Candidates,
			"deleted":          response.Deleted,
			"hasMore":          response.HasMore,
			"candidateIds":     response.CandidateIDs,
			"skipped":          response.Skipped,
			"errors":           response.Errors,
		},
	})
}

func requireStorageOrphanAdmin(c *gin.Context) (auth.Principal, bool) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return auth.Principal{}, false
	}
	if !isTenantAdmin(principal) && !principal.HasPermission(settings.PermissionManage) {
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
		return auth.Principal{}, false
	}
	return principal, true
}

func isTenantAdmin(principal auth.Principal) bool {
	for _, role := range principal.Roles {
		if role.Code == "admin" {
			return true
		}
	}
	return false
}

func (s *Service) respondOrphanError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrValidation), errors.Is(err, tenant.ErrMissingTenantID):
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
	case errors.Is(err, ErrForbidden):
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
	case errors.Is(err, ErrStorageUnavailable), errors.Is(err, storage.ErrUnavailable):
		s.log.Error("storage orphan request failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error_kind", safeCleanupErrorKind(err)))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	default:
		s.log.Error("storage orphan request failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error_kind", safeCleanupErrorKind(err)))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}

func parseOrphanOperationRequest(body io.Reader) (orphanOperationRequest, error) {
	if body == nil {
		return orphanOperationRequest{}, nil
	}
	raw, err := io.ReadAll(io.LimitReader(body, 1024*1024))
	if err != nil {
		return orphanOperationRequest{}, ErrValidation
	}
	if strings.TrimSpace(string(raw)) == "" {
		return orphanOperationRequest{}, nil
	}
	var request orphanOperationRequest
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return orphanOperationRequest{}, ErrValidation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return orphanOperationRequest{}, ErrValidation
	}
	if request.BatchLimit < 0 || request.GracePeriodHours < 0 {
		return orphanOperationRequest{}, ErrValidation
	}
	return request, nil
}

func normalizeOrphanBatchLimit(limit int) int {
	if limit <= 0 {
		return defaultOrphanBatchLimit
	}
	if limit > maxOrphanBatchLimit {
		return maxOrphanBatchLimit
	}
	return limit
}

func normalizeOrphanGracePeriodHours(hours int) int {
	if hours <= 0 {
		return defaultOrphanGracePeriodHours
	}
	if hours > maxOrphanGracePeriodHours {
		return maxOrphanGracePeriodHours
	}
	return hours
}

func (s *Service) encodeOrphanCursor(state orphanCursorState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(orphanCursorKey(s.storageConfig))
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, payload, nil)
	return "v1." + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (s *Service) decodeOrphanCursor(tenantID string, raw string, classCount int) (orphanCursorState, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return orphanCursorState{TenantID: tenantID}, nil
	}
	if !strings.HasPrefix(raw, "v1.") {
		return orphanCursorState{}, ErrValidation
	}
	sealed, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, "v1."))
	if err != nil {
		return orphanCursorState{}, ErrValidation
	}
	block, err := aes.NewCipher(orphanCursorKey(s.storageConfig))
	if err != nil {
		return orphanCursorState{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return orphanCursorState{}, err
	}
	if len(sealed) <= aead.NonceSize() {
		return orphanCursorState{}, ErrValidation
	}
	nonce := sealed[:aead.NonceSize()]
	ciphertext := sealed[aead.NonceSize():]
	payload, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return orphanCursorState{}, ErrValidation
	}
	var state orphanCursorState
	if err := json.Unmarshal(payload, &state); err != nil {
		return orphanCursorState{}, ErrValidation
	}
	if state.TenantID != tenantID || state.ClassIndex < 0 || state.ClassIndex >= classCount || state.StorageConfig != orphanCursorConfigFingerprint(s.storageConfig) {
		return orphanCursorState{}, ErrValidation
	}
	return state, nil
}

func orphanCursorKey(storageConfig config.StorageConfig) []byte {
	storageConfig = config.NormalizeStorageConfig(storageConfig)
	sum := sha256.Sum256([]byte("storage-orphan-cursor\x00" + storageConfig.Endpoint + "\x00" + storageConfig.AccessKey + "\x00" + storageConfig.SecretKey))
	return sum[:]
}

func orphanCursorConfigFingerprint(storageConfig config.StorageConfig) string {
	storageConfig = config.NormalizeStorageConfig(storageConfig)
	sum := sha256.Sum256([]byte(storageConfig.BucketOriginals + "\x00" + storageConfig.BucketGenerated + "\x00" + storageConfig.BucketThumbnails))
	return hex.EncodeToString(sum[:16])
}

func orphanObjectClasses(storageConfig config.StorageConfig) []orphanObjectClass {
	storageConfig = config.NormalizeStorageConfig(storageConfig)
	return []orphanObjectClass{
		{Name: "originals", Bucket: storageConfig.BucketOriginals},
		{Name: "generated", Bucket: storageConfig.BucketGenerated},
		{Name: "thumbnails", Bucket: storageConfig.BucketThumbnails},
	}
}

func parseBackendObjectKey(key string) (backendObjectKey, bool) {
	parts := strings.Split(strings.TrimSpace(key), "/")
	if len(parts) != 7 {
		return backendObjectKey{}, false
	}
	if parts[0] != "tenants" || parts[2] != "projects" || parts[4] != "assets" {
		return backendObjectKey{}, false
	}
	if parts[1] == "" || parts[3] == "" || parts[5] == "" {
		return backendObjectKey{}, false
	}

	switch parts[6] {
	case "original.jpg", "original.png", "original.webp":
		return backendObjectKey{TenantID: parts[1], ProjectID: parts[3], AssetID: parts[5], FileKind: "original"}, true
	case "thumbnail.jpg":
		return backendObjectKey{TenantID: parts[1], ProjectID: parts[3], AssetID: parts[5], FileKind: "thumbnail"}, true
	default:
		return backendObjectKey{}, false
	}
}

func objectClassMatches(className string, fileKind string) bool {
	switch className {
	case "originals", "generated":
		return fileKind == "original"
	case "thumbnails":
		return fileKind == "thumbnail"
	default:
		return false
	}
}

func storageOrphanCandidateID(className string, objectKey string) string {
	sum := sha256.Sum256([]byte(className + "\x00" + objectKey))
	return "sha256:" + hex.EncodeToString(sum[:])
}
