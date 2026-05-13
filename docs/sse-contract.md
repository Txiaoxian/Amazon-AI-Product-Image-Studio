# SSE Contract

## Endpoint

```text
GET /api/v1/events/tasks
```

Query params:

- `projectId`: optional project filter.
- `taskId`: optional task filter.
- `lastEventId`: optional fallback when `Last-Event-ID` header is unavailable.

Authentication uses the normal HttpOnly Cookie.

P7 implementation scope:

- `P7-BE-SSE-STREAM` implements this endpoint after `task_events` exists.
- `P7-FE-TASK-CLIENT-SSE` may update the frontend SSE client and event types, but it must not replace the main generation workbench flow. P8 owns workbench backendization.

## Browser rules

- Frontend must use EventSource or an equivalent SSE client.
- Frontend must not poll task status.
- Frontend must not use `setInterval` or repeated fetch loops for task progress.

## Reconnect and replay

The server must support:

- `Last-Event-ID` header.
- `lastEventId` query fallback.
- Historical event replay from MySQL `task_events`.
- Heartbeat events to keep the connection alive.
- Safe reconnect after network interruption.

If a client reconnects with a known event ID, the server sends all visible events after that ID before streaming live events.

## Event frame

Example:

```text
id: evt_000000000001
event: TASK_STARTED
data: {"taskId":"task_...","status":"RUNNING","startedAt":"2026-05-09T07:00:00Z"}
```

`id` is the durable `task_events.id`.

## Event types

- `TASK_QUEUED`: task created and queued.
- `TASK_STARTED`: worker started execution.
- `TASK_PROGRESS`: optional progress update.
- `IMAGE_OUTPUT`: output image asset created.
- `USAGE_RECORDED`: usage/cost record created.
- `TASK_FAILED`: task failed.
- `TASK_COMPLETED`: task succeeded.
- `TASK_CANCELLED`: task cancelled.
- `TASK_RETRIED`: retry scheduled.
- `TASK_TIMED_OUT`: task timed out.
- `HEARTBEAT`: connection keepalive.

Status mapping:

- `TASK_COMPLETED` is an event type, not the canonical terminal task status.
- Successful task records use status `SUCCEEDED`.
- Failed/cancelled/timed-out task records use `FAILED`, `CANCELLED`, and `TIMED_OUT`.

## Payload principles

- Payloads use camelCase.
- Payloads must include `taskId`.
- Project-scoped events include `projectId`.
- Asset output events include `assetId`, `thumbnailUrl` or an authorized preview URL, dimensions, MIME type, and output index.
- Error payloads include sanitized `errorCode` and `message`.
- Payloads must not contain API keys, Authorization headers, Cookies, or image base64.

## Persistence rule

For task events:

1. Write the event to MySQL.
2. Publish or fan out to active SSE clients.

MySQL is the replay source. Redis pub/sub may accelerate live delivery but cannot be the only event source.

## Authorization

The stream only emits events visible to the authenticated user:

- Tenant must match.
- User must have project visibility or admin permission.
- Task filter must still pass object-level checks.

P7 tests must prove:

- Reconnect with `Last-Event-ID` replays only visible events after that ID.
- `lastEventId` query fallback behaves the same as the header.
- Cross-tenant and non-member project events are not emitted.
- Heartbeat frames do not leak task metadata.
