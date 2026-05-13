# Architecture

## Current frontend baseline

The current repository is a pure frontend local app:

- React + TypeScript + Vite.
- Tailwind CSS.
- Dexie / IndexedDB for image blobs, history records, and prompt templates.
- localStorage for settings and Provider API keys.
- Frontend Provider Adapters for OpenAI, Gemini/Nano Banana, and OpenAI-compatible relay calls.
- Static Nginx Docker deployment.

This baseline must be preserved while the platform backend is added. Platformization is not a rewrite.

## Current transition state after R6

The repository is now structurally split into `frontend/`, `backend/`, `deploy/`, and `docs/`, and has backend/frontend infrastructure, authentication/RBAC, project management, reference asset management, and Provider/model management foundations.

Important transition facts:

- The frontend still has legacy local Provider adapters, localStorage Provider settings for the old workbench, and IndexedDB image/history storage.
- The backend currently has configuration, logging, router, health, response helpers, middleware, MySQL/GORM migrations, auth, RBAC, project APIs, asset APIs, MinIO storage abstraction, upload validation, authorized downloads, Provider APIs, model APIs, API key encryption, and SSRF-validated Provider testing.
- The frontend has an API client, SSE client foundation, auth integration, project selection/creation, project asset upload/list/favorite/delete/download UI, project-scoped reference selection, and admin Provider/model management UI.
- The workbench still uses the legacy local generation flow. Task queue execution, real backend Provider Adapter runtime, and SSE task updates are not implemented yet.
- Docker Compose has buildable runtime foundations from P3. Routine development still uses the shared local MySQL/Redis/MinIO services documented in `docs/local-development.md`.

This transition state is allowed only as an incremental migration baseline. It is not the target architecture and must not be copied into new platform features.

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
