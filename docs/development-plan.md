# Development Plan

## Development Environment Rule

Routine development and feature verification must use the existing shared local services documented in `docs/local-development.md`:

- MySQL: `dev-mysql8`
- Redis: `dev-redis`
- MinIO: `dev-minio`

The user has authorized future development tasks to use these shared local services for functional verification, including creating, updating, deleting, enqueueing, uploading, downloading, and cleaning up task-owned test data. Test data must be clearly attributable to the task or branch, and agents must not drop the project database, delete shared buckets, flush shared Redis, or remove unrelated data unless explicitly instructed.

Do not create project-specific MySQL, Redis, or MinIO containers for ordinary feature work. `deploy/docker-compose.yml` is reserved for deployment verification; if it starts project containers, clean them up afterwards unless the user explicitly asks to keep them:

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

## Current State After P18 Real Provider Smoke Merge

The project has moved from a pure frontend local app to a backend-backed multi-user platform foundation.

Phase status:

| Phase | Status | Result |
| --- | --- | --- |
| P0 | Complete | Planning docs and project Agent rules created. |
| P1 | Complete | Monorepo structure established: `frontend/`, `backend/`, `deploy/`, `docs/`. |
| P2 | Complete | Backend API infrastructure and frontend API/SSE client foundations. |
| P3 | Complete | Docker Compose runtime repair, backend API/Worker images, frontend `/api/` proxy, no AI relay. |
| P4 | Complete | MySQL/GORM, tenants, auth, RBAC foundation, HttpOnly Cookie JWT, CSRF, frontend login. |
| P5 | Complete | Projects, project members, MinIO-backed assets, upload validation, frontend project/asset UI. |
| P6 | Complete | Provider and model management with encrypted keys, SSRF defense, capability APIs, admin UI. |
| P7 | Complete | Task APIs, Redis queue, Worker execution, Provider Adapter runtime, SSE replay, usage/API logs. |
| P8 | Complete | Frontend generation/edit/history production flow moved to backend tasks, SSE, assets, and models. |
| P9 | Complete | Audit/usage reads, upload-policy settings, production secret guards, security/deploy regression. |
| P10 | Complete | Worker pool, SSE bridge lifecycle, Provider/model lifecycle, admin UI hardening, backend history query. |
| P11 | Complete | Backend and frontend tenant user/role administration are merged: user list/create/update/disable/enable, role assignment, role/permission reads, RBAC UI gating, and password/secret safety checks. |
| P12 | Complete | Seller workflow review completed. Frontend unified history, project/asset workflow polish, and backend project-member invariant hardening are merged and regressed. |
| P13 | Complete | Runtime-backed tenant task defaults, malformed-row hardening, task concurrency policy, storage cleanup foundation, storage retention runtime, storage quota accounting, frontend system settings, and R13 regression are complete. |
| P14 | Complete | Provider/model lifecycle integrity, backend usage/cost reporting, frontend cost observability, and R14 regression are complete. |
| P15 | Complete | Release hardening is complete. Core flow E2E, final security regression, deployment runbook validation, and R15 release-readiness review passed. |
| P16 | Complete | Production launch hardening is complete: deployment cleanup traps, runtime database log retention, backend thumbnail policy, and R16 regression passed. |
| P17 | Complete | Storage governance and observability are complete. Conservative orphan cleanup, strict quota reservation, backend production diagnostics, and R17 regression passed. |
| P18 | In Progress | Provider/model/default-setting serialization and opt-in real Provider smoke tooling are complete; production dry-run and R18 Go/No-Go review remain. |

R11 found no blocking issues across the complete P11 code range. `P11-BE-USER-ROLE-ADMIN` was reviewed and merged after fixing role/status permission boundaries. `P11-FE-USER-ROLE-ADMIN` was reviewed and merged after frontend permission gating, CSRF write requests, password non-persistence, and current-user disable protection were verified.

R11 validation passed:

- `cd frontend && npm run lint`
- `cd frontend && npm run type-check`
- `cd frontend && npm run test`
- `cd frontend && npm run build`
- `cd backend && go test ./...`
- `cd backend && go test -race ./...`
- `cd backend && go vet ./...`
- `cd backend && go build ./cmd/api ./cmd/worker`
- `docker compose -f deploy/docker-compose.yml config`
- `git diff --check 2b186fb..HEAD`
- Focused P11 frontend/backend sensitive-pattern scans for forbidden browser storage, polling, Provider direct calls, unsafe response fields, and secret markers.

`P12-FE-UNIFIED-HISTORY` was reviewed and merged. The frontend history production path now uses `GET /api/v1/projects/{projectId}/history` instead of joining task and asset lists in the browser. The task passed frontend lint, type-check, targeted history/API tests, full frontend tests, build, and whitespace checks. Non-blocking follow-up: history thumbnails currently use authorized asset download URLs; later polish can prefer safe same-origin thumbnail URLs when available.

`P12-FE-PROJECT-WORKFLOW-POLISH` was reviewed, fixed, and merged. The seller workspace now supports project edit, asset filters, asset metadata edit, project member entry points backed by real member APIs, upload/project-switch stale protection, and filtered-list consistency after favorite or metadata mutations. The task passed frontend lint, type-check, targeted project/asset tests, full frontend tests, build, and whitespace checks. Non-blocking follow-up: member remove success and member update/remove error paths can receive more granular frontend tests when the project-member backend invariants are hardened.

`P12-BE-PROJECT-MEMBER-HARDENING` was reviewed, fixed, and merged. Backend project member write paths now prevent deleting or downgrading the final `OWNER`, keep owner-transfer paths valid when another `OWNER` remains, preserve tenant/RBAC/project-role authorization, and verify blocked writes do not create successful operation logs. Validation passed backend project-route focused tests, full backend tests, race tests, vet, API/Worker builds, and whitespace checks. Non-blocking follow-up: MySQL-backed concurrent owner-mutation coverage can be added in later integration or E2E work.

R12 reviewed the complete P12 code range from `f843b1e..HEAD` and found no blocking issues. Validation passed frontend lint, type-check, tests, build, backend tests, race tests, vet, API/Worker builds, Docker Compose config, whitespace checks, and frontend forbidden-pattern scans for Provider direct calls, Provider key storage, task polling, and sensitive browser storage. Non-blocking follow-ups remain: prefer safe same-origin thumbnail URLs for history cards when available, add MySQL-backed concurrent owner-mutation coverage during later integration/E2E work, and keep P13 writable settings blocked until each setting has a real runtime consumer.

`P13-BE-RUNTIME-DEFAULTS` was reviewed and merged. The backend now exposes tenant `taskDefaults.{defaultProviderId,defaultModelId}` through the existing system-settings contract and consumes it only when task creation omits both Provider/model IDs. Explicit pairs remain unchanged; mixed explicit/default requests, absent or cleared defaults, invalid Provider/model ownership or enabled state, and capability-invalid default-backed submissions fail without task creation, enqueueing, or a successful `task.create` log. Validation passed focused settings/task/API tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, and whitespace checks.

`P13-BE-RUNTIME-DEFAULTS-HARDENING` was reviewed and merged. Invalid JSON, partial IDs, unknown fields, and blank IDs in a manually corrupted or legacy `task_defaults` row now fail closed as `422 VALIDATION_ERROR` for default-backed task creation, without task/event/enqueue/success-audit side effects. Explicit valid Provider/model submissions do not read unused damaged defaults, and genuine settings-storage failures remain sanitized `500 INTERNAL_ERROR`. Validation passed focused API/task/settings tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, and whitespace checks.

`P13-BE-CONCURRENCY-POLICY` was reviewed, fixed, and merged. The backend now exposes tenant `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}` only with a live Worker consumer. Tenant overrides can narrow or match environment hard caps, global concurrency stays environment-owned, Provider row `concurrencyLimit` remains an additional stricter cap, and malformed persisted concurrency settings fail closed before Provider execution or output/usage/API-call success side effects. Validation passed focused settings and Worker tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, and whitespace checks.

`P13-BE-STORAGE-CLEANUP-FOUNDATION` was reviewed and merged. Upload rollback after object write now uses an independent bounded cleanup context when metadata persistence fails, and backend asset cleanup has a tenant-scoped, batch-limited, idempotent foundation for physically deleting soft-deleted original and thumbnail objects with durable `purged_at` tracking. Validation passed focused asset/storage/database/API tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, and whitespace checks.

`P13-BE-STORAGE-RETENTION-RUNTIME` was reviewed and merged. The backend now exposes nullable `storageRetention.deletedAssetRetentionDays`, keeps automatic physical cleanup disabled by default, and runs a Worker maintenance loop that reads valid active-tenant retention settings and calls the cleanup foundation with tenant/cutoff/batch boundaries. Malformed/null/inactive settings fail closed and cleanup errors are logged with sanitized metadata only. Validation passed focused system-settings/settings/asset/database/worker tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, and whitespace checks. Later P13/P17 slices added storage quota accounting and conservative orphan discovery.

`P13-BE-STORAGE-QUOTA-ACCOUNTING` was reviewed and merged. The backend now exposes nullable `storageQuota.maxBytes` with read-only computed `storageQuota.usedBytes`, computes usage from tenant-scoped `image_assets.size_bytes` where `purged_at IS NULL`, and enforces quota before reference uploads and Worker output asset persistence. Quota failures return sanitized stable errors and avoid successful asset rows, task outputs, output events, usage records, successful operation logs, and object leaks. Validation passed focused system-settings/settings/asset/database/task tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, and whitespace checks. Later P17 work replaced this optimistic check with strict tenant-scoped quota counters and reservations for concurrent uploads and Worker outputs.

`P13-FE-SYSTEM-SETTINGS` was reviewed and merged. The frontend admin settings tab now displays and edits only runtime-backed settings: upload policy, task defaults, task concurrency, storage retention, and storage quota. Each save sends one top-level settings patch with CSRF, `storageQuota.usedBytes` remains read-only, and deferred settings such as log retention, orphan cleanup, manual cleanup, MinIO listing, and Provider secrets remain hidden. Validation passed frontend lint, type-check, targeted admin settings tests, full frontend tests, build, whitespace checks, and forbidden-pattern scans.

R13 reviewed the complete P13 code range from `eeba51f..HEAD` after merging frontend system settings. No blocking issues were found. Validation passed frontend lint, type-check, tests, build; backend tests, race tests, vet, API/Worker builds; Docker Compose config; whitespace checks; and frontend forbidden-pattern scans for direct AI Provider calls, browser Provider-key storage, task polling, deferred settings, bucket/object-key exposure, and sensitive auth strings. Later phases completed thumbnail policy, log retention, conservative orphan cleanup, and strict quota reservations. Remaining follow-ups include optional minimum bound for `WORKER_RETENTION_MAINTENANCE_INTERVAL`, built-in `asset:*` permission reconciliation for existing tenants, and stronger Provider/model transaction serialization.

`P14-BE-PROVIDER-MODEL-INTEGRITY` was reviewed, fixed, and merged. Provider/model management now rejects model create/update/enable paths that target disabled, deleted, or cross-tenant Providers; default task settings are revalidated when loaded; Provider delete takes a row lock and remains blocked while same-tenant non-deleted linked models exist; Provider disable through both `/disable` and `PATCH status=DISABLED` is rejected while enabled linked models remain. Failed writes do not record successful operation logs and conflict responses stay non-sensitive. Validation passed focused Provider/model/settings/API tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, and whitespace checks. Same-Provider `model_name` uniqueness was not enforced at this point because runtime task execution uses stable `modelId` references; P18 later tightened the write path with transaction serialization and duplicate rejection.

`P14-BE-USAGE-COST-REPORTING` was reviewed, fixed, and merged. Worker usage persistence now uses deterministic decimal cost estimation with stable 8-decimal formatting, invalid or incomplete pricing produces zero cost without failing successful Provider tasks, and admin usage summary supports tenant/user/project/Provider/model dimensions with tenant isolation, multi-currency grouping, stable pagination, and exact large-decimal cost preservation. Validation passed focused audit/API/task/database tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, and whitespace checks. Non-blocking follow-up: usage summary currently performs per-page exact cost aggregation in Go; later high-volume tuning may add a more efficient query/index strategy.

`P14-FE-COST-OBSERVABILITY` was reviewed, fixed, and merged. The admin usage tab now consumes backend usage/cost summaries for tenant/user/project/Provider/model dimensions, shows tenant totals, applies date/task/user/project/Provider/model filters consistently to totals, summaries, and records, supports summary drilldown into record filters, protects against stale usage responses, and displays backend cost strings without client-side authoritative recalculation. Validation passed frontend lint, type-check, tests, build, Compose config, whitespace checks, and focused forbidden-pattern scans.

R14 reviewed the complete P14 range from `5585d99..HEAD` after merging frontend cost observability. No blocking issues were found. Validation passed frontend lint/type-check/test/build, backend full tests/race/vet/build, P14 focused backend tests with `-count=1`, Docker Compose config, whitespace checks, and focused frontend scans for direct AI Provider calls, browser Provider-key storage, task polling, AI relay paths, image base64 persistence, and object storage leakage. The same-Provider `model_name` follow-up was later resolved by P18 write-path serialization. Remaining non-blocking follow-ups are usage-summary high-volume tuning and eventual removal of the frontend tenant-totals compatibility fallback for older `404` contracts.

`P15-E2E-CORE-FLOWS` was reviewed and merged. The backend now has an API-level core-flow integration test that starts from init-admin, validates HttpOnly session and CSRF behavior, creates Provider/model/project/reference asset/task data through real routes, rejects an SVG masquerading as PNG, runs a fake Worker execution without external AI calls, verifies task events and SSE Last-Event-ID replay, confirms generated asset download and project history, and reads usage summary/records plus operation/API call logs with sensitive markers excluded. Validation passed focused backend API/task/SSE/audit tests, full backend tests, race tests, vet, API/Worker builds, full frontend lint/type-check/test/build, Docker Compose config, whitespace checks, and changed-file forbidden-pattern scan.

`P15-SECURITY-FINAL-REGRESSION` was reviewed and merged. The repository now has `scripts/security-regression.sh` as a focused final security regression entry point. It runs focused backend and frontend security tests, production forbidden-pattern scans, backend sensitive-marker scans, frontend `/api/` proxy safety checks, Docker Compose config validation, and whitespace checks. The P15 core-flow test was extended with low-permission negative assertions for output asset download, project history, usage reads, operation logs, API call logs, and API call detail. Validation passed the security regression script, full backend tests/race/vet/build, full frontend lint/type-check/test/build, Compose config, and whitespace checks. Non-blocking: cross-tenant negative coverage remains mapped primarily to existing focused tests.

`P15-DEPLOY-RUNBOOK-FINAL` was reviewed and merged. The repository now includes `deploy/RUNBOOK.md` and `scripts/deploy-release-validation.sh`. The deploy validation script supports `--help`, safe default validation, explicit `--up`, and cleanup through `--down`; it verifies Compose config, frontend `/api/` and SSE proxy safety, image builds, security regression, live health checks, frontend proxy health, and cleanup. Validation passed default deploy release validation, live `--up --down` Compose health/proxy checks, full backend tests/race/vet/build, full frontend lint/type-check/test/build, Compose config, and whitespace checks. P16 later added the cleanup trap for failed or interrupted `--up --down` runs.

R15 reviewed the complete P15 range from `3db7980..HEAD` and found no blocking release-readiness issues. Validation passed `scripts/security-regression.sh`, `scripts/deploy-release-validation.sh`, live `scripts/deploy-release-validation.sh --up --down`, post-cleanup container/volume checks, full backend tests/race/vet/build, full frontend lint/type-check/test/build, Docker Compose config, and whitespace checks. The live Compose run confirmed MySQL, Redis, MinIO, backend API, backend Worker, and frontend health; MinIO bootstrap completion; backend health endpoints; frontend `/api/` proxy health; SSE auth-boundary routing; and cleanup of project containers and volumes.

`P16-BE-LOG-RETENTION` was reviewed, fixed, and merged. Backend `logRetention` is now runtime-backed: the admin system-settings API can read/write nullable retention days for `operation_logs`, `api_call_logs`, and `task_events`, and the Worker maintenance loop consumes those settings per active tenant. Cleanup is batch-limited, tenant-scoped, fail-closed for malformed settings, preserves non-terminal task events for SSE/recovery, and records sanitized aggregate cleanup audit metadata. Validation passed focused settings/API/Worker tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, security regression, and whitespace checks.

`P16-BE-THUMBNAIL-POLICY` was reviewed, fixed, and merged. New reference uploads and Worker generated/edited outputs now generate bounded backend JPEG thumbnails, store them in the configured MinIO thumbnails bucket, persist `thumbnail_object_key`, return `/api/v1/assets/{assetId}/thumbnail` only when a thumbnail exists, and stream thumbnails through backend `asset:read` authorization. Existing assets without thumbnails remain usable with empty `thumbnailUrl`, and thumbnail bytes remain outside quota accounting until a later explicit schema/counter task.

R16 reviewed the complete P16 range from `4b1913e..HEAD` and found no blocking issues. Validation passed backend focused tests, full backend tests, race tests, vet, API/Worker builds, frontend lint/type-check/test/build, Docker Compose config, security regression, default deployment release validation, live `scripts/deploy-release-validation.sh --up --down`, and post-cleanup container/volume checks.

`P17-BE-ORPHAN-CLEANUP` was reviewed, fixed, and merged. The backend now has admin-only storage orphan scan and cleanup endpoints with dry-run default, explicit cleanup confirmation, tenant/admin permission checks, bounded MinIO listing, opaque continuation cursor, recognized backend object-key pattern checks, MySQL metadata exclusion, age gating, retry-safe delete failure handling, and sanitized aggregate operation logs. Validation passed focused storage/asset/API tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, security regression, and whitespace checks.

`P17-BE-STORAGE-QUOTA-RESERVATION` was reviewed, fixed, and merged. The backend now uses tenant-scoped quota counter/reservation tables for reference uploads and Worker generated/edited outputs. Asset-creating paths reserve original bytes before MinIO writes, finalize reservations inside metadata transactions, release on validation/storage/DB failures, keep soft-deleted-but-not-purged assets counted, decrement after physical purge, and reconcile counters from MySQL metadata rather than MinIO listing. Released or malformed reservations fail closed without successful asset/task-output side effects, and concurrent upload tests verify combined over-limit requests cannot exceed tenant quota. Validation passed focused database/settings/asset/API/task tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, security regression, and whitespace checks.

`P17-BE-OBSERVABILITY-METRICS` was reviewed, fixed, and merged. The backend now exposes `GET /api/v1/admin/diagnostics/summary` as an admin-only, tenant-scoped, read-only diagnostics endpoint requiring `audit:read`. It returns bounded aggregate sections for task statuses and recent failures, Redis queue depth, Provider/API-call failure rates, storage quota/asset usage, and recent maintenance summaries. The endpoint does not mutate state, call Providers, trigger cleanup, enqueue tasks, decrypt Provider keys, or expose Redis keys, queue payloads, object keys, buckets, MinIO URLs, signed URLs, raw Provider/log metadata, Authorization/Cookie/JWT values, Provider secrets, or image base64. Review fixes added production Redis queue inspector wiring, untruncated Provider totals, queue `reason="queue_unavailable"`, and fail-closed maintenance metadata parsing. Validation passed focused diagnostics/queue/task/settings/asset tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, security regression, and whitespace checks.

R17 reviewed the complete P17 range after merging orphan cleanup, strict quota reservation, and production diagnostics. No blocking storage-governance or observability issues were found. Validation passed full backend tests, backend race tests, vet, API/Worker builds, full frontend lint/type-check/test/build, Docker Compose config, security regression, whitespace checks, and default deployment release validation with image builds. P18 may start from latest `main`.

`P18-BE-PROVIDER-MODEL-SERIALIZATION` was reviewed and merged. Provider/model/default settings write paths now use stronger row-locking on MySQL paths, model create/update/enable/delete paths lock target rows where needed, `taskDefaults` updates lock Provider/model rows before persisting, and same-tenant same-Provider non-deleted `modelName` duplicates are rejected without a destructive unique-index migration. Validation passed focused backend Provider/model/settings/API/task tests, full backend tests, race tests, vet, API/Worker builds, Docker Compose config, security regression, and whitespace checks. Existing Provider API shape, frontend, Provider Adapter runtime, Worker/SSE/task execution, storage lifecycle, and deployment scripts were not changed.

`P18-E2E-REAL-PROVIDER-SMOKE` was reviewed and merged. The repository now has an optional, manual `scripts/real-provider-smoke.sh` entry point plus `scripts/real-provider-smoke-test.sh`. The script is safe by default, supports `--help`, `--dry-run`, and explicit `--run`, requires `REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS` before any billable path, rejects direct AI Provider API bases, bounds output count and timeout, uses only the platform `/api/v1` backend, and keeps secrets out of stdout/stderr. `deploy/RUNBOOK.md` documents manual usage without putting secrets into shell history. Validation passed the script test, default deployment release validation, security regression, Docker Compose config, dry-run/manual guard checks, and whitespace checks. No real Provider call was executed during automated validation. The R18 pre-review found that temporary-file registration inside command substitution did not reliably reach the parent-shell exit trap; `P18-PROD-DRY-RUN` must merge the cleanup hardening and failure-path regression before Go/No-Go.

## Completed Platform Capabilities

The current `main` branch supports:

- Authenticated multi-user access with tenant context and RBAC enforcement.
- Backend tenant user administration: list, create, detail, update safe fields, disable/enable, role assignment, role reads, and permission reads.
- Frontend tenant user/role administration UI gated by `user:*` and `role:*` permissions, with password inputs kept transient and write requests sent through CSRF-protected backend APIs.
- Project and project-member management foundations, including backend last-`OWNER` protection for member update/delete paths.
- MinIO-backed reference/generated/edited image assets with backend authorization and backend-generated authorized JPEG thumbnails for new assets.
- Admin Provider and model management with encrypted Provider credentials and SSRF-safe Provider URLs.
- Backend task creation, Redis queueing, Worker execution, Provider Adapter AI calls, output assets, usage records, API call logs, and SSE task updates.
- Frontend workbench submission through backend task APIs and SSE only.
- Frontend history reads backend-owned project history from `GET /projects/{projectId}/history`, with pagination, generated/edited filtering, stale-response protection, authorized detail/download, and backend `editSourceAssetId` re-edit.
- Seller workspace project/asset workflow supports project edit, reference upload, asset filtering, asset metadata edit, favorite/delete/download/detail/use-as-reference, and project member list/add/update/remove entry points through backend APIs.
- Admin observability for usage records, operation logs, API call logs, and upload-policy settings.
- Backend runtime-backed task defaults: tenant admins can store an enabled same-tenant Provider/model pair, task creation resolves it only when both IDs are omitted, and malformed persisted defaults fail closed without creation side effects.
- Backend runtime-backed task concurrency policy: tenant admins can configure tenant/user/Provider/model limits within environment hard caps, and Worker Redis semaphore acquisition consumes those effective limits before Provider execution.
- Backend storage cleanup foundation: upload rollback cleanup no longer depends on canceled request contexts, and soft-deleted image assets can be physically purged through an internal tenant-scoped, cutoff-based, idempotent cleanup service.
- Backend storage orphan cleanup: tenant admins or settings managers can run conservative dry-run scans and confirmed cleanup for MinIO objects that match backend object-key patterns and are not referenced by MySQL metadata, without exposing raw object keys or buckets.
- Backend storage quota reservation: reference uploads and Worker output persistence use tenant-scoped MySQL reservation/counter accounting to prevent concurrent writes from exceeding `storageQuota.maxBytes`; counters reconcile from `image_assets` metadata and do not use MinIO listing as quota truth.
- Backend runtime-backed storage retention policy: tenant admins can optionally set `storageRetention.deletedAssetRetentionDays`; Worker maintenance consumes it and defaults to disabled when unset or cleared.
- Backend runtime-backed storage quota policy: tenant admins can optionally set `storageQuota.maxBytes`; `storageQuota.usedBytes` is computed from tenant-scoped asset metadata, and uploads/Worker output persistence enforce the quota before creating new asset metadata.
- Frontend admin system settings UI for active runtime-backed settings only: upload policy, task defaults, task concurrency, storage retention, and storage quota.
- Backend Provider/model lifecycle integrity: Provider delete and disable are guarded against linked model states that would leave active models pointing at unavailable Providers; model/default-setting writes lock and revalidate Provider/model references; same Provider non-deleted model names are rejected by write-path checks.
- Backend deterministic usage/cost reporting: Worker usage records use stable decimal cost estimation, pricing failures are zero-cost non-fatal cases, and admin usage summary supports tenant/user/project/Provider/model aggregation with exact cost strings.
- Frontend admin cost observability: the usage tab displays tenant totals, dimension summaries, filtered usage records, drilldown filters, multi-currency cost strings from the backend, and stale-response protection without Provider direct calls, browser Provider-key storage, or polling.
- Backend core-flow E2E coverage: init admin, Provider/model setup, project/reference asset upload, task creation, fake Worker execution, SSE replay, output asset download, project history, usage, and logs are now verified in one integration path without external AI calls.
- Final security regression entry point: `scripts/security-regression.sh` consolidates focused security tests, frontend forbidden-pattern scans, backend sensitive-marker scans, Compose config validation, `/api/` proxy safety checks, and whitespace checks.
- Deployment release runbook and validation entry point: `deploy/RUNBOOK.md` documents Compose release operations, health checks, init-admin, MinIO bootstrap, SSE proxy behavior, backup/restore, upgrade/rollback, log troubleshooting, and cleanup. `scripts/deploy-release-validation.sh` automates config/build/security/health/proxy checks; backup/restore and rollback still require an explicit target-environment rehearsal.
- Optional real Provider smoke tooling: `scripts/real-provider-smoke.sh` can be run manually with explicit confirmation to validate the backend-mediated Provider/task/SSE/output path; default help/dry-run modes never call real AI Providers or consume credits.
- Docker Compose deployment topology for frontend, backend API, backend Worker, MySQL, Redis, and MinIO.

Hard platform rules remain unchanged:

- The browser must not call AI Providers directly.
- The browser must not store AI Provider API keys.
- Task state must use SSE, not polling.
- MySQL is task state truth; Redis is queue/wakeup/lock/cache infrastructure.
- MinIO stores image bytes; MySQL stores metadata and object keys only.
- Tenant filters, RBAC, object authorization, sensitive-log redaction, and Provider SSRF defense are mandatory.

## Full Platform Completion Target

The platform is not finished until these product capabilities are complete and verified:

1. Tenant, user, role, and project-member administration are usable from backend and frontend.
2. Seller-facing project workflows are polished enough for repeated use: create/select product projects, manage reference/generated/edited assets, inspect details, download, favorite, delete, and re-edit.
3. Frontend history consumes the backend unified project history endpoint instead of joining task and asset lists client-side.
4. Runtime settings have real consumers before they become writable: default Provider/model, tenant/user/model/provider concurrency, upload policy, storage quota, and retention.
5. Storage lifecycle is operational: thumbnail generation or clear thumbnail policy, orphan cleanup, retention cleanup, and safe MinIO bucket/bootstrap guidance.
6. Provider/model lifecycle and data integrity are hardened for concurrent admin operations.
7. Usage and cost reporting are accurate enough for tenant/user/project/model/provider views and future billing.
8. Security regression covers auth, tenant isolation, RBAC, object authorization, upload validation, Provider SSRF, sensitive redaction, task state transitions, SSE replay, and frontend forbidden patterns.
9. End-to-end release validation proves the core seller flow: init admin, configure Provider/model, create project, upload reference image, submit generation/edit task, receive SSE updates, view output asset, download, re-edit, and inspect logs/usage.

## Remaining Roadmap

### P11: Identity, Team, And RBAC Administration

Goal: complete the multi-user/team control plane.

Suggested order:

1. `P11-BE-USER-ROLE-ADMIN`
   - Backend tenant user CRUD, disable/enable, role assignment, role/permission reads.
   - Completed and merged. It preserves existing auth/session/RBAC behavior, blocks self-disable and last-active-admin loss, and requires `role:manage` for role assignment and `user:disable` for status changes.
2. `P11-FE-USER-ROLE-ADMIN`
   - Admin UI for users, roles, permissions, status changes, and role assignment.
   - Completed and merged. It consumes the merged backend contracts, gates UI and data loading by permissions, avoids rendering unsafe response fields, keeps created-user passwords transient, and uses `/disable`, `/enable`, and `/roles` write endpoints with CSRF.
3. `R11`
   - Completed. Full P11-range review and regression found no blocking issues.

Parallelism: P11 is complete. Move to P12 from latest `main`.

### P12: Seller Workflow And History Completion

Goal: make project/history/asset workflows coherent for daily seller use.

Suggested order:

1. `P12-FE-UNIFIED-HISTORY`
   - Switch frontend history to `GET /api/v1/projects/{projectId}/history`.
   - Fix history loading, empty, error, pagination, detail, download, and re-edit states.
   - Completed and merged. The browser no longer builds the production history feed by joining tasks and generated/edited asset lists.
2. `P12-FE-PROJECT-WORKFLOW-POLISH`
   - Improve project creation/editing/member entry points and asset management ergonomics.
   - Keep the existing UI concepts; do not rewrite the app shell.
   - Completed and merged. Project edit, asset filters, asset metadata edit, project member API entry points, and project-switch stale-state protections are now in the frontend.
3. `P12-BE-PROJECT-MEMBER-HARDENING`
   - Add missing project-member invariants such as preventing loss of the last `OWNER` where appropriate.
   - Completed and merged. Backend member update/delete paths now preserve at least one project `OWNER` and keep blocked attempts out of successful operation logs.
4. `R12`
   - End-to-end seller workflow review.
   - Completed. No blocking issues found across unified history, seller project/asset workflow, project member APIs, last-`OWNER` protection, permissions, operation logs, and forbidden frontend patterns.

Parallelism: P12 is complete. Move to P13 serially because runtime settings must not be exposed before their backend consumers exist.

### P13: Runtime Settings, Quotas, And Storage Lifecycle

Goal: make admin settings honest and operational by connecting every writable field to a runtime consumer.

Suggested order:

1. `P13-BE-RUNTIME-DEFAULTS`
   - Completed and merged. Tenant `taskDefaults.{defaultProviderId,defaultModelId}` are stored through system settings and consumed by task creation only when both `providerId` and `modelId` are omitted.
2. `P13-BE-RUNTIME-DEFAULTS-HARDENING`
   - Completed and merged. Malformed or one-sided persisted `task_defaults` now fail closed under the public task validation contract without creation side effects; explicit Provider/model submissions remain independent of corrupted unused defaults.
3. `P13-BE-CONCURRENCY-POLICY`
   - Completed and merged. `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}` is writable only within environment hard caps and is consumed by Worker Redis semaphore acquisition; global concurrency remains environment-owned.
4. `P13-BE-STORAGE-CLEANUP-FOUNDATION`
   - Completed and merged. Uploaded-object rollback uses an independent bounded cleanup context, and soft-deleted assets have an internal tenant-scoped physical purge foundation with durable `purged_at` tracking.
5. `P13-BE-STORAGE-RETENTION-RUNTIME`
   - Completed and merged. Nullable `storageRetention.deletedAssetRetentionDays` is writable only with a Worker maintenance consumer; unset/null disables automatic physical cleanup and malformed settings fail closed.
6. `P13-BE-STORAGE-QUOTA-ACCOUNTING`
   - Completed and merged. Nullable `storageQuota.maxBytes` is writable only with reference-upload and Worker-output consumers; `storageQuota.usedBytes` is read-only and computed from tenant-scoped unpurged asset metadata.
7. `P13-FE-SYSTEM-SETTINGS`
   - Completed and merged. Frontend admin UI exposes only active runtime-backed settings and sends one CSRF-protected top-level patch per settings group.
8. `R13`
   - Completed. No blocking issues found across runtime settings honesty, malformed settings fail-closed behavior, Worker-consumed concurrency and retention settings, storage quota enforcement, frontend settings safety, and forbidden frontend patterns.

Parallelism: mostly serial because settings fields must not be exposed before runtime consumers exist.

### P14: Provider, Model, And Cost Operations

Goal: harden Provider/model operations and usage/cost reporting for real operations.

Suggested order:

1. `P14-BE-PROVIDER-MODEL-INTEGRITY`
   - Completed and merged. Provider delete/disable and model create/update/enable now preserve Provider/model lifecycle integrity. P18 later tightened same-Provider `model_name` writes with transaction serialization and duplicate rejection.
2. `P14-BE-USAGE-COST-REPORTING`
   - Completed and merged. Worker cost estimation is deterministic and backend usage summary supports tenant/user/project/Provider/model dimensions with exact decimal costs.
3. `P14-FE-COST-OBSERVABILITY`
   - Completed and merged. The existing admin observability usage tab now exposes tenant totals, cost-aware filters, drilldowns, multi-currency display, and stale-response protection backed by the merged usage/cost APIs.
4. `R14`
   - Completed. Provider lifecycle, data integrity, usage/cost reporting, and frontend cost observability were reviewed and regressed with no blocking issues.

Parallelism: P14 is complete. Move to P15 from latest `main`.

### P15: Release Hardening And End-To-End QA

Goal: prepare the full platform for an operator-run server deployment.

Suggested order:

1. `P15-E2E-CORE-FLOWS`
   - Completed and merged. Automated backend integration coverage now verifies init-admin, Provider/model setup, project, reference upload, task creation, fake Worker success, SSE replay, output asset download, history, usage, and logs without external AI calls.
2. `P15-SECURITY-FINAL-REGRESSION`
   - Completed and merged. Added a reusable final security regression entry point, low-permission negative assertions in the core-flow test, frontend/backend/deploy safety scans, and explicit mapping for auth, tenant isolation, RBAC, object authorization, upload validation, Provider SSRF, sensitive redaction, task state/SSE replay, frontend forbidden patterns, and deployment config checks.
3. `P15-DEPLOY-RUNBOOK-FINAL`
   - Completed and merged. Added the deployment release validation script and operator runbook, and verified Compose config/build/up/health/proxy/down cleanup.
4. `R15`
   - Completed. Final release-readiness review passed with no blocking issues.

Parallelism: P15 is complete. Future post-R15 work should start from latest `main` after the next scope is selected.

### P16: Production Launch Hardening

Goal: remove the remaining launch-blocking operational risks before a stable production rollout.

Suggested order:

1. `P16-DEPLOY-SCRIPT-HARDENING`
   - First task. Harden `scripts/deploy-release-validation.sh --up --down` so failed live validation still attempts cleanup and does not leave project Compose containers or volumes behind.
   - Add script-level regression coverage for cleanup trap behavior without using real secrets or external AI Providers.
   - Completed and merged. The script now uses scoped cleanup traps for `--up --down`, has fake-command regression coverage for failure/signal cleanup paths, and passed live Compose `--up --down` validation with no project containers or volumes left behind.
2. `P16-BE-LOG-RETENTION`
   - Implement a real backend/Worker consumer for operation/API/task/error log retention before exposing retention as writable runtime state.
   - Cleanup must be tenant-safe where applicable, batch-limited, auditable, and sensitive-log safe.
   - Completed and merged. The implemented scope is existing database-backed logs only: `operation_logs`, `api_call_logs`, and terminal-task `task_events`. Container stdout/stderr and external log aggregation retention remain deployment responsibilities.
3. `P16-BE-THUMBNAIL-POLICY`
   - Decide and implement the production thumbnail policy.
   - Preferred target: generate MinIO thumbnail objects for uploaded references and Worker outputs, store only metadata/object keys in MySQL, and make frontend asset/history views use authorized thumbnail access.
   - Completed and merged. New reference uploads and Worker outputs generate bounded JPEG thumbnails in MinIO, persist `thumbnail_object_key`, expose authorized same-origin thumbnail URLs, keep legacy no-thumbnail assets usable, and preserve cleanup rollback behavior.
4. `R16`
   - Review production launch hardening as a batch before moving into long-running storage operations.
   - Completed. Full P16 review and regression found no blocking issues.

Parallelism: P16 is complete. Move to P17 from latest `main`.

### P17: Storage Governance And Observability

Goal: make long-running production operation safer under storage growth, cleanup drift, and queue/runtime visibility needs.

Suggested order:

1. `P17-BE-ORPHAN-CLEANUP`
   - Add MinIO orphan discovery, dry-run, execution, retry, and audit support.
   - Must be tenant-scoped by metadata, batch-limited, conservative by default, and never delete objects just because a bucket listing looks unfamiliar.
   - Completed and merged. Admin storage orphan scan/cleanup now uses dry-run by default, explicit cleanup confirmation, bounded listing, opaque cursors, age gating, metadata exclusion, sanitized audit, and retry-safe failure handling.
2. `P17-BE-STORAGE-QUOTA-RESERVATION`
   - Add strict quota reservation/counter behavior for concurrent uploads and Worker output writes.
   - Include reconciliation for counters versus metadata and clear behavior for failed reservations.
   - Completed and merged. Reference upload, Worker output persistence, physical purge accounting, stale reservation reconciliation, and fail-closed malformed state handling are active.
3. `P17-BE-OBSERVABILITY-METRICS`
   - Add admin-only production diagnostics for API/Worker health detail, queue depth, running/failed task counts, Provider failure rates, storage usage, and maintenance job results.
   - This can start as JSON diagnostics before Prometheus or external monitoring integration.
   - Completed and merged. The diagnostics endpoint is read-only, tenant-scoped, permission-gated, bounded, and aggregate-only.
4. `R17`
   - Review storage governance and observability behavior before final Provider/admin consistency work.
   - Completed. Full P17 review and regression found no blocking issues.

Parallelism: P17 is complete. Start P18 serially from latest `main`.

### P18: Production Confidence And Go/No-Go

Goal: prove the system can be operated with real Provider configuration and stable admin consistency.

Suggested order:

1. `P18-BE-PROVIDER-MODEL-SERIALIZATION`
   - Strengthen transaction serialization for Provider/model enable/disable/delete/update and default-setting interactions.
   - Revisit same-Provider `model_name` uniqueness as an explicit product/data-integrity decision.
   - Completed and merged. The write path now enforces same-Provider non-deleted modelName uniqueness without a destructive migration.
2. `P18-E2E-REAL-PROVIDER-SMOKE`
   - Add an optional, manual real Provider smoke script.
   - It must not run in default CI, must not commit real keys, and must have explicit cost controls.
   - Completed and merged. The script is opt-in, backend-only, cost-bounded, direct-Provider guarded, and covered by fake-curl safety tests.
3. `P18-PROD-DRY-RUN`
   - Execute the runbook against the target or staging server: init admin, tenant/user setup, Provider/model config, fake or real task, backup/restore, rollback, and security/deploy gates.
4. `R18-STABLE-PRODUCTION-READINESS`
   - Main-agent Go/No-Go review for stable production launch.

Parallelism: keep P18 mostly serial because real Provider smoke and production dry-run rely on all earlier hardening being merged.

## Worktree Scheduling Policy

- Public contracts and phase plans are updated by the main agent only.
- New feature work starts from latest `main`.
- Use “serial contract first -> limited parallel implementation -> serial review and integration”.
- First task in a new area should be serial. Parallelism is allowed only when write scopes are independent and shared contracts are already merged.
- Every worktree task from P11 onward must include:
  - task name
  - goal
  - allowed files
  - forbidden files
  - dependencies
  - concrete development content
  - security requirements
  - acceptance criteria
  - test commands
  - existing behavior to preserve
  - allowed intermediate state
  - forbidden half-migrated states
  - failure modes and edge cases
  - required regression tests

## Standard Regression Commands

Frontend:

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
```

Backend:

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker
```

Deployment config:

```bash
docker compose -f deploy/docker-compose.yml config
```

Full deployment validation is reserved for deployment/release tasks and must clean up the Compose stack afterwards unless the user asks to keep it.

## Current Priority

Start `P18-PROD-DRY-RUN` from latest `main`. Provider/model/default-setting serialization and optional real Provider smoke tooling have been merged. The next production risk is proving the operator runbook end to end with deployment validation, security regression, backup/restore rehearsal, optional real Provider smoke dry-run, and a sanitized Go/No-Go evidence package.
