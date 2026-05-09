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

## Audit requirements

Record operation logs for:

- Role and permission changes.
- User creation, disable, and role assignment.
- Provider and model changes.
- Project member changes.
- Asset deletion and downloads when required by policy.
