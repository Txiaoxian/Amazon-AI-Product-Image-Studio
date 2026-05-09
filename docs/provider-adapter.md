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
- Block localhost and loopback.
- Block private IP ranges.
- Block link-local and multicast ranges.
- Block Docker-internal hostnames.
- Validate DNS resolved IPs.
- Reject redirects to forbidden targets.

## Existing frontend Provider code

Current frontend files under `src/providers/` are migration references only. Their behavior can guide backend adapters, but production frontend code must not call Providers directly.
