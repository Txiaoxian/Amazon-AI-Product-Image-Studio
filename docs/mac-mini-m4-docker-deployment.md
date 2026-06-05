# Mac mini M4 Docker 部署说明

本文用于在本机 Mac mini M4 上验证完整 Docker Compose 部署拓扑。它会启动项目专属的 `mysql`、`redis`、`minio`、`backend-api`、`backend-worker` 和 `frontend` 容器。

日常功能开发不要使用本文方式；日常开发应使用 `docs/mac-mini-m4-local-startup.md` 中的本机代码启动方式，并复用共享本地服务。

## 适用范围

- 本机部署验证。
- Docker Desktop / Docker Compose 可用。
- Apple Silicon 原生构建，镜像会按本机架构构建。
- 不用于生产发布，也不替代 X86 服务器部署说明。

## 1. 准备环境文件

从仓库根目录执行：

```bash
cp .env.example .env.local.docker
```

编辑 `.env.local.docker`，至少替换以下占位值：

```text
MYSQL_ROOT_PASSWORD
MYSQL_PASSWORD
REDIS_PASSWORD
MINIO_ROOT_USER
MINIO_ROOT_PASSWORD
MINIO_ACCESS_KEY
MINIO_SECRET_KEY
JWT_SIGNING_SECRET
API_KEY_ENCRYPTION_KEY
```

本机 Docker 部署可以保留：

```text
APP_ENV=local
FRONTEND_BIND_HOST=127.0.0.1
FRONTEND_PORT=8080
BACKEND_API_BIND_HOST=127.0.0.1
BACKEND_API_PORT=8081
CORS_ALLOWED_ORIGINS=http://localhost:8080
COOKIE_SECURE=false
```

不要把 Provider API Key 写进环境文件。Provider API Key 必须通过后端管理接口配置。

## 2. 一次构建并启动

从仓库根目录执行：

```bash
docker compose --env-file .env.local.docker -f deploy/docker-compose.yml config
docker compose --env-file .env.local.docker -f deploy/docker-compose.yml up -d --build
docker compose --env-file .env.local.docker -f deploy/docker-compose.yml ps
```

Compose 会同时启动：

- `mysql`
- `redis`
- `minio`
- `minio-bootstrap`
- `backend-api`
- `backend-worker`
- `frontend`

`minio-bootstrap` 是一次性服务，用于创建或确认 `product-originals`、`product-generated`、`product-thumbnails` buckets。

## 3. 验证访问

默认入口：

```text
前端：http://127.0.0.1:8080
后端：http://127.0.0.1:8081
```

健康检查：

```bash
curl -fsS http://127.0.0.1:8081/healthz
curl -fsS http://127.0.0.1:8081/api/v1/healthz
curl -fsS http://127.0.0.1:8080/api/v1/healthz
```

查看日志：

```bash
docker compose --env-file .env.local.docker -f deploy/docker-compose.yml logs --tail=120 backend-api
docker compose --env-file .env.local.docker -f deploy/docker-compose.yml logs --tail=120 backend-worker
docker compose --env-file .env.local.docker -f deploy/docker-compose.yml logs --tail=120 frontend
```

## 4. 初始化管理员

API 健康后，只执行一次：

```bash
curl -fsS -X POST http://127.0.0.1:8081/api/v1/auth/init-admin \
  -H 'Content-Type: application/json' \
  -d '{
    "tenantName": "Local Docker Tenant",
    "email": "admin@example.com",
    "password": "replace-with-a-strong-local-password",
    "displayName": "Local Docker Admin"
  }'
```

初始化完成后，访问 `http://127.0.0.1:8080` 登录。

## 5. MinIO 说明

项目 Compose 内的 MinIO 默认只在 Compose 网络内使用，不暴露 Console 到宿主机。这样可以避免本机部署验证误用为长期共享对象存储。

如果需要查看对象，优先通过平台后端的鉴权下载接口验证；不要直接绕过后端读取对象作为业务验收。

## 6. 停止和清理

停止但保留数据卷：

```bash
docker compose --env-file .env.local.docker -f deploy/docker-compose.yml down
```

部署验证结束后清理项目专属容器和数据卷：

```bash
docker compose --env-file .env.local.docker -f deploy/docker-compose.yml down -v --remove-orphans
```

不要对共享本地开发服务执行上述清理命令；共享服务使用的是 `/Volumes/wohenhaoqi/data/ApplicationsData/dev-env/compose/docker-compose.yml`。

## 7. 常见问题

- `backend-api` 启动失败并提示 placeholder secret：检查 `.env.local.docker` 中是否仍有 `change-me` 占位值。生产模式会拒绝占位 secret；本机也建议替换。
- 前端能打开但 API 失败：确认 `curl http://127.0.0.1:8080/api/v1/healthz` 是否通过。前端容器只允许代理 `/api/` 到 `backend-api:8080`，不能代理 AI Provider。
- Worker 不健康：检查 MySQL、Redis、MinIO 是否健康，并查看 `backend-worker` 日志。
