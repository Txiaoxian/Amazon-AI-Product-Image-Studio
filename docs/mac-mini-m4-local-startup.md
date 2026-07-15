# Mac mini M4 本地启动说明

本文用于在本机 macOS / Apple Silicon 环境中直接启动前端、后端 API 和 Worker。日常功能开发优先使用这种方式：代码在本机运行，MySQL、Redis、MinIO 使用已经存在的共享本地开发服务。

不要为普通开发额外启动项目专属 MySQL、Redis、MinIO 容器；项目 Compose 栈只用于部署验证。

## 适用范围

- 本机：Mac mini M4 / macOS / arm64。
- 前端：`frontend/` 内的 React + Vite 开发服务。
- 后端：`backend/` 内的 Go API 与 Go Worker。
- 数据服务：复用全局本机开发环境中的 `dev-mysql8`、`dev-redis`、`dev-minio`。

真实本机密码只记录在全局说明中：

```text
/Users/wohenhaoqi/.codex/agent-instructions/10-local-dev-environment.md
```
项目文档不得写入真实本机密码或生产 secret。

## 1. 检查共享本地服务

共享服务 Compose 文件：

```text
/Volumes/wohenhaoqi/data/ApplicationsData/dev-env/compose/docker-compose.yml
```

查看状态：

```bash
docker compose -f /Volumes/wohenhaoqi/data/ApplicationsData/dev-env/compose/docker-compose.yml ps
```

如果服务已存在但停止，启动它们：

```bash
docker compose -f /Volumes/wohenhaoqi/data/ApplicationsData/dev-env/compose/docker-compose.yml start
```

首次或需要重建共享环境时：

```bash
docker compose -f /Volumes/wohenhaoqi/data/ApplicationsData/dev-env/compose/docker-compose.yml up -d
```

## 2. 本机服务连接信息

| 服务 | 容器 | 本机地址 | 本项目用途 |
| --- | --- | --- | --- |
| MySQL 8 | `dev-mysql8` | `127.0.0.1:3306` | 租户、用户、项目、资产元数据、任务、日志、用量 |
| Redis | `dev-redis` | `127.0.0.1:6379` | 队列、锁、限流、SSE 桥接、临时状态 |
| MinIO | `dev-minio` | API `http://127.0.0.1:9000`，Console `http://127.0.0.1:9001` | 原图、生成图、缩略图 |

本项目数据库：

```text
amazon_ai_image_studio
```

本项目 MinIO buckets：

```text
product-originals
product-generated
product-thumbnails
```

## 3. 准备 MySQL 数据库

如果数据库不存在，创建它：

```bash
docker exec -e MYSQL_PWD='xiaolong20' dev-mysql8 \
  mysql -uroot -e "CREATE DATABASE IF NOT EXISTS amazon_ai_image_studio CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;"
```

检查 MySQL：

```bash
docker exec -e MYSQL_PWD='xiaolong20' dev-mysql8 \
  mysql -uroot -e "SHOW DATABASES LIKE 'amazon_ai_image_studio';"
```

## 4. 准备 MinIO Buckets

如果本机安装了 `mc`：

```bash
mc alias set studio-local http://127.0.0.1:9000 minioadmin 'xiaolong20'
mc mb --ignore-existing studio-local/product-originals
mc mb --ignore-existing studio-local/product-generated
mc mb --ignore-existing studio-local/product-thumbnails
```

也可以通过 MinIO Console 创建：

```text
http://127.0.0.1:9001
```

登录账号和 secret 从全局本机环境文档读取。

## 5. 准备后端本地环境变量

后端不会自动读取 `.env` 文件。建议在仓库根目录创建一个本地忽略文件，例如 `.env.local.backend`。该文件会被 `.gitignore` 忽略，禁止提交。

```bash
cat > .env.local.backend <<'EOF'
APP_ENV=local
LOG_LEVEL=info
TZ=Asia/Shanghai

BACKEND_HTTP_HOST=127.0.0.1
BACKEND_HTTP_PORT=8081
API_READ_TIMEOUT=15s
API_WRITE_TIMEOUT=15s

MYSQL_HOST=127.0.0.1
MYSQL_PORT=3306
MYSQL_DATABASE=amazon_ai_image_studio
MYSQL_USER=root
MYSQL_PASSWORD='xiaolong20'
MYSQL_CONNECT_TIMEOUT=10s
MYSQL_MAX_OPEN_CONNS=25
MYSQL_MAX_IDLE_CONNS=5
MYSQL_CONN_MAX_LIFETIME=30m
MIGRATIONS_MODE=startup-gate

REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0

MINIO_ENDPOINT=http://127.0.0.1:9000
MINIO_REGION=us-east-1
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY='xiaolong20'
MINIO_BUCKET_ORIGINALS=product-originals
MINIO_BUCKET_GENERATED=product-generated
MINIO_BUCKET_THUMBNAILS=product-thumbnails

JWT_SIGNING_SECRET=local-dev-jwt-secret-at-least-32-bytes
JWT_ISSUER=amazon-ai-product-image-studio
JWT_ACCESS_TOKEN_TTL_MINUTES=60
AUTH_COOKIE_NAME=studio_auth
COOKIE_SECURE=false
COOKIE_SAME_SITE=Lax
CSRF_ENABLED=true
CSRF_COOKIE_NAME=studio_csrf
CSRF_HEADER_NAME=X-CSRF-Token
CORS_ALLOWED_ORIGINS=http://localhost:5173,http://127.0.0.1:5173

API_KEY_ENCRYPTION_KEY=local-dev-provider-key-at-least-32-bytes
API_KEY_ENCRYPTION_KEY_ID=local-dev-v1

UPLOAD_MAX_FILE_SIZE_MB=25
UPLOAD_MAX_WIDTH=8192
UPLOAD_MAX_HEIGHT=8192
UPLOAD_MAX_PIXELS=40000000
UPLOAD_ALLOWED_MIME_TYPES=image/jpeg,image/png,image/webp

TASK_QUEUE_NAME=image-tasks
TASK_GLOBAL_CONCURRENCY=8
TASK_POLICY_MAX_CONCURRENCY=8
TASK_TENANT_CONCURRENCY=2
TASK_USER_CONCURRENCY=2
TASK_PROVIDER_CONCURRENCY=2
TASK_MODEL_CONCURRENCY=2
WORKER_CONCURRENCY=8
WORKER_RETENTION_MAINTENANCE_INTERVAL=1h
WORKER_RETENTION_MAINTENANCE_BATCH_LIMIT=100

PROVIDER_TIMEOUT_SECONDS=120
PROVIDER_MAX_RETRIES=2
EOF
```

写入后，把占位的 MySQL 和 MinIO secret 替换成本机真实值。不要把 Provider API Key 写入该文件；Provider API Key 必须通过后端管理接口配置并加密保存。

## 6. 启动后端 API

新终端：

```bash
cd /Volumes/wohenhaoqi/data/Projects/gpt-image/backend
set -a
source ../.env.local.backend
set +a
go run ./cmd/api
```

健康检查：

```bash
curl -fsS http://127.0.0.1:8081/healthz
curl -fsS http://127.0.0.1:8081/api/v1/healthz
```

首次本地初始化管理员：

```bash
curl -fsS -X POST http://127.0.0.1:8081/api/v1/auth/init-admin \
  -H 'Content-Type: application/json' \
  -d '{
    "tenantName": "Local Tenant",
    "email": "admin@example.com",
    "password": "replace-with-a-strong-local-password",
    "displayName": "Local Admin"
  }'
```

`init-admin` 只允许初始化第一个租户管理员。平台已经初始化后，该接口会返回冲突。

## 7. 启动后端 Worker

再开一个终端：

```bash
cd /Volumes/wohenhaoqi/data/Projects/gpt-image/backend
set -a
source ../.env.local.backend
set +a
go run ./cmd/worker
```

Worker 依赖 MySQL、Redis、MinIO 都可用后才会进入可处理任务状态。

## 8. 启动前端

前端 Vite 配置已经将 `/api` 代理到后端 API，默认目标为 `http://127.0.0.1:8081`。

```bash
cd /Volumes/wohenhaoqi/data/Projects/gpt-image/frontend
npm ci
VITE_DEV_API_PROXY_TARGET=http://127.0.0.1:8081 npm run dev
```

访问：

```text
http://127.0.0.1:5173
```

如果需要在局域网内访问本机开发服务：

```bash
cd /Volumes/wohenhaoqi/data/Projects/gpt-image/frontend
VITE_DEV_API_PROXY_TARGET=http://127.0.0.1:8081 npm run dev:host
```

## 9. 常用验证命令

前端：

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
```

后端：

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker ./cmd/provider-key-rotation ./cmd/provision-tenant
```

共享服务：

```bash
docker exec dev-redis redis-cli ping
curl -fsS http://127.0.0.1:9000/minio/health/live
```

## 10. 本地数据使用规则

- 后续开发任务允许使用共享本地 MySQL、Redis、MinIO 做功能验证，包括增删改查测试数据。
- 测试数据应使用清晰前缀，例如 `codex_`、任务名或分支名。
- 不要删除非当前任务创建的数据。
- 不要执行 `FLUSHALL`、`FLUSHDB`、删除 MinIO bucket、删除项目数据库等全局破坏操作，除非用户明确要求。
- 本地 Provider API Key 仍必须通过后端管理接口配置，不能写进前端、环境模板、README 或日志。
