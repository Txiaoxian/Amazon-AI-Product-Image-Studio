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

## Current state after P9 deployment release validation

The repository has a Docker Compose topology and buildable frontend/backend images validated after P9 release readiness work. P5-P10 platform features should still use the shared local development services for routine development unless the task explicitly requires Compose deployment validation.

Current verified state:

- `docker compose -f deploy/docker-compose.yml config` passes.
- `docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend` passes.
- `docker compose -f deploy/docker-compose.yml up -d` reaches healthy states for `mysql`, `redis`, `minio`, `backend-api`, `backend-worker`, and `frontend`.
- Frontend `/api/` proxy reaches `backend-api:8080`, and the frontend Nginx config contains no AI Provider or relay proxy.
- Frontend local lint, type-check, tests, and build pass after P9 deployment validation.
- Backend `go test`, race tests, `go vet`, API build, and worker build pass after P9 deployment validation.
- Shared local `dev-mysql8`, `dev-redis`, and `dev-minio` are the expected routine validation services and were verified reachable in R5.

Known runtime notes:

- Compose remains the deployment topology, not the default routine development environment.
- Frontend container must continue to proxy `/api/` only to `backend-api:8080`.
- Frontend Nginx must not proxy AI Provider traffic.
- The Compose stack includes the one-shot `minio-bootstrap` service for required buckets.
- Future release-affecting tasks must re-run Compose config/build/up/healthcheck before claiming release readiness.

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

P5 asset storage expects the configured MinIO buckets to exist before uploads are exercised. Current backend request handlers do not create buckets. Deployment or environment bootstrap must create or verify `MINIO_BUCKET_ORIGINALS`, `MINIO_BUCKET_GENERATED`, and `MINIO_BUCKET_THUMBNAILS`.

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

## P9 release validation expectations

`P9-DEPLOY-RELEASE-VALIDATION` is deployment-specific, so it may start the project Compose stack even though ordinary feature validation uses the shared local development services. It must clean the stack up afterwards unless the user explicitly asks to keep it.

Required checks:

- `docker compose -f deploy/docker-compose.yml config`
- `docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend`
- `docker compose -f deploy/docker-compose.yml up -d`
- `docker compose -f deploy/docker-compose.yml ps`
- API health from the published or internal route, depending on Compose topology.
- Frontend serves the built app and proxies `/api/` only to `backend-api`.
- Worker process stays running and reports the configured health/readiness signal.
- MySQL, Redis, and MinIO services are healthy.
- MinIO required buckets are created or the runbook clearly documents the bootstrap step.
- SSE route preserves streaming headers and is not buffered by the frontend/reverse proxy path.
- Production placeholder secrets are not used in deployment examples.

Documentation outputs:

- Update `.env.example` only with placeholders, never real local credentials.
- Update this deployment plan with actual P9 validation results and remaining operational notes.
- Add or update a release runbook if one does not exist yet, including initialization admin flow, bucket/bootstrap notes, backup/restore notes, and cleanup commands.

## P9 deployment validation result

Validation date: 2026-05-20.

Actual commands executed from the repository root unless noted:

```bash
cd backend && go test ./...
cd backend && go test -race ./...
cd backend && go vet ./...
cd backend && go build ./cmd/api ./cmd/worker
cd frontend && npm ci
cd frontend && npm run lint
cd frontend && npm run type-check
cd frontend && npm run test
cd frontend && npm run build
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml logs --tail=120 backend-api
docker compose -f deploy/docker-compose.yml logs --tail=120 backend-worker
docker compose -f deploy/docker-compose.yml logs --tail=120 frontend
docker compose -f deploy/docker-compose.yml run --rm --no-deps minio-bootstrap
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

Observed result:

- Backend tests, race tests, vet, and API/Worker builds passed.
- Frontend lint, type-check, tests, and build passed after installing dependencies with `npm ci`.
- Compose config passed.
- Compose image build passed for `backend-api`, `backend-worker`, and `frontend`.
- `docker compose up -d` started the stack successfully.
- `mysql`, `redis`, `minio`, `backend-api`, `backend-worker`, and `frontend` reached `healthy`.
- `minio-bootstrap` exited with code `0` and created or verified `product-originals`, `product-generated`, and `product-thumbnails`.
- Direct API health at `http://127.0.0.1:8081/healthz` returned `database`, `redis`, and `minio` as `ok`.
- Frontend proxy health at `http://127.0.0.1:8080/api/v1/healthz` returned the same backend API health payload, proving `/api/` routes to `backend-api:8080`.
- Frontend root and a deep SPA route both returned `200 text/html`; built JS and CSS assets returned `200` and were not swallowed by SPA fallback.
- Runtime Nginx config showed only `/api/v1/events/` and `/api/` proxy locations, both targeting `backend-api:8080`; no OpenAI, Gemini, custom Provider, or AI relay proxy was present.
- The SSE proxy location has `proxy_buffering off`, cache disabled, long read/send timeouts, and `X-Accel-Buffering: no`.
- API and Worker both rejected placeholder `JWT_SIGNING_SECRET` and placeholder `API_KEY_ENCRYPTION_KEY` when run with `APP_ENV=production`.
- Cleanup completed with `docker compose -f deploy/docker-compose.yml down -v --remove-orphans`; follow-up checks showed no project containers or project volumes left behind.

Operational notes:

- `.env.example` contains placeholders only. Do not use it unchanged for staging or production.
- The Compose stack now includes a repeatable one-shot `minio-bootstrap` service using `mc mb --ignore-existing` for required buckets.
- Current Redis 7.4 logs can include a go-redis `maintnotifications` fallback warning. It did not affect health checks or Worker/API operation during validation.
