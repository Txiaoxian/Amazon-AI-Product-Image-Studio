package api

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/sse"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
)

func TestTaskSSELastEventIDReplaysOnlyVisibleEventsAfterCursor(t *testing.T) {
	router, server, db, _, adminSession := newSSERouteTestServer(t, 200*time.Millisecond)
	projectID := createTaskTestProject(t, router, adminSession, "SSE Replay Project")
	seedSSETask(t, db, adminSession.tenantID, projectID, adminSession.userID, "task-replay")
	first := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-replay", task.EventTaskQueued)
	second := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-replay", "TASK_PROGRESS")

	response, cancel := openTaskSSE(t, server, "/api/v1/events/tasks", adminSession.cookies, map[string]string{"Last-Event-ID": first.ID})
	defer closeSSE(response, cancel)

	frame := readSSEFrame(t, bufio.NewReader(response.Body))
	assertSSETaskEventFrame(t, frame, second)
	if strings.Contains(frame, first.ID) {
		t.Fatalf("replayed cursor event %s in frame: %q", first.ID, frame)
	}
}

func TestTaskSSELastEventIDQueryFallbackMatchesHeaderReplay(t *testing.T) {
	router, server, db, _, adminSession := newSSERouteTestServer(t, 200*time.Millisecond)
	projectID := createTaskTestProject(t, router, adminSession, "SSE Query Cursor Project")
	seedSSETask(t, db, adminSession.tenantID, projectID, adminSession.userID, "task-query-cursor")
	first := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-query-cursor", task.EventTaskQueued)
	second := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-query-cursor", task.EventTaskCancelled)

	response, cancel := openTaskSSE(t, server, "/api/v1/events/tasks?lastEventId="+first.ID, adminSession.cookies, nil)
	defer closeSSE(response, cancel)

	frame := readSSEFrame(t, bufio.NewReader(response.Body))
	assertSSETaskEventFrame(t, frame, second)
}

func TestTaskSSEReplayOrderingAndCursorAfterLatestHeartbeat(t *testing.T) {
	router, server, db, _, adminSession := newSSERouteTestServer(t, 5*time.Millisecond)
	projectID := createTaskTestProject(t, router, adminSession, "SSE Ordered Replay Project")
	seedSSETask(t, db, adminSession.tenantID, projectID, adminSession.userID, "task-ordered-replay")
	first := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-ordered-replay", task.EventTaskQueued)
	second := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-ordered-replay", "TASK_PROGRESS")
	third := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-ordered-replay", task.EventTaskCompleted)

	response, cancel := openTaskSSE(t, server, "/api/v1/events/tasks?lastEventId=evt_00000000000000000000", adminSession.cookies, nil)
	reader := bufio.NewReader(response.Body)
	assertSSETaskEventFrame(t, readSSEFrame(t, reader), first)
	assertSSETaskEventFrame(t, readSSEFrame(t, reader), second)
	assertSSETaskEventFrame(t, readSSEFrame(t, reader), third)
	closeSSE(response, cancel)

	afterLatest, afterLatestCancel := openTaskSSE(t, server, "/api/v1/events/tasks?lastEventId="+third.ID, adminSession.cookies, nil)
	defer closeSSE(afterLatest, afterLatestCancel)
	frame := readSSEFrame(t, bufio.NewReader(afterLatest.Body))
	if !strings.Contains(frame, "event: HEARTBEAT\n") {
		t.Fatalf("cursor after latest frame = %q, want heartbeat/no replay", frame)
	}
	for _, forbidden := range []string{first.ID, second.ID, third.ID, "task-ordered-replay"} {
		if strings.Contains(frame, forbidden) {
			t.Fatalf("cursor after latest replayed %q: %q", forbidden, frame)
		}
	}
}

func TestTaskSSEHeartbeatCatchesUpPersistedEventsWithoutBrokerNotification(t *testing.T) {
	router, server, db, broker, adminSession := newSSERouteTestServer(t, 5*time.Millisecond)
	projectID := createTaskTestProject(t, router, adminSession, "SSE Heartbeat Catchup Project")
	seedSSETask(t, db, adminSession.tenantID, projectID, adminSession.userID, "task-heartbeat-catchup")

	response, cancel := openTaskSSE(t, server, "/api/v1/events/tasks", adminSession.cookies, nil)
	defer closeSSE(response, cancel)
	reader := bufio.NewReader(response.Body)
	waitForSubscribers(t, broker, 1)

	event := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-heartbeat-catchup", "TASK_PROGRESS")

	frame := readSSEFrame(t, reader)
	assertSSETaskEventFrame(t, frame, event)
}

func TestTaskSSEClosesWhenSessionVersionIsRevoked(t *testing.T) {
	router, server, _, broker, adminSession := newSSERouteTestServer(t, 5*time.Millisecond)

	response, cancel := openTaskSSE(t, server, "/api/v1/events/tasks", adminSession.cookies, nil)
	defer closeSSE(response, cancel)
	reader := bufio.NewReader(response.Body)
	waitForSubscribers(t, broker, 1)

	logoutResponse := performJSON(router, http.MethodPost, "/api/v1/auth/logout", nil, adminSession.cookies, adminSession.csrfHeader())
	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want %d: %s", logoutResponse.Code, http.StatusOK, logoutResponse.Body.String())
	}
	readSSEUntilClosed(t, reader)
}

func TestTaskSSEReplayIsBoundedAndContinuesOnHeartbeatCatchup(t *testing.T) {
	router, server, db, _, adminSession := newSSERouteTestServerWithMaxReplay(t, 5*time.Millisecond, 2)
	projectID := createTaskTestProject(t, router, adminSession, "SSE Bounded Replay Project")
	seedSSETask(t, db, adminSession.tenantID, projectID, adminSession.userID, "task-bounded-replay")
	first := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-bounded-replay", task.EventTaskQueued)
	second := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-bounded-replay", "TASK_PROGRESS")
	third := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-bounded-replay", task.EventTaskCompleted)

	response, cancel := openTaskSSE(t, server, "/api/v1/events/tasks?lastEventId=evt_00000000000000000000", adminSession.cookies, nil)
	defer closeSSE(response, cancel)
	reader := bufio.NewReader(response.Body)

	assertSSETaskEventFrame(t, readSSEFrame(t, reader), first)
	assertSSETaskEventFrame(t, readSSEFrame(t, reader), second)
	assertSSETaskEventFrame(t, readSSEFrame(t, reader), third)
}

func TestTaskSSERejectsMalformedEventIDWithSanitizedValidationError(t *testing.T) {
	router, _, _, _, adminSession := newSSERouteTestServer(t, 200*time.Millisecond)

	response := performJSON(router, http.MethodGet, "/api/v1/events/tasks?lastEventId=not-an-event-id", nil, adminSession.cookies, nil)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("malformed cursor status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}
	if strings.Contains(response.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("malformed cursor opened stream: content-type=%q", response.Header().Get("Content-Type"))
	}
	body := strings.ToLower(response.Body.String())
	if !strings.Contains(body, "invalid request") || strings.Contains(body, "not-an-event-id") {
		t.Fatalf("malformed cursor response not sanitized: %s", response.Body.String())
	}
}

func TestTaskSSERejectsExplicitCrossTenantTaskFilterBeforeOpeningStream(t *testing.T) {
	router, _, db, _, adminSession := newSSERouteTestServer(t, 200*time.Millisecond)
	seedOtherTenantTask(t, db)

	response := performJSON(router, http.MethodGet, "/api/v1/events/tasks?taskId=task-tenant-b", nil, adminSession.cookies, nil)
	if response.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant task filter status = %d, want %d: %s", response.Code, http.StatusNotFound, response.Body.String())
	}
	if strings.Contains(response.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("cross-tenant task filter opened stream: content-type=%q", response.Header().Get("Content-Type"))
	}
	for _, forbidden := range []string{"tenant-b", "task-tenant-b", "project-tenant-b"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("cross-tenant task filter leaked %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestTaskSSEHeartbeatFrameDoesNotLeakTaskMetadata(t *testing.T) {
	_, server, _, _, adminSession := newSSERouteTestServer(t, 5*time.Millisecond)

	response, cancel := openTaskSSE(t, server, "/api/v1/events/tasks", adminSession.cookies, nil)
	defer closeSSE(response, cancel)

	frame := readSSEFrame(t, bufio.NewReader(response.Body))
	if !strings.Contains(frame, "event: HEARTBEAT\n") || !strings.Contains(frame, "data: {}\n") {
		t.Fatalf("heartbeat frame = %q, want HEARTBEAT empty data", frame)
	}
	for _, forbidden := range []string{"taskId", "projectId", "tenantId", "prompt"} {
		if strings.Contains(frame, forbidden) {
			t.Fatalf("heartbeat leaked %s: %q", forbidden, frame)
		}
	}
}

func TestTaskSSEProjectIDFilterSendsOnlyThatVisibleProject(t *testing.T) {
	router, server, db, _, adminSession := newSSERouteTestServer(t, 20*time.Millisecond)
	projectA := createTaskTestProject(t, router, adminSession, "SSE Project A")
	projectB := createTaskTestProject(t, router, adminSession, "SSE Project B")
	seedSSETask(t, db, adminSession.tenantID, projectA, adminSession.userID, "task-project-a")
	seedSSETask(t, db, adminSession.tenantID, projectB, adminSession.userID, "task-project-b")
	eventA := seedSSEEvent(t, db, adminSession.tenantID, projectA, "task-project-a", task.EventTaskQueued)
	eventB := seedSSEEvent(t, db, adminSession.tenantID, projectB, "task-project-b", task.EventTaskQueued)

	response, cancel := openTaskSSE(t, server, "/api/v1/events/tasks?projectId="+projectA, adminSession.cookies, nil)
	defer closeSSE(response, cancel)
	reader := bufio.NewReader(response.Body)

	frame := readSSEFrame(t, reader)
	assertSSETaskEventFrame(t, frame, eventA)
	next := readSSEFrame(t, reader)
	if strings.Contains(next, eventB.ID) || strings.Contains(next, projectB) {
		t.Fatalf("project filter leaked project B event: %q", next)
	}
	if !strings.Contains(next, "event: HEARTBEAT\n") {
		t.Fatalf("second frame = %q, want heartbeat after filtered replay", next)
	}
}

func TestTaskSSETaskIDFilterSendsOnlyThatVisibleTask(t *testing.T) {
	router, server, db, _, adminSession := newSSERouteTestServer(t, 20*time.Millisecond)
	projectID := createTaskTestProject(t, router, adminSession, "SSE Task Filter Project")
	seedSSETask(t, db, adminSession.tenantID, projectID, adminSession.userID, "task-filter-a")
	seedSSETask(t, db, adminSession.tenantID, projectID, adminSession.userID, "task-filter-b")
	eventA := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-filter-a", task.EventTaskQueued)
	eventB := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-filter-b", task.EventTaskQueued)

	response, cancel := openTaskSSE(t, server, "/api/v1/events/tasks?taskId=task-filter-a", adminSession.cookies, nil)
	defer closeSSE(response, cancel)
	reader := bufio.NewReader(response.Body)

	frame := readSSEFrame(t, reader)
	assertSSETaskEventFrame(t, frame, eventA)
	next := readSSEFrame(t, reader)
	if strings.Contains(next, eventB.ID) || strings.Contains(next, "task-filter-b") {
		t.Fatalf("task filter leaked task B event: %q", next)
	}
	if !strings.Contains(next, "event: HEARTBEAT\n") {
		t.Fatalf("second frame = %q, want heartbeat after filtered replay", next)
	}
}

func TestTaskSSESkipsCrossTenantAndNonMemberProjectEvents(t *testing.T) {
	router, server, db, _, adminSession := newSSERouteTestServer(t, 20*time.Millisecond)
	visibleProject := createTaskTestProject(t, router, adminSession, "SSE Visible Project")
	hiddenProject := createTaskTestProject(t, router, adminSession, "SSE Hidden Project")

	seedActiveUser(t, db, adminSession.tenantID, "seller-sse", "seller-sse@example.com", "Seller SSE", "seller-sse-password-123")
	assignRole(t, db, adminSession.tenantID, "seller-sse", "seller")
	addMember := performJSON(router, http.MethodPost, "/api/v1/projects/"+visibleProject+"/members", map[string]string{
		"userId": "seller-sse",
		"role":   project.RoleViewer,
	}, adminSession.cookies, adminSession.csrfHeader())
	if addMember.Code != http.StatusCreated {
		t.Fatalf("add seller SSE member status = %d: %s", addMember.Code, addMember.Body.String())
	}
	sellerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "seller-sse@example.com", "seller-sse-password-123")

	seedSSETask(t, db, adminSession.tenantID, visibleProject, adminSession.userID, "task-visible-member")
	seedSSETask(t, db, adminSession.tenantID, hiddenProject, adminSession.userID, "task-hidden-nonmember")
	visibleEvent := seedSSEEvent(t, db, adminSession.tenantID, visibleProject, "task-visible-member", task.EventTaskQueued)
	hiddenEvent := seedSSEEvent(t, db, adminSession.tenantID, hiddenProject, "task-hidden-nonmember", task.EventTaskQueued)
	crossTenantEvent := seedSSECrossTenantEvent(t, db)

	response, cancel := openTaskSSE(t, server, "/api/v1/events/tasks", sellerSession.cookies, nil)
	defer closeSSE(response, cancel)
	reader := bufio.NewReader(response.Body)

	frame := readSSEFrame(t, reader)
	assertSSETaskEventFrame(t, frame, visibleEvent)
	next := readSSEFrame(t, reader)
	for _, forbidden := range []string{hiddenEvent.ID, hiddenProject, "task-hidden-nonmember", crossTenantEvent.ID, "tenant-b", "task-tenant-b"} {
		if strings.Contains(next, forbidden) {
			t.Fatalf("invisible event leaked marker %q: %q", forbidden, next)
		}
	}
	if !strings.Contains(next, "event: HEARTBEAT\n") {
		t.Fatalf("second frame = %q, want heartbeat after visible replay", next)
	}
}

func TestTaskSSELiveFanoutUsesMySQLReplaySource(t *testing.T) {
	router, server, db, broker, adminSession := newSSERouteTestServer(t, 100*time.Millisecond)
	projectID := createTaskTestProject(t, router, adminSession, "SSE Live Project")
	seedSSETask(t, db, adminSession.tenantID, projectID, adminSession.userID, "task-live")

	response, cancel := openTaskSSE(t, server, "/api/v1/events/tasks", adminSession.cookies, nil)
	defer closeSSE(response, cancel)
	reader := bufio.NewReader(response.Body)
	waitForSubscribers(t, broker, 1)

	broker.PublishTaskEvent(context.Background(), database.TaskEvent{
		Sequence:         999,
		ID:               "evt_00000000000000000999",
		TenantID:         adminSession.tenantID,
		TaskID:           "task-live",
		ProjectID:        projectID,
		EventType:        "TASK_PROGRESS",
		EventPayloadJSON: `{"taskId":"task-live","projectId":"` + projectID + `"}`,
	})
	first := readSSEFrame(t, reader)
	if strings.Contains(first, "evt_00000000000000000999") || strings.Contains(first, "TASK_PROGRESS") {
		t.Fatalf("stream sent broker-only event without MySQL replay source: %q", first)
	}
	if !strings.Contains(first, "event: HEARTBEAT\n") {
		t.Fatalf("first broker-only wake frame = %q, want heartbeat/no event", first)
	}

	event := seedSSEEvent(t, db, adminSession.tenantID, projectID, "task-live", "TASK_PROGRESS")
	broker.PublishTaskEvent(context.Background(), database.TaskEvent{Sequence: event.Sequence})
	frame := readSSEFrame(t, reader)
	assertSSETaskEventFrame(t, frame, event)
}

func TestTaskSSEDisconnectCleansSubscription(t *testing.T) {
	_, server, _, broker, adminSession := newSSERouteTestServer(t, 50*time.Millisecond)

	response, cancel := openTaskSSE(t, server, "/api/v1/events/tasks", adminSession.cookies, nil)
	waitForSubscribers(t, broker, 1)
	closeSSE(response, cancel)
	waitForSubscribers(t, broker, 0)
}

func newSSERouteTestServer(t *testing.T, heartbeat time.Duration) (http.Handler, *httptest.Server, *gorm.DB, *sse.Broker, projectRouteSession) {
	return newSSERouteTestServerWithMaxReplay(t, heartbeat, 0)
}

func newSSERouteTestServerWithMaxReplay(t *testing.T, heartbeat time.Duration, maxReplayEvents int) (http.Handler, *httptest.Server, *gorm.DB, *sse.Broker, projectRouteSession) {
	t.Helper()

	db := newAuthRouteTestDB(t)
	broker := sse.NewBroker(16)
	router := NewRouter(RouterOptions{
		Config:             authRouteTestConfig("test"),
		Logger:             discardLogger(),
		Database:           db,
		TaskEnqueuer:       &fakeTaskEnqueuer{},
		SSEBroker:          broker,
		SSEHeartbeat:       heartbeat,
		SSEMaxReplayEvents: maxReplayEvents,
	})

	initResponse := performJSON(router, http.MethodPost, "/api/v1/auth/init-admin", map[string]string{
		"tenantName":  "Studio Tenant",
		"email":       "admin@example.com",
		"displayName": "Admin User",
		"password":    "initial-password-123",
	}, nil, nil)
	if initResponse.Code != http.StatusCreated {
		t.Fatalf("init admin status = %d, want %d: %s", initResponse.Code, http.StatusCreated, initResponse.Body.String())
	}
	data := decodeData(t, initResponse)
	authCookie := findCookie(t, initResponse, "studio_auth")
	csrfCookie := findCookie(t, initResponse, "studio_csrf")
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return router, server, db, broker, projectRouteSession{
		tenantID: nestedString(t, data, "tenant", "id"),
		userID:   nestedString(t, data, "user", "id"),
		cookies:  []*http.Cookie{authCookie, csrfCookie},
		csrf:     csrfCookie.Value,
	}
}

func openTaskSSE(t *testing.T, server *httptest.Server, path string, cookies []*http.Cookie, headers map[string]string) (*http.Response, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+path, nil)
	if err != nil {
		cancel()
		t.Fatalf("build SSE request: %v", err)
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := server.Client().Do(request)
	if err != nil {
		cancel()
		t.Fatalf("open SSE stream: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		cancel()
		t.Fatalf("SSE status = %d, want 200: %s", response.StatusCode, string(body))
	}
	if contentType := response.Header.Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		_ = response.Body.Close()
		cancel()
		t.Fatalf("SSE Content-Type = %q, want text/event-stream", contentType)
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-cache" {
		_ = response.Body.Close()
		cancel()
		t.Fatalf("SSE Cache-Control = %q, want no-cache", cacheControl)
	}
	if buffering := response.Header.Get("X-Accel-Buffering"); buffering != "no" {
		_ = response.Body.Close()
		cancel()
		t.Fatalf("SSE X-Accel-Buffering = %q, want no", buffering)
	}
	return response, cancel
}

func closeSSE(response *http.Response, cancel context.CancelFunc) {
	cancel()
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
}

func readSSEFrame(t *testing.T, reader *bufio.Reader) string {
	t.Helper()

	type result struct {
		frame string
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		var builder strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				ch <- result{err: err}
				return
			}
			if line == "\n" || line == "\r\n" {
				ch <- result{frame: builder.String()}
				return
			}
			builder.WriteString(line)
		}
	}()

	select {
	case result := <-ch:
		if result.err != nil {
			t.Fatalf("read SSE frame: %v", result.err)
		}
		return result.frame
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE frame")
		return ""
	}
}

func readSSEUntilClosed(t *testing.T, reader *bufio.Reader) {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		for {
			_, err := reader.ReadString('\n')
			if err != nil {
				done <- err
				return
			}
		}
	}()

	select {
	case err := <-done:
		if err != io.EOF && !strings.Contains(strings.ToLower(err.Error()), "closed") {
			t.Fatalf("read SSE close error = %v, want EOF/closed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for revoked SSE stream to close")
	}
}

func assertSSETaskEventFrame(t *testing.T, frame string, event database.TaskEvent) {
	t.Helper()
	for _, expected := range []string{
		"id: " + event.ID + "\n",
		"event: " + event.EventType + "\n",
		"data: " + event.EventPayloadJSON + "\n",
	} {
		if !strings.Contains(frame, expected) {
			t.Fatalf("frame %q missing %q for event %#v", frame, expected, event)
		}
	}
}

func waitForSubscribers(t *testing.T, broker *sse.Broker, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if broker.SubscriberCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("subscriber count = %d, want %d", broker.SubscriberCount(), want)
}

func seedSSETask(t *testing.T, db *gorm.DB, tenantID string, projectID string, userID string, taskID string) {
	t.Helper()
	now := time.Now().UTC()
	record := database.GenerationTask{
		ID:                taskID,
		TenantID:          tenantID,
		ProjectID:         projectID,
		Type:              task.TypeImageGeneration,
		ProviderID:        "provider-sse",
		ModelID:           "model-sse",
		Status:            task.StatusQueued,
		Prompt:            "SSE test prompt",
		ParamsJSON:        `{}`,
		InputAssetIDsJSON: `[]`,
		Attempt:           1,
		MaxAttempts:       3,
		CreatedBy:         userID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed SSE task %s: %v", taskID, err)
	}
}

func seedSSEEvent(t *testing.T, db *gorm.DB, tenantID string, projectID string, taskID string, eventType string) database.TaskEvent {
	t.Helper()
	scope, err := tenant.NewScope(tenantID)
	if err != nil {
		t.Fatalf("tenant scope: %v", err)
	}
	event := database.TaskEvent{
		TenantID:         tenantID,
		TaskID:           taskID,
		ProjectID:        projectID,
		EventType:        eventType,
		EventPayloadJSON: `{"taskId":"` + taskID + `","projectId":"` + projectID + `"}`,
		CreatedAt:        time.Now().UTC(),
	}
	if err := task.NewRepository(db).CreateEvent(context.Background(), scope, &event); err != nil {
		t.Fatalf("seed SSE event %s/%s: %v", taskID, eventType, err)
	}
	return event
}

func seedSSECrossTenantEvent(t *testing.T, db *gorm.DB) database.TaskEvent {
	t.Helper()
	seedOtherTenantTask(t, db)
	scope, err := tenant.NewScope("tenant-b")
	if err != nil {
		t.Fatalf("tenant B scope: %v", err)
	}
	event := database.TaskEvent{
		TenantID:         "tenant-b",
		TaskID:           "task-tenant-b",
		ProjectID:        "project-tenant-b",
		EventType:        task.EventTaskQueued,
		EventPayloadJSON: `{"taskId":"task-tenant-b","projectId":"project-tenant-b"}`,
		CreatedAt:        time.Now().UTC(),
	}
	if err := task.NewRepository(db).CreateEvent(context.Background(), scope, &event); err != nil {
		t.Fatalf("seed tenant B SSE event: %v", err)
	}
	if event.TenantID != "tenant-b" {
		t.Fatalf("cross tenant event tenant = %q, want tenant-b", event.TenantID)
	}
	return event
}
