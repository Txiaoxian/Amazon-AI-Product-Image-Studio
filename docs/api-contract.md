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

## Provider APIs

- `GET /providers`
- `POST /providers`
- `GET /providers/{providerId}`
- `PATCH /providers/{providerId}`
- `POST /providers/{providerId}/test`
- `POST /providers/{providerId}/enable`
- `POST /providers/{providerId}/disable`

Provider responses never include full API keys.

## Model APIs

- `GET /models`
- `POST /models`
- `GET /models/{modelId}`
- `PATCH /models/{modelId}`
- `POST /models/{modelId}/enable`
- `POST /models/{modelId}/disable`

Frontend uses enabled model capability fields to render dynamic parameters.

## Usage and audit APIs

- `GET /usage/summary`
- `GET /usage/records`
- `GET /audit/operation-logs`
- `GET /audit/api-call-logs`
- `GET /system-settings`
- `PATCH /system-settings`

Audit APIs require admin permissions.
