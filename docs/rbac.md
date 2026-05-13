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

## Audit requirements

Record operation logs for:

- Role and permission changes.
- User creation, disable, and role assignment.
- Provider and model changes.
- Project member changes.
- Asset deletion and downloads when required by policy.
- Task create, cancel, retry, failure, timeout, and terminal completion when required by policy.
