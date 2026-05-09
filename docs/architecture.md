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
