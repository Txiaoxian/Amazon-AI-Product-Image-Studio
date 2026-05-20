# MinIO Bootstrap

`deploy/docker-compose.yml` uses the `minio-bootstrap` one-shot service to
create the required buckets idempotently with `mc mb --ignore-existing`.

Image objects must live in MinIO. MySQL should store metadata and object keys,
not image blobs.

Required buckets are configured by environment variables:

- `MINIO_BUCKET_ORIGINALS`
- `MINIO_BUCKET_GENERATED`
- `MINIO_BUCKET_THUMBNAILS`

Do not put real MinIO credentials or object data in this directory.
