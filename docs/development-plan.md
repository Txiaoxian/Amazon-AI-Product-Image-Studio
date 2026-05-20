# Development Plan

## Local development environment rule

Development and feature verification must use the existing global local environment described in `docs/local-development.md`.

- Use `dev-mysql8`, `dev-redis`, and `dev-minio` for routine validation.
- Do not create project-specific MySQL, Redis, or MinIO containers for normal feature work.
- If project Compose is used for deployment-specific verification, clean it up afterwards with `docker compose -f deploy/docker-compose.yml down -v --remove-orphans` unless the user explicitly asks to keep it.
- Do not copy real local service passwords into project docs, `.env.example`, source code, tests, or logs.

## Status after R7

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
- P5 project and asset management:
  - project/member APIs, MinIO-backed reference assets, upload validation, authorized downloads, and frontend project/asset integration.
- P6 Provider and model management:
  - encrypted Provider credentials, SSRF-safe Provider management/testing, model capability APIs, and frontend admin management UI.
- P7 task, queue, runtime, and SSE foundation:
  - task APIs, SSE replay, Redis queueing, Worker execution, backend Provider Adapter runtime, generated/edited assets, usage/API call logs, and frontend task/SSE contract layer.

Current review result:

- Frontend and backend local test/build commands pass.
- `docker compose -f deploy/docker-compose.yml config` passes.
- P4 through P7 have completed and passed their merge reviews. The next migration step is P8 frontend backendization.
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

- R7 confirmed current task execution uses stable `modelId` references, so `(tenant_id, provider_id, model_name)` uniqueness was not required for P7 runtime. A later management/data-integrity decision may still tighten it.
- Before broader P8/P9 lifecycle UX depends on Provider/model deletion behavior, decide how models should behave when their Provider is soft-deleted: block Provider deletion, hide linked models, or cascade-disable linked models.

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

- P7 real Provider Adapter execution added SSRF-safe outbound transport / connect-time IP validation for DNS rebinding defense.
- R7 confirmed current task execution uses stable `modelId` references, so `(tenant_id, provider_id, model_name)` uniqueness is not required by the current runtime path; a later management/data-integrity decision may still tighten it.
- P8/P9 must decide how linked models behave when their Provider is soft-deleted: block Provider deletion, hide linked models, or cascade-disable linked models.
- Production startup hardening must reject default placeholder secrets, including `JWT_SIGNING_SECRET` and `API_KEY_ENCRYPTION_KEY`, before release.
- Legacy frontend generation still uses browser Provider adapters, localStorage Provider API keys, and IndexedDB history as migration baseline. These remain P8 removal targets, not acceptable platform behavior.
- `ProviderModelAdminPanel` is large and should be split during later frontend maintenance, but this is not a security or merge blocker.

## P7: Task queue, worker, Provider Adapter, and SSE

Status: completed. `P7-BE-TASK-FOUNDATION`, `P7-BE-SSE-STREAM`, `P7-BE-WORKER-QUEUE`, `P7-BE-PROVIDER-ADAPTER-RUNTIME`, `P7-FE-TASK-CLIENT-SSE`, and `R7` have been reviewed, merged into `main`, and verified. P7 intentionally stops before replacing the main frontend workbench flow; P8 owns that migration.

P7 execution order:

1. `P7-BE-TASK-FOUNDATION`: completed and merged. Added MySQL task/event/output/log/usage models and migrations, task repository/service, task create/list/detail/cancel/retry APIs, task event writer, Redis enqueue abstraction, and stable `task_events.sequence` replay cursor. No real Provider call yet.
2. `P7-BE-SSE-STREAM`: completed and merged. Implemented SSE endpoint with heartbeat, `Last-Event-ID`, `lastEventId` query fallback, MySQL replay by `task_events.sequence`, tenant/project/task authorization filtering, in-process fanout wakeups, and tests.
3. `P7-BE-WORKER-QUEUE`: completed and merged. Added Redis reliable queue, Worker claim loop, task state machine, idempotency, cancellation, retry, timeout, recovery, concurrency limits, Redis wakeups for cross-process SSE delivery, and fake/stub Provider execution.
4. `P7-BE-PROVIDER-ADAPTER-RUNTIME`: completed and merged. Added real backend Provider Adapter execution for OpenAI, Gemini, and OpenAI-compatible Providers; SSRF-safe outbound transport; output upload to MinIO; asset creation; `api_call_logs`; `usage_records`; sanitized Provider errors.
5. `P7-FE-TASK-CLIENT-SSE`: completed and merged. Added frontend task API wrappers, task/SSE types, SSE client tests, and task event reducer utilities without replacing the main workbench generation flow.
6. `R7`: completed. Main-agent review, integration regression, security review, and public contract cleanup passed before P8.

P7 serial/parallel policy:

- Start serially with `P7-BE-TASK-FOUNDATION`; do not parallelize until task schema, task event schema, status names, and API response contracts are merged.
- `P7-BE-PROVIDER-ADAPTER-RUNTIME` merged after review and security fixes.
- `P7-FE-TASK-CLIENT-SSE` merged as a contract-layer task only and did not start P8 workbench replacement early.

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
- Real Provider execution, output asset creation, usage records, and API call logs are implemented by `P7-BE-PROVIDER-ADAPTER-RUNTIME`.

P7 Provider Adapter runtime result:

- `P7-BE-PROVIDER-ADAPTER-RUNTIME` merged into `main` after review and two security fixes.
- Real backend generation/edit execution now goes through Provider Adapter implementations for OpenAI, Gemini, and OpenAI-compatible Providers.
- Runtime execution uses model capability rows as the trusted allowlist, validates Provider URLs before use, and uses connect-time SSRF-safe outbound transport before dialing.
- Successful execution uploads generated/edited images to MinIO, creates image assets and task outputs, records usage and API call logs, and emits `IMAGE_OUTPUT`, `USAGE_RECORDED`, and terminal task events.
- Provider errors and runtime metadata are recursively sanitized before persistence. Review fixes closed both “API Key appears as a value” and “API Key appears as a JSON map key” leakage paths for the currently decrypted Provider secret.
- Residual runtime boundary: unknown secrets that are neither supplied to the redactor as known secrets nor matched by heuristics cannot be detected automatically. Current Provider API keys are passed as known secrets, so the active runtime path is covered.

P7 frontend task client result:

- `P7-FE-TASK-CLIENT-SSE` merged into `main` after review.
- Frontend task API wrappers now cover create/list/detail/cancel/retry with the existing authenticated client and in-memory CSRF flow.
- Frontend task/SSE types use canonical `SUCCEEDED` status, typed event payloads, EventSource with `lastEventId` fallback, heartbeat handling, and a pure task event reducer suitable for P8 integration.
- The change did not replace the current workbench generation path, add task polling, add Provider direct calls, or add Provider key persistence.

P7 R7 review result:

- Full frontend validation passed: lint, type-check, 18 test files / 63 tests, and production build.
- Full backend validation passed: `go test ./...`, `go test -race ./...`, `go vet ./...`, and API/worker builds.
- Focused uncached integration tests passed for `internal/api`, `internal/task`, `internal/provider`, `internal/provideradapter`, and `internal/sse`.
- Docker Compose config validation passed, and shared `dev-mysql8`, `dev-redis`, and `dev-minio` services were confirmed healthy/reachable for local verification.
- Static scans found only the already-documented P8 migration leftovers: legacy browser Provider adapters and local settings persistence. P7 added no new direct Provider fetch, Provider key persistence, or polling path.

P7 residual risks and carry-forward items:

- P8 must remove or isolate the legacy browser Provider adapters, localStorage Provider API key persistence, and IndexedDB-backed local history from production generation paths.
- Worker still runs a single processing loop instead of honoring `WORKER_CONCURRENCY` as a worker pool.
- API Redis event subscription lifecycle still uses a background context and should later be tied to API server shutdown.
- Unknown secrets that are neither supplied to the redactor nor matched by heuristics remain outside automatic detection; configured Provider API keys are covered in the active runtime path.
- Current task execution uses stable `modelId` references, so duplicate `(tenant_id, provider_id, model_name)` values did not block P7 runtime. A later admin/data-integrity decision is still needed if stricter model-name uniqueness is desired.
- Provider soft-delete behavior for linked models remains unresolved and should be decided before broader P8/P9 lifecycle UX depends on it.

## P8: Frontend backendization

Status: completed after R8 regression/review. P8 switched the existing frontend workbench from the local/browser execution path to the backend platform path while preserving the current workbench concepts and making backend task APIs, SSE, project assets, and model capabilities the production source of truth.

P8 goals:

- Replace browser generation/edit submission with backend task creation plus SSE state updates.
- Drive selectable models and parameter controls from enabled backend model capability rows instead of `frontend/src/providers/registry.ts`.
- Use project asset IDs as task inputs and backend-generated assets as outputs; do not use image blobs in IndexedDB as platform truth.
- Replace local history as the primary workbench history source with project task history plus generated/edited assets.
- Remove Provider API key inputs and Provider API URL fields from the normal settings flow; Provider credentials remain admin-only backend-managed data.
- Remove or strictly isolate browser Provider adapters from production imports and stop persisting Provider keys in localStorage.
- Keep IndexedDB only for non-sensitive drafts, prompt templates, temporary previews, or an explicit future compatibility/import flow. Do not silently upload old local blobs into tenant storage.

P8 workbench decisions:

- The workbench loads enabled backend models by capability and treats the selected backend `modelId` plus its `providerId` as the submission source of truth.
- A model is selectable only when it is enabled and has usable Provider metadata. If a model/provider becomes unavailable after selection, task creation remains the final server-side guard; the frontend must surface the validation failure, refresh capabilities, and require reselection.
- P8 does not require `POST /assets/{assetId}/edit-source`. Generation/edit tasks should submit `referenceAssetIds` and `editSourceAssetId` directly through task creation.
- Task progress uses EventSource/SSE only. Project-level and task-level UIs may compose reducers by `taskId`, but polling remains forbidden.
- Local UI preferences may remain local if they contain no credentials. Provider API keys and Provider API URLs must not remain in persisted browser settings.
- R7 confirmed current runtime uses stable `modelId` references, so model-name uniqueness is not a P8 blocker. Provider soft-delete linked-model policy remains a P9/admin-hardening decision; P8 must handle stale model availability safely in the UI.

P8 execution order:

1. `P8-FE-WORKBENCH-FOUNDATION`: completed and merged. It added backend model capability loading, backend-ready workbench input types, and project asset ID reference state while keeping the default production submit path legacy-safe until the next task.
2. `P8-FE-TASK-WORKBENCH`: completed and merged. It switched the default workbench submission path to backend task creation + SSE lifecycle, added cancel/retry handling, and drives live result rendering from authorized backend output assets.
3. `P8-FE-HISTORY-ASSET-SOURCE`: completed and merged. It moved the default history/detail/download/re-edit path to backend task history plus generated/edited assets, while keeping old IndexedDB history as an explicit collapsed compatibility entry.
4. `P8-FE-LEGACY-RETIREMENT`: completed and merged. It removed browser Provider adapters, retired normal Provider key/API URL settings, removed `legacyFile` asset-reference payloads, and proved the production import graph no longer reaches old Provider or IndexedDB history/image modules.
5. `R8`: completed. Main-agent review, regression, migration verification, and public contract cleanup passed before P9.

P8 current result:

- Backend model capabilities, backend task input, and project asset references are the production workbench state model.
- The default production workbench creates backend tasks only, advances task state from SSE, and renders live backend output assets instead of browser Provider responses.
- Backend task history, authorized asset download, backend result detail, and backend asset re-editing are now the default production history path.
- Browser Provider adapter files and frontend Provider registry/types have been removed. Normal settings no longer accept or persist Provider API Key or Provider API URL.
- Project asset references now submit backend `assetId` values only. The temporary `legacyFile` compatibility payload has been removed, and project switching clears pending references before task creation.
- IndexedDB is no longer the production source for generated images or history. Prompt templates and old local DB helper files may still exist as residual non-production code or test scaffolding, but they must not be reintroduced into the main workbench path or silently uploaded into tenant storage.
- Remaining P9 follow-ups: generic HTTP `422` handling is still broader than a future stale-model-specific error contract; frontend history currently joins separately paged task/asset lists; history load failure can still render an empty-state panel beside the error state; unreachable legacy display/DB helper code should be deleted or quarantined during hardening.

R8 verification result:

- Frontend regression passed: `npm run lint`, `npm run type-check`, `npm run test`, and `npm run build`; 18 test files / 59 tests passed.
- Backend regression passed: `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go build ./cmd/api ./cmd/worker`.
- Docker Compose config validation passed with `docker compose -f deploy/docker-compose.yml config`.
- Sensitive frontend static scan for `localStorage`, `sessionStorage`, `indexedDB`, Provider `Authorization`, direct Provider hosts, `setInterval`, and `setTimeout` returned no production-code hits.
- Provider import static scan found only backend Provider management API paths: `frontend/src/api/providers.ts` and `frontend/src/components/admin/ProviderModelAdminPanel.tsx`. These are allowed backend admin API consumers, not browser AI Provider calls.
- `frontend/src/providers/` no longer exists. A separate residual scan still finds unreachable legacy display/DB helper code (`LegacyHistoryPanel`, legacy result rendering branches, old local history/image helpers, `useStorageUsage`), but these are outside the production workbench import graph and are P9 cleanup candidates, not P8 blockers.

P8 serial policy:

- Execute P8 serially. All four frontend tasks touch the same workbench boundary and would otherwise create avoidable merge and behavior conflicts.
- Do not retire old local code until the backend submission, SSE result path, and backend history path have replaced it.
- Do not start P9 admin hardening as a substitute for P8 migration work.

P8 acceptance gates:

- Browser generation creates backend tasks only; no production workbench import path calls OpenAI, Gemini, or relay URLs.
- Workbench task status comes from SSE only; no `setInterval`, repeated `setTimeout`, or looped fetch status checks.
- Browser storage contains no Provider API key or Provider API URL persistence used by production workbench behavior.
- Generated/edited assets and task history come from backend APIs/authorized downloads; IndexedDB is not the primary history/image source.
- Existing upload, prompt, parameter, result, history, download, and edit concepts remain available in backend-backed form.

P8 intentionally does not resolve:

- Worker pool implementation for `WORKER_CONCURRENCY`.
- API Redis subscription lifecycle binding to server shutdown.
- General redaction of unknown secrets outside known-secret and heuristic rules.
- Provider soft-delete linked-model backend policy or optional model-name uniqueness hardening; these remain P9/admin lifecycle concerns.

## P9: Usage, audit, settings, hardening, and release readiness

Status: completed through deployment release validation. `P9-BE-AUDIT-USAGE-READS`, `P9-BE-PRODUCTION-SECRET-GUARD`, `P9-BE-RUNTIME-SETTINGS-CONTRACT`, `P9-FE-ADMIN-OBSERVABILITY-SETTINGS`, `P9-SECURITY-REGRESSION`, and `P9-DEPLOY-RELEASE-VALIDATION` have been reviewed or locally validated as appropriate. The first attempt to package broad settings hardening exposed a contract bug: writable settings cannot be honest until their runtime consumers are in scope.

P9 must be split into small serial tasks rather than one broad worktree. The first batch should start with backend read contracts before any frontend admin UI:

1. `P9-BE-AUDIT-USAGE-READS`: completed and merged. Backend usage, operation log, and API call log read APIs now enforce admin RBAC, tenant isolation, pagination, deterministic ordering, and shared recursive response redaction.
2. `P9-BE-PRODUCTION-SECRET-GUARD`: completed and merged. API and Worker startup now reject placeholder `JWT_SIGNING_SECRET` and placeholder `API_KEY_ENCRYPTION_KEY` in production while preserving non-production defaults.
3. `P9-BE-RUNTIME-SETTINGS-CONTRACT`: completed and merged. Backend system settings now expose only the first honest runtime-backed slice: tenant upload policy consumed by asset upload validation. Default Provider/model selection, tenant concurrency, storage quota, and log retention remain deferred until their runtime consumers are deliberately in scope.
4. `P9-FE-ADMIN-OBSERVABILITY-SETTINGS`: completed and merged. Frontend admin UI now consumes paginated usage/audit reads and the narrow runtime-backed `uploadPolicy` settings contract without exposing deferred settings.
5. `P9-SECURITY-REGRESSION`: completed and merged. Added targeted security regression tests for SSRF, tenant isolation, object permissions, upload validation, sensitive logging, SSE replay visibility, production secret guards, frontend static regressions, and residual legacy helper deletion.
6. `P9-DEPLOY-RELEASE-VALIDATION`: completed locally. Docker Compose config/build/up/healthcheck passed; frontend, backend API, backend Worker, MySQL, Redis, and MinIO were validated in the Compose topology; release documentation and runbook were updated.

P9 deployment validation result:

- Backend `go test`, race tests, vet, and API/Worker builds passed.
- Frontend lint, type-check, tests, and production build passed after dependency installation with `npm ci`.
- Compose config passed and images built for `backend-api`, `backend-worker`, and `frontend`.
- Compose startup reached healthy state for `mysql`, `redis`, `minio`, `backend-api`, `backend-worker`, and `frontend`.
- `minio-bootstrap` exited successfully and can be rerun idempotently.
- API health checks now report MySQL, Redis, and MinIO bucket readiness.
- Frontend `/api/` proxy reached `backend-api:8080`; SPA fallback and static assets worked.
- Nginx runtime config contains no AI Provider or relay proxy.
- Production placeholder secret guards failed fast for both API and Worker.
- The Compose stack and volumes were removed with `docker compose -f deploy/docker-compose.yml down -v --remove-orphans`.

P9 carry-forward risks:

- Unknown secrets cannot be redacted unless they are supplied as known secrets or match heuristic rules.
- P9 audit reads now use the shared redaction package and support exact known-secret scrubbing through an injected redactor seam, but production read APIs intentionally do not widen Provider API-key decryption scope. If historical dirty rows must be scrubbed for non-heuristic secrets at read time, define a trusted minimal secret source and lifecycle before implementing it.
- P9 system settings currently expose only settings with live runtime consumers. Tenant-scoped upload policy is implemented and consumed by asset validation; `defaultProviderId/defaultModelId`, tenant concurrency, storage quotas, and log retention remain deferred because their task/worker/quota/cleanup consumers are not yet in scope.
- Provider soft-delete linked-model policy remains unresolved.
- History list pagination is currently assembled by the frontend from task and asset lists; a backend history query should be considered if pagination correctness becomes important.
- R8-identified unreachable legacy display/storage helpers were deleted in P9 security regression. Future legacy cleanup should be based on production import-graph evidence, not broad deletion.
- Admin observability UI is intentionally acceptable as one component for this slice, but should be split after security regression if it becomes a maintenance hotspot.
- API call log detail UI has no stale-request guard; a slower detail response can overwrite a newer click for the same admin user. This is a display correctness issue, not a security blocker, and should be considered for a follow-up frontend hardening task.
- Redis 7.4 may emit go-redis `maintnotifications` fallback warnings during API/Worker operation. P9 validation confirmed these warnings are non-blocking, but operators may want to revisit Redis/client compatibility if log noise becomes a concern.

## Phase boundaries

- Do not implement Provider calls before API key encryption, SSRF validation, and logging redaction are in place.
- Do not integrate frontend task status before backend SSE replay behavior exists.
- Do not remove old frontend generation logic until backend task creation, Provider Adapter execution, MinIO asset persistence, and SSE delivery can replace it.
- Do not treat localStorage API keys, frontend Provider calls, IndexedDB image Blob storage, or frontend Nginx AI relay as acceptable platform behavior.
