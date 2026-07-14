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
- `POST /auth/captcha`: create a one-time captcha challenge.
- `GET /auth/captcha/{captchaId}/image`: read the short-lived PNG captcha image.
- `POST /auth/login`: login.
- `POST /auth/logout`: logout.
- `GET /me`: current user, tenant, roles, and permissions.
- `PATCH /me/password`: change password.

Login contract:

- The frontend login form sends `email`, `password`, and optional `captchaId` / `captchaCode`; it does not ask users to enter `tenantId`.
- `tenantId` remains an optional compatibility field for API/operator clients. A deployment with multiple active tenants should configure `AUTH_DEFAULT_TENANT_ID` for the tenant-specific login entry point.
- Captcha challenges are short-lived, stored server-side in Redis, and consumed atomically on the first validation attempt.
- An active tenant administrator may omit captcha fields. A non-admin user must submit a valid challenge when `AUTH_CAPTCHA_ENABLED=true`.
- Invalid password, missing captcha, invalid captcha, and expired captcha use the same sanitized `401 INVALID_CREDENTIALS` response; captcha and role state must not be disclosed.

## Tenant and user APIs

- `GET /tenants/current`
- `PATCH /tenants/current`
- `GET /users`
- `POST /users`
- `GET /users/{userId}`
- `PATCH /users/{userId}`
- `POST /users/{userId}/disable`
- `POST /users/{userId}/enable`
- `POST /users/{userId}/roles`
- `GET /users/{userId}/ai-access`
- `PUT /users/{userId}/ai-access`

All user APIs require tenant scope and appropriate RBAC permissions.

Tenant contract:

- Additional tenant provisioning is an operator-only CLI workflow, not an unauthenticated HTTP endpoint and not an implicit platform-super-admin API. The CLI creates one tenant, its built-in roles/grants, and one initial tenant admin transactionally.
- `GET /tenants/current` returns only the authenticated caller's tenant metadata.
- `PATCH /tenants/current` may update the current tenant `name` only. It must not accept `tenantId`, `status`, credentials, settings, or arbitrary metadata.
- Tenant metadata writes require tenant admin access plus `system:settings:manage`, use the authenticated tenant scope, and write a sanitized operation log.

P20 tenant implementation status:

- `backend/cmd/provision-tenant` implements additional-tenant provisioning as a default-safe operator CLI with explicitly confirmed transactional apply.
- `GET /tenants/current` and `PATCH /tenants/current` are implemented with authenticated tenant scope. PATCH accepts only `name`.

P11 implementation status:

- `GET /users`, `GET /users/{userId}`, `POST /users`, `PATCH /users/{userId}`, `POST /users/{userId}/disable`, `POST /users/{userId}/enable`, `POST /users/{userId}/roles`, `GET /roles`, and `GET /permissions` are implemented.
- User responses include safe public fields only: `id`, `tenantId`, `email`, `displayName`, `status`, `lastLoginAt`, timestamps, and role summaries. They must not include `passwordHash`, JWT, CSRF token, Cookie, Authorization header, or internal sensitive fields.
- `POST /users` requires `user:create`; assigning any `roleIds` during create also requires tenant admin access or `role:manage`.
- `PATCH /users/{userId}` may update safe fields such as `displayName`; changing `status` additionally requires tenant admin access or `user:disable`.
- `/disable` and `/enable` require tenant admin access or `user:disable`.
- `POST /users/{userId}/roles` requires tenant admin access or `role:manage`, validates all role IDs are active roles in the caller tenant, and replaces roles transactionally.
- The backend rejects self-disable and any operation that would remove the last active admin in a tenant.
- User creation, update, disable/enable, and role replacement write redacted operation logs.
- The frontend user/role admin UI consumes these endpoints through the shared API client, sends CSRF headers on writes, gates data loading and controls by permissions, and keeps initial passwords as transient form input only.
- `GET /users/{userId}/ai-access` and `PUT /users/{userId}/ai-access` are tenant-administrator-only. PUT transactionally replaces `modelIds`, rejects missing/cross-tenant/deleted models, and writes a sanitized operation log.
- Ordinary users do not receive self-service Provider/model grant endpoints or management controls. They only select from their assigned enabled models in generation parameters.

## RBAC APIs

- `GET /roles`
- `POST /roles`
- `PATCH /roles/{roleId}`
- `DELETE /roles/{roleId}`
- `GET /permissions`
- `PUT /roles/{roleId}/permissions`

Role-management contract:

- The built-in `admin`, `seller`, and `viewer` roles are reserved and immutable through tenant HTTP APIs. Startup reconciliation owns their required grants.
- Tenant admins may create, update, disable, and delete custom roles only within the authenticated tenant.
- `PUT /roles/{roleId}/permissions` replaces grants for one custom role transactionally using permission IDs from the global permission dictionary.
- Deleting a custom role must fail while it is assigned to any user. Successful deletion removes only that role's tenant-scoped grants and role row.
- Role write endpoints require tenant admin access or `role:manage`, enforce object-level tenant scope, use CSRF protection, and record sanitized operation logs.

P20 role-management implementation status:

- Custom-role create/update/delete and permission replacement are implemented.
- Built-in role mutation, cross-tenant role IDs, invalid permission IDs, and deletion of assigned custom roles fail without partial writes.

## Project APIs

- `GET /projects`
- `POST /projects`
- `GET /projects/{projectId}`
- `PATCH /projects/{projectId}`
- `DELETE /projects/{projectId}`
- `GET /projects/{projectId}/members`
- `GET /projects/{projectId}/member-candidates`
- `POST /projects/{projectId}/members`
- `PATCH /projects/{projectId}/members/{userId}`
- `DELETE /projects/{projectId}/members/{userId}`

Project object APIs require tenant scope and project membership or admin permission.

Project payload rules for P5:

- Create/update fields: `name`, `brand`, `asin`, `site`, `notes`, `status`, `sortOrder`.
- `name` is required.
- `status` values: `ACTIVE`, `ARCHIVED`.
- `sortOrder` is an integer used by the seller workspace project tabs. Lists sort by `sortOrder ASC` with deterministic timestamp/ID tie-breakers.
- Response records include `id`, `tenantId`, product fields, `status`, `sortOrder`, `createdBy`, `createdAt`, `updatedAt`.
- Deleted projects are soft deleted and excluded from normal lists.

Project member rules for P5:

- Member role values: `OWNER`, `EDITOR`, `VIEWER`.
- Project member APIs require `project:member:manage` or tenant admin permission.
- Project object APIs must combine RBAC permission and project membership. For example, asset upload requires `asset:upload` and project `OWNER` or `EDITOR`.
- A project must retain at least one `OWNER`. Updating or deleting the final `OWNER` returns `409 CONFLICT` and must not write a successful project-member operation log.
- Owner transfer is supported by adding or promoting another `OWNER` before downgrading or removing the original `OWNER`.
- `GET /projects/{projectId}/member-candidates` returns active same-tenant platform users that may be added to the project. It supports bounded search such as `q`, returns safe display fields only, and must not require callers to type raw user IDs.
- Project member responses include safe user display fields such as `userName`, `userEmail`, and `userStatus` so the frontend can show names instead of opaque IDs.

Current P5 implementation status:

- Backend implements project CRUD, project member APIs, tenant-scoped object authorization, operation logs, and last-`OWNER` protection for member update/delete paths.
- Frontend uses project APIs for project selection, project creation/editing, and seller workspace project-member management entry points.
- Project member management is backend-ready for daily seller workflows; later frontend polish can add more granular member mutation error states without changing this contract.
- Current seller workspace UI shows projects as top tabs with project name and brand, sorted by `sortOrder`. Dragging tabs updates project order through project update APIs. Project edit/member management is opened from a secondary management modal rather than inline raw-ID forms.

## Asset APIs

- `GET /projects/{projectId}/assets`
- `POST /projects/{projectId}/assets/uploads`: upload reference image.
- `GET /assets/{assetId}`
- `PATCH /assets/{assetId}`
- `DELETE /assets/{assetId}`
- `POST /assets/{assetId}/favorite`
- `DELETE /assets/{assetId}/favorite`
- `GET /assets/{assetId}/download`
- `GET /assets/{assetId}/thumbnail`: stream generated thumbnail image through backend authorization. The endpoint is available only when the asset has a stored `thumbnail_object_key`; it must not expose MinIO bucket names, object keys, signed URLs, or public object URLs.
- `POST /assets/{assetId}/edit-source`: optional future convenience endpoint for preparing an asset as an edit reference. P8 does not require it; the backendized workbench should submit `referenceAssetIds` and `editSourceAssetId` directly through task creation.

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
- `GET /assets/{assetId}/thumbnail` must require the same tenant/object authorization envelope as asset reads. If no thumbnail exists, return a sanitized 404 and keep `thumbnailUrl` empty in list/detail/history payloads until thumbnail generation is available for that asset.
- Current P5 backend implementation provides list, upload, detail, update, soft delete, favorite/unfavorite, and download. P8 can backendize edit flows through task creation without adding `edit-source`.
- Current P5 frontend implementation consumes project-scoped asset list/upload and asset detail/update/delete/favorite/download through the authenticated API client with `credentials: include`.
- P5 frontend downloads use the backend download endpoint as a blob response; the browser must not talk to MinIO directly.
- P5 frontend must not depend on `POST /assets/{assetId}/edit-source`; selecting an asset as a local reference is transition UI only until P8 backendization.
- P8 workbench tasks should use project asset IDs directly in `referenceAssetIds` / `editSourceAssetId`; uploaded files must enter the backend asset library before they become durable task inputs.
- P16 thumbnail policy is active: new reference uploads and Worker-created generated/edited outputs populate `thumbnailUrl` with `/api/v1/assets/{assetId}/thumbnail` only after a backend-generated thumbnail object has been stored and `thumbnail_object_key` is persisted. Existing assets without thumbnails keep an empty `thumbnailUrl` until an explicit backfill task exists.

## Admin Storage Orphan APIs

P17 introduces conservative admin-only storage orphan operations. These APIs are not a general MinIO browser and must not expose bucket names, object keys, MinIO URLs, signed URLs, image base64, Authorization, Cookie, JWT, or Provider secrets.

- `POST /admin/storage/orphans/scan`: dry-run only. It returns aggregate candidate, skipped, and error counts plus sanitized candidate samples.
- `POST /admin/storage/orphans/cleanup`: executes deletion only when the caller explicitly sets `dryRun=false` and provides a confirmation string such as `DELETE_ORPHANS`; otherwise it behaves as a dry-run.
- Required permission: tenant admin or an explicit storage/system-management permission already available to admin settings operators, such as `system:settings:manage`, until a dedicated `storage:manage` permission is introduced.
- Request fields should include `bucketKinds`, `tenantId` only for system-wide super-admin use if such a role exists, `minAgeHours`, `batchLimit`, and an optional cursor. Tenant admins must be scoped to their own tenant.
- Candidate eligibility must require both a recognized backend object-key pattern and absence from trusted MySQL metadata. Bucket listing alone is never sufficient reason to delete.
- Response samples must use hashes or opaque candidate IDs, not raw object keys.
- Cleanup must be idempotent for already-missing objects and retry-safe for storage errors. Failed deletes must remain discoverable in later scans.
- Every execution must record sanitized aggregate operation logs.
- Implementation status: merged in `P17-BE-ORPHAN-CLEANUP`. Scan/cleanup are backend-only admin APIs with dry-run default, explicit cleanup confirmation, bounded listing, opaque continuation cursor, metadata exclusion, and sanitized audit.

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
- `providerId`: Provider ID in the current tenant. Starting with P13, it may be omitted only together with `modelId`, in which case backend resolves tenant `taskDefaults`.
- `modelId`: Model ID in the current tenant and owned by the Provider. Starting with P13, it may be omitted only together with `providerId`.
- `imageType`: optional ecommerce image category such as `MAIN`, `A_PLUS`, `SCENE`, `DETAIL`, `DIMENSION`, `SELLING_POINT`, `PROMOTION`, or `COMPARISON`.
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
- Task creation must validate Provider/model enabled state, same-tenant ownership, model capabilities, reference asset ownership, and parameter values before enqueue. `IMAGE_GENERATION` with `referenceAssetIds` requires both generation and edit capability because the Worker maps reference-guided generation to the Provider image-edit operation.
- The frontend must not provide or override `tenantId`, `createdBy`, `status`, `attempt`, `queuedAt`, or other server-owned fields.
- Redis enqueue payload should contain task ID only; Worker reloads full state from MySQL.
- P7 may add task API wrappers to the frontend, but P8 owns replacing the main generation workbench flow.

Current P7 implementation status:

- Backend implements task create/list/detail/cancel/retry APIs, MySQL task/event/output/log/usage schema, operation logs, and Redis enqueue abstraction.
- Task event replay cursor is `task_events.sequence`; `task_events.id` is derived from that sequence and is safe to use as the SSE `id`.
- Backend now implements SSE long connections, Worker execution, real Provider runtime execution, generated/edited output asset creation, usage records, and API call logs.
- Frontend now has task API wrappers and SSE client/reducer contract utilities. Workbench backendization is still deferred to P8.
- P13 extends task creation with runtime-backed tenant defaults: when both Provider/model IDs are absent, backend resolves `taskDefaults` and applies the same validation and enqueue contract; explicit requests remain valid without consulting defaults.

P8 workbench contract:

- The workbench lists enabled backend models and uses capability responses to render allowed task parameters.
- Task creation remains the final validator when a selected Provider/model is disabled, deleted, or otherwise becomes unavailable after the UI loaded it.
- Browser code must not construct Provider-facing request payloads beyond the platform task contract.

## History APIs

- `GET /projects/{projectId}/history`: list backend-generated project history as a single paginated feed of generated/edited assets paired with their source task.

History query params for P10:

- `pageNum`: default `1`, positive integer.
- `pageSize`: default follows backend pagination defaults and must be capped by the same maximum page-size policy used by task/assets lists.
- `kind`: optional `GENERATED` or `EDITED`; absent means both generated and edited output assets.
- `imageType`: optional `MAIN`, `A_PLUS`, `SCENE`, `DETAIL`, `DIMENSION`, `SELLING_POINT`, `PROMOTION`, or `COMPARISON`; when present, only results created for that image type are returned.

History response fields for P10:

- Standard page envelope: `records`, `total`, `pageNum`, `pageSize`.
- Each `records[]` item contains:
  - `asset`: same safe asset response shape as `GET /assets/{assetId}`, including backend download/preview URL but never `objectKey`, MinIO URL, image bytes, or Blob data.
  - `task`: same task response shape as `GET /tasks/{taskId}`, including `outputAssetIds`.

History API requirements:

- The endpoint is read-only and requires Cookie auth.
- The endpoint must authorize the project with `task:read` and the same project-member/admin access semantics used by project task reads.
- Queries must filter by `tenant_id`, `project_id`, `image_assets.deleted_at IS NULL`, generated/edited asset kind, and same-tenant linked tasks.
- The backend must build the feed from backend-owned relationships, preferably `task_outputs -> image_assets -> generation_tasks`; client-provided task IDs, tenant IDs, or asset/task joins are never trusted.
- Sorting must be deterministic, newest output asset first, with a stable tie-breaker such as asset ID.
- Orphaned generated/edited assets without a same-tenant visible task/output link must not appear in the history feed.
- This endpoint does not create tasks, does not touch Redis/SSE, does not read or write MinIO objects, and does not expose operation/API call log metadata.
- P10 backend history query is implemented and merged.
- P12 frontend unified-history migration is implemented and merged. The production frontend history feed consumes this endpoint directly and must not rebuild the feed by joining task and generated/edited asset lists in the browser.

## Provider APIs

- `GET /providers`
- `POST /providers`
- `GET /providers/{providerId}`
- `PATCH /providers/{providerId}`
- `DELETE /providers/{providerId}`
- `POST /providers/{providerId}/test`
- `POST /providers/{providerId}/enable`
- `POST /providers/{providerId}/disable`

Provider access boundary:

- All Provider list/detail and mutation endpoints are tenant-administrator-only. A non-admin cannot gain Provider CRUD access merely through a custom `provider:*` permission.
- Ordinary users do not need Provider endpoints; the assigned model response supplies the safe `providerName` needed by the generation selector.

Provider APIs require `provider:read` or `provider:manage` as appropriate. Provider object APIs must filter by `tenant_id`; cross-tenant Provider IDs should return `404` or a non-revealing authorization failure.

Provider request fields for P6:

- `type`: `OPENAI`, `GEMINI`, or `OPENAI_COMPATIBLE`.
- `name`: display name, required.
- `baseUrl`: required for custom/OpenAI-compatible Providers; official Provider defaults may be supplied by backend config but still pass SSRF validation before use.
- `apiKey`: accepted only on create or explicit rotation/update. If omitted on update, the existing encrypted key is retained.
- `timeoutSeconds`: bounded positive integer. Current maximum is `600` seconds. The UI should explain that long timeouts are intended for slow image generation and are not a replacement for Worker/task timeout policy.
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

P10 Provider lifecycle policy:

- `DELETE /providers/{providerId}` must fail with `409 CONFLICT` when any non-deleted model in the same tenant still references the Provider.
- The response must use the standard error envelope with a non-sensitive code such as `PROVIDER_HAS_LINKED_MODELS`; it may include a linked-model count but must not leak model names from another tenant.
- Soft-deleted models do not block Provider deletion.
- Cross-tenant models must never block or reveal another tenant's Provider deletion.
- Provider deletion must not cascade-delete or cascade-disable models in this phase.
- Current P10 implementation status: this deletion policy is implemented and merged. The conflict response uses `PROVIDER_HAS_LINKED_MODELS`, and tests cover same-tenant enabled/disabled linked models, soft-deleted linked models, cross-tenant linked models, RBAC/not-found behavior, and non-leaky responses/logs. Provider disable behavior was later tightened by P14.

P14 Provider lifecycle policy:

- `POST /providers/{providerId}/disable` and `PATCH /providers/{providerId}` with `status=DISABLED` must fail with `409 CONFLICT` while same-tenant non-deleted enabled models still reference the Provider.
- The conflict response uses `PROVIDER_HAS_ENABLED_MODELS` and must not leak model names, Provider secrets, cross-tenant identifiers, Authorization headers, Cookies, or raw Provider payloads.
- Provider disable may succeed when linked same-tenant models are already disabled; it does not cascade-delete or cascade-disable models.
- Model create, model Provider migration, and model enable must reject disabled, deleted, or cross-tenant Providers. Failed writes must not record successful `provider.*` or `model.*` operation logs.
- Current P18 implementation status: Provider/model lifecycle integrity is implemented and merged. Provider/model/default-setting writes use stronger row locking, and model create/update paths reject duplicate same-tenant same-Provider non-deleted `model_name` values without a destructive unique-index migration.

## Model APIs

- `GET /models`
- `POST /models`
- `GET /models/{modelId}`
- `PATCH /models/{modelId}`
- `DELETE /models/{modelId}`
- `POST /models/{modelId}/enable`
- `POST /models/{modelId}/disable`

Model access boundary:

- Tenant administrators can list and manage all same-tenant models.
- Non-admin model list/detail reads require the existing read capability and an explicit same-tenant row in `user_model_access_grants`; an unassigned detail is returned as not found.
- Model create/update/delete/enable/disable operations are tenant-administrator-only even if a custom role contains `model:manage`.
- Task creation rechecks the selected model grant in the same transaction before any task, event, audit-success row, or queue side effect is created.

Model APIs require `model:read` or `model:manage` as appropriate. Model object APIs must filter by `tenant_id`, and `providerId` must belong to the same tenant.

Model request fields for P6:

- `providerId`: required.
- `modelName`: Provider-facing model id/name.
- `displayName`: user-facing model name.
- `supportsGenerate`, `supportsEdit`, `supportsMultiReference`, `supportsN`.
- `maxOutputCount`: positive integer, constrained by `supportsN`.
- `supportedSizes`, `supportedQualities`, `supportedOutputFormats`: arrays of strings, validated and stored as structured JSON.
- `supportedQualities` accepts normalized values for two Provider-specific meanings. OpenAI `gpt-image-2` uses the official ordered values `auto`, `low`, `medium`, `high`; the frontend displays them as “自动、低质量、中等质量、高质量”, while the adapter sends the original protocol values unchanged. Gemini output resolution remains `1k`, `2k`, `4k`, stored in lowercase and converted to Provider-required uppercase at request time. Legacy `standard`/`hd` values remain readable for existing rows but are not offered by the `gpt-image-2` preset.
- For OpenAI and OpenAI-compatible `gpt-image-2`, `supportedSizes` stores the platform's ordered aspect-ratio choices: `auto`, `1:1`, `1.62:1`, `2:3`, `3:2`, `3:4`, `4:3`, `4:5`, `5:4`, `9:16`, `16:9`, `21:9`. At Provider-call time the backend adapter converts non-auto ratios into OpenAI-compliant `WIDTHxHEIGHT` values. Existing explicit pixel values remain accepted and pass through unchanged for backward compatibility.
- Frontend model editing and generation use matching labels for those semantics: OpenAI-style `gpt-image-2` models show “图片比例 / 生成质量”; Gemini models show “画面比例 / 输出分辨率”. Presets are UI helpers only; the backend model row remains the trusted persisted capability source.
- `pricing`: structured JSON with currency and unit prices; exact Provider billing interpretation can be refined before P7 usage accounting.
- `status`: `ENABLED` or `DISABLED`; enable/disable endpoints are the preferred state transition API.

Model response fields for P6:

- `id`, `tenantId`, `providerId`, `providerName`, `providerType`, `modelName`, `displayName`, capability fields, `pricing`, `status`, `createdAt`, `updatedAt`. `providerType` lets generation parameters use the same Provider-specific labels as model configuration without guessing from legacy capability values.

Current P6 model backend implementation status:

- Backend implements model CRUD, soft delete, enable/disable, tenant-scoped queries, same-tenant Provider checks, RBAC, operation logs, capability validation, pricing metadata validation, and model responses for frontend dynamic parameter rendering.
- Current model list filters include status, enabled shorthand, Provider ID, and generation/edit capability filtering.
- Frontend Provider/model management is implemented and merged. Model capability forms manage generate/edit, multi-reference, `n`, max output count, supported sizes, qualities, formats, pricing metadata, and status.
- Current frontend Provider/model management uses Simplified Chinese labels where practical and exposes size/ratio and quality capabilities through preset checkboxes rather than free-form-only text entry.
- Current P7 task execution uses stable `modelId` references. P18 nevertheless tightens admin/data integrity by rejecting duplicate `(tenant_id, provider_id, model_name)` values for non-deleted models in write paths.
- P10 implements linked model behavior for Provider deletion: non-deleted linked models block Provider deletion; admins must soft-delete linked models first.
- P14 tightens model write behavior: create/update/enable must reject disabled, deleted, or cross-tenant Providers; task defaults loaded from persisted settings must also fail closed when the referenced Provider/model is no longer enabled and same-tenant. P18 adds row-lock serialization and rejects duplicate same-Provider non-deleted `model_name` values in write paths.

Frontend uses enabled model capability fields to render dynamic parameters. P6 only manages capabilities; P8 applies those capabilities to the generation workbench after backend task creation and SSE exist.

## Usage and audit APIs

- `GET /admin/usage/summary`
- `GET /admin/usage/records`
- `GET /admin/operation-logs`
- `GET /admin/api-call-logs`
- `GET /admin/api-call-logs/:id`
- `GET /admin/diagnostics/summary`

Current P9 backend contract:

- All routes above require tenant admin access plus the matching RBAC permission: `usage:read` for usage endpoints, `audit:read` for operation/API call logs.
- List endpoints return the standard envelope with `records`, `total`, `pageNum`, and `pageSize`.
- Shared query parameters: `pageNum`, `pageSize`, `sortBy=createdAt`, `sortOrder=asc|desc`, `createdAtFrom`, `createdAtTo`.
- Usage filters: `taskId`, `userId`, `projectId`, `providerId`, `modelId`; summary accepts `dimension=tenant|user|project|provider|model`.
- Operation log filters: `actorUserId`, `action`, `resourceType`, `resourceId`.
- API call log filters: `taskId`, `userId`, `projectId`, `providerId`, `modelId`, `status=SUCCESS|FAILURE`, `requestId`.
- Usage/raw metadata, operation metadata, API call request/response payloads, and Provider errors are recursively redacted before serialization.
- `tenantId` appears only for rows already scoped to the caller tenant; cross-tenant detail probes return `404` without existence disclosure.
- Frontend implementation status: `P9-FE-ADMIN-OBSERVABILITY-SETTINGS` consumes these routes through `frontend/src/api/admin.ts`, keeps list reads paginated, and gates visible sections by `usage:read` or `audit:read`.

P17 diagnostics contract:

- `GET /admin/diagnostics/summary` is an admin-only, read-only JSON endpoint for production diagnostics. It requires tenant admin access plus `audit:read` unless a later task deliberately adds a narrower diagnostics permission.
- The response must use the standard envelope and contain aggregate sections only:
  - `tasks`: counts by active/terminal statuses, recent failures, retrying/timeout counts, and bounded recent-failure samples with task IDs and sanitized error codes/messages only.
  - `queue`: pending/processing/delayed/dead counts and section-level availability status; it must not expose Redis keys, queue payload contents, claim IDs, or task payload bodies.
  - `providers`: bounded Provider/API-call aggregate counts and failure rates for a fixed or query-bounded time window; raw request/response metadata must not be returned.
  - `storage`: `storageQuota.maxBytes`, read-only `usedBytes`, asset counts, soft-deleted/purged counts, and cleanup/orphan aggregate state without bucket/object-key/MinIO URL exposure.
  - `maintenance`: latest sanitized aggregate operation-log summaries for storage retention, log retention, and orphan cleanup when available.
- Query parameters should be limited to safe controls such as `windowHours` and `limit`; defaults must be bounded.
- Database-backed sections are tenant-scoped. Redis or storage-inspection failures should be represented as sanitized section-level `status=unavailable` where possible rather than leaking infrastructure details.
- Diagnostics must not mutate state, enqueue tasks, run cleanup, test Providers, call AI Providers, or read/decrypt Provider API keys.
- Implementation status: merged in `P17-BE-OBSERVABILITY-METRICS`. The endpoint returns task status/recent failure aggregates, Redis queue depth with `reason="queue_unavailable"` on unavailable queue inspection, untruncated Provider/API-call totals with bounded Provider breakdowns, storage quota/asset aggregates, and fail-closed sanitized maintenance summaries. Maintenance metadata strings are not passed through generically: `status` is enum-only, `completedAt` is RFC3339/RFC3339Nano-only, numeric fields must be JSON numbers, and unknown/array/map/string payloads are dropped rather than redacted into client-visible raw content.

P14 usage/cost reporting contract:

- Backend cost estimation is deterministic and uses model `pricing.unitPrices` plus Provider usage metadata without relying on frontend-calculated costs.
- Persisted `usage_records.estimated_cost` is formatted with 8 decimal places and never negative. Missing, invalid, negative, or incomplete pricing produces zero estimated cost rather than failing an otherwise successful Provider task.
- Usage summary includes tenant-scoped aggregate view with `dimension=tenant` in addition to user, project, Provider, and model. `dimensionId` for the tenant view is the current tenant ID.
- Usage/cost queries remain tenant-scoped, paginated, stable under equal timestamps, and redacted. Raw usage may be returned only after recursive redaction.
- Summary cost strings preserve exact decimal values and do not round through float conversion. Multi-currency results are grouped by dimension and currency.
- Current P14 implementation status: backend usage/cost reporting, frontend cost observability, and R14 are merged and reviewed. The frontend admin usage tab consumes this contract for tenant totals, tenant/user/project/Provider/model summaries, filters, drilldown, multi-currency display, and usage records without client-side authoritative cost recalculation.

Current settings contract:

- `GET /admin/system-settings`
- `PATCH /admin/system-settings`

The active runtime-backed settings slices are intentionally narrow:

```json
{
  "uploadPolicy": {
    "maxFileSizeBytes": 26214400,
    "maxWidth": 8192,
    "maxHeight": 8192,
    "maxPixels": 40000000
  },
  "taskDefaults": {
    "defaultProviderId": "provider_123",
    "defaultModelId": "model_123"
  },
  "taskConcurrency": {
    "tenantLimit": 2,
    "userLimit": 2,
    "providerLimit": 2,
    "modelLimit": 2
  },
  "storageRetention": {
    "deletedAssetRetentionDays": null
  },
  "storageQuota": {
    "maxBytes": null,
    "usedBytes": 0
  },
  "logRetention": {
    "operationLogRetentionDays": null,
    "apiCallLogRetentionDays": null,
    "taskEventRetentionDays": null
  }
}
```

- Both routes require tenant admin access plus `system:settings:manage`.
- `GET /admin/system-settings` returns the effective tenant upload policy, using the environment-configured upload limits when the tenant has no override row yet.
- `PATCH /admin/system-settings` may update one or more fields under `uploadPolicy`; omitted fields keep their current effective values.
- Tenant overrides must stay positive and may only narrow or match the environment-configured upload hard caps. Runtime asset validation remains the security boundary and consumes the effective tenant policy for request-body size, dimensions, and pixel-count checks.
- Allowed MIME types remain configuration-owned security policy; the settings API must not make SVG or any non-allowlisted type writable.
- P13 has added `taskDefaults` because task creation is the runtime consumer:
  - `GET /admin/system-settings` returns `taskDefaults` with nullable `defaultProviderId` and `defaultModelId`.
  - `PATCH /admin/system-settings` may update `taskDefaults` only when both IDs are supplied together, or clear both with `null` values.
  - The backend must validate tenant ownership, enabled Provider, enabled model, and model ownership by Provider before saving defaults.
  - Task creation may omit both `providerId` and `modelId`; in that case backend resolves the tenant defaults and then runs the same Provider/model/capability validation used for explicit requests.
  - Task creation with only one of `providerId` or `modelId` omitted remains invalid to avoid ambiguous mixed-default requests.
  - If defaults are absent, stale, disabled, deleted, cross-tenant, or capability-incompatible, default-backed task creation returns validation failure and must not enqueue a task or write a successful task operation log.
  - A malformed stored `task_defaults` value, including invalid JSON, unknown fields, blank IDs, or only one populated ID, is invalid server-side configuration. A default-backed task request must fail closed as `422 VALIDATION_ERROR` without creating a task, event, enqueue operation, or successful operation log.
  - A task request that supplies both valid `providerId` and `modelId` must not depend on or fail because of an unused malformed `task_defaults` row.
- P13 has added `taskConcurrency` together with its Worker runtime consumer:
  - `GET /admin/system-settings` returns effective `tenantLimit`, `userLimit`, `providerLimit`, and `modelLimit` after the backend slice is merged.
  - `PATCH /admin/system-settings` may update positive integer fields under `taskConcurrency`; omitted fields retain the current effective values.
  - Values may only narrow or match environment-configured tenant/user/Provider/model hard caps. Global concurrency is not a tenant-visible or tenant-writable field.
  - Worker applies the effective values when acquiring a new Redis semaphore lease. A positive Provider `concurrencyLimit` remains an additional stricter Provider cap.
  - Malformed persisted `task_concurrency` configuration causes affected eligible execution to fail with a sanitized task-configuration failure before Provider calls or output/usage/API-call persistence; actual settings storage failures retry without bypassing the limiter.
- P13 storage cleanup foundation is merged as backend-internal cleanup capability:
  - Upload rollback cleanup no longer depends on a canceled request context after object write.
  - Soft-deleted image assets have an internal cleanup service with tenant, cutoff, batch, not-found idempotency, storage-error retry, and durable `purged_at` tracking.
  - It does not expose a public cleanup API.
- P13 has added `storageRetention` only together with its Worker maintenance runtime consumer:
  - `GET /admin/system-settings` returns `storageRetention.deletedAssetRetentionDays`.
  - The value is nullable. `null` means automatic physical cleanup of soft-deleted assets is disabled for the tenant.
  - No tenant override defaults to `null`; the backend must not unexpectedly enable physical deletion.
  - `PATCH /admin/system-settings` may set a positive integer day count or clear it with `null`; omitted fields retain the current value.
  - Valid range is `1..3650` days unless a later public contract deliberately changes the range.
  - Worker maintenance resolves the tenant setting, computes `cutoff = now - deletedAssetRetentionDays`, and calls the asset cleanup foundation for that tenant.
  - Malformed persisted `storage_retention` must fail closed: Worker skips cleanup for that tenant and logs only sanitized metadata. API reads/writes must return sanitized errors under the existing settings error shape.
- P13 has added `storageQuota` together with backend quota consumers:
  - `GET /admin/system-settings` returns `storageQuota.maxBytes` and read-only `storageQuota.usedBytes`.
  - `maxBytes` is nullable. `null` means no tenant storage quota is enforced.
  - `usedBytes` is computed from tenant-scoped `image_assets.size_bytes` for rows whose MinIO objects are still expected to exist. Soft-deleted but not purged rows count; purged rows do not.
  - `PATCH /admin/system-settings` may set a positive integer `maxBytes` or clear it with `null`; `usedBytes` is never writable.
  - Reference uploads and Worker output asset persistence must reject writes that would exceed the quota and must not leave successful asset metadata, successful task output events, or sensitive object identifiers in responses/logs.
  - Malformed persisted `storage_quota` must fail closed for new asset writes. Existing assets must not be deleted or hidden because of quota settings.
  - P17 quota reservation keeps this public API shape stable while adding server-side reservation/counter behavior. Internal reservation IDs, counter rows, lock keys, and reconciliation details must not be returned to the frontend.
  - `usedBytes` reflects the tenant-scoped quota counter initialized/reconciled from MySQL `image_assets` metadata. MinIO bucket listing is not quota truth.
- P16 contracts `logRetention` only together with a Worker runtime consumer:
  - `GET /admin/system-settings` returns nullable `operationLogRetentionDays`, `apiCallLogRetentionDays`, and `taskEventRetentionDays`.
  - `PATCH /admin/system-settings` may set each field to a positive integer day count or clear it with `null`; omitted fields retain their current value.
  - `null` means automatic retention cleanup for that log category is disabled.
  - Valid range is `1..3650` days unless a later public contract deliberately changes it.
  - Worker maintenance resolves active tenant settings, computes per-category cutoffs, deletes only rows older than the cutoff, limits each batch, and writes sanitized aggregate audit metadata after cleanup.
  - `taskEventRetentionDays` may delete events only for terminal tasks older than the cutoff. It must preserve events for queued/running/cancelling/retrying tasks so live SSE and recovery semantics are not broken.
  - The setting covers existing database-backed logs only: `operation_logs`, `api_call_logs`, and `task_events`. Container stdout/stderr and external log aggregation retention remain deployment responsibilities.
  - Malformed persisted `log_retention` must fail closed: Worker skips cleanup for that tenant and logs only sanitized metadata. API reads/writes must return sanitized errors under the existing settings error shape.
- Manual cleanup triggers remain absent from `system-settings`. P17 dedicated admin storage orphan endpoints have real runtime behavior and must not be mirrored as writable settings.
- Implementation status: backend `GET/PATCH /admin/system-settings` and asset-upload runtime consumption are merged in `P9-BE-RUNTIME-SETTINGS-CONTRACT`; backend `taskDefaults` write/read, task-creation runtime consumption, and malformed-row fail-closed hardening are merged in `P13-BE-RUNTIME-DEFAULTS` and `P13-BE-RUNTIME-DEFAULTS-HARDENING`; backend `taskConcurrency` read/write and Worker consumption are merged in `P13-BE-CONCURRENCY-POLICY`; backend storage cleanup foundation is merged in `P13-BE-STORAGE-CLEANUP-FOUNDATION`; backend `storageRetention` read/write and Worker maintenance consumption are merged in `P13-BE-STORAGE-RETENTION-RUNTIME`; backend `storageQuota` read/write, computed usage, reference-upload enforcement, and Worker-output enforcement are merged in `P13-BE-STORAGE-QUOTA-ACCOUNTING`; backend strict quota reservation/counter/reconciliation is merged in `P17-BE-STORAGE-QUOTA-RESERVATION`; backend `logRetention` read/write and Worker maintenance cleanup are merged in `P16-BE-LOG-RETENTION`; backend thumbnail generation and authorized thumbnail streaming are merged in `P16-BE-THUMBNAIL-POLICY`; backend admin storage orphan scan/cleanup is merged in `P17-BE-ORPHAN-CLEANUP`.
- Frontend implementation status: `P13-FE-SYSTEM-SETTINGS` exposes active runtime-backed settings: `uploadPolicy`, `taskDefaults`, `taskConcurrency`, `storageRetention`, and `storageQuota`; P19 added runtime-backed nullable `logRetention` controls after the backend consumer was merged and reviewed. Each save sends one CSRF-protected top-level patch per settings group, keeps `storageQuota.usedBytes` read-only, and keeps non-consumed settings absent from UI and requests.
