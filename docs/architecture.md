# Architecture

## Original frontend baseline

The repository started as a pure frontend local app:

- React + TypeScript + Vite.
- Tailwind CSS.
- Dexie / IndexedDB for image blobs, history records, and prompt templates.
- localStorage for settings and Provider API keys.
- Frontend Provider Adapters for OpenAI, Gemini/Nano Banana, and OpenAI-compatible relay calls.
- Static Nginx Docker deployment.

This baseline was preserved during the early platformization phases so existing UI concepts could be migrated instead of rewritten.

## Current Platform State During P20

The repository is structurally split into `frontend/`, `backend/`, `deploy/`, and `docs/`, and has completed P18 production dry-run plus the merged P19/P20 operational hardening slices. The platform now has backend/frontend infrastructure, authentication/RBAC, tenant user and role administration, project management, project member management with last-`OWNER` protection, MinIO-backed reference/generated/edited assets, backend-generated authorized thumbnails for new assets, seller project/asset workflow polish, Provider/model management, task APIs, SSE delivery, reliable Redis queueing, Worker processing, real backend Provider Adapter runtime, unified backend project history, usage/audit reads, runtime-backed upload-policy/task-default/task-concurrency/storage-retention/storage-quota/log-retention settings, conservative orphan cleanup, strict quota reservations, admin diagnostics, release validation, security regression, deployment runbook, optional real Provider smoke tooling, host TLS proxy template checks, frontend dependency audit gates, existing-tenant built-in-role reconciliation, fixed CSRF header contract, and Docker Compose release checks.

Important current facts:

- The production frontend workbench now uses backend model capabilities, backend task creation, SSE task state, authorized backend assets, and backend unified project history.
- Browser Provider adapters, frontend Provider registry/types, and normal local Provider API Key/API URL settings have been removed.
- IndexedDB is no longer the production source for generated images or history. It may still support non-sensitive prompt templates and explicitly non-production tests/helpers only.
- The backend currently has configuration, logging, router, health, response helpers, middleware, explicit MySQL/GORM migrations under `backend/internal/database`, auth, RBAC, user/role admin APIs, project APIs, project member APIs, asset APIs, MinIO storage abstraction, upload validation, authorized downloads, Provider APIs, model APIs, API key encryption, SSRF-validated Provider testing, task APIs, SSE replay, reliable Redis queueing, Worker state transitions, backend Provider Adapter runtime, MinIO output assets, usage records, API call logs, operation logs, audit/usage read APIs, runtime-backed upload-policy/task-default/task-concurrency/storage-retention/storage-quota/log-retention settings, production secret guards, Worker process concurrency, API Redis subscriber lifecycle ownership, conservative orphan cleanup, strict quota reservations, tenant-scoped diagnostics, Provider/model/default-setting write serialization, security regression entry points, deployment validation scripts, and optional real Provider smoke tooling.
- The frontend has an API client, task/SSE client contracts, auth integration, user/role admin UI, project selection/creation/editing, project member entry points, project asset upload/list/filter/favorite/delete/download/detail/metadata-edit UI, project-scoped reference selection by backend `assetId`, admin Provider/model management UI, admin usage/audit/settings UI, backend task-backed workbench submission, backend result rendering, and backend unified-history/detail/download/re-edit flows.
- The frontend consumes the backend-owned project history query at `GET /api/v1/projects/{projectId}/history`; it must not rebuild the production history feed by joining task and generated/edited asset lists in the browser.
- The backend now exposes tenant `taskDefaults` and consumes them only when task creation omits both Provider/model IDs. Malformed persisted defaults fail closed for default-backed requests without task creation side effects.
- The backend now exposes tenant `taskConcurrency` only with a Worker runtime consumer. Tenant values can narrow environment hard caps, global concurrency remains deployment-owned, and malformed persisted concurrency settings fail closed before Provider execution.
- The backend has an internal asset cleanup foundation for upload rollback and physical purge of soft-deleted objects. Worker maintenance now consumes nullable tenant `storageRetention.deletedAssetRetentionDays` and `logRetention` settings; unset/null/malformed settings fail closed and do not delete anything.
- Docker Compose has buildable runtime foundations, P15 release validation, P16 cleanup traps, P18 live dry-run cleanup evidence, a deployment runbook, an external TLS reverse-proxy template/static checker, and optional real Provider smoke tooling. Routine development still uses the shared local MySQL/Redis/MinIO services documented in `docs/local-development.md`.

Remaining follow-ups are documented in `docs/development-plan.md` and `docs/security.md`. P20 must finish Provider master-key rotation, operator tenant provisioning, tenant/custom-role operations, backup/restore rehearsal, and final stable-production Go/No-Go review. Writable settings still cannot be exposed before their runtime consumers exist.

## Target platform architecture

The target platform is a multi-user, multi-tenant image generation platform:

- Frontend: React + TypeScript + Vite + Tailwind CSS.
- Backend API: Go + Gin + GORM.
- Backend Worker: Go worker process.
- Database: MySQL 8.
- Queue/cache/locks: Redis.
- Object storage: MinIO.
- Deployment: Docker Compose.

## Target repository layout

```text
gpt-image/
  frontend/
    src/
    public/
    package.json
    vite.config.ts
    Dockerfile
    nginx.conf
  backend/
    cmd/
      api/
      worker/
    internal/
      auth/
      tenant/
      rbac/
      project/
      asset/
      task/
      sse/
      queue/
      provider/
      provideradapter/
      model/
      audit/
      settings/
      storage/
      database/
  deploy/
    docker-compose.yml
    mysql/
    minio/
    nginx/
  docs/
  agent-instructions/
  AGENTS.md
  .env.example
  README.md
```

This layout is active. Explicit migrations currently live in `backend/internal/database/migrations.go`.

## Service boundaries

The frontend owns presentation, user interactions, local draft state, API client calls, and SSE consumption. It does not own Provider credentials, Provider calls, task execution, or durable business data.

The API service owns authentication, tenant context, RBAC, business APIs, upload validation, task creation, Provider/model management, SSE delivery, audit logging, and usage queries.

The worker service owns queue consumption, concurrency control, Provider Adapter execution, output upload, usage extraction, task status transitions, and task event persistence.

MySQL is the final source of truth. Redis is only for queueing, locks, cache, rate limits, concurrency semaphores, and temporary acceleration. MinIO stores image bytes.

## Main data flow

1. User logs in through `/api/v1/auth/login`.
2. Backend sets an HttpOnly Cookie and returns current user metadata.
3. Frontend loads projects, assets, available Providers, and enabled model capabilities.
4. User submits a generation or edit request.
5. API validates permissions and input, creates a MySQL task, writes an initial task event, and enqueues a Redis job.
6. Frontend opens or reuses the SSE task stream.
7. Worker claims the job, writes task events, calls a Provider Adapter, uploads outputs to MinIO, records usage and API call logs, and marks the task terminal.
8. SSE pushes queued, running, output, usage, completed, failed, cancelled, retry, and heartbeat events to the frontend.

## Compatibility strategy

- Existing UI components are retained and adapted to backend APIs.
- Existing prompt, upload, parameter, result, and history UI concepts remain.
- Browser Provider code has been removed from the production import graph and must not be reintroduced.
- Existing local history data may be used only through an explicit future import/compatibility feature if one is approved; it is not primary platform data.

## Transition guardrails

- Do not add new browser-side AI Provider calls.
- Do not add new frontend API key storage.
- Do not expand legacy Nginx AI relay behavior.
- Do not make IndexedDB the source of truth for any new platform feature.
- New platform features must be designed around backend API, MySQL, Redis, MinIO, Provider Adapter, and SSE contracts.
