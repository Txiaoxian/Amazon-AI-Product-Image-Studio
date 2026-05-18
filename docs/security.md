# Security Plan

## Current transition risks after R8

The current `main` branch has completed and passed R8 verification for the P8 frontend backendization work. Browser AI Provider execution, browser Provider credential persistence, and IndexedDB-backed generated image/history production paths are no longer acceptable platform behavior. The following table records the P8 transition risks and their current status so future agents do not reintroduce them:

| Risk | Previous location | Status after P8 | Acceptance check |
| --- | --- | --- | --- |
| Frontend stores Provider API keys in localStorage | `frontend/src/hooks/useSettings.ts` | Resolved. Normal Provider settings were removed; Provider keys are submitted only through backend Provider management forms and are not persisted in browser storage. | Static scan and tests must continue to show no Provider API key/API URL persistence in localStorage, sessionStorage, IndexedDB, URL params, or client-visible config. |
| Frontend directly calls OpenAI, Gemini, and relay APIs | `frontend/src/providers/**`, old browser Provider adapters | Resolved. Browser Provider adapter files and frontend Provider registry/types were removed; workbench generation creates backend tasks only. | Browser generation flow creates backend tasks only; no Provider `Authorization` header or direct Provider host appears in production frontend code. |
| Image blobs and history are primary data in IndexedDB | `frontend/src/db/**` | Resolved for production workbench. Backend project assets and task history are the source of truth; remaining IndexedDB use is limited to prompt templates and residual non-production helpers/tests. | Project assets and task history APIs are the primary data source; old local blobs are not silently uploaded and must not re-enter the production history path. |
| Legacy local upload validation is client-side and MIME-based only | `frontend/src/lib/file.ts` and old local generation path | Resolved for generation path. Reference uploads go through backend asset upload validation; frontend precheck remains UX only. | Backend asset upload rejects forged MIME, invalid magic bytes, SVG, oversized dimensions, and excessive pixel count. |

Remaining P9 review risks:

- Unreachable legacy display components and old IndexedDB helper files may still exist. They must remain outside the production import graph and should be deleted or explicitly quarantined in P9.
- Generic frontend `422` handling currently treats validation errors broadly as stale model/capability failures; a narrower backend error contract is a P9 hardening candidate.
- Frontend history currently joins separately paged task and asset lists; a backend history query would reduce pagination edge cases.
- Historical dirty rows containing non-heuristic secrets still need a future design if exact read-time scrubbing is required; P9 audit reads intentionally do not widen Provider plaintext key decryption into the admin read path without a trusted minimal secret source and lifecycle.
- Writable system settings remain constrained to fields with live runtime consumers. The next approved slice is tenant upload policy backed by asset validation; default Provider/model IDs, tenant concurrency, storage quotas, and retention remain deferred until their task/worker/cleanup consumers are explicit.

Resolved transition item:

- P3 removed the frontend Nginx AI relay route. The frontend container must continue to proxy `/api/` only to `backend-api` and must not proxy AI Providers.
- P5 backend asset upload now validates MIME, magic bytes, size, dimensions, and pixel count before storing reference images in MinIO.
- P5 frontend project/asset UI now uses authenticated backend project and asset APIs for reference uploads, metadata, favorite/delete, and downloads. It does not talk to MinIO directly and did not add new AI Provider direct calls, Provider API key persistence, auth token persistence, or task polling.
- P6 backend Provider/model management now stores Provider API keys encrypted at rest, returns only masked key metadata, validates Provider URLs for save/update/test, records redacted operation logs, and exposes tenant-scoped Provider/model APIs.
- P6 frontend Provider/model management now submits Provider API keys only to backend APIs, displays only masked metadata, clears submitted and unsubmitted key drafts, and does not persist Provider keys in browser storage.
- P7 Provider runtime now uses connect-time SSRF-safe outbound transport before real Provider calls and recursively redacts runtime metadata before persistence. Review fixes explicitly covered API keys appearing as values and as nested JSON map keys.
- P7 frontend task client work now uses EventSource/SSE contracts and did not introduce polling, new Provider direct calls, or new Provider API key persistence.
- P8 frontend backendization replaced the production workbench with backend task API + SSE + authorized backend assets, removed normal browser Provider settings, removed browser Provider adapters, removed `legacyFile` reference payloads, and moved history/detail/download/re-edit to backend assets and tasks.
- R8 verified frontend, backend, and Compose config regression. Sensitive frontend static scan returned no production-code hits for browser Provider credentials, Provider Authorization headers, direct Provider hosts, task polling, or sensitive browser storage. Provider static-scan hits are limited to backend Provider management API consumers.
- P9 audit/usage read APIs now use shared recursive redaction, tenant-scoped queries, admin RBAC, and deterministic pagination. Review fixes centralized the redaction implementation and proved exact known-secret scrubbing through a controlled injection seam without expanding production Provider-key decryption scope.
- P9 production startup hardening now rejects placeholder `JWT_SIGNING_SECRET` and placeholder `API_KEY_ENCRYPTION_KEY` before API or Worker startup can proceed in production, while keeping non-production defaults available.

P5 review hardening backlog:

- Uploaded-object cleanup after metadata persistence failure should use an independent cleanup context or background cleanup job so request cancellation cannot prevent cleanup.
- Built-in `asset:*` permissions are seeded for new tenants; existing tenants need a future permission reconciliation path.
- MinIO bucket creation or verification remains an environment/deployment responsibility.
- Frontend upload precheck limits are currently UX-only and not the platform security boundary. Backend upload validation remains authoritative until system upload limits are exposed to the frontend.

P4 review hardening backlog:

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
- Frontend may collect Provider API keys only in backend Provider management forms for immediate submission. It must never persist Provider API keys. The legacy local settings flow has been removed in P8; Provider credentials must be managed through backend APIs only.

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

Current P9 audit-read status:

- Audit/usage read responses use the same shared redaction implementation as Provider runtime rather than a forked heuristic-only copy.
- Exact known-secret value/key removal is supported when a trusted redactor is explicitly injected.
- Production admin read APIs currently default to heuristic redaction only. This is intentional: Provider plaintext API keys are not broadly decrypted just to scrub historical dirty rows.
- If future requirements demand exact read-time scrubbing of historical non-heuristic secrets, first design a narrowly scoped secret source, authorized lifecycle, and retention policy; do not widen decryption ad hoc inside read handlers.

P8 migration security requirements:

- Browser generation/edit flows must create backend tasks only and never create Provider `Authorization` headers.
- Any browser-persisted settings that survive P8 must be non-sensitive UI preferences only; Provider API keys and Provider API URLs are forbidden.
- Existing local history blobs may remain only as explicit compatibility data if retained; they must not be silently uploaded into tenant storage or remain the normal production history source.
- Workbench status must consume SSE only. Polling is still forbidden even during migration fallback handling.

Current R8 frontend security status:

- Production workbench generation/edit flows create backend tasks and use SSE for status.
- Static scan after `P8-FE-LEGACY-RETIREMENT` found no production direct Provider host, Provider `Authorization` header, Provider key persistence, or polling loop.
- Remaining `providers` static-scan hits are backend Provider management API paths, not browser AI Provider calls.
- Remaining IndexedDB usage must stay limited to prompt templates or explicitly non-production residual helpers until P9 cleanup.

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
