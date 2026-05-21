# Development Plan

## Development Environment Rule

Routine development and feature verification must use the existing shared local services documented in `docs/local-development.md`:

- MySQL: `dev-mysql8`
- Redis: `dev-redis`
- MinIO: `dev-minio`

Do not create project-specific MySQL, Redis, or MinIO containers for ordinary feature work. `deploy/docker-compose.yml` is reserved for deployment verification; if it starts project containers, clean them up afterwards unless the user explicitly asks to keep them:

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

## Current State After R10

The project has moved from a pure frontend local app to a backend-backed multi-user platform foundation.

Completed phases:

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

R10 found no blocking issues. Full frontend validation, backend validation, API/Worker builds, Docker Compose config, and static security scans passed.

## Completed Platform Capabilities

The current `main` branch supports:

- Authenticated multi-user access with tenant context and RBAC enforcement.
- Project and project-member management foundations.
- MinIO-backed reference/generated/edited image assets with backend authorization.
- Admin Provider and model management with encrypted Provider credentials and SSRF-safe Provider URLs.
- Backend task creation, Redis queueing, Worker execution, Provider Adapter AI calls, output assets, usage records, API call logs, and SSE task updates.
- Frontend workbench submission through backend task APIs and SSE only.
- Admin observability for usage records, operation logs, API call logs, and upload-policy settings.
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
   - Must preserve existing auth/session/RBAC behavior.
2. `P11-FE-USER-ROLE-ADMIN`
   - Admin UI for users, roles, permissions, status changes, and role assignment.
   - Must not expose password hashes, JWTs, CSRF tokens, or tenant internals.
3. `R11`
   - Review and regression for tenant isolation, RBAC, auth edge cases, and frontend admin UX.

Parallelism: start serially with backend contracts. Frontend may start only after backend APIs are reviewed and merged.

### P12: Seller Workflow And History Completion

Goal: make project/history/asset workflows coherent for daily seller use.

Suggested order:

1. `P12-FE-UNIFIED-HISTORY`
   - Switch frontend history to `GET /api/v1/projects/{projectId}/history`.
   - Fix history loading, empty, error, pagination, detail, download, and re-edit states.
2. `P12-FE-PROJECT-WORKFLOW-POLISH`
   - Improve project creation/editing/member entry points and asset management ergonomics.
   - Keep the existing UI concepts; do not rewrite the app shell.
3. `P12-BE-PROJECT-MEMBER-HARDENING`
   - Add missing project-member invariants such as preventing loss of the last `OWNER` where appropriate.
4. `R12`
   - End-to-end seller workflow review.

Parallelism: `P12-FE-UNIFIED-HISTORY` and `P12-BE-PROJECT-MEMBER-HARDENING` can run in parallel after P11 if their file scopes do not overlap. Project workflow polish should follow history migration.

### P13: Runtime Settings, Quotas, And Storage Lifecycle

Goal: make admin settings honest and operational by connecting every writable field to a runtime consumer.

Suggested order:

1. `P13-BE-RUNTIME-DEFAULTS`
   - Default Provider/model settings consumed by task creation when the request omits explicit IDs, or keep them unavailable if the product chooses explicit-only submission.
2. `P13-BE-CONCURRENCY-POLICY`
   - Tenant/user/provider/model concurrency policies loaded from settings and consumed by Worker limiters.
3. `P13-BE-STORAGE-QUOTA-RETENTION`
   - Storage quota accounting, log/asset retention policy, orphan cleanup, and independent cleanup context/job.
4. `P13-FE-SYSTEM-SETTINGS`
   - Frontend admin UI only for settings that are active and runtime-backed.
5. `R13`
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

Start P11 with `P11-BE-USER-ROLE-ADMIN`. Do not start P12 frontend history or P13 settings until P11 backend contracts are reviewed and merged, unless the user explicitly chooses a different sequence.
