# Deployment Plan

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

API routes:

- `/api/v1/*` to backend API.
- `/api/v1/events/*` must preserve streaming behavior and avoid buffering.

## Deployment verification

Minimum checks:

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
```

Then verify:

- Frontend loads.
- API health returns healthy.
- Worker is running.
- MySQL, Redis, and MinIO are reachable.
