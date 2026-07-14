# Business Requirements

## Objective

Build a multi-user platform for Amazon sellers to generate, edit, organize, and audit AI product images.

The product is primarily used by Chinese-speaking operators in this repository context. User-facing UI copy, configuration help text, validation feedback, and platform-owned error messages should use Simplified Chinese whenever practical. Technical identifiers such as API field names, enum values, model IDs, MIME types, and Provider type codes may remain in their canonical form.

## Users and tenants

- The platform supports multiple tenants or teams.
- Users belong to a tenant.
- Tenant data is isolated by `tenant_id`.
- Roles include at least admin, seller, and viewer.
- Operators can provision second and later tenants safely without reopening the one-time first-admin bootstrap endpoint.
- Tenant administrators can update their own tenant name and manage custom roles without gaining platform-wide access.
- Later expansion should support multiple stores or seller teams under the same platform.

## Product projects

Each Amazon product should map to a project. A project stores:

- Product name.
- Brand.
- ASIN.
- Marketplace/site.
- Notes.
- Members and permissions.
- Sort order and active/archive status.
- Reference images.
- Generated images.
- Edited images.
- Task history.
- Prompt history or prompt templates.

## Image workbench

The workbench must support:

- Project selection.
- Reference image upload or selection from asset library.
- Project tabs at the top of the workspace, showing project name and brand in project sort order.
- Prompt input.
- Each image type provides several built-in recommended prompts. A recommendation is copied into the editor and remains fully editable before submission.
- User-saved prompt templates are isolated by image type. Switching between main, A+, scene, detail, dimension, selling-point, promotion, and comparison images must not mix saved templates.
- Recommended prompts that contain dimensions, specifications, performance claims, promotions, or comparisons must require user-supplied facts and must not instruct the model to invent missing data.
- Provider selection.
- Model selection.
- Dynamic parameters based on model capabilities.
- Image type selection: main image, A+ image, scene image, detail image, selling-point image, size chart, comparison image.
- Size, quality, output format, and output count selection.
- Task submission.

## Asset management

Images must support:

- Reference, generated, and edited categories.
- Thumbnail display.
- Favorite flag.
- Soft delete.
- Download through backend authorization.
- Detail view.
- Use as reference for another edit.
- Project-level filtering.

## Task management

Generation and edit requests are durable tasks with these statuses:

- `QUEUED`
- `RUNNING`
- `SUCCEEDED`
- `FAILED`
- `CANCELLED`
- `RETRYING`
- `TIMED_OUT`

Users can cancel and retry eligible tasks. Task state must be visible in real time through SSE.

## Provider and model management

Admins manage:

- OpenAI official Providers.
- Gemini official Providers.
- OpenAI-compatible relay Providers.
- Custom relay Providers.
- Encrypted API keys.
- Base URLs.
- Enable/disable state.
- Timeout settings.
- Concurrency limits.
- Provider health tests.

Admins manage models and capabilities:

- Provider ownership.
- Model name and display name.
- Generate support.
- Edit support.
- Multiple reference image support.
- `n` parameter support.
- Maximum output count.
- Supported sizes.
- Supported quality values.
- Supported output formats.

Model size/aspect-ratio and quality configuration should be manageable through reusable preset choices rather than requiring admins to type every supported value manually. OpenAI `gpt-image-2` presets use official quality values (`auto`, `low`, `medium`, `high`) and the platform's ordered product aspect ratios; Gemini Nano Banana 2 presets keep aspect ratio independent from `1K`, `2K`, and `4K` output resolution.
- Pricing config.
- Enable/disable state.

## Usage and audit

The platform must record:

- API call logs for every Provider call.
- Token usage when available.
- Image count.
- Estimated cost.
- Raw usage payload.
- Operation logs for security and audit-sensitive actions.

Sensitive data must be redacted from all logs.
