# Development Plan

## Status after P2

Completed:

- P0 documentation and project Agent rules.
- P1 repository structure foundation:
  - frontend moved into `frontend/`.
  - Go backend skeleton added under `backend/`.
  - Docker Compose skeleton and `.env.example` added.
- P2 backend and frontend infrastructure:
  - backend config, logging, response helpers, router, request ID, security headers, CORS, recovery, and access log middleware.
  - frontend API client and task SSE client foundations.

Current review result:

- Frontend and backend local test/build commands pass.
- `docker compose -f deploy/docker-compose.yml config` passes.
- `docker compose -f deploy/docker-compose.yml build backend-api` fails because `backend/Dockerfile` is missing.
- The app is still in transition state. Old local frontend AI calls, localStorage API keys, and IndexedDB image Blob storage remain as migration baselines only.

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

Tasks:

- Implement GORM/MySQL connection and explicit migrations.
- Add tenant-aware model and repository helpers.
- Add tenant, user, role, permission, user role, role permission, and operation log tables.
- Implement init-admin, login, logout, current user, and password change APIs.
- Add HttpOnly Cookie JWT, CSRF foundation, auth middleware, tenant context middleware, and RBAC middleware.
- Add frontend login/current user wiring only after backend auth contracts are stable.

Verification:

- Backend unit tests cover migrations, auth happy paths, auth failures, cookie flags, CSRF behavior, and tenant context.
- Frontend auth tests cover login, unauthenticated state, and current user loading when implemented.

## P5: Project and asset management

Tasks:

- Implement project CRUD and project member checks.
- Implement MinIO storage service.
- Implement reference image upload, image asset metadata, thumbnail metadata, favorite, soft delete, detail, and authorized download.
- Validate uploads by magic bytes, MIME allowlist, file size, width, height, and pixel count. SVG is forbidden.
- MySQL stores metadata and `object_key`; image bytes go to MinIO only.

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
