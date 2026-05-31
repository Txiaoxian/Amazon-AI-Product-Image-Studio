# Release Runbook Pointer

The canonical operator runbook is [`deploy/RUNBOOK.md`](../deploy/RUNBOOK.md).

Keep release commands, production secret handling, MinIO bootstrap, SSE proxy
requirements, backup/restore steps, upgrade/rollback instructions, real
Provider smoke guidance, and production dry-run instructions in that single
file so operator documentation does not drift.

Repository-wide planning and validation evidence remain in:

- [`docs/deployment.md`](deployment.md)
- [`docs/development-plan.md`](development-plan.md)
- [`docs/security.md`](security.md)

Production rules remain unchanged:

- Provider API keys are configured through backend admin APIs only.
- Frontend code must not call AI Providers or relay endpoints directly.
- Frontend code must not persist Provider API keys.
- Task progress uses SSE, never polling.
- Production secrets must not use placeholders or enter committed files,
  logs, screenshots, shell history, or validation evidence.
- MySQL and MinIO backups must be taken and restored as a consistent pair.
