#!/usr/bin/env bash

set -Eeuo pipefail

rm -f /run/studio/minio-ready
export MC_CONFIG_DIR=/run/studio/mc
mkdir -p "${MC_CONFIG_DIR}"
chmod 0700 "${MC_CONFIG_DIR}"

env -u MINIO_ACCESS_KEY -u MINIO_SECRET_KEY /usr/local/bin/minio server /data/minio \
  --address 127.0.0.1:9000 \
  --console-address 127.0.0.1:9001 &
minio_pid=$!

terminate() {
  kill -TERM "${minio_pid}" 2>/dev/null || true
  wait "${minio_pid}" 2>/dev/null || true
}
trap terminate TERM INT EXIT

deadline=$((SECONDS + ${STARTUP_TIMEOUT_SECONDS:-180}))
until curl -fsS http://127.0.0.1:9000/minio/health/ready >/dev/null 2>&1; do
  if ! kill -0 "${minio_pid}" 2>/dev/null; then
    wait "${minio_pid}"
    exit 1
  fi
  (( SECONDS < deadline )) || { printf 'MinIO 未在限定时间内就绪\n' >&2; exit 1; }
  sleep 1
done

/usr/local/bin/mc alias set studio-local http://127.0.0.1:9000 \
  "${MINIO_ROOT_USER}" "${MINIO_ROOT_PASSWORD}" >/dev/null
for bucket in \
  "${MINIO_BUCKET_ORIGINALS}" \
  "${MINIO_BUCKET_GENERATED}" \
  "${MINIO_BUCKET_THUMBNAILS}"; do
  /usr/local/bin/mc mb --ignore-existing "studio-local/${bucket}" >/dev/null
done

touch /run/studio/minio-ready
printf 'MinIO buckets 已就绪。\n'
wait "${minio_pid}"
trap - TERM INT EXIT
