# Codex Agent 任务计划

## 用途

本文档是当前 worktree 任务规划入口。P0-P10 的历史细节已刻意压缩，因为完整任务历史保留在 git 中。后续 agent 生成新任务包时，应以本文档和 `docs/development-plan.md` 为准，不要继续复制旧阶段的大段历史文本。

## 调度模型

本项目采用：

1. 主 agent 冻结或更新公共合同。
2. 少量子 agent worktree 实现彼此独立的开发切片。
3. 主 agent review、合并、运行回归，并更新文档。

进入新阶段时，应先串行推进，直到共享后端/API 合同稳定。只有当写入范围和运行时合同互不重叠时，才允许有限并行。

## 只能由主 Agent 修改的文件

子 agent 不得修改：

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

如果子 agent 发现合同缺口或矛盾，只能在最终交付中报告，不能直接修改公共合同文件。

## 任务包必填字段

从 P11 开始，每个新的 worktree 任务包必须包含：

- 任务名称
- 目标
- 推荐线程名
- 推荐分支名
- 起始分支
- 子 agent 完整启动 prompt
- 允许修改文件
- 禁止修改文件
- 前置依赖
- 具体开发内容
- 安全要求
- 验收标准
- 测试命令
- 必须保持的现有行为
- 允许的中间态
- 禁止的半迁移状态
- 失败模式与边界场景
- 必须新增或更新的回归测试

高风险后端任务必须包含失败模式矩阵。迁移类任务必须明确说明旧路径、允许的中间态和目标路径。

## 子 Agent 交付要求

子 agent 的最终回复必须包含：

- 修改文件清单
- 执行的测试命令和结果
- 必须回归场景到具体测试文件/测试名的映射
- 安全自查结果
- 刻意未修改的范围
- 需要主 agent 决策的合同缺口或问题

如果任务无法在不破坏现有行为或不扩大禁止范围的前提下完成，子 agent 必须停止并报告冲突。

## 本地环境验证授权

- 后续开发任务默认可以使用 `docs/local-development.md` 中记录的共享本地 MySQL、Redis、MinIO 环境做功能验证。
- 用户已授权 agent 在共享本地环境中对任务自有测试数据执行增删改查、入队、出队、上传、下载和清理操作。
- 测试数据必须使用容易识别的前缀或命名，例如 `codex_`、阶段名、任务名或分支名。
- 不得删除无关数据，不得 drop 项目数据库，不得删除共享 MinIO bucket，不得对共享 Redis 执行 `FLUSHALL` / `FLUSHDB`，除非用户明确要求。
- 不得把本地真实密码、Access Key、Secret Key 写入仓库、日志、测试快照或最终交付。
- 如果任务使用了共享本地服务，最终交付必须说明创建/修改了哪些类别的测试数据，以及是否已清理。

## 当前状态

R12 已完成，未发现阻塞问题。`P13-BE-RUNTIME-DEFAULTS` 已 review 并合并，P13 仍在进行中。

已完成的平台基础：

- P0-P3：文档、monorepo 布局、后端/前端/部署骨架、Docker 运行时修复。
- P4-P6：数据库、租户、认证、RBAC、项目、资产、Provider/模型管理。
- P7-P8：任务队列、Worker、Provider Adapter 运行时、SSE、前端后端化。
- P9-P10：审计/用量读取、运行时生效的上传策略、生产密钥保护、安全/部署验证、Worker 并发池、SSE bridge 生命周期、Provider/模型生命周期、admin 硬化、后端统一历史查询。
- P11-P12：用户/角色管理、统一历史、卖家项目/资产流程和项目最后 `OWNER` 约束。
- P13 已合并切片：租户 `taskDefaults.{defaultProviderId,defaultModelId}` 的读写与任务创建真实消费路径。

当前已知后续项：

- 前端历史已改为消费 `GET /api/v1/projects/{projectId}/history`。
- 前端项目/资产工作流已完成一轮产品化打磨：项目编辑、资产筛选、资产元数据编辑、项目成员入口、上传切项目 stale 保护和筛选后列表一致性均已合并。
- 后端项目成员写路径已补齐最后一个 `OWNER` 保护：不能删除或降级项目最后一个 `OWNER`，但允许先新增或提升另一个 `OWNER` 后再完成 owner 转移。
- R12 已验证 P12 范围内的卖家工作流、统一历史、项目/资产 UI、项目成员 API、最后 `OWNER` 保护、操作日志、权限边界、前端禁止模式和 Compose 配置。
- 用户/角色管理后端接口和前端管理 UI 已完成；后续租户/团队更深层能力可在新的任务中继续补齐。
- 上传策略与 `taskDefaults` 已有真实运行时消费者；其他运行时设置在消费者落地前不得暴露为可写配置。
- `P13-BE-RUNTIME-DEFAULTS` review 的非阻塞遗留：手工或历史写入的非法 `task_defaults` 行在缺省任务创建路径需要统一 fail closed 为 `422`，且不能产生任务、队列或成功审计副作用。
- 存储清理、保留周期、配额、缩略图策略和 orphan cleanup 仍需实现。
- Provider/模型并发管理操作可能需要更强的事务序列化。
- 最终 E2E 和发布验证仍需完整 seller flow 回归。

## 建议剩余阶段

### P11：身份、团队与 RBAC 管理

目标：让用户、角色、权限和租户/团队管理真正可用。

建议任务：

1. `P11-BE-USER-ROLE-ADMIN`
   - 后端用户管理、禁用/启用、角色分配、角色/权限读取。
   - 已完成并合并。后端合同已稳定，可供前端接入。
2. `P11-FE-USER-ROLE-ADMIN`
   - 前端用户与角色管理 UI。
   - 已完成并合并。前端按 `user:*` / `role:*` 权限展示入口、加载数据和开放写操作；创建用户密码只作为瞬时表单输入；禁用/启用和角色分配走后端 CSRF 写接口。
3. `R11`
   - 已完成整批 review 和回归，未发现阻塞问题。
   - 已通过前端 lint/type-check/test/build、后端 test/race/vet/build、Compose config、whitespace 检查和 P11 敏感模式扫描。

### P12：卖家工作流与历史记录补齐

目标：补齐日常卖家使用所需的项目、历史和资产工作流。

建议任务：

1. `P12-FE-UNIFIED-HISTORY`
   - 将前端历史切换到后端统一历史接口。
   - 已完成并合并。历史列表不再由浏览器 join tasks/assets 生成。
2. `P12-FE-PROJECT-WORKFLOW-POLISH`
   - 改进项目选择、编辑、成员入口和资产管理体验。
   - 已完成并合并。前端项目/资产工作流已接入真实后端项目成员 API，并修复项目切换与筛选状态一致性问题。
3. `P12-BE-PROJECT-MEMBER-HARDENING`
   - 根据产品规则补齐项目成员约束，例如最后一个 `OWNER` 保护。
   - 已完成并合并。后端 member update/delete 路径会保留至少一个项目 `OWNER`，并验证被拒绝的写入不会记录成功 operation log。
4. `R12`
   - 卖家工作流 review 和回归。
   - 已完成。未发现阻塞问题；通过前端 lint/type-check/test/build、后端 test/race/vet/build、Compose config、whitespace 检查和前端禁止模式扫描。

### P13：运行时设置、配额与存储生命周期

目标：只暴露有真实运行时消费者的设置，并让存储生命周期具备可运维能力。

建议任务：

1. `P13-BE-RUNTIME-DEFAULTS`
   - 已完成并合并。`taskDefaults.{defaultProviderId,defaultModelId}` 已通过系统设置 API 读写；任务创建仅在两个 ID 同时省略时解析默认配置，且继续执行现有 Provider/模型/能力/资产校验。
2. `P13-BE-RUNTIME-DEFAULTS-HARDENING`
   - 关闭 review 发现的非法持久化默认配置错误映射缺口：缺省请求安全返回 `422` 且无副作用，显式 Provider/模型请求不受未使用的损坏默认配置影响。
3. `P13-BE-CONCURRENCY-POLICY`
   - 将租户/用户/Provider/模型并发设置加载到 Worker 限流器。
4. `P13-BE-STORAGE-QUOTA-RETENTION`
   - 存储配额、保留周期、orphan cleanup 和独立 cleanup context/job。
5. `P13-FE-SYSTEM-SETTINGS`
   - 仅为已经运行时生效的设置提供前端 admin UI。
6. `R13`
   - 设置和存储生命周期 review。

### P14：Provider、模型、用量与成本运营

目标：强化运营数据完整性，并让用量/成本统计对实际运营有用。

建议任务：

1. `P14-BE-PROVIDER-MODEL-INTEGRITY`
   - 强化 Provider 删除与模型创建/更新之间的事务序列化；按需要决定是否实现模型名称唯一约束。
2. `P14-BE-USAGE-COST-REPORTING`
   - 改进按租户、用户、项目、Provider、模型聚合的用量和成本统计。
3. `P14-FE-COST-OBSERVABILITY`
   - 前端成本/用量看板和明细钻取。
4. `R14`
   - Provider 生命周期和成本统计 review。

### P15：发布硬化与端到端 QA

目标：证明完整平台可部署、可使用、可运维。

建议任务：

1. `P15-E2E-CORE-FLOWS`
   - 覆盖核心卖家和 admin 端到端流程。
2. `P15-SECURITY-FINAL-REGRESSION`
   - 最终禁止模式扫描和安全回归套件。
3. `P15-DEPLOY-RUNBOOK-FINAL`
   - Compose 发布验证、运维手册、备份/恢复说明和健康检查最终化。
4. `R15`
   - 最终发布就绪 review。

## 下一个任务包：P13-BE-RUNTIME-DEFAULTS-HARDENING

### 调度决策

- 本任务串行执行，不与并发政策、存储生命周期或前端设置任务并行。
- 理由：它修复已合并运行时设置路径的错误合同边界；继续增加设置字段前必须先保证当前活动设置 fail closed。
- 本任务不增加任何新的可写系统设置字段，也不更改公共 API 成功响应结构。

### 任务信息

- 任务名称：`P13-BE-RUNTIME-DEFAULTS-HARDENING`
- 目标：将损坏的租户 `task_defaults` 持久化值在缺省任务创建路径中的表现固定为标准、无泄漏、无副作用的 `422 VALIDATION_ERROR`，同时证明显式 Provider/模型提交不读取未使用的损坏默认配置。
- 推荐线程名：`P13-BE-RUNTIME-DEFAULTS-HARDENING`
- 推荐分支名：`codex/p13-backend-runtime-defaults-hardening`
- 起始分支：包含 `P13-BE-RUNTIME-DEFAULTS` 与本任务公共合同文档更新的最新 `main`
- 前置依赖：`P13-BE-RUNTIME-DEFAULTS` 已合并；`docs/api-contract.md` 和 `docs/database-schema.md` 已冻结非法持久化默认配置的 fail-closed 合同。

### 允许修改文件

- `backend/internal/task/**`
- `backend/internal/settings/**`，仅在需要统一或暴露可判定的验证错误类型时修改
- `backend/internal/api/*task*_test.go`
- `backend/internal/api/*system_settings*_test.go`，仅当需要证明原设置 API 错误行为未退化

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/internal/provider/**`
- `backend/internal/provideradapter/**`
- `backend/internal/model/**`
- `backend/internal/queue/**`
- `backend/internal/sse/**`
- `backend/internal/asset/**`
- `backend/internal/database/**`
- 任何新的设置字段、迁移、Worker 限流逻辑或无关重构

### 具体开发内容

1. 先新增能够复现 review 问题的 API 回归测试，再做最小修复。
2. 在任务创建解析 `taskDefaults` 的边界，将 `settings` 包报告的已存储默认配置验证失败映射为任务 API 的标准验证失败，不向客户端暴露内部存储内容。
3. 覆盖非法 JSON、只有一个 ID、含未知字段或空 ID 等损坏存储值；这些行只能通过测试种子或模拟历史脏数据建立，不得放宽设置写接口。
4. 证明请求显式提供合法 `providerId` 与 `modelId` 时不读取 `task_defaults`，即使数据库内默认配置损坏仍能沿原路径创建任务。
5. 保持现有合法默认配置、clear、禁用/删除/跨租户/能力不兼容失败路径不变。

### 必须保持的现有行为

- 合法 `taskDefaults` 仅在任务请求同时省略 `providerId` 与 `modelId` 时生效。
- 显式提供成对 Provider/模型 ID 的任务创建语义不变。
- 只提供一个 ID、缺少默认值、默认对象失效或 capability 不匹配继续返回 `422` 且无任务创建副作用。
- 系统设置写接口继续拒绝非成对、跨租户、禁用、归属错误或未知字段的默认值。
- Cookie、CSRF、tenant/RBAC、任务审计、Redis 队列、SSE 与 Provider Adapter 安全边界不得改变。

### 允许的中间态

- `taskDefaults` 已为活动、可写、运行时生效的设置项；前端尚未提供配置界面，可以继续仅由 API 管理。
- 并发、配额和保留周期字段继续不出现在活动可写设置响应或写入面。

### 禁止的半迁移状态

- 将损坏的默认配置当成服务器 `500` 返回给缺省任务请求，或将原始配置内容暴露给客户端/日志。
- 在验证失败后创建任务、事件、Redis 入队项或成功的 `task.create` 操作日志。
- 为避开错误而回退到任意 Provider/模型、跨租户对象或未校验 capability 的默认值。
- 让显式任务创建因未使用的损坏 `task_defaults` 行而失败。
- 扩展系统设置字段、前端 UI、迁移或 Worker 限流范围。

### 失败模式与边界场景

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| 两个 ID 均省略，`task_defaults.value_json` 为非法 JSON | `422 VALIDATION_ERROR`；无任务/事件/入队/成功操作日志 | 是 |
| 两个 ID 均省略，持久化值只有 Provider ID 或只有 Model ID | `422 VALIDATION_ERROR`；无副作用 | 是 |
| 两个 ID 均省略，持久化值含未知字段或空 ID | `422 VALIDATION_ERROR`；无副作用 | 是 |
| 两个 ID 均显式提供且合法，但存储默认值损坏 | 正常创建并入队；不依赖默认值 | 是 |
| 合法默认配置缺省创建 | 保持当前成功行为 | 回归现有测试 |
| absent/cleared/disabled/deleted/cross-tenant/capability-invalid 默认配置 | 保持当前 `422` 且无副作用行为 | 回归现有测试 |

### 安全要求

- 不解密、不读取、不记录 Provider API Key 或任何凭据。
- 错误响应与日志不得包含 `value_json` 原文、Provider/模型跨租户详情、Authorization、Cookie、token、base64 或内部错误栈。
- 所有默认设置读取和任务对象操作继续按 `tenant_id` 过滤。
- 不引入 frontend Provider 直连、轮询或任何浏览器存储行为。

### 必须新增或更新的回归测试

- 在 `backend/internal/api/*task*_test.go` 新增默认配置损坏矩阵测试，至少覆盖非法 JSON、单边 ID、未知字段或空 ID，并逐项断言 `422` 与无副作用。
- 新增显式成对 Provider/模型 ID 在损坏默认行存在时仍成功创建并入队的测试。
- 保留并运行已有合法默认值、clear、禁用/删除/跨租户/能力不兼容、CSRF/RBAC 和操作日志脱敏测试。

### 验收标准

- 损坏的存储默认配置不再导致缺省任务创建返回 `500`。
- 缺省失败路径统一返回标准 `422 VALIDATION_ERROR`，不创建任务、事件、队列项或成功操作日志。
- 显式 Provider/模型任务创建不受损坏但未使用的默认设置影响。
- 未新增可写设置字段、迁移、前端/部署代码或敏感数据暴露。

### 测试命令

```bash
cd backend
go test ./internal/api ./internal/task ./internal/settings -count=1
go test ./... -count=1
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
git diff --check main...HEAD
```

如使用共享本地 MySQL、Redis 或 MinIO 做额外验证，必须按 `docs/local-development.md` 使用任务命名空间数据，并在交付中记录清理结果；本任务不得另起项目专属依赖容器。

## 标准验证命令

前端任务：

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
```

后端任务：

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker
```

部署配置：

```bash
docker compose -f deploy/docker-compose.yml config
```

部署运行时检查可以使用 `deploy/docker-compose.yml`，但必须在验证后清理：

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```
