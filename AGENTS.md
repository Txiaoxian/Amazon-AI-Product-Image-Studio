# Project AGENTS.md

This is the project-level main instruction file for Amazon AI Product Image Studio. It is an index only. Detailed rules live in `agent-instructions/`.

## How to use

- Read this file first, then read the matching sub-instruction files for the task.
- If multiple topics apply, read all matching files.
- Global Codex instructions still apply. More specific project rules in this directory take precedence for this repository.
- Keep this structure when editing project instructions: main `AGENTS.md` plus focused files under `agent-instructions/`.
- For worktree task planning, migration tasks, or review handoff, read `agent-instructions/07-task-package-and-review-rules.md` in addition to the domain-specific rules.

## Project decision

The target platform structure is a monorepo-style root:

- `frontend/` for the existing React + TypeScript + Vite + Tailwind app.
- `backend/` for the Go + Gin + GORM API and worker services.
- `deploy/` for Docker Compose, service config, and deployment assets.
- `docs/` for architecture, contracts, security, and development plans.

P0/P1 are complete. The `frontend/`, `backend/`, `deploy/`, and `docs/` split is now the active repository structure.

## Mandatory platform rules

- The frontend must not call OpenAI, Gemini, or any AI relay directly.
- The frontend must not store AI Provider API keys in localStorage, IndexedDB, sessionStorage, source code, or client-visible config.
- Task status must use SSE. Do not use polling, `setInterval`, or repeated fetch loops for task progress.
- All AI calls must go through the Go backend and a Provider Adapter.
- Every business table must include `tenant_id`, and tenant-scoped queries must filter by `tenant_id`.
- Images must be stored in MinIO. MySQL stores metadata and `object_key`, never image blobs.
- API keys must be encrypted at rest and never returned in full to the frontend.
- Logs must not contain full API keys, Authorization headers, Cookies, or image base64 data.
- Provider `base_url` must defend against SSRF and block localhost, loopback, private, link-local, and Docker-internal targets.
- File uploads must validate true file type, size, dimensions, and pixel count. SVG upload is forbidden.
- Object ID APIs must check object-level authorization, not only login state.
- Image downloads must pass through backend authorization.
- 平台面向用户的文本应尽量使用简体中文，尤其是配置标签、说明文案、校验提示和错误信息；必要的技术标识、枚举值、模型 ID、API 字段名可保持原始形式。

## Instruction index

| File | When to read | Summary |
| --- | --- | --- |
| `agent-instructions/01-project-overview.md` | All tasks | Product goal, current state, target stack, non-goals. |
| `agent-instructions/02-architecture-rules.md` | Architecture, directories, service boundaries | Target monorepo layout, service boundaries, data flow, hard architecture rules. |
| `agent-instructions/03-frontend-rules.md` | Frontend code, UI, state, API integration | Preserve existing React UI while replacing local AI/data paths with backend contracts. |
| `agent-instructions/04-backend-rules.md` | Go backend, database, worker, queues | Gin/GORM structure, tenant filters, Redis queue, MySQL source of truth. |
| `agent-instructions/05-security-rules.md` | Auth, RBAC, uploads, Provider config, logging | Security requirements for cookies, SSRF, API keys, uploads, audit, and logs. |
| `agent-instructions/06-testing-and-delivery.md` | Verification, delivery notes, PR handoff | Required validation commands, Simplified Chinese Git messages, and delivery expectations by change type. |
| `agent-instructions/07-task-package-and-review-rules.md` | Worktree task planning, migration tasks, review handoff | Required task-package sections, intermediate-state rules, failure matrices, and regression-test mapping. |

## Related planning docs

- `docs/architecture.md`
- `docs/business-requirements.md`
- `docs/database-schema.md`
- `docs/api-contract.md`
- `docs/sse-contract.md`
- `docs/rbac.md`
- `docs/provider-adapter.md`
- `docs/task-queue.md`
- `docs/storage.md`
- `docs/security.md`
- `docs/deployment.md`
- `docs/local-development.md`
- `docs/development-plan.md`
- `docs/codex-agent-tasks.md`
