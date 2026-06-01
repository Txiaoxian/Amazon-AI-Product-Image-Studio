# Production Dry-Run Evidence Template

Use this template for the R18 Go/No-Go review. Keep only sanitized stage
results. Do not attach env files, dumps, secrets, Provider responses, image
outputs, bucket names, object keys, signed URLs, or service credentials.

## Release Candidate

- Commit:
- Operator:
- Environment:
- Date:

## Commands

Default no-cost rehearsal:

```bash
bash scripts/prod-dry-run.sh
```

Explicit production env preflight, using an existing restricted file outside
the repository:

```bash
bash scripts/prod-dry-run.sh \
  --production-env-file /secure/runtime/production.env
```

Optional live Compose rehearsal with scoped cleanup:

```bash
bash scripts/prod-dry-run.sh --live-compose
docker compose -f deploy/docker-compose.yml ps -a
docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true
```

## Sanitized Evidence

| Check | Result | Sanitized note |
| --- | --- | --- |
| Production env preflight | PASS / FAIL / NOT RUN | Values must never be copied here. |
| Deployment release validation | PASS / FAIL | |
| Security regression | PASS / FAIL | |
| Real Provider smoke guardrail dry-run | PASS / FAIL | |
| Backup/restore rehearsal guardrail dry-run | PASS / FAIL | |
| Compose config validation | PASS / FAIL | |
| Live Compose health path | PASS / FAIL / NOT RUN | |
| Live Compose cleanup | PASS / FAIL / NOT RUN | Record container/volume absence only. |
| Optional real Provider smoke | PASS / FAIL / NOT RUN | Manual billable step; sanitized status only. |

## Backup And Restore Rehearsal

Use [PRODUCTION_BACKUP_RESTORE_TEMPLATE.md](./PRODUCTION_BACKUP_RESTORE_TEMPLATE.md)
for the isolated live rehearsal and production operator procedure evidence.

- [ ] MySQL and MinIO backups are treated as one consistency point.
- [ ] Backup destination is outside the repository and access restricted.
- [ ] Restore target is stopped or isolated before rehearsal.
- [ ] Restore procedure uses the matching MySQL and MinIO backup set.
- [ ] Release validation is run after restore before reopening traffic.
- [ ] No dump, object listing, object key, signed URL, or credential is attached.

## Rollback Rehearsal

- [ ] Previous release commit or artifact is identified.
- [ ] Matching runtime configuration location is identified without copying values.
- [ ] Maintenance or write-stop procedure is identified.
- [ ] Matching MySQL and MinIO restore point is identified.
- [ ] Health, frontend proxy, SSE boundary, login, upload, task, and download checks are assigned.

## Go / No-Go

Go only when:

- All required default checks pass.
- Explicit production env preflight passes for the target runtime file.
- Any live Compose rehearsal completed cleanup with no project containers or
  volumes left behind.
- Backup, restore, and rollback rehearsal checklists are complete.
- Optional real Provider smoke is either approved and passed or explicitly
  recorded as not run.

Decision: GO / NO-GO

Blocking issues:
