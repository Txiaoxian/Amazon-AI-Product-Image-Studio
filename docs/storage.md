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

Current P5 backend behavior validates before object write and attempts to delete the uploaded object if metadata persistence fails. A later hardening task should move this cleanup to an independent timeout context or background cleanup path so request cancellation cannot prevent cleanup.

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

Reference uploads and generated outputs should create thumbnails. Thumbnail creation may run synchronously for small files or through a worker path later.

## Deletion

Default deletion is soft delete in MySQL. Physical MinIO deletion should be handled by a controlled cleanup job after retention rules are defined.

Current P5 backend behavior soft-deletes asset metadata and leaves physical object deletion to a future retention/cleanup job.
