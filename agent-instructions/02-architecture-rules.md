# Architecture Rules

## Target directory layout

The long-term repository root is the platform root:

```text
gpt-image/
  frontend/
  backend/
  deploy/
  docs/
  agent-instructions/
  AGENTS.md
  README.md
  .env.example
```

The current frontend files are still at the repository root. Moving them into `frontend/` is a P1 prerequisite and must be mechanical: preserve code, tests, styles, and behavior.

## Service boundaries

- Frontend owns UI, local draft state, API client, SSE client, and browser interactions.
- Backend API owns authentication, authorization, business APIs, upload authorization, task creation, Provider/model management, and SSE delivery.
- Backend Worker owns queued task execution, Provider calls, output upload, usage recording, and task event creation.
- MySQL is the final source of truth for users, tenants, projects, assets, tasks, task events, logs, and usage.
- Redis is for queues, locks, concurrency limits, cache, rate limits, and temporary state only.
- MinIO stores image objects and thumbnails.

## Required data flow

1. User authenticates through backend and receives HttpOnly Cookie.
2. Frontend loads projects, assets, providers, and model capabilities from `/api/v1`.
3. User creates a generation task.
4. Backend persists the task in MySQL and enqueues it in Redis.
5. Worker claims the task, calls the selected Provider Adapter, stores outputs in MinIO, and writes task events to MySQL.
6. Frontend receives task events over SSE and updates UI without polling.

## Hard architecture rules

- No frontend AI Provider calls.
- No frontend API key storage.
- No frontend task polling.
- No business SQL query without tenant scope.
- No image blob storage in MySQL.
- No direct business code dependency on a concrete Provider SDK or URL. Use Provider Adapter interfaces.
