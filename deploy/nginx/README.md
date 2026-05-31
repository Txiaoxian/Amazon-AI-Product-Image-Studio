# Host Nginx TLS Reverse Proxy

Use
[`amazon-ai-product-image-studio.conf.template`](./amazon-ai-product-image-studio.conf.template)
as the production host-level Nginx template. It terminates external TLS and
proxies only to the loopback-bound frontend at `127.0.0.1:8080`.

The frontend container remains the only application upstream for this host
proxy. Do not route external traffic directly to `backend-api`, OpenAI,
Gemini, or any relay. The frontend Nginx continues to forward `/api/` to the
internal backend service.

Before enabling the site:

1. Keep public traffic blocked.
2. Copy the template into the host Nginx configuration directory.
3. Replace `__PUBLIC_HOST__` with the deployed DNS name.
4. Make the operator-managed certificate and private key available at the
   configured placeholder paths, or update those paths outside this
   repository.
5. Run `bash scripts/tls-reverse-proxy-check.sh --config <host-config>`.
6. Run `nginx -t`, then reload Nginx.

The `/api/v1/events/` location is intentionally separate. It uses HTTP/1.1,
clears `Connection`, disables proxy buffering and cache, sets long timeouts,
and returns `X-Accel-Buffering: no` so SSE task updates continue streaming.

Certificate issuance and renewal are operator or platform responsibilities.
Do not commit real certificates or private keys.
