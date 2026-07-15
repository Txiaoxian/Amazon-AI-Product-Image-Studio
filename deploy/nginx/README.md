# 主机 Nginx TLS 反向代理

使用
[`amazon-ai-product-image-studio.conf.template`](./amazon-ai-product-image-studio.conf.template)
作为生产环境主机级 Nginx 模板。该模板负责终止外部 TLS，并且
只代理到绑定在环回地址 `127.0.0.1:8080` 的前端。

前端容器仍是该主机反向代理的唯一上游应用。不要将外部流量
直接路由到 `backend-api`、OpenAI、Gemini 或任何中转站。
前端 Nginx 继续将 `/api/` 转发到内部后端服务。

启用站点之前：

1. 保持公网流量关闭。
2. 将模板复制到主机 Nginx 配置目录中。
3. 将 `__PUBLIC_HOST__` 替换为部署的 DNS 名称。
4. 将运维人员管理的证书和私钥放到配置中的占位路径，或者在
   仓库外更新这些路径。
5. 运行 `bash scripts/tls-reverse-proxy-check.sh --config <host-config>`。
6. 运行 `nginx -t`，然后重新加载 Nginx。

`/api/v1/events/` 路径有意使用独立配置。它使用 HTTP/1.1，
清除 `Connection`，禁用代理缓冲与缓存，设置较长超时时间，
并返回 `X-Accel-Buffering: no`，确保 SSE 任务更新能够持续流式传输。

证书的签发和续期由运维人员或平台负责。
不要提交真实的证书或私钥。
