# Security Plan

## Current transition risks after P2

The current `main` branch is still a migration baseline, not a compliant platform release. The following risks are known and must be removed in the documented phases:

| Risk | Current location | Required removal phase | Acceptance check |
| --- | --- | --- | --- |
| Frontend stores Provider API keys in localStorage | `frontend/src/hooks/useSettings.ts` | P8 frontend backendization | No API key field is persisted in localStorage, sessionStorage, IndexedDB, URL params, or client-visible config. |
| Frontend directly calls OpenAI, Gemini, and relay APIs | `frontend/src/providers/**`, `frontend/src/hooks/useGeneration.ts` | P8 frontend backendization, after P6/P7 backend replacements exist | Browser generation flow creates backend tasks only; no Provider `Authorization` header is created in frontend. |
| Image blobs and history are primary data in IndexedDB | `frontend/src/db/**` | P8 frontend backendization, after P5 asset APIs exist | Project assets and task history APIs are the primary data source; IndexedDB is limited to drafts or temporary previews. |
| Frontend Nginx includes legacy AI relay routing | `frontend/nginx.conf` | P3 runtime deployment repair | Frontend Nginx proxies `/api/` to backend only and has no AI Provider relay location. |
| Upload validation is client-side and MIME-based only | `frontend/src/lib/file.ts` | P5 asset management | Backend upload rejects forged MIME, invalid magic bytes, SVG, oversized dimensions, and excessive pixel count. |

These risks are tracked so future agents do not treat the transition state as acceptable production architecture.

## Authentication

Use JWT stored in HttpOnly Cookie.

Cookie requirements:

- HttpOnly.
- Secure in production.
- SameSite protection.
- Reasonable expiration.

Frontend JavaScript must not read authentication tokens.

## Authorization

Use RBAC plus tenant and object-level checks.

Every object ID endpoint must verify:

- Tenant ownership.
- Required permission code.
- Project membership or admin override when applicable.

## API key protection

- Encrypt Provider API keys before storing them.
- Never return full API keys to frontend.
- Show only masked metadata such as hint and last update time.
- Do not log secrets.
- Frontend must not collect or persist Provider API keys after P8. Provider credentials must be managed through backend APIs only.

## Sensitive logging policy

Do not log:

- Full API keys.
- Authorization headers.
- Cookies.
- Passwords.
- Image base64.
- Raw uploaded image bytes.

Log sanitized request IDs, Provider IDs, model IDs, task IDs, durations, statuses, and redacted errors.

## SSRF defense

Provider base URL validation must block:

- `localhost`.
- `127.0.0.0/8`.
- `::1`.
- RFC1918 private ranges.
- Link-local ranges.
- Multicast ranges.
- Docker internal service names.
- Redirects to blocked targets.

Default policy is HTTPS only.

## Upload defense

Uploads must validate true file type and image properties. SVG is forbidden because it can contain script and external references.

Validation must include:

- MIME allowlist.
- Magic byte detection.
- Size limit.
- Width and height limit.
- Pixel-count limit.

Client-side checks may remain for user experience, but they are not a security boundary. Backend validation is mandatory before any object is stored in MinIO.

## CSRF and CORS

Because auth uses Cookies, state-changing APIs need CSRF protection. CORS must be limited to configured frontend origins with credentials enabled only for trusted origins.

## Rate and concurrency limits

Apply limits to:

- Login attempts.
- Uploads.
- Task creation.
- Provider calls.
- SSE connections.

Concurrency limits must exist at global, tenant, user, Provider, and model dimensions.

## Audit

Record operation logs for security-sensitive actions. Metadata must be useful for audit but redacted for secrets.

## Security acceptance checks

- No frontend AI Provider calls.
- No frontend API key storage.
- No complete API key in API response.
- No sensitive data in logs.
- Object-level checks on every object ID API.
- Upload rejects SVG and invalid image bytes.
- Docker frontend has no AI relay route.
- Task status uses SSE, not polling.
