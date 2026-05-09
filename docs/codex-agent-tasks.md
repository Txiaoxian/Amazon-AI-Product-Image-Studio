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
- `docs/development-plan.md`
- `docs/codex-agent-tasks.md`

子 agent 如果发现合同缺失、冲突或不可实现，只能在最终回复中报告问题，不能直接修改上述文件。

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

- 确认前端无 token 读取和 API Key 持久化。
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

## 后续批次建议

R2 之后再按两到三个 worktree 的上限推进，每一批完成后都由主 agent 串行 review 和集成。

建议顺序：

1. 后端数据库迁移与 tenant-aware repository helper。
2. 认证、初始化管理员、HttpOnly Cookie 和当前用户 API。
3. RBAC 和项目成员对象级权限。
4. 项目与资产 API、MinIO 上传下载和缩略图。
5. Provider/model 管理、API Key 加密、SSRF 防护。
6. Redis 队列、Worker、任务状态机和并发限制。
7. SSE 服务端、历史补发和前端任务状态接入。
8. 前端工作台后端化，移除 AI 直连和 API Key 设置入口。
9. 用量统计、API 调用日志、操作日志和系统设置。
10. 安全回归、Docker Compose 全链路验证和发布文档。

每个后续任务都必须沿用本文的字段模板：任务名称、目标、允许修改文件、禁止修改文件、前置依赖、具体开发内容、安全要求、验收标准、测试命令。

## 子 agent 交付格式

每个子 agent 最终回复必须包含：

- 修改文件清单。
- 未修改但依赖的公共合同文件清单。
- 执行的测试命令和结果。
- 安全约束自查结果。
- 遇到的合同缺口或需要主 agent 决策的问题。

子 agent 不得自行修订公共合同文件。
