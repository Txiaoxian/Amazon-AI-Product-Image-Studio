# Development Plan

## Local development environment rule

Development and feature verification must use the existing global local environment described in `docs/local-development.md`.

- Use `dev-mysql8`, `dev-redis`, and `dev-minio` for routine validation.
- Do not create project-specific MySQL, Redis, or MinIO containers for normal feature work.
- If project Compose is used for deployment-specific verification, clean it up afterwards with `docker compose -f deploy/docker-compose.yml down -v --remove-orphans` unless the user explicitly asks to keep it.
- Do not copy real local service passwords into project docs, `.env.example`, source code, tests, or logs.

## Status after P4

Completed:

- P0 documentation and project Agent rules.
- P1 repository structure foundation:
  - frontend moved into `frontend/`.
  - Go backend skeleton added under `backend/`.
  - Docker Compose skeleton and `.env.example` added.
- P2 backend and frontend infrastructure:
  - backend config, logging, response helpers, router, request ID, security headers, CORS, recovery, and access log middleware.
  - frontend API client and task SSE client foundations.
- P3 runtime deployment repair:
  - backend API and worker Dockerfile targets.
  - frontend Nginx `/api/` proxy.
  - worker readiness healthcheck.
  - removal of frontend Nginx AI relay routing.
- P4 database, tenant, auth, RBAC, and frontend auth:
  - GORM/MySQL connection, explicit migrations, tenant-aware repository helper, and core auth/RBAC tables.
  - init-admin, login, logout, `/me`, password change, HttpOnly Cookie JWT, CSRF, auth middleware, tenant context, RBAC guard, and operation log recording.
  - frontend auth API wrappers, login screen, current-user loading, logout, 401 handling, and in-memory CSRF token usage.

Current review result:

- Frontend and backend local test/build commands pass.
- `docker compose -f deploy/docker-compose.yml config` passes.
- The P4 auth chain is good enough to continue into P5.
- The app is still in transition state. Old local frontend AI calls, localStorage Provider API keys, and IndexedDB image Blob storage remain as migration baselines only and must be removed in P8.
- Non-blocking P4 hardening backlog:
  - Reject default JWT signing secret when `APP_ENV=production`.
  - Align frontend CSRF header usage with configurable backend `CSRF_HEADER_NAME` before allowing non-default deployments.
  - Make audit metadata redaction recursive before broader modules start writing complex metadata.

## P3: Runtime deployment repair

Priority: do this before database, authentication, or business API work.

Tasks:

- Add a backend Dockerfile with `api` and `worker` targets.
- Make backend worker healthcheck real by creating/removing the configured readiness file.
- Remove frontend Nginx AI relay routing.
- Add frontend Nginx `/api/` proxy to `backend-api:8080`.
- Add Vite dev `/api` proxy to the local backend API.
- Keep existing frontend UI and local generation behavior unchanged.

Verification:

```bash
cd frontend && npm run lint && npm run type-check && npm run test && npm run build
cd ../backend && go test ./... && go test -race ./... && go vet ./...
cd .. && docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
```

## P4: Database, tenant, auth, and RBAC foundation

Status: completed and merged to `main`.

Tasks:

- Implement GORM/MySQL connection and explicit migrations.
- Add tenant-aware model and repository helpers.
- Add tenant, user, role, permission, user role, role permission, and operation log tables.
- Implement init-admin, login, logout, current user, and password change APIs.
- Add HttpOnly Cookie JWT, CSRF foundation, auth middleware, tenant context middleware, and RBAC middleware.
- Add frontend login/current user wiring only after backend auth contracts are stable.

Verification:

- Backend unit tests cover migrations, auth happy paths, auth failures, cookie flags, CSRF behavior, and tenant context.
- Frontend auth tests cover login, unauthenticated state, logout, 401 handling, and current user loading.
- Post-merge checks passed:

```bash
cd frontend && npm run lint && npm run type-check && npm run test && npm run build
cd ../backend && go test ./... && go test -race ./... && go vet ./... && go build ./cmd/api ./cmd/worker
cd .. && docker compose -f deploy/docker-compose.yml config
```

## P5: Project and asset management

Status: completed. Backend project, backend asset storage, frontend project/asset integration, and R5 review have all been reviewed and merged into `main`. Do not treat P5 as generation backendization: old frontend AI Provider direct calls, localStorage Provider API keys, and IndexedDB local history remain documented transition risks and must be removed in P8 after P6/P7 backend replacements exist.

Execution order:

1. `P5-BE-PROJECTS`: backend project and project member foundation. Completed and merged.
2. `P5-BE-ASSET-STORAGE`: backend MinIO storage service, upload validation, asset APIs, and authorized downloads. Completed and merged.
3. `P5-FE-PROJECT-ASSETS`: frontend project selector, asset list, reference upload, download, favorite/delete actions, and project-scoped reference selection. Completed and merged.
4. `R5`: main-agent review, integration, regression, and contract cleanup. Completed.

P5 backend project requirements:

- Implement migrations and models for `projects` and `project_members`.
- Implement project CRUD with soft delete.
- Implement project membership APIs and object-level authorization helpers.
- Every project query must filter by `tenant_id`.
- Cross-tenant project IDs must return `404` or a non-revealing authorization failure.

P5 backend project result:

- Implemented project CRUD, project member management, and project authorization helpers.
- Every project query is tenant-scoped.
- Project creators become `OWNER` members.
- Operation logs are written for project and member changes.

P5 backend asset requirements:

- Implement migrations and models for `image_assets`.
- Implement MinIO client/storage service using the existing environment config.
- Implement reference image upload to MinIO.
- Implement asset metadata, thumbnail metadata placeholder, list, detail, favorite/unfavorite, soft delete, and authorized download.
- Validate uploads by magic bytes, MIME allowlist, file size, width, height, and pixel count. SVG is forbidden.
- MySQL stores metadata and `object_key`; image bytes go to MinIO only.
- Use the shared local `dev-minio` environment for routine integration checks.

P5 backend asset result:

- Implemented `image_assets`, MinIO object storage abstraction, upload validation, asset metadata APIs, favorite/unfavorite, soft delete, and authorized download.
- Asset access uses `asset -> project` authorization and reuses project RBAC plus membership checks.
- Reference uploads write image bytes to MinIO and metadata to MySQL only.
- Review note: uploaded-object cleanup after a DB failure is implemented, but should later use an independent cleanup context and/or cleanup job for request-cancellation edge cases.
- Review note: new built-in asset permissions are seeded for new tenants; a later reconciliation task should backfill built-in permissions for existing tenants.
- Review note: bucket creation/bootstrap remains a deployment or local-environment responsibility.

P5 frontend requirements:

- Add project and asset API wrappers using the existing authenticated API client.
- Add project selection/creation entry points without replacing the generation workflow yet.
- Add reference asset upload/list/detail/download UI consistent with the existing workbench.
- Keep old local IndexedDB history available until P8, but do not treat it as backend asset truth.
- Use the merged backend contracts for `/api/v1/projects`, `/api/v1/projects/{projectId}/assets`, and `/api/v1/assets/{assetId}`.
- Do not introduce task polling, Provider/model management, or frontend generation backendization in P5.

P5 frontend result:

- Added authenticated project and asset API wrappers on top of the existing frontend API client.
- Added project selector/create UI, reference upload, asset list, asset detail actions, favorite/delete/download actions, and "use as reference" workbench wiring.
- State-changing project and asset requests use the existing in-memory CSRF flow; no JWT, CSRF token, or Provider credential persistence was added.
- Download goes through backend authorization and is handled as a browser blob response.
- Existing generation submission and local history remain available as transition behavior until P8.

P5 acceptance gates:

- Project and asset APIs require authentication, tenant filtering, RBAC, and object-level authorization.
- Cross-tenant project/asset access is blocked or invisible.
- Upload rejects forged MIME, invalid magic bytes, SVG, oversized dimensions, and excessive pixel count.
- Download requires backend authorization.
- Frontend does not store Provider API keys, auth tokens, or image blobs as backend asset truth.

P5 R5 review result:

- P5 backend project and asset APIs enforce authentication, tenant filters, RBAC, project membership, object-level authorization, and soft delete behavior.
- P5 asset upload tests cover forged MIME, invalid image bytes, SVG rejection, file size, dimensions, and pixel-count validation.
- P5 frontend integration did not add new AI Provider direct calls, API key persistence, auth token persistence, or task polling.
- Shared local development services were used for validation; no project-specific MySQL, Redis, or MinIO environment was created.

P5 non-blocking backlog:

- Frontend project creation currently clears form inputs before the async create result is known; later UX hardening should clear only after success.
- Frontend upload precheck still uses the local 15 MB limit while the backend default allows 25 MB. Until system settings are exposed to the frontend, backend validation remains the source of truth.
- Uploaded-object cleanup after a DB failure is implemented, but should later use an independent cleanup context and/or cleanup job for request-cancellation edge cases.
- New built-in asset permissions are seeded for new tenants; a later reconciliation task should backfill built-in permissions for existing tenants.
- Bucket creation/bootstrap remains a deployment or local-environment responsibility.

P5 full verification after merge:

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ../backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
git diff --check
```

Actual R5 local checks also confirmed the shared local services were running and reachable:

```bash
docker ps --format '{{.Names}}\t{{.Status}}' | rg '^dev-(mysql8|redis|minio)\b'
docker exec dev-redis redis-cli ping
nc -vz 127.0.0.1 3306
nc -vz 127.0.0.1 6379
nc -vz 127.0.0.1 9000
```

P6 Provider/model management has completed and passed R6 integration review. The project is ready to enter P7 task queue, Worker, Provider Adapter execution, and SSE.

## P6: Provider and model management

Status: completed. `P6-BE-PROVIDER-SECURITY`, `P6-BE-MODEL-CAPABILITIES`, `P6-FE-PROVIDER-MODEL-MGMT`, and `R6` have been reviewed, merged into `main`, and verified. P6 added Provider/model management only; it did not implement worker generation/edit execution, task queue processing, SSE task delivery, or frontend generation backendization.

P6 execution order:

1. `P6-BE-PROVIDER-SECURITY`: backend Provider schema, encryption, SSRF validation, Provider CRUD, enable/disable, sanitized Provider test, operation logs, and security tests. Completed and merged.
2. `P6-BE-MODEL-CAPABILITIES`: backend model capability schema, validation, CRUD, enable/disable, pricing metadata, and enabled model capability listing. Completed and merged.
3. `P6-FE-PROVIDER-MODEL-MGMT`: frontend admin UI and API wrappers for Provider/model management. Completed and merged.
4. `R6`: main-agent review, integration regression, security review, and public contract cleanup. Completed.

P6 serial/parallel policy:

- `P6-BE-PROVIDER-SECURITY` was intentionally developed first and merged before frontend Provider UI work.
- `P6-BE-MODEL-CAPABILITIES` was intentionally developed after Provider security and before frontend Provider/model UI.
- Frontend Provider/model management was developed after both backend contracts were stable and merged.

P6 backend Provider requirements:

- Add `ai_providers` model/migration fields needed for tenant-scoped Provider configuration.
- Support Provider types `OPENAI`, `GEMINI`, and `OPENAI_COMPATIBLE`.
- Encrypt API keys at rest using the configured API key encryption secret.
- Return only masked key metadata such as hint and update time; never return full API keys.
- Validate Provider `base_url` before save and before test/use with SSRF defenses.
- Implement CRUD, enable/disable, and sanitized Provider test.
- Write operation logs for create/update/delete/enable/disable/test.
- Ensure logs and errors do not contain API keys, Authorization headers, Cookies, or image base64.

P6 backend Provider result:

- Implemented tenant-scoped `ai_providers` model/migration, repository, service, routes, and tests.
- Implemented Provider CRUD, enable/disable, soft delete, backend-only Provider test, API key encryption, masked responses, recursive audit metadata redaction, and operation logs.
- Implemented SSRF validation for Provider `baseUrl` on create/update and before Provider test, including tests for loopback, private ranges, link-local, multicast, Docker service names, embedded credentials, non-HTTPS schemes, and redirect-to-blocked-target.
- Verified Provider test does not create tasks, assets, usage records, or call frontend Provider code.

P6 Provider non-blocking backlog:

- Before P7 real Provider Adapter execution, add an SSRF-safe HTTP transport or `DialContext` so the actual outbound dial re-checks resolved IPs and avoids DNS rebinding between validation and connection.
- Before production release, reject default placeholder `API_KEY_ENCRYPTION_KEY` at startup alongside other production secret checks.

P6 backend model requirements:

- Add `ai_models` model/migration fields for Provider ownership and capabilities.
- Support generation/edit flags, multi-reference support, `n` support, max output count, supported sizes, qualities, output formats, pricing config, and enable/disable state.
- Validate model capability combinations, for example disabled `supports_n` must not accept output counts above 1.
- Ensure models are tenant-scoped and belong to a Provider in the same tenant.
- Expose enabled model capability responses for the frontend to render dynamic parameters later.

P6 backend model result:

- Implemented tenant-scoped `ai_models` model/migration, repository, service, routes, and tests.
- Implemented model CRUD, enable/disable, soft delete, capability validation, pricing metadata validation, Provider same-tenant checks, RBAC, and operation logs.
- Verified cross-tenant Provider/model access is rejected or invisible, and model responses do not expose Provider credentials or sensitive request data.

P6 model non-blocking backlog:

- Before P7 task execution decides model selection semantics, decide whether `(tenant_id, provider_id, model_name)` should be unique. The current migration indexes this tuple but does not enforce uniqueness.
- Before P7/P8 depends on Provider/model lifecycle behavior, decide how models should behave when their Provider is soft-deleted: block Provider deletion, hide linked models, or cascade-disable linked models.

P6 frontend requirements:

- Add Provider/model API wrappers using the existing authenticated API client.
- Add admin-only Provider/model management screens or panels consistent with the existing UI style.
- Provider forms may accept API keys only for immediate submission to the backend; they must not persist keys in localStorage, sessionStorage, IndexedDB, URL params, or client-visible config.
- Display masked key metadata only.
- Render Provider test results and sanitized errors without leaking credentials.
- Do not modify the legacy generation submit path in P6.

P6 frontend result:

- Implemented Provider/model API wrappers using the authenticated API client with `credentials: include` and CSRF headers for state-changing requests.
- Implemented admin Provider/model management UI for list, create, edit, delete, enable/disable, and Provider test.
- Provider API keys are accepted only as form input for immediate backend submission. The UI shows only masked key metadata, clears submitted and unsubmitted key drafts, and does not persist keys to browser storage.
- Added tests for API wrappers, permission-hidden management entry, Provider key one-time submission, key draft cleanup on modal close, and permission/validation error states.
- Did not modify legacy generation submission, browser Provider adapters, local history, IndexedDB, or P8 migration paths.

P6 acceptance gates:

- Provider and model APIs require authentication, tenant filtering, RBAC, and object-level checks where IDs are used.
- API keys are encrypted in MySQL and are not returned in API responses.
- SSRF tests block localhost, loopback, private ranges, link-local ranges, Docker-internal hostnames, invalid schemes, and redirects to blocked targets.
- Provider test uses backend-only execution with timeout and redacted logs; it must not create generation tasks or output assets.
- Frontend does not save Provider API keys or introduce new AI Provider direct calls.
- P6 does not change task queue, worker generation, SSE task events, or frontend generation backendization.

R6 verification result:

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ../backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
git diff --check
```

Actual R6 result:

- Frontend validation passed: lint, type-check, 16 test files / 58 tests, and production build.
- Backend validation passed: `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go build ./cmd/api ./cmd/worker`.
- Docker Compose config validation passed.
- P6 frontend diff security scan found only Provider type enum/test text references to `OPENAI`, `GEMINI`, and `OPENAI_COMPATIBLE`; no new browser Provider direct fetch, Authorization header, API key persistence, or polling path was introduced.

P6 residual risks and carry-forward items:

- P7 real Provider Adapter execution must add SSRF-safe outbound transport or `DialContext` connect-time IP validation. Save/test URL validation alone is not enough against DNS rebinding.
- P7 must decide whether `(tenant_id, provider_id, model_name)` uniqueness is required before task execution depends on model lookup semantics.
- P7/P8 must decide how linked models behave when their Provider is soft-deleted: block Provider deletion, hide linked models, or cascade-disable linked models.
- Production startup hardening must reject default placeholder secrets, including `JWT_SIGNING_SECRET` and `API_KEY_ENCRYPTION_KEY`, before release.
- Legacy frontend generation still uses browser Provider adapters, localStorage Provider API keys, and IndexedDB history as migration baseline. These remain P8 removal targets, not acceptable platform behavior.
- `ProviderModelAdminPanel` is large and should be split during later frontend maintenance, but this is not a security or merge blocker.

## P7: Task queue, worker, Provider Adapter, and SSE

Status: in progress. `P7-BE-TASK-FOUNDATION`, `P7-BE-SSE-STREAM`, and `P7-BE-WORKER-QUEUE` have been reviewed, merged into `main`, and verified. P7 must continue in ordered slices because task schema, events, queue semantics, Provider execution, and SSE replay depend on shared contracts.

P7 execution order:

1. `P7-BE-TASK-FOUNDATION`: completed and merged. Added MySQL task/event/output/log/usage models and migrations, task repository/service, task create/list/detail/cancel/retry APIs, task event writer, Redis enqueue abstraction, and stable `task_events.sequence` replay cursor. No real Provider call yet.
2. `P7-BE-SSE-STREAM`: completed and merged. Implemented SSE endpoint with heartbeat, `Last-Event-ID`, `lastEventId` query fallback, MySQL replay by `task_events.sequence`, tenant/project/task authorization filtering, in-process fanout wakeups, and tests.
3. `P7-BE-WORKER-QUEUE`: completed and merged. Added Redis reliable queue, Worker claim loop, task state machine, idempotency, cancellation, retry, timeout, recovery, concurrency limits, Redis wakeups for cross-process SSE delivery, and fake/stub Provider execution.
4. `P7-BE-PROVIDER-ADAPTER-RUNTIME`: next task. Add real backend Provider Adapter execution for OpenAI, Gemini, and OpenAI-compatible Providers; SSRF-safe outbound transport; output upload to MinIO; asset creation; `api_call_logs`; `usage_records`; sanitized Provider errors.
5. `P7-FE-TASK-CLIENT-SSE`: frontend task API wrappers, task/SSE types, SSE client integration tests, and task event reducer utilities. This must not replace the main workbench generation flow; P8 owns that migration.
6. `R7`: main-agent review, integration regression, security review, and public contract cleanup before P8.

P7 serial/parallel policy:

- Start serially with `P7-BE-TASK-FOUNDATION`; do not parallelize until task schema, task event schema, status names, and API response contracts are merged.
- `P7-BE-WORKER-QUEUE` has merged. Start `P7-BE-PROVIDER-ADAPTER-RUNTIME` from latest `main`; do not parallelize it with frontend task client work.
- `P7-BE-PROVIDER-ADAPTER-RUNTIME` must use Worker state handling as merged and must add connect-time SSRF protection before real outbound Provider calls.
- `P7-FE-TASK-CLIENT-SSE` starts only after task and SSE contracts are stable enough for frontend types and API wrappers.

P7 canonical status decision:

- Backend/API task statuses are `QUEUED`, `RUNNING`, `SUCCEEDED`, `FAILED`, `CANCELLED`, `RETRYING`, and `TIMED_OUT`.
- SSE event type `TASK_COMPLETED` represents a transition to task status `SUCCEEDED`.
- Existing transitional frontend type `COMPLETED` must be updated in P7/P8 before frontend task status is used in production paths.

P7 task foundation result:

- Task creation persists MySQL task state, writes `TASK_QUEUED`, then enqueues Redis with task ID only.
- Enqueue failure transitions the task to `FAILED` with sanitized `ENQUEUE_FAILED` metadata; it must not return success with an unqueued `QUEUED` task.
- `task_events.sequence` is the durable, monotonic replay cursor. `task_events.id` is derived from that sequence as an SSE-safe string such as `evt_00000000000000000001`.
- SSE replay must compare by `sequence`, not by `created_at` or lexical random IDs.

P7 SSE stream result:

- `GET /api/v1/events/tasks` is implemented under the authenticated `/api/v1` route group.
- `Last-Event-ID` header and `lastEventId` query fallback parse `evt_...` IDs back to `task_events.sequence`.
- Historical replay reads MySQL with `sequence > cursor`, orders by `sequence ASC`, and filters every event by tenant, project visibility, and optional task filter.
- Heartbeat frames are emitted as `HEARTBEAT` with empty JSON payload and no task metadata.
- Live delivery uses an in-process broker inside API and Redis wakeups from Worker/API processes. MySQL remains the only replay source.

P7 Worker queue result:

- `P7-BE-WORKER-QUEUE` merged into `main` after review and fix.
- Redis reliable queue supports enqueue, delayed promotion, claim, ack, stale claim recovery, max-delivery dead-letter handling, and malformed claim recovery tests.
- Worker consumes task IDs only, reloads task state from MySQL, transitions eligible tasks to `RUNNING`, writes task events, and uses fake/stub execution until Provider Adapter runtime is implemented.
- Redis wakeups now notify API SSE streams after persisted task events. Wakeups carry only minimal sequence/task metadata; SSE still reloads visible events from MySQL.
- Concurrency limits are implemented for global, tenant, user, Provider, and model dimensions with stale lock cleanup.
- Non-blocking carry-forward risks: Worker currently runs a single processing loop and does not yet apply `WORKER_CONCURRENCY` as a worker pool; the API Redis event subscriber uses a background context that should be tied to server lifecycle later.
- Real Provider execution, output asset creation, usage records, and API call logs remain owned by `P7-BE-PROVIDER-ADAPTER-RUNTIME`.

## P8: Frontend backendization

Tasks:

- Replace frontend generation flow with task creation API plus SSE events.
- Replace local history as primary data source with project assets and task history APIs.
- Replace local API key settings with backend Provider/model management UI.
- Remove or isolate browser Provider adapters from production call paths.
- Remove localStorage API key persistence.
- Keep IndexedDB only for drafts, temporary previews, or explicit compatibility/import flows.

## P9: Usage, audit, settings, hardening, and release readiness

Tasks:

- Implement usage summaries, API call logs, operation audit views, and system settings.
- Add security regression tests for SSRF, tenant isolation, object-level authorization, upload validation, sensitive logging, and SSE replay visibility.
- Run full Docker Compose deployment verification.
- Update release documentation and deployment instructions.

## Phase boundaries

- Do not implement Provider calls before API key encryption, SSRF validation, and logging redaction are in place.
- Do not integrate frontend task status before backend SSE replay behavior exists.
- Do not remove old frontend generation logic until backend task creation, Provider Adapter execution, MinIO asset persistence, and SSE delivery can replace it.
- Do not treat localStorage API keys, frontend Provider calls, IndexedDB image Blob storage, or frontend Nginx AI relay as acceptable platform behavior.
