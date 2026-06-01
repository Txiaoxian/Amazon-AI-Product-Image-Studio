# RBAC Plan

## Principles

- RBAC is tenant scoped.
- `tenant_id` isolation is mandatory.
- RBAC does not replace object-level authorization.
- Project membership can further restrict project and asset access.

## Built-in roles

### admin

Tenant administrator.

Typical permissions:

- Manage users.
- Manage roles.
- Manage Providers and models.
- Manage system settings.
- View audit logs.
- Access all tenant projects.

### seller

Operational seller user.

Typical permissions:

- Create and manage own or assigned projects.
- Upload assets.
- Create, cancel, and retry generation tasks.
- Download visible assets.
- View usage for visible projects.

### viewer

Read-only user.

Typical permissions:

- View assigned projects.
- View assets and task history.
- Download if explicitly allowed.
- No Provider, model, user, or system settings access.

## Permission code groups

User and role:

- `user:read`
- `user:create`
- `user:update`
- `user:disable`
- `role:read`
- `role:manage`

P11 user-admin role mapping:

- User list/detail require tenant admin access or `user:read`.
- User create requires tenant admin access or `user:create`.
- Creating a user with one or more `roleIds` also requires tenant admin access or `role:manage`.
- User safe-field update requires tenant admin access or `user:update`.
- Changing user `status` through PATCH, `/disable`, or `/enable` requires tenant admin access or `user:disable`.
- Role replacement requires tenant admin access or `role:manage`.
- Role and permission reads require tenant admin access or `role:read`.
- User-admin object APIs must always filter by `tenant_id`; cross-tenant user and role IDs must not leak existence.
- The backend must reject self-disable and any update that would remove the tenant's last active admin.
- The frontend user/role admin panel must mirror these boundaries: do not load `/users` without `user:read`, do not load `/roles` or `/permissions` without `role:read`, do not submit `roleIds` without `role:manage`, do not expose status actions without `user:disable`, and disable current-user status actions in the UI.
- Created-user passwords are transient UI input only. They must not be written to localStorage, sessionStorage, IndexedDB, logs, or rendered after successful creation.

Tenant and custom-role operations:

- Additional tenant creation is an operator CLI responsibility. Tenant HTTP APIs must never infer a platform-wide super-admin from a tenant admin session.
- `GET /tenants/current` reads only the authenticated tenant.
- `PATCH /tenants/current` updates only the authenticated tenant name and requires tenant admin access plus `system:settings:manage`.
- Built-in `admin`, `seller`, and `viewer` roles are reserved. Tenant HTTP APIs must not mutate, disable, delete, or replace their grants.
- Custom role create/update/delete and permission replacement require tenant admin access or `role:manage`.
- Custom role object APIs must filter by `tenant_id`; cross-tenant IDs return the existing sanitized not-found shape.
- Custom role deletion must fail while users still reference the role. Grant replacement and successful deletion are transactional and auditable.

Project:

- `project:read`
- `project:create`
- `project:update`
- `project:delete`
- `project:member:manage`

Asset:

- `asset:read`
- `asset:upload`
- `asset:update`
- `asset:delete`
- `asset:download`

Task:

- `task:read`
- `task:create`
- `task:cancel`
- `task:retry`

Provider and model:

- `provider:read`
- `provider:manage`
- `model:read`
- `model:manage`

Audit and settings:

- `usage:read`
- `audit:read`
- `system:settings:manage`

## Object-level authorization

Every API that receives an object ID must verify:

1. Object exists.
2. Object belongs to the current tenant.
3. User has permission through role, project membership, or ownership.

Returning `404` is preferred when revealing existence would leak cross-tenant data.

## Project membership

Project members can have project-level roles such as:

- `OWNER`
- `EDITOR`
- `VIEWER`

Project role checks should combine with tenant RBAC. For example, a user needs `task:create` and project editor access to submit a task in that project.

Project member writes require tenant admin access or the relevant tenant RBAC permission plus project `OWNER` access. No member write path may leave a project without an `OWNER`: deleting or downgrading the final `OWNER` must fail with a conflict, and owner transfer must happen by adding or promoting another `OWNER` first.

Current P5 project/asset role mapping:

- Project create requires tenant RBAC `project:create`; the creator becomes project `OWNER`.
- Project read accepts tenant admin access or `project:read` plus project membership.
- Project update/delete require tenant admin access or the matching RBAC permission plus project `OWNER`.
- Asset read/download accept tenant admin access or `asset:read`/`asset:download` plus project `OWNER`, `EDITOR`, or `VIEWER`.
- Asset upload/update require tenant admin access or `asset:upload`/`asset:update` plus project `OWNER` or `EDITOR`.
- Asset delete requires tenant admin access or `asset:delete` plus project `OWNER`.
- Asset object APIs resolve `asset -> project` first, then apply tenant, RBAC, and project membership checks.

P6 Provider/model role mapping:

- Provider list/detail require `provider:read` or tenant admin access.
- Provider create/update/delete/enable/disable/test require `provider:manage` or tenant admin access.
- Model list/detail require `model:read` or tenant admin access.
- Model create/update/delete/enable/disable require `model:manage` or tenant admin access.
- Provider and model object APIs must still filter by `tenant_id`; RBAC alone is not sufficient.
- Sellers and viewers should not receive Provider/model management permissions by default.

P7 task role mapping:

- Task create requires `task:create` plus project `OWNER` or `EDITOR`, unless tenant admin.
- Task list/detail requires `task:read` plus project visibility, unless tenant admin.
- Task cancel requires `task:cancel` plus project `OWNER` or `EDITOR`, unless tenant admin.
- Task retry requires `task:retry` plus project `OWNER` or `EDITOR`, unless tenant admin.
- Task event SSE streams require the same visibility rules as task read. Event filtering must be applied per tenant and per project/task object, not only when the connection is opened.
- Worker execution uses backend service authority only; it must not bypass tenant/project checks when reading task-related assets and metadata.

P9 audit/settings role mapping:

- Usage summary and usage-record reads require tenant admin access plus `usage:read`.
- Operation-log and API-call-log list/detail reads require tenant admin access plus `audit:read`.
- Cross-tenant log/detail probes must return no rows or `404` without existence disclosure.
- System settings reads and writes must require tenant admin access plus `system:settings:manage`.

## Audit requirements

Record operation logs for:

- Role and permission changes.
- User creation, disable, and role assignment.
- User safe-field update and enable.
- Provider and model changes.
- Project member changes.
- Asset deletion and downloads when required by policy.
- Task create, cancel, retry, failure, timeout, and terminal completion when required by policy.
