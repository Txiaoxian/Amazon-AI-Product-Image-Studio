# Testing and Delivery

## General delivery rules

- State what changed, what was intentionally not changed, and how it was verified.
- Keep changes scoped to the requested phase.
- Do not mix documentation, frontend refactors, backend implementation, and deployment changes unless the task explicitly asks for that combination.

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

For API and worker changes, also verify the Docker Compose path once available:

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml up -d
```

## Contract verification

When changing API, SSE, RBAC, queue, storage, or security behavior:

- Update the matching doc in `docs/`.
- Add or update tests for the contract.
- Include manual verification examples with curl, browser, or service logs where appropriate.
