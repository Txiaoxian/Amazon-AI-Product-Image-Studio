# Task Queue Plan

## Principles

- Task creation is synchronous only until MySQL persistence and Redis enqueue succeed.
- MySQL is the final source of task state.
- Redis is the queue, lock, concurrency, and rate-limit layer.
- Worker execution must be idempotent.
- SSE events are persisted in MySQL before delivery.

## Task lifecycle

Statuses:

- `QUEUED`
- `RUNNING`
- `SUCCEEDED`
- `FAILED`
- `CANCELLED`
- `RETRYING`
- `TIMED_OUT`

Expected transitions:

- `QUEUED` to `RUNNING`
- `QUEUED` to `CANCELLED`
- `RUNNING` to `SUCCEEDED`
- `RUNNING` to `FAILED`
- `RUNNING` to `TIMED_OUT`
- `RUNNING` to `CANCELLED` when cancellation is honored before Provider completion
- `FAILED` to `RETRYING`
- `RETRYING` to `QUEUED`

Terminal states: `SUCCEEDED`, `FAILED`, `CANCELLED`, `TIMED_OUT`.

## Queue design

Use Redis for durable-ish queue delivery and worker coordination. The implementation may use Redis Streams or a reliable list pattern, but it must support:

- Claiming jobs.
- Re-delivery after worker crash.
- Backoff for retry.
- Dead-letter handling.
- Visibility into pending jobs.

The queue payload should contain task ID only. Worker loads full task state from MySQL.

## Concurrency limits

Enforce concurrency at these dimensions:

- Global.
- Tenant.
- User.
- Provider.
- Model.

Redis semaphores or locks can enforce active counts. MySQL state must still be checked to recover after crashes.

## Worker idempotency

Before execution, worker must:

1. Load task by ID with tenant context.
2. Check task status and attempt.
3. Transition to `RUNNING` transactionally if eligible.
4. Write `TASK_STARTED` event.

Before creating output assets, worker must prevent duplicate output records for the same task and output index.

## Cancellation

Cancellation request:

1. Checks tenant and object authorization.
2. Marks eligible task cancelled or cancellation requested.
3. Writes `TASK_CANCELLED` event when terminal cancellation is reached.

If a Provider call cannot be interrupted, worker must ignore Provider output if MySQL state is already terminal cancelled.

## Retry

Retry creates a new attempt for eligible failed, timed out, or cancelled tasks when allowed by policy. Retry must preserve original prompt and parameters unless the API explicitly accepts replacements.

## Timeout and recovery

Tasks have `timeout_at`. A recovery loop should:

- Mark overdue running tasks as `TIMED_OUT`.
- Release stale concurrency locks.
- Requeue safe tasks when status and attempt allow.

## SSE events

Every meaningful transition writes to `task_events`:

- Queued.
- Started.
- Progress.
- Output created.
- Usage recorded.
- Failed.
- Completed.
- Cancelled.
- Retried.
- Timed out.
