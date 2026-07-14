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
- Auth and CSRF: `JWT_SIGNING_SECRET`, `AUTH_LOGIN_RATE_LIMIT_MAX_FAILURES`,
  `AUTH_LOGIN_RATE_LIMIT_WINDOW`, `AUTH_COOKIE_NAME`, `COOKIE_DOMAIN`,
  `COOKIE_SECURE`, `COOKIE_SAME_SITE`, `CSRF_ENABLED`, `CSRF_COOKIE_NAME`,
  `CSRF_HEADER_NAME`.
- CORS: `CORS_ALLOWED_ORIGINS`.
- API-key encryption: `API_KEY_ENCRYPTION_KEY`,
  `API_KEY_ENCRYPTION_KEY_ID`.
- Upload limits: `UPLOAD_MAX_FILE_SIZE_MB`, `UPLOAD_MAX_WIDTH`,
  `UPLOAD_MAX_HEIGHT`, `UPLOAD_MAX_PIXELS`, `UPLOAD_ALLOWED_MIME_TYPES`.
- Queue and worker controls: `TASK_*`, `WORKER_*`, `MIGRATIONS_MODE`.
- Provider runtime defaults: `PROVIDER_TIMEOUT_SECONDS`,
  `PROVIDER_MAX_RETRIES`, `PROVIDER_MAX_RESPONSE_SIZE_MB`,
  `PROVIDER_MAX_OUTPUT_IMAGE_SIZE_MB`.

Provider image responses are decoded through the worker temporary directory and
then streamed into MinIO. Size the worker's temporary storage for the configured
task concurrency and output limits. `PROVIDER_MAX_RESPONSE_SIZE_MB` defaults to
1024 MiB and `PROVIDER_MAX_OUTPUT_IMAGE_SIZE_MB` defaults to 512 MiB; the latter
must not exceed the former. These settings do not change the user upload limit.

For production, set `APP_ENV=production`, `COOKIE_SECURE=true`, restrict
`CORS_ALLOWED_ORIGINS` to the public frontend origin, and bind public traffic
through a TLS-terminating reverse proxy. Keep `FRONTEND_BIND_HOST=127.0.0.1`
and `BACKEND_API_BIND_HOST=127.0.0.1`; neither Compose port is a public
listener.

`CSRF_HEADER_NAME` is a fixed compatibility contract and must remain
`X-CSRF-Token`. The backend and production preflight reject aliases.

## Production Secrets

Generate unique high-entropy values per environment for:

- `MYSQL_ROOT_PASSWORD`, `MYSQL_PASSWORD`.
- `REDIS_PASSWORD`.
- `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD`, `MINIO_ACCESS_KEY`,
  `MINIO_SECRET_KEY`.
- `JWT_SIGNING_SECRET`.
- `API_KEY_ENCRYPTION_KEY`.

`API_KEY_ENCRYPTION_KEY` must be a valid 32-byte key encoded as expected by the
backend configuration. The current application has one active encryption key.
Use the operator workflow below before changing that active key in an
environment with stored Provider credentials.

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

## External TLS Reverse Proxy

Do not open public traffic until the host-level TLS reverse proxy is enabled
and validated. Use
[`nginx/amazon-ai-product-image-studio.conf.template`](./nginx/amazon-ai-product-image-studio.conf.template)
as the auditable production template.

The host proxy:

- redirects HTTP port `80` to HTTPS;
- terminates TLS on port `443`;
- adds HSTS, `X-Content-Type-Options`, `X-Frame-Options`, and
  `Referrer-Policy`;
- proxies `/`, `/api/`, and `/api/v1/events/` only to the loopback-bound
  frontend at `127.0.0.1:8080`;
- never proxies the public edge directly to `backend-api`, OpenAI, Gemini, or
  a relay.

Certificate issuance and renewal are operator or platform responsibilities.
The repository template contains placeholder certificate and private-key
paths only. Do not commit real certificates or private keys.

To prepare the host config:

```bash
cp deploy/nginx/amazon-ai-product-image-studio.conf.template \
  /etc/nginx/conf.d/amazon-ai-product-image-studio.conf
# Replace __PUBLIC_HOST__ and update operator-managed TLS paths if needed.
bash scripts/tls-reverse-proxy-check.sh \
  --config /etc/nginx/conf.d/amazon-ai-product-image-studio.conf
nginx -t
# Reload Nginx only after nginx -t succeeds.
```

Use the platform-approved reload command after `nginx -t`, such as
`systemctl reload nginx`. Then validate the HTTPS response headers, HTTP
redirect, and SSE route before opening traffic:

```bash
curl -fsSI http://studio.example.com/
curl -fsSI https://studio.example.com/
curl -N --max-time 10 https://studio.example.com/api/v1/events/tasks
```

The unauthenticated SSE request may return an authorization error; it still
checks edge routing. Before Go/No-Go, use an authenticated browser session or
restricted cookie jar to confirm task events arrive incrementally without
batching.

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

## Additional Tenant Provisioning

The unauthenticated init-admin endpoint is only for the first tenant. Create
second and later tenants through the operator CLI. The default command validates
input without opening the database or writing rows:

```bash
export PROVISION_TENANT_NAME='Seller Team'
export PROVISION_TENANT_ADMIN_EMAIL='seller-admin@example.com'
export PROVISION_TENANT_ADMIN_DISPLAY_NAME='Seller Admin'
docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -e PROVISION_TENANT_NAME -e PROVISION_TENANT_ADMIN_EMAIL \
  -e PROVISION_TENANT_ADMIN_DISPLAY_NAME \
  --entrypoint provision-tenant backend-api
```

Apply only after reviewing the dry-run:

```bash
PROVISION_TENANT_CONFIRM=I_UNDERSTAND_TENANT_PROVISIONING \
  docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -e PROVISION_TENANT_NAME -e PROVISION_TENANT_ADMIN_EMAIL \
  -e PROVISION_TENANT_ADMIN_DISPLAY_NAME -e PROVISION_TENANT_CONFIRM \
  --entrypoint provision-tenant backend-api --apply
unset PROVISION_TENANT_CONFIRM
```

The apply path transactionally creates one tenant, built-in roles and grants,
one tenant admin, and a sanitized audit record. It prints only the new tenant ID
needed for login. Both commands read the initial password from container stdin
without echo when `PROVISION_TENANT_ADMIN_PASSWORD` is not passed. Do not retain
the password in the shell environment.

## Provider Master-Key Rotation

Rotate the Provider API-key encryption master key only during an approved
maintenance window with Provider writes paused. First run the default dry-run
against all Provider rows. The dry-run validates active Provider credentials and
reports only a count of soft-deleted Provider rows that still need credential
crypto erase:

```bash
export PROVIDER_KEY_ROTATION_OLD_SECRET='<current secret>'
export PROVIDER_KEY_ROTATION_OLD_KEY_ID='<current key id>'
export PROVIDER_KEY_ROTATION_NEW_SECRET='<new secret>'
export PROVIDER_KEY_ROTATION_NEW_KEY_ID='<new key id>'
docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -e PROVIDER_KEY_ROTATION_OLD_SECRET -e PROVIDER_KEY_ROTATION_OLD_KEY_ID \
  -e PROVIDER_KEY_ROTATION_NEW_SECRET -e PROVIDER_KEY_ROTATION_NEW_KEY_ID \
  --entrypoint provider-key-rotation backend-api
```

Apply only after the dry-run succeeds:

```bash
PROVIDER_KEY_ROTATION_CONFIRM=I_UNDERSTAND_PROVIDER_KEY_ROTATION \
  docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -e PROVIDER_KEY_ROTATION_OLD_SECRET -e PROVIDER_KEY_ROTATION_OLD_KEY_ID \
  -e PROVIDER_KEY_ROTATION_NEW_SECRET -e PROVIDER_KEY_ROTATION_NEW_KEY_ID \
  -e PROVIDER_KEY_ROTATION_CONFIRM \
  --entrypoint provider-key-rotation backend-api --apply
unset PROVIDER_KEY_ROTATION_OLD_SECRET PROVIDER_KEY_ROTATION_OLD_KEY_ID
unset PROVIDER_KEY_ROTATION_NEW_SECRET PROVIDER_KEY_ROTATION_NEW_KEY_ID
unset PROVIDER_KEY_ROTATION_CONFIRM
```

The apply path serializes one database transaction, re-encrypts all eligible
active credentials, crypto-erases credential material from soft-deleted Provider
rows, and rolls back fully if any active row fails. It never prints plaintext,
ciphertext, hint, URL, tenant, or Provider details. After apply succeeds, deploy
API and Worker with the new `API_KEY_ENCRYPTION_KEY` and
`API_KEY_ENCRYPTION_KEY_ID`, then run backend-mediated Provider smoke checks.

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

The external TLS reverse proxy template preserves streaming behavior:

- Use HTTP/1.1 upstream connections.
- Clear the upstream `Connection` header.
- Disable response buffering for `/api/v1/events/*`.
- Disable proxy cache for `/api/v1/events/*`.
- Set long read timeouts for SSE.
- Forward `Host`, `X-Real-IP`, `X-Forwarded-For`, and
  `X-Forwarded-Proto=https`.

## Release Validation

Run the repeatable release gate:

```bash
bash scripts/deploy-release-validation.sh
```

The default gate includes the host TLS reverse proxy template static check.
Run it directly against an installed host config before reloading Nginx:

```bash
bash scripts/tls-reverse-proxy-check.sh \
  --config /etc/nginx/conf.d/amazon-ai-product-image-studio.conf
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

## Production Dry-Run

Before a Go/No-Go review, run the sanitized operator rehearsal:

```bash
bash scripts/prod-dry-run.sh
```

The default mode runs release validation, focused security regression, the
real Provider smoke guardrail dry-run, the backup/restore rehearsal guardrail
dry-run, and Compose config validation. It does not start persistent services,
replace data, or call a real AI Provider.

For an explicit production environment acceptance check, point the script at
the existing restricted runtime env file outside the repository:

```bash
bash scripts/prod-dry-run.sh \
  --production-env-file /secure/runtime/production.env
```

The preflight reads but never sources the file and never prints values. It
fails closed unless production mode, secure cookies, restricted non-localhost
HTTPS CORS origins, and required non-placeholder secrets are present.

For a live Compose rehearsal with cleanup:

```bash
bash scripts/prod-dry-run.sh --live-compose
docker compose -f deploy/docker-compose.yml ps -a
docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true
```

`--live-compose` delegates to `deploy-release-validation.sh --up --down`, so
the existing scoped cleanup trap removes this project's Compose containers and
volumes even when live validation fails. Do not replace it with broad Docker
prune commands.

Use [PRODUCTION_DRY_RUN_TEMPLATE.md](./PRODUCTION_DRY_RUN_TEMPLATE.md) for the
R18 review evidence. Record sanitized stage results only. Do not attach env
files, dumps, secrets, Provider responses, image outputs, bucket names, object
keys, signed URLs, or service credentials.

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

To include that same optional check in the production dry-run summary, keep
using the hidden-input setup above and explicitly opt in:

```bash
REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS \
bash scripts/prod-dry-run.sh --real-provider-smoke
```

This only delegates to `real-provider-smoke.sh --run`; there is no second
Provider call path. Skip it unless an operator has approved the billable
backend-mediated smoke.

Do not place real keys in committed files, shell scripts, screenshots, or
shared logs. The script only calls this platform's `/api/v1` backend, creates
`codex-smoke-*` Provider/model/project/task data, defaults to one output image,
and prints sanitized IDs and counts only. Review and remove smoke data after
the validation if the environment should stay clean.

## Backup

Back up MySQL and MinIO together so object keys and objects stay consistent.
Pause writes or take a maintenance window when possible.

Before using production data, rehearse the repository Compose procedure in its
disposable isolated project:

```bash
bash scripts/backup-restore-rehearsal.sh
BACKUP_RESTORE_REHEARSAL_CONFIRM=I_UNDERSTAND_DATA_REPLACEMENT \
  bash scripts/backup-restore-rehearsal.sh --live
```

The default command is guardrail-only. `--live` creates a dynamically named
Compose project, starts only its isolated MySQL and MinIO services, creates a
task-owned fixture, backs up and restores a matching pair, repeats the restore
as a rollback rehearsal, and removes its containers and volumes. It must never
be pointed at the shared local development services or a production runtime.
Record sanitized evidence with
[PRODUCTION_BACKUP_RESTORE_TEMPLATE.md](./PRODUCTION_BACKUP_RESTORE_TEMPLATE.md).

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
    mc mirror --overwrite --remove local/"$MINIO_BUCKET_ORIGINALS" /backup/"$MINIO_BUCKET_ORIGINALS"
    mc mirror --overwrite --remove local/"$MINIO_BUCKET_GENERATED" /backup/"$MINIO_BUCKET_GENERATED"
    mc mirror --overwrite --remove local/"$MINIO_BUCKET_THUMBNAILS" /backup/"$MINIO_BUCKET_THUMBNAILS"
  '
```

In production, the operator must use the approved backup tool for the runtime
platform, establish a write-stop or platform-supported consistency point, keep
the MySQL and MinIO backups as one matching set, and store backups outside the
Compose host. The repository rehearsal script is not a production backup tool.

## Restore

Restore into a stopped or isolated stack:

```bash
docker compose -f deploy/docker-compose.yml down --remove-orphans
docker compose -f deploy/docker-compose.yml up -d mysql redis minio
docker compose -f deploy/docker-compose.yml exec -T mysql \
  sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' \
  < backup/mysql.sql
```

Restore MinIO objects with the same bucket names, then start the application.
The `--remove` option makes the restore exact by deleting bucket objects that
are absent from the matching backup. Run this only against the stopped or
isolated restore target:

```bash
docker compose -f deploy/docker-compose.yml up minio-bootstrap
docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -v "$PWD/backup/minio:/backup:ro" \
  --entrypoint /bin/sh minio-bootstrap -c '
    mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
    mc mirror --overwrite --remove /backup/"$MINIO_BUCKET_ORIGINALS" local/"$MINIO_BUCKET_ORIGINALS"
    mc mirror --overwrite --remove /backup/"$MINIO_BUCKET_GENERATED" local/"$MINIO_BUCKET_GENERATED"
    mc mirror --overwrite --remove /backup/"$MINIO_BUCKET_THUMBNAILS" local/"$MINIO_BUCKET_THUMBNAILS"
  '
docker compose -f deploy/docker-compose.yml up -d backend-api backend-worker frontend
```

Validate with the release checks before allowing users back in.

For production restore, the operator must use the approved platform restore
tooling and restore the matching MySQL and MinIO set from the same consistency
point. Do not use the repository rehearsal script against a production
runtime.

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
3. If migrations or task writes occurred, use operator-approved tooling to
   restore the matching MySQL and MinIO backups from the same consistency
   point.
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
