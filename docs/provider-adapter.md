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
- P7 real Provider Adapter execution must add an outbound SSRF-safe dialer or equivalent connect-time IP validation before real generation/edit calls are allowed.

Current P6 model capability result:

- Backend model capability management is implemented and merged.
- Model records are tenant-scoped, reference a Provider in the same tenant, and expose generation/edit capabilities, multi-reference support, `n` support, max output count, supported sizes, supported qualities, supported output formats, pricing metadata, and enabled/disabled state.
- Capability and pricing JSON are validated before persistence.
- P7 Provider Adapter execution must consume these backend model records as the trusted source for allowed task parameters; it must not infer allowed image parameters from frontend constants.
- Current P7 runtime uses stable `modelId` references, so same-Provider `model_name` uniqueness was not required for execution. P10 keeps model-name uniqueness deferred and defines Provider deletion behavior for linked models.
- P10 Provider/model lifecycle policy: Provider deletion is blocked while any non-deleted same-tenant model still references the Provider. Provider disable remains allowed and does not cascade to models. Soft-deleted models do not block Provider deletion.

Current P6 frontend management result:

- Frontend Provider/model management is implemented and merged.
- Provider API keys are submitted only through authenticated backend Provider APIs, displayed only as masked metadata, and cleared from form drafts after save or modal close.
- The P6 frontend did not add browser Provider direct calls, Provider Authorization headers, task polling, or workbench generation backendization.

P7 runtime boundary:

- `P7-BE-PROVIDER-ADAPTER-RUNTIME` is the first phase allowed to execute real backend Provider generation/edit calls.
- Runtime execution must use the Provider Adapter interface and the backend model capability table as the trusted source of allowed parameters.
- Runtime execution starts after `P7-BE-WORKER-QUEUE` merged reliable queue consumption, Worker state handling, Redis SSE wakeups, and fake/stub execution.
- Browser Provider adapters under `frontend/src/providers/**` remain migration references until P8 removes or isolates them from production generation paths.

Current P7 runtime result:

- Real backend Provider Adapter execution is implemented and merged for OpenAI, Gemini, and OpenAI-compatible Providers.
- Runtime execution validates the final outbound dial target with SSRF-safe transport before connecting, in addition to save/use-time URL validation.
- Successful runtime execution writes generated/edited images to MinIO, creates assets and task outputs, records usage/API call logs, and emits output/usage/terminal task events.
- Provider errors and runtime metadata are recursively redacted. Review fixes explicitly cover the decrypted Provider API key when it appears both as a value and as a nested JSON map key.
- Unknown secrets that are not supplied to the redactor and do not match heuristic rules remain outside automatic detection; configured Provider API keys are supplied as known secrets in the active runtime path.

P8 frontend migration rule:

- The production workbench must consume backend Provider/model/task APIs only.
- `frontend/src/providers/**` may remain temporarily as explicit legacy/import reference code during migration, but it must not remain in production workbench imports after P8 completes.
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

For P6, implementations may expose only a probe/test capability. `Generate` and `Edit` may remain unimplemented until P7, but business code must still depend on interfaces rather than concrete Provider URLs or SDK calls.

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

## Existing frontend Provider code

Current frontend files under `src/providers/` are migration references only. Their behavior can guide backend adapters, but production frontend code must not call Providers directly.
