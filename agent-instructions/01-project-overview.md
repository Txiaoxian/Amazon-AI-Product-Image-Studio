# Project Overview

## Product

Amazon AI Product Image Studio is an AI product image generation and editing platform for Amazon sellers.

The platform helps sellers:

- Generate product images with AI.
- Edit existing product images with AI.
- Upload product reference images.
- Manage one project per product.
- Store generated, edited, and reference images in a project asset library.
- Track prompts, task history, usage, costs, and audit events.

## Current state

The repository has been platformized through R12:

- The original React + TypeScript + Vite + Tailwind frontend now lives under `frontend/`.
- The Go + Gin + GORM backend now lives under `backend/` with API and Worker entrypoints.
- Docker Compose deployment assets live under `deploy/`.
- Authentication, RBAC, tenant isolation, tenant user/role administration, projects, project members, assets, Provider/model management, task APIs, Redis queueing, Worker execution, SSE, unified project history, usage/audit reads, upload-policy settings, and runtime hardening through P12 are implemented.
- Production generation/edit flows go through backend task APIs, backend Provider Adapters, Redis queueing, Worker execution, MinIO assets, and SSE. The browser must not call AI Providers directly.
- Browser Provider adapters, normal-user Provider API key/API URL settings, and IndexedDB-backed production history/image paths have been removed or retired from the production import graph.
- Seller-facing project workflows now include backend-backed project edit, asset filtering, asset metadata edit, project member entry points, unified history, authorized detail/download/re-edit, and backend last-`OWNER` protection for member writes.
- IndexedDB may still support non-sensitive local prompt-template convenience data and tests; it must not be reintroduced as platform history or image truth.

The existing frontend UI concepts should still be preserved and evolved. Do not rewrite the UI from scratch unless a later task explicitly authorizes a replacement.

## Target stack

- Frontend: React + TypeScript + Vite + Tailwind CSS.
- Backend: Go + Gin + GORM.
- Database: MySQL 8.
- Queue and cache: Redis.
- Object storage: MinIO.
- Auth: JWT with HttpOnly Cookie.
- Authorization: RBAC plus `tenant_id` isolation.
- Task status: SSE only.
- Deployment: Docker Compose.

## Historical P0 non-goals

P0 is complete. Its old constraints are historical only: P0 did not implement backend business code, move frontend files, refactor React components, or replace local storage paths. Current tasks must follow the latest phase plan in `docs/development-plan.md` and `docs/codex-agent-tasks.md`.

## Implementation posture

- Prefer incremental changes with explicit contracts.
- Preserve existing user-facing concepts while keeping the current backend-backed production paths honest.
- Do not remove or replace an existing production path unless the backend-backed equivalent exists, is validated, and the task explicitly owns the migration.
