# Amazon AI Product Image Studio

Amazon AI Product Image Studio 是面向亚马逊卖家的多用户 AI 产品图片生成与编辑平台。平台支持按商品建立项目、上传参考图、创建生成/编辑任务、通过 SSE 查看任务状态，并在统一资产库和历史记录中管理图片、用量和审计信息。

浏览器只访问平台后端，不直接访问 OpenAI、Gemini 或任何 AI 中转站。

## 当前架构

- 前端：React + TypeScript + Vite + Tailwind CSS。
- 后端 API：Go + Gin + GORM，负责认证、RBAC、租户隔离、业务 API、上传校验、任务创建、Provider/模型管理和 SSE。
- 后端 Worker：从 Redis 队列领取任务，通过 Provider Adapter 调用 AI Provider，将输出写入 MinIO，并记录任务事件、用量和日志。
- MySQL 8：业务数据最终事实来源，保存租户、用户、角色、项目、资产元数据、任务、事件、日志和用量。
- Redis：任务队列、锁、并发限制、限流、SSE 桥接和临时状态。
- MinIO：保存原图、生成图和缩略图；MySQL 只保存元数据和 `object_key`。
- 部署：Docker Compose，包含 `frontend`、`backend-api`、`backend-worker`、`mysql`、`redis`、`minio`。

## 目录结构

```text
gpt-image/
  frontend/            # React + TypeScript + Vite 前端
  backend/             # Go API、Worker、领域服务和迁移
  deploy/              # Docker Compose、Nginx 模板、部署 Runbook
  docs/                # 架构、契约、安全、开发和部署文档
  scripts/             # 发布验证、安全回归、运维脚本
  agent-instructions/  # 项目级 Agent 规则
```

## 快速文档入口

- [Mac mini M4 本地启动说明](docs/mac-mini-m4-local-startup.md)：本机直接启动前端、后端 API、Worker，并连接共享本地 MySQL、Redis、MinIO。
- [Mac mini M4 Docker 部署说明](docs/mac-mini-m4-docker-deployment.md)：本机完整 Docker Compose 部署验证。
- [X86 线上服务器 Docker 部署说明](docs/x86-server-docker-deployment.md)：线上 X86 Linux 服务器一键式 Compose 部署。
- [本地开发环境](docs/local-development.md)：共享本地服务策略、数据使用规则和验证命令。
- [部署计划](docs/deployment.md)：部署拓扑、健康检查、发布验证和运维要求。
- [部署 Runbook](deploy/RUNBOOK.md)：生产初始化、租户创建、Provider 密钥轮换、备份恢复、升级回滚和排障。

## 本地直接启动

日常开发推荐使用本机代码启动方式，并复用共享本地服务：

```bash
# 后端 API
cd backend
set -a
source ../.env.local.backend
set +a
go run ./cmd/api
```

```bash
# 后端 Worker
cd backend
set -a
source ../.env.local.backend
set +a
go run ./cmd/worker
```

```bash
# 前端
cd frontend
npm ci
VITE_DEV_API_PROXY_TARGET=http://127.0.0.1:8081 npm run dev
```

详细环境变量、MySQL/Redis/MinIO 连接方式和初始化步骤见 [Mac mini M4 本地启动说明](docs/mac-mini-m4-local-startup.md)。

## 本机 Docker 部署验证

本机完整 Compose 部署会启动项目专属 MySQL、Redis、MinIO，不用于普通功能开发：

```bash
cp .env.example .env.local.docker
docker compose --env-file .env.local.docker -f deploy/docker-compose.yml config
docker compose --env-file .env.local.docker -f deploy/docker-compose.yml up -d --build
docker compose --env-file .env.local.docker -f deploy/docker-compose.yml ps
```

默认入口：

```text
前端：http://127.0.0.1:8080
后端：http://127.0.0.1:8081
```

详细说明见 [Mac mini M4 Docker 部署说明](docs/mac-mini-m4-docker-deployment.md)。

## X86 服务器 Docker 部署

线上服务器最小部署路径：

```bash
git clone https://github.com/Txiaoxian/Amazon-AI-Product-Image-Studio.git
cd Amazon-AI-Product-Image-Studio
cp .env.example .env
chmod 600 .env
# 编辑 .env，替换所有生产 secret，并设置 APP_ENV=production、CORS_ALLOWED_ORIGINS、COOKIE_SECURE=true
docker compose --env-file .env -f deploy/docker-compose.yml config
docker compose --env-file .env -f deploy/docker-compose.yml up -d --build
docker compose --env-file .env -f deploy/docker-compose.yml ps
```

生产环境必须使用 HTTPS 反向代理。详细说明见 [X86 线上服务器 Docker 部署说明](docs/x86-server-docker-deployment.md)。

## 健康检查

```bash
curl -fsS http://127.0.0.1:8081/healthz
curl -fsS http://127.0.0.1:8081/api/v1/healthz
curl -fsS http://127.0.0.1:8080/api/v1/healthz
```

初始化第一个管理员：

```bash
curl -fsS -X POST http://127.0.0.1:8081/api/v1/auth/init-admin \
  -H 'Content-Type: application/json' \
  -d '{
    "tenantName": "Default Tenant",
    "email": "admin@example.com",
    "password": "replace-with-a-strong-password",
    "displayName": "Admin"
  }'
```

## 验证命令

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

部署与安全：

```bash
bash scripts/deploy-release-validation.sh
bash scripts/security-regression.sh
docker compose -f deploy/docker-compose.yml config
```

## 安全基线

- 前端不得直接调用 OpenAI、Gemini、其他 AI Provider 或 AI relay。
- 前端不得在 `localStorage`、`sessionStorage`、IndexedDB、源码或客户端可见配置中保存 Provider API Key。
- Provider API Key 只能通过后端管理接口配置，必须加密落库，且不得完整返回前端。
- Provider `base_url` 必须经过 SSRF 防护，阻止 localhost、回环、私网、链路本地和 Docker 内部目标。
- 任务状态只能通过 SSE 推送，不得使用轮询、`setInterval` 或重复 fetch 循环。
- 图片必须存储在 MinIO；上传必须校验真实文件类型、大小、尺寸和像素数，并禁止 SVG。
- 图片下载必须经过后端鉴权；所有对象 ID API 必须执行对象级授权。
- 所有业务表和租户范围查询必须使用 `tenant_id` 隔离。
- 日志不得包含完整 API Key、Authorization Header、Cookie、密码或图片 base64 数据。

更完整的架构、安全和开发计划见：

- [架构文档](docs/architecture.md)
- [API 合同](docs/api-contract.md)
- [SSE 合同](docs/sse-contract.md)
- [安全文档](docs/security.md)
- [开发计划](docs/development-plan.md)
