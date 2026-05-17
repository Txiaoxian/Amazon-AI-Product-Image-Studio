# Security Plan

## Current transition risks after R6

The current `main` branch is still a migration baseline, not a compliant platform release. The following risks are known and must be removed in the documented phases:

| Risk | Current location | Required removal phase | Acceptance check |
| --- | --- | --- | --- |
| Frontend stores Provider API keys in localStorage | `frontend/src/hooks/useSettings.ts` | P8 frontend backendization | No API key field is persisted in localStorage, sessionStorage, IndexedDB, URL params, or client-visible config. |
| Frontend directly calls OpenAI, Gemini, and relay APIs | `frontend/src/providers/**`, `frontend/src/hooks/useGeneration.ts` | P8 frontend backendization, after P6/P7 backend replacements exist | Browser generation flow creates backend tasks only; no Provider `Authorization` header is created in frontend. |
| Image blobs and history are primary data in IndexedDB | `frontend/src/db/**` | P8 frontend backendization, after P5 asset APIs exist | Project assets and task history APIs are the primary data source; IndexedDB is limited to drafts or temporary previews. |
| Legacy local upload validation is client-side and MIME-based only | `frontend/src/lib/file.ts` | P8 frontend backendization for legacy generation paths | Backend asset upload already rejects forged MIME, invalid magic bytes, SVG, oversized dimensions, and excessive pixel count. Legacy local paths must not be treated as platform asset storage. |

These risks are tracked so future agents do not treat the transition state as acceptable production architecture.

Resolved transition item:

- P3 removed the frontend Nginx AI relay route. The frontend container must continue to proxy `/api/` only to `backend-api` and must not proxy AI Providers.
- P5 backend asset upload now validates MIME, magic bytes, size, dimensions, and pixel count before storing reference images in MinIO.
- P5 frontend project/asset UI now uses authenticated backend project and asset APIs for reference uploads, metadata, favorite/delete, and downloads. It does not talk to MinIO directly and did not add new AI Provider direct calls, Provider API key persistence, auth token persistence, or task polling.
- P6 backend Provider/model management now stores Provider API keys encrypted at rest, returns only masked key metadata, validates Provider URLs for save/update/test, records redacted operation logs, and exposes tenant-scoped Provider/model APIs.
- P6 frontend Provider/model management now submits Provider API keys only to backend APIs, displays only masked metadata, clears submitted and unsubmitted key drafts, and does not persist Provider keys in browser storage.
- P7 Provider runtime now uses connect-time SSRF-safe outbound transport before real Provider calls and recursively redacts runtime metadata before persistence. Review fixes explicitly covered API keys appearing as values and as nested JSON map keys.

P5 review hardening backlog:

- Uploaded-object cleanup after metadata persistence failure should use an independent cleanup context or background cleanup job so request cancellation cannot prevent cleanup.
- Built-in `asset:*` permissions are seeded for new tenants; existing tenants need a future permission reconciliation path.
- MinIO bucket creation or verification remains an environment/deployment responsibility.
- Frontend upload precheck limits are currently UX-only and not the platform security boundary. Backend upload validation remains authoritative until system upload limits are exposed to the frontend.

P4 review hardening backlog:

- Production startup must reject default `JWT_SIGNING_SECRET` and other placeholder secrets.
- Frontend CSRF header handling currently uses the default `X-CSRF-Token`; non-default `CSRF_HEADER_NAME` deployments need an explicit frontend config/source of truth before use.
- Audit metadata redaction currently covers auth usage but should be recursive before Provider, asset, and task modules write nested metadata.

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
- Frontend may collect Provider API keys only in backend Provider management forms for immediate submission. It must never persist Provider API keys, and after P8 the legacy local settings flow must be removed. Provider credentials must be managed through backend APIs only.

P6 Provider management must additionally enforce:

- API keys may be accepted by Provider create/update forms only for immediate submission to the backend.
- The backend response must never include plaintext API keys or encrypted key material.
- Updating a Provider without `apiKey` must retain the existing encrypted key.
- Rotating an API key must create an operation log with redacted metadata only.
- Provider test must use decrypted credentials only in backend memory and must redact outbound request metadata before logs or API responses.

Current P6 Provider backend status:

- Provider API key encryption, masked responses, rotation metadata, backend-only Provider test, and recursive operation-log metadata redaction are implemented.
- Provider test does not create tasks, assets, or usage records.
- Frontend Provider/model management is implemented and does not persist Provider API keys or create Provider direct browser calls.
- Full production startup hardening must still reject default placeholder `API_KEY_ENCRYPTION_KEY` before release.

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

P6 must validate Provider URLs at both persistence time and outbound test/use time. Save-time validation alone is insufficient because DNS can change after a Provider is saved.

Provider URL validation must reject:

- Non-HTTP(S) schemes.
- Plain HTTP unless a future explicitly documented local-development exception is approved.
- URLs with embedded credentials.
- Hostnames that are Docker Compose service names or resolve to blocked IP ranges.
- Redirect chains that land on a blocked target.

SSRF tests are required before P6 merge.

Current P6 Provider backend status:

- Provider save/update/test SSRF tests cover blocked schemes, embedded credentials, localhost, loopback, private ranges, link-local, multicast, Docker service names, DNS resolution to blocked ranges, and redirects to blocked targets.
- Before P7 real Provider Adapter execution, outbound Provider clients must use connect-time IP validation or an SSRF-safe dialer to prevent DNS rebinding between validation and connection.

P7 Provider runtime requirement:

- Real Provider generation/edit calls must not start until the outbound HTTP transport validates the final dial target at connection time.
- Provider runtime logs and api_call_logs must recursively redact Authorization, Cookie, API keys, bearer tokens, image base64, and raw image bytes.
- Provider runtime must treat model capability rows as the trusted parameter allowlist.

Current P7 Provider runtime status:

- The runtime requirement above is implemented and merged.
- Configured Provider API keys are decrypted only in backend memory, passed into the redactor as known secrets, and removed from persisted metadata whether they appear as values or nested JSON map keys.
- Residual boundary: unknown secrets that are neither supplied as known secrets nor matched by heuristic rules cannot be identified automatically. This is a generic limit of redaction, not an uncovered path for the currently configured Provider API key.

## Upload defense

Uploads must validate true file type and image properties. SVG is forbidden because it can contain script and external references.

Validation must include:

- MIME allowlist.
- Magic byte detection.
- Size limit.
- Width and height limit.
- Pixel-count limit.

Client-side checks may remain for user experience, but they are not a security boundary. Backend validation is mandatory before any object is stored in MinIO.

P5 upload implementation must verify all of the following before writing to MinIO:

- The authenticated user can access the target project in the current tenant.
- The request MIME type is in the allowlist.
- Magic bytes decode as JPEG, PNG, or WebP.
- The decoded image dimensions and pixel count are within configured limits.
- The filename is treated as untrusted metadata only and never used directly in MinIO object keys.

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
