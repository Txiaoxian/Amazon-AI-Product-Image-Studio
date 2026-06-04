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

## Current state after P20 operational hardening

The repository has a Docker Compose topology and buildable frontend/backend images validated after P9 release readiness work. R12 re-verified Compose config after P12 seller workflow and project-member hardening. P15 security regression added `scripts/security-regression.sh`, which now validates focused security tests, frontend forbidden-pattern scans, backend sensitive-marker scans, frontend `/api/` proxy safety, Compose config, and whitespace checks.

Platform feature work should still use the shared local development services for routine development unless the task explicitly requires Compose deployment validation.

Current verified state:

- `docker compose -f deploy/docker-compose.yml config` passes.
- `docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend` passes.
- `docker compose -f deploy/docker-compose.yml up -d` reaches healthy states for `mysql`, `redis`, `minio`, `backend-api`, `backend-worker`, and `frontend`.
- Frontend `/api/` proxy reaches `backend-api:8080`, and the frontend Nginx config contains no AI Provider or relay proxy.
- Frontend local lint, type-check, tests, and build pass after R12 validation.
- Backend `go test`, race tests, `go vet`, API build, and worker build pass after R12 validation.
- R12 validation confirmed `docker compose -f deploy/docker-compose.yml config` still passes after unified frontend history, seller project/asset workflow polish, and backend project-member last-`OWNER` hardening.
- Shared local `dev-mysql8`, `dev-redis`, and `dev-minio` are the expected routine validation services and were verified reachable in R5.
- `scripts/real-provider-smoke.sh` exists as an optional manual smoke entry point for backend-mediated real Provider validation. Its default help/dry-run paths do not call AI Providers or consume credits.
- `scripts/prod-dry-run.sh` passed safe default and live Compose rehearsal modes with scoped cleanup; no project containers or volumes remained afterwards.
- `deploy/nginx/amazon-ai-product-image-studio.conf.template` and `scripts/tls-reverse-proxy-check.sh` define and validate the host TLS edge. External traffic routes only to loopback frontend `127.0.0.1:8080`.
- Compose and backend configuration pin the browser/backend CSRF request-header contract to `X-CSRF-Token`; deployment must not override it.
- Production backend startup and production dry-run preflight now fail closed unless `CSRF_ENABLED=true`.
- Login rate limiting uses Redis with `AUTH_LOGIN_RATE_LIMIT_MAX_FAILURES` and `AUTH_LOGIN_RATE_LIMIT_WINDOW`; defaults are conservative and do not expose email/IP in Redis keys.
- `backend/cmd/provider-key-rotation` provides a default-safe dry-run and explicitly confirmed transactional apply path for Provider master-key rotation. Active Provider credentials are re-encrypted; historical soft-deleted Provider credential remnants are count-reported in dry-run and crypto-erased in apply.
- `backend/cmd/provision-tenant` provides a default-safe dry-run and explicitly confirmed transactional apply path for additional tenant creation.
- The `backend-api` image bundles both operator CLI binaries so Compose servers can run them through scoped `docker compose run --rm --no-deps` commands without a host Go toolchain.
- `scripts/backup-restore-rehearsal.sh` passed an isolated live Compose matching MySQL/MinIO restore and rollback rehearsal with scoped cleanup.
- The frontend tenant/custom-role administration UI is merged and the frontend test toolchain was upgraded to `vitest@^4.1.8`; frontend audit reports zero vulnerabilities.

Known runtime notes:

- Compose remains the deployment topology, not the default routine development environment.
- Frontend container must continue to proxy `/api/` only to `backend-api:8080`.
- Frontend Nginx must not proxy AI Provider traffic.
- The Compose stack includes the one-shot `minio-bootstrap` service for required buckets.
- Future release-affecting tasks must re-run Compose config/build/up/healthcheck before claiming release readiness.
- Provider master-key rotation and backup/restore rehearsal are implemented. Operators must still execute their default-safe checks and approved target-environment procedures before production changes.
- R20 deployment follow-ups for production env-file propagation, redacted health-failure logs, bounded container log rotation, exact MinIO restore semantics, startup migration serialization, Worker quota reconciliation, Provider attempt ledgering, and Redis-backed login rate limiting are now implemented. P21 still requires the remaining SSE/session/lease/readiness/frontend-cleanup hardening slices before final Go/No-Go.

## Services

Docker Compose must support:

- `frontend`
- `backend-api`
- `backend-worker`
- `mysql`
- `redis`
- `minio`

Required host service:

- TLS reverse proxy for public HTTPS routing using `deploy/nginx/amazon-ai-product-image-studio.conf.template` or an equivalent configuration validated by `scripts/tls-reverse-proxy-check.sh`.

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

## P15 deployment runbook result

Validation date: 2026-05-26.

`P15-DEPLOY-RUNBOOK-FINAL` was reviewed and merged. It kept public deployment contracts stable and added repeatable operator validation.

Added outputs:

- `scripts/deploy-release-validation.sh` with `--help`, safe default checks, explicit `--up`, and cleanup through `--down`.
- `deploy/RUNBOOK.md` covering prerequisites, `.env` setup with placeholders only, production secret replacement, startup, health checks, init-admin, MinIO bucket/bootstrap, SSE proxy behavior, backup/restore, upgrade/rollback, log inspection, and cleanup.

Actual checks passed:

- `bash scripts/deploy-release-validation.sh --help`
- `bash scripts/deploy-release-validation.sh`
- `bash scripts/deploy-release-validation.sh --up --down`
- `bash scripts/security-regression.sh --help`
- `docker compose -f deploy/docker-compose.yml config`
- `docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend`
- Full frontend lint, type-check, tests, and build
- Full backend tests, race tests, vet, and API/Worker builds
- `git diff --check main...HEAD`

Live Compose validation confirmed:

- `mysql`, `redis`, `minio`, `backend-api`, `backend-worker`, and `frontend` reached healthy/running states.
- `minio-bootstrap` completed successfully.
- Backend `/healthz` and `/api/v1/healthz` returned HTTP 200.
- Frontend root returned HTTP 200.
- Frontend `/api/v1/healthz` proxied to backend and returned HTTP 200.
- Frontend `/api/v1/events/tasks` reached the backend auth boundary and returned HTTP 401.
- Cleanup with `docker compose -f deploy/docker-compose.yml down -v --remove-orphans` removed project containers and volumes.

Follow-up status:

- P16 has added the cleanup trap so a failed or interrupted `--up --down` run still attempts automatic Compose cleanup.
- P16 has added backend database-log retention through the Worker maintenance process. Operators still manage container stdout/stderr and external log aggregation retention outside the backend `logRetention` setting.
- P16 has added backend-generated asset thumbnails for new reference uploads and Worker outputs. Operators must keep the configured thumbnails bucket available along with originals/generated buckets; object access still goes through backend authorization.

## R15 deployment readiness result

Validation date: 2026-05-26.

R15 re-ran the deployment readiness gates on latest `main` after all P15 slices were merged. `scripts/deploy-release-validation.sh`, live `scripts/deploy-release-validation.sh --up --down`, full frontend and backend regression, Docker Compose config, and whitespace checks all passed. Follow-up checks confirmed no `amazon-ai-product-image-studio` Compose containers or volumes remained after cleanup.

## P16 deployment script hardening plan

`P16-DEPLOY-SCRIPT-HARDENING` is the first stable-production task after R15.

Required behavior:

- `scripts/deploy-release-validation.sh --up --down` must attempt cleanup when live validation fails, the script errors, or the process receives SIGINT/SIGTERM.
- `--up` without `--down` must keep the current operator-inspection behavior and leave the stack running.
- `--down` without `--up` must remain cleanup-only.
- Default validation must not start or delete the Compose stack.
- Cleanup must stay scoped to `deploy/docker-compose.yml` and must not use broad Docker prune commands.
- Script-level tests should use fake Docker commands where possible so cleanup-trap failure paths are covered without relying on real infrastructure failures.

## P16 deployment script hardening result

Validation date: 2026-05-26.

`P16-DEPLOY-SCRIPT-HARDENING` was reviewed and merged. The deployment validation script now has scoped cleanup traps for `--up --down` so live validation failure, script error, SIGINT, or SIGTERM still attempts project Compose cleanup. `--up` without `--down` keeps the operator-inspection behavior, `--down` alone remains cleanup-only, and default validation still does not start or delete the Compose stack.

Added output:

- `scripts/deploy-release-validation-test.sh`, a fake-command shell regression suite for deploy script cleanup behavior.

Actual checks passed:

- `bash -n scripts/deploy-release-validation.sh`
- `bash -n scripts/deploy-release-validation-test.sh`
- `bash scripts/deploy-release-validation.sh --help`
- `bash scripts/deploy-release-validation-test.sh`
- `bash scripts/security-regression.sh`
- `bash scripts/deploy-release-validation.sh`
- `bash scripts/deploy-release-validation.sh --up --down`
- `docker compose -f deploy/docker-compose.yml ps -a`
- `docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true`
- `docker compose -f deploy/docker-compose.yml config`
- `git diff --check main...HEAD`

## P16 backend log retention result

Validation date: 2026-05-27.

`P16-BE-LOG-RETENTION` was reviewed, fixed, and merged. Backend `logRetention` covers database-backed `operation_logs`, `api_call_logs`, and terminal-task `task_events` only. The Worker maintenance loop consumes active tenant settings, applies bounded batch cleanup, preserves non-terminal task events for SSE/recovery, and records sanitized aggregate audit metadata. Container stdout/stderr, host logs, and external log aggregation retention remain deployment/operator responsibilities.

## R16 production launch hardening result

R16 reviewed the full P16 range after deployment script hardening, backend log retention, and backend thumbnail policy were merged. No blocking deployment issues were found.

Validation passed:

- `bash scripts/deploy-release-validation-test.sh`
- `bash scripts/security-regression.sh`
- `bash scripts/deploy-release-validation.sh`
- `bash scripts/deploy-release-validation.sh --up --down`
- `docker compose -f deploy/docker-compose.yml ps -a`
- `docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true`
- `docker compose -f deploy/docker-compose.yml config`

The live Compose run verified MySQL, Redis, MinIO, MinIO bootstrap, backend API, backend Worker, and frontend health checks; backend `/healthz`; frontend `/api/` proxy health; SSE auth-boundary routing; and cleanup of project containers and volumes.

Live Compose validation confirmed the stack reached healthy/running state, frontend `/api/` and SSE auth-boundary proxy checks passed, cleanup completed, and follow-up checks showed no project containers or project volumes left behind.

## P18 optional real Provider smoke result

Validation date: 2026-05-29.

`P18-E2E-REAL-PROVIDER-SMOKE` was reviewed and merged. It added:

- `scripts/real-provider-smoke.sh`, a manual, opt-in smoke script for the backend-mediated real Provider path.
- `scripts/real-provider-smoke-test.sh`, a fake-command regression suite for script guardrails and redaction.
- A short optional smoke section in `deploy/RUNBOOK.md`.

Safety properties:

- No arguments and `--help` only print usage.
- `--dry-run` validates local guardrails and does not call any API.
- `--run` requires `REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS`, required environment variables, bounded output count, bounded timeout, and a platform `/api/v1` API base.
- The script rejects direct AI Provider API bases such as OpenAI or Google API hosts.
- The script uses a temporary cookie jar and tracked temporary payload/output files, then removes them on exit.
- Script output does not print full Provider API keys, Authorization headers, Cookies, CSRF tokens, JWTs, image base64, object keys, bucket names, signed URLs, or raw Provider responses.
- Default release validation and security regression scripts still do not call real AI Providers.

Checks executed:

```bash
bash scripts/real-provider-smoke.sh --help
bash scripts/real-provider-smoke.sh --dry-run
bash scripts/real-provider-smoke.sh --run
bash scripts/real-provider-smoke-test.sh
bash scripts/deploy-release-validation-test.sh
bash scripts/deploy-release-validation.sh --help
bash scripts/deploy-release-validation.sh
bash scripts/security-regression.sh
docker compose -f deploy/docker-compose.yml config
git diff --check main...HEAD
```

All checks passed except the plain `--run` guard check, which intentionally failed before any API call because the confirmation variable was absent. No real Provider call was executed during automated validation.
