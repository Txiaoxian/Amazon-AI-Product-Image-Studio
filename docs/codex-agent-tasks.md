# Codex Agent Task Plan

## Purpose

This file is the current worktree planning entry point. It intentionally keeps historical P0-P10 details short because the full task history is preserved in git. Future agents should use this file plus `docs/development-plan.md` to generate the next task package, not copy old phase text forward.

## Scheduling Model

The project uses:

1. Main agent freezes or updates public contracts.
2. A small number of child-agent worktrees implement independent slices.
3. Main agent reviews, merges, runs regression, and updates docs.

For a new phase, start serially until shared backend/API contracts are stable. Use limited parallelism only when file scopes and runtime contracts do not overlap.

## Files Only The Main Agent May Modify

Child agents must not modify:

- `AGENTS.md`
- `agent-instructions/**`
- `docs/architecture.md`
- `docs/business-requirements.md`
- `docs/database-schema.md`
- `docs/api-contract.md`
- `docs/sse-contract.md`
- `docs/rbac.md`
- `docs/provider-adapter.md`
- `docs/task-queue.md`
- `docs/storage.md`
- `docs/security.md`
- `docs/deployment.md`
- `docs/local-development.md`
- `docs/development-plan.md`
- `docs/codex-agent-tasks.md`

If a child agent finds a contract gap or contradiction, it must report it in its final handoff instead of editing public contracts.

## Mandatory Task Package Sections

Every new worktree task from P11 onward must include:

- task name
- goal
- recommended thread name
- recommended branch name
- starting branch
- complete child-agent startup prompt
- allowed files
- forbidden files
- dependencies
- concrete development content
- security requirements
- acceptance criteria
- test commands
- existing behavior that must be preserved
- allowed intermediate state
- forbidden half-migrated states
- failure modes and edge cases
- regression tests that must be added or updated

High-risk backend tasks must include a failure-mode matrix. Migration tasks must explicitly describe the old path, allowed intermediate state, and target path.

## Child-Agent Handoff Requirements

A child-agent final response must include:

- files changed
- tests run and results
- mapping from required regression scenarios to concrete test files/test names
- security self-check
- intentionally unchanged areas
- contract gaps or decisions needed from the main agent

If the task cannot be completed without breaking a required existing behavior or widening forbidden scope, the child agent must stop and report the conflict.

## Current Status

R10 is complete and found no blocking issues.

Completed platform foundations:

- P0-P3: documentation, monorepo layout, backend/frontend/deploy skeletons, Docker runtime repair.
- P4-P6: database, tenants, auth, RBAC, projects, assets, Provider/model management.
- P7-P8: task queue, Worker, Provider Adapter runtime, SSE, and frontend backendization.
- P9-P10: audit/usage reads, runtime-backed upload policy, security/deploy validation, Worker pool, SSE bridge lifecycle, Provider/model lifecycle, admin hardening, and backend unified history query.

Current known follow-ups:

- Frontend history should consume `GET /api/v1/projects/{projectId}/history`.
- Full user/role/tenant administration is not yet implemented as a product surface.
- Runtime settings beyond upload policy must not become writable until their runtime consumers exist.
- Storage cleanup, retention, quota, thumbnail policy, and orphan cleanup still need operational implementation.
- Provider/model concurrent admin operations may need stronger transaction serialization.
- Final E2E and release validation still need a full seller-flow pass.

## Recommended Remaining Phases

### P11: Identity, Team, And RBAC Administration

Goal: make user, role, permission, and tenant/team administration usable.

Recommended tasks:

1. `P11-BE-USER-ROLE-ADMIN`
   - Backend user admin, disable/enable, role assignment, role/permission reads.
   - Serial first task because it defines contracts used by frontend admin UI.
2. `P11-FE-USER-ROLE-ADMIN`
   - Frontend admin UI for user and role administration.
   - Starts only after backend task is reviewed and merged.
3. `R11`
   - Main-agent review and regression.

### P12: Seller Workflow And History Completion

Goal: finish the daily seller-facing project/history/asset workflow.

Recommended tasks:

1. `P12-FE-UNIFIED-HISTORY`
   - Switch frontend history to backend unified history endpoint.
2. `P12-FE-PROJECT-WORKFLOW-POLISH`
   - Improve project selection/edit/member/asset ergonomics.
3. `P12-BE-PROJECT-MEMBER-HARDENING`
   - Add project-member invariants such as last-owner protection if product rules require it.
4. `R12`
   - Seller workflow review and regression.

### P13: Runtime Settings, Quotas, And Storage Lifecycle

Goal: expose only settings that have real runtime consumers and make storage lifecycle operational.

Recommended tasks:

1. `P13-BE-RUNTIME-DEFAULTS`
   - Decide and implement default Provider/model behavior or keep explicit-only task creation.
2. `P13-BE-CONCURRENCY-POLICY`
   - Load tenant/user/provider/model concurrency settings into Worker limiters.
3. `P13-BE-STORAGE-QUOTA-RETENTION`
   - Storage quota, retention, orphan cleanup, and independent cleanup context/job.
4. `P13-FE-SYSTEM-SETTINGS`
   - Admin UI for active runtime-backed settings only.
5. `R13`
   - Settings and storage lifecycle review.

### P14: Provider, Model, Usage, And Cost Operations

Goal: harden operational integrity and make usage/cost reporting useful.

Recommended tasks:

1. `P14-BE-PROVIDER-MODEL-INTEGRITY`
   - Stronger serialization for Provider delete versus model create/update; optional model-name uniqueness decision.
2. `P14-BE-USAGE-COST-REPORTING`
   - Improve usage/cost aggregation by tenant, user, project, Provider, and model.
3. `P14-FE-COST-OBSERVABILITY`
   - Frontend cost/usage dashboard and drilldowns.
4. `R14`
   - Provider lifecycle and cost reporting review.

### P15: Release Hardening And End-To-End QA

Goal: prove the complete platform is deployable and usable.

Recommended tasks:

1. `P15-E2E-CORE-FLOWS`
   - End-to-end core seller and admin flows.
2. `P15-SECURITY-FINAL-REGRESSION`
   - Final forbidden-pattern scans and security regression suite.
3. `P15-DEPLOY-RUNBOOK-FINAL`
   - Compose release validation, runbook, backup/restore notes, and healthcheck finalization.
4. `R15`
   - Final release-readiness review.

## Next Task To Generate

Unless the user chooses otherwise, generate `P11-BE-USER-ROLE-ADMIN` next.

Recommended branch:

```text
codex/p11-backend-user-role-admin
```

Recommended starting branch:

```text
latest main
```

Serial/parallel policy:

- Start P11 serially with the backend task.
- Do not start frontend user/role admin until backend contracts are reviewed and merged.
- Do not start P13 writable settings work until each setting field has a named runtime consumer in scope.

## Standard Verification Commands

Frontend tasks:

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
```

Backend tasks:

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

Deployment runtime checks may use `deploy/docker-compose.yml`, but must clean up with:

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```
