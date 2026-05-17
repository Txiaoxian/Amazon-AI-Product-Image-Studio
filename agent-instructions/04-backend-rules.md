# Backend Rules

## Stack

Use Go + Gin + GORM for backend services.

Expected backend layout:

```text
backend/
  cmd/
    api/
    worker/
  internal/
    auth/
    tenant/
    rbac/
    project/
    asset/
    task/
    sse/
    provider/
    usage/
    audit/
    storage/
  migrations/
```

## API service

- Use Gin route groups under `/api/v1`.
- Add middleware for request ID, logging, panic recovery, security headers, authentication, tenant context, and RBAC.
- Use consistent response and error shapes documented in `docs/api-contract.md`.
- Validate request bodies and query params at route boundaries.
- Never expose internal stack traces or raw third-party error payloads to clients.

## Database

- Use MySQL 8 as the final source of truth.
- Use GORM with explicit model definitions and migrations.
- Every business table must include `tenant_id`.
- Tenant-scoped repositories must require tenant context and include `tenant_id` filters.
- Use transactions for state transitions involving tasks, outputs, usage, and events.

## Worker

- Worker processes jobs from Redis and persists state transitions to MySQL.
- Worker must be idempotent. Duplicate queue deliveries must not duplicate outputs, usage records, or terminal events.
- Task state transitions must follow the documented state machine.
- Cancellation, retry, timeout, and recovery must read MySQL state before acting.
- Worker, queue, auth, Provider, and other stateful backend tasks must define a failure-mode or state-transition matrix before implementation and add tests for each in-scope branch.
- If a high-risk branch is intentionally deferred, the task package and final handoff must say so explicitly.

## Redis

Redis may be used for:

- Job queues.
- Locks.
- Concurrency semaphores.
- Rate limits.
- Cache.
- Temporary task delivery acceleration.

Redis must not be treated as the final source of task state.
