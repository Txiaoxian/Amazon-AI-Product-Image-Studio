# Database Schema Plan

## Principles

- MySQL 8 is the final source of truth.
- Every business table includes `tenant_id`.
- Tenant-scoped queries must include `tenant_id`.
- IDs should be non-enumerable strings or UUID-style values.
- MySQL stores image metadata and MinIO `object_key`, never image bytes.
- Soft delete is the default for user-visible business data.
- State transitions that affect tasks, outputs, usage, and events must be transactional.
- Startup migrations must be serialized on MySQL paths with a database advisory lock. API and Worker startup must fail closed if migration lock acquisition/release fails or an applied migration's required tables, columns, or indexes are missing.

## Migration runtime policy

- MySQL startup migrations use a named advisory lock so concurrent API/Worker processes cannot apply DDL at the same time.
- SQLite/unit-test paths bypass the MySQL advisory lock but still execute the same idempotent migration runner where applicable.
- Incremental DDL must be retry-safe: already-existing columns/indexes are skipped, and failed migrations are not recorded as applied until all statements and schema checks succeed.
- After every migration, schema checks verify the required tables, columns, and indexes. A missing object on an already-applied migration is a startup-blocking integrity failure.
- Migration errors must not expose database URLs, passwords, Authorization headers, Cookies, JWTs, Provider keys, or raw SQL error payloads containing secrets.

## Core tables

### tenants

Stores tenant/team metadata.

Key fields: `id`, `name`, `status`, `created_at`, `updated_at`.

Additional tenant provisioning is an operator-only transactional workflow: create the tenant root, reconcile built-in roles/grants, create the initial tenant admin, and assign the built-in `admin` role atomically. A failed provisioning attempt must not leave a partial tenant.

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

The built-in role codes `admin`, `seller`, and `viewer` are reserved and reconciled by backend startup. Tenant HTTP APIs may manage custom roles only; they must not mutate the built-in role definitions or grants.

### projects

Stores Amazon product projects.

Key fields: `id`, `tenant_id`, `name`, `brand`, `asin`, `site`, `notes`, `status`, `created_by`, `created_at`, `updated_at`, `deleted_at`.

P5 implementation notes:

- `status` values: `ACTIVE`, `ARCHIVED`.
- `name` is required.
- `deleted_at` implements soft delete.
- Project list/detail queries must always include `tenant_id` and exclude soft-deleted rows by default.
- Suggested indexes: `(tenant_id, status, created_at)`, `(tenant_id, asin)`, `(tenant_id, deleted_at)`.

### project_members

Stores project-specific membership.

Key fields: `id`, `tenant_id`, `project_id`, `user_id`, `role`, `created_at`, `updated_at`.

P5 implementation notes:

- `role` values: `OWNER`, `EDITOR`, `VIEWER`.
- `(tenant_id, project_id, user_id)` must be unique.
- Foreign keys must include tenant scope where supported by the existing schema style.
- Project creators should become `OWNER` members transactionally.

### image_assets

Stores image metadata.

Key fields: `id`, `tenant_id`, `project_id`, `kind`, `category`, `object_key`, `thumbnail_object_key`, `mime_type`, `size_bytes`, `width`, `height`, `sha256`, `is_favorite`, `source_task_id`, `created_by`, `created_at`, `updated_at`, `deleted_at`, `purged_at`.

`kind` values: `REFERENCE`, `GENERATED`, `EDITED`.

P5 implementation notes:

- P5 upload creates `REFERENCE` assets only.
- `object_key` must be unique and non-guessable.
- `sha256` should be indexed for future deduplication checks but must not bypass tenant/project authorization.
- `deleted_at` implements soft delete. Soft-deleted assets are hidden from normal lists and downloads.
- Suggested indexes: `(tenant_id, project_id, created_at)`, `(tenant_id, project_id, kind)`, `(tenant_id, is_favorite)`, `(tenant_id, deleted_at)`.

P13 storage cleanup foundation status:

- `P13-BE-STORAGE-CLEANUP-FOUNDATION` adds nullable physical purge marker `purged_at` and index `(tenant_id, deleted_at, purged_at)`.
- Physical cleanup queries must include `tenant_id`, `deleted_at IS NOT NULL`, `deleted_at < cutoff`, and `purged_at IS NULL`.
- `purged_at` records object cleanup completion only. It must not replace `deleted_at`, and cleanup must not hard-delete `image_assets` rows.
- `thumbnail_object_key` is active for backend-generated MinIO thumbnail objects after `P16-BE-THUMBNAIL-POLICY`. New reference uploads and Worker generated/edited outputs persist this key when a bounded JPEG thumbnail has been stored. MySQL still stores only metadata/object keys, not image bytes.

### prompt_templates

Stores tenant/project prompt templates.

Key fields: `id`, `tenant_id`, `project_id`, `title`, `prompt`, `created_by`, `created_at`, `updated_at`, `deleted_at`.

### ai_providers

Stores Provider configuration.

Key fields: `id`, `tenant_id`, `type`, `name`, `base_url`, `encrypted_api_key`, `api_key_hint`, `status`, `timeout_seconds`, `concurrency_limit`, `created_at`, `updated_at`.

`type` values: `OPENAI`, `GEMINI`, `OPENAI_COMPATIBLE`.

P6 implementation notes:

- `tenant_id` is mandatory and all Provider queries must filter by it.
- `status` values: `ENABLED`, `DISABLED`.
- `encrypted_api_key` stores a versioned encrypted payload only; plaintext API keys must never be stored.
- `api_key_hint` stores non-sensitive display metadata such as the last 4 characters and must not be enough to reconstruct the key.
- Suggested additional fields: `api_key_updated_at`, `last_test_status`, `last_tested_at`, `last_test_error`, `created_by`, `deleted_at`.
- Soft delete is preferred so audit history and Provider/model references remain explainable.
- Suggested indexes: `(tenant_id, type)`, `(tenant_id, status)`, `(tenant_id, deleted_at)`, unique `(tenant_id, name, deleted_at)` if supported by the chosen MySQL strategy.
- Provider `base_url` must be SSRF-validated before insert/update and before any test/use path.

### ai_models

Stores model capability metadata.

Key fields: `id`, `tenant_id`, `provider_id`, `model_name`, `display_name`, `supports_generate`, `supports_edit`, `supports_multi_reference`, `supports_n`, `max_output_count`, `supported_sizes_json`, `supported_qualities_json`, `supported_output_formats_json`, `pricing_json`, `status`, `created_at`, `updated_at`.

P6 implementation notes:

- `tenant_id` is mandatory and all model queries must filter by it.
- `provider_id` must reference an `ai_providers` row in the same tenant.
- `status` values: `ENABLED`, `DISABLED`.
- `supported_sizes_json`, `supported_qualities_json`, `supported_output_formats_json`, and `pricing_json` must be validated structured JSON, not arbitrary unbounded blobs.
- `max_output_count` must be consistent with `supports_n`.
- Implemented additional fields: `created_by`, `deleted_at`.
- Implemented indexes include `(tenant_id, provider_id)`, `(tenant_id, status)`, `(tenant_id, provider_id, model_name)`, `(tenant_id, supports_generate)`, `(tenant_id, supports_edit)`, `(tenant_id, deleted_at)`, and `created_by`.
- Current implementation does not enforce uniqueness on `(tenant_id, provider_id, model_name)`; R7 confirmed current task execution uses `modelId`, so runtime does not require that invariant. A later management/data-integrity decision may still tighten it.
- Current implementation keeps model rows independently soft-deletable. P10 resolved Provider deletion behavior: deletion is blocked while any non-deleted same-tenant model still references that Provider; soft-deleted models do not block deletion. P14 tightened Provider disable and model write behavior: Provider disable is blocked while enabled linked models exist, disabled linked models may remain, and model create/update/enable rejects disabled, deleted, or cross-tenant Providers.
- Generated and edited task execution must not begin in P6; models are configuration data for P7/P8.

### generation_tasks

Stores durable task state.

Key fields: `id`, `tenant_id`, `project_id`, `provider_id`, `model_id`, `status`, `prompt`, `image_type`, `params_json`, `input_asset_ids_json`, `attempt`, `max_attempts`, `queued_at`, `started_at`, `finished_at`, `timeout_at`, `created_by`, `error_code`, `error_message`, `created_at`, `updated_at`.

P7 implementation notes:

- `tenant_id` is mandatory and all task queries must filter by it.
- Canonical status values: `QUEUED`, `RUNNING`, `SUCCEEDED`, `FAILED`, `CANCELLED`, `RETRYING`, `TIMED_OUT`.
- `TASK_COMPLETED` is an SSE event type; task rows use `SUCCEEDED` for successful terminal status.
- Task rows should store service-owned fields only from backend logic; clients must not provide `tenant_id`, `status`, `attempt`, timestamps, or `created_by`.
- Suggested indexes: `(tenant_id, project_id, created_at)`, `(tenant_id, status)`, `(tenant_id, created_by, created_at)`, `(tenant_id, provider_id, status)`, `(tenant_id, model_id, status)`, and `(tenant_id, timeout_at)`.

### task_events

Stores SSE-replayable task events.

Key fields: `sequence`, `id`, `tenant_id`, `task_id`, `project_id`, `event_type`, `event_payload_json`, `created_at`.

`sequence` is a MySQL `BIGINT UNSIGNED AUTO_INCREMENT` primary key and is the durable replay cursor. `id` is a unique SSE-safe event ID derived from `sequence`, formatted like `evt_00000000000000000001`.

P7 implementation notes:

- `task_events` is the replay source for SSE. Redis pub/sub or in-process fanout may only accelerate live delivery.
- Historical replay must compare by `sequence`; do not use `created_at` or random ID suffixes for replay ordering.
- Payload JSON must be bounded, structured, camelCase, and redacted.
- Implemented indexes: `(tenant_id, task_id, sequence)`, `(tenant_id, project_id, sequence)`, and `(tenant_id, sequence)`.

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

P14 usage/cost reporting keeps `estimated_cost` deterministic, non-negative, and formatted to 8 decimal places. Missing, invalid, negative, or incomplete model pricing produces zero estimated cost rather than failing otherwise successful task completion. Usage records remain queryable by tenant, user, project, Provider, model, and tenant-wide aggregate views. Summary cost output preserves exact decimal strings and groups by currency.

### operation_logs

Stores audit events.

Key fields: `id`, `tenant_id`, `actor_user_id`, `action`, `resource_type`, `resource_id`, `ip`, `user_agent`, `metadata_json`, `created_at`.

### system_settings

Stores tenant-scoped settings.

Key fields: `id`, `tenant_id`, `key`, `value_json`, `created_at`, `updated_at`.

Implementation notes:

- `(tenant_id, key)` must be unique.
- The first active key is `upload_policy`.
- `upload_policy.value_json` is a bounded object with `maxFileSizeBytes`, `maxWidth`, `maxHeight`, and `maxPixels`.
- Stored upload-policy values are tenant overrides only; effective runtime values fall back to environment-configured upload limits when no override exists.
- Tenant upload-policy overrides may only narrow or match the environment-configured hard caps and are consumed by backend asset upload validation before file persistence.
- P13 has added the active key `task_defaults` because the runtime consumer is task creation. `task_defaults.value_json` stores `defaultProviderId` and `defaultModelId` for the tenant.
- `task_defaults` rows must be tenant scoped. Task creation must revalidate the referenced Provider/model on every default-backed task create, including tenant ownership, enabled state, model ownership by Provider, and model capability support.
- Malformed, partial, or otherwise invalid stored `task_defaults` values must fail closed for default-backed task creation; they must not create tasks, events, enqueue work, or successful operation logs. Explicit Provider/model task requests must not require reading unused defaults.
- `task_concurrency` was added in `P13-BE-CONCURRENCY-POLICY` with its Worker runtime consumer. Its bounded JSON fields are `tenantLimit`, `userLimit`, `providerLimit`, and `modelLimit`.
- `task_concurrency` values are positive tenant overrides that may narrow or match the environment-configured tenant/user/Provider/model concurrency hard caps. It must not store or expose a tenant-controlled global limit.
- `storage_retention` was added in `P13-BE-STORAGE-RETENTION-RUNTIME` with its Worker maintenance consumer. Its bounded JSON field is nullable `deletedAssetRetentionDays`.
- `storage_retention.deletedAssetRetentionDays = null` means automatic physical cleanup is disabled for the tenant. A positive integer means Worker computes `cutoff = now - days` and invokes the tenant-scoped asset cleanup foundation.
- Malformed or unsupported `storage_retention` values must fail closed for Worker cleanup: skip deletion for that tenant and log sanitized metadata only.
- The next contracted key is `storage_quota`. It may be persisted only in `P13-BE-STORAGE-QUOTA-ACCOUNTING`, which must also wire the reference-upload and Worker-output quota consumers. Its bounded JSON field is nullable `maxBytes`.
- `storage_quota.maxBytes = null` means storage quota enforcement is disabled for the tenant. A positive integer means new reference uploads and generated/edited output assets must fit within the tenant quota before successful asset metadata is created.
- `P17-BE-STORAGE-QUOTA-RESERVATION` added tenant-scoped `storage_quota_counters` and `storage_quota_reservations`. They include `tenant_id`, keep MySQL `image_assets` metadata as the reconciliation source of truth, and support atomic reservation/finalization/release for concurrent reference uploads and Worker outputs.
- `storageQuota.usedBytes` is a computed API field, not stored in `system_settings`. It must be calculated from tenant-scoped `image_assets.size_bytes` where `purged_at IS NULL`; soft-deleted but not yet purged assets still count because their MinIO objects still exist.
- Malformed or unsupported `storage_quota` values must fail closed for new asset writes: reject the write with a sanitized error and do not create successful asset/task-output side effects.
- `log_retention` is active after `P16-BE-LOG-RETENTION` because it has a Worker runtime consumer. Its bounded JSON fields are nullable `operationLogRetentionDays`, `apiCallLogRetentionDays`, and `taskEventRetentionDays`.
- `log_retention.* = null` means automatic cleanup for that database-backed log category is disabled. A positive integer means Worker computes `cutoff = now - days` and deletes tenant-scoped rows older than the cutoff in bounded batches.
- `taskEventRetentionDays` cleanup must only delete events for terminal tasks older than the cutoff. It must not delete events for queued, running, cancelling, or retryable tasks because `task_events` is the SSE replay source.
- Log retention applies only to existing database-backed logs: `operation_logs`, `api_call_logs`, and `task_events`. It does not manage container stdout/stderr, external log aggregation, or MinIO object cleanup.
- Malformed or unsupported `log_retention` values must fail closed for Worker cleanup: skip deletion for that tenant and log sanitized metadata only.
- Implementation status: `P9-BE-RUNTIME-SETTINGS-CONTRACT` merged the `system_settings` model/migration and `upload_policy` runtime path. `P13-BE-RUNTIME-DEFAULTS` and `P13-BE-RUNTIME-DEFAULTS-HARDENING` merged the `task_defaults` path and malformed-row fail-closed behavior. `P13-BE-CONCURRENCY-POLICY` merged `task_concurrency` read/write and Worker consumption. `P13-BE-STORAGE-RETENTION-RUNTIME` merged `storage_retention` read/write and Worker maintenance consumption. `P13-BE-STORAGE-QUOTA-ACCOUNTING` merged `storage_quota` read/write, computed usage, and asset-write consumers. `P16-BE-LOG-RETENTION` merged `log_retention` read/write and Worker maintenance cleanup. `P16-BE-THUMBNAIL-POLICY` activated `image_assets.thumbnail_object_key` for new assets without adding image blobs to MySQL. `P17-BE-ORPHAN-CLEANUP` added admin storage orphan scan/cleanup without adding a new table. `P17-BE-STORAGE-QUOTA-RESERVATION` added quota counter/reservation tables and wired them into asset upload, Worker output persistence, physical purge accounting, and reconciliation.

## Indexing expectations

- All business tables index `tenant_id`.
- Common filters should use compound indexes such as `(tenant_id, project_id, created_at)`.
- Task queries need indexes on `(tenant_id, status)`, `(tenant_id, project_id, created_at)`, and `(tenant_id, created_by, created_at)`.
- Task events need `(tenant_id, task_id, sequence)`, `(tenant_id, project_id, sequence)`, and `(tenant_id, sequence)`.
