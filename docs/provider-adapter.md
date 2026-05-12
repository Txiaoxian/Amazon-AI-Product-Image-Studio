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

## Existing frontend Provider code

Current frontend files under `src/providers/` are migration references only. Their behavior can guide backend adapters, but production frontend code must not call Providers directly.
