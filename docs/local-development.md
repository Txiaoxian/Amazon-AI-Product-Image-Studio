# 本地开发环境

本项目的日常本地开发使用已经存在的机器级共享服务，不为普通功能开发额外启动项目专属 MySQL、Redis 或 MinIO 容器。

详细启动步骤见：

- `docs/mac-mini-m4-local-startup.md`：Mac mini M4 本机前端、后端 API、Worker 启动说明。
- `docs/mac-mini-m4-docker-deployment.md`：Mac mini M4 上的完整 Docker Compose 部署验证。

## 真实凭据来源

本机共享服务的真实密码只记录在全局说明中：

```text
/Users/wohenhaoqi/.codex/agent-instructions/10-local-dev-environment.md
```

项目文档、`.env.example`、README、日志和最终交付说明不得写入真实本机密码或生产 secret。

## 开发策略

- 后续开发任务允许使用共享本地 MySQL、Redis、MinIO 进行功能验证。
- 允许对当前任务拥有的测试数据执行增删改查。
- 测试数据应使用清晰前缀，例如 `codex_`、阶段名、任务名或分支名。
- 不要删除非当前任务创建的数据。
- 不要执行 `FLUSHALL`、`FLUSHDB`、删除 MinIO bucket、删除项目数据库等全局破坏操作，除非用户明确要求。
- `deploy/docker-compose.yml` 是部署拓扑，不是普通开发默认环境。
- 如果部署专项验证启动了项目 Compose 栈，验证结束后应清理，除非用户要求保留：

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

## 共享服务概览

| 服务 | 容器 | 本机地址 | 说明 |
| --- | --- | --- | --- |
| MySQL 8 | `dev-mysql8` | `127.0.0.1:3306` | 项目库 `amazon_ai_image_studio` |
| Redis | `dev-redis` | `127.0.0.1:6379` | 本机开发无密码 |
| MinIO | `dev-minio` | API `http://127.0.0.1:9000`，Console `http://127.0.0.1:9001` | buckets：`product-originals`、`product-generated`、`product-thumbnails` |

共享服务 Compose 文件：

```text
/Volumes/wohenhaoqi/data/ApplicationsData/dev-env/compose/docker-compose.yml
```

查看状态：

```bash
docker compose -f /Volumes/wohenhaoqi/data/ApplicationsData/dev-env/compose/docker-compose.yml ps
```

启动共享服务：

```bash
docker compose -f /Volumes/wohenhaoqi/data/ApplicationsData/dev-env/compose/docker-compose.yml start
```

## 后端本地环境变量

后端直接从环境变量读取配置，不会自动读取 `.env`。建议创建本地忽略文件 `.env.local.backend`，并用 `source` 导入。完整示例见 `docs/mac-mini-m4-local-startup.md`。

最小关键变量：

```bash
export APP_ENV=local
export BACKEND_HTTP_HOST=127.0.0.1
export BACKEND_HTTP_PORT=8081

export MYSQL_HOST=127.0.0.1
export MYSQL_PORT=3306
export MYSQL_DATABASE=amazon_ai_image_studio
export MYSQL_USER=root
export MYSQL_PASSWORD='<从全局本机环境文档读取>'

export REDIS_ADDR=127.0.0.1:6379
export REDIS_PASSWORD=

export MINIO_ENDPOINT=http://127.0.0.1:9000
export MINIO_ACCESS_KEY=minioadmin
export MINIO_SECRET_KEY='<从全局本机环境文档读取>'
export MINIO_BUCKET_ORIGINALS=product-originals
export MINIO_BUCKET_GENERATED=product-generated
export MINIO_BUCKET_THUMBNAILS=product-thumbnails

export JWT_SIGNING_SECRET=local-dev-jwt-secret-at-least-32-bytes
export API_KEY_ENCRYPTION_KEY=local-dev-provider-key-at-least-32-bytes
export API_KEY_ENCRYPTION_KEY_ID=local-dev-v1
export CORS_ALLOWED_ORIGINS=http://localhost:5173,http://127.0.0.1:5173
export COOKIE_SECURE=false
export CSRF_ENABLED=true
```

不要把 Provider API Key 写入环境文件；Provider API Key 必须通过后端管理接口配置。

## 常用启动命令

后端 API：

```bash
cd backend
set -a
source ../.env.local.backend
set +a
go run ./cmd/api
```

后端 Worker：

```bash
cd backend
set -a
source ../.env.local.backend
set +a
go run ./cmd/worker
```

前端：

```bash
cd frontend
npm ci
VITE_DEV_API_PROXY_TARGET=http://127.0.0.1:8081 npm run dev
```

## 常用验证

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

## 部署验证例外

部署验证可以使用项目 Compose 栈：

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
```

部署验证结束后清理：

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

不要把共享本地服务凭据复制到 `.env`、`.env.example`、Compose 文件、文档、日志或最终交付说明中。
