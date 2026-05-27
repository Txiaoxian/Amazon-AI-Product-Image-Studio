# Storage Plan

## Principles

- MinIO stores all image bytes.
- MySQL stores metadata and `object_key`.
- MySQL never stores image blobs.
- Downloads require backend authorization.
- SVG upload is forbidden.

## Buckets

Recommended buckets:

- `product-images`: original reference, generated, and edited images.
- `product-image-thumbnails`: generated thumbnails.

Buckets may be separated by environment using prefixes or environment-specific bucket names.

P5 should use the existing MinIO environment variables and the shared local `dev-minio` service for routine validation. Do not create project-specific MinIO containers or volumes for ordinary P5 development.

Current P5 implementation uses the configured originals bucket, defaulting to `product-originals`, for uploaded reference images. Bucket creation is not performed by request handlers; shared local and deployment environments must create or verify required buckets before upload tests.

## Object key naming

Use deterministic, non-guessable object keys:

```text
tenants/{tenantId}/projects/{projectId}/assets/{assetId}/original.{ext}
tenants/{tenantId}/projects/{projectId}/assets/{assetId}/thumb.{ext}
```

Never trust user file names for object keys. Original file names may be stored as metadata after sanitization.

## Upload validation

Validate:

- Declared MIME type.
- Magic bytes.
- Extension after MIME validation.
- File size.
- Width and height.
- Pixel count.

Allowed types:

- JPEG.
- PNG.
- WebP.

Forbidden:

- SVG.
- Unknown binary files.
- Files with image extensions but invalid image magic.

P5 validation must happen before any object is written. If validation fails, no image metadata row or MinIO object should remain. If a DB write fails after object upload, the implementation must either delete the just-uploaded object or record enough information for deterministic cleanup.

Current P5 backend behavior validates before object write and attempts to delete the uploaded object if metadata persistence fails. P13 storage cleanup foundation must move this cleanup to an independent bounded context or equivalent background cleanup path so request cancellation cannot prevent cleanup.

Current P5 frontend behavior uploads reference images through the backend multipart endpoint only. The browser never uploads directly to MinIO, and frontend MIME/size checks are only UX hints; backend validation is authoritative.

## Metadata

`image_assets` stores:

- Tenant ID.
- Project ID.
- Asset kind.
- Category.
- Object key.
- Thumbnail object key.
- MIME type.
- Size.
- Width and height.
- SHA-256.
- Favorite flag.
- Source task ID.
- Created by.

## Download

Downloads must:

1. Authenticate user.
2. Check tenant.
3. Check project or asset permission.
4. Stream object through backend or issue a short-lived signed URL.

Public permanent object URLs are not allowed for private tenant assets.

P5 may stream through the backend as the default implementation. Short-lived signed URLs are allowed only after authentication, tenant filtering, and object-level authorization pass.

Current P5 backend behavior streams downloads through the backend after `asset -> project` authorization. Public permanent MinIO URLs remain forbidden.

Current P5 frontend behavior downloads through `GET /assets/{assetId}/download` and handles the response as a browser blob. Frontend code must not construct MinIO URLs or expose object keys as downloadable URLs.

P8 frontend migration rule:

- Generated and edited workbench results must be read from backend task outputs / authorized asset downloads, not from IndexedDB image blobs.
- Existing browser history blobs may remain only for an explicit compatibility/import flow if one is later approved. They are not platform assets and must not be silently promoted into MinIO-backed tenant storage.

## Generated and edited outputs

P7 Worker output handling must:

1. Receive normalized image bytes from a backend Provider Adapter.
2. Validate MIME, dimensions, and pixel count before persistence when possible.
3. Write output objects to MinIO using backend-generated object keys.
4. Create `image_assets` rows with `kind=GENERATED` or `kind=EDITED`, `task_id`, `project_id`, and `tenant_id`.
5. Create `task_outputs` rows with `task_id`, `asset_id`, and stable `output_index`.
6. Write `IMAGE_OUTPUT` task events after metadata is committed.

Worker retries and duplicate queue delivery must not create duplicate output assets for the same task/output index.

## Thumbnail generation

Reference uploads and generated/edited Worker outputs should create thumbnails as part of the backend persistence path for new assets.

P16 thumbnail policy target:

- Generate bounded thumbnails from already validated image bytes on the backend.
- Store thumbnail bytes in the configured thumbnails bucket, defaulting to `product-thumbnails`.
- Store only `thumbnail_object_key` in MySQL. Do not store thumbnail blobs in MySQL.
- Expose thumbnail access only through a same-origin backend-authorized endpoint such as `GET /api/v1/assets/{assetId}/thumbnail`.
- Keep `thumbnailUrl` empty for assets that do not have `thumbnail_object_key`; do not expose MinIO bucket names, object keys, permanent public URLs, or browser-generated thumbnail data.
- New reference uploads and new Worker outputs should either persist original object, thumbnail object, metadata, task output, and task event consistently or roll back uploaded objects. Do not create asset metadata that points at a missing thumbnail.
- Existing assets without thumbnails are not backfilled in P16. Backfill and orphan discovery remain storage-governance work.
- Thumbnail bytes are intentionally bounded operational overhead in this phase. Quota enforcement still uses the existing metadata source of truth until a later schema/counter task explicitly adds thumbnail-byte accounting.

## Deletion

Default deletion is soft delete in MySQL. Physical MinIO deletion must be handled by controlled backend cleanup paths after retention rules are enabled for a tenant.

Current backend behavior soft-deletes asset metadata for normal deletes. `P13-BE-STORAGE-CLEANUP-FOUNDATION` adds the internal cleanup foundation for physically deleting already soft-deleted asset objects, and `P13-BE-STORAGE-RETENTION-RUNTIME` adds Worker maintenance scheduling when a tenant explicitly enables retention.

P13 storage cleanup foundation status:

- Upload rollback cleanup no longer depends on the HTTP request context after the object has been written. If metadata persistence fails after object upload, backend attempts object cleanup with an independent bounded context.
- Physical cleanup is tenant scoped and metadata driven. Cleanup code must never accept an object key from the browser or another untrusted caller as the source of truth for deletion.
- Cleanup may delete only assets that are already soft deleted and older than a caller-supplied cutoff.
- Cleanup is batch limited and idempotent. Missing MinIO objects count as successful cleanup; non-not-found storage errors leave the asset eligible for retry.
- `image_assets.purged_at` records physical cleanup completion, so repeated cleanup runs do not repeatedly delete already purged objects.
- Do not hard-delete image asset rows in this foundation task. Metadata remains useful for audit/history and future accounting.

P13 retention runtime rules:

- `storageRetention.deletedAssetRetentionDays` is nullable. `null` disables automatic physical cleanup for that tenant.
- The Worker maintenance loop is the runtime consumer for `storageRetention`. It computes a tenant cutoff from the configured day count and calls the cleanup foundation.
- Worker must skip tenants with absent, null, malformed, or unsupported retention settings. It must not delete anything for those tenants.
- Storage quota accounting/enforcement is active in the backend. Frontend may display or edit quota only through the system-settings API and only for tenant admins with `system:settings:manage`.
- Log retention, orphan object listing, manual cleanup triggers, and frontend settings remain deferred until their own runtime consumers are explicitly implemented.

P13 storage quota rules:

- `storageQuota.maxBytes` is nullable. `null` means no tenant storage quota is enforced.
- `storageQuota.usedBytes` is read-only and must be computed from tenant-scoped `image_assets` metadata, counting records whose bytes still exist in MinIO. Soft-deleted but not yet purged assets still count; rows with `purged_at IS NOT NULL` do not count.
- Quota enforcement must run before creating new reference upload assets and before Worker persists generated/edited output assets. Exceeding quota must fail without successful asset metadata, successful task output events, or leaked object keys.
- Quota accounting must not use MinIO bucket listing as its source of truth in this phase. MySQL metadata remains the authoritative accounting source.
- Current non-blocking limitation: quota checks are optimistic and do not yet reserve bytes under concurrent writers. A future task may add strict reservation/counter semantics if operationally required.
