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

## Current State During P13 Runtime Settings Work

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
| P13 | In progress | Runtime-backed tenant task defaults are merged; hardening, concurrency policy, storage lifecycle, frontend settings, and R13 remain. |

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

P13 review found one non-blocking hardening item that must be completed before adding further writable settings: if a `task_defaults` row is malformed or contains only one ID due to manual database mutation or legacy corruption, a default-backed task currently crosses package error boundaries as an internal error. The next serial task must make this path fail closed as `422 VALIDATION_ERROR` with no queue or audit side effects, while keeping explicit Provider/model submissions independent of unreadable defaults.

## Completed Platform Capabilities

The current `main` branch supports:

- Authenticated multi-user access with tenant context and RBAC enforcement.
- Backend tenant user administration: list, create, detail, update safe fields, disable/enable, role assignment, role reads, and permission reads.
- Frontend tenant user/role administration UI gated by `user:*` and `role:*` permissions, with password inputs kept transient and write requests sent through CSRF-protected backend APIs.
- Project and project-member management foundations, including backend last-`OWNER` protection for member update/delete paths.
- MinIO-backed reference/generated/edited image assets with backend authorization.
- Admin Provider and model management with encrypted Provider credentials and SSRF-safe Provider URLs.
- Backend task creation, Redis queueing, Worker execution, Provider Adapter AI calls, output assets, usage records, API call logs, and SSE task updates.
- Frontend workbench submission through backend task APIs and SSE only.
- Frontend history reads backend-owned project history from `GET /projects/{projectId}/history`, with pagination, generated/edited filtering, stale-response protection, authorized detail/download, and backend `editSourceAssetId` re-edit.
- Seller workspace project/asset workflow supports project edit, reference upload, asset filtering, asset metadata edit, favorite/delete/download/detail/use-as-reference, and project member list/add/update/remove entry points through backend APIs.
- Admin observability for usage records, operation logs, API call logs, and upload-policy settings.
- Backend runtime-backed task defaults: tenant admins can store an enabled same-tenant Provider/model pair, and task creation resolves that pair only when both IDs are omitted.
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
   - Review passed for normal API-produced state; malformed persisted default rows require the focused hardening task below.
2. `P13-BE-RUNTIME-DEFAULTS-HARDENING`
   - Normalize malformed or one-sided persisted `task_defaults` failures to the public task validation contract.
   - Default-backed requests must return `422 VALIDATION_ERROR` with no task, enqueue, event, or successful operation log; explicit Provider/model submissions must remain unaffected by corrupted unused defaults.
3. `P13-BE-CONCURRENCY-POLICY`
   - Tenant/user/provider/model concurrency policies loaded from settings and consumed by Worker limiters.
4. `P13-BE-STORAGE-QUOTA-RETENTION`
   - Storage quota accounting, log/asset retention policy, orphan cleanup, and independent cleanup context/job.
5. `P13-FE-SYSTEM-SETTINGS`
   - Frontend admin UI only for settings that are active and runtime-backed.
6. `R13`
   - Settings/quotas/storage lifecycle review.

Parallelism: mostly serial because settings fields must not be exposed before runtime consumers exist.

### P14: Provider, Model, And Cost Operations

Goal: harden Provider/model operations and usage/cost reporting for real operations.

Suggested order:

1. `P14-BE-PROVIDER-MODEL-INTEGRITY`
   - Stronger transaction serialization for Provider delete versus model create/update.
   - Decide and implement optional `(tenant_id, provider_id, model_name)` uniqueness if required.
2. `P14-BE-USAGE-COST-REPORTING`
   - Improve cost estimation and aggregation by tenant, user, project, Provider, and model.
3. `P14-FE-COST-OBSERVABILITY`
   - Frontend views for cost/usage trends and drilldowns.
4. `R14`
   - Review Provider lifecycle, data integrity, and cost reporting.

Parallelism: backend Provider integrity and usage reporting may run in parallel only after their contracts are frozen.

### P15: Release Hardening And End-To-End QA

Goal: prepare the full platform for an operator-run server deployment.

Suggested order:

1. `P15-E2E-CORE-FLOWS`
   - Automated or scripted coverage for init-admin, login, Provider/model setup, project, upload, task, SSE, output asset, download, edit, usage, and logs.
2. `P15-SECURITY-FINAL-REGRESSION`
   - Full security regression suite and forbidden frontend pattern scans.
3. `P15-DEPLOY-RUNBOOK-FINAL`
   - Final Docker Compose release validation, backup/restore notes, healthcheck/runbook updates, and environment checks.
4. `R15`
   - Final release-readiness review.

Parallelism: E2E and deployment docs may proceed in parallel after P13/P14 are stable; final security regression must run after all feature work is merged.

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

Run `P13-BE-RUNTIME-DEFAULTS-HARDENING` serially from latest `main` before opening any additional writable settings. It must close the malformed persisted-default error mapping found in review without changing the active settings fields or task success semantics. After that review is merged, define the exact runtime-backed concurrency policy contract before implementing it.
