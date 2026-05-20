# Release Runbook

This runbook covers the Docker Compose release topology for Amazon AI Product Image Studio. Use placeholders in repository files only; real secrets must come from the target environment secret manager or operator-managed `.env`.

## Preconditions

- Docker Engine and Docker Compose are running.
- Required images can be pulled or are already present.
- Host ports `127.0.0.1:8080` and `127.0.0.1:8081` are available for local validation.
- Production and staging environments replace every `change-me` placeholder before startup.
- Provider API keys are configured only through backend admin APIs after initialization, never in frontend or Compose files.

## Environment

Create an untracked `.env` from `.env.example` and replace secrets:

```bash
cp .env.example .env
```

Required production replacements include:

- `JWT_SIGNING_SECRET`
- `API_KEY_ENCRYPTION_KEY`
- `MYSQL_ROOT_PASSWORD`
- `MYSQL_PASSWORD`
- `REDIS_PASSWORD`
- `MINIO_ROOT_PASSWORD`
- `MINIO_SECRET_KEY`

Set `COOKIE_SECURE=true` behind HTTPS. Keep `PROVIDER_ALLOW_PRIVATE_BASE_URLS` absent; Provider SSRF protections must stay enforced by backend runtime code.

If `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` differ from the MinIO root user, provision that service account with access to the configured buckets before starting API and Worker containers.

## Build And Start

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
```

Expected state:

- `mysql`, `redis`, `minio`, `backend-api`, `backend-worker`, and `frontend` are `healthy`.
- `minio-bootstrap` exits with code `0`.
- `frontend` publishes `127.0.0.1:8080`.
- `backend-api` publishes `127.0.0.1:8081`.

## MinIO Bootstrap

The `minio-bootstrap` service creates required buckets idempotently:

- `MINIO_BUCKET_ORIGINALS`
- `MINIO_BUCKET_GENERATED`
- `MINIO_BUCKET_THUMBNAILS`

Rerun the bootstrap safely:

```bash
docker compose -f deploy/docker-compose.yml run --rm --no-deps minio-bootstrap
```

## Health Checks

```bash
curl -fsS http://127.0.0.1:8081/healthz
curl -fsS http://127.0.0.1:8080/api/v1/healthz
curl -fsSI http://127.0.0.1:8080/
curl -fsSI http://127.0.0.1:8080/workbench/deep/link
```

The API health payload must report `database`, `redis`, and `minio` as `ok`. The frontend `/api/` health request must return the backend API health payload.

Verify Nginx proxy rules:

```bash
docker compose -f deploy/docker-compose.yml exec -T frontend nginx -T 2>/dev/null | \
  rg -n "location /api|proxy_pass|proxy_buffering|X-Accel-Buffering|api\\.openai|generativelanguage|gemini|relay"
```

Only `/api/v1/events/` and `/api/` should proxy to `backend-api:8080`. The SSE location must have buffering disabled. No AI Provider or relay proxy should appear.

## Initialize Administrator

After the API is healthy, initialize the first administrator from a trusted operator machine:

```bash
curl -fsS -X POST http://127.0.0.1:8081/api/v1/auth/init-admin \
  -H 'Content-Type: application/json' \
  --data '{"email":"admin@example.com","password":"<strong-admin-password>","displayName":"Admin"}'
```

Use environment-specific admin credentials. Avoid leaving the password in shell history; a secure operator script or temporary request body file is safer for real environments.

## Logs

```bash
docker compose -f deploy/docker-compose.yml logs --tail=120 backend-api
docker compose -f deploy/docker-compose.yml logs --tail=120 backend-worker
docker compose -f deploy/docker-compose.yml logs --tail=120 frontend
```

Logs must not contain full API keys, Authorization headers, Cookies, passwords, or image base64 data. Redis 7.4 may emit a go-redis `maintnotifications` fallback warning; it is non-blocking if health checks remain green.

## Backup And Restore Outline

Back up:

- MySQL database dump from the `mysql` service.
- MinIO object data for the configured buckets.
- Environment secret material from the external secret store.

Redis stores queues, locks, limits, and temporary delivery state only; it is not the source of truth for task status. For restore, recover MySQL and MinIO from the same point-in-time window, then start Redis empty unless a deliberate queue recovery plan exists.

## Troubleshooting

- Compose config fails: check `.env` syntax, unsupported Compose version, and unresolved variables.
- Image build fails: verify image registry access, Dockerfile contexts, and `package-lock.json` / `go.sum` availability.
- API health is degraded: check MySQL credentials/connectivity, Redis password/address, and MinIO bucket bootstrap.
- Worker restarts or is not healthy: inspect `backend-worker` logs and confirm `WORKER_HEALTHCHECK_FILE` is writable inside the container.
- Frontend `/api/` fails: confirm `backend-api` is healthy and frontend Nginx still proxies only to `backend-api:8080`.
- Production startup exits immediately: replace placeholder `JWT_SIGNING_SECRET` and `API_KEY_ENCRYPTION_KEY`.
- Port binding fails: change `FRONTEND_PORT` or `BACKEND_API_PORT`, or stop the conflicting local process.

## Stop And Clean Up

For local release validation, remove project containers, networks, and volumes:

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

For production, only use `-v` when intentionally destroying persisted MySQL, Redis, and MinIO volumes.
