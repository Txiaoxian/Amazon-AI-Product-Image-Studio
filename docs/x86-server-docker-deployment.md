# X86 线上服务器 Docker 部署说明

本文用于在 X86 Linux 服务器上一键式部署 Amazon AI Product Image Studio。部署拓扑包含前端、后端 API、后端 Worker、MySQL 8、Redis 和 MinIO；不需要单独在宿主机安装 MySQL、Redis 或 MinIO。

生产环境必须使用 HTTPS 入口。建议用宿主机 Nginx、Caddy、Traefik 或云厂商负载均衡把公网 HTTPS 流量转发到本机 `127.0.0.1:8080`。

## 1. 服务器前置条件

- X86_64 Linux 服务器。
- 已安装 Docker Engine 和 Docker Compose plugin。
- 已安装 Git。
- 服务器能访问所需基础镜像源。
- 已准备域名和 TLS 证书，或已有反向代理 / 负载均衡。

验证：

```bash
docker version
docker compose version
git --version
uname -m
```

`uname -m` 应为 `x86_64` 或等价 AMD64 架构。

## 2. 拉取代码

```bash
git clone https://github.com/Txiaoxian/Amazon-AI-Product-Image-Studio.git
cd Amazon-AI-Product-Image-Studio
```

如果服务器已有仓库：

```bash
cd Amazon-AI-Product-Image-Studio
git pull --ff-only
```

## 3. 准备生产环境文件

```bash
cp .env.example .env
chmod 600 .env
```

编辑 `.env`。生产环境必须至少调整：

```text
APP_ENV=production
TZ=Asia/Shanghai

FRONTEND_BIND_HOST=127.0.0.1
FRONTEND_PORT=8080
BACKEND_API_BIND_HOST=127.0.0.1
BACKEND_API_PORT=8081

CORS_ALLOWED_ORIGINS=https://你的域名
COOKIE_SECURE=true
COOKIE_SAME_SITE=Lax
CSRF_ENABLED=true

MYSQL_ROOT_PASSWORD=<高强度随机值>
MYSQL_PASSWORD=<高强度随机值>
REDIS_PASSWORD=<高强度随机值>
MINIO_ROOT_USER=<高强度随机值>
MINIO_ROOT_PASSWORD=<高强度随机值>
MINIO_ACCESS_KEY=<高强度随机值>
MINIO_SECRET_KEY=<高强度随机值>
JWT_SIGNING_SECRET=<至少 32 字符高强度随机值>
API_KEY_ENCRYPTION_KEY=<至少 32 字符高强度随机值>
API_KEY_ENCRYPTION_KEY_ID=prod-v1
```

生产禁止：

- 使用任何 `change-me` 占位值。
- 把 Provider API Key 写进 `.env`。
- 把 `CORS_ALLOWED_ORIGINS` 设置为 `*`、localhost、回环地址、内网地址或 HTTP 地址。
- 把数据库、Redis、MinIO 端口直接暴露到公网。

如果服务器无法访问默认镜像镜像源，可在 `.env` 中把镜像变量改成可访问的官方或私有镜像源，例如：

```text
FRONTEND_NODE_IMAGE=node:24-alpine
FRONTEND_NGINX_IMAGE=nginx:1.29-alpine
BACKEND_GO_IMAGE=golang:1.25-alpine
BACKEND_ALPINE_IMAGE=alpine:3.22
MYSQL_IMAGE=mysql:8.4
REDIS_IMAGE=redis:7.4-alpine
MINIO_IMAGE=minio/minio:RELEASE.2025-04-22T22-12-26Z
MINIO_MC_IMAGE=minio/mc:RELEASE.2025-04-16T18-13-26Z
```

## 4. 一次部署

从仓库根目录执行：

```bash
docker compose --env-file .env -f deploy/docker-compose.yml config
docker compose --env-file .env -f deploy/docker-compose.yml up -d --build
docker compose --env-file .env -f deploy/docker-compose.yml ps
```

这一步会构建并启动：

- `frontend`
- `backend-api`
- `backend-worker`
- `mysql`
- `redis`
- `minio`
- `minio-bootstrap`

`minio-bootstrap` 会在 Compose 网络内创建或确认 required buckets，不需要单独安装 MinIO CLI。

## 5. 配置公网 HTTPS 入口

推荐公网只暴露 HTTPS。宿主机反向代理应转发：

```text
https://你的域名/      -> http://127.0.0.1:8080/
https://你的域名/api/  -> http://127.0.0.1:8080/api/
```

项目已提供 Nginx 模板：

```text
deploy/nginx/amazon-ai-product-image-studio.conf.template
```

检查模板：

```bash
bash scripts/tls-reverse-proxy-check.sh
```

如果暂时只做内网验收，可以直接访问：

```text
http://127.0.0.1:8080
```

不要在正式生产中跳过 HTTPS。

## 6. 健康检查

```bash
curl -fsS http://127.0.0.1:8081/healthz
curl -fsS http://127.0.0.1:8081/api/v1/healthz
curl -fsS http://127.0.0.1:8080/api/v1/healthz
docker compose --env-file .env -f deploy/docker-compose.yml ps
```

查看日志：

```bash
docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 backend-api
docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 backend-worker
docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 frontend
```

## 7. 初始化第一个管理员

API 健康后，只执行一次：

```bash
curl -fsS -X POST http://127.0.0.1:8081/api/v1/auth/init-admin \
  -H 'Content-Type: application/json' \
  -d '{
    "tenantName": "Default Tenant",
    "email": "admin@example.com",
    "password": "replace-with-a-strong-production-password",
    "displayName": "Admin"
  }'
```

不要把生产管理员密码写进脚本、README、工单或日志。

第二个及后续租户使用 operator CLI：

```bash
docker compose --env-file .env -f deploy/docker-compose.yml run --rm --no-deps \
  --entrypoint provision-tenant backend-api
```

详细流程见 `deploy/RUNBOOK.md`。

## 8. 备份、升级和回滚

必须持久化并备份：

- MySQL 数据卷。
- Redis 数据卷。
- MinIO 数据卷。
- 当前 `.env`，但只能进入受控 secret 备份位置。

升级：

```bash
git pull --ff-only
docker compose --env-file .env -f deploy/docker-compose.yml up -d --build
docker compose --env-file .env -f deploy/docker-compose.yml ps
```

停止服务但保留数据：

```bash
docker compose --env-file .env -f deploy/docker-compose.yml down
```

不要在生产环境执行 `down -v`，除非明确要删除所有数据卷。

备份、恢复、回滚演练请执行并阅读：

```bash
bash scripts/backup-restore-rehearsal.sh --help
deploy/RUNBOOK.md
```

## 9. 发布前最低验证

生产上线前至少执行：

```bash
bash scripts/deploy-release-validation.sh
bash scripts/security-regression.sh
docker compose --env-file .env -f deploy/docker-compose.yml config
docker compose --env-file .env -f deploy/docker-compose.yml up -d --build
docker compose --env-file .env -f deploy/docker-compose.yml ps
```

验收重点：

- 前端只能代理 `/api/` 到后端，不能代理 AI Provider。
- 后端 `/healthz` 和 `/api/v1/healthz` 健康。
- Worker 正常运行。
- MySQL、Redis、MinIO 健康。
- SSE 路由经过 HTTPS 入口时不被缓冲。
- Provider API Key 只能通过后端管理接口配置并加密落库。
