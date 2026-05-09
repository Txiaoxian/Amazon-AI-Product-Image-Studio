# Development Plan

## P0: Documentation and Agent rules

Create planning docs and project Agent rules. Do not implement backend code. Do not move frontend files.

Deliverables:

- `docs/*.md`
- `AGENTS.md`
- `agent-instructions/*.md`

## P1: Repository structure and platform foundation

Prerequisite: mechanically move the existing frontend into `frontend/`.

Tasks:

- Update frontend scripts and paths after move.
- Add `backend/` Go module skeleton.
- Add `deploy/` skeleton.
- Add `.env.example`.
- Preserve current frontend behavior.

Verification:

- Run frontend lint, type-check, test, and build from `frontend/`.
- Run `go test ./...` once backend skeleton exists.

## P2: Auth, tenant, user, and RBAC foundation

Tasks:

- Implement tenant model.
- Implement admin initialization.
- Implement login/logout/current user.
- Implement users, roles, permissions.
- Add auth, tenant, and RBAC middleware.
- Record operation logs.

## P3: Project and asset management

Tasks:

- Implement projects and project members.
- Implement MinIO upload for reference images.
- Implement asset metadata, thumbnails, favorite, soft delete, detail, and download authorization.

## P4: Provider and model management

Tasks:

- Implement Provider CRUD.
- Encrypt API keys.
- Add SSRF-safe base URL validation.
- Implement Provider test.
- Implement model capability management.

## P5: Task queue, worker, and SSE

Tasks:

- Implement generation task creation.
- Add Redis queue.
- Add worker claim/execution/retry/timeout/cancel paths.
- Add task event persistence.
- Add SSE with heartbeat, Last-Event-ID, reconnect, and replay.
- Enforce global, tenant, user, Provider, and model concurrency limits.

## P6: Frontend backend integration

Tasks:

- Replace frontend AI direct calls with task creation API.
- Replace local task status with SSE.
- Replace local history as primary source with project assets and task history.
- Replace API key settings with Provider/model admin UI.
- Keep existing workbench UI patterns.

## P7: Usage, audit, and system settings

Tasks:

- Implement usage summaries.
- Implement API call log views.
- Implement operation audit views.
- Implement system settings for upload, storage, default Provider/model, concurrency, and retention.

## P8: Hardening and release readiness

Tasks:

- Security review.
- Upload abuse checks.
- SSRF tests.
- Tenant isolation tests.
- SSE reconnect tests.
- Docker Compose deployment verification.
- Documentation update.

## Phase boundaries

Do not skip P0 contracts. Do not implement Provider calls before API key encryption, SSRF validation, and logging redaction are defined. Do not integrate frontend task status before SSE replay behavior exists.
