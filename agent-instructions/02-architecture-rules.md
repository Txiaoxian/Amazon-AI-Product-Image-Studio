# Architecture Rules

## Target directory layout

The repository root is the active platform root:

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

The P1 frontend mechanical move is complete. Do not move backend code under `frontend/` or re-mix app roots. New frontend work stays in `frontend/`, backend work stays in `backend/`, deployment work stays in `deploy/`, and public contracts stay in `docs/`.

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
5. Worker claims the task, applies runtime concurrency limits, calls the selected Provider Adapter, stores outputs in MinIO, and writes task events to MySQL.
6. Frontend receives task events over SSE and updates UI without polling.
7. Project history currently uses backend task/assets data in the frontend; the P10 backend unified history endpoint exists and should be consumed by a later frontend migration task.

## Hard architecture rules

- No frontend AI Provider calls.
- No frontend API key storage.
- No frontend task polling.
- No business SQL query without tenant scope.
- No image blob storage in MySQL.
- No direct business code dependency on a concrete Provider SDK or URL. Use Provider Adapter interfaces.
