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

R12 已完成，未发现阻塞问题。`P13-BE-RUNTIME-DEFAULTS` 与 `P13-BE-RUNTIME-DEFAULTS-HARDENING` 已 review 并合并，P13 仍在进行中。

已完成的平台基础：

- P0-P3：文档、monorepo 布局、后端/前端/部署骨架、Docker 运行时修复。
- P4-P6：数据库、租户、认证、RBAC、项目、资产、Provider/模型管理。
- P7-P8：任务队列、Worker、Provider Adapter 运行时、SSE、前端后端化。
- P9-P10：审计/用量读取、运行时生效的上传策略、生产密钥保护、安全/部署验证、Worker 并发池、SSE bridge 生命周期、Provider/模型生命周期、admin 硬化、后端统一历史查询。
- P11-P12：用户/角色管理、统一历史、卖家项目/资产流程和项目最后 `OWNER` 约束。
- P13 已合并切片：租户 `taskDefaults.{defaultProviderId,defaultModelId}` 的读写、任务创建真实消费路径，以及损坏持久化默认值的 fail-closed hardening。

当前已知后续项：

- 前端历史已改为消费 `GET /api/v1/projects/{projectId}/history`。
- 前端项目/资产工作流已完成一轮产品化打磨：项目编辑、资产筛选、资产元数据编辑、项目成员入口、上传切项目 stale 保护和筛选后列表一致性均已合并。
- 后端项目成员写路径已补齐最后一个 `OWNER` 保护：不能删除或降级项目最后一个 `OWNER`，但允许先新增或提升另一个 `OWNER` 后再完成 owner 转移。
- R12 已验证 P12 范围内的卖家工作流、统一历史、项目/资产 UI、项目成员 API、最后 `OWNER` 保护、操作日志、权限边界、前端禁止模式和 Compose 配置。
- 用户/角色管理后端接口和前端管理 UI 已完成；后续租户/团队更深层能力可在新的任务中继续补齐。
- 上传策略与 `taskDefaults` 已有真实运行时消费者；损坏的 `task_defaults` 行已在缺省创建路径 fail closed 为无副作用 `422`。其他运行时设置在消费者落地前不得暴露为可写配置。
- 下一串行任务仅开放 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`，并必须在同一任务中让 Worker Redis semaphore 消费该策略；环境限制继续作为硬上限与缺省值。
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
   - 已完成并合并。非法持久化默认配置在缺省创建路径安全返回 `422` 且无副作用，显式 Provider/模型请求不受未使用的损坏默认配置影响。
3. `P13-BE-CONCURRENCY-POLICY`
   - 在同一任务中增加 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}` 设置切片，并将其加载到 Worker Redis semaphore；不开放租户级 global 字段。
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

## 下一个任务包：P13-BE-CONCURRENCY-POLICY

### 调度决策

- 本任务串行执行，不与存储生命周期或前端设置任务并行。
- 理由：本任务同时修改系统设置公共响应与 Worker 运行时限流语义；设置字段只有在 Worker 同步消费后才是真实可写状态。
- 本任务只增加 `taskConcurrency` 设置切片，不开放全局并发配置，不改任务、SSE 或 Provider Adapter 的业务响应结构。

### 任务信息

- 任务名称：`P13-BE-CONCURRENCY-POLICY`
- 目标：在同一个后端切片内新增租户 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}` 管理接口和 Worker Redis semaphore 消费路径，使租户管理员只能收紧任务执行并发限制，且任何损坏配置或读取故障都不能绕过限流。
- 推荐线程名：`P13-BE-CONCURRENCY-POLICY`
- 推荐分支名：`codex/p13-backend-concurrency-policy`
- 起始分支：已合并 `P13-BE-RUNTIME-DEFAULTS-HARDENING` 与本任务公共合同文档的最新 `main`
- 前置依赖：`P13-BE-RUNTIME-DEFAULTS-HARDENING` 已合并；`docs/api-contract.md`、`docs/database-schema.md` 与 `docs/task-queue.md` 已冻结 `taskConcurrency` 的字段、硬上限和 Worker 消费合同。

### 允许修改文件

- `backend/internal/settings/**`
- `backend/internal/task/worker.go`
- `backend/internal/task/worker_test.go`
- `backend/internal/api/router.go`，仅限向 settings service 传入已有 queue hard caps
- `backend/internal/api/*system_settings*_test.go`
- `backend/cmd/worker/main.go` 与 `backend/cmd/worker/*_test.go`，仅在 Worker policy resolver 注入所必需时修改

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
- `backend/internal/database/**` 与任何 migration
- `backend/internal/config/**`，现有环境 hard caps 已足够，不增加新的部署配置
- `backend/internal/task/service.go`、任务创建/SSE/Provider runtime/output persistence 主流程
- 存储配额、保留周期、全局并发可写字段或无关重构

### 具体开发内容

1. 先新增设置 API 与 Worker failure matrix 测试，再做最小实现。
2. 在已有 `system_settings` 泛型表中增加 key `task_concurrency`，并在系统设置响应/PATCH 中增加 `taskConcurrency`：
   - `tenantLimit`
   - `userLimit`
   - `providerLimit`
   - `modelLimit`
3. 所有字段均为正整数；PATCH 允许只更新部分字段，省略字段沿用当前 effective 值。没有 override 时，GET 返回现有 `config.QueueConfig` 中 tenant/user/provider/model 的环境缺省值。
4. tenant override 只能小于或等于对应环境 hard cap；不增加或暴露 `globalLimit`，不改变 `TASK_GLOBAL_CONCURRENCY` 的部署控制边界。
5. 将有效策略接入 Worker 的 semaphore acquire 路径：Worker 在已加载 tenant-scoped task execution context 后、获得 Redis lease 前读取策略，并使用 effective tenant/user/provider/model 限制。Provider 行已有的正值 `concurrencyLimit` 继续与 effective Provider limit 取更小值。
6. 对损坏的 `task_concurrency` 持久化内容做 fail closed：Worker 不调用 Provider、不写 output/usage/api-call 成功数据，而以现有通用任务配置失败语义终止该 eligible task；真实 settings 存储/数据库错误仅进入 retry 路径，不能绕过限制或误标成配置非法。
7. 系统设置更新继续写脱敏 operation log，仅记录 key、changed fields 和非敏感整数限制。

### 必须保持的现有行为

- `uploadPolicy` 与 `taskDefaults` 的 API、运行时消费及 hardening 语义不变。
- Worker 的全局/租户/用户/Provider/模型 Redis lease、释放、stale reap、retry 和 MySQL 状态权威语义不变。
- `TASK_GLOBAL_CONCURRENCY`、现有环境 tenant/user/provider/model limits 和 Provider `concurrencyLimit` 的现有保护不得被放宽。
- Cookie、CSRF、tenant/RBAC、任务审计、Redis payload、SSE、Provider Adapter、输出资产和敏感日志边界不得改变。

### 允许的中间态

- `taskConcurrency` 后端 API 和 Worker 消费在本任务一起生效；前端设置页尚不展示该切片，可以继续仅由 API 管理。
- 存储配额和保留周期字段继续不出现在活动可写设置响应或写入面。
- 设置变更仅影响后续新申请的 Redis lease；已有 lease 运行至释放或 TTL 清理，不强行中止正在执行的任务。

### 禁止的半迁移状态

- 暴露可写 `taskConcurrency` 字段但 Worker 仍只使用静态环境限制，或 Worker 消费未通过 API/tenant/RBAC 约束的第二配置源。
- 允许 tenant override 提高环境 hard cap，或允许租户写入 global limit。
- 存储策略损坏/读取失败时忽略设置并不限流地继续 Provider 执行。
- 改变已有运行中 lease 的任务状态、重复输出、usage、API call log 或终态事件幂等性。
- 增加数据库列/migration、前端 UI、队列实现、存储配额/retention 或不相关 Provider/模型改动。

### 失败模式与边界场景

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| 无 `task_concurrency` override | GET 返回环境 effective 值；Worker 继续使用环境限制 | 是 |
| admin PATCH 合法、较严格的单字段或多字段策略 | 仅当前 tenant 的 effective 值变化，写 sanitized operation log；Worker lease 使用新限制 | 是 |
| non-admin、缺 CSRF、跨 tenant 探测 | `403` 或既有授权行为；不得读写其他 tenant 策略 | 是 |
| 零值、负值、非整数、未知字段、超出环境 hard cap、尝试 global 字段 | `422 VALIDATION_ERROR`；现有策略和 operation log 不变 | 是 |
| tenant/user/provider/model 某一 effective limit 已满 | Worker 不进入 `RUNNING`/Provider 执行，按现有 concurrency-limited retry 行为保留 eligible task | 是 |
| Provider 行 `concurrencyLimit` 比策略 Provider limit 更严格 | Redis Provider dimension 使用更小值 | 是 |
| 已有 lease 后 PATCH 更严格策略 | 已有执行不被中止；新 lease 使用更新后的策略 | 是 |
| `task_concurrency.value_json` 被手工写成非法/部分/越界配置 | eligible task 以 sanitized `TASK_CONFIGURATION_INVALID` 失败；无 Provider/output/usage/API-call 成功副作用 | 是 |
| Worker 读取设置遇到真实数据库/基础设施错误 | claim retry；不得执行 Provider、不得以配置错误终止、不得绕过 limiter | 是 |
| 重复 claim、取消、超时、recovery/stale lease | 现有状态机与幂等语义不变 | 回归现有测试 |

### 安全要求

- 不解密、不读取、不记录 Provider API Key 或任何凭据。
- 设置响应、操作日志与 Worker 错误不得包含 `value_json` 原文、跨租户详情、Authorization、Cookie、token、base64 或内部错误栈。
- 所有设置读取、写入、Worker policy lookup 和任务对象操作继续按 `tenant_id` 过滤。
- Tenant policy 只能收紧平台 hard caps；Global limit 始终由服务端环境控制。
- 不引入 frontend Provider 直连、轮询或任何浏览器存储行为。

### 必须新增或更新的回归测试

- 在 `backend/internal/api/*system_settings*_test.go` 覆盖 fallback GET、合法 PATCH/tenant 隔离/脱敏审计，以及非法、越界和 global/未知字段拒绝。
- 在 `backend/internal/task/worker_test.go` 覆盖每个 effective dimension、Provider 更严格 cap、策略变更只影响新 lease、非法持久化策略 fail closed、存储读取错误 retry 且不调用 executor。
- 运行已有 Worker duplicate/cancel/timeout/recovery/stale lease 测试以及现有 `uploadPolicy`、`taskDefaults`、CSRF/RBAC、操作日志测试，证明没有回归。

### 验收标准

- `taskConcurrency` 仅在真实 Worker consumer 同步落地的前提下进入系统设置 API，且仅允许 tenant 收紧环境 hard caps。
- Worker 按 tenant/user/provider/model effective limits 获取 Redis lease，global 与 Provider stricter-cap 语义不退化。
- 损坏配置和存储故障均不可能导致无约束 Provider 执行；错误、审计与响应不泄漏敏感或内部数据。
- 未新增 migration、前端/部署代码、配额/retention 字段或无关范围修改。

### 测试命令

```bash
cd backend
go test ./internal/api ./internal/task ./internal/settings ./cmd/worker -count=1
go test ./... -count=1
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
git diff --check main...HEAD
```

如使用共享本地 MySQL、Redis 做额外 policy/lease 验证，必须按 `docs/local-development.md` 使用任务命名空间数据，并在交付中记录清理结果；本任务不得另起项目专属依赖容器，不需要为了并发策略写入 MinIO 测试对象。

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
