# API Contract

## Base

All platform APIs use:

```text
/api/v1
```

Authentication uses HttpOnly Cookie. Frontend requests must include credentials.

## Response shape

Success:

```json
{
  "data": {},
  "requestId": "req_..."
}
```

Paginated success:

```json
{
  "data": {
    "records": [],
    "total": 0,
    "pageNum": 1,
    "pageSize": 20
  },
  "requestId": "req_..."
}
```

Error:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request.",
    "details": {}
  },
  "requestId": "req_..."
}
```

## Common HTTP semantics

- `400`: malformed request.
- `401`: not authenticated.
- `403`: authenticated but not authorized.
- `404`: resource not found or not visible to this tenant/user.
- `409`: conflict.
- `422`: validation failed.
- `429`: rate limited or concurrency limited.
- `500`: internal error with sanitized message.

## Naming and formats

- JSON fields use camelCase.
- Enum values use UPPER_SNAKE.
- Time fields use ISO 8601 strings.
- Pagination query params: `pageNum`, `pageSize`.
- Sorting query params: `sortBy`, `sortOrder`.

## Authentication APIs

- `POST /auth/init-admin`: initialize first administrator.
- `POST /auth/login`: login.
- `POST /auth/logout`: logout.
- `GET /me`: current user, tenant, roles, and permissions.
- `PATCH /me/password`: change password.

## Tenant and user APIs

- `GET /tenants/current`
- `PATCH /tenants/current`
- `GET /users`
- `POST /users`
- `GET /users/{userId}`
- `PATCH /users/{userId}`
- `POST /users/{userId}/disable`
- `POST /users/{userId}/roles`

All user APIs require tenant scope and appropriate RBAC permissions.

## RBAC APIs

- `GET /roles`
- `POST /roles`
- `PATCH /roles/{roleId}`
- `DELETE /roles/{roleId}`
- `GET /permissions`
- `PUT /roles/{roleId}/permissions`

## Project APIs

- `GET /projects`
- `POST /projects`
- `GET /projects/{projectId}`
- `PATCH /projects/{projectId}`
- `DELETE /projects/{projectId}`
- `GET /projects/{projectId}/members`
- `POST /projects/{projectId}/members`
- `PATCH /projects/{projectId}/members/{userId}`
- `DELETE /projects/{projectId}/members/{userId}`

Project object APIs require tenant scope and project membership or admin permission.

Project payload rules for P5:

- Create/update fields: `name`, `brand`, `asin`, `site`, `notes`, `status`.
- `name` is required.
- `status` values: `ACTIVE`, `ARCHIVED`.
- Response records include `id`, `tenantId`, product fields, `status`, `createdBy`, `createdAt`, `updatedAt`.
- Deleted projects are soft deleted and excluded from normal lists.

Project member rules for P5:

- Member role values: `OWNER`, `EDITOR`, `VIEWER`.
- Project member APIs require `project:member:manage` or tenant admin permission.
- Project object APIs must combine RBAC permission and project membership. For example, asset upload requires `asset:upload` and project `OWNER` or `EDITOR`.

Current P5 implementation status:

- Backend implements project CRUD, project member APIs, tenant-scoped object authorization, and operation logs.
- Frontend uses `GET /projects` and `POST /projects` for project selection and project creation in the workbench.
- Project member management is backend-ready; a richer frontend management screen can be added later without changing this contract.

## Asset APIs

- `GET /projects/{projectId}/assets`
- `POST /projects/{projectId}/assets/uploads`: upload reference image.
- `GET /assets/{assetId}`
- `PATCH /assets/{assetId}`
- `DELETE /assets/{assetId}`
- `POST /assets/{assetId}/favorite`
- `DELETE /assets/{assetId}/favorite`
- `GET /assets/{assetId}/download`
- `POST /assets/{assetId}/edit-source`: prepare asset as edit reference. Deferred until task/workbench backendization; P5 frontend must not depend on this endpoint unless it is implemented in the active branch.

Downloads must stream through backend authorization or use short-lived signed URLs created after authorization.

Asset payload rules for P5:

- Upload uses `multipart/form-data`.
- Required file field: `file`.
- Optional fields: `kind`, `category`, `filename`, `isFavorite`.
- P5 upload accepts `kind=REFERENCE` only. Generated and edited assets are created by later task/worker flows.
- Asset response fields: `id`, `tenantId`, `projectId`, `kind`, `category`, `filename`, `mimeType`, `fileSize`, `width`, `height`, `thumbnailUrl`, `previewUrl`, `isFavorite`, `createdBy`, `createdAt`, `updatedAt`.
- Asset list supports `kind`, `category`, `favorite`, `pageNum`, and `pageSize` query params.
- `PATCH /assets/{assetId}` may update `category`, `filename`, and `isFavorite`; it must not change `tenantId`, `projectId`, `objectKey`, or image dimensions.
- `DELETE /assets/{assetId}` is soft delete.
- `GET /assets/{assetId}/download` must require backend authorization and must not expose permanent public MinIO URLs.
- Current P5 backend implementation provides list, upload, detail, update, soft delete, favorite/unfavorite, and download. Edit-source remains a later workbench/backendization concern.
- Current P5 frontend implementation consumes project-scoped asset list/upload and asset detail/update/delete/favorite/download through the authenticated API client with `credentials: include`.
- P5 frontend downloads use the backend download endpoint as a blob response; the browser must not talk to MinIO directly.
- P5 frontend must not depend on `POST /assets/{assetId}/edit-source`; selecting an asset as a local reference is transition UI only until P8 backendization.

## Task APIs

- `POST /projects/{projectId}/tasks`: create generation/edit task.
- `GET /projects/{projectId}/tasks`
- `GET /tasks/{taskId}`
- `POST /tasks/{taskId}/cancel`
- `POST /tasks/{taskId}/retry`

Task creation persists the task and enqueues Redis work. Task progress must be consumed through SSE.

Task request fields for P7:

- `type`: `IMAGE_GENERATION` or `IMAGE_EDIT`.
- `prompt`: required text prompt.
- `providerId`: required Provider ID in the current tenant.
- `modelId`: required model ID in the current tenant and owned by the Provider.
- `imageType`: optional ecommerce image category such as `MAIN`, `A_PLUS`, `SCENE`, `DETAIL`, `DIMENSION`, `SELLING_POINT`, or `COMPARISON`.
- `referenceAssetIds`: optional list of project asset IDs for generation/edit references.
- `editSourceAssetId`: required for edit tasks when the selected model needs an edit source.
- `parameters`: structured object containing only model-supported values such as size, quality, output format, output count, aspect ratio, and image type settings.

Task response fields for P7:

- `id`, `tenantId`, `projectId`, `type`, `status`, `prompt`, `providerId`, `modelId`, `imageType`, `parameters`, `inputAssetIds`, `outputAssetIds`, `attempt`, `maxAttempts`, `queuedAt`, `startedAt`, `finishedAt`, `timeoutAt`, `errorCode`, `errorMessage`, `createdBy`, `createdAt`, `updatedAt`.

Task statuses for P7:

- `QUEUED`, `RUNNING`, `SUCCEEDED`, `FAILED`, `CANCELLED`, `RETRYING`, `TIMED_OUT`.

P7 task API requirements:

- Task APIs require Cookie auth and CSRF for state-changing requests.
- Project-scoped task APIs must check tenant, RBAC, and project membership or admin access.
- Task creation must validate Provider/model enabled state, same-tenant ownership, model capabilities, reference asset ownership, and parameter values before enqueue.
- The frontend must not provide or override `tenantId`, `createdBy`, `status`, `attempt`, `queuedAt`, or other server-owned fields.
- Redis enqueue payload should contain task ID only; Worker reloads full state from MySQL.
- P7 may add task API wrappers to the frontend, but P8 owns replacing the main generation workbench flow.

Current P7 implementation status:

- Backend implements task create/list/detail/cancel/retry APIs, MySQL task/event/output/log/usage schema, operation logs, and Redis enqueue abstraction.
- Task event replay cursor is `task_events.sequence`; `task_events.id` is derived from that sequence and is safe to use as the SSE `id`.
- Backend now implements SSE long connections, Worker execution, real Provider runtime execution, generated/edited output asset creation, usage records, and API call logs.
- Frontend workbench backendization is still deferred to P8; P7 frontend work only adds task API/SSE client contracts.

## Provider APIs

- `GET /providers`
- `POST /providers`
- `GET /providers/{providerId}`
- `PATCH /providers/{providerId}`
- `DELETE /providers/{providerId}`
- `POST /providers/{providerId}/test`
- `POST /providers/{providerId}/enable`
- `POST /providers/{providerId}/disable`

Provider APIs require `provider:read` or `provider:manage` as appropriate. Provider object APIs must filter by `tenant_id`; cross-tenant Provider IDs should return `404` or a non-revealing authorization failure.

Provider request fields for P6:

- `type`: `OPENAI`, `GEMINI`, or `OPENAI_COMPATIBLE`.
- `name`: display name, required.
- `baseUrl`: required for custom/OpenAI-compatible Providers; official Provider defaults may be supplied by backend config but still pass SSRF validation before use.
- `apiKey`: accepted only on create or explicit rotation/update. If omitted on update, the existing encrypted key is retained.
- `timeoutSeconds`: bounded positive integer.
- `concurrencyLimit`: bounded non-negative integer. `0` means use global/system default unless implementation chooses a stricter explicit default.
- `status`: `ENABLED` or `DISABLED`; enable/disable endpoints are the preferred state transition API.

Provider response fields for P6:

- `id`, `tenantId`, `type`, `name`, `baseUrl`, `status`, `timeoutSeconds`, `concurrencyLimit`, `apiKeyHint`, `apiKeyUpdatedAt`, `lastTestStatus`, `lastTestedAt`, `createdAt`, `updatedAt`.
- Provider responses never include full API keys, encrypted API key values, Authorization headers, or raw Provider responses.

Provider test contract for P6:

- `POST /providers/{providerId}/test` performs a backend-only connectivity/authentication probe with timeout and SSRF protection.
- It returns sanitized fields such as `status`, `durationMs`, `checkedAt`, `httpStatus`, `requestId`, and `message`.
- It must not create generation tasks, upload output assets, write usage records, or expose raw Provider payloads.
- It should write an operation log and may update sanitized `lastTest*` Provider metadata.

Current P6 Provider backend implementation status:

- Backend implements Provider CRUD, soft delete, enable/disable, Provider test, tenant-scoped queries, RBAC, operation logs, API key encryption, and masked Provider responses.
- Provider test is backend-only and does not create tasks, assets, or usage records.
- Frontend Provider/model management is implemented and merged. The UI sends Provider API keys only as immediate form submissions, displays only masked metadata, clears submitted and unsubmitted key drafts, and does not persist Provider keys in browser storage.

## Model APIs

- `GET /models`
- `POST /models`
- `GET /models/{modelId}`
- `PATCH /models/{modelId}`
- `DELETE /models/{modelId}`
- `POST /models/{modelId}/enable`
- `POST /models/{modelId}/disable`

Model APIs require `model:read` or `model:manage` as appropriate. Model object APIs must filter by `tenant_id`, and `providerId` must belong to the same tenant.

Model request fields for P6:

- `providerId`: required.
- `modelName`: Provider-facing model id/name.
- `displayName`: user-facing model name.
- `supportsGenerate`, `supportsEdit`, `supportsMultiReference`, `supportsN`.
- `maxOutputCount`: positive integer, constrained by `supportsN`.
- `supportedSizes`, `supportedQualities`, `supportedOutputFormats`: arrays of strings, validated and stored as structured JSON.
- `pricing`: structured JSON with currency and unit prices; exact Provider billing interpretation can be refined before P7 usage accounting.
- `status`: `ENABLED` or `DISABLED`; enable/disable endpoints are the preferred state transition API.

Model response fields for P6:

- `id`, `tenantId`, `providerId`, `providerName`, `modelName`, `displayName`, capability fields, `pricing`, `status`, `createdAt`, `updatedAt`.

Current P6 model backend implementation status:

- Backend implements model CRUD, soft delete, enable/disable, tenant-scoped queries, same-tenant Provider checks, RBAC, operation logs, capability validation, pricing metadata validation, and model responses for frontend dynamic parameter rendering.
- Current model list filters include status, enabled shorthand, Provider ID, and generation/edit capability filtering.
- Frontend Provider/model management is implemented and merged. Model capability forms manage generate/edit, multi-reference, `n`, max output count, supported sizes, qualities, formats, pricing metadata, and status.
- P7 must decide whether `(tenant_id, provider_id, model_name)` uniqueness is required before task execution depends on model lookup semantics.
- P7/P8 must decide how linked models behave when their Provider is soft-deleted.

Frontend uses enabled model capability fields to render dynamic parameters. P6 only manages capabilities; P8 applies those capabilities to the generation workbench after backend task creation and SSE exist.

## Usage and audit APIs

- `GET /usage/summary`
- `GET /usage/records`
- `GET /audit/operation-logs`
- `GET /audit/api-call-logs`
- `GET /system-settings`
- `PATCH /system-settings`

Audit APIs require admin permissions.
