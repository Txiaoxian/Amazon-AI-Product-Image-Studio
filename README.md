# Amazon AI Product Image Studio

Amazon AI Product Image Studio 是面向亚马逊卖家的多用户 AI 产品图片平台。用户可以按商品管理项目，上传参考图，创建生成或编辑任务，并在统一历史中查看任务状态、图片资产、用量和审计记录。

当前生产路径由 React 前端、Go API、Go Worker、MySQL、Redis 和 MinIO 组成。浏览器只访问平台后端，不直接访问 OpenAI、Gemini 或其他 AI Provider。

## 平台架构

- 前端：React + TypeScript + Vite + Tailwind CSS。
- 后端 API：Go + Gin + GORM，负责认证、授权、业务 API、上传校验、任务创建、Provider/模型管理和 SSE 推送。
- 后端 Worker：从 Redis 队列领取任务，通过 Provider Adapter 调用 AI Provider，将输出写入 MinIO，并记录任务事件和用量。
- MySQL 8：保存租户、用户、项目、资产元数据、任务、事件、日志和用量，是业务数据的最终事实来源。
- Redis：用于任务队列、锁、并发限制、缓存、限流和临时状态。
- MinIO：保存原图、生成图和缩略图。MySQL 只保存元数据与 `object_key`。
- 认证与授权：JWT HttpOnly Cookie、CSRF 防护、RBAC、`tenant_id` 隔离和对象级授权。
- 任务状态：仅使用 SSE 推送，不使用轮询。

核心任务链路：

1. 用户通过后端登录并获得 HttpOnly Cookie。
2. 前端通过 `/api/v1` 创建生成或编辑任务。
3. API 将任务写入 MySQL，并投递到 Redis。
4. Worker 通过 Provider Adapter 调用配置的 AI Provider。
5. Worker 将输出图片写入 MinIO，并将任务事件写回 MySQL。
6. 前端通过 SSE 接收状态更新，并从后端读取统一历史和经过授权的图片资源。

## 目录结构

```text
gpt-image/
  frontend/            # React + TypeScript + Vite 前端
  backend/             # Go API、Worker 和领域实现
  deploy/              # Docker Compose、运行配置和部署 Runbook
  docs/                # 架构、契约、安全和开发文档
  scripts/             # 发布验证、安全回归和受控 Smoke 脚本
  agent-instructions/  # 项目级 Agent 规则
```

## 本地开发

日常开发和功能验证复用本机共享的 MySQL、Redis 和 MinIO 服务，不为普通功能开发启动项目专属数据容器。环境准备、共享服务检查和后端环境变量说明见 [`docs/local-development.md`](docs/local-development.md)。

启动前端开发服务：

```bash
cd frontend
npm install
npm run dev
```

同一局域网内访问开发服务：

```bash
cd frontend
npm run dev:host
```

## 验证命令

前端验证：

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
```

后端验证：

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker ./cmd/provider-key-rotation ./cmd/provision-tenant
```

## Docker Compose 部署

`deploy/docker-compose.yml` 用于部署拓扑和部署验证，不用于普通功能开发。首次部署前从模板创建环境文件，并替换所有生产密钥占位值：

```bash
cp .env.example .env
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
```

默认访问入口：

- 前端：`http://127.0.0.1:8080`
- 后端健康检查：`http://127.0.0.1:8081/healthz`
- 前端代理健康检查：`http://127.0.0.1:8080/api/v1/healthz`

部署、初始化管理员、MinIO Bucket、SSE 代理、备份恢复、升级回滚和清理流程见 [`deploy/RUNBOOK.md`](deploy/RUNBOOK.md)。

## 发布验证

提交发布前运行可重复的发布门禁和安全回归：

```bash
bash scripts/deploy-release-validation.sh
bash scripts/security-regression.sh
```

需要启动完整 Compose 栈进行健康检查时：

```bash
bash scripts/deploy-release-validation.sh --up --down
```

该模式会验证 Compose 服务、后端健康检查、前端 `/api/` 代理和 SSE 鉴权边界，并在结束后清理项目 Compose 容器和数据卷。

真实 Provider Smoke 是手动、显式启用且可能产生费用的检查，不属于默认发布验证或 CI。先阅读 [`deploy/RUNBOOK.md`](deploy/RUNBOOK.md) 中的 Optional Real Provider Smoke 说明。

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

更完整的安全要求见 [`docs/security.md`](docs/security.md)，平台契约与当前开发阶段见 [`docs/architecture.md`](docs/architecture.md) 和 [`docs/development-plan.md`](docs/development-plan.md)。
