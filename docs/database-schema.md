# Database Schema Plan

## Principles

- MySQL 8 is the final source of truth.
- Every business table includes `tenant_id`.
- Tenant-scoped queries must include `tenant_id`.
- IDs should be non-enumerable strings or UUID-style values.
- MySQL stores image metadata and MinIO `object_key`, never image bytes.
- Soft delete is the default for user-visible business data.
- State transitions that affect tasks, outputs, usage, and events must be transactional.

## Core tables

### tenants

Stores tenant/team metadata.

Key fields: `id`, `name`, `status`, `created_at`, `updated_at`.

### users

Stores user accounts.

Key fields: `id`, `tenant_id`, `email`, `display_name`, `password_hash`, `status`, `last_login_at`, `created_at`, `updated_at`.

### roles and permissions

Tables:

- `roles`
- `permissions`
- `user_roles`
- `role_permissions`

All role assignment tables include `tenant_id`. System permissions may be seeded globally but assignments remain tenant scoped.

### projects

Stores Amazon product projects.

Key fields: `id`, `tenant_id`, `name`, `brand`, `asin`, `site`, `notes`, `status`, `created_by`, `created_at`, `updated_at`, `deleted_at`.

### project_members

Stores project-specific membership.

Key fields: `id`, `tenant_id`, `project_id`, `user_id`, `role`, `created_at`, `updated_at`.

### image_assets

Stores image metadata.

Key fields: `id`, `tenant_id`, `project_id`, `kind`, `category`, `object_key`, `thumbnail_object_key`, `mime_type`, `size_bytes`, `width`, `height`, `sha256`, `is_favorite`, `source_task_id`, `created_by`, `created_at`, `updated_at`, `deleted_at`.

`kind` values: `REFERENCE`, `GENERATED`, `EDITED`.

### prompt_templates

Stores tenant/project prompt templates.

Key fields: `id`, `tenant_id`, `project_id`, `title`, `prompt`, `created_by`, `created_at`, `updated_at`, `deleted_at`.

### ai_providers

Stores Provider configuration.

Key fields: `id`, `tenant_id`, `type`, `name`, `base_url`, `encrypted_api_key`, `api_key_hint`, `status`, `timeout_seconds`, `concurrency_limit`, `created_at`, `updated_at`.

`type` values: `OPENAI`, `GEMINI`, `OPENAI_COMPATIBLE`.

### ai_models

Stores model capability metadata.

Key fields: `id`, `tenant_id`, `provider_id`, `model_name`, `display_name`, `supports_generate`, `supports_edit`, `supports_multi_reference`, `supports_n`, `max_output_count`, `supported_sizes_json`, `supported_qualities_json`, `supported_output_formats_json`, `pricing_json`, `status`, `created_at`, `updated_at`.

### generation_tasks

Stores durable task state.

Key fields: `id`, `tenant_id`, `project_id`, `provider_id`, `model_id`, `status`, `prompt`, `image_type`, `params_json`, `input_asset_ids_json`, `attempt`, `max_attempts`, `queued_at`, `started_at`, `finished_at`, `timeout_at`, `created_by`, `error_code`, `error_message`, `created_at`, `updated_at`.

### task_events

Stores SSE-replayable task events.

Key fields: `id`, `tenant_id`, `task_id`, `project_id`, `event_type`, `event_payload_json`, `created_at`.

The `id` is the SSE `id` and must be monotonically sortable.

### task_outputs

Maps task outputs to image assets.

Key fields: `id`, `tenant_id`, `task_id`, `asset_id`, `output_index`, `created_at`.

### api_call_logs

Records AI Provider calls.

Key fields: `id`, `tenant_id`, `task_id`, `provider_id`, `model_id`, `status`, `duration_ms`, `request_id`, `http_status`, `error_code`, `error_message`, `redacted_request_json`, `redacted_response_json`, `created_at`.

Do not store API keys, Authorization headers, Cookies, or image base64.

### usage_records

Stores usage and estimated cost.

Key fields: `id`, `tenant_id`, `task_id`, `user_id`, `project_id`, `provider_id`, `model_id`, `input_tokens`, `output_tokens`, `image_count`, `estimated_cost`, `currency`, `raw_usage_json`, `created_at`.

### operation_logs

Stores audit events.

Key fields: `id`, `tenant_id`, `actor_user_id`, `action`, `resource_type`, `resource_id`, `ip`, `user_agent`, `metadata_json`, `created_at`.

### system_settings

Stores tenant-scoped settings.

Key fields: `id`, `tenant_id`, `key`, `value_json`, `created_at`, `updated_at`.

## Indexing expectations

- All business tables index `tenant_id`.
- Common filters should use compound indexes such as `(tenant_id, project_id, created_at)`.
- Task queries need indexes on `(tenant_id, status)`, `(tenant_id, project_id, created_at)`, and `(tenant_id, created_by, created_at)`.
- Task events need `(tenant_id, task_id, id)` and `(tenant_id, project_id, id)`.
