#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.yml"
PROJECT_NAME="amazon-ai-product-image-studio-backup-restore-rehearsal-$$"
RUN_LIVE=0
STACK_MANAGED=0
CLEANUP_RAN=0
TMP_DIR=""

usage() {
  cat <<'EOF'
Usage: bash scripts/backup-restore-rehearsal.sh [--live] [--help]

Runs the P20 backup/restore/rollback rehearsal for an isolated Compose project.

Default safe mode:
  - prints guardrail validation only
  - does not start Docker services
  - does not create backups or replace data
  - does not contact any AI Provider

Options:
  --live  Start an isolated Compose data stack, create task-owned fixture data,
          back up MySQL and MinIO as one pair, destroy and restore the fixture,
          repeat restore as a rollback rehearsal, then remove project containers
          and volumes. Requires:
          BACKUP_RESTORE_REHEARSAL_CONFIRM=I_UNDERSTAND_DATA_REPLACEMENT
  --help  Show this help text without executing Docker commands.

The live mode is only for this repository's disposable Compose rehearsal
environment. It does not operate shared local development services or replace
an operator-approved production backup/restore procedure.
EOF
}

log() {
  printf '[backup-restore-rehearsal] %s\n' "$*"
}

fail() {
  printf '[backup-restore-rehearsal][fail] %s\n' "$*" >&2
  exit 1
}

compose() {
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup_compose_stack() {
  if [[ "$CLEANUP_RAN" -eq 1 ]]; then
    return 0
  fi
  CLEANUP_RAN=1

  local output
  output="$(mktemp /tmp/backup-restore-rehearsal-cleanup.XXXXXX)"
  set +e
  compose down -v --remove-orphans >"$output" 2>&1
  local status=$?
  set -e
  rm -f "$output"
  if [[ "$status" -ne 0 ]]; then
    log "scoped cleanup failed"
    return "$status"
  fi
  log "stage passed: scoped project cleanup"
}

cleanup_on_exit() {
  local status=$?
  trap - EXIT INT TERM

  if [[ "$STACK_MANAGED" -eq 1 ]]; then
    set +e
    cleanup_compose_stack
    local cleanup_status=$?
    set -e
    if [[ "$cleanup_status" -ne 0 && "$status" -eq 0 ]]; then
      status="$cleanup_status"
    fi
  fi

  if [[ -n "$TMP_DIR" ]]; then
    rm -rf "$TMP_DIR"
  fi
  exit "$status"
}

trap cleanup_on_exit EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "required command not found: $1"
  fi
}

run_stage() {
  local name="$1"
  shift
  local output
  output="$(mktemp "$TMP_DIR/stage.XXXXXX")"

  log "stage: $name"
  set +e
  "$@" >"$output" 2>&1
  local status=$?
  set -e
  rm -f "$output"
  if [[ "$status" -ne 0 ]]; then
    fail "stage failed: $name"
  fi
  log "stage passed: $name"
}

create_fixture_metadata() {
  compose exec -T mysql sh -c '
    mysql -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE" -e "
      CREATE TABLE IF NOT EXISTS backup_restore_rehearsal_fixture (
        tenant_id VARCHAR(64) NOT NULL,
        fixture_id VARCHAR(64) NOT NULL PRIMARY KEY,
        object_key VARCHAR(255) NOT NULL
      );
      DELETE FROM backup_restore_rehearsal_fixture
      WHERE fixture_id = '\''p20-rehearsal-fixture'\'';
      INSERT INTO backup_restore_rehearsal_fixture (tenant_id, fixture_id, object_key)
      VALUES ('\''p20-rehearsal-tenant'\'', '\''p20-rehearsal-fixture'\'', '\''rehearsal/fixture-object.txt'\'');
    " >/dev/null
  '
}

wait_for_mysql() {
  local attempt
  for ((attempt = 1; attempt <= 60; attempt++)); do
    if compose exec -T mysql sh -c \
      'mysqladmin ping -h 127.0.0.1 -uroot -p"$MYSQL_ROOT_PASSWORD" --silent' \
      >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  return 1
}

create_fixture_object() {
  compose run --rm --no-deps --entrypoint /bin/sh minio-bootstrap -c '
    set -eu
    mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
    printf "%s" "p20-backup-restore-rehearsal-fixture" |
      mc pipe local/"$MINIO_BUCKET_ORIGINALS"/rehearsal/fixture-object.txt >/dev/null
  '
}

backup_mysql() {
  compose exec -T mysql sh -c \
    'mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' \
    >"$TMP_DIR/mysql.sql"
}

backup_minio() {
  mkdir -p \
    "$TMP_DIR/minio/originals" \
    "$TMP_DIR/minio/generated" \
    "$TMP_DIR/minio/thumbnails"
  compose run --rm --no-deps \
    -v "$TMP_DIR/minio:/backup" \
    --entrypoint /bin/sh minio-bootstrap -c '
      set -eu
      mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
      mc mirror --overwrite local/"$MINIO_BUCKET_ORIGINALS" /backup/originals >/dev/null
      mc mirror --overwrite local/"$MINIO_BUCKET_GENERATED" /backup/generated >/dev/null
      mc mirror --overwrite local/"$MINIO_BUCKET_THUMBNAILS" /backup/thumbnails >/dev/null
    '
}

destroy_fixture_metadata() {
  compose exec -T mysql sh -c '
    mysql -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE" -e "
      DELETE FROM backup_restore_rehearsal_fixture
      WHERE fixture_id = '\''p20-rehearsal-fixture'\'';
    " >/dev/null
  '
}

destroy_fixture_object() {
  compose run --rm --no-deps --entrypoint /bin/sh minio-bootstrap -c '
    set -eu
    mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
    mc rm --force local/"$MINIO_BUCKET_ORIGINALS"/rehearsal/fixture-object.txt >/dev/null
  '
}

restore_mysql() {
  compose exec -T mysql sh -c \
    'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' \
    <"$TMP_DIR/mysql.sql"
}

restore_minio() {
  compose run --rm --no-deps \
    -v "$TMP_DIR/minio:/backup:ro" \
    --entrypoint /bin/sh minio-bootstrap -c '
      set -eu
      mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
      mc mirror --overwrite /backup/originals local/"$MINIO_BUCKET_ORIGINALS" >/dev/null
      mc mirror --overwrite /backup/generated local/"$MINIO_BUCKET_GENERATED" >/dev/null
      mc mirror --overwrite /backup/thumbnails local/"$MINIO_BUCKET_THUMBNAILS" >/dev/null
    '
}

verify_fixture_metadata() {
  compose exec -T mysql sh -c '
    test "$(
      mysql -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE" -Nse "
        SELECT COUNT(*)
        FROM backup_restore_rehearsal_fixture
        WHERE tenant_id = '\''p20-rehearsal-tenant'\''
          AND fixture_id = '\''p20-rehearsal-fixture'\''
          AND object_key = '\''rehearsal/fixture-object.txt'\'';
      "
    )" = "1"
  '
}

verify_fixture_object() {
  compose run --rm --no-deps --entrypoint /bin/sh minio-bootstrap -c '
    set -eu
    mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
    test "$(
      mc cat local/"$MINIO_BUCKET_ORIGINALS"/rehearsal/fixture-object.txt
    )" = "p20-backup-restore-rehearsal-fixture"
  '
}

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --live)
      RUN_LIVE=1
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
  shift
done

if [[ "$RUN_LIVE" -ne 1 ]]; then
  log "guardrail dry-run passed: isolated Compose project only; no Docker commands, data replacement, shared services, or Provider calls"
  log "use --live with explicit confirmation for the disposable Compose rehearsal"
  exit 0
fi

if [[ "${BACKUP_RESTORE_REHEARSAL_CONFIRM:-}" != "I_UNDERSTAND_DATA_REPLACEMENT" ]]; then
  fail "refusing --live without BACKUP_RESTORE_REHEARSAL_CONFIRM=I_UNDERSTAND_DATA_REPLACEMENT"
fi

require_command docker
if [[ ! -f "$COMPOSE_FILE" ]]; then
  fail "Compose file not found"
fi

TMP_DIR="$(mktemp -d /tmp/backup-restore-rehearsal.XXXXXX)"
chmod 700 "$TMP_DIR"
STACK_MANAGED=1

log "starting isolated Compose backup/restore/rollback rehearsal"
run_stage "start isolated MySQL and MinIO services" compose up -d mysql minio
run_stage "wait for isolated MySQL readiness" wait_for_mysql
run_stage "bootstrap isolated MinIO buckets" compose up minio-bootstrap
run_stage "create isolated fixture metadata" create_fixture_metadata
run_stage "create isolated fixture object" create_fixture_object
run_stage "backup isolated MySQL" backup_mysql
run_stage "backup isolated MinIO" backup_minio
run_stage "destroy fixture metadata" destroy_fixture_metadata
run_stage "destroy fixture object" destroy_fixture_object
run_stage "restore matching backup pair" restore_mysql
run_stage "restore matching backup pair object data" restore_minio
run_stage "verify restored fixture metadata" verify_fixture_metadata
run_stage "verify restored fixture object" verify_fixture_object
run_stage "destroy fixture metadata for rollback" destroy_fixture_metadata
run_stage "destroy fixture object for rollback" destroy_fixture_object
run_stage "rollback restore matching backup pair" restore_mysql
run_stage "rollback restore matching backup pair object data" restore_minio
run_stage "verify rollback fixture metadata" verify_fixture_metadata
run_stage "verify rollback fixture object" verify_fixture_object
cleanup_compose_stack || fail "scoped cleanup failed"
STACK_MANAGED=0

log "sanitized evidence summary: isolated rehearsal passed"
log "sanitized evidence summary: matching MySQL and MinIO pair restored and rollback-rehearsed"
log "sanitized evidence summary: scoped project cleanup passed"
