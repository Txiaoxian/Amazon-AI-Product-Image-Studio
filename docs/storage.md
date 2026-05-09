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

## Thumbnail generation

Reference uploads and generated outputs should create thumbnails. Thumbnail creation may run synchronously for small files or through a worker path later.

## Deletion

Default deletion is soft delete in MySQL. Physical MinIO deletion should be handled by a controlled cleanup job after retention rules are defined.
