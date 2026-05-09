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

## Asset APIs

- `GET /projects/{projectId}/assets`
- `POST /projects/{projectId}/assets/uploads`: upload reference image.
- `GET /assets/{assetId}`
- `PATCH /assets/{assetId}`
- `DELETE /assets/{assetId}`
- `POST /assets/{assetId}/favorite`
- `DELETE /assets/{assetId}/favorite`
- `GET /assets/{assetId}/download`
- `POST /assets/{assetId}/edit-source`: prepare asset as edit reference.

Downloads must stream through backend authorization or use short-lived signed URLs created after authorization.

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
