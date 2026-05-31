# Deployment Runbook

This runbook is for operators deploying Amazon AI Product Image Studio with
`deploy/docker-compose.yml`.

Run commands from the repository root unless a command says otherwise.

## Safety Rules

- Never commit `.env` or real secret values.
- Do not put Provider API keys in `.env`; configure them through the backend
  admin APIs so they are encrypted at rest.
- Keep the frontend pointed at the backend only. It must not proxy or call
  OpenAI, Gemini, or relay endpoints directly.
- Use SSE for task status. Do not add polling checks to production deployment
  paths.
- Keep object storage in MinIO. MySQL stores metadata and object keys only.

## Environment Variables

Start from the template:

```bash
cp .env.example .env
```

Required groups:

- `COMPOSE_PROJECT_NAME`, `APP_ENV`, `LOG_LEVEL`, `TZ`.
- Host bindings: `FRONTEND_BIND_HOST`, `FRONTEND_PORT`,
  `BACKEND_API_BIND_HOST`, `BACKEND_API_PORT`.
- Build inputs: `FRONTEND_*`, `BACKEND_*`, image variables.
- MySQL: `MYSQL_ROOT_PASSWORD`, `MYSQL_DATABASE`, `MYSQL_USER`,
  `MYSQL_PASSWORD`, connection pool settings.
- Redis: `REDIS_PASSWORD`, `REDIS_DB`, `REDIS_APPENDONLY`.
- MinIO: `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD`, `MINIO_ACCESS_KEY`,
  `MINIO_SECRET_KEY`, `MINIO_BUCKET_ORIGINALS`,
  `MINIO_BUCKET_GENERATED`, `MINIO_BUCKET_THUMBNAILS`.
- Auth and CSRF: `JWT_SIGNING_SECRET`, `AUTH_COOKIE_NAME`,
  `COOKIE_DOMAIN`, `COOKIE_SECURE`, `COOKIE_SAME_SITE`, `CSRF_ENABLED`,
  `CSRF_COOKIE_NAME`, `CSRF_HEADER_NAME`.
- CORS: `CORS_ALLOWED_ORIGINS`.
- API-key encryption: `API_KEY_ENCRYPTION_KEY`,
  `API_KEY_ENCRYPTION_KEY_ID`.
- Upload limits: `UPLOAD_MAX_FILE_SIZE_MB`, `UPLOAD_MAX_WIDTH`,
  `UPLOAD_MAX_HEIGHT`, `UPLOAD_MAX_PIXELS`, `UPLOAD_ALLOWED_MIME_TYPES`.
- Queue and worker controls: `TASK_*`, `WORKER_*`, `MIGRATIONS_MODE`.
- Provider runtime defaults: `PROVIDER_TIMEOUT_SECONDS`,
  `PROVIDER_MAX_RETRIES`.

For production, set `APP_ENV=production`, `COOKIE_SECURE=true`, restrict
`CORS_ALLOWED_ORIGINS` to the public frontend origin, and bind public traffic
through a TLS-terminating reverse proxy.

## Production Secrets

Generate unique high-entropy values per environment for:

- `MYSQL_ROOT_PASSWORD`, `MYSQL_PASSWORD`.
- `REDIS_PASSWORD`.
- `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD`, `MINIO_ACCESS_KEY`,
  `MINIO_SECRET_KEY`.
- `JWT_SIGNING_SECRET`.
- `API_KEY_ENCRYPTION_KEY`.

`API_KEY_ENCRYPTION_KEY` must be a valid 32-byte key encoded as expected by the
backend configuration. The current application has one active encryption key
and does not yet ship an in-place Provider-key re-encryption workflow. Do not
rotate this key on an environment with stored Provider credentials until an
approved migration procedure has been implemented and rehearsed.

## Startup Order

Compose encodes the startup order with health checks:

1. MySQL, Redis, and MinIO start first.
2. `minio-bootstrap` creates the required buckets idempotently.
3. `backend-api` starts after data services and bucket bootstrap are healthy.
4. `backend-worker` starts after `backend-api` and data services are healthy.
5. `frontend` starts after `backend-api` is healthy.

Start the stack:

```bash
docker compose -f deploy/docker-compose.yml up -d
```

Check status:

```bash
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml logs --tail=100 backend-api backend-worker frontend
```

## Init Admin

After the API is healthy, create the first tenant admin once:

```bash
curl -fsS -X POST http://127.0.0.1:8081/api/v1/auth/init-admin \
  -H 'Content-Type: application/json' \
  -d '{
    "tenantName": "Default Tenant",
    "email": "admin@example.com",
    "password": "replace-with-a-strong-password",
    "displayName": "Admin"
  }'
```

The endpoint returns conflict after an admin already exists. Do not automate
this with hard-coded production credentials.

## MinIO Bootstrap

`minio-bootstrap` runs `mc mb --ignore-existing` for:

- `MINIO_BUCKET_ORIGINALS`
- `MINIO_BUCKET_GENERATED`
- `MINIO_BUCKET_THUMBNAILS`

Re-run only the bootstrap service when bucket creation needs to be retried:

```bash
docker compose -f deploy/docker-compose.yml up minio-bootstrap
```

Do not delete buckets during routine deploys. Image downloads and task outputs
depend on object keys stored in MySQL.

## SSE Proxy

The frontend image uses `frontend/nginx.conf`:

- `/api/` proxies to `http://backend-api:8080`.
- `/api/v1/events/` proxies to `http://backend-api:8080`.
- SSE buffering is disabled with `proxy_buffering off` and
  `X-Accel-Buffering: no`.

If adding an external reverse proxy, preserve streaming behavior:

- Use HTTP/1.1 upstream connections.
- Disable response buffering for `/api/v1/events/*`.
- Set long read timeouts for SSE.
- Forward `Host`, `X-Real-IP`, `X-Forwarded-For`, and
  `X-Forwarded-Proto`.

## Release Validation

Run the repeatable release gate:

```bash
bash scripts/deploy-release-validation.sh
```

For a live Compose smoke test:

```bash
bash scripts/deploy-release-validation.sh --up
docker compose -f deploy/docker-compose.yml ps
```

The `--up` mode starts the stack, waits for health checks, verifies backend
health endpoints, verifies the frontend root, verifies frontend `/api/` proxy
health, checks the SSE proxy auth boundary, and leaves the stack running for
operator inspection. Clean it up after inspection:

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

The script also exposes a cleanup-only shortcut:

```bash
bash scripts/deploy-release-validation.sh --down
```

Focused security regression:

```bash
bash scripts/security-regression.sh
```

Manual deployment checks:

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
curl -fsS http://127.0.0.1:8081/healthz
curl -fsS http://127.0.0.1:8081/api/v1/healthz
curl -fsS http://127.0.0.1:8080/api/v1/healthz
```

## Optional Real Provider Smoke

The real Provider smoke is a manual, opt-in check. It is not part of default
release validation or CI because it can create billable AI calls.

Dry-run the guardrails first:

```bash
bash scripts/real-provider-smoke.sh --dry-run
```

Run it only when you intentionally want a real backend-mediated AI call:

```bash
export SMOKE_API_BASE_URL=http://127.0.0.1:8081/api/v1
export SMOKE_ADMIN_EMAIL=admin@example.com
export SMOKE_MODEL_NAME=your-image-model
read -rsp "Admin password: " SMOKE_ADMIN_PASSWORD && export SMOKE_ADMIN_PASSWORD
printf "\n"
read -rsp "Provider API key: " SMOKE_PROVIDER_API_KEY && export SMOKE_PROVIDER_API_KEY
printf "\n"
REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS \
bash scripts/real-provider-smoke.sh --run
```

Do not place real keys in committed files, shell scripts, screenshots, or
shared logs. The script only calls this platform's `/api/v1` backend, creates
`codex-smoke-*` Provider/model/project/task data, defaults to one output image,
and prints sanitized IDs and counts only. Review and remove smoke data after
the validation if the environment should stay clean.

## Backup

Back up MySQL and MinIO together so object keys and objects stay consistent.
Pause writes or take a maintenance window when possible.

MySQL logical backup:

```bash
docker compose -f deploy/docker-compose.yml exec -T mysql \
  sh -c 'mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' \
  > backup/mysql.sql
```

MinIO object backup to a local directory while the stack is running:

```bash
mkdir -p backup/minio
docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -v "$PWD/backup/minio:/backup" \
  --entrypoint /bin/sh minio-bootstrap -c '
    mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
    mc mirror local/"$MINIO_BUCKET_ORIGINALS" /backup/"$MINIO_BUCKET_ORIGINALS"
    mc mirror local/"$MINIO_BUCKET_GENERATED" /backup/"$MINIO_BUCKET_GENERATED"
    mc mirror local/"$MINIO_BUCKET_THUMBNAILS" /backup/"$MINIO_BUCKET_THUMBNAILS"
  '
```

In production, prefer the approved backup tool for the runtime platform and
store backups outside the Compose host.

## Restore

Restore into a stopped or isolated stack:

```bash
docker compose -f deploy/docker-compose.yml down --remove-orphans
docker compose -f deploy/docker-compose.yml up -d mysql redis minio
docker compose -f deploy/docker-compose.yml exec -T mysql \
  sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' \
  < backup/mysql.sql
```

Restore MinIO objects with the same bucket names, then start the application:

```bash
docker compose -f deploy/docker-compose.yml up minio-bootstrap
docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -v "$PWD/backup/minio:/backup:ro" \
  --entrypoint /bin/sh minio-bootstrap -c '
    mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
    mc mirror /backup/"$MINIO_BUCKET_ORIGINALS" local/"$MINIO_BUCKET_ORIGINALS"
    mc mirror /backup/"$MINIO_BUCKET_GENERATED" local/"$MINIO_BUCKET_GENERATED"
    mc mirror /backup/"$MINIO_BUCKET_THUMBNAILS" local/"$MINIO_BUCKET_THUMBNAILS"
  '
docker compose -f deploy/docker-compose.yml up -d backend-api backend-worker frontend
```

Validate with the release checks before allowing users back in.

## Upgrade

1. Pull or check out the release.
2. Review `.env.example` for new variables and update `.env`.
3. Run `bash scripts/deploy-release-validation.sh`.
4. Create backups for MySQL and MinIO.
5. Build images:

```bash
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
```

6. Start in order:

```bash
docker compose -f deploy/docker-compose.yml up -d mysql redis minio minio-bootstrap
docker compose -f deploy/docker-compose.yml up -d backend-api backend-worker frontend
```

7. Validate health, logs, login, init-admin state, uploads, task creation, SSE
   task updates, and downloads.

## Rollback

Rollback must keep MySQL and MinIO consistent:

1. Stop writes or put the service in maintenance.
2. Check out the previous release and restore the matching `.env`.
3. If migrations or task writes occurred, restore the matching MySQL and MinIO
   backups from the same point in time.
4. Rebuild and start:

```bash
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
```

5. Run health and proxy checks before reopening traffic.

## Log Troubleshooting

Common commands:

```bash
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml logs --tail=200 backend-api
docker compose -f deploy/docker-compose.yml logs --tail=200 backend-worker
docker compose -f deploy/docker-compose.yml logs --tail=200 frontend
docker compose -f deploy/docker-compose.yml logs --tail=200 mysql redis minio minio-bootstrap
```

Symptoms to check:

- API unhealthy: inspect MySQL, Redis, MinIO health and bucket bootstrap logs.
- Worker unhealthy: inspect `WORKER_HEALTHCHECK_FILE`, queue settings, and
  backend-worker logs.
- Frontend `/api/` failures: verify `backend-api` health and nginx proxy
  config.
- SSE stalls: verify reverse proxy buffering is disabled and read timeouts are
  long enough.
- Upload failures: verify upload limits, MIME type, image dimensions, MinIO
  buckets, and backend authorization logs.

Logs must not include full API keys, Authorization headers, Cookies, passwords,
or image base64 data. Treat any such log line as a security incident.

## Cleanup

Stop containers but keep named volumes:

```bash
docker compose -f deploy/docker-compose.yml down --remove-orphans
```

Stop containers and delete Compose-managed data volumes:

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

Use `down -v` only for disposable local validation stacks or after verified
backups. It removes MySQL, Redis, and MinIO volume data for this Compose
project.
