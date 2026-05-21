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

## Current platform state after R10

The repository is structurally split into `frontend/`, `backend/`, `deploy/`, and `docs/`, and has passed R10 review. It now has backend/frontend infrastructure, authentication/RBAC, project management, reference asset management, Provider/model management, task APIs, SSE delivery, reliable Redis queueing, Worker processing, real backend Provider Adapter runtime, usage/audit reads, runtime-backed upload-policy settings, release validation, and P10 runtime hardening.

Important current facts:

- The production frontend workbench now uses backend model capabilities, backend task creation, SSE task state, authorized backend assets, and backend task/asset history.
- Browser Provider adapters, frontend Provider registry/types, and normal local Provider API Key/API URL settings have been removed.
- IndexedDB is no longer the production source for generated images or history. It may still support non-sensitive prompt templates and explicitly non-production tests/helpers only.
- The backend currently has configuration, logging, router, health, response helpers, middleware, explicit MySQL/GORM migrations under `backend/internal/database`, auth, RBAC, project APIs, asset APIs, MinIO storage abstraction, upload validation, authorized downloads, Provider APIs, model APIs, API key encryption, SSRF-validated Provider testing, task APIs, SSE replay, reliable Redis queueing, Worker state transitions, backend Provider Adapter runtime, MinIO output assets, usage records, API call logs, operation logs, audit/usage read APIs, runtime-backed upload-policy settings, production secret guards, Worker process concurrency, and API Redis subscriber lifecycle ownership.
- The frontend has an API client, task/SSE client contracts, auth integration, project selection/creation, project asset upload/list/favorite/delete/download UI, project-scoped reference selection by backend `assetId`, admin Provider/model management UI, admin usage/audit/settings UI, backend task-backed workbench submission, backend result rendering, and backend history/detail/download/re-edit flows.
- P10 added a backend-owned project history query at `GET /api/v1/projects/{projectId}/history`. The frontend still uses the existing task/assets join until a later frontend migration consumes this unified endpoint.
- Docker Compose has buildable runtime foundations and passed P9 release validation; R10 re-verified Compose config. Routine development still uses the shared local MySQL/Redis/MinIO services documented in `docs/local-development.md`.

Remaining R10 follow-ups are documented in `docs/development-plan.md` and `docs/security.md`. The main open product/runtime follow-up is frontend consumption of the unified backend history endpoint; Provider delete/model create-update transaction hardening and further admin component splitting are non-blocking maintenance items.

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
