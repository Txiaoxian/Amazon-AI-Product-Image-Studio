# Deployment Plan

## Local development environment

Routine development and validation must use the shared machine-level services documented in `docs/local-development.md`.

- Use `dev-mysql8` on `127.0.0.1:3306` instead of creating a project-specific MySQL container.
- Use `dev-redis` on `127.0.0.1:6379` instead of creating a project-specific Redis container.
- Use `dev-minio` on `127.0.0.1:9000` instead of creating a project-specific MinIO container.
- Do not commit local service credentials to this repository. Credentials stay in the global local development document.

`deploy/docker-compose.yml` is the deployment topology. It is not the default routine development environment.

If a deployment-specific verification starts the project Compose stack, clean it up afterwards unless the user explicitly asks to keep it:

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

## Current state after P2

The repository has a Docker Compose topology, but it is not yet a working full-stack deployment.

Current verified state:

- `docker compose -f deploy/docker-compose.yml config` passes.
- Frontend local build and backend local Go builds pass.
- `docker compose -f deploy/docker-compose.yml build backend-api` fails because `backend/Dockerfile` does not exist.

Known runtime gaps:

- Backend Compose services reference `backend/Dockerfile`, which must be added in P3.
- Worker healthcheck expects `WORKER_HEALTHCHECK_FILE`, but the worker does not create it yet.
- Frontend container must proxy `/api/` to `backend-api:8080`; otherwise the frontend API client default `/api/v1` path is not routable in Docker.
- Frontend Nginx still contains legacy AI relay routing and must not proxy AI Provider traffic in the platform deployment.

## Services

Docker Compose must support:

- `frontend`
- `backend-api`
- `backend-worker`
- `mysql`
- `redis`
- `minio`

Optional later service:

- reverse proxy for TLS and routing.

## Target Compose layout

```text
deploy/
  docker-compose.yml
  mysql/
    init/
  minio/
  nginx/
```

## Environment file

Root `.env.example` should document:

- MySQL database, user, and password.
- Redis address and password if enabled.
- MinIO endpoint, access key, secret key, and bucket names.
- JWT signing secret.
- Cookie domain and secure mode.
- Allowed frontend origins.
- API key encryption secret.
- Upload limits.
- Concurrency limits.
- Provider timeout defaults.

Never commit real secrets.

## Health checks

Required health checks:

- MySQL ready.
- Redis ping.
- MinIO health endpoint.
- Backend API `/healthz`.
- Worker readiness or heartbeat.
- Frontend static service.

## Volumes

Persist:

- MySQL data.
- Redis data when persistence is enabled.
- MinIO object data.

Application containers should be stateless.

## Startup order

The API and worker depend on MySQL, Redis, and MinIO health. Migrations should run before API/worker serve traffic. The migration mechanism can be a one-shot container or API startup gate, but it must be explicit.

## Routing

Frontend routes:

- Static frontend assets.
- SPA fallback.
- `/api/*` proxy to `backend-api:8080` inside the Compose network.

API routes:

- `/api/v1/*` to backend API.
- `/api/v1/events/*` must preserve streaming behavior and avoid buffering.

Forbidden routing:

- The frontend container must not proxy OpenAI, Gemini, OpenAI-compatible relay, or custom AI Provider traffic.
- Nginx/static frontend deployment must not be used as an AI relay.

## P3 deployment acceptance

P3 is complete only when all of these pass from the repository root:

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
```

Expected health state:

- `mysql`, `redis`, and `minio` are healthy.
- `backend-api` is healthy through `/healthz`.
- `backend-worker` is healthy through the configured readiness mechanism.
- `frontend` is healthy and serves the app.

The P3 deployment check validates runtime wiring only. It does not require business APIs, database migrations, authentication, task execution, Provider calls, or MinIO asset flows to be implemented yet.

## Deployment verification

Minimum checks:

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
```

Then verify:

- Frontend loads.
- API health returns healthy.
- Worker is running.
- MySQL, Redis, and MinIO are reachable.

After local deployment verification, remove the project-specific containers and volumes unless the environment is intentionally being kept for deployment debugging:

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```
