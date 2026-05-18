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

## Current transition state after P8

The repository is now structurally split into `frontend/`, `backend/`, `deploy/`, and `docs/`, and has backend/frontend infrastructure, authentication/RBAC, project management, reference asset management, Provider/model management, task APIs, SSE delivery, Worker queue processing, and backend Provider Adapter runtime foundations.

Important transition facts:

- The production frontend workbench now uses backend model capabilities, backend task creation, SSE task state, authorized backend assets, and backend task/asset history.
- Browser Provider adapters, frontend Provider registry/types, and normal local Provider API Key/API URL settings have been removed.
- IndexedDB is no longer the production source for generated images or history. It may still support non-sensitive prompt templates and residual non-production helpers/tests until R8/P9 cleanup.
- The backend currently has configuration, logging, router, health, response helpers, middleware, MySQL/GORM migrations, auth, RBAC, project APIs, asset APIs, MinIO storage abstraction, upload validation, authorized downloads, Provider APIs, model APIs, API key encryption, SSRF-validated Provider testing, task APIs, SSE replay, reliable Redis queueing, Worker state transitions, backend Provider Adapter runtime, MinIO output assets, usage records, and API call logs.
- The frontend has an API client, task/SSE client contracts, auth integration, project selection/creation, project asset upload/list/favorite/delete/download UI, project-scoped reference selection by backend `assetId`, admin Provider/model management UI, backend task-backed workbench submission, backend result rendering, and backend history/detail/download/re-edit flows.
- R8 must verify the full P8 migration result before P9 hardening starts. Known cleanup candidates include unreachable legacy display components, old IndexedDB helper files, broad frontend `422` handling, and frontend-side joining of separately paged task/asset lists.
- Docker Compose has buildable runtime foundations from P3. Routine development still uses the shared local MySQL/Redis/MinIO services documented in `docs/local-development.md`.

This transition state is closer to the target platform but still requires R8 verification and P9 hardening before release.

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
      provider/
      usage/
      audit/
      storage/
    migrations/
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

P0 documents this structure. P1 performs the frontend mechanical move into `frontend/`.

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
- Existing frontend Provider code becomes migration reference only, not production call path.
- Existing local history may be used for an import or compatibility feature later, but not as primary platform data.

## Transition guardrails

- Do not add new browser-side AI Provider calls.
- Do not add new frontend API key storage.
- Do not expand legacy Nginx AI relay behavior.
- Do not make IndexedDB the source of truth for any new platform feature.
- New platform features must be designed around backend API, MySQL, Redis, MinIO, Provider Adapter, and SSE contracts.
