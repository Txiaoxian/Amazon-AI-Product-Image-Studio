# Codex Agent Task Plan

## 调度原则

本项目采用“先串行定公共合同 -> 有限并行开发 -> 串行 review 和集成”的方式推进。

主 agent 职责：

- 串行维护公共合同文件。
- 切分 worktree 任务和写入范围。
- Review 子 agent 结果，解决冲突，更新合同。
- 串行集成并运行跨模块验证。

子 agent 职责：

- 只在自己的独立 worktree 内工作。
- 只修改任务允许的文件。
- 不修改公共合同文件。
- 不绕过 AGENTS.md、`agent-instructions/` 和 `docs/` 中的强制规则。

## 公共合同文件

以下文件只能由主 agent 修改：

- `AGENTS.md`
- `agent-instructions/**`
- `docs/architecture.md`
- `docs/business-requirements.md`
- `docs/database-schema.md`
- `docs/api-contract.md`
- `docs/sse-contract.md`
- `docs/rbac.md`
- `docs/provider-adapter.md`
- `docs/task-queue.md`
- `docs/storage.md`
- `docs/security.md`
- `docs/deployment.md`
- `docs/local-development.md`
- `docs/development-plan.md`
- `docs/codex-agent-tasks.md`

子 agent 如果发现合同缺失、冲突或不可实现，只能在最终回复中报告问题，不能直接修改上述文件。

## 本地开发环境规则

开发和功能验证必须优先使用 `docs/local-development.md` 记录的全局本地环境：

- MySQL 使用 `dev-mysql8`。
- Redis 使用 `dev-redis`。
- MinIO 使用 `dev-minio`。
- 子 agent 不得为了普通功能开发启动项目专属 MySQL、Redis、MinIO 容器或创建项目专属 Docker 数据卷。
- `deploy/docker-compose.yml` 只用于部署骨架或部署回归验证；如需启动项目 Compose 栈，验证后必须清理，除非用户明确要求保留。
- 不得把全局本地环境中的真实密码复制到项目文档、源码、测试或日志中。

## 串行阶段 0：主 agent 冻结公共合同

### 任务名称

T0 - 公共合同冻结与 worktree 准备

### 目标

由主 agent 在任何子 agent 开工前确认 API、SSE、数据库、RBAC、Provider、队列、存储、安全和部署合同已经足够支撑第一批开发。

### 允许修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`

### 禁止修改文件

- `src/**`
- `public/**`
- `backend/**`
- `frontend/**`
- `deploy/**`
- `package.json`
- `package-lock.json`
- `Dockerfile`
- `nginx.conf`
- 任何业务代码文件

### 前置依赖

- P0 文档已创建。

### 具体开发内容

- 检查公共合同是否互相冲突。
- 明确第一批并行任务的写入范围。
- 为每个子 agent 创建独立 worktree 或分支。
- 明确第一批最多 3 个子 agent 并行。

### 安全要求

- 合同必须保留这些硬约束：前端不直连 AI Provider、不保存 API Key、不轮询任务状态；后端必须使用 Provider Adapter、SSE、RBAC、`tenant_id`、MinIO、API Key 加密和日志脱敏。

### 验收标准

- 公共合同文件没有明显冲突。
- 第一批子 agent 的允许修改文件互不重叠。
- 子 agent 禁止修改公共合同文件的规则明确。

### 测试命令

```bash
find docs agent-instructions -maxdepth 2 -type f | sort
git diff --check -- docs AGENTS.md agent-instructions
```

## 第一批有限并行开发

第一批最多 3 个子 agent 并行。三个任务分别处理前端机械搬迁、后端骨架、部署骨架，写入范围互不重叠。

## 子任务 1：前端机械搬迁

### 任务名称

P1-FE - 将现有前端机械移动到 `frontend/`

### 目标

把当前根目录前端工程整体移动到 `frontend/`，保持现有 UI、交互、测试和构建行为不变。

### 允许修改文件

- `frontend/**`
- 当前根目录前端文件的移动或删除：
  - `src/**`
  - `public/**`
  - `index.html`
  - `manifest.json`
  - `package.json`
  - `package-lock.json`
  - `vite.config.ts`
  - `vitest.config.ts`
  - `tsconfig.json`
  - `tsconfig.app.json`
  - `tsconfig.node.json`
  - `eslint.config.js`
  - `tailwind.config.ts`
  - `postcss.config.js`
  - `Dockerfile`
  - `nginx.conf`
  - `README.md` 仅允许更新前端命令路径说明，不允许改平台合同

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `.env.example`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- 任何后端业务代码

### 前置依赖

- T0 公共合同冻结完成。
- 当前前端测试在根目录通过。

### 具体开发内容

- 新建 `frontend/`。
- 将现有前端源码、配置、静态资源和 Docker 静态部署文件移动到 `frontend/`。
- 修正路径引用、npm 脚本、Dockerfile build context 和测试配置。
- 保持当前 UI 和行为不变，不改业务逻辑。
- 保留现有 Dexie、localStorage、前端 Provider 代码作为后续迁移基线，不在本任务替换。

### 安全要求

- 不新增任何 AI Provider 调用。
- 不新增任何 API Key 存储路径。
- 不引入轮询任务状态逻辑。
- 不扩大现有静态 Nginx relay 能力；如移动 `nginx.conf`，只保持原行为。

### 验收标准

- 前端工程从 `frontend/` 可安装、测试和构建。
- 现有测试通过。
- UI 代码只发生路径搬迁和必要引用修复。
- 根目录不再混杂前端应用根语义。

### 测试命令

```bash
cd frontend
npm ci
npm run lint
npm run type-check
npm run test
npm run build
```

## 子任务 2：后端基础骨架

### 任务名称

P1-BE - 创建 Go API 与 Worker 基础骨架

### 目标

创建可编译、可测试的 `backend/` Go 工程骨架，为后续认证、数据库、队列和 Provider 模块提供基础目录和启动入口。

### 允许修改文件

- `backend/**`

### 禁止修改文件

- `frontend/**`
- 当前根目录前端文件
- `deploy/**`
- `.env.example`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`

### 前置依赖

- T0 公共合同冻结完成。
- 不依赖 P1-FE 完成，可与 P1-FE 并行。

### 具体开发内容

- 创建 `backend/go.mod` 和必要的 `go.sum`。
- 创建 `backend/cmd/api` 和 `backend/cmd/worker` 入口。
- 创建基础包结构：`internal/config`、`internal/logger`、`internal/httpx`、`internal/health`。
- API 服务提供 `/healthz`。
- Worker 提供可启动的空 worker 入口和健康日志。
- 配置从环境变量读取，但不连接真实 MySQL、Redis、MinIO。
- 不实现认证、业务 API、Provider 调用或数据库模型。

### 安全要求

- 不硬编码密钥、数据库密码、Provider Key 或默认生产凭据。
- 日志不得输出环境变量完整内容。
- HTTP 基础中间件预留 request ID 和 panic recovery。

### 验收标准

- `backend/` 可以独立 `go test ./...`。
- API 和 worker 入口能编译。
- `/healthz` 返回简单健康状态。
- 未实现任何业务行为。

### 测试命令

```bash
cd backend
go test ./...
go vet ./...
go test -race ./...
```

## 子任务 3：部署骨架

### 任务名称

P1-DEPLOY - 创建 Docker Compose 与环境变量骨架

### 目标

创建 `deploy/` 和根 `.env.example`，定义平台服务拓扑、环境变量名称、数据卷和健康检查，为后续本地部署提供骨架。

### 允许修改文件

- `deploy/**`
- `.env.example`
- `.gitignore` 仅允许补充本项目环境文件忽略规则

### 禁止修改文件

- `frontend/**`
- 当前根目录前端源码和配置
- `backend/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `package.json`
- `package-lock.json`

### 前置依赖

- T0 公共合同冻结完成。
- 不依赖 P1-FE 或 P1-BE 完成，可与二者并行。

### 具体开发内容

- 创建 `deploy/docker-compose.yml`。
- 定义服务：`frontend`、`backend-api`、`backend-worker`、`mysql`、`redis`、`minio`。
- 添加 MySQL、Redis、MinIO 数据卷。
- 添加基础 healthcheck。
- 创建 `deploy/mysql/init/`、`deploy/minio/`、`deploy/nginx/` 占位目录或说明文件。
- 创建 `.env.example`，覆盖 MySQL、Redis、MinIO、JWT、Cookie、CORS、API Key 加密、上传限制、并发限制和 Provider timeout 变量。

### 安全要求

- `.env.example` 只能包含占位值，不得包含真实密钥。
- Redis/MySQL/MinIO 默认不得暴露不必要的公网端口。
- Compose 中不得写入真实 Provider API Key。
- JWT、Cookie、API Key 加密等变量必须明确要求生产环境更换。

### 验收标准

- Compose 配置语法有效。
- 服务名与 `docs/deployment.md` 一致。
- `.env.example` 变量覆盖平台启动所需基础配置。
- 不要求本任务完成可运行业务服务。

### 测试命令

```bash
docker compose -f deploy/docker-compose.yml config
git diff --check -- deploy .env.example .gitignore
```

## 串行阶段 1：第一批 review 和集成

### 任务名称

R1 - 第一批结果 review 与集成

### 目标

主 agent 串行 review 三个 worktree 的变更，解决路径、服务名、环境变量和测试命令冲突，形成可继续开发的平台基础。

### 允许修改文件

- 所有第一批任务允许修改的文件。
- 公共合同文件，仅限主 agent 根据实际集成结果修正。

### 禁止修改文件

- 未经 review 的业务模块实现。
- Provider 调用、认证、RBAC、任务队列、SSE 等业务功能代码。

### 前置依赖

- P1-FE、P1-BE、P1-DEPLOY 完成并提交各自结果。

### 具体开发内容

- 审查第一批各 worktree 的 diff。
- 确认 `frontend/`、`backend/`、`deploy/` 不互相覆盖。
- 串行合并或手工整合变更。
- 若集成发现合同需要调整，由主 agent 修改对应 `docs/` 文件。
- 运行跨目录基础验证。

### 安全要求

- 检查是否引入真实密钥。
- 检查前端是否新增 AI 直连或任务轮询。
- 检查 Compose 是否暴露不必要端口。

### 验收标准

- 第一批变更集成后工作区结构清晰。
- 前端仍可测试和构建。
- 后端骨架可测试。
- Compose 配置可解析。
- 公共合同如有变更，由主 agent 更新并记录。

### 测试命令

```bash
cd frontend && npm run lint && npm run type-check && npm run test && npm run build
cd ../backend && go test ./... && go vet ./...
cd .. && docker compose -f deploy/docker-compose.yml config
git diff --check
```

## 第二批有限并行开发

第二批必须等 R1 完成后启动。建议最多 2 个子 agent 并行，先做后端基础能力和前端 API 基础设施，仍不触碰公共合同文件。

## 子任务 4：后端配置与基础中间件

### 任务名称

P2-BE-INFRA - 后端配置、日志、错误和中间件基础

### 目标

在后端骨架上补齐 API 服务通用基础设施，为后续认证、租户、RBAC 和业务 API 提供统一入口。

### 允许修改文件

- `backend/**`

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `.env.example`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`

### 前置依赖

- R1 集成完成。
- 后端骨架已存在。

### 具体开发内容

- 实现环境变量解析和校验。
- 实现 request ID、访问日志、panic recovery、安全响应头中间件。
- 实现统一成功和错误响应 helper。
- 实现基础路由注册结构。
- 为后续 auth、tenant、rbac、audit 包预留接口，但不实现业务。

### 安全要求

- 日志默认脱敏 Authorization、Cookie、密码、API Key。
- 错误响应不得暴露堆栈。
- CORS 默认关闭或仅允许显式配置来源。

### 验收标准

- API 基础中间件有单元测试。
- 错误响应结构符合 `docs/api-contract.md`。
- 未新增业务 API。

### 测试命令

```bash
cd backend
go test ./...
go vet ./...
```

## 子任务 5：前端 API 与 SSE 客户端基础

### 任务名称

P2-FE-CLIENT - 前端 API Client 与 SSE Client 基础设施

### 目标

在 `frontend/` 中建立后端 API 和 SSE 客户端基础层，但不替换现有工作台业务流。

### 允许修改文件

- `frontend/src/api/**`
- `frontend/src/types/**`
- `frontend/src/lib/**` 中新增通用 API/SSE helper
- `frontend/src/test/**` 中新增 API/SSE helper 测试

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/src/providers/**` 除非主 agent 后续单独批准
- `frontend/src/hooks/useGeneration.ts`
- `frontend/src/App.tsx`
- 现有工作台 UI 组件

### 前置依赖

- R1 集成完成。
- `frontend/` 目录已完成机械搬迁。

### 具体开发内容

- 新增统一 API request helper，默认携带 credentials。
- 新增统一错误解析类型。
- 新增分页响应、错误响应、当前用户、项目、资产、任务、Provider、模型的前端类型壳。
- 新增 SSE client helper，支持 `Last-Event-ID` 记录、heartbeat 处理和断线重连回调接口。
- 只写基础设施，不接入现有生成 UI。

### 安全要求

- 不读取 Cookie 值。
- 不存储认证 token。
- 不新增 API Key 字段到持久化前端状态。
- SSE helper 不得用轮询替代。

### 验收标准

- API helper 能统一处理成功和错误响应。
- SSE helper 使用 EventSource 或等价 SSE 机制。
- 新增测试覆盖错误解析和 SSE 事件分发逻辑。
- 现有 UI 行为不变。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
```

## 串行阶段 2：第二批 review 和合同校正

### 任务名称

R2 - 第二批 review 与公共合同校正

### 目标

主 agent 串行 review 后端基础设施和前端 client 基础层，确认两边接口形状仍符合公共合同。

### 允许修改文件

- 第二批任务允许修改的文件。
- 公共合同文件，仅限主 agent 修改。

### 禁止修改文件

- 未完成 review 的业务 API。
- 任务队列、Provider 调用、MinIO 上传等后续功能。

### 前置依赖

- P2-BE-INFRA 和 P2-FE-CLIENT 完成。

### 具体开发内容

- Review API 响应结构、错误码、SSE helper 行为。
- 统一命名差异。
- 主 agent 必要时更新 `docs/api-contract.md` 或 `docs/sse-contract.md`。
- 运行前后端基础验证。

### 安全要求

- 确认 P2 新增 API/SSE client 无 token 读取和 API Key 持久化。
- 旧前端 API Key localStorage 路径作为迁移遗留记录到 R2 review 结论，不在 R2 直接删除。
- 确认后端日志脱敏测试有效。

### 验收标准

- 前后端基础设施可继续支持 P3 业务切片。
- 公共合同与实现没有已知冲突。

### 测试命令

```bash
cd frontend && npm run lint && npm run type-check && npm run test && npm run build
cd ../backend && go test ./... && go vet ./...
cd .. && git diff --check
```

## R2 后 review 结论摘要

P1 和 P2 已合并到 `main`。当前 `main` 可作为后续开发基线，但不是平台化完成状态。

已通过：

- 前端 `lint`、`type-check`、`test`、`build`。
- 后端 `go test`、`go test -race`、`go vet`、API/worker build。
- `docker compose -f deploy/docker-compose.yml config`。

阻塞和过渡风险：

- `deploy/docker-compose.yml` 引用 `backend/Dockerfile`，但文件不存在，导致 `docker compose build backend-api` 失败。
- `backend-worker` healthcheck 依赖 readiness 文件，但当前 worker 不会创建该文件。
- `frontend/nginx.conf` 仍包含旧 AI relay 路由，平台部署中必须移除。
- frontend API client 默认 `/api/v1`，但 frontend Nginx 尚未代理 `/api/` 到 `backend-api:8080`。
- 旧前端仍有 AI Provider 直连、localStorage API Key、IndexedDB Blob。它们是迁移基线，只能在 P8 前端后端化阶段移除，不能作为新功能路径。

## 第三批串行开发

第三批只开 1 个子 agent，串行完成运行时修复。不要并行做数据库、认证或业务 API，避免在 Compose 不可构建状态下继续堆业务。

## 子任务 6：运行时部署修复

### 任务名称

P3-RUNTIME - 修复 Docker Compose 运行时基础

### 目标

让当前平台骨架从“Compose config 可解析”推进到“frontend、backend-api、backend-worker 镜像可构建，基础容器可启动并通过 healthcheck”。

### 允许修改文件

- `backend/Dockerfile`
- `backend/cmd/worker/**`
- `frontend/nginx.conf`
- `frontend/vite.config.ts`
- `deploy/docker-compose.yml`
- `.env.example`
- `README.md` 仅允许更新运行命令说明

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/src/providers/**`
- `frontend/src/hooks/useGeneration.ts`
- `frontend/src/hooks/useSettings.ts`
- `frontend/src/db/**`
- 任何认证、RBAC、数据库模型、Provider Adapter、任务队列、SSE 服务端业务实现

### 前置依赖

- R2 review 已完成。
- `main` 已包含 P1 和 P2 合并结果。

### 具体开发内容

- 新增 backend 多阶段 Dockerfile，至少包含 `api` 和 `worker` build targets。
- 确保 Dockerfile 使用非特权运行用户或等价的最小权限运行方式。
- worker 启动后创建 `WORKER_HEALTHCHECK_FILE` 指定的 readiness 文件，优雅退出时清理该文件。
- 移除 frontend Nginx 旧 `/relay2` AI 中转路由。
- 在 frontend Nginx 增加 `/api/` 反向代理到 `backend-api:8080`，并确保 SSE 路由禁用 buffering、保持长连接。
- 在 Vite dev server 增加 `/api` 代理到本地后端 API，保持 API client 默认 `/api/v1` 可用于开发。
- 如需调整 Compose healthcheck 命令或 env 名称，只允许围绕现有服务启动和健康检查修复，不引入业务依赖。

### 安全要求

- 不新增任何 AI Provider 代理或浏览器直连路径。
- 不新增 API Key、JWT secret、数据库密码等真实密钥。
- 前端 Nginx 只能代理后端 API，不得代理 OpenAI、Gemini 或中转站。
- 后端镜像不得把源码外的本地缓存、`.env`、node_modules 或生成产物打进镜像。
- Compose 服务默认只绑定本机端口，不扩大公网暴露面。

### 验收标准

- `backend-api` 和 `backend-worker` 镜像均可构建。
- `frontend` 镜像可构建。
- `docker compose up -d` 后 MySQL、Redis、MinIO、backend-api、backend-worker、frontend 均能进入 healthy 或 running 状态。
- `frontend/nginx.conf` 中不存在 AI relay 路由。
- frontend 容器内 `/api/v1/healthz` 可代理到 backend API。
- 未实现任何业务 API、认证、数据库模型、Provider 调用、任务队列或 SSE 服务端业务。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ../backend
go test ./...
go test -race ./...
go vet ./...

cd ..
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
git diff --check
```

## 后续批次顺序

R3 集成完成后，再按最多 2 个 worktree 并行推进。公共合同文件仍只能由主 agent 修改。

## 子任务 7：数据库与租户基础

### 任务名称

P4-BE-DATABASE - 数据库迁移、GORM 基础与租户仓储

### 目标

建立 MySQL/GORM 数据层、migration runner 和 tenant-aware repository 基础，为认证、项目、资产和任务提供可靠数据源。

### 允许修改文件

- `backend/**`
- `deploy/mysql/init/**` 仅允许增加空库初始化或 migration 说明
- `.env.example` 仅允许补充数据库连接变量

### 禁止修改文件

- `frontend/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- Provider 调用、任务队列、SSE 服务端业务

### 前置依赖

- P3-RUNTIME 合并完成。
- 主 agent 已确认 `docs/database-schema.md` 中基础表合同可执行。

### 具体开发内容

- 增加 MySQL/GORM 连接配置和健康依赖。
- 增加 migration runner。
- 增加基础表模型和迁移：tenants、users、roles、permissions、user_roles、role_permissions、operation_logs。
- 增加 tenant context/repository helper，确保 tenant-scoped 查询必须显式传入 `tenant_id`。
- 增加数据库测试，可使用隔离测试库或可替代的 repository 单元测试策略。

### 安全要求

- 所有业务表必须有 `tenant_id`，系统级字典表例外必须在代码注释中说明。
- 不记录数据库密码。
- 不拼接 SQL 注入风险语句。

### 验收标准

- 迁移可重复执行且幂等。
- tenant-aware repository 测试能证明跨租户数据不可见。
- `go test ./...`、`go test -race ./...`、`go vet ./...` 通过。

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
```

## 子任务 8：认证与 RBAC 基础

### 任务名称

P4-BE-AUTH - 初始化管理员、登录会话、租户上下文与 RBAC

### 目标

实现平台最小可用认证链路和权限基础，包括 init-admin、login、logout、me、password change、HttpOnly Cookie、CSRF、auth middleware、tenant context 和 RBAC guard。

### 允许修改文件

- `backend/**`
- `.env.example` 仅允许补充 auth/cookie/CSRF 变量

### 禁止修改文件

- `frontend/**`
- `deploy/**` 除非仅修正 auth 相关 env 透传
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- Provider、资产、任务、SSE 业务实现

### 前置依赖

- P4-BE-DATABASE 合并完成。

### 具体开发内容

- 实现管理员初始化保护逻辑。
- 实现密码哈希和登录校验。
- 设置 JWT HttpOnly Cookie，生产环境支持 Secure 和 SameSite。
- 实现 logout 清除 Cookie。
- 实现 current user API 返回用户、租户、角色、权限。
- 实现密码修改 API。
- 实现 CSRF 基础保护和测试。
- 实现 auth、tenant、RBAC middleware，并接入 `/api/v1` 业务路由组。
- 记录登录、退出、管理员初始化、密码修改等 operation logs。

### 安全要求

- 不返回 password hash。
- 不允许前端读取 token。
- Cookie 配置必须可按生产环境加固。
- 登录失败不泄露用户是否存在。
- state-changing API 必须考虑 CSRF。

### 验收标准

- Auth API 响应符合 `docs/api-contract.md`。
- Cookie、CSRF、失败路径、权限失败路径有测试。
- 操作日志记录敏感动作且不记录密码/token。

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
```

## 子任务 9：前端认证接入

### 任务名称

P4-FE-AUTH - 前端登录与当前用户基础接入

### 目标

基于后端认证 API 接入登录态，让前端具备登录、退出、当前用户加载、未登录展示和基础错误处理。

### 允许修改文件

- `frontend/src/api/**`
- `frontend/src/types/**`
- `frontend/src/components/**`
- `frontend/src/hooks/**`
- `frontend/src/App.tsx`
- `frontend/src/test/**`

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/src/providers/**`
- 旧生成流程替换

### 前置依赖

- P4-BE-AUTH 合并完成。

### 具体开发内容

- 增加 auth API wrappers。
- 增加登录界面或登录状态入口，遵循现有 UI 风格。
- 加载 `/api/v1/me` 并维护当前用户状态。
- 实现 logout。
- 未登录时不展示需要登录的后端数据入口。
- 不替换现有生成工作台业务流。

### 安全要求

- 不读取 Cookie。
- 不存储 JWT 或 session token。
- 不新增 API Key 持久化。

### 验收标准

- 登录、退出、me 加载、401 处理有测试。
- 现有本地工作台测试继续通过。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
```

## 串行阶段 4：P4 review 和集成

### 任务名称

R4 - P4 数据库、认证、RBAC、前端认证集成 review

### 目标

由主 agent 串行 review 并合并 P4-BE-DATABASE、P4-BE-AUTH、P4-FE-AUTH，确认平台进入 P5 前具备数据库、租户、认证、RBAC 和前端登录态基础。

### 已完成结果

- `P4-BE-DATABASE` 已合并：GORM/MySQL 连接、migration runner、核心 auth/RBAC/operation log 表、tenant-aware repository helper。
- `P4-BE-AUTH` 已合并：init-admin、login、logout、`/me`、password change、HttpOnly Cookie JWT、CSRF、auth middleware、tenant context、RBAC guard、operation logs。
- `P4-FE-AUTH` 已合并：frontend auth API wrappers、login UI、current user loading、logout、401 handling、in-memory CSRF token handling。

### P4 review 结论

- 允许进入 P5。
- P4 合并后前端 lint/type-check/test/build 通过。
- P4 合并后后端 test/race/vet/build 通过。
- Compose config 通过。

### P4 非阻塞遗留

- 生产环境需要拒绝默认 `JWT_SIGNING_SECRET` 等占位密钥。
- 前端 CSRF Header 当前使用默认 `X-CSRF-Token`，如未来允许非默认 `CSRF_HEADER_NAME`，需要增加前端配置来源。
- 审计 metadata 脱敏应在 Provider、资产、任务模块写入嵌套 metadata 前改为递归脱敏。
- 旧前端 Provider 直连、localStorage Provider API Key、IndexedDB Blob 历史仍是 P8 迁移遗留，不得在 P5 扩大使用。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ../backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
```

## P5 执行策略

P5 原始“项目与资产管理”范围较大，必须拆成串行 worktree。第一批不要并行做 Provider/model、任务队列、SSE 或前端工作台后端化。公共合同文件仍只能由主 agent 修改。

当前状态：

- `P5-BE-PROJECTS` 已 review 并合并到 `main`。
- `P5-BE-ASSET-STORAGE` 已 review 并合并到 `main`。
- `P5-FE-PROJECT-ASSETS` 已 review 并合并到 `main`。
- `R5` 已完成主 agent review、集成回归和公共合同文档更新。
- 下一步进入 P6 Provider/model 管理，第一项是 `P6-BE-PROVIDER-SECURITY`；仍不要并行启动任务队列、SSE 或前端生成后端化。

推荐顺序：

1. `P5-BE-PROJECTS` - completed.
2. `P5-BE-ASSET-STORAGE` - completed.
3. `P5-FE-PROJECT-ASSETS` - completed.
4. `R5` - completed.

## 子任务 10：项目与项目成员后端基础

### 任务名称

P5-BE-PROJECTS - Project CRUD、项目成员与对象授权基础

### 目标

实现项目和项目成员后端基础，为资产上传、下载和后续任务归属提供可靠对象授权边界。

### 允许修改文件

- `backend/**`

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `.env.example`，除非只补充本任务确实缺失且非敏感的项目配置占位
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- Provider 调用、MinIO 资产上传下载、任务队列、SSE 业务、前端工作台后端化

### 前置依赖

- P4-BE-AUTH 和 P4-FE-AUTH 合并完成。
- 主 agent 已更新 P5 API、数据库、存储、安全合同。

### 具体开发内容

- 增加或补齐 `projects`、`project_members` migration 和 GORM models。
- 实现 project repository/service，所有查询必须显式带 `tenant_id`。
- 实现 project CRUD API：列表、创建、详情、更新、软删除。
- 实现 project member API：列表、新增、更新、移除。
- 创建项目时把创建者写为 `OWNER` 成员。
- 实现项目对象授权 helper：tenant admin 可管理本租户项目；普通用户需具备 RBAC 权限和项目成员角色。
- 写 operation logs：project create/update/delete、member create/update/delete。
- 增加后端测试覆盖跨租户不可见、成员权限、软删除、操作日志。

### 安全要求

- 所有 project 查询必须按 `tenant_id` 过滤。
- 对象 ID 接口必须检查对象级权限，不能只检查登录状态。
- 跨租户 project ID 不得泄露存在性，优先返回 `404` 或非揭示性错误。
- 普通用户必须同时具备 RBAC 权限和项目成员角色。
- 不记录 Cookie、Authorization、密码、JWT、API Key。

### 验收标准

- project CRUD 符合 `docs/api-contract.md`。
- project member role 支持 `OWNER`、`EDITOR`、`VIEWER`。
- 跨租户访问 project 被拒绝或不可见。
- 软删除项目不出现在默认列表和详情中。
- 操作日志记录敏感动作且 metadata 脱敏。

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

### Review 结果

- 允许合并，已合并。
- 已实现 project CRUD、project member 管理、tenant-scoped 查询、对象级授权 helper 和 operation logs。
- 非阻塞遗留：后续可增加“至少保留一个 OWNER”约束，避免普通项目成员失去自助管理入口。

## 子任务 11：资产存储与图片上传后端基础

### 任务名称

P5-BE-ASSET-STORAGE - MinIO storage service、上传校验、资产 API 与授权下载

### 目标

实现后端图片资产基础能力，让参考图可以进入 MinIO 和 MySQL 元数据表，并支持列表、详情、收藏、软删除、授权下载。

### 允许修改文件

- `backend/**`
- `.env.example` 仅允许补充缺失的非敏感存储配置占位

### 禁止修改文件

- `frontend/**`
- `deploy/**`，除非只修正本任务必需且已存在的 MinIO env 透传
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- Provider 调用、任务队列、SSE 业务、前端工作台后端化

### 前置依赖

- `P5-BE-PROJECTS` 合并完成。
- 全局本地 `dev-minio` 可用于集成验证。

### 具体开发内容

- 增加或补齐 `image_assets` migration 和 GORM model。
- 实现 MinIO storage client/service，封装 put/get/delete 或 signed URL 创建能力。
- 使用非猜测 object key：`tenants/{tenantId}/projects/{projectId}/assets/{assetId}/original.{ext}`。
- 实现 asset repository/service，所有查询带 `tenant_id`。
- 实现 `GET /projects/{projectId}/assets`。
- 实现 `POST /projects/{projectId}/assets/uploads`，P5 只创建 `REFERENCE` 资产。
- 实现 `GET /assets/{assetId}`、`PATCH /assets/{assetId}`、`DELETE /assets/{assetId}`。
- 实现 favorite/unfavorite。
- 实现 `GET /assets/{assetId}/download`，默认通过后端鉴权后流式下载。
- 上传前校验真实文件类型、magic bytes、大小、宽高、像素数量，拒绝 SVG。
- 失败路径必须避免留下孤儿对象或孤儿元数据。
- 写 operation logs：asset upload/update/delete/download/favorite。

### 安全要求

- 图片文件必须保存到 MinIO，MySQL 只保存 metadata 和 `object_key`。
- MySQL 禁止保存 Blob。
- 上传校验不能只信任客户端 MIME 或扩展名。
- 文件名只可作为清洗后的展示 metadata，不能直接参与 object key。
- 下载必须经过 backend auth、tenant filter、RBAC 和 project membership/object-level authorization。
- 不记录图片 base64、原始文件字节、Cookie、Authorization、API Key。

### 验收标准

- 跨租户 asset 访问被拒绝或不可见。
- 伪造 MIME、无效 magic bytes、SVG、超限文件、超限尺寸和超限像素上传失败。
- 成功上传后 MinIO 有对象，MySQL 有 metadata 和 object key，无 Blob。
- 下载 API 需要授权并可返回正确 content type。
- 删除为软删除，软删除 asset 不可下载。

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

如果执行真实 MinIO 集成检查，必须使用 `docs/local-development.md` 中的共享 `dev-minio`，不得启动项目专属 MinIO。

### Review 结果

- 允许合并，已合并。
- 已实现 `image_assets`、MinIO object store、上传校验、asset CRUD/favorite/download API、软删除和 operation logs。
- 资产访问通过 `asset -> project` 关系复用项目 RBAC 与 membership 检查。
- 非阻塞遗留：上传后 DB 失败的对象清理应后续改为独立 cleanup context 或清理任务；已有租户的内置 `asset:*` 权限需要后续 reconciliation；MinIO bucket bootstrap 仍由部署或本地环境负责。

## 子任务 12：前端项目与资产接入

### 任务名称

P5-FE-PROJECT-ASSETS - 项目选择、参考图上传、资产列表与下载 UI

### 目标

接入 P5 后端项目和资产 API，让已登录用户可以选择/创建项目、上传参考图、查看资产列表、收藏/删除/下载资产，并在工作台选择项目资产作为参考图。

### 允许修改文件

- `frontend/src/api/**`
- `frontend/src/types/**`
- `frontend/src/components/**`
- `frontend/src/hooks/**`
- `frontend/src/App.tsx`
- `frontend/src/test/**`

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/src/providers/**`
- 旧生成流程替换
- Provider/model 管理、任务队列、SSE 业务

### 前置依赖

- `P5-BE-PROJECTS` 已合并完成。
- `P5-BE-ASSET-STORAGE` 已合并完成。
- `codex/p5-frontend-project-assets` 必须基于包含上述两个后端合并提交的最新 `main`。

### 具体开发内容

- 增加 project 和 asset API wrappers，使用已有 authenticated API client 和 `credentials: include`。
- 按已合并后端接口接入 `/api/v1/projects`、`/api/v1/projects/{projectId}/assets`、`/api/v1/assets/{assetId}`。
- 增加项目列表、创建项目、项目选择状态。
- 增加项目资产列表、详情入口、收藏、软删除和下载动作。
- 增加参考图上传 UI，使用 multipart/form-data 直传后端。
- 上传前可保留客户端 MIME/大小预检作为 UX，但明确以后端校验为准。
- 将“选择项目”和“选择参考资产”接入现有工作台，不替换生成提交路径。
- 旧 IndexedDB 历史仍可展示，但不能冒充后端资产库。
- 处理 401/403/404/422 错误状态和 loading/empty/error UI。
- 不要在 P5-FE 中实现任务创建、SSE、Provider/model 管理或替换旧生成提交路径。

### 安全要求

- 不读取 Cookie。
- 不保存 JWT、session token 或 CSRF token 到 localStorage/sessionStorage/IndexedDB。
- 不新增 Provider API Key 持久化。
- 不新增 AI Provider 直连。
- 不使用轮询查询任务状态。
- 不渲染或记录图片 base64。

### 验收标准

- 已登录用户可以创建/选择项目。
- 已登录用户可以上传参考图并在资产列表看到 metadata。
- favorite/delete/download 操作调用后端 API 且处理错误状态。
- 现有本地工作台核心测试继续通过。
- 未登录状态不会展示项目资产操作。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
git diff --check
```

### Review 结果

- 允许合并，已合并。
- 已接入项目列表/创建/选择、参考资产上传、资产列表、详情动作、收藏、软删除、下载和选择为参考图。
- 前端请求复用 authenticated API client 和 in-memory CSRF；没有新增 JWT、CSRF token、Provider API Key 持久化。
- 没有新增 AI Provider 直连、任务轮询、Provider/model 管理或生成提交后端化。
- 非阻塞遗留：项目创建失败时表单已提前清空，后续应改为成功后清空；前端上传预检仍是 15 MB，后续应从系统设置或后端合同读取限制。

## 串行阶段 5：P5 review 和集成

### 任务名称

R5 - P5 项目与资产管理 review、集成和合同校准

### 目标

主 agent 串行 review P5 子任务，确认项目/资产/MinIO 上传下载链路满足租户隔离、对象级权限和上传安全要求，再进入 P6 Provider/model。

### 允许修改文件

- P5 子任务允许修改的文件。
- 公共合同文件，仅限主 agent 根据实际实现校准。

### 禁止修改文件

- Provider 调用执行路径。
- 任务队列、Worker 真实任务执行、SSE 服务端业务。
- 前端生成工作台后端化替换。

### 前置依赖

- `P5-BE-PROJECTS`、`P5-BE-ASSET-STORAGE`、`P5-FE-PROJECT-ASSETS` 均已提交。

### 具体开发内容

- Review project/asset 代码和测试。
- 串行合并或要求整改。
- 跑前后端回归。
- 使用共享本地 MySQL/Redis/MinIO 做必要集成检查。
- 更新公共合同中的 P5 实际完成状态和遗留风险。

### 安全要求

- 重点检查 tenant filter、object authorization、upload validation、MinIO object key、download auth、sensitive logging。
- 不允许引入 frontend AI Provider 直连或 API Key 持久化新增路径。

### 验收标准

- P5 合并后前端、后端、Compose config 均通过。
- 跨租户 project/asset 访问测试通过。
- 上传安全测试覆盖 forged MIME、invalid magic bytes、SVG、尺寸/像素/大小超限。
- 下载必须鉴权。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ../backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
git diff --check
```

### R5 结果

- P5 三个子任务均已合并到 `main`。
- 主 agent 完成 P5 全量 review，确认项目/资产 API 具备认证、tenant filter、RBAC、项目成员授权、对象级授权、上传校验、软删除和授权下载。
- 前端项目/资产 UI 没有新增 Provider 直连、API Key 持久化、auth token 持久化或任务状态轮询。
- 合并后回归通过：
  - `cd frontend && npm run lint && npm run type-check && npm run test && npm run build`
  - `cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./cmd/api ./cmd/worker`
  - `docker compose -f deploy/docker-compose.yml config`
  - `git diff --check`
- 已确认共享本地 `dev-mysql8`、`dev-redis`、`dev-minio` 运行并可达，没有创建项目专属开发环境。
- 公共合同文档已按 P5 实际结果更新，项目可以进入 P6 Provider/model 管理。

## P6 执行策略

P6 的核心目标是安全落地 Provider/model 管理，为 P7 worker 调用 AI Provider 提供可信配置源。P6 不做真实生成/编辑任务执行，不做 SSE 任务事件，不替换前端生成提交路径。

当前状态：

- `P6-BE-PROVIDER-SECURITY` 已 review 并合并到 `main`。
- 下一步只启动 `P6-BE-MODEL-CAPABILITIES`，从最新 `main` 派生或重置 `codex/p6-backend-model-capabilities`。

执行顺序：

1. `P6-BE-PROVIDER-SECURITY` - completed.
2. `P6-BE-MODEL-CAPABILITIES` - next.
3. `P6-FE-PROVIDER-MODEL-MGMT` - 在后端 Provider/model 合同稳定后做前端管理 UI。
4. `R6` - 主 agent 串行 review、合并、回归和合同校准。

并行策略：

- P6 第一批已只开 1 个子 agent：`P6-BE-PROVIDER-SECURITY`。
- `P6-BE-MODEL-CAPABILITIES` 仍应串行启动，不要让前端 Provider 管理 UI 与模型后端合同并行开发。
- `P6-BE-MODEL-CAPABILITIES` 与 `P6-FE-PROVIDER-MODEL-MGMT` 默认串行；只有主 agent 明确批准时，前端才可以在后端合同冻结后做有限并行。

## 子任务 13：Provider 后端安全底座

### 任务名称

P6-BE-PROVIDER-SECURITY - Provider CRUD、API Key 加密、SSRF 与测试探针

### 目标

实现后端 Provider 管理安全底座，包括 tenant-scoped Provider CRUD、API Key 加密/脱敏、SSRF-safe base URL 校验、启用/禁用、删除、Provider test 和审计日志。

### 允许修改文件

- `backend/internal/provider/**`
- `backend/internal/database/**`
- `backend/internal/api/**`
- `backend/internal/config/**`
- `backend/internal/audit/**`
- `backend/internal/logger/**`
- `backend/cmd/**` 仅限路由/依赖注入所需改动
- `.env.example` 仅限补充非敏感 Provider/API key encryption 配置占位

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- 任务队列、Worker 真实 Provider 生成/编辑执行路径
- SSE 服务端业务
- 前端工作台后端化替换

### 前置依赖

- R5 P5 项目与资产管理集成完成。
- 主 agent 已更新 P6 Provider/API/安全/数据库合同。

### 具体开发内容

- 增加 `ai_providers` model/migration，所有 Provider 记录必须包含 `tenant_id`。
- 实现 Provider repository/service/handler，所有查询显式过滤 `tenant_id`。
- 实现 `GET/POST/GET by id/PATCH/DELETE /providers` 和 enable/disable API。
- 实现 API Key 加密服务，使用 `API_KEY_ENCRYPTION_KEY` 或既有配置源；明文 key 只在请求处理和 backend memory 中短暂存在。
- Provider 响应只返回 `apiKeyHint`、`apiKeyUpdatedAt` 等脱敏元数据，不返回明文或密文。
- 实现 Provider `baseUrl` SSRF validator：保存前校验，Provider test/use 前再次校验。
- 实现 `POST /providers/{providerId}/test` 的 backend-only 探针，带 timeout、SSRF 检查、脱敏响应和 operation log；P6 不生成图片、不创建 task、不写 asset。
- 写 Provider create/update/delete/enable/disable/test operation logs，并确保 metadata 递归脱敏。
- 增加单元/路由测试覆盖 API Key 加密脱敏、tenant 隔离、RBAC、SSRF 阻断、Provider test 脱敏和日志脱敏。

### 安全要求

- API Key 不明文入库。
- API Key 不完整返回前端，也不返回 encrypted/ciphertext 字段。
- 日志和 operation log 不记录 Authorization、Cookie、API Key、密码、JWT、图片 base64 或原始 Provider 响应。
- SSRF 必须阻止 localhost、loopback、private、link-local、multicast、Docker internal hostnames、非 HTTP(S) scheme、URL embedded credentials 和重定向到禁用目标。
- Provider object APIs 必须同时校验登录、RBAC、`tenant_id` 和对象存在性；跨租户 Provider 不得泄露存在性。

### 验收标准

- Provider CRUD、enable/disable、delete、test API 符合 `docs/api-contract.md`。
- MySQL 中不出现明文 API Key。
- Provider 响应不包含明文或密文 API Key。
- SSRF 测试覆盖阻断清单和允许的公开 HTTPS URL。
- Provider test 不创建 task、asset、usage record，也不调用前端 Provider 代码。
- Operation logs 存在且敏感字段脱敏。

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker
git diff --check
```

### Review 结果

- 允许合并，已合并。
- 已实现 `ai_providers` migration/model、tenant-scoped repository/service/routes、Provider CRUD、enable/disable、soft delete、backend-only Provider test、API Key AES-GCM 加密、masked response、SSRF validator、recursive audit metadata redaction 和 operation logs。
- 验证覆盖 API Key 加密/脱敏、tenant 隔离、RBAC、SSRF 阻断、Provider test 脱敏、日志脱敏，以及 Provider test 不创建 asset。
- 非阻塞遗留：P7 真实 Provider Adapter 执行前应增加 SSRF-safe outbound dialer，避免 DNS rebinding；P9 生产启动前应拒绝默认 `API_KEY_ENCRYPTION_KEY` 占位值。
- 合并前验证通过：`cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./cmd/api ./cmd/worker`，`docker compose -f deploy/docker-compose.yml config`，`git diff --check`。

## 子任务 14：模型能力后端管理

### 任务名称

P6-BE-MODEL-CAPABILITIES - 模型 CRUD、能力配置、价格元数据与启用状态

### 目标

实现后端模型能力管理，为后续任务创建、动态参数展示、Provider Adapter 执行和用量估算提供可信模型配置。

### 允许修改文件

- `backend/internal/model/**`
- `backend/internal/provider/**` 仅限读取 Provider 或复用授权/验证 helper
- `backend/internal/database/**`
- `backend/internal/api/**`
- `backend/cmd/**` 仅限路由/依赖注入所需改动

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- Worker 真实 Provider 调用执行路径
- 任务队列和 SSE 服务端业务
- 前端工作台后端化替换

### 前置依赖

- `P6-BE-PROVIDER-SECURITY` 已 review 并合并。

### 具体开发内容

- 增加 `ai_models` model/migration，所有模型记录必须包含 `tenant_id`。
- 实现 model repository/service/handler，所有查询显式过滤 `tenant_id`。
- 实现 `GET/POST/GET by id/PATCH /models` 和 enable/disable API。
- 校验 `providerId` 属于当前 tenant。
- 校验能力字段：generate/edit、多参考图、`n`、最大输出数量、尺寸、质量、输出格式、价格 JSON。
- `supportsN=false` 时 `maxOutputCount` 不能大于 1。
- 实现 enabled model capability list，供 P8 工作台按能力动态渲染参数。
- 写 model create/update/delete/enable/disable operation logs。
- 增加测试覆盖 tenant 隔离、RBAC、Provider 同租户约束、能力字段校验、启用/禁用状态。

### 安全要求

- 模型 API 必须要求 `model:read` 或 `model:manage`。
- 所有模型查询必须带 `tenant_id`。
- 不允许通过跨租户 `providerId` 创建或读取模型。
- 价格和 capability JSON 必须结构化校验，不能保存无界任意对象。
- 不写 Provider API Key、Authorization、Cookie 或图片 base64 到日志。

### 验收标准

- Model CRUD、enable/disable API 符合 `docs/api-contract.md`。
- 模型能力响应足够前端后续动态渲染尺寸、质量、格式、数量和生成/编辑能力。
- 跨租户 Provider/model 访问被拒绝或不可见。
- Operation logs 记录模型管理动作且 metadata 脱敏。

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker
git diff --check
```

## 子任务 15：Provider/model 前端管理

### 任务名称

P6-FE-PROVIDER-MODEL-MGMT - Provider 与模型管理前端 UI

### 目标

实现管理员使用的 Provider/model 管理 UI 和 API wrappers。前端只提交 Provider API Key 到后端，不保存、不回显、不参与 AI 调用。

### 允许修改文件

- `frontend/src/api/**`
- `frontend/src/types/**`
- `frontend/src/components/**`
- `frontend/src/hooks/**`
- `frontend/src/App.tsx`
- `frontend/src/test/**`

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/src/providers/**`
- `frontend/src/hooks/useGeneration.ts`
- `frontend/src/db/**`
- 前端生成工作台后端化替换
- 任务队列、SSE、Provider Adapter 执行

### 前置依赖

- `P6-BE-PROVIDER-SECURITY` 已合并。
- `P6-BE-MODEL-CAPABILITIES` 已合并。

### 具体开发内容

- 增加 Provider/model API wrappers，复用 authenticated API client 和 `credentials: include`。
- 增加 Provider 列表、创建、编辑、删除、启用/禁用和 test UI。
- Provider 表单中的 API Key 字段仅用于提交；编辑页面显示 masked metadata，不回显完整 key。
- 增加模型列表、创建、编辑、启用/禁用 UI。
- 模型表单支持 capability 字段：生成/编辑、多参考图、`n`、最大输出数量、尺寸、质量、输出格式、价格配置。
- 处理 loading/empty/error、401/403/404/422 状态。
- 管理入口只对具备 Provider/model 权限的用户展示；最终鉴权仍以后端为准。

### 安全要求

- 不保存 Provider API Key 到 localStorage、sessionStorage、IndexedDB、URL、React persisted state 或 client-visible config。
- 不新增 AI Provider 直连，不创建 Provider Authorization header。
- 不使用轮询查询 Provider test 或任务状态。
- 不渲染后端未脱敏错误为 HTML。

### 验收标准

- 管理员可以通过后端 API 管理 Provider 和模型。
- Provider API Key 提交后页面只显示 masked metadata。
- 前端测试覆盖 API wrappers、表单提交不持久化 key、错误状态和权限隐藏。
- `rg` 检查没有在新增代码中引入 Provider direct fetch 或 API Key persistence。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
git diff --check
```

## 串行阶段 6：P6 review 和集成

### 任务名称

R6 - P6 Provider/model 管理 review、集成和合同校准

### 目标

主 agent 串行 review P6 子任务，确认 Provider/model 管理满足 API Key 加密、SSRF、防敏感日志、RBAC 和租户隔离要求，再进入 P7 任务队列、Worker 和 SSE。

### 允许修改文件

- P6 子任务允许修改的文件。
- 公共合同文件，仅限主 agent 根据实际实现校准。

### 禁止修改文件

- Worker 真实 Provider 生成/编辑执行路径。
- 任务队列真实执行。
- SSE 任务事件业务。
- 前端生成工作台后端化替换。

### 前置依赖

- `P6-BE-PROVIDER-SECURITY`、`P6-BE-MODEL-CAPABILITIES`、`P6-FE-PROVIDER-MODEL-MGMT` 均已提交。

### 具体开发内容

- Review Provider/model 代码和测试。
- 串行合并或要求整改。
- 跑前后端回归。
- 使用共享本地 MySQL/Redis/MinIO 做必要检查，不创建项目专属环境。
- 更新公共合同中的 P6 实际完成状态和遗留风险。

### 安全要求

- 重点检查 API Key encryption、masking、日志脱敏、SSRF validator、Provider test、tenant filter、RBAC、前端不持久化 key。
- 不允许引入 frontend AI Provider 直连或本地 API Key 存储新增路径。

### 验收标准

- P6 合并后前端、后端、Compose config 均通过。
- SSRF 阻断测试、API Key 加密/脱敏测试、Provider/model tenant 隔离测试通过。
- Provider test 不创建 task、asset、usage record。
- 前端 Provider/model 管理 UI 不保存 API Key。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ../backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
git diff --check
```

## 子任务 17：任务队列、Worker 和 SSE 服务端

### 任务名称

P7-TASK-SSE - Redis 队列、Worker 状态机、Provider Adapter 执行与 SSE

### 目标

实现生成/编辑任务的后端执行链路和实时事件流。

### 允许修改文件

- `backend/**`
- `frontend/src/api/**`
- `frontend/src/lib/taskSseClient.ts`
- `frontend/src/types/**`
- `frontend/src/test/**`

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- 前端工作台主流程替换，除非仅为类型或 client 兼容

### 前置依赖

- R6 P6 Provider/model 管理集成完成。

### 具体开发内容

- 后端实现 task 创建、取消、重试、详情、列表 API。
- 持久化 generation_tasks 和 task_events。
- Redis 入队和 worker claim。
- Worker 实现状态机、幂等、超时、取消、重试和恢复。
- Worker 通过 Provider Adapter 调用 AI Provider，输出上传 MinIO，创建 asset，记录 usage_records 和 api_call_logs。
- 后端 SSE 支持 heartbeat、`Last-Event-ID`、`lastEventId` query fallback、MySQL 历史补发和权限过滤。
- 实现 global、tenant、user、Provider、model 并发限制。

### 安全要求

- Redis 不是最终任务状态源。
- 所有任务查询和事件流必须按 tenant/project 权限过滤。
- Provider 原始错误和请求响应必须脱敏后记录。
- SSE payload 不包含 API Key、Cookie、Authorization 或图片 base64。

### 验收标准

- 任务状态机测试覆盖 queued/running/completed/failed/cancelled/retry/timed_out。
- SSE replay 和不可见事件过滤有测试。
- Worker 重复领取不会重复输出资产或用量记录。

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
```

## 子任务 18：前端工作台后端化

### 任务名称

P8-FE-BACKENDIZATION - 替换前端生成链路并移除本地密钥路径

### 目标

把现有工作台从浏览器直连 AI 迁移为后端 task API + SSE + 项目资产库，并移除本地 API Key 持久化。

### 允许修改文件

- `frontend/**`

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`

### 前置依赖

- P7-TASK-SSE 合并完成。

### 具体开发内容

- `useGeneration` 改为创建后端任务并消费 SSE。
- 结果区由 task events 和 asset 输出驱动。
- 历史区改为项目任务/资产 API，IndexedDB 不再是主数据源。
- 设置弹窗删除或替换本地 API Key 输入。
- 浏览器 Provider adapters 从生产路径移除或隔离为明确的 legacy/import 代码。
- 保留原有上传、提示词、参数、结果、多图、历史和再次编辑 UI 概念。

### 安全要求

- 浏览器不再创建 Provider Authorization header。
- 不再把 Provider API Key 写入 localStorage、sessionStorage、IndexedDB、URL 或 client-visible config。
- 不用轮询任务状态。
- 不记录图片 base64。

### 验收标准

- `rg` 检查生产路径中无前端 AI direct fetch、无 API Key 持久化、无任务轮询。
- 生成任务通过后端 API 创建，状态通过 SSE 更新。
- 项目资产成为生成图和参考图的主数据源。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
```

## 子任务 19：审计、用量、系统设置和发布硬化

### 任务名称

P9-AUDIT-HARDENING - 用量审计、系统设置、安全回归和发布验证

### 目标

补齐平台运维和发布所需能力，完成发布候选版安全与部署验收。

### 允许修改文件

- `backend/**`
- `frontend/**`
- `deploy/**`
- `.env.example`
- `README.md`

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- 未经主 agent 批准的大规模重构

### 前置依赖

- P8-FE-BACKENDIZATION 合并完成。

### 具体开发内容

- 实现 usage summary、usage records、operation logs、api call logs 查询。
- 实现 system settings API 和必要前端界面。
- 增加日志保留、上传限制、默认 Provider/model、并发限制配置。
- 执行安全回归并补齐缺口。
- 完成 Docker Compose 全链路验证和部署说明更新。

### 安全要求

- 审计和 API 调用日志不得包含完整 API Key、Authorization、Cookie、图片 base64 或原始图片字节。
- 系统设置修改必须有 RBAC 和 operation log。
- 发布前必须通过租户隔离、对象权限、SSRF、上传安全、SSE replay 安全测试。

### 验收标准

- 管理端可查看用量、审计、API 调用日志和系统设置。
- 全量测试、Docker Compose build/up/healthcheck 通过。
- 发布文档说明环境变量、初始化管理员、数据卷、备份和安全注意事项。

### 测试命令

```bash
cd backend && go test ./... && go test -race ./... && go vet ./...
cd ../frontend && npm run lint && npm run type-check && npm run test && npm run build
cd .. && docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
```

## 子 agent 交付格式

每个子 agent 最终回复必须包含：

- 修改文件清单。
- 未修改但依赖的公共合同文件清单。
- 执行的测试命令和结果。
- 安全约束自查结果。
- 遇到的合同缺口或需要主 agent 决策的问题。

子 agent 不得自行修订公共合同文件。
