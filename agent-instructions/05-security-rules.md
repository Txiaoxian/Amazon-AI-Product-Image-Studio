# Security Rules

## Authentication and sessions

- Use JWT in HttpOnly Cookie.
- Cookies must use Secure in production and SameSite protection.
- Frontend JavaScript must not read auth tokens.
- Add CSRF protection for state-changing endpoints when Cookie auth is used.

## Authorization

- Use RBAC plus tenant isolation.
- All object ID endpoints must perform object-level authorization.
- Project membership must be checked for project and asset access.
- Admin-only Provider/model/system settings APIs must require explicit permissions.

## Secrets

- API keys must be encrypted before storing in MySQL.
- API keys must never be returned in full to the frontend.
- Responses may include only masked key metadata, such as last 4 characters and update time.
- Logs must not contain API keys, Authorization headers, Cookies, passwords, or image base64 data.

## Provider SSRF protection

Provider `base_url` must be validated before save and before use:

- Allow only `https://` by default.
- Block localhost, loopback, private ranges, link-local ranges, multicast ranges, and Docker-internal hostnames.
- Resolve DNS and validate resolved IPs.
- Block redirects to forbidden targets.

## Upload security

- Validate declared MIME type and magic bytes.
- Allow only JPEG, PNG, and WebP.
- Forbid SVG.
- Enforce file size, dimensions, and pixel-count limits.
- Store uploads in MinIO, not MySQL.
- Downloads must go through backend authorization.

## Audit

Record operation logs for sensitive and business actions:

- Login and logout.
- User, role, Provider, model, and system setting changes.
- Project and asset changes.
- Task create, cancel, retry, failure, and completion.
