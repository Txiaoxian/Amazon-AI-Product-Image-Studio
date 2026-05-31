# Task Queue Plan

## Principles

- Task creation is synchronous only until MySQL persistence and Redis enqueue succeed.
- MySQL is the final source of task state.
- Redis is the queue, lock, concurrency, and rate-limit layer.
- Worker execution must be idempotent.
- SSE events are persisted in MySQL before delivery.
- P7 is split into foundation, SSE, Worker queue, Provider Adapter runtime, frontend task client, and R7 review. Do not implement all concerns in one worktree.

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

Status naming note:

- `SUCCEEDED` is the canonical task status for successful completion.
- SSE event type `TASK_COMPLETED` represents the transition into `SUCCEEDED`.
- Existing frontend transitional status names must be aligned before task status is used by production UI.

## Queue design

Use Redis for durable-ish queue delivery and worker coordination. The implementation may use Redis Streams or a reliable list pattern, but it must support:

- Claiming jobs.
- Re-delivery after worker crash.
- Backoff for retry.
- Dead-letter handling.
- Visibility into pending jobs.

The queue payload should contain task ID only. Worker loads full task state from MySQL.

P7 foundation requirement:

- `P7-BE-TASK-FOUNDATION` has created the enqueue abstraction and writes task IDs to Redis after MySQL persistence.
- Enqueue failure marks the task `FAILED` with sanitized `ENQUEUE_FAILED` metadata rather than returning success for an unqueued task.
- `P7-BE-WORKER-QUEUE` has implemented reliable queue claim, visibility timeout, ack, delayed retry promotion, stale claim recovery, and dead-letter handling.

## Concurrency limits

Enforce concurrency at these dimensions:

- Global.
- Tenant.
- User.
- Provider.
- Model.

Redis semaphores or locks can enforce active counts. MySQL state must still be checked to recover after crashes.

P13 concurrency-policy contract:

- `TASK_GLOBAL_CONCURRENCY` remains deployment-owned and is never tenant writable.
- `TASK_TENANT_CONCURRENCY`, `TASK_USER_CONCURRENCY`, `TASK_PROVIDER_CONCURRENCY`, and `TASK_MODEL_CONCURRENCY` remain environment hard caps and the fallback limits when a tenant has no override.
- The tenant settings slice `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}` may only narrow or match those hard caps.
- Worker must resolve the tenant policy after loading the tenant-scoped task execution context and before acquiring Redis semaphore leases. A successfully acquired new lease uses the effective policy; existing leases are not retroactively changed.
- Provider `concurrencyLimit`, when positive, remains an additional stricter Provider-dimensional cap; the effective Provider limit is the minimum of environment cap, tenant policy, and Provider limit.
- Malformed stored concurrency policy must fail an eligible task with sanitized `TASK_CONFIGURATION_INVALID` before Provider execution, outputs, usage, or API call logging. A settings storage/infrastructure read failure must leave the task eligible for retry and must not bypass concurrency enforcement.
- Implementation status: `P13-BE-CONCURRENCY-POLICY` is merged. Future queue or worker changes must preserve this policy resolution order, fail-closed behavior, and lease-release semantics.

## Worker idempotency

Before execution, worker must:

1. Load task by ID with tenant context.
2. Check task status and attempt.
3. Transition to `RUNNING` transactionally if eligible.
4. Write `TASK_STARTED` event.

Before creating output assets, worker must prevent duplicate output records for the same task and output index.

P7 implementation boundary:

- `P7-BE-WORKER-QUEUE` validates idempotency and status transitions with fake/stub execution.
- `P7-BE-PROVIDER-ADAPTER-RUNTIME` adds real Provider calls, MinIO outputs, task_outputs, usage_records, and api_call_logs after the Worker state machine is stable.

P7 Worker queue result:

- Worker queue execution is merged and uses MySQL task state as the authority before every claim and transition.
- Redis payloads contain task ID only; Worker reloads tenant, project, Provider, model, prompt, and task parameters from MySQL.
- Worker-written events publish minimal Redis wakeups so API SSE streams can replay persisted MySQL events.
- Concurrency limits exist for global, tenant, user, Provider, and model dimensions, with stale lock cleanup.
- P10 SSE bridge lifecycle follow-up is merged: API Redis subscription lifecycle is tied to API server shutdown instead of an unbounded background context.

P7 Provider runtime result:

- Real Provider execution is now merged behind the Worker state machine.
- Successful runs create MinIO objects, generated/edited assets, `task_outputs`, `usage_records`, and `api_call_logs`, then emit `IMAGE_OUTPUT`, `USAGE_RECORDED`, and terminal events through the existing persisted-event flow.
- Provider runtime uses SSRF-safe outbound transport and recursive redaction before persistence. Review fixes closed both current API key value leakage and current API key-as-map-key leakage paths.
- The previous runtime carry-forward item after P10 worker-pool merge is resolved: API Redis subscription lifecycle is now bound to API server shutdown.

P10 Worker pool result:

- `P10-BE-WORKER-POOL` is merged.
- `WORKER_CONCURRENCY` controls the number of in-process Worker processing loops.
- Worker process concurrency is distinct from global/tenant/user/Provider/model execution limits. Worker loop count controls how many queue claims can be processed in parallel by one worker process; Redis concurrency limits still decide whether a claimed task may run.
- The worker pool preserves the existing queue contract: Redis payloads contain task IDs only, MySQL is reloaded before every state transition, queue finalization happens per claim, and duplicate claims must not duplicate output assets, usage records, API call logs, or terminal events.
- Recovery remains a single loop per Worker process so multiple processing loops do not duplicate timeout/recovery work.
- `P10-BE-SSE-BRIDGE-LIFECYCLE` is merged. `P10-BE-PROVIDER-MODEL-LIFECYCLE` is also merged; Provider deletion is now blocked while same-tenant non-deleted linked models exist.

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

P7 SSE boundary:

- `P7-BE-SSE-STREAM` consumes persisted `task_events` and live fanout only.
- Replay must use `task_events.sequence` as the cursor and emit `task_events.id` as the SSE `id`.
- MySQL remains the replay source. Redis pub/sub or in-process fanout may accelerate live delivery but cannot replace MySQL event persistence.
- The merged SSE implementation uses an API-process in-process broker plus Redis cross-process wakeups. The SSE API must still reload events from MySQL before sending them.

P10 SSE bridge lifecycle plan:

- Completed and merged.
- Redis task-event pub/sub remains a wakeup channel carrying only event sequence IDs.
- The API subscriber stops with the API server lifecycle instead of using an unbounded background context.
- Subscriber shutdown closes the Redis pub/sub path and router/API tests prove the subscriber can stop without logging `context.Canceled` as an unexpected failure.
- SSE replay semantics, heartbeat, `Last-Event-ID`, and frontend EventSource behavior did not change.

P10 Provider/model lifecycle plan:

- Provider deletion must be blocked while any non-deleted linked models exist in the same tenant.
- Soft-deleted models no longer block Provider deletion.
- Cross-tenant models must not block or reveal another tenant's Provider deletion.

P14 Provider/model lifecycle update:

- Provider disable through both `/disable` and `PATCH status=DISABLED` must be blocked while enabled same-tenant linked models exist.
- Model create/update/enable must reject disabled, deleted, or cross-tenant Providers.
- Loaded `taskDefaults` must be revalidated before task creation, so disabled/deleted Provider or model references fail closed and do not enqueue tasks.
- P18 write-path integrity rejects duplicate same-tenant same-Provider non-deleted `model_name` values while task execution continues to use stable `modelId` references.
