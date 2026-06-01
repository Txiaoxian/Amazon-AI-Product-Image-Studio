# Production Backup/Restore/Rollback Evidence Template

Use this template for operator-reviewed production readiness evidence. Keep
only sanitized stage results. Do not attach dumps, storage listings, storage
paths, credentials, secret values, certificates, Provider responses, image
outputs, or signed URLs.

The repository rehearsal script is limited to its disposable isolated Compose
project. Production backup and restore must use operator-approved runtime
tooling and a documented consistency point.

## Release Candidate

- Commit:
- Operator:
- Environment:
- Date:

## Isolated Compose Rehearsal

Default guardrail-only check:

```bash
bash scripts/backup-restore-rehearsal.sh
```

Explicit live rehearsal:

```bash
BACKUP_RESTORE_REHEARSAL_CONFIRM=I_UNDERSTAND_DATA_REPLACEMENT \
  bash scripts/backup-restore-rehearsal.sh --live
docker compose -f deploy/docker-compose.yml ps -a
docker volume ls --format '{{.Name}}' |
  rg '^amazon-ai-product-image-studio-backup-restore-rehearsal-' || true
```

## Sanitized Evidence

| Check | Result | Sanitized note |
| --- | --- | --- |
| Default guardrail-only mode | PASS / FAIL | No Docker commands or data replacement. |
| Isolated Compose startup | PASS / FAIL / NOT RUN | Disposable rehearsal project only. |
| Matching MySQL and MinIO backup pair | PASS / FAIL / NOT RUN | Do not attach backup content or listings. |
| Fixture destruction and restore | PASS / FAIL / NOT RUN | Record status only. |
| Rollback restore from matching pair | PASS / FAIL / NOT RUN | Record status only. |
| Scoped cleanup | PASS / FAIL / NOT RUN | Record container and volume absence only. |
| Real Provider calls | NOT RUN | Rehearsal must not call a Provider. |

## Production Operator Procedure

- [ ] Maintenance window or write-stop procedure is approved.
- [ ] Approved platform backup tool is identified.
- [ ] MySQL and MinIO are captured at one documented consistency point.
- [ ] Backup storage is access-restricted and outside the Compose host.
- [ ] Restore target is stopped or isolated.
- [ ] Matching MySQL and MinIO restore set is selected.
- [ ] Approved platform restore tool is used.
- [ ] Release health, frontend proxy, SSE boundary, login, upload, task, and
      download checks are assigned before reopening traffic.
- [ ] Rollback release artifact and matching runtime configuration location are
      identified without copying values.

## Decision

Decision: GO / NO-GO

Blocking issues:
