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

The current repository is a pure frontend local app:

- React + TypeScript + Vite.
- Tailwind CSS.
- Dexie / IndexedDB for local history and image blobs.
- localStorage for settings and API keys.
- Frontend Provider Adapters that call OpenAI, Gemini, and OpenAI-compatible relays directly.
- Static Nginx Docker deployment.

This frontend must be preserved and evolved. Do not rewrite the UI from scratch.

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

## Non-goals for P0

- Do not implement backend business code.
- Do not move frontend files yet.
- Do not refactor existing React components.
- Do not replace IndexedDB or localStorage code in P0.

## Implementation posture

- Prefer incremental changes with explicit contracts.
- Keep existing frontend behavior working until the backend replacement path is ready.
- Do not remove existing logic before the backend-backed equivalent exists and is validated.
