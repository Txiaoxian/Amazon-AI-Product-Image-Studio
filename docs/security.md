# Security Plan

## Current Transition Risks During P20 Operational Hardening

The current `main` branch has completed P15 release hardening, P16 production-launch hardening, P17 storage governance/diagnostics, P18 production dry-run, and the merged P19/P20 operational hardening slices. Browser AI Provider execution, browser Provider credential persistence, and IndexedDB-backed generated image/history production paths are no longer acceptable platform behavior. The following table records the resolved transition risks and their current status so future agents do not reintroduce them:

| Risk | Previous location | Current status after R12 | Acceptance check |
| --- | --- | --- | --- |
| Frontend stores Provider API keys in localStorage | `frontend/src/hooks/useSettings.ts` | Resolved. Normal Provider settings were removed; Provider keys are submitted only through backend Provider management forms and are not persisted in browser storage. | Static scan and tests must continue to show no Provider API key/API URL persistence in localStorage, sessionStorage, IndexedDB, URL params, or client-visible config. |
| Frontend directly calls OpenAI, Gemini, and relay APIs | `frontend/src/providers/**`, old browser Provider adapters | Resolved. Browser Provider adapter files and frontend Provider registry/types were removed; workbench generation creates backend tasks only. | Browser generation flow creates backend tasks only; no Provider `Authorization` header or direct Provider host appears in production frontend code. |
| Image blobs and history are primary data in IndexedDB | `frontend/src/db/**` | Resolved for production workbench. Backend project assets and task history are the source of truth; remaining IndexedDB use is limited to prompt templates and residual non-production helpers/tests. | Project assets and task history APIs are the primary data source; old local blobs are not silently uploaded and must not re-enter the production history path. |
| Legacy local upload validation is client-side and MIME-based only | `frontend/src/lib/file.ts` and old local generation path | Resolved for generation path. Reference uploads go through backend asset upload validation; frontend precheck remains UX only. | Backend asset upload rejects forged MIME, invalid magic bytes, SVG, oversized dimensions, and excessive pixel count. |

Remaining security and hardening risks:

- Provider/model lifecycle integrity is now hardened: Provider deletion is blocked while same-tenant non-deleted linked models exist, Provider disable is blocked while enabled linked models exist, model write paths reject unavailable Providers, failed lifecycle writes do not record successful operation logs, and P18 write-path serialization rejects duplicate same-tenant same-Provider non-deleted `model_name` values without a destructive migration.
- Historical dirty rows containing non-heuristic secrets still need a future design if exact read-time scrubbing is required; P9 audit reads intentionally do not widen Provider plaintext key decryption into the admin read path without a trusted minimal secret source and lifecycle.
- Writable system settings remain constrained to fields with live runtime consumers. Tenant upload policy is backed by asset validation, task defaults are backed by task creation, `taskConcurrency` is backed by Worker semaphore acquisition, `storageRetention` is backed by Worker maintenance cleanup, `storageQuota` is backed by reference upload and Worker output persistence checks, and `logRetention` is backed by Worker database-log cleanup. Frontend settings may expose only settings with merged backend consumers; it must not expose manual cleanup triggers, raw MinIO listing, or any field without a runtime consumer.

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
- P9 runtime settings now expose only tenant upload policy and enforce it in backend asset upload validation. Deferred settings are absent from responses and rejected on writes until their runtime consumers exist.
- P9 frontend admin observability/settings UI now consumes only backend admin contracts, gates sections by `usage:read`, `audit:read`, and `system:settings:manage`, keeps lists paginated, PATCHes settings with CSRF through the shared API client, and does not persist Provider keys, auth tokens, log metadata, or system settings payloads in browser storage.
- P9 security regression added targeted tests for SSRF, redaction, tenant/object authorization, upload validation, task/SSE replay visibility, production secret guards, frontend production import safety, and deleted the unreachable legacy history display/storage helpers identified during P8/R8.
- R9 verified the full P9 code range with frontend lint/type-check/test/build, backend tests/race/vet/build, Docker Compose config/build/up/health, API health, frontend static route, and Compose cleanup. R9 found no blocking security issues.
- P10 Worker pool, SSE bridge lifecycle, and Provider/model lifecycle hardening completed without changing tenant, Provider Adapter, SSE replay, task status, or sensitive logging contracts.
- P10 frontend admin observability hardening added stale-response protection for API call details, keeps detail metadata bounded/redacted, preserves upload-policy-only system settings, and does not write Provider keys, auth tokens, log metadata, or settings payloads to browser storage.
- P10 backend history query now provides a read-only, tenant-scoped, project-authorized `GET /projects/{projectId}/history` endpoint. It uses backend-owned task output, asset, and task joins; returns only non-deleted generated/edited output assets; excludes orphan and cross-tenant rows; and does not expose object keys, MinIO URLs, image bytes, Provider secrets, auth headers, cookies, or API call metadata.
- R10 reviewed the complete P10 code range and found no blocking security issues. Validation passed frontend lint/type-check/test/build, backend tests/race/vet/build, Docker Compose config, forbidden frontend Provider/polling/storage scans, and whitespace checks.
- P11 backend user-admin APIs now enforce tenant-scoped user and role queries, require RBAC for every operation, hash newly created user passwords, redact sensitive fields from responses and operation logs, block self-disable, block loss of the last active tenant admin, require `role:manage` for assigning roles during create or role replacement, and require `user:disable` for status changes. The task passed backend tests, race tests, vet, API/Worker builds, Compose config, and whitespace checks.
- P11 frontend user/role admin UI now gates entry, user reads, role/permission reads, create/update/disable/enable, and role assignment by the current session permissions. It sends CSRF headers through the shared API client, does not call user-admin endpoints when permissions are absent, disables current-user status actions, avoids rendering unsafe response fields, and keeps created-user passwords out of localStorage, sessionStorage, IndexedDB, and post-success UI. The task passed frontend lint, type-check, targeted tests, full tests, build, and whitespace checks.
- R11 reviewed the complete P11 code range and found no blocking security issues. Validation passed frontend lint/type-check/test/build, backend tests/race/vet/build, Docker Compose config, P11 frontend/backend sensitive-pattern scans, and whitespace checks.
- P12 frontend unified-history migration now consumes the backend-owned, tenant-scoped `GET /projects/{projectId}/history` feed directly. The browser no longer builds production history by joining task lists with generated/edited asset lists. The UI preserves project-switch stale-response protection, authorized detail/download/re-edit, backend `editSourceAssetId`, non-leaky history errors, and unsafe response-field suppression.
- P12 frontend project/asset workflow polish now uses backend APIs for project edit, asset filtering, asset metadata edit, member list/add/update/remove entry points, reference upload, download, delete, favorite, and use-as-reference. It preserves CSRF write requests, stale project-switch protections, and filtered-list consistency without adding Provider direct calls, browser Provider key storage, or task polling.
- P12 backend project-member hardening now rejects deleting or downgrading the final project `OWNER`, allows owner transfer only after another `OWNER` remains, preserves tenant/RBAC/project-role checks, and verifies rejected writes do not create successful operation logs.
- R12 reviewed the complete P12 code range and found no blocking security issues. Validation passed frontend lint/type-check/test/build, backend tests/race/vet/build, Docker Compose config, forbidden frontend Provider/polling/storage scans, and whitespace checks.
- P13 backend runtime defaults now store only tenant-scoped Provider/model IDs, validate enabled same-tenant ownership on settings writes, revalidate default-backed task requests, and reject absent, cleared, stale, disabled, deleted, cross-tenant, or capability-incompatible defaults without enqueue or successful task audit side effects. Focused/full backend tests, race tests, vet, builds, Compose config, and whitespace checks passed before merge.
- P13 runtime-default hardening now converts malformed persisted `task_defaults` rows to sanitized `422 VALIDATION_ERROR` for default-backed requests with no task/event/enqueue/success-audit side effects, leaves explicit Provider/model requests independent of unused defaults, and preserves sanitized internal errors for real settings-storage failures.
- P13 task concurrency policy now exposes tenant/user/Provider/model limits only with a Worker runtime consumer. Tenant values may only narrow or match environment hard caps, global concurrency remains environment-owned, Provider row limits remain additional stricter caps, and malformed persisted `task_concurrency` fails closed before Provider execution or successful output/usage/API-call side effects.
- P13 storage cleanup foundation now uses an independent bounded cleanup context for upload rollback after object write and adds an internal tenant-scoped physical cleanup service for soft-deleted assets. It deletes only metadata-selected original/thumbnail objects older than a caller-supplied cutoff, treats missing objects as idempotent success, leaves failed deletes retryable, and tracks physical cleanup with `purged_at`.
- P13 storage retention runtime now exposes nullable `storageRetention.deletedAssetRetentionDays` only with a Worker maintenance consumer. Unset or cleared retention is disabled, malformed persisted settings fail closed, inactive tenants are skipped, and cleanup logs avoid object keys, bucket names, MinIO URLs, image bytes, Authorization, Cookie, JWT, and Provider API keys.
- P14 Provider/model lifecycle integrity prevents enabled models from pointing at disabled or deleted Providers through normal management APIs, revalidates loaded task defaults, and keeps lifecycle conflict responses non-sensitive.
- P14 backend usage/cost reporting keeps cost estimation deterministic and non-negative, treats invalid pricing as zero-cost without failing successful Provider tasks, keeps usage summary tenant-scoped across tenant/user/project/Provider/model dimensions, and preserves recursive redaction for raw usage/API metadata.
- P14 frontend cost observability consumes only same-origin backend admin usage APIs, gates usage views by `usage:read`, keeps cost display based on backend strings, avoids polling and browser Provider-key storage, and does not expose raw Provider secrets, Authorization/Cookie/JWT values, image base64, bucket names, or object keys.
- P15 core-flow E2E coverage now verifies the happy-path security envelope across init-admin, HttpOnly session cookies, readable CSRF cookie, Provider/model setup, upload validation, fake Worker execution, SSE replay, output asset download, history, usage, and logs without calling external AI Providers.
- P15 final security regression now provides `scripts/security-regression.sh` and extends the core-flow test with low-permission negative assertions for output asset download, project history, usage reads, operation logs, API call logs, and API call detail. The script also runs focused security tests, frontend production forbidden-pattern scans, backend sensitive-marker scans, frontend `/api/` proxy safety checks, Compose config validation, and whitespace checks.
- R15 release-readiness review re-ran the final security regression and deployment validation on latest `main`; no blocking security issues were found, and live Compose cleanup left no project containers or project volumes.
- P16 deploy script hardening now adds scoped cleanup traps to `scripts/deploy-release-validation.sh --up --down`, including failure and SIGINT/SIGTERM paths. Validation confirmed no project Compose containers or volumes remain after live deployment checks.
- P16 backend log retention now exposes `logRetention` only with a Worker runtime consumer. Cleanup is tenant-scoped, batch-limited, skips malformed settings fail-closed, preserves non-terminal task events for SSE/recovery, and records only sanitized aggregate audit metadata.
- P16 thumbnail policy is now backend-owned. New thumbnails are generated only from images that passed backend validation, stored in MinIO, and accessed through `GET /api/v1/assets/{assetId}/thumbnail` after login, tenant, project/member, RBAC, and object authorization checks. Responses and logs must continue to avoid bucket names, object keys, MinIO URLs, image base64, Authorization, Cookie, JWT, and Provider secrets.
- P17 orphan cleanup is conservative. It scans storage only through backend code, and deletion eligibility requires recognized backend object-key patterns plus absence from trusted MySQL metadata; a raw bucket listing is not sufficient. Dry-run and execution responses use sanitized aggregate counts and hashes/opaque IDs instead of raw object keys.
- P13 storage quota accounting now exposes nullable `storageQuota.maxBytes` only with real backend consumers. `storageQuota.usedBytes` is read-only and computed from tenant-scoped unpurged asset metadata; upload and Worker output quota failures must not create successful metadata, output events, usage records, success logs, or leak object identifiers.
- P17 storage quota reservation now makes quota enforcement safe under concurrent writers. Reservation IDs and counter internals are server-only; responses/logs/audit metadata must not leak them or any object key/bucket/MinIO URL. Failed reservations, expired reservations, cleanup failures, released reservations with positive finalize attempts, and malformed counters must fail closed without creating successful asset metadata.
- P17 production diagnostics is implemented as an admin-only, tenant-scoped, read-only, aggregate-only backend endpoint. Diagnostics must not expose raw Provider payloads, operation/API log metadata, queue payload contents, Redis keys, object keys, bucket names, MinIO URLs, signed URLs, image base64, Authorization, Cookie, JWT, or Provider secrets. Maintenance metadata parsing is fail-closed by field type: only numeric aggregate counts, boolean flags, enum statuses, and RFC3339 timestamps may survive.
- P18 Provider/model/default-setting serialization adds row locks and write-path duplicate model-name rejection so concurrent admin changes cannot silently leave unavailable default references or duplicate active model names.
- P18 optional real Provider smoke tooling remains manual and cost-bounded. Its default help/dry-run modes do not call any API, explicit `--run` requires `REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS`, and direct AI Provider API bases are rejected.
- P18 production dry-run fixed the temporary-file cleanup registration bug, added failure-path regression, and passed safe default plus live Compose rehearsal with project-scoped cleanup.
- P19 host TLS reverse-proxy hardening added an auditable Nginx template and static guardrails. Public traffic must terminate TLS and proxy only to loopback frontend `127.0.0.1:8080`; it must never route directly to backend-api, AI Providers, or relays.
- P19 API startup now reconciles missing built-in roles and grants for existing tenants without deleting custom roles or grants.
- P20 pins the CSRF request-header contract to `X-CSRF-Token` across frontend, backend, CORS, Compose, and production-env preflight. Custom header aliases are rejected fail closed.
- P20 frontend tenant/custom-role administration uses only same-origin backend APIs with CSRF-protected writes. Built-in roles remain read-only in the UI, and role drafts, passwords, tokens, and sensitive responses are not persisted in browser storage.
- P21 Provider credential lifecycle hardening now crypto-erases `encrypted_api_key`, `api_key_hint`, and `api_key_updated_at` during Provider soft delete. Provider master-key rotation apply also scrubs historical soft-deleted Provider rows that still contain credential material, while dry-run reports count-only erase candidates.
- P21 production CSRF hardening now rejects `CSRF_ENABLED=false` in `APP_ENV=production` at backend startup and in production dry-run preflight.
- P21 reliable queue hardening now uses Redis Lua atomic migration for retry, ack, dead-letter, stale recovery, and delayed promotion. MySQL-backed queued/retrying delivery reconciliation repairs missing Redis delivery state while preserving MySQL as the task source of truth.
- P21 deployment hardening now propagates a single production env file through Compose/release-validation commands, redacts health-failure logs, configures bounded `json-file` logging for long-running services, and keeps exact MinIO restore semantics in backup/restore rehearsal.
- P21 frontend workbench hardening now submits the selected Amazon image type through backend task `imageType`, preserves backend history image types on re-edit, and normalizes invalid drafts before submission.
- P21 login hardening now uses Redis-backed failed-login rate limiting keyed by an opaque hash of tenant, normalized email, and IP. Limit checks happen before user lookup, failures are counted after invalid credentials, successful login resets the counter before success audit/session response, and Redis limiter failures fail closed without echoing credentials.
- P21 migration startup hardening now serializes API/Worker migrations with a MySQL advisory lock on MySQL paths, skips the lock only for SQLite/unit-test paths, and fails closed when an applied migration's expected schema objects are missing.
- P21 quota maintenance now reconciles tenant storage quota counters from MySQL metadata through bounded rotating active-tenant batches. It does not use MinIO listing as quota truth and logs only sanitized aggregate error kinds.
- P21 Provider attempt ledger hardening now persists an `ATTEMPTING` API-call row before external Provider execution and finalizes it after success, failure, timeout, or cancellation. Prewrite failure prevents the Provider call; finalize failure fails the task closed without output/usage side effects. Request/response metadata is recursively redacted and drops object keys, buckets, MinIO URLs, signed URLs, Authorization/Cookie/JWT/API-key/base64 fields.

Storage and P5 review hardening backlog:

- Frontend settings UI now exposes only active runtime-backed settings. It shows `storageQuota.maxBytes`, read-only `storageQuota.usedBytes`, and nullable `logRetention`; the UI must still hide orphan cleanup, manual cleanup triggers, MinIO object listing, bucket names, object keys, Provider secrets, and any setting without a backend consumer.
- Built-in `asset:*` permissions and other built-in grants are reconciled for existing tenants during API startup.
- MinIO bucket creation or verification remains an environment/deployment responsibility.
- Frontend upload precheck limits are currently UX-only and not the platform security boundary. Backend upload validation remains authoritative until system upload limits are exposed to the frontend.

P20 operational controls:

- Provider master-key rotation uses the operator-only `backend/cmd/provider-key-rotation` CLI with default dry-run, explicit apply confirmation, serialized transaction processing, full rollback on any bad row, and sanitized count-only output.
- Second and later tenant provisioning uses the operator-only `backend/cmd/provision-tenant` CLI until a deliberate platform-level administration model exists. Its apply path is explicitly confirmed and transactional.
- Tenant HTTP APIs remain tenant-scoped; tenant admin sessions never imply platform-wide super-admin rights.
- Custom-role HTTP writes are tenant-scoped, CSRF protected, audited, and blocked for built-in roles. Deletion fails while a user assignment exists.
- Backup/restore/rollback rehearsal runs only against a disposable dynamically named Compose project and never against shared development or production services.

R20 release-blocking security follow-ups:

- JWT sessions need revocation semantics for logout, password change, user disable, and long-lived SSE authorization.
- SSE replay/catch-up, concurrency lease lifecycle, session revocation, Worker readiness, and final frontend legacy cleanup still must fail closed under partial failures.

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
- P9 production startup hardening rejects default placeholder `API_KEY_ENCRYPTION_KEY` before API or Worker startup in production.

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

SSRF tests are required for Provider save/update/test and real runtime execution paths.

Current P6 Provider backend status:

- Provider save/update/test SSRF tests cover blocked schemes, embedded credentials, localhost, loopback, private ranges, link-local, multicast, Docker service names, DNS resolution to blocked ranges, and redirects to blocked targets.
- P7 real Provider Adapter execution uses connect-time IP validation / SSRF-safe transport to prevent DNS rebinding between validation and connection.

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

Current R12 frontend security status:

- Production workbench generation/edit flows create backend tasks and use SSE for status.
- Static scans through R12 found no production direct Provider host, Provider `Authorization` header, Provider key persistence, sensitive browser storage, or polling loop.
- Remaining `providers` static-scan hits are backend Provider management API paths, not browser AI Provider calls.
- Remaining IndexedDB usage must stay limited to prompt templates or explicitly non-production code paths. Frontend production history now consumes the unified backend history endpoint directly.

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
