# Provider Adapter Plan

## Goal

All AI calls must go through backend Provider Adapters. Business services must not hard-code OpenAI, Gemini, or relay-specific request logic.

## Provider types

- `OPENAI`: official OpenAI image APIs.
- `GEMINI`: official Gemini image APIs.
- `OPENAI_COMPATIBLE`: OpenAI-compatible relay or custom AI gateway.

## Provider configuration

Provider records store:

- Display name.
- Type.
- Base URL.
- Encrypted API key.
- API key hint.
- Enable/disable state.
- Timeout.
- Concurrency limit.
- Optional headers or compatibility config, if later approved.

API keys must be encrypted at rest and never returned in full to the frontend.

Provider timeout policy:

- `timeout_seconds` is bounded by the backend Provider API and currently allows `1..600` seconds.
- Ten-minute timeouts are for slow image-generation calls only. They do not replace task timeout, Worker lease, cancellation, retry, or SSE lifecycle controls.
- The frontend must show this explanation in Simplified Chinese near the timeout field.

Provider master-key rotation is an operator-only maintenance workflow. It must use a backend CLI with a safe dry-run default and an explicit apply confirmation. Rotation must re-encrypt all eligible active Provider credentials in one database transaction, leave active Provider credential hints and credential-update timestamps unchanged, roll back fully if any active row cannot be processed, crypto-erase credential material from soft-deleted Provider rows, and never print plaintext keys, encrypted payloads, hints, base URLs, or tenant/object details.

Provider soft delete must crypto-erase stored credentials in the same write path. A deleted Provider record may remain as metadata/audit history, but `encrypted_api_key`, `api_key_hint`, and `api_key_updated_at` must be cleared before the delete succeeds. Historical soft-deleted rows created before this policy are scrubbed by the provider key rotation apply workflow.

Current implementation:

- `backend/cmd/provider-key-rotation` implements the operator workflow.
- Default mode validates all eligible rows without writing. `--apply` additionally requires `PROVIDER_KEY_ROTATION_CONFIRM=I_UNDERSTAND_PROVIDER_KEY_ROTATION`.
- The service serializes the database transaction, locks eligible Provider rows, re-encrypts active Provider `encrypted_api_key` values, and fails the complete rotation when any active row cannot be decrypted or re-encrypted.
- Soft-deleted Provider rows are not re-encrypted. If any deleted row still contains encrypted key material or key metadata, the apply path clears it and the dry-run path reports it as a deleted-provider erase candidate.
- Encrypted payload key IDs must match the decrypting cipher key ID. A mismatched key ID fails closed.
- Operators must run the apply path during an approved Provider-write maintenance window, then deploy API and Worker with the new active key and key ID.

## P6 management boundary

P6 implements Provider/model management and safe Provider testing only. It does not implement generation/edit execution, Redis task processing, output asset creation, or frontend workbench backendization.

P6 must produce these backend foundations:

- Tenant-scoped Provider CRUD with enable/disable and soft delete.
- API key encryption/decryption service for backend use only.
- Masked Provider responses with `apiKeyHint` and key update metadata only.
- SSRF validator used before Provider create/update and before Provider test/use.
- Sanitized Provider test/probe path with timeout, operation log, and redacted errors.
- Tenant-scoped model capability CRUD with enable/disable and structured capability validation.

P7 will use these records and helpers when implementing real Provider Adapter generation/edit calls.

Current P6 Provider security result:

- Backend Provider security foundations are implemented and merged.
- Provider records are tenant-scoped and API keys are stored as encrypted payloads.
- Provider responses expose only masked key metadata and never return encrypted key material.
- Provider test is backend-only and writes sanitized operation logs without creating tasks, assets, or usage records.
- SSRF validation is implemented for Provider save/update/test and covers blocked hostnames, blocked IP ranges, unsupported schemes, embedded credentials, and redirects to blocked targets.
- P7 real Provider Adapter execution added the required SSRF-safe outbound transport with connect-time IP validation before real generation/edit calls.

Current P6 model capability result:

- Backend model capability management is implemented and merged.
- Model records are tenant-scoped, reference a Provider in the same tenant, and expose generation/edit capabilities, multi-reference support, `n` support, max output count, supported sizes, supported qualities, supported output formats, pricing metadata, and enabled/disabled state.
- Capability and pricing JSON are validated before persistence.
- P7 Provider Adapter execution must consume these backend model records as the trusted source for allowed task parameters; it must not infer allowed image parameters from frontend constants.
- Current P7 runtime uses stable `modelId` references. P18 strengthens control-plane integrity by serializing Provider/model/default-setting writes and rejecting duplicate same-tenant same-Provider non-deleted `model_name` values in model write paths.
- P14 Provider/model lifecycle policy is implemented and merged: Provider deletion is blocked while any non-deleted same-tenant model still references the Provider; Provider disable is blocked while enabled linked models exist; model create/update/enable rejects disabled, deleted, or cross-tenant Providers; soft-deleted models do not block Provider deletion.
- P21 Provider credential lifecycle hardening is implemented: Provider delete immediately crypto-erases encrypted key material and key metadata, and provider master-key rotation apply scrubs any historical soft-deleted Provider rows that still contain credential material.

Current P6 frontend management result:

- Frontend Provider/model management is implemented and merged.
- Provider API keys are submitted only through authenticated backend Provider APIs, displayed only as masked metadata, and cleared from form drafts after save or modal close.
- The P6 frontend did not add browser Provider direct calls, Provider Authorization headers, task polling, or workbench generation backendization.

P7 runtime boundary:

- `P7-BE-PROVIDER-ADAPTER-RUNTIME` is the first phase allowed to execute real backend Provider generation/edit calls.
- Runtime execution must use the Provider Adapter interface and the backend model capability table as the trusted source of allowed parameters.
- Runtime execution starts after `P7-BE-WORKER-QUEUE` merged reliable queue consumption, Worker state handling, Redis SSE wakeups, and fake/stub execution.
- Browser Provider adapters under `frontend/src/providers/**` were removed in P8 and must not be reintroduced into production generation paths.

Current P7 runtime result:

- Real backend Provider Adapter execution is implemented and merged for OpenAI, Gemini, and OpenAI-compatible Providers.
- Business-layer `IMAGE_GENERATION` tasks that include `referenceAssetIds` remain generation tasks in the public API, but the Worker maps them to the Provider edit operation so the actual image bytes are sent to `/images/edits`. OpenAI-compatible edit requests use the multipart `image[]` field for every reference.
- Runtime execution validates the final outbound dial target with SSRF-safe transport before connecting, in addition to save/use-time URL validation.
- Successful runtime execution writes generated/edited images to MinIO, creates assets and task outputs, records usage/API call logs, and emits output/usage/terminal task events.
- Provider errors and runtime metadata are recursively redacted. Review fixes explicitly cover the decrypted Provider API key when it appears both as a value and as a nested JSON map key.
- Known Provider quota failures are normalized to `PROVIDER_INSUFFICIENT_QUOTA`, omit account balances from persisted/user-facing messages, and are not retried automatically.
- Unknown secrets that are not supplied to the redactor and do not match heuristic rules remain outside automatic detection; configured Provider API keys are supplied as known secrets in the active runtime path.

Current P21 Provider attempt ledger result:

- Runtime execution writes an `ATTEMPTING` API-call ledger before it calls the Provider Adapter runtime.
- The same ledger row is finalized after success, Provider failure, timeout, or cancellation.
- Prewrite failure prevents any external Provider call. Finalize failure fails the task closed without output/usage success side effects.
- Ledger metadata uses the runtime redactor plus an additional Provider-runtime metadata filter that removes object keys, buckets, MinIO URLs, signed URLs, Authorization/Cookie/JWT/API-key markers, and image base64 fields before persistence.

P8 frontend migration result:

- The production workbench consumes backend Provider/model/task APIs only.
- `frontend/src/providers/**` no longer exists in the production frontend source tree.
- Browser settings may retain non-sensitive UI preferences if still useful, but must not persist Provider API keys or Provider API URLs.

## Adapter interface

The backend should define an internal interface equivalent to:

```go
type ImageProviderAdapter interface {
    Generate(ctx context.Context, req ImageGenerateRequest) (ImageGenerateResult, error)
    Edit(ctx context.Context, req ImageEditRequest) (ImageGenerateResult, error)
    Test(ctx context.Context, provider ProviderConfig) error
}
```

Adapter inputs use normalized platform types. Adapter outputs use normalized image bytes, metadata, usage, and Provider request IDs.

P6 initially exposed only probe/test capability. P7 implemented `Generate` and `Edit` through backend Provider Adapters. Business code must continue to depend on interfaces rather than concrete Provider URLs or SDK calls.

Provider test behavior:

- OpenAI official and OpenAI-compatible Providers should use a lightweight authenticated endpoint when available, such as model listing or a configured health-compatible path.
- Gemini official Providers should use a lightweight authenticated endpoint when available.
- If a Provider type lacks a reliable low-cost probe, P6 may validate configuration and perform a minimal HTTP request that proves DNS, TLS, SSRF validation, timeout, and authentication handling without creating images.
- Test responses must be normalized and sanitized before persistence or API response.

## Model capability configuration

Models define:

- Supports generation.
- Supports editing.
- Supports multiple reference images.
- Supports `n`.
- Maximum output count.
- Supported sizes.
- Supported quality values.
- Supported output formats.
- Pricing config.
- Enable/disable state.

The frontend renders parameters from model capability responses, not hard-coded Provider constants.

Frontend model management provides Provider-specific capability helpers instead of a cross-product of resolution and ratio values:

- OpenAI and OpenAI-compatible `gpt-image-2` models expose a product-friendly aspect-ratio list independently from the official generation-quality values (`auto`, `low`, `medium`, `high`). The UI displays those quality values in Simplified Chinese but persists and submits the original protocol values in official order.
- The OpenAI adapter converts the configured ratio to a canonical `WIDTHxHEIGHT` immediately before both generation and edit requests. The quality value is not used as a resolution tier and is passed through unchanged.
- Gemini image models store aspect ratio separately from output resolution (`1k`, `2k`, `4k`). The adapter maps them to `generationConfig.imageConfig.aspectRatio` and uppercase `imageSize` respectively.

Canonical `gpt-image-2` ratio mapping:

| Capability value | Outbound OpenAI size |
| --- | --- |
| `auto` | `auto` |
| `1:1` | `1024x1024` |
| `1.62:1` | `1296x800` |
| `2:3` | `1024x1536` |
| `3:2` | `1536x1024` |
| `3:4` | `1152x1536` |
| `4:3` | `1536x1152` |
| `4:5` | `1024x1280` |
| `5:4` | `1280x1024` |
| `9:16` | `864x1536` |
| `16:9` | `1536x864` |
| `21:9` | `1792x768` |

These dimensions are multiples of 16 and stay within the official `gpt-image-2` aspect-ratio and resolution limits. Explicit pixel values from legacy model rows pass through unchanged. The conversion applies only to `gpt-image-2` and dated `gpt-image-2-*` snapshots, so other OpenAI-compatible model contracts are not silently rewritten.

The model editor and task form must use matching human-readable labels for those semantics. Presets are convenience templates for administrator data entry; persisted model capability JSON remains authoritative, and runtime task validation continues to use backend model rows rather than frontend constants.

## Logging

Every Provider call writes `api_call_logs` with:

- Provider ID.
- Model ID.
- Task ID.
- Duration.
- Status.
- HTTP status when available.
- Sanitized error.
- Redacted request and response metadata.

Never log API keys, Authorization headers, Cookies, or image base64.

## Large image response handling

OpenAI-compatible relays may return large `b64_json` payloads and are not required to compress them. The worker decodes the `data` array one image at a time into local temporary files, validates from those files, and streams the originals into MinIO. It must not load the full JSON response and decoded image bytes into memory at the same time.

- `PROVIDER_MAX_RESPONSE_SIZE_MB` limits the complete JSON response; default `1024`.
- `PROVIDER_MAX_OUTPUT_IMAGE_SIZE_MB` limits one decoded image; default `512` and must not exceed the response limit.
- These limits are independent of `UPLOAD_MAX_FILE_SIZE_MB`, which applies to user uploads.
- Temporary files are removed after success, failure, cancellation, timeout, or retry handoff.
- Response bodies, Base64 values, and temporary file paths must never be persisted in API call logs.

See [ADR-001](decisions/001-stream-provider-image-responses.md) for the rationale and rejected alternatives.

## SSRF protection

Provider base URLs must be validated on save and before use:

- HTTPS only by default.
- Reject URLs with embedded credentials or unsupported schemes.
- Block localhost and loopback.
- Block private IP ranges.
- Block link-local and multicast ranges.
- Block Docker-internal hostnames.
- Validate DNS resolved IPs.
- Reject redirects to forbidden targets.
- Enforce request timeouts.
- Re-check resolved IPs immediately before outbound calls; validation at save time alone is not sufficient.

Recommended P6 tests:

- Reject `http://localhost`, `http://127.0.0.1`, `http://[::1]`, private RFC1918 addresses, link-local addresses, and Docker service names such as `backend-api`, `mysql`, `redis`, and `minio`.
- Reject DNS names that resolve to blocked ranges.
- Reject redirects from an allowed URL to a blocked target.
- Accept a syntactically valid public HTTPS Provider URL.

Additional P7 requirement:

- The actual HTTP transport used for real Provider generation/edit calls must validate the final dial target at connection time. Do not rely only on URL validation performed earlier in the request flow.

## Removed frontend Provider code

The old browser Provider Adapter files under `frontend/src/providers/**` were removed in P8. If future work needs Provider behavior references, use git history or backend Provider Adapter tests; do not recreate browser-side Provider calls.
