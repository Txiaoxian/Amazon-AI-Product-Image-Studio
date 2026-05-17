# Testing and Delivery

## General delivery rules

- State what changed, what was intentionally not changed, and how it was verified.
- Keep changes scoped to the requested phase.
- Do not mix documentation, frontend refactors, backend implementation, and deployment changes unless the task explicitly asks for that combination.
- Final handoff for a worktree task must map every required regression scenario or failure mode from the task package to the actual test file and test name that covers it.
- If an implementation cannot preserve a required existing behavior inside the allowed scope, stop and report the conflict instead of silently widening scope or shipping a half-migrated state.

## Local development environment

- Routine development verification must use the shared local services in `docs/local-development.md`.
- Use `dev-mysql8`, `dev-redis`, and `dev-minio` for local MySQL, Redis, and MinIO checks.
- Do not start project-specific MySQL, Redis, or MinIO containers for ordinary feature validation.
- Do not create project-specific Docker data volumes for ordinary feature validation.
- Do not copy real local service passwords into project files, tests, logs, or final answers.
- `deploy/docker-compose.yml` may be used for deployment-specific verification only. If it starts project-specific containers, clean them up afterwards unless the user asks to keep them:

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

## P0 documentation verification

For P0 docs and Agent rules:

```bash
find docs agent-instructions -maxdepth 2 -type f | sort
git diff -- docs AGENTS.md agent-instructions
```

No frontend build is required for P0 because source code is not changed.

## Frontend verification

Before delivering frontend code changes:

```bash
npm run lint
npm run type-check
npm run test
npm run build
```

After the frontend moves to `frontend/`, run these commands from `frontend/`.

## Backend verification

Before delivering backend code changes, run the relevant Go commands from `backend/`:

```bash
go test ./...
go test -race ./...
go vet ./...
```

For API and worker changes that need MySQL, Redis, or MinIO, connect to the shared local services from `docs/local-development.md`.

For deployment-specific changes, also verify the Docker Compose path:

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml up -d
```

After deployment-specific local verification, clean up the project Compose stack unless instructed otherwise:

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

## Contract verification

When changing API, SSE, RBAC, queue, storage, or security behavior:

- Update the matching doc in `docs/`.
- Add or update tests for the contract.
- Include manual verification examples with curl, browser, or service logs where appropriate.

## Migration verification

For migration tasks, verify all three layers:

1. Existing behavior that must still work before replacement.
2. New intermediate state that the task intentionally introduces.
3. Forbidden half-migrated states that must not appear.

Frontend migration tasks should add regression coverage for visible UI state versus actual submitted payload whenever both old and new paths coexist.

Backend high-risk tasks should add regression coverage for the state or failure matrix named in the task package, not only the happy path.
