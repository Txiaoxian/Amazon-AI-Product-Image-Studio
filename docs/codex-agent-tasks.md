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

## P8 及后续任务强制执行的任务包标准

从 P8 及后续任务开始，任务包必须是“实现合同”，不能只写功能清单。除原有字段外，每个新 worktree 任务包还必须包含：

1. `必须保持的现有行为`
2. `允许的中间态`
3. `禁止的半迁移状态`
4. `失败模式与边界场景`
5. `必须新增或更新的回归测试`

这些要求来自近期 P7/P8 review 的真实复盘：

- `P7-BE-TASK-FOUNDATION` 首轮实现暴露出 replay cursor 没有被提前冻结成稳定单调合同。
- `P7-BE-WORKER-QUEUE` 首轮实现遗漏 orphaned claim recovery。
- `P7-BE-PROVIDER-ADAPTER-RUNTIME` 首轮实现未覆盖 API Key 出现在 metadata value 和 map key 两种脱敏边界。
- `P8-FE-WORKBENCH-FOUNDATION` 首轮实现把 backend-ready 输入准备过早暴露为默认生产 UI，导致“新 UI / 旧请求脱节”、项目资产参考图静默失效、本地历史再次编辑死路。

后续任务包必须把这些“原本靠 review 才显形的隐含约束”提前写出来。

### 任务包编写规则

- 迁移类任务必须写清楚：
  - 旧生产路径是什么。
  - 本任务结束后允许停在哪个中间态。
  - 最终目标路径由哪个后续任务接管。
  - 在替代路径接管前，哪些旧行为必须继续可用。
  - 哪些半迁移状态绝对禁止出现。
- 前端迁移任务必须包含三列表：

| Old path | Allowed intermediate state | Target path |
| --- | --- | --- |

- Worker、queue、auth、RBAC、Provider、security、state-machine 等高风险后端任务必须附 `失败模式 / 状态转移矩阵`，至少覆盖任务范围内的 happy path、duplicate delivery、cancel、timeout、retry、recovery、third-party failure、cross-tenant/unauthorized、sensitive-data redaction 等 relevant 分支。
- 如果某个高风险分支明确不在本任务范围内，必须在任务包中显式写出“延后到哪个任务”，不能留成默认空白。
- 子 agent 最终回复必须把“必须新增或更新的回归测试”逐条映射到真实测试文件和测试名。
- 如果实现过程中发现“保持旧行为”与“任务目标”发生冲突，而任务包没有授权破坏旧行为，子 agent 必须停止并上报，不得自行选择一个会破坏生产路径的中间态。

### 主 agent review 规则

主 agent 对后续 P8/P9 任务 review 时，必须显式检查：

- 任务包要求保留的旧行为是否仍在。
- 实际落地的中间态是否等于任务包允许的中间态。
- 是否出现任务包禁止的半迁移状态。
- 失败模式矩阵中的每一项是否有测试或明确的延期说明。
- 子 agent 的最终回复是否把回归场景映射到了真实测试。

如果 review 连续暴露同一类遗漏，主 agent 需要优先更新后续任务包和本节规则，而不是只在单次 review 中重复口头提醒。

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
- `P6-BE-MODEL-CAPABILITIES` 已 review 并合并到 `main`。
- `P6-FE-PROVIDER-MODEL-MGMT` 已 review、修复阻塞问题并合并到 `main`。
- `R6` 已完成。项目下一步进入 P7。

执行顺序：

1. `P6-BE-PROVIDER-SECURITY` - completed.
2. `P6-BE-MODEL-CAPABILITIES` - completed.
3. `P6-FE-PROVIDER-MODEL-MGMT` - completed.
4. `R6` - completed.

并行策略：

- P6 第一批已只开 1 个子 agent：`P6-BE-PROVIDER-SECURITY`。
- `P6-BE-MODEL-CAPABILITIES` 已串行完成并合并，不再与前端 Provider/model UI 并行。
- `P6-FE-PROVIDER-MODEL-MGMT` 已串行完成并合并。P7 仍必须按新的 P7 执行策略推进，不要把任务队列、Worker、Provider Adapter runtime 和 SSE 合成一个大任务。

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

### Review 结果

- 允许合并，已合并。
- 已实现 `ai_models` migration/model、tenant-scoped repository/service/routes、model CRUD、enable/disable、soft delete、capability validation、pricing metadata validation、Provider same-tenant checks、RBAC 和 operation logs。
- 验证覆盖 tenant 隔离、RBAC、Provider 同租户约束、能力字段校验、启用/禁用状态、日志脱敏和模型响应不暴露 Provider 凭据。
- 非阻塞遗留：R7 已确认当前任务执行按 `modelId` 工作，`model_name` 非唯一不阻塞现有 runtime；若后续需要更强管理约束，仍需决定同一 Provider 下是否强制唯一。Provider soft delete 后关联模型是阻止删除、隐藏还是级联禁用，仍需在 P8/P9 前确定。
- 合并前验证通过：`cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./cmd/api ./cmd/worker`，`git diff --check`。

## 子任务 15：Provider/model 前端管理

### 任务名称

P6-FE-PROVIDER-MODEL-MGMT - Provider 与模型管理前端 UI

### 推荐执行信息

- 推荐线程名：`P6-FE-PROVIDER-MODEL-MGMT`
- 推荐分支名：`codex/p6-frontend-provider-model-mgmt`
- 起始分支：最新 `main`
- 开发顺序：串行执行。该任务完成、review、合并和回归后，再进入 `R6`；不要并行启动 P7。

### 子 agent 完整启动 prompt

```text
你是 P6-FE-PROVIDER-MODEL-MGMT 子 agent。

当前任务分支必须是：codex/p6-frontend-provider-model-mgmt。
开始开发前必须执行：
1. git status --short --branch
2. git branch --show-current
如果当前不在 codex/p6-frontend-provider-model-mgmt，必须先签入该分支后再继续；不要在 main 或其他分支开发。

任务目标：
实现管理员使用的 Provider/model 管理前端 UI 和 API wrappers。前端只把 Provider API Key 作为一次性表单字段提交给后端，不保存、不回显、不参与 AI 调用。P6 只做管理 UI，不替换旧生成工作台提交路径，不实现任务队列、SSE 或真实 Provider Adapter 执行。

必须先阅读：
- AGENTS.md
- agent-instructions/01-project-overview.md
- agent-instructions/02-architecture-rules.md
- agent-instructions/03-frontend-rules.md
- agent-instructions/05-security-rules.md
- agent-instructions/06-testing-and-delivery.md
- docs/api-contract.md
- docs/rbac.md
- docs/provider-adapter.md
- docs/security.md
- docs/development-plan.md
- docs/codex-agent-tasks.md

允许修改文件：
- frontend/src/api/**
- frontend/src/types/**
- frontend/src/components/**
- frontend/src/hooks/**
- frontend/src/App.tsx
- frontend/src/test/**

禁止修改文件：
- backend/**
- deploy/**
- docs/**
- AGENTS.md
- agent-instructions/**
- frontend/src/providers/**
- frontend/src/hooks/useGeneration.ts
- frontend/src/db/**
- 任务队列、SSE、Provider Adapter 执行相关代码
- 前端生成工作台后端化替换

具体开发内容：
1. 新增 Provider API wrapper，复用现有 authenticated API client、统一 envelope、CSRF header 和 credentials include。
2. 新增 Model API wrapper，复用现有 authenticated API client、统一 envelope、CSRF header 和 credentials include。
3. 增加前端 Provider/model 类型，字段必须匹配 docs/api-contract.md 中 P6 Provider/Model 合同。
4. 增加管理员 Provider 管理 UI：列表、创建、编辑、删除、启用、禁用、test。
5. Provider 表单中的 API Key 只能作为用户本次输入提交；编辑页只能显示 apiKeyHint/apiKeyUpdatedAt 等 masked metadata，不得回显完整 key。
6. 增加管理员模型管理 UI：列表、创建、编辑、删除、启用、禁用。
7. 模型表单支持 capability 字段：supportsGenerate、supportsEdit、supportsMultiReference、supportsN、maxOutputCount、supportedSizes、supportedQualities、supportedOutputFormats、pricing、status。
8. 处理 loading、empty、error、401、403、404、422 状态。
9. 管理入口只对具备 provider/model 管理权限的用户展示；最终鉴权仍以后端为准。
10. 保持现有 React + TypeScript + Vite + Tailwind 风格，不重写主应用，不大规模重构旧工作台。

安全要求：
- 不保存 Provider API Key 到 localStorage、sessionStorage、IndexedDB、URL、React persisted state 或 client-visible config。
- 不新增 OpenAI、Gemini 或 OpenAI-Compatible Provider 直连。
- 不创建浏览器侧 Provider Authorization header。
- 不使用轮询查询 Provider test、任务状态或任何后端状态。
- 不修改旧生成提交路径；旧本地 Provider/API Key 路径仍是 P8 迁移遗留，不得在本任务扩大使用。
- 不渲染后端错误为 HTML。

验收标准：
- 管理员可以通过后端 API 管理 Provider 和模型。
- Provider API Key 提交后页面只显示 masked metadata。
- 前端测试覆盖 API wrappers、表单提交不持久化 key、错误状态和权限隐藏。
- 新增代码没有引入 Provider direct fetch、Provider Authorization header、API Key persistence 或任务轮询。
- frontend lint、type-check、test、build 全部通过。

测试命令：
cd frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ..
git diff -- frontend/src/api frontend/src/components frontend/src/hooks frontend/src/types frontend/src/test | rg -n "^\\+.*(localStorage|sessionStorage|indexedDB|Authorization|Bearer|setInterval|setTimeout|openai|gemini|relay2)" || true
git diff --check

交付要求：
- 提交到 codex/p6-frontend-provider-model-mgmt。
- 最终说明改动范围、验证命令结果、未改动项和任何遗留风险。
```

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

### Review 结果

- 允许合并，已合并。
- 已实现 Provider/model API wrappers、管理员 Provider/model 管理 UI、Provider test UI、启用/禁用/删除/编辑表单、权限隐藏、错误状态和 masked key metadata 展示。
- 初次 review 发现 Provider API Key draft 在关闭弹窗后仍可能残留；子 agent 已在 `fix: clear Provider key draft on admin modal close` 中修复，并新增回归测试。
- 验证确认 Provider API Key 只作为一次性请求字段提交；关闭弹窗后清空未提交 key；没有写入 localStorage、sessionStorage、IndexedDB、URL 或 client-visible config。
- 没有新增浏览器 Provider 直连、Provider Authorization header、任务轮询、旧生成路径替换或前端 Provider Adapter 修改。

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

### R6 结果

- P6 三个开发任务均已合并到 `main`。
- 主 agent 完成 P6 review、前端阻塞问题复审、合并和 R6 回归。
- 前端验证通过：`cd frontend && npm run lint && npm run type-check && npm run test && npm run build`，16 个 test files / 58 个 tests 全部通过。
- 后端验证通过：`cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./cmd/api ./cmd/worker`。
- Compose 验证通过：`docker compose -f deploy/docker-compose.yml config`。
- P6 frontend diff security scan 只命中 Provider 类型枚举和测试文本中的 `OPENAI`、`GEMINI`、`OPENAI_COMPATIBLE`；未发现新增 Provider direct fetch、Authorization header、API Key persistence 或任务轮询路径。
- 公共合同已更新 P6 实际完成状态、R6 验证结果和 P7 遗留风险。

## P7 执行策略

P7 的目标是落地任务创建、Redis 队列、Worker 状态机、Provider Adapter 运行时、MinIO 输出资产、用量/API 调用日志和 SSE 实时事件流。P7 不替换前端主工作台生成链路；P8 才负责把旧浏览器直连生成迁移到 task API + SSE。

执行顺序：

1. `P7-BE-TASK-FOUNDATION` - completed and merged. It freezes task schema, status names, event writer, task API, Redis enqueue abstraction, and `task_events.sequence` replay cursor.
2. `P7-BE-SSE-STREAM` - completed and merged. It depends on the merged task event schema and replays by `task_events.sequence`.
3. `P7-BE-WORKER-QUEUE` - completed and merged. It added reliable Redis queue consumption, Worker state handling, Redis wakeups, concurrency limits, and fake/stub execution.
4. `P7-BE-PROVIDER-ADAPTER-RUNTIME` - completed and merged. It added real Provider execution, SSRF-safe outbound transport, MinIO outputs, usage, API call logs, and redacted Provider errors.
5. `P7-FE-TASK-CLIENT-SSE` - completed and merged. It added API/SSE client types plus reducer utilities without replacing the main workbench.
6. `R7` - completed. 主 agent 串行 review、集成回归、安全审查和公共合同校准已完成。

并行策略：

- 第一项 `P7-BE-TASK-FOUNDATION` 已串行完成。
- `P7-BE-WORKER-QUEUE` 已串行完成并合并。
- `P7-BE-PROVIDER-ADAPTER-RUNTIME` 已串行完成、修复安全问题并合并。
- `P7-FE-TASK-CLIENT-SSE` 已串行完成并合并，只消费稳定合同，没有提前替换 P8 工作台。

P7 统一状态约定：

- 任务状态：`QUEUED`、`RUNNING`、`SUCCEEDED`、`FAILED`、`CANCELLED`、`RETRYING`、`TIMED_OUT`。
- SSE `TASK_COMPLETED` event 表示 task status 进入 `SUCCEEDED`。
- 前端已有 transitional `COMPLETED` 类型不得作为新后端合同继续扩散；P7/P8 使用 `SUCCEEDED`。

P7 foundation actual result:

- `P7-BE-TASK-FOUNDATION` merged into `main` after review and fix.
- Task create/list/detail/cancel/retry APIs are implemented under `/api/v1`.
- `generation_tasks` and `task_events` are the MySQL source of truth.
- `task_events.sequence` is the stable monotonic replay cursor; `task_events.id` is derived from sequence and emitted as SSE `id`.
- Redis enqueue payload contains task ID only. Enqueue failure transitions the task to `FAILED` with sanitized `ENQUEUE_FAILED` metadata.
- Worker execution, SSE long connection, real Provider calls, and output asset creation are now implemented by later merged P7 tasks.

P7 SSE stream actual result:

- `P7-BE-SSE-STREAM` merged into `main` after review.
- Backend SSE endpoint is available at `GET /api/v1/events/tasks`.
- Replay uses MySQL `task_events.sequence` and emits sequence-derived `task_events.id`.
- `Last-Event-ID`, `lastEventId`, heartbeat, visible project/task filtering, cross-tenant isolation, and disconnect cleanup are covered by tests.
- Live fanout uses the API-process broker plus Redis wakeups from Worker/API processes. MySQL remains the source of truth for replay and authorization.

P7 Worker queue actual result:

- `P7-BE-WORKER-QUEUE` merged into `main` after review and fix.
- Redis reliable queue supports enqueue, delayed promotion, claim, ack, stale claim recovery, max-delivery dead-letter handling, and malformed claim recovery tests.
- Worker consumes task ID payloads only, reloads task state from MySQL, writes `TASK_STARTED`, `TASK_PROGRESS`, terminal events, and uses fake/stub execution until Provider Adapter runtime is implemented.
- Worker-written events publish minimal Redis wakeups so API SSE streams can replay persisted MySQL events without Redis becoming the event source of truth.
- Global, tenant, user, Provider, and model concurrency limits are implemented with stale lock cleanup.
- Non-blocking carry-forward risks after P10 worker-pool merge: API Redis event subscription uses a background context that should later be tied to server lifecycle.
- Real Provider calls, MinIO output assets, `task_outputs`, `usage_records`, and `api_call_logs` are implemented by `P7-BE-PROVIDER-ADAPTER-RUNTIME`.

P7 Provider Adapter runtime actual result:

- `P7-BE-PROVIDER-ADAPTER-RUNTIME` merged into `main` after review and follow-up security fixes.
- Backend Provider Adapter runtime now executes OpenAI, Gemini, and OpenAI-compatible image generation/edit requests through normalized request/result types.
- Connect-time SSRF-safe outbound transport validates the final dial target; save/use-time URL validation is still enforced.
- Worker runtime persists generated/edited MinIO objects, image assets, `task_outputs`, `usage_records`, `api_call_logs`, and output/usage/terminal task events.
- Runtime logs and metadata use recursive redaction with the decrypted Provider API key as a known secret. Review fixes covered API-key leakage when the secret appeared as a value and when it appeared as a nested JSON map key.
- Residual risk: secrets unknown to the redactor and not matched by heuristics cannot be recognized automatically. The active Provider runtime path passes the decrypted Provider API key into the redactor, so current configured Provider keys are covered.

P7 frontend task client actual result:

- `P7-FE-TASK-CLIENT-SSE` merged into `main` after review.
- Frontend task API wrappers now cover create/list/detail/cancel/retry using the existing authenticated client and CSRF flow.
- Frontend SSE types and reducer utilities use canonical `SUCCEEDED`, typed task event payloads, EventSource, heartbeat handling, and `lastEventId` fallback.
- The task did not replace the main workbench, add frontend Provider direct calls, persist Provider keys, or introduce polling.

## 子任务 17：P7 后端任务基础

### 任务名称

P7-BE-TASK-FOUNDATION - 任务 schema、状态机基础、事件写入和任务 API

### 目标

建立任务系统基础合同：MySQL task/event/output/log/usage schema、任务 API、事件写入、Redis enqueue 抽象和状态机基础。此任务不执行真实 Provider 调用，不实现 SSE 长连接，不替换前端工作台。

### 状态

Completed and merged into `main`. Review required one fix for deterministic SSE replay cursors; the final implementation uses `task_events.sequence` and sequence-derived `task_events.id`.

### 允许修改文件

- `backend/internal/task/**`
- `backend/internal/database/**`
- `backend/internal/api/**`
- `backend/internal/config/**` 仅限任务/队列配置
- `backend/internal/queue/**` 或等价 Redis enqueue 抽象
- `backend/cmd/api/**` 仅限路由/依赖注入
- `backend/cmd/worker/**` 仅限空依赖注入或编译兼容
- `backend/internal/audit/**` 仅限复用 operation log helper
- `backend/internal/rbac/**` 仅限复用权限 helper

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- 真实 Provider Adapter 执行
- SSE 长连接实现
- Worker claim/execution loop
- 前端工作台后端化替换

### 前置依赖

- R6 P6 Provider/model 管理集成完成。

### 具体开发内容

- 增加或补齐 `generation_tasks`、`task_events`、`task_outputs`、`api_call_logs`、`usage_records` migration 和 GORM models。
- 实现 task repository/service，所有查询显式过滤 `tenant_id`。
- 实现 `POST /projects/{projectId}/tasks`、`GET /projects/{projectId}/tasks`、`GET /tasks/{taskId}`、`POST /tasks/{taskId}/cancel`、`POST /tasks/{taskId}/retry`。
- 创建任务时校验项目权限、Provider/model 同租户、Provider/model enabled、模型能力和任务参数。
- MySQL 持久化成功后再 enqueue Redis；任务创建在 MySQL 与 enqueue 之间必须有可恢复策略。
- 实现 task event writer：先写 MySQL task_events，再预留 live fanout hook。
- 实现取消/重试的持久状态变更和事件写入，但 Worker 实际执行留给后续任务。
- 写 operation logs：task create/cancel/retry。
- 增加测试覆盖 tenant 隔离、项目权限、Provider/model 约束、状态流转、event 持久化、enqueue 失败处理和日志脱敏。

### 安全要求

- Redis 不是最终任务状态源。
- 所有任务 API 必须鉴权、RBAC、tenant filter 和项目成员/管理员对象级校验。
- 请求参数不得允许前端传入 `tenantId`、`createdBy`、`status` 等服务端字段。
- Task/event payload 不得包含 API Key、Authorization、Cookie、图片 base64 或原始 Provider 响应。
- 创建任务不能调用 AI Provider，不能上传输出资产。

### 验收标准

- 任务 API 符合 `docs/api-contract.md`。
- `generation_tasks` 和 `task_events` 以 MySQL 为最终状态源。
- 创建任务写入 `TASK_QUEUED` event 并 enqueue Redis。
- 跨租户、无项目权限、disabled Provider/model、能力不匹配的请求被拒绝。
- 不存在真实 Provider 调用、输出 asset 创建或 SSE 长连接业务。

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker
git diff --check
```

## 子任务 18：P7 SSE 服务端事件流

### 任务名称

P7-BE-SSE-STREAM - 任务事件 SSE、heartbeat、Last-Event-ID 和历史补发

### 目标

实现后端任务事件 SSE 流，支持 heartbeat、断线重连、`Last-Event-ID`、`lastEventId` query fallback、MySQL 历史补发和 tenant/project/task 权限过滤。

### 状态

Completed and merged into `main`. The implementation uses MySQL replay as source of truth and an in-process broker as an API-process live wakeup. Cross-process Worker-to-SSE wakeup remains a required `P7-BE-WORKER-QUEUE` item.

### 允许修改文件

- `backend/internal/sse/**`
- `backend/internal/task/**` 仅限事件读取/fanout 接口
- `backend/internal/api/**`
- `backend/internal/rbac/**` 仅限复用权限 helper
- `backend/cmd/api/**` 仅限依赖注入

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- Worker 执行 loop
- 真实 Provider Adapter 调用

### 前置依赖

- `P7-BE-TASK-FOUNDATION` 已合并。

### 具体开发内容

- 实现 `GET /api/v1/events/tasks`。
- 支持 `projectId`、`taskId`、`lastEventId` query，以及 `Last-Event-ID` header。
- 从 MySQL `task_events` replay 可见历史事件，再进入 live stream。
- 解析 `Last-Event-ID` / `lastEventId` 的 `evt_000...` 值为 `sequence`，历史补发查询必须使用 `sequence > cursor` 并按 `sequence ASC` 排序。
- 实现 heartbeat event，避免连接静默断开。
- 实现 live fanout，可使用 Redis pub/sub 或进程内 broker；MySQL 仍是 replay source。
- 事件 payload 使用 camelCase，禁止敏感字段。
- 增加测试覆盖 replay、heartbeat frame、Last-Event-ID、query fallback、不可见 task/project 过滤、跨租户隔离和断开清理。

### 安全要求

- SSE endpoint 必须使用 Cookie auth。
- 每个事件在发送前必须做 tenant/project/task 可见性校验。
- 不得向无权限用户泄露事件存在性。
- Payload 不得包含 API Key、Authorization、Cookie、图片 base64 或原始 Provider 响应。

### 验收标准

- SSE 合同符合 `docs/sse-contract.md`。
- `Last-Event-ID` 和 `lastEventId` 均能补发历史事件。
- heartbeat 可被测试验证。
- 不可见事件不会发送给当前用户。

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker
git diff --check
```

## 子任务 19：P7 Worker 队列和状态机

### 任务名称

P7-BE-WORKER-QUEUE - Redis reliable queue、Worker claim、状态机、幂等和并发限制

### 状态

Completed and merged into `main`. Review required one fix for orphaned Worker queue claims; the final implementation recovers stale processing entries, routes exceeded deliveries to dead-letter, publishes Redis wakeups after persisted task events, and keeps fake/stub execution until Provider Adapter runtime.

### 目标

实现 Worker 对 Redis 队列的可靠消费和任务状态机执行骨架，使用 fake/stub Provider execution 验证 claim、幂等、取消、重试、超时、恢复和并发限制。真实 Provider 调用留给 `P7-BE-PROVIDER-ADAPTER-RUNTIME`。

### 允许修改文件

- `backend/cmd/worker/**`
- `backend/internal/task/**`
- `backend/internal/queue/**`
- `backend/internal/config/**`
- `backend/internal/database/**` 仅限必要索引或状态字段补齐
- `backend/internal/api/**` 仅限状态兼容测试

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- 真实 Provider Adapter HTTP 调用
- 前端工作台后端化替换

### 前置依赖

- `P7-BE-TASK-FOUNDATION` 已合并。
- `P7-BE-SSE-STREAM` 已合并；Worker must integrate with the established task event writer and live wakeup contract.

### 具体开发内容

- 实现 Redis reliable queue claim/ack/retry/dead-letter 或等价可靠队列模式。
- Worker 从队列加载 task ID，再从 MySQL 加载完整任务状态。
- 实现事务性 `QUEUED/RETRYING -> RUNNING` claim 和 `TASK_STARTED` event。
- 实现 fake/stub execution 完成路径，写 `TASK_PROGRESS`、`TASK_COMPLETED` 或失败事件。
- Worker 写入 task event 后必须发布 Redis pub/sub 或等价跨进程 wakeup，使 API 进程内 SSE stream 能及时 replay MySQL 新事件；Redis 不能成为事件状态源。
- 实现取消、重试、超时和 recovery loop。
- 实现 global、tenant、user、Provider、model concurrency limiter，确保 crash recovery 能释放 stale locks。
- 保证重复领取不会重复创建 task output、usage 或 terminal events。

### 安全要求

- Worker 不信任 Redis payload 中除 task ID 以外的信息。
- Worker 每次执行都从 MySQL 重读 task、tenant、Provider、model 和 project 状态。
- Worker event fanout 只能传递事件 ID/sequence 或最小唤醒信号；不得传递 API Key、Authorization、Cookie、图片 base64 或原始 Provider 响应。
- 日志不得包含 prompt 以外敏感数据；不得记录 API Key、Authorization、Cookie、图片 base64。

### 验收标准

- 状态机测试覆盖 queued/running/succeeded/failed/cancelled/retrying/timed_out。
- 重复领取和 worker crash/recovery 测试不产生重复输出或重复 terminal event。
- 并发限制覆盖 global、tenant、user、Provider、model。
- Worker 写入事件后，SSE stream 能通过跨进程 wakeup 从 MySQL replay 出 `TASK_STARTED`、`TASK_PROGRESS`、`TASK_COMPLETED` 或失败事件。
- Worker 可独立 build 和 test。

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker
git diff --check
```

## 子任务 20：P7 Provider Adapter 运行时

### 任务名称

P7-BE-PROVIDER-ADAPTER-RUNTIME - 真实 Provider 调用、SSRF-safe transport、MinIO 输出、用量和 API 调用日志

### 目标

把 Worker fake execution 替换为后端 Provider Adapter 真实执行路径，支持 OpenAI、Gemini 和 OpenAI-compatible Provider，输出图片进入 MinIO 和资产库，并记录 api_call_logs 与 usage_records。

### 状态

Completed and merged into `main`. Review required follow-up fixes so Provider runtime redaction covers both current API key values and current API keys used as nested JSON map keys before logs are persisted.

### 允许修改文件

- `backend/internal/provider/**`
- `backend/internal/provideradapter/**` 或等价 adapter 包
- `backend/internal/task/**`
- `backend/internal/asset/**`
- `backend/internal/storage/**`
- `backend/internal/database/**`
- `backend/internal/config/**`
- `backend/cmd/worker/**`

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- 浏览器 Provider adapter 生产路径

### 前置依赖

- `P7-BE-WORKER-QUEUE` 已合并。
- P7 SSRF-safe outbound transport 方案已由主 agent 或文档确认：real Provider runtime must validate Provider URL at save/use time and also validate the final dial target at connection time to defend against DNS rebinding.

### 具体开发内容

- 定义 backend Provider Adapter interface 和 normalized request/result types。
- 实现 OpenAI、Gemini、OpenAI-compatible adapters 或最小可测试执行路径。
- 实现 SSRF-safe HTTP transport / `DialContext`，连接时校验最终 IP，防 DNS rebinding。
- Worker 解密 API key 只在 backend memory 中使用，不写日志或响应。
- 按 model capability 校验尺寸、质量、输出格式、n、多参考图和编辑能力。
- 输出图片上传 MinIO，创建 GENERATED/EDITED asset，写 task_outputs。
- 记录 api_call_logs、usage_records 和 task events：`IMAGE_OUTPUT`、`USAGE_RECORDED`、`TASK_COMPLETED`/`TASK_FAILED`。
- Provider 错误、request/response metadata 必须递归脱敏。

### 安全要求

- 不允许业务代码绕过 Provider Adapter 直接拼 Provider HTTP 调用。
- 出站 HTTP 必须做 connect-time SSRF 防护。
- API Key 不落库明文、不返回前端、不写日志。
- API call logs 不保存 Authorization、Cookie、完整 API Key、图片 base64 或原始图片字节。

### 验收标准

- Adapter 单元测试覆盖 OpenAI/Gemini/OpenAI-compatible request mapping、错误脱敏和 usage normalization。
- SSRF-safe transport 测试覆盖 DNS rebinding/blocked IP/redirect blocked。
- Worker 成功路径创建 MinIO object、asset、task_output、usage_record、api_call_log 和 task events。
- Worker 失败路径写 sanitized error，不创建半成品资产。

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker
git diff --check
```

## 子任务 21：P7 前端 Task API 和 SSE Client 合同

### 任务名称

P7-FE-TASK-CLIENT-SSE - Task API wrappers、SSE client 类型和事件 reducer

### 目标

为 P8 工作台后端化准备前端 task API wrappers、SSE client 类型和事件 reducer。P7 前端只做合同层和测试，不替换现有生成工作台主流程。

### 状态

Completed and merged into `main`. Review confirmed it stayed at the contract layer and did not start P8 workbench backendization early.

### 允许修改文件

- `frontend/src/api/**`
- `frontend/src/types/**`
- `frontend/src/lib/taskSseClient.ts`
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
- `frontend/src/components/studio/**` 主流程替换

### 前置依赖

- `P7-BE-TASK-FOUNDATION` 已合并。
- `P7-BE-SSE-STREAM` 合同稳定。
- `P7-BE-PROVIDER-ADAPTER-RUNTIME` 已合并，task 输出、usage 和 API call log 行为已稳定到可供前端合同层消费。

### 具体开发内容

- 新增 task API wrappers：create/list/detail/cancel/retry，复用 authenticated API client 和 CSRF。
- 更新 task/status/event TypeScript 类型，使用 `SUCCEEDED` 而不是 transitional `COMPLETED`。
- 扩展 `taskSseClient` 以匹配后端 SSE event frame、heartbeat、Last-Event-ID query fallback 和 typed payloads。
- 增加 task event reducer utility，能把 queued/started/progress/output/usage/failed/completed/cancelled/retried/timed_out event 合成为 UI 可消费状态。
- 增加测试覆盖 wrappers、credentials/CSRF、SSE URL、heartbeat 忽略或处理、replay id、event reducer。

### 安全要求

- 不使用轮询、`setInterval` 或 repeated fetch 查询任务状态。
- 不新增浏览器 Provider direct fetch 或 Provider Authorization header。
- 不保存 API Key 到 localStorage、sessionStorage、IndexedDB、URL 或 client-visible config。

### 验收标准

- 前端 task/SSE 合同与 `docs/api-contract.md`、`docs/sse-contract.md` 一致。
- 任务状态不再扩散 `COMPLETED` 作为后端新合同。
- 没有替换 P8 工作台主流程。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
git diff --check
```

## 串行阶段 7：P7 review 和集成

### 任务名称

R7 - P7 task/Worker/Provider/SSE review、集成和合同校准

### 目标

主 agent 串行 review P7 子任务，确认任务状态、Redis 队列、Worker 幂等、Provider Adapter 安全、MinIO 输出、用量/API 日志和 SSE replay 均满足平台规则，再进入 P8 前端工作台后端化。

### 允许修改文件

- P7 子任务允许修改的文件。
- 公共合同文件，仅限主 agent 根据实际实现校准。

### 禁止修改文件

- P8 前端工作台主流程替换。
- 未经 review 的部署环境大改。

### 前置依赖

- `P7-BE-TASK-FOUNDATION`、`P7-BE-SSE-STREAM`、`P7-BE-WORKER-QUEUE`、`P7-BE-PROVIDER-ADAPTER-RUNTIME`、`P7-FE-TASK-CLIENT-SSE` 均已合并。

### 具体开发内容

- Review P7 全部代码和测试。
- 串行合并或要求整改。
- 使用共享本地 MySQL/Redis/MinIO 做必要集成检查，不创建项目专属开发环境。
- 跑前后端、Compose config 和必要 Worker/SSE 集成测试。
- 更新公共合同中的 P7 实际完成状态和遗留风险。

### 安全要求

- 重点检查 SSE replay 可见性、tenant/project/task authorization、Provider Adapter SSRF-safe transport、API Key 解密范围、日志脱敏、Worker 幂等和并发限制。
- 不允许引入 frontend AI Provider direct call、API Key persistence 或任务状态轮询。

### 验收标准

- P7 合并后前端、后端、Compose config 均通过。
- Worker 重复领取不会重复输出资产、usage 或 terminal event。
- SSE replay 不泄露跨租户或无权限事件。
- Provider runtime 不泄露 API Key、Authorization、Cookie、图片 base64 或原始 Provider payload。

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

### R7 结果

- P7 五个开发任务均已合并到 `main`，P7 可以结束并进入 P8。
- 前端验证通过：`cd frontend && npm run lint && npm run type-check && npm run test && npm run build`，18 个 test files / 63 个 tests 全部通过。
- 后端验证通过：`cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./cmd/api ./cmd/worker`。
- 关键包不走缓存验证通过：`go test ./internal/api ./internal/task ./internal/provider ./internal/provideradapter ./internal/sse -count=1`。
- Compose 验证通过：`docker compose -f deploy/docker-compose.yml config`。
- 已确认共享本地 `dev-mysql8`、`dev-redis`、`dev-minio` 服务健康可达；未创建项目专属开发环境。
- 静态扫描只命中既有 P8 迁移遗留：浏览器 Provider adapter 与本地设置存储。P7 未新增 Provider direct fetch、API Key persistence 或任务轮询路径。

### R7 非阻塞遗留 after R9

- 已在 P8/P9 处理：旧前端 Provider 直连、localStorage API Key、IndexedDB 本地历史主路径和 unreachable legacy display/storage helpers 已从生产路径移除或隔离。
- 已在 P10 处理：Worker now honors `WORKER_CONCURRENCY` as an in-process processing pool.
- P10 后续处理：API Redis event subscription 仍使用 background context，后续应绑定 server shutdown 生命周期。
- 当前 runtime 对已知 Provider API Key 已覆盖 value/key 两类脱敏；未知且不命中启发式规则的 secret 仍无法自动识别。
- 当前任务执行按 `modelId` 工作，未被 `model_name` 非唯一阻塞；若后续需要更强管理约束，仍需决定 `(tenant_id, provider_id, model_name)` 是否唯一。
- P10 后续处理：Provider soft delete 后的 linked-model 行为仍需确定。

## 第八批串行开发

P8 目标是把已有工作台从旧的浏览器直连执行路径迁移到后端平台路径。由于四个任务都会碰到工作台状态、组件和历史展示边界，P8 必须串行推进，不做并行 worktree。

### P8 调度顺序

1. `P8-FE-WORKBENCH-FOUNDATION` - completed and merged. 先切换模型与引用资产的来源。
2. `P8-FE-TASK-WORKBENCH` - completed and merged. 再切换提交、状态和结果输出。
3. `P8-FE-HISTORY-ASSET-SOURCE` - completed and merged. 再切换历史与再次编辑来源。
4. `P8-FE-LEGACY-RETIREMENT` - completed and merged. 最后退役旧直连和旧本地持久化生产路径。
5. `R8` - completed. 主 agent 串行 review、回归、静态扫描和合同校准。

### P8 总体约束

- P8 不新建后端业务合同；它消费 P5-P7 已冻结的项目、资产、模型、任务和 SSE 合同。
- P8 不提前处理 `WORKER_CONCURRENCY` worker pool、API Redis subscription shutdown、未知 secret 自动识别、Provider soft-delete linked-model 策略。这些属于 P9 或后续硬化。
- P8 不做 silent migration：旧 IndexedDB Blob 不得被悄悄上传到租户资产库。
- P8 允许保留非敏感本地 UI 偏好，但不得继续持久化 Provider API Key 或 Provider API URL。
- P8 结束后，主工作台生产路径不得再导入 `frontend/src/providers/**`，不得再把 IndexedDB 当成生成图或历史主数据源。

## 子任务 22：工作台模型与引用基础

### 任务名称

P8-FE-WORKBENCH-FOUNDATION - 切换工作台模型能力和引用资产来源

### 当前状态

Completed and merged into `main`.

Review result:

- 已建立 backend capability 加载、backend-ready task input、asset ID reference state。
- 首轮 review 发现默认工作台曾出现“新 UI / 旧请求脱节”、项目资产参考图静默失效、本地历史再次编辑死路；修复后再 review 通过。
- 最终实现保留默认 legacy 提交路径，只把 backend mode 作为显式准备态，保证迁移中间态仍可用且语义一致。
- 合并前验证通过：`npm run lint`、`npm run type-check`、`npm run test`、`npm run build`，18 个 test files / 67 个 tests 全部通过。

### 目标

在不替换提交链路的前提下，先把工作台的模型选择、参数展示和引用图选择切换到后端模型能力与项目资产 ID，为后续 task API 接入建立稳定状态模型。

### 允许修改文件

- `frontend/src/App.tsx`
- `frontend/src/api/models.ts`
- `frontend/src/components/studio/**`
- `frontend/src/components/projects/**`
- `frontend/src/hooks/useProjectAssets.ts`
- `frontend/src/hooks/useSettings.ts`
- `frontend/src/types/**`
- `frontend/src/test/controlPanel.test.tsx`
- `frontend/src/test/projectAssetsWorkbench.test.tsx`
- 与本任务新增代码直接对应的前端测试文件

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/src/hooks/useGeneration.ts`
- `frontend/src/hooks/useHistory.ts`
- `frontend/src/components/history/**`
- `frontend/src/providers/**`
- `frontend/src/db/**`

### 前置依赖

- R7 已完成。
- P5 asset API、P6 model capability API、P7 task API 合同已冻结。

### 具体开发内容

- 让工作台从 enabled backend model capability list 加载可选模型，而不是从 `providers/registry.ts` 读取生产可选项。
- 由后端能力字段驱动尺寸、质量、输出格式、输出数量和编辑/多参考图相关参数控件。
- 让参考图选择以 `assetId` 为稳定输入；本地 `File`/Blob 只能作为上传前临时状态，不能成为任务输入真值。
- 明确模型失效后的 UI 行为：刷新能力、提示重新选择，不在浏览器端伪造 Provider 能力。
- 保持现有上传、提示词、控制面板和视觉结构，不提前替换 `useGeneration`。

### 安全要求

- 不新增 Provider 直连请求、Authorization header 或 Provider credential 字段。
- 不新增 localStorage、sessionStorage、IndexedDB 中的敏感字段。
- 上传前端预检只能作为 UX，不能代替后端资产校验。

### 验收标准

- 工作台模型选择来自后端 capability API，生产选择逻辑不再依赖前端 Provider registry。
- 参数控件会根据 backend capability 禁用或隐藏不支持的组合。
- 参考图提交准备态持有项目资产 ID，而不是持久化 Blob。
- 现有生成提交行为尚未被改动，后续任务可以在这个状态模型上接入 task API。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
```

## 子任务 23：任务执行工作台

### 任务名称

P8-FE-TASK-WORKBENCH - 用 task API 与 SSE 替换工作台执行链路

### 当前状态

Completed and merged into `main`.

Review result:

- 默认工作台已切到 backend task create + SSE；主路径不会再触发浏览器 Provider submit。
- 结果区已经由 backend output assets 驱动，duplicate submit、stale model create failure、cancel/retry、SSE replay/heartbeat/canonical statuses 都有回归覆盖。
- 旧本地历史仍以显式兼容模式保留，再次编辑没有被提前打成死路。
- 合并前验证通过：`npm run lint`、`npm run type-check`、`npm run test`、`npm run build`，19 个 test files / 73 个 tests 全部通过。
- 非阻塞遗留：backend 结果图点击还没有真实详情内容；所有 HTTP `422` 暂时都按 stale model/capability 处理；asset reference 仍保留 `legacyFile` 兼容负担。这三项分别由后续历史切换、未来更细错误码合同和 legacy retirement 继续收口。

### 目标

把工作台生成/编辑提交从浏览器 Provider 执行切到后端任务创建，并用 SSE 驱动排队、执行、输出、失败、取消和完成状态。

### 允许修改文件

- `frontend/src/App.tsx`
- `frontend/src/api/tasks.ts`
- `frontend/src/components/studio/**`
- `frontend/src/hooks/useGeneration.ts`
- `frontend/src/lib/taskSseClient.ts`
- `frontend/src/types/**`
- `frontend/src/test/taskApi.test.ts`
- `frontend/src/test/taskSseClient.test.ts`
- `frontend/src/test/taskEventReducer.test.ts`
- 与本任务新增代码直接对应的前端测试文件

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/src/components/history/**`
- `frontend/src/hooks/useHistory.ts`
- `frontend/src/db/**`
- `frontend/src/providers/**`

### 前置依赖

- `P8-FE-WORKBENCH-FOUNDATION` 已合并。
- P7 task API、SSE event、cancel/retry 合同已可消费。

### 必须保持的现有行为

- 项目选择、参考图上传/选择、提示词编辑、现有结果区布局和本地历史查看入口在切换提交链路时仍然可用。
- 旧本地历史在 `P8-FE-HISTORY-ASSET-SOURCE` 接管前仍可查看和再次编辑；本任务不得把它提前打成死路。
- 工作台当前展示的可提交参数必须与实际创建的 backend task payload 一致。

### 允许的中间态

| Old path | Allowed intermediate state | Target path |
| --- | --- | --- |
| 浏览器 Provider submit + 本地结果 | 默认提交改为 backend task API + SSE，结果区显示 backend 输出资产；旧历史面板仍暂时保留为显式 legacy 数据源 | backend task API + SSE + backend 资产 + backend 历史 |

- 新生成任务可以已经走 backend，但历史面板尚未成为 backend 主数据源；这个差异必须在 UI 和代码语义上保持清楚，不能伪装成同一个数据源。

### 禁止的半迁移状态

- 不允许继续出现“backend UI 参数 / legacy 实际请求”脱节。
- 不允许同时触发浏览器 Provider submit 和 backend task create。
- 不允许用轮询补 SSE 的断线重连。
- 不允许把 backend task 输出复制回 IndexedDB 作为新的主历史真值。
- 不允许旧历史入口仍显示可用，点击后却无法继续当前允许的再次编辑路径。

### 失败模式与边界场景

| 场景 | 预期 |
| --- | --- |
| task create 成功 | 只创建一个 backend task，并进入 SSE 驱动状态流 |
| task create 因 stale Provider/model/capability 失败 | 明确展示后端错误，刷新能力并要求重选 |
| duplicate submit | 不重复创建 task |
| SSE 断线重连 / replay | 通过 EventSource 和历史补发恢复，不启用 polling |
| `FAILED` / `CANCELLED` / `RETRYING` / `TIMED_OUT` / `SUCCEEDED` | UI 与后端 canonical status 一致 |
| `IMAGE_OUTPUT` 先于 terminal event 到达 | 结果区可增量显示输出，最终状态仍由 terminal event 决定 |

### 必须新增或更新的回归测试

- backend task payload 与可见工作台参数一致。
- duplicate submit 不会创建多个 task。
- stale model create failure 会触发明确错误和重新选择路径。
- SSE reconnect / replay、heartbeat、terminal states 均不依赖 polling。
- backend 输出资产驱动结果区，不从 Provider 原始响应或本地 base64 渲染。
- 旧本地历史入口在本任务结束时仍保持明确且可用的兼容行为。

### 具体开发内容

- 把 `useGeneration` 改为创建后端 task，提交 `projectId`、`providerId`、`modelId`、`referenceAssetIds`、`editSourceAssetId` 和被 capability 允许的参数。
- 复用现有 task SSE client / reducer，以 SSE 事件驱动 `QUEUED`、`RUNNING`、`IMAGE_OUTPUT`、`USAGE`、`FAILED`、`CANCELLED`、`RETRYING`、`TIMED_OUT`、`SUCCEEDED` 状态。
- 结果区由后端 task output assets 和授权下载/预览信息驱动，不从 Provider 原始响应或本地 base64 直接渲染。
- 接入取消与重试操作，并处理 EventSource 重连、heartbeat、历史事件补发和 duplicate-submit 防护。
- 保留原有工作台布局、提示词体验和结果区概念。

### 安全要求

- 任务状态只能来自 SSE，禁止 `setInterval`、循环 `fetch`、轮询 fallback。
- 浏览器不得拼装 Provider-facing payload，不得创建 Provider Authorization header。
- 不渲染或记录图片 base64、Provider 原始 payload、API Key 或 Cookie。

### 验收标准

- 生成/编辑任务通过后端 API 创建，主工作台不再调用浏览器 Provider Adapter。
- 状态变化只由 SSE 推进，断线重连后可继续恢复事件。
- 结果区显示后端输出资产，失败/取消/重试/超时状态与后端合同一致。
- task create 的服务器校验失败会被清晰展示，并触发能力刷新或重新选择路径。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
```

## 子任务 24：历史与资产主数据切换

### 任务名称

P8-FE-HISTORY-ASSET-SOURCE - 用后端任务与资产替换本地历史主路径

### 当前状态

Completed and merged into `main`.

Review result:

- 默认历史、详情、下载和再次编辑路径已经切到 backend task/assets，旧 IndexedDB 历史被收成显式折叠的兼容入口。
- 当前 backend 结果图详情入口已经接到真实 backend asset/task detail，不再是 no-op。
- 首轮 review 发现 legacy 生成后误刷 backend history、backend re-edit source 会残留污染普通生成；修复后再 review 通过。
- 合并前验证通过：`npm run lint`、`npm run type-check`、`npm run test`、`npm run build`，20 个 test files / 83 个 tests 全部通过。
- 非阻塞遗留：当前 history 由前端对分页 task/assets 列表做 join，未来最好由 backend 提供统一 history query；history 加载失败且为空时仍可能同时显示 empty state 与 error state。

### 目标

把历史面板、图片详情、下载和再次编辑的主数据源切换到后端项目任务与生成/编辑资产，不再以 IndexedDB 历史记录作为平台真值。

### 允许修改文件

- `frontend/src/App.tsx`
- `frontend/src/api/assets.ts`
- `frontend/src/api/tasks.ts`
- `frontend/src/components/history/**`
- `frontend/src/components/modals/ImageDetailModal.tsx`
- `frontend/src/components/studio/ResultCanvas.tsx`
- `frontend/src/hooks/useHistory.ts`
- `frontend/src/hooks/useProjectAssets.ts`
- `frontend/src/types/**`
- 与本任务新增代码直接对应的前端测试文件

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/src/providers/**`
- `frontend/src/hooks/useSettings.ts`
- `frontend/src/db/**`

### 前置依赖

- `P8-FE-TASK-WORKBENCH` 已合并。
- 后端 task list/detail、asset download/detail 合同已稳定。

### 必须保持的现有行为

- `P8-FE-TASK-WORKBENCH` 已接管的 backend task submit、SSE 状态流和 backend result asset 显示必须保持可用。
- 授权下载、当前项目上下文和再次编辑概念必须继续存在。
- 若保留 legacy history 兼容入口，它必须保持显式、可理解、非默认，不得偷偷与 backend 主历史混成一体。
- 当前 backend 结果图虽然已经可展示，但详情入口还没有真实 backend 内容；本任务必须把它接到后端 asset/task detail，不得继续保留无反馈点击。

### 允许的中间态

| Old path | Allowed intermediate state | Target path |
| --- | --- | --- |
| IndexedDB local history 为默认历史 | 默认历史切到 backend task/assets；legacy history 若保留，只能是显式兼容入口 | backend task history + generated/edited assets 为唯一生产主路径 |

### 禁止的半迁移状态

- 不允许把 local history 与 backend history 混成用户无法区分的同一列表。
- 不允许把旧 IndexedDB Blob 静默上传进租户资产库。
- 不允许再次编辑按钮还存在，但实际仍依赖已失效的本地 Blob 回灌。
- 不允许下载绕过后端鉴权或前端构造 MinIO 公网 URL。
- 不允许为了刷新历史而引入 polling。

### 失败模式与边界场景

| 场景 | 预期 |
| --- | --- |
| 当前项目无历史 | 显示空态，不回退到别的项目数据 |
| 后端 task 有输出资产 | 历史项、详情、下载、再次编辑都基于授权 asset |
| 输出资产已删除或不可见 | 展示非泄露性错误，不暴露跨租户存在性 |
| 下载失败 | 保持 UI 可恢复，不绕过后端鉴权 |
| 再次编辑时 source asset 不可用 | 明确提示并阻止创建无效任务 |
| 切换项目 | 历史、详情和编辑上下文随项目切换，不串数据 |
| 点击当前 backend 结果图 | 打开真实 backend 资产/任务详情，不出现无反馈点击 |

### 必须新增或更新的回归测试

- 默认历史来源已经是 backend task/assets，而非 IndexedDB。
- 空态、项目切换、asset 不可见或已删除场景不串租户/项目数据。
- 下载继续通过后端授权接口。
- 再次编辑使用 backend `assetId` / `editSourceAssetId`，不依赖 Blob 回灌。
- 当前 backend 结果图详情入口已经接到后端详情数据，不再是 no-op。
- 若保留 legacy history 兼容入口，测试必须证明它是显式且非默认的。

### 具体开发内容

- 历史面板改读项目 task history 与 generated/edited assets。
- 图片详情、下载和再次编辑入口都基于后端 asset/task 信息工作。
- 接管当前 backend 结果图的详情入口，让结果区和历史区共用真实的后端 asset/task detail 语义。
- 再次编辑使用已有 `assetId` 作为 `editSourceAssetId`，不再依赖 IndexedDB Blob 回灌。
- 明确旧本地历史的展示策略：若保留，只能是单独、显式、非默认的兼容入口；不得混成平台主历史。
- 保持用户能查看过去结果、下载、再次编辑的工作流完整。

### 安全要求

- 下载必须继续经过后端鉴权接口，前端不能构造 MinIO 公网 URL。
- 不把旧 IndexedDB Blob 自动上传到租户存储。
- 不暴露跨项目或跨租户资产；前端错误提示不得泄露对象是否存在。

### 验收标准

- 历史面板默认展示后端项目任务/资产数据。
- 再次编辑从后端资产创建新任务，不依赖本地 Blob。
- 下载和详情继续使用授权后端 API。
- IndexedDB 已不再是平台历史或生成图主数据源。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
```

## 子任务 25：旧链路退役

### 任务名称

P8-FE-LEGACY-RETIREMENT - 移除或隔离旧直连与旧本地持久化生产路径

### 当前状态

Completed and merged into `main`.

Review result:

- 删除了浏览器 Provider adapter、frontend Provider registry/types、普通 Provider API Key/API URL 设置入口和相关旧测试。
- 生产工作台只使用 `BackendControlPanel`、backend task API、SSE、authorized backend assets 和 backend history/detail/download/re-edit。
- `legacyFile` 已从 project asset reference 类型和生产路径中移除；项目资产作为参考图只提交 backend `assetId`。
- 项目切换会清空待提交参考图，避免把旧项目 assetId 带入新项目任务。
- 静态扫描确认生产路径没有 Provider direct host、Provider Authorization header、Provider key storage、task polling 或 `frontend/src/providers/**` import。
- 合并前验证通过：`npm run lint`、`npm run type-check`、`npm run test`、`npm run build`，18 个 test files / 59 个 tests 全部通过。
- 非阻塞遗留：`ResultCanvas`、`ImageDetailModal`、`LegacyHistoryPanel`、旧 IndexedDB history/image helper 和 `useStorageUsage` 仍有不可达或非生产残留，P9 应删除或明确 quarantine；generic HTTP `422` handling、history frontend join 和 error/empty-state overlap 仍待 P9/hardening 收口。

### 目标

在后端工作流已经替换完成后，清理旧浏览器 Provider 直连、Provider 凭据设置、IndexedDB 图片/历史生产引用和相关测试，完成 P8 的迁移收口。

### 允许修改文件

- `frontend/src/App.tsx`
- `frontend/src/components/modals/SettingsModal.tsx`
- `frontend/src/hooks/useSettings.ts`
- `frontend/src/hooks/useGeneration.ts`
- `frontend/src/hooks/useHistory.ts`
- `frontend/src/providers/**`
- `frontend/src/db/**`
- `frontend/src/lib/constants.ts`
- `frontend/src/test/**`
- 与本任务新增代码直接对应的前端文件

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- 未经主 agent 同意的大规模 UI 重写

### 前置依赖

- `P8-FE-HISTORY-ASSET-SOURCE` 已合并。
- 后端任务、SSE、资产和历史路径已经能覆盖主工作流。

### 必须保持的现有行为

- backend task submit、SSE、backend history、authorized download、再次编辑和 admin Provider/model 管理必须继续可用。
- 非敏感 UI 偏好如果仍有价值可以保留，但不得夹带 Provider credentials。
- 如果保留 legacy import/compat 入口，它必须是明确、默认关闭、不会触发 Provider 请求或租户写入的独立路径。
- `P8-FE-HISTORY-ASSET-SOURCE` 已建立的默认 backend history/detail/download/re-edit 路径必须继续是生产主路径。

### 允许的中间态

| Old path | Allowed intermediate state | Target path |
| --- | --- | --- |
| 旧 Provider adapters、Provider local settings、IndexedDB 主历史仍在代码中 | 旧代码被删除或严格隔离为非生产兼容引用；主工作台只走 backend API/SSE/assets | 生产前端完全 backendized，旧直连和旧敏感本地路径退出 |

### 禁止的半迁移状态

- 不允许主工作台 import 旧 Provider adapter。
- 不允许旧 API Key / Provider URL 从 localStorage 挪到另一种浏览器存储“换皮继续存在”。
- 不允许删除旧入口后留下仍被 UI 引用的死链接或失效设置项。
- 不允许删除兼容代码时顺手静默上传、删除或改写旧用户本地 Blob。
- 不允许为了兼容旧行为重新引入 direct fetch 或 polling。

### 失败模式与边界场景

| 场景 | 预期 |
| --- | --- |
| 浏览器里仍残留旧 Provider key/settings | 启动后不再被生产路径读取或使用 |
| 旧 IndexedDB 里仍有历史 Blob | 不会被静默上传，不会成为默认历史源 |
| 用户打开普通设置页 | 不再看到 Provider API Key / Provider URL 输入 |
| 兼容代码若保留 | 默认不可达，且不触发 Provider 网络调用、任务创建或租户写入 |
| 旧 legacy history UI 被删除或收口 | 不留下仍可见但已失效的按钮、菜单或提示语 |
| 静态扫描命中 `localStorage` / `providers` | 每个保留命中都有明确、非敏感、非生产理由 |

### 必须新增或更新的回归测试

- 普通设置流不再接受或保存 Provider API Key / Provider URL。
- 主工作台生产 imports 不再触达 `frontend/src/providers/**`。
- IndexedDB 不再承担生成图或历史主数据职责。
- 浏览器旧设置残留不会被 production flow 读取使用。
- 若保留 legacy 兼容入口，测试必须证明它不会再触发 Provider submit、任务创建或新的本地生成。
- 若移除 legacy 兼容入口，测试必须证明不存在仍可见但失效的按钮和文案。
- 静态扫描和必要测试覆盖 direct fetch、Authorization、polling、sensitive storage 的移除。

### 具体开发内容

- 删除或彻底隔离生产路径上的 `frontend/src/providers/**` 导入；如保留兼容代码，必须显式命名并从主工作台不可达。
- 从普通设置流移除 Provider API Key 和 Provider API URL 持久化；只保留非敏感 UI 偏好。
- 删除已经失去生产意义的 IndexedDB image/history 主路径引用和相关回归测试，或把它们降级为清晰隔离的兼容代码。
- 收口当前显式 legacy history / legacy generation 兼容路径：要么删除，要么隔离为不会触发 Provider submit、不会创建任务、不会再被默认工作流使用的非生产入口。
- 用静态扫描和测试证明旧直连、旧密钥持久化、旧 history primary path 已退出生产路径。

### 安全要求

- 静态扫描必须确认生产路径不再存在 Provider API Key 持久化、Provider Authorization header 构造、AI Provider direct fetch 或任务状态轮询。
- 不得把旧敏感配置迁移到其他浏览器存储位置。
- 如果保留兼容入口，必须默认关闭、明确命名、且不触发网络调用或租户数据写入。

### 验收标准

- 主工作台生产 imports 中无 `frontend/src/providers/**`。
- 普通设置界面不再接收或保存 Provider API Key / API URL。
- IndexedDB 不再承担生成图或历史主数据职责。
- 若仍保留 `localStorage`，只允许非敏感 UI 偏好；任何命中都必须在 review 中逐条确认。
- P8 迁移后的主流程在刷新、重新登录、任务重连和再次编辑场景下仍可用。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build

rg -n "localStorage|sessionStorage|indexedDB|Authorization|openai|gemini|setInterval|setTimeout" frontend/src
```

## 串行阶段 8：P8 review 和集成

### 任务名称

R8 - 前端后端化 review、回归和迁移验收

### 目标

由主 agent 串行 review P8 四个子任务，确认主工作台已完成从浏览器本地链路到后端平台链路的迁移，再进入 P9。

### 当前状态

Completed by the main agent on `main`. The temporary branch `codex/r8-p8-regression-review` was deleted before execution at user request.

R8 verification result:

- P8 four frontend tasks are merged into `main`.
- Frontend regression passed: `npm run lint`, `npm run type-check`, `npm run test`, and `npm run build`; 18 test files / 59 tests passed.
- Backend regression passed: `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go build ./cmd/api ./cmd/worker`.
- Docker Compose config validation passed with `docker compose -f deploy/docker-compose.yml config`.
- Sensitive frontend static scan returned no production-code hits for `localStorage`, `sessionStorage`, `indexedDB`, Provider `Authorization`, direct Provider hosts, `setInterval`, or `setTimeout`.
- Provider import static scan found only backend Provider management API paths: `frontend/src/api/providers.ts` and `frontend/src/components/admin/ProviderModelAdminPanel.tsx`. These are allowed admin API consumers.
- `frontend/src/providers/` no longer exists.

R8 residual scan classification:

| Static scan class | Result | Disposition |
| --- | --- | --- |
| Browser Provider adapters / registry | No files under `frontend/src/providers/` | Resolved in P8 |
| Provider key storage / Provider URL storage | No production hits | Resolved in P8 |
| Direct Provider hosts / Provider Authorization header | No production hits | Resolved in P8 |
| Task polling (`setInterval` / `setTimeout`) | No production hits | Resolved in P8 |
| Backend Provider management API | `frontend/src/api/providers.ts`, `ProviderModelAdminPanel.tsx` | Allowed; talks only to `/api/v1/providers` backend endpoints |
| Prompt template IndexedDB | `PromptEditor` -> `promptTemplateRepository` | Allowed non-sensitive local UX data |
| Legacy display / old DB helpers | `LegacyHistoryPanel`, legacy branches in `ResultCanvas` / `ImageDetailModal`, old local history/image helpers, `useStorageUsage` | Resolved in P9 security regression; deleted or protected by static regression coverage |

### 允许修改文件

- `docs/**`

### 禁止修改文件

- `frontend/**`
- `backend/**`
- `deploy/**`
- `AGENTS.md`
- `agent-instructions/**`

### 前置依赖

- `P8-FE-WORKBENCH-FOUNDATION`
- `P8-FE-TASK-WORKBENCH`
- `P8-FE-HISTORY-ASSET-SOURCE`
- `P8-FE-LEGACY-RETIREMENT`

### 具体开发内容

- Review P8 合并结果和残余兼容代码。
- 跑前端完整回归、后端必要回归、Compose config 和关键静态扫描。
- 确认项目资产、任务 API、SSE 和历史 UI 已成为主生产路径。
- 更新公共合同中的 P8 实际完成状态、残余风险和 P9 前置条件。
- 明确 P9 首批任务是否从审计/用量/系统设置横切任务拆小，避免一个过大的 `P9-AUDIT-HARDENING` worktree。

### 安全要求

- 重点检查浏览器直连、密钥持久化、任务轮询、未经授权下载、静默上传旧 Blob、图片 base64 日志。
- 不允许因兼容入口重新引入旧违规链路。
- 对静态扫描保留命中必须逐条分类：backend Provider 管理 API、非敏感 prompt-template IndexedDB、不可达 legacy residue、测试-only 命中，不能笼统放过。

### 验收标准

- P8 四个子任务均已合并到 `main`。
- 主工作台生产路径只使用 backend API + SSE + authorized assets。
- 静态扫描和回归测试未发现旧直连、旧密钥持久化或 polling；允许保留的本地存储命中已逐条确认只承载非敏感 UI 偏好。
- 文档已经反映 P8 实际完成情况和 P9 遗留。
- P9 的下一批任务包必须按 P8/P9 强制任务包标准拆小，并说明是否串行或有限并行。

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
rg -n "from ['\\\"]\\.\\.?/providers|localStorage|sessionStorage|indexedDB|Authorization|setInterval|setTimeout" frontend/src
git diff --check
```

## P9：用量、审计、系统设置、硬化和发布准备

P9 must not run as one broad worktree. Start serially with backend read APIs, then add backend settings, then frontend admin UI, then security/deploy hardening.

### P9 调度顺序

1. `P9-BE-AUDIT-USAGE-READS` - completed and merged. Defines safe backend read contracts for usage, operation logs, and API call logs.
2. `P9-BE-PRODUCTION-SECRET-GUARD` - completed and merged. Rejects production placeholder secrets before API or Worker startup can proceed.
3. `P9-BE-RUNTIME-SETTINGS-CONTRACT` - completed and merged. Implemented only the first runtime-backed settings slice: tenant upload policy consumed by backend asset validation.
4. `P9-FE-ADMIN-OBSERVABILITY-SETTINGS` - completed and merged. Added frontend admin views for usage/logs/settings using only merged backend contracts.
5. `P9-SECURITY-REGRESSION` - completed and merged. Added targeted security regression tests and residual legacy helper cleanup.
6. `P9-DEPLOY-RELEASE-VALIDATION` - completed, reviewed, and merged. Compose build/up/healthcheck passed, release docs/runbook were updated, and the project Compose stack was cleaned up.

First batch completed: `P9-BE-AUDIT-USAGE-READS` merged after review-driven redaction fixes.

Second batch correction: the original `P9-BE-SYSTEM-SETTINGS-HARDENING` package was too broad. Review of the live code showed that writable defaults/upload limits/concurrency cannot honestly land without changing task creation, asset validation, and worker runtime consumers. Split the work:

1. open only `P9-BE-PRODUCTION-SECRET-GUARD`;
2. after it merges, main agent must define `P9-BE-RUNTIME-SETTINGS-CONTRACT` with the actual runtime write scope before any settings API is exposed.

Second batch completed: `P9-BE-PRODUCTION-SECRET-GUARD` merged after review required the startup-path failure matrix to be completed for both API and Worker entrypoints.

Third batch completed: `P9-BE-RUNTIME-SETTINGS-CONTRACT` exposed only tenant upload policy in its first slice and wired it into backend asset validation. `defaultProviderId/defaultModelId`, tenant concurrency, storage quotas, and log retention stay deferred because task creation, Worker limit resolution, quota enforcement, and cleanup jobs are not in scope yet.

Fourth batch completed: `P9-FE-ADMIN-OBSERVABILITY-SETTINGS` merged after review. The accepted frontend UI consumes paginated usage/audit reads and only the narrow `uploadPolicy` system setting, keeps Provider/model admin intact, and adds tests for permission gating, deferred-setting absence, browser-storage safety, and admin API contracts.

Fifth batch completed: `P9-SECURITY-REGRESSION` merged after review. The accepted changes added targeted SSRF, redaction, tenant/object authorization, upload validation, SSE replay, task permission, production frontend static-safety tests, and deleted unreachable legacy history display/storage helpers.

Sixth batch completed: `P9-DEPLOY-RELEASE-VALIDATION` merged after review. This was the deployment-specific exception to the shared-local-services rule: the child agent validated the project Compose stack and cleaned it up afterwards. No product feature or API contract changes were introduced.

R9 completed after all P9 development tasks merged. Main-agent review covered P9 code from R8 completion through `P9-DEPLOY-RELEASE-VALIDATION`, excluded the later P10 planning commit from P9 scope, and found no blocking issues. Full frontend, backend, race, vet, build, Compose config/build/up/health, API health, frontend static route, and Compose cleanup checks passed. Non-blocking carry-forward items are: admin API-call detail stale-response guard, large admin observability/settings component split, and explicit Redis health-check client lifecycle if health dependencies later become reloadable.

P10 starts serially after R9. The first task is `P10-BE-WORKER-POOL`; do not start SSE bridge lifecycle, Provider/model lifecycle, frontend admin hardening, or history query work in parallel.

## 子任务 26：审计与用量只读 API

### 任务名称

P9-BE-AUDIT-USAGE-READS - 后端用量、操作日志和 API 调用日志查询

### 目标

为 admin 用户提供 tenant-scoped usage summary、usage records、operation logs、api call logs 的只读查询 API，带分页、过滤、RBAC、对象级边界和递归脱敏。

### 状态

Completed and merged into `main`. Review required one follow-up fix to centralize redaction into `backend/internal/redaction`, prove exact known-secret value/key scrubbing through a controlled injection seam, and add deterministic same-timestamp pagination coverage. Production read handlers intentionally do not widen Provider plaintext-key decryption scope; if historical dirty rows need exact non-heuristic scrubbing later, first define a trusted minimal secret source and lifecycle.

### 允许修改文件

- `backend/internal/api/**`
- `backend/internal/audit/**`
- `backend/internal/task/**` 仅限复用或补充 usage/api-call/log 查询 DTO，不改 Worker 状态机
- `backend/internal/rbac/**` 仅限新增权限码/测试
- `backend/internal/database/**` 仅限必要索引或只读查询支持
- `backend/internal/httpx/**` 仅限复用分页/响应 helper
- `backend/cmd/**` 仅限路由注册需要
- `backend/internal/**/**/*_test.go`

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- Provider runtime、Worker claim/complete/cancel 状态机，除非发现编译阻塞并先报告

### 前置依赖

- R8 completed.
- Existing P7/P8 task, provider runtime, usage record, api call log, and operation log writes are merged.

### 必须保持的现有行为

- P8 backendized workbench, task/SSE, authorized asset download, Provider/model management, tenant isolation, RBAC, and redaction behavior remain stable.
- Existing logs and usage records keep their write semantics; this task only exposes safe read APIs.

### 允许的中间态

- Backend read APIs may land before frontend admin UI.
- API response shape may be minimal but must be stable, paginated, tenant-scoped, and documented in code/tests.

### 禁止的半迁移状态

- No endpoint may return cross-tenant rows.
- No endpoint may return full API keys, Authorization headers, Cookies, image base64, raw image bytes, or unredacted nested metadata.
- No unpaginated list endpoint.
- No UI stubs in this task.

### 失败模式与边界场景

| 场景 | 预期 |
| --- | --- |
| non-admin user queries logs or usage | RBAC rejects with 403 |
| tenant A asks for tenant B records by ID or filter | no rows or 404/403 without existence leak |
| metadata contains known secret, nested secret, Authorization, Cookie, or base64-like image payload | response is recursively redacted |
| empty result set | returns empty page with pagination metadata |
| invalid page/filter params | returns validation error without SQL/log leakage |
| large result set | requires pagination and deterministic ordering |

### 必须新增或更新的回归测试

- RBAC denial for each read API.
- Tenant isolation for usage records, operation logs, and API call logs.
- Recursive metadata redaction in list/detail responses.
- Pagination and filter validation.
- No direct exposure of raw Provider request/response payload secrets.

### 具体开发内容

- Add admin-only read endpoints under `/api/v1/admin` or the existing admin route pattern for:
  - usage summary by user/project/model/provider/date range.
  - usage records list.
  - operation logs list.
  - API call logs list/detail.
- Reuse existing response envelope and pagination conventions.
- Reuse or centralize redaction helpers so nested metadata is sanitized before response serialization.
- Add permissions such as `usage:read`, `operation_log:read`, and `api_call_log:read` only if not already present.

### 安全要求

- All queries must include tenant scope.
- All endpoints require authentication and admin/RBAC permissions.
- Log/API metadata must be recursively redacted at response time even if already redacted at write time.
- Do not log request filters if they may contain secrets.

### 验收标准

- Backend exposes safe, paginated read APIs for usage, operation logs, and API call logs.
- Tests prove RBAC, tenant isolation, pagination, and redaction.
- No frontend or deployment files are changed.

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker
```

## 子任务 27：生产 secret 启动硬化

### 任务名称

P9-BE-PRODUCTION-SECRET-GUARD - 生产 placeholder secret 启动拒绝

### 目标

仅实现生产环境 placeholder secret 启动拒绝，确保 `APP_ENV=production` 时 API 和 Worker 都不能带默认 JWT signing secret 或默认 API-key encryption secret 启动。此任务不实现 system settings API，也不制造尚未被 runtime 消费的可写设置。

### 状态

Completed and merged into `main`. Review required one follow-up test fix so both API and Worker startup paths explicitly cover placeholder JWT and placeholder API-key-encryption rejection. Production startup now fails fast on both placeholders, explicit replacement secrets still load, and non-production defaults remain valid.

### 允许修改文件

- `backend/internal/config/**`
- `backend/internal/**/**/*_test.go`
- `backend/cmd/api/**`
- `backend/cmd/worker/**`

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/internal/api/**`
- `backend/internal/database/**`
- `backend/internal/task/**`
- `backend/internal/asset/**`
- `backend/internal/provider/**`
- `backend/internal/model/**`
- `backend/internal/rbac/**`
- Worker claim/complete/cancel 状态机
- SSE、queue、Provider Adapter runtime、task execution 主流程

### 前置依赖

- `P9-BE-AUDIT-USAGE-READS` completed and merged.
- Existing config defaults and startup entrypoints are available on `main`.

### 必须保持的现有行为

- Existing local/development/test startup behavior remains valid.
- Existing auth, RBAC, Provider/model management, task/SSE flow, P8 backendized workbench, and P9 audit reads remain unchanged.

### 允许的中间态

- Only startup guard behavior lands in this task.
- The future system-settings contract remains deferred until runtime consumers are deliberately in scope.

### 禁止的半迁移状态

- No production startup may continue with placeholder `JWT_SIGNING_SECRET`, placeholder `API_KEY_ENCRYPTION_KEY`, or similarly unsafe built-in defaults.
- No new settings API, schema, or fake-active settings surface.
- No log output may reveal actual secret values.

### 失败模式与边界场景

| 场景 | 预期 |
| --- | --- |
| `APP_ENV=production` with placeholder JWT or API-key encryption secret | API and worker startup fail fast before serving work |
| `APP_ENV=production` with explicit non-placeholder secrets | startup succeeds |
| non-production local/dev config uses placeholders | existing developer flow remains available unless explicitly tightened later |
| startup validation fails | error message identifies the config field but never echoes the secret value |

### 必须新增或更新的回归测试

- Production startup rejection for placeholder JWT and API-key encryption secrets in both API and worker startup paths.
- Production startup acceptance for explicit replacement secrets.
- Non-production startup remains valid with existing local defaults.
- Error text is operator-useful without containing secret values.

### 具体开发内容

- Add startup validation that rejects built-in placeholder production secrets for API and worker entrypoints.
- Keep the validation close to config loading / startup so both entrypoints share the same rule rather than duplicating ad hoc checks.
- Keep secret-validation errors safe: clear enough for operators, never echo actual secret values.

### 安全要求

- Production secret validation must not log actual secret values.
- Do not add any settings API or widen secret handling scope in this task.

### 验收标准

- Production API and worker startup fail before serving when placeholder secrets are configured.
- Explicit production secrets and existing non-production defaults still load correctly.
- No settings API, schema, frontend, deploy, docs, or unrelated runtime paths are modified.

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker
```

## 子任务 28：运行时设置合同

### 任务名称

P9-BE-RUNTIME-SETTINGS-CONTRACT - 真实可执行的系统设置 API 与 runtime 消费链路

### 状态

Completed and merged into `main`. Review accepted the narrow contract because every exposed writable field is consumed by backend asset upload runtime. The merged backend keeps deferred settings absent from responses and rejected on writes, enforces tenant isolation/RBAC/CSRF, records sanitized operation logs, and uses environment upload limits as hard caps and fallback defaults.

### 推荐执行信息

- 推荐线程名：`P9-BE-RUNTIME-SETTINGS-CONTRACT`
- 推荐分支名：`codex/p9-backend-runtime-settings-contract`
- 起始分支：最新 `main`
- 开发顺序：串行执行。该任务完成、review、合并和回归后，再进入 `P9-FE-ADMIN-OBSERVABILITY-SETTINGS`；不要并行启动前端 admin UI。

### 目标

实现第一段真实可执行的 system settings 合同：只暴露 tenant-scoped `uploadPolicy`，并让 backend asset upload 在每次请求时消费该策略。此任务不实现默认 Provider/model、租户并发、存储配额或日志保留周期，因为这些字段当前还没有同批次 runtime consumer。

### 设置字段与 runtime consumer 对照表

| 对外字段 | Runtime consumer | 本任务是否在范围内 |
| --- | --- | --- |
| `uploadPolicy.maxFileSizeBytes` | Asset upload request-body limit and upload validator | 是 |
| `uploadPolicy.maxWidth` | Asset upload image-dimension validator | 是 |
| `uploadPolicy.maxHeight` | Asset upload image-dimension validator | 是 |
| `uploadPolicy.maxPixels` | Asset upload pixel-count validator | 是 |
| `defaultProviderId` | Task creation fallback resolution | 否，继续要求显式 `providerId` |
| `defaultModelId` | Task creation fallback resolution | 否，继续要求显式 `modelId` |
| `tenantConcurrency` | Worker concurrency-dimension resolution | 否 |
| `storageQuotaBytes` | Asset/task storage quota enforcement | 否 |
| `logRetentionDays` | Cleanup/retention job | 否 |

### 允许修改文件

- `backend/internal/api/**`
- `backend/internal/asset/**`
- `backend/internal/database/**`
- `backend/internal/settings/**`
- `backend/internal/rbac/**` 仅限复用或测试 `system:settings:manage`
- `backend/internal/httpx/**` 仅限必要分页/响应复用
- `backend/cmd/api/**` 仅限路由装配
- `backend/internal/**/**/*_test.go`

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/internal/task/**`
- `backend/cmd/worker/**`
- `backend/internal/provider/**`
- `backend/internal/model/**`
- `backend/internal/sse/**`
- `backend/internal/queue/**`
- Provider Adapter runtime
- Worker claim/complete/cancel 状态机

### 前置依赖

- `P9-BE-AUDIT-USAGE-READS` completed and merged.
- `P9-BE-PRODUCTION-SECRET-GUARD` completed and merged.
- Public contract docs now state that only runtime-backed upload policy is active in this slice.

### 必须保持的现有行为

- Existing task creation still requires explicit `providerId` and `modelId`.
- Existing non-admin flows, project/asset authorization, Provider/model management, task/SSE flow, and P8 backendized workbench remain unchanged.
- Existing environment upload config remains the hard-cap fallback for tenants with no override row.
- Allowed MIME types remain config-owned and SVG remains forbidden.

### 允许的中间态

- Backend admin settings API may land before frontend admin UI.
- Only tenant upload policy is exposed as writable active state.
- Tenants without a persisted override continue using effective upload limits derived from current config.

### 禁止的半迁移状态

- No response may expose fake-active `defaultProviderId`, `defaultModelId`, concurrency, quota, or retention fields.
- No settings API may accept values that asset upload does not actually consume.
- No tenant upload-policy override may raise limits above the configured environment hard caps.
- No static asset upload validation path may silently bypass tenant overrides once the override exists.

### 失败模式与边界场景

| 场景 | 预期 |
| --- | --- |
| non-admin reads or writes system settings | RBAC rejects with `403` |
| tenant A reads or updates settings | only tenant A effective settings are visible or changed |
| tenant has no override row | GET returns config-derived effective upload policy; upload uses same limits |
| PATCH narrows file size / dimensions / pixels | subsequent uploads for that tenant enforce the new narrower limits |
| PATCH attempts zero, negative, malformed, or over-hard-cap values | validation fails; old effective policy remains unchanged |
| tenant A override exists, tenant B has none | tenant B still uses its own config fallback and is unaffected |
| PATCH succeeds | sanitized operation log is recorded without secrets or raw file content |
| client sends deferred fields such as `defaultProviderId` or `tenantConcurrency` | validation rejects them rather than pretending they are active |

### 必须新增或更新的回归测试

- RBAC denial for GET/PATCH system-settings endpoints.
- Effective GET fallback when no tenant override exists.
- Tenant isolation for reads and updates.
- PATCH validation for invalid and over-hard-cap upload values.
- Asset upload enforcement after tenant policy is narrowed.
- Deferred-field rejection for fake-active settings.
- Sanitized operation-log write on successful update.

### 具体开发内容

- Add `system_settings` persistence for tenant-scoped settings, with unique `(tenant_id, key)` handling and the first active key `upload_policy`.
- Add a settings domain/service/repository for effective tenant upload policy resolution.
- Implement `GET /api/v1/admin/system-settings` and `PATCH /api/v1/admin/system-settings`.
- Keep the response/request surface limited to `uploadPolicy.{maxFileSizeBytes,maxWidth,maxHeight,maxPixels}`.
- Resolve effective upload policy per request in the asset upload path so tenant overrides are enforced before image persistence.
- Keep config upload values as hard caps and fallback defaults; tenant overrides may only narrow or match them.
- Record operation logs for successful settings updates with sanitized metadata.

### 安全要求

- System settings APIs require Cookie auth, tenant admin access, CSRF for PATCH, and `system:settings:manage`.
- All settings reads/writes must filter by `tenant_id`.
- Upload-policy overrides must never widen the configured MIME allowlist or permit SVG.
- Operation logs must not contain secrets, Authorization headers, Cookies, image bytes, or image base64.
- Do not expose deferred settings as active writable state.

### 验收标准

- Active system settings contract is truthful: every exposed writable field is consumed by asset upload runtime.
- Tenant upload-policy overrides are tenant isolated, bounded by config hard caps, and enforced by real uploads.
- Tenants without overrides preserve current behavior through config fallback.
- Deferred settings remain absent from API responses and rejected on writes.
- No frontend, worker, task, provider, model, queue, SSE, deploy, docs, or agent-rule files are modified.

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker
git diff --check
```

## 子任务 29：管理端用量、日志和系统设置 UI

### 任务名称

P9-FE-ADMIN-OBSERVABILITY-SETTINGS - 前端管理端观测与设置页面

### 状态

Completed and merged into `main`. Review found no blockers. Non-blocking follow-ups: `AdminObservabilitySettingsPanel` is large and can be split later; API-call detail lacks a stale-request guard, so slow detail responses can overwrite a newer click for the same admin user.

### 目标

在已有 admin Provider/model 管理基础上增加 usage summary、usage records、operation logs、api call logs 和 system settings UI，只消费 P9 后端真实合同，不展示未生效的假设置。

### 推荐执行信息

- 推荐线程名：`P9-FE-ADMIN-OBSERVABILITY-SETTINGS`
- 推荐分支名：`codex/p9-frontend-admin-observability-settings`
- 起始分支：最新 `main`
- 开发顺序：串行执行。该任务完成、review、合并和回归后，再进入 `P9-SECURITY-REGRESSION`；不要并行启动安全回归或部署发布任务。

### 允许修改文件

- `frontend/src/api/**`
- `frontend/src/types/**`
- `frontend/src/components/admin/**`
- `frontend/src/App.tsx` 仅限接入现有 admin 入口和权限可见性
- `frontend/src/components/**` 仅限 admin 入口/状态展示所需的局部复用，不重构工作台
- `frontend/src/test/**`

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/src/providers/**` 或任何已退役浏览器 Provider 直连代码
- `frontend/src/hooks/useGeneration.ts`、task/SSE runtime、workbench 生成主流程，除非发现编译阻塞并先报告
- 任何 localStorage/sessionStorage/IndexedDB auth token、Provider API key、Provider URL 或日志敏感字段持久化代码

### 前置依赖

- `P9-BE-AUDIT-USAGE-READS`
- `P9-BE-RUNTIME-SETTINGS-CONTRACT`
- Existing P8 backendized workbench and P6 Provider/model admin UI are merged on `main`.

### 必须保持的现有行为

- Provider/model admin management remains usable and keeps its current key-masking and no-browser-persistence behavior.
- Workbench generation still uses backend task creation plus SSE only; no polling or Provider direct calls are reintroduced.
- Existing project/asset/task/history flows remain unchanged.
- Non-admin users, or users lacking the relevant permission, do not see admin observability/settings controls.

### 允许的中间态

- Frontend admin UI may ship before a richer backend dashboard aggregation exists.
- Empty states may be simple if the backend returns no records.
- Settings UI may cover only `uploadPolicy.{maxFileSizeBytes,maxWidth,maxHeight,maxPixels}` because that is the only active backend settings slice.

### 禁止的半迁移状态

- Do not display `defaultProviderId`, `defaultModelId`, tenant concurrency, storage quota, or log retention as active editable controls.
- Do not create frontend-only settings that are not saved through or consumed by backend runtime.
- Do not fetch unbounded logs or usage rows.
- Do not render unredacted secrets or provide a "show raw secret" path.
- Do not add `setInterval`, repeated `setTimeout`, repeated fetch loops, or polling for admin lists.
- Do not use browser storage for filters if those filters may contain IDs, errors, metadata, or sensitive text.

### 失败模式与边界场景

| 场景 | 预期 |
| --- | --- |
| user lacks `usage:read` | usage summary/records UI is hidden or disabled without triggering unauthorized fetches |
| user lacks `audit:read` | operation/API-call log UI is hidden or disabled without triggering unauthorized fetches |
| user lacks `system:settings:manage` | settings UI is hidden or read-only without PATCH capability |
| backend returns empty pages | UI shows bounded empty state with pagination metadata preserved |
| backend returns validation error for settings PATCH | form keeps user input visible, shows API error, and does not claim success |
| backend returns redacted metadata | UI displays redacted fields only; no attempt is made to recover hidden values |
| API call log detail is large | UI uses bounded/truncated/preformatted display and remains responsive |
| admin changes upload policy | PATCH sends only `uploadPolicy` fields through existing API client with CSRF support |
| deferred settings are requested by product copy or old docs | task stops and reports conflict instead of adding fake controls |

### 必须新增或更新的回归测试

- API wrapper tests for usage summary, usage records, operation logs, API call logs list/detail, and GET/PATCH system settings URLs/query serialization.
- Permission-gating tests proving users without `usage:read`, `audit:read`, or `system:settings:manage` do not see or trigger the corresponding admin UI/API calls.
- UI tests for loading, empty, error, and paginated list states.
- Settings form tests proving only `uploadPolicy` fields are rendered and PATCHed.
- Regression test proving deferred settings such as default Provider/model, concurrency, quota, and retention are absent from the UI.
- Regression test proving no Provider API key or auth token is written to browser storage by the new admin UI.

### 具体开发内容

- Add frontend API client functions for:
  - `GET /api/v1/admin/usage/summary`
  - `GET /api/v1/admin/usage/records`
  - `GET /api/v1/admin/operation-logs`
  - `GET /api/v1/admin/api-call-logs`
  - `GET /api/v1/admin/api-call-logs/:id`
  - `GET /api/v1/admin/system-settings`
  - `PATCH /api/v1/admin/system-settings`
- Add or extend frontend types for paginated admin read responses, usage summaries, usage records, operation logs, API call logs, redacted metadata, and system settings upload policy.
- Integrate an admin observability/settings view into the existing admin entry pattern without breaking `ProviderModelAdminPanel`.
- Use existing auth session permissions to decide which admin sections are visible or enabled.
- Use explicit user actions/page changes for list fetching; keep page size bounded.
- Render redacted JSON/metadata safely and compactly; avoid raw unbounded dumps.
- Implement settings update UX for the narrow upload policy only, including loading/success/error states.

### 安全要求

- Frontend must not store Provider API keys, Authorization headers, Cookies, auth tokens, log metadata, Provider errors, or system settings payloads in localStorage/sessionStorage/IndexedDB.
- Frontend must not call AI Providers, MinIO, or backend-internal service names directly.
- PATCH requests must go through the existing API client so credentials and CSRF behavior remain consistent.
- Permission checks are UI hygiene only; backend remains authoritative. Do not rely on frontend gating as security.
- Never add fake controls for deferred settings.

### 验收标准

- Admin users with matching permissions can view paginated usage/audit data and update upload-policy settings through backend APIs.
- Users lacking permissions do not see or trigger unauthorized admin sections.
- Provider/model management and P8 workbench behavior remain unchanged.
- UI and tests prove deferred settings are absent.
- No backend, deploy, docs, AGENTS, or agent-instruction files are modified.

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
git diff --check
```

## 子任务 30：安全回归与残余 legacy 清理

### 任务名称

P9-SECURITY-REGRESSION - 安全回归、残余 legacy 清理和发布前硬化

### 状态

Completed and merged into `main`. Review found no blockers. The merged changes added targeted regression coverage for Provider SSRF, redaction, tenant/object authorization, upload validation, task/SSE visibility, frontend production import safety, and removed the unreachable `LegacyHistoryItem`, `LegacyHistoryPanel`, and `useStorageUsage` helpers.

### 推荐执行信息

- 推荐线程名：`P9-SECURITY-REGRESSION`
- 推荐分支名：`codex/p9-security-regression`
- 起始分支：最新 `main`
- 开发顺序：串行执行。该任务完成、review、合并和回归后，再进入 `P9-DEPLOY-RELEASE-VALIDATION`；不要并行启动部署发布验证。

### 目标

补齐 SSRF、租户隔离、对象级权限、上传安全、敏感日志、SSE replay 可见性、生产 secret、前端静态安全和残余 legacy code 的回归测试与最小清理。此任务优先补测试、证明安全边界和清理明显不可达的遗留代码；不要把它扩大成新功能开发。

### 允许修改文件

- `backend/internal/**/**/*_test.go`
- `backend/internal/api/**` 仅限为新增安全回归测试做最小 bug fix；若需要改变公开合同，先停止报告
- `backend/internal/provider/**`、`backend/internal/provideradapter/**` 仅限 SSRF/redaction/security regression 的最小 bug fix
- `backend/internal/asset/**`、`backend/internal/storage/**` 仅限 upload/object-permission regression 的最小 bug fix
- `backend/internal/task/**`、`backend/internal/sse/**`、`backend/internal/queue/**` 仅限 task/SSE replay/security regression 的最小 bug fix，不改状态机语义
- `backend/internal/config/**`、`backend/cmd/**` 仅限 production secret guard regression 的最小 bug fix
- `frontend/src/test/**`
- `frontend/src/**/*.test.ts`
- `frontend/src/**/*.test.tsx`
- `frontend/src/db/**`、`frontend/src/lib/**` 仅限删除或 quarantine 已证明不在生产 import graph 的 legacy helper
- `frontend/src/components/**` 仅限删除或 quarantine 已证明不在生产 import graph 的 legacy display helper
- `frontend/src/vite-env.d.ts` 或测试 setup 文件，仅限测试需要

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `deploy/**`
- `frontend/src/hooks/useGeneration.ts`
- `frontend/src/api/tasks.ts`
- `frontend/src/components/studio/**`
- `frontend/src/components/admin/**`，除非新增安全回归测试证明已有管理端安全 bug 且修复很小
- Provider/model/task/asset public API response contracts，除非先报告合同冲突
- Worker claim/complete/cancel 状态机语义
- 任何新增 AI Provider browser direct call、Provider key browser persistence、task polling、unbounded admin log fetch、fake system setting、MinIO direct browser access

### 前置依赖

- `P9-FE-ADMIN-OBSERVABILITY-SETTINGS` completed and merged.
- R8 frontend backendization verification completed.
- Existing P5-P9 backend security foundations are merged: upload validation, Provider URL validation, runtime SSRF transport, recursive redaction, production secret guard, admin read APIs, and runtime-backed upload policy settings.

### 必须保持的现有行为

- Workbench generation continues to use backend task API plus SSE only.
- Frontend admin Provider/model and observability/settings UI remain usable and keep their no-secret-browser-persistence behavior.
- Existing auth, RBAC, tenant isolation, project/asset/task APIs, Provider/model management, task queue/worker, SSE replay, MinIO-backed asset download, and Docker Compose config behavior remain stable.
- Residual legacy helper cleanup must not remove code still imported by tests that protect migration invariants unless the test is updated to preserve the same invariant.

### 允许的中间态

- Security regression tests may be added before all release hardening is complete.
- Residual legacy files may be deleted if static import checks prove they are not in the production graph.
- If a legacy file is risky but cannot be safely deleted in this task, explicitly quarantine it with tests/comments that prevent production imports rather than partially rewiring behavior.
- If a new regression test exposes a broad security bug, commit the failing characterization only if useful, then stop and report the required follow-up scope.

### 禁止的半迁移状态

- No production frontend path may re-import legacy browser Provider adapters, local Provider settings, IndexedDB image/history primary source, or relay URLs.
- No backend security test may be weakened, skipped, or replaced with a snapshot that does not assert behavior.
- No secret may be added to fixtures, logs, docs, snapshots, or final output.
- No cross-tenant read/write behavior may be accepted as a test setup shortcut.
- No system settings field may become visible or writable without a runtime consumer.
- No task status polling may be introduced while adding tests or cleanup.

### 失败模式与边界场景

| Area | Scenario | Expected result |
| --- | --- | --- |
| Provider URL / SSRF | save/update/test/runtime URL points to localhost, loopback, private IP, link-local, Docker hostname, DNS rebinding, or redirect to blocked IP | request is rejected before Provider call or before following redirect |
| Provider redaction | API key appears as value, nested key, error body key, Authorization/Cookie, or base64-like image payload | persisted logs and admin read responses are redacted or dropped |
| Tenant isolation | tenant A probes tenant B project, asset, task, API call log, operation log, usage record, Provider, model, or settings object IDs | response is 403/404/no rows without existence leak |
| Object permissions | viewer/seller/admin attempts project, asset download/delete, task cancel/retry, Provider/model/settings/admin reads outside permission | backend rejects with expected status and no side effect |
| Upload validation | forged MIME, SVG, oversized bytes, over dimensions, over pixel count, tenant upload-policy override | backend rejects before MinIO persistence; valid allowed JPEG/PNG/WebP still works |
| SSE replay | Last-Event-ID before/after known sequence, cross-tenant task stream, heartbeat, reconnect replay | replay is ordered, scoped, heartbeat-safe, and no cross-tenant events leak |
| Production secrets | production startup uses placeholder JWT or API key encryption secret | API and Worker startup fail before serving work; non-production defaults remain usable |
| Frontend static safety | production frontend imports or contains Provider direct calls, API key persistence, task polling, relay proxy assumptions, MinIO direct URLs, deferred settings UI | static test fails |
| Residual legacy code | legacy display/DB helper is unreachable but confusing | delete it or quarantine it with a failing test if re-imported by production code |

### 必须新增或更新的回归测试

- Backend SSRF regression tests for Provider save/update/test and runtime transport edge cases not already covered.
- Backend tenant isolation/object-permission regression tests for the highest-risk object ID APIs not already covered.
- Backend upload validation regression tests for forged MIME/SVG/size/dimension/pixel/tenant policy edge cases.
- Backend SSE replay regression tests for Last-Event-ID ordering, heartbeat presence, reconnect replay, and cross-tenant rejection.
- Backend sensitive redaction regression tests for nested known-secret key/value and admin read response paths.
- Backend production secret guard regression tests must remain green for both API and Worker.
- Frontend static safety test scanning production imports/source for Provider direct calls, API key persistence, task polling, relay URLs, MinIO direct access, and deferred settings UI.
- Regression or import-graph test proving deleted/quarantined legacy helpers cannot re-enter the production path.

### 具体开发内容

- Inventory existing P5-P9 security tests and avoid duplicating already-covered cases.
- Add targeted tests for the gaps above, using existing test helpers and fixtures.
- Run frontend static scans/tests against production source, excluding test-only fixtures where appropriate.
- Delete or quarantine residual legacy display/DB helper files only after proving they are outside the production import graph.
- Apply only minimal code fixes needed for newly added tests when the fix stays within the allowed scope and does not change public contracts.
- If a fix requires broad runtime/API/state-machine changes, stop and report the exact failing test and proposed follow-up task.

### 安全要求

- Do not introduce real secrets in test data. Use obvious fake tokens such as `fake-secret-for-redaction-test`.
- Do not log full API keys, Authorization headers, Cookies, image base64, or raw upload bytes.
- Preserve tenant filtering on every business query touched by fixes.
- Preserve backend authorization as the source of truth; frontend checks are regression guards only.
- Keep Redis/MySQL/MinIO usage on shared local development services if an integration check is needed; do not create project-specific service containers for routine validation.

### 验收标准

- New regression tests cover the named failure modes or explicitly document why a case is already covered.
- No production frontend import path contains browser Provider direct calls, Provider key persistence, task polling, relay assumptions, MinIO direct access, or deferred settings UI.
- Residual legacy helper files are deleted or quarantined only when safe.
- Existing frontend, backend, task/SSE, Provider/model, admin, and asset flows remain green.
- No docs, deploy, AGENTS, or agent-instruction files are modified.

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ../frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ..
git diff --check
```

## 子任务 31：部署和发布验证

### 任务名称

P9-DEPLOY-RELEASE-VALIDATION - Docker Compose 全链路和发布文档

### 推荐执行信息

- 推荐线程名：`P9-DEPLOY-RELEASE-VALIDATION`
- 推荐分支名：`codex/p9-deploy-release-validation`
- 起始分支：最新 `main`
- 开发顺序：串行执行。该任务是 P9 最后一段发布验证；不要并行启动新的业务开发任务。

### 目标

执行 Docker Compose build/up/healthcheck，验证 API、worker、frontend、MySQL、Redis、MinIO 组合可运行，更新部署文档、环境变量说明、初始化管理员、数据卷、备份/恢复和安全注意事项。

### 允许修改文件

- `deploy/**`
- `backend/Dockerfile`
- `frontend/Dockerfile`
- `frontend/nginx.conf` 或 `frontend/nginx/**`
- `.env.example`
- `docs/deployment.md`
- `docs/development-plan.md` 仅限记录部署验证实际结果与遗留风险
- `docs/local-development.md` 仅限引用共享环境规则，不得复制真实本机凭据
- `docs/release-runbook.md` 或 `docs/operations-runbook.md` 如需要新增发布/运维说明
- `backend/internal/health/**`、`backend/cmd/**` 仅限修复 Compose health/readiness 所需的最小问题
- `frontend/**` 仅限修复 container build、Nginx `/api` proxy、static serving、SPA fallback 所需的最小问题

### 禁止修改文件

- `AGENTS.md`
- `agent-instructions/**`
- `docs/api-contract.md`
- `docs/sse-contract.md`
- `docs/rbac.md`
- `docs/provider-adapter.md`
- `docs/task-queue.md`
- `docs/storage.md`
- `docs/security.md`，除非发现 deployment-only 安全文档错误并先报告
- 后端业务 API、Provider Adapter runtime、task state machine、worker claim/complete/cancel 语义、RBAC 权限模型
- 前端工作台、Provider/model/admin UI、task/SSE client 业务行为，除非 container build 暴露编译错误且修复很小
- 任何真实本机或生产 secret、真实数据库密码、真实 MinIO key、真实 Provider API key

### 前置依赖

- `P9-SECURITY-REGRESSION` completed and merged.
- Frontend and backend local quality gates pass on `main`.
- P3 runtime Compose skeleton exists.
- `docs/local-development.md` remains the routine development environment reference.

### 必须保持的现有行为

- Routine feature validation still uses shared local services; Compose is used here only for deployment topology validation.
- Frontend container proxies `/api/` only to `backend-api:8080`; it must not proxy AI Providers.
- SSE paths must preserve streaming behavior and avoid proxy buffering.
- API and Worker must reject production placeholder secrets.
- MySQL remains the task/status source of truth, Redis remains queue/cache/lock/limit temporary infrastructure, MinIO stores images.
- Existing frontend/backend tests remain green.

### 允许的中间态

- Compose validation may use placeholder non-production secrets from `.env.example` only when `APP_ENV` is not production.
- If Compose cannot fully pass because a host dependency or port is unavailable, document the exact blocker, cleanup state, and minimal follow-up.
- Deployment docs may record manual bootstrap requirements such as MinIO bucket creation if the code intentionally does not create buckets.

### 禁止的半迁移状态

- Do not leave project-specific Compose containers, networks, or volumes running after validation unless the user explicitly asks to keep them.
- Do not change runtime code to bypass production secret guards just to make Compose easier.
- Do not add AI Provider relay routes to Nginx or frontend config.
- Do not commit generated local `.env` files, real credentials, database dumps, object data, or logs containing secrets.
- Do not mark deployment validation complete if health checks are failing or unverified.

### 失败模式与边界场景

| Area | Scenario | Expected result |
| --- | --- | --- |
| Compose config | `docker compose config` with `.env.example`-compatible placeholders | valid config with no unresolved variables needed for local deployment validation |
| Image build | backend API, backend worker, frontend images build from clean context | build succeeds without relying on host-only files |
| Startup health | `mysql`, `redis`, `minio`, `backend-api`, `backend-worker`, `frontend` start | services become healthy/running or blocker is documented precisely |
| API health | frontend/API published route and internal API route are checked | `/healthz` succeeds and does not expose secrets |
| Frontend proxy | `/api/` goes to backend-api and SSE path is not buffered | API requests work; SSE response keeps streaming headers |
| Forbidden proxy | OpenAI/Gemini/custom Provider paths are attempted or config-scanned | frontend does not proxy Provider traffic |
| Secrets | production placeholders with `APP_ENV=production` | API/Worker fail fast; docs warn operators to replace placeholders |
| MinIO buckets | bucket bootstrap is required | buckets are created by deployment setup or documented as a required preflight |
| Cleanup | validation ends | `docker compose down -v --remove-orphans` is run unless user asks to keep stack |

### 必须新增或更新的回归/验证材料

- Update deployment docs with exact commands run and observed results.
- Update `.env.example` comments/placeholders if required for current services, without real credentials.
- Add or update release runbook with initialization admin flow, MinIO bucket bootstrap, backup/restore outline, health checks, log/secret cautions, and cleanup commands.
- If Nginx/proxy config changes, add a static config check or documented manual verification showing `/api/` only proxies backend API and does not proxy AI Providers.

### 具体开发内容

- Inspect current `deploy/docker-compose.yml`, Dockerfiles, frontend Nginx config, `.env.example`, and health endpoints.
- Run Compose config/build/up/ps/log/health checks from repository root.
- Verify frontend static service, API `/healthz`, Worker process/readiness, MySQL, Redis, MinIO health, and `/api/` proxy path.
- Verify SSE route is not buffered by the frontend/reverse-proxy path where practical.
- Verify production placeholder secret guards still fail fast when `APP_ENV=production`.
- Update deployment documentation and release runbook with actual results and any remaining operational notes.
- Clean up Compose stack and volumes after validation unless the user explicitly asks to keep it.

### 安全要求

- Never write real secrets to repo files, logs, docs, screenshots, or final output.
- Use placeholder values only in `.env.example`.
- Do not weaken production secret guard, SSRF protections, upload validation, tenant isolation, RBAC, task/SSE security, or frontend no-provider-direct-call guarantees.
- If logs are quoted in handoff, redact credentials, cookies, Authorization headers, and Provider/API keys.

### 验收标准

- Compose config/build/up/health validation is executed and documented.
- Frontend, backend API, backend worker, MySQL, Redis, and MinIO are validated in the Compose topology.
- `/api/` proxy works and AI Provider proxying remains absent.
- Release/deployment docs explain required env vars, startup, initialization admin flow, MinIO bucket bootstrap, backup/restore basics, health checks, and cleanup.
- Project Compose stack is cleaned up after validation unless explicitly kept.
- Existing frontend/backend quality gates remain green.

### 测试命令

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ../frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ..
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml logs --tail=120 backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
git diff --check
```

## 子任务 32：P10 Worker 并发池

### 任务名称

P10-BE-WORKER-POOL - 让 Worker 并发配置成为真实处理池

### 推荐执行信息

- 推荐线程名：`P10-BE-WORKER-POOL`
- 推荐分支名：`codex/p10-backend-worker-pool`
- 起始分支：最新 `main`
- 开发顺序：completed and merged. 该任务触碰 Worker runtime，不要与 SSE lifecycle、Provider/model lifecycle 或前端 admin hardening 并行。

### 目标

把 Worker 从单个处理 loop 升级为可配置的 worker pool，使一个 Worker 进程可以按 `WORKER_CONCURRENCY` 并行 claim/process 任务，同时保持 Redis 队列语义、MySQL 状态权威、任务幂等、取消/重试/超时、dead-letter、输出资产去重和全局/租户/用户/Provider/模型并发限制。

### 允许修改文件

- `backend/internal/task/**`
- `backend/cmd/worker/**`
- `backend/internal/config/**`
- `backend/internal/queue/**` 仅限测试 helper 或为 Worker pool 暴露必要的窄接口；不得重写 reliable queue 语义
- `.env.example`
- `deploy/docker-compose.yml` 仅限传递 `WORKER_CONCURRENCY`

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `backend/internal/api/**`
- `backend/internal/provideradapter/**`
- `backend/internal/provider/**`
- `backend/internal/asset/**`
- `backend/internal/storage/**`
- `backend/internal/sse/**`
- Provider request/response contracts
- Task event names, status names, SSE replay cursor semantics
- Redis reliable queue payload contract, unless先报告主 agent

### 前置依赖

- `P9-DEPLOY-RELEASE-VALIDATION` completed, reviewed, and merged into `main`.
- `R9` completed with no blocking findings, and P10 is cleared to start from latest `main`.
- Existing Worker reliable queue, Provider runtime execution, MinIO output persistence, usage/API call logs, and SSE wakeup paths are already merged.
- Current known carry-forward risk: Worker process still runs one processing loop even though operator-facing concurrency settings exist.

### 必须保持的现有行为

- Queue payload remains task ID only.
- Worker reloads every task from MySQL before state transition and does not trust Redis payload data beyond task ID.
- MySQL remains the final task status and task event source of truth.
- Redis reliable queue claim/ack/retry/dead-letter/stale recovery behavior remains compatible with existing tests.
- Existing global/tenant/user/Provider/model concurrency limiters still gate actual task execution.
- Cancellation, retry, timeout, stale claim recovery, dead-letter marking, duplicate delivery idempotency, output asset de-duplication, usage record de-duplication, API call logging, and terminal event de-duplication must remain intact.
- `WORKER_CONCURRENCY=1` or missing config must preserve current single-loop effective behavior.

### 允许的中间态

- A Worker process may use one recovery goroutine/ticker plus N processing goroutines.
- Recovery may remain single-owner per process; it does not need to run once per worker goroutine.
- The task can add `WORKER_CONCURRENCY` config parsing and Compose/env documentation through `.env.example` and `deploy/docker-compose.yml`.
- If a worker goroutine sees transient claim/process/finalization errors, it may log and continue as the current loop does, provided cancellation and unrecoverable setup errors still stop cleanly.

### 禁止的半迁移状态

- Do not create N Worker processes in Compose as a substitute for an in-process worker pool.
- Do not conflate `WORKER_CONCURRENCY` with global/tenant/user/Provider/model concurrency limits.
- Do not let concurrent loops run duplicate recovery transitions or duplicate timeout work.
- Do not ack/retry/dead-letter a claim from more than one goroutine.
- Do not let context cancellation convert an in-flight cancelled Provider call into a successful task.
- Do not create duplicate output assets, `task_outputs`, `usage_records`, `api_call_logs`, or terminal task events under duplicate delivery or parallel execution.
- Do not change frontend task behavior, SSE client behavior, Provider Adapter runtime contracts, or admin UI.

### 失败模式与边界场景

| Area | Scenario | Expected result |
| --- | --- | --- |
| Config | `WORKER_CONCURRENCY` missing, `1`, `2+`, `0`, negative, or non-integer | missing/`1` preserves single-loop behavior; positive values configure pool size; invalid values fail config load |
| Pool startup | Worker starts with concurrency N | exactly N processing loops can claim/process work; recovery remains single-owner |
| Parallel processing | two or more queued tasks are available and limits permit | tasks can process concurrently inside one Worker process |
| Global limit | pool size exceeds `TASK_GLOBAL_CONCURRENCY` | limiter prevents more than configured active executions |
| Dimension limits | tenant/user/Provider/model limits are lower than pool size | limiter enforces those dimensions without losing claims |
| Duplicate delivery | same task is claimed again after completion or during stale recovery | no duplicate outputs, usage, API call logs, or terminal events |
| Cancellation | context cancellation or user cancellation occurs while tasks are in flight | Worker exits cleanly; cancelled tasks do not become `SUCCEEDED` because of late output |
| Retry/failure | processor returns retryable error in one goroutine | only that claim is retried/failed; other goroutines continue |
| Dead-letter | queue returns `ErrDeadLettered` for a claim | task is marked dead-lettered once; other goroutines continue |
| Shutdown | parent context is cancelled | all worker goroutines stop, ready file cleanup in `cmd/worker` still happens, and `Run` returns `context.Canceled` or equivalent cancellation error |
| Finalization failure | ack/retry/dead-letter finalization fails | existing queue-failure handling remains intact and does not crash unrelated workers |

### 必须新增或更新的回归测试

- Config tests for valid and invalid `WORKER_CONCURRENCY`.
- Worker unit test proving configured pool size can process multiple claims concurrently.
- Worker test proving recovery runs once per process, not once per processing goroutine.
- Worker test proving shutdown cancels all processing loops without goroutine leaks or hung `Run`.
- Worker/processor regression proving duplicate delivery under parallel execution does not duplicate outputs/events/usage/API-call records.
- Existing cancellation/timeout/retry/dead-letter tests must remain green.

### 具体开发内容

- Add `Worker.Concurrency` config parsing if it does not already exist, using `WORKER_CONCURRENCY` as the env var.
- Pass configured worker concurrency from `backend/cmd/worker/main.go` into `task.NewWorker`.
- Extend `task.WorkerOptions` with a bounded positive concurrency setting.
- Refactor `Worker.Run` to coordinate N processing loops plus one recovery loop under the parent context.
- Keep claim processing and `applyResult` behavior scoped to the goroutine that owns the claim.
- Ensure logs include enough worker-loop identity to debug concurrent execution without logging secrets.
- Update `.env.example` and `deploy/docker-compose.yml` to expose `WORKER_CONCURRENCY` as Worker process loop count.
- Add focused tests before broad regression.

### 安全要求

- Do not log API keys, Authorization headers, Cookies, Provider raw responses, image base64, or raw image bytes.
- Do not weaken tenant isolation, object authorization, Provider SSRF, upload validation, recursive redaction, task event visibility, or production secret guards.
- Do not bypass Redis concurrency limits or MySQL task-state checks for speed.
- Treat all Redis queue payload data except task ID as untrusted.

### 验收标准

- `WORKER_CONCURRENCY` is documented in `.env.example` and passed by Compose to `backend-worker`.
- Worker process can process multiple tasks concurrently when configured and when runtime limits allow.
- Existing queue, task, Provider runtime, SSE wakeup, and persistence behavior remains compatible.
- Recovery remains single-owner per process or explicitly guarded.
- Tests cover the named failure modes or explicitly defer cases outside scope with a precise reason.
- No docs or frontend files are modified by the child agent.

### 测试命令

```bash
cd backend
go test ./internal/config ./internal/task ./cmd/worker -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
git diff --check
```

### 完成状态

- `P10-BE-WORKER-POOL` completed, reviewed, and merged into `main`.
- Worker now runs one recovery loop plus `WORKER_CONCURRENCY` processing loops inside a single Worker process.
- `WORKER_CONCURRENCY` is parsed by backend config, documented in `.env.example`, and passed to `backend-worker` by Compose.
- Tests cover valid/invalid config, pool concurrency, global limiter below pool size, single-owner recovery, shutdown cancellation, duplicate delivery de-duplication, and retry finalization failure isolation.

## 子任务 33：P10 SSE bridge 生命周期

### 任务名称

P10-BE-SSE-BRIDGE-LIFECYCLE - 绑定 API Redis 任务事件订阅到 API 生命周期

### 推荐执行信息

- 推荐线程名：`P10-BE-SSE-BRIDGE-LIFECYCLE`
- 推荐分支名：`codex/p10-backend-sse-bridge-lifecycle`
- 起始分支：最新 `main`
- 开发顺序：串行执行。该任务触碰 API startup/router 和 Redis task-event wakeup bridge，不要与 Provider/model lifecycle、frontend admin hardening 或 history query 并行。

### 目标

把 API 进程中的 Redis task-event subscriber 从 `context.Background()` 改为绑定 API server lifecycle。API 收到 shutdown 信号后，Redis wakeup subscriber 必须随 API shutdown context 停止并关闭 pub/sub，不留下长期 goroutine，同时保持 SSE replay、heartbeat、`Last-Event-ID`、MySQL replay source 和 Redis wakeup-only 语义不变。

### 允许修改文件

- `backend/cmd/api/**`
- `backend/internal/api/router.go`
- `backend/internal/api/*_test.go` 仅限 router/API startup/SSE bridge lifecycle 测试
- `backend/internal/queue/task_event_wakeup.go`
- `backend/internal/queue/task_event_wakeup_test.go`
- `backend/internal/queue/**` 仅限 task-event wakeup subscriber 接口/测试 helper 的窄改动

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `backend/internal/task/**`
- `backend/internal/sse/**`，除非先报告主 agent 并说明为什么 lifecycle 无法只在 API/queue 边界解决
- `backend/internal/provideradapter/**`
- `backend/internal/provider/**`
- `backend/internal/asset/**`
- `backend/internal/storage/**`
- `deploy/**`
- `.env.example`
- Task event names, task statuses, SSE frame format, `Last-Event-ID` parsing, replay cursor semantics, Redis wakeup payload schema, and frontend EventSource behavior

### 前置依赖

- `P10-BE-WORKER-POOL` completed, reviewed, and merged into `main`.
- Existing P7/P8/P9 SSE contracts are stable: MySQL is replay source, Redis pub/sub is wakeup-only, frontend consumes EventSource/SSE only.
- Current known carry-forward risk: `backend/internal/api/router.go` starts Redis task-event subscriber with `context.Background()`, so it is not tied to API server shutdown.

### 必须保持的现有行为

- SSE replay continues to read `task_events` from MySQL and filter by tenant/project/task visibility.
- `Last-Event-ID` header and `lastEventId` query fallback keep the same behavior.
- Heartbeat frames keep the same behavior and must not contain task metadata.
- Redis task-event wakeup payload remains sequence-only and must not include tenant, task, project, event payload, Authorization, Cookie, API key, or base64 data.
- Redis pub/sub remains an acceleration/wakeup mechanism only; it must not become the event source of truth.
- API task service still publishes both in-process broker events and Redis wakeups when Redis bridge is enabled.
- Tests using `APP_ENV=test` or explicit router options must not unexpectedly start real Redis subscribers.
- Existing task/SSE route tests and queue wakeup tests must remain green.

### 允许的中间态

- `RouterOptions` may gain a lifecycle context and/or injectable task-event subscriber for tests.
- `queue.StartTaskEventSubscriber` may return a done channel or cancellation result if useful for tests and shutdown observation.
- API `main` may create the signal context before router construction so the router can use that context for background bridge work.
- Test-only fake subscribers may be introduced inside test files.

### 禁止的半迁移状态

- Do not continue using `context.Background()` in the production API path that starts the Redis task-event subscriber.
- Do not remove Redis wakeups or make live SSE depend only on in-process broker.
- Do not make SSE handlers read directly from Redis pub/sub as a source of event details.
- Do not alter SSE event IDs, event names, payload shape, heartbeat cadence contract, or replay ordering.
- Do not leak Redis subscriber errors or internal shutdown details to API clients.
- Do not leave goroutines running after router/API lifecycle tests cancel their context.
- Do not introduce real Redis/MySQL/MinIO dependency into unit tests that can use fakes.

### 失败模式与边界场景

| Area | Scenario | Expected result |
| --- | --- | --- |
| API startup | Redis task-event bridge enabled outside test env | subscriber starts with API lifecycle context, not `context.Background()` |
| API shutdown | lifecycle context is cancelled | subscriber exits and closes pub/sub path cleanly |
| Router tests | router is built in test env | no real Redis subscriber starts unless explicitly injected/enabled |
| Subscriber error | subscriber returns an error before context cancellation | error is logged without crashing router construction |
| Context cancellation | subscriber returns `context.Canceled` after lifecycle cancellation | no warning/error log is emitted as an unexpected failure |
| Wakeup payload | Redis wakeup contains only `sequence` | no tenant/task/project/payload/secret/base64 fields are published |
| Malformed wakeup | subscriber receives invalid JSON or zero sequence | message is ignored and subscriber continues |
| SSE replay | live wakeup publishes sequence only | SSE service reloads visible events from MySQL and keeps tenant/object filtering |
| Contract preservation | existing SSE route tests run | `Last-Event-ID`, heartbeat, replay ordering, and visibility behavior remain unchanged |

### 必须新增或更新的回归测试

- API/router or cmd/api test proving production bridge startup receives a cancellable lifecycle context instead of an unbounded background context.
- Test proving lifecycle cancellation stops the task-event subscriber and does not log an unexpected error for `context.Canceled`.
- Test proving router construction in test env does not start a real Redis subscriber by default.
- Queue wakeup tests must continue to prove sequence-only payload, malformed payload ignore, zero sequence ignore, and channel naming.
- Existing SSE route tests must remain green.

### 具体开发内容

- Inspect `backend/internal/api/router.go`, `backend/cmd/api/main.go`, and `backend/internal/queue/task_event_wakeup.go`.
- Introduce the smallest lifecycle hook needed for API startup to pass its signal/shutdown context into Redis task-event subscriber startup.
- If needed, define a narrow subscriber interface in `queue` so tests can inject a fake subscriber without Redis.
- Ensure `StartTaskEventSubscriber` or equivalent startup helper has observable cancellation behavior in tests.
- Move API signal context creation before router construction if required, and pass it through `newRouter`/`RouterOptions`.
- Preserve `shouldStartRedisTaskEventBridge` behavior unless a test-only override is needed.
- Do not change task service event publishing semantics.
- Add focused tests before running broad regression.

### 安全要求

- Do not log Authorization headers, Cookies, API keys, Provider raw responses, image base64, or raw event payload JSON.
- Redis wakeup payload must remain sequence-only.
- Do not weaken tenant isolation, object authorization, SSE visibility filtering, CSRF/auth middleware, Provider SSRF, upload validation, recursive redaction, or production secret guards.
- Treat Redis pub/sub messages as untrusted input and ignore malformed data.

### 验收标准

- Production API path no longer starts the Redis task-event subscriber with `context.Background()`.
- API lifecycle cancellation stops the Redis task-event subscriber.
- Router/API tests can prove no background subscriber leak remains.
- Existing SSE replay and heartbeat behavior is unchanged.
- Redis wakeup payload remains sequence-only.
- No frontend, docs, deploy, Provider, asset, storage, or task runtime files are modified.

### 测试命令

```bash
cd backend
go test ./internal/queue ./internal/api ./cmd/api -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
git diff --check
```


## 子 agent 交付格式

每个子 agent 最终回复必须包含：

- 修改文件清单。
- 未修改但依赖的公共合同文件清单。
- 执行的测试命令和结果。
- 安全约束自查结果。
- 遇到的合同缺口或需要主 agent 决策的问题。

子 agent 不得自行修订公共合同文件。
