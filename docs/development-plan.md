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

The project is ready to enter P6 Provider and model management.

## P6: Provider and model management

Tasks:

- Implement Provider CRUD, enable/disable, and Provider test.
- Encrypt Provider API keys at rest.
- Return only masked credential metadata to frontend.
- Implement SSRF-safe Provider `base_url` validation before save and before use.
- Implement model capability management for generation/edit, multi-reference support, `n`, max output count, sizes, quality, output formats, and price config.

## P7: Task queue, worker, Provider Adapter, and SSE

Tasks:

- Implement generation/edit task creation.
- Persist tasks and task events in MySQL.
- Enqueue Redis jobs after task persistence.
- Implement worker claim, execution, cancellation, retry, timeout, and recovery.
- Add backend Provider Adapter interface and concrete OpenAI/Gemini/OpenAI-compatible adapters.
- Store generated outputs in MinIO and create asset records.
- Record API call logs and usage records.
- Implement SSE with heartbeat, `Last-Event-ID`, `lastEventId` fallback, replay from MySQL, and authorization filtering.
- Enforce global, tenant, user, Provider, and model concurrency limits.

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
