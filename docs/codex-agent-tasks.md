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

R14 已完成，未发现阻塞问题。P14 的 Provider/model 生命周期完整性、后端 usage/cost reporting、前端成本可观测性和整批回归均已合并并验证。

已完成的平台基础：

- P0-P3：文档、monorepo 布局、后端/前端/部署骨架、Docker 运行时修复。
- P4-P6：数据库、租户、认证、RBAC、项目、资产、Provider/模型管理。
- P7-P8：任务队列、Worker、Provider Adapter 运行时、SSE、前端后端化。
- P9-P10：审计/用量读取、运行时生效的上传策略、生产密钥保护、安全/部署验证、Worker 并发池、SSE bridge 生命周期、Provider/模型生命周期、admin 硬化、后端统一历史查询。
- P11-P12：用户/角色管理、统一历史、卖家项目/资产流程和项目最后 `OWNER` 约束。
- P13 已合并切片：租户 `taskDefaults.{defaultProviderId,defaultModelId}` 的读写、任务创建真实消费路径、损坏持久化默认值的 fail-closed hardening、Worker 实际消费的 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`、soft-delete 资产物理清理基础服务、Worker 实际消费的 `storageRetention.deletedAssetRetentionDays`、引用图上传/Worker 输出实际消费的 `storageQuota.maxBytes` 与只读 `storageQuota.usedBytes`，以及只展示 active runtime-backed settings 的前端 admin 设置页。
- P14 已合并切片：Provider/model 生命周期完整性、后端确定性 usage/cost reporting、前端 admin 成本可观测性和 R14。Worker 成本估算使用稳定 decimal，非法 pricing 归零且不失败成功任务；admin usage summary 支持 tenant/user/project/Provider/model 维度、tenant isolation、多币种分组和 exact decimal cost；前端 usage tab 支持 tenant totals、过滤、drilldown、多币种展示和 stale response 防护。

当前已知后续项：

- 前端历史已改为消费 `GET /api/v1/projects/{projectId}/history`。
- 前端项目/资产工作流已完成一轮产品化打磨：项目编辑、资产筛选、资产元数据编辑、项目成员入口、上传切项目 stale 保护和筛选后列表一致性均已合并。
- 后端项目成员写路径已补齐最后一个 `OWNER` 保护：不能删除或降级项目最后一个 `OWNER`，但允许先新增或提升另一个 `OWNER` 后再完成 owner 转移。
- R12 已验证 P12 范围内的卖家工作流、统一历史、项目/资产 UI、项目成员 API、最后 `OWNER` 保护、操作日志、权限边界、前端禁止模式和 Compose 配置。
- 用户/角色管理后端接口和前端管理 UI 已完成；后续租户/团队更深层能力可在新的任务中继续补齐。
- 上传策略、`taskDefaults`、`taskConcurrency`、`storageRetention` 与 `storageQuota` 已有真实运行时消费者；损坏的 `task_defaults`、`task_concurrency`、`storage_retention` 与 `storage_quota` 配置必须 fail closed，不能绕过校验、限流、Provider 执行边界、清理边界或资产写入边界。其他运行时设置在消费者落地前不得暴露为可写配置。
- 下一任务是 `P15-E2E-CORE-FLOWS`，串行补齐可由 operator 执行的核心端到端验证。
- 缩略图策略、完整 orphan cleanup 和 log retention 仍需实现。
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
   - 已完成并合并。租户并发策略只允许收紧环境 hard caps，并已由 Worker Redis semaphore acquisition 实际消费；global 并发仍由环境配置控制。
4. `P13-BE-STORAGE-CLEANUP-FOUNDATION`
   - 已完成并合并。上传后 metadata 失败 cleanup 不再依赖 request context；soft-delete 资产已有 tenant/cutoff/batch/idempotent 的内部物理清理基础和 `purged_at` 标记。
5. `P13-BE-STORAGE-RETENTION-RUNTIME`
   - 已完成并合并。`storageRetention.deletedAssetRetentionDays` 默认 `null`/disabled，Worker maintenance loop 只消费合法 active-tenant 设置并调用 cleanup foundation。
6. `P13-BE-STORAGE-QUOTA-ACCOUNTING`
   - 已完成并合并。`storageQuota.maxBytes` 默认 `null`/unlimited，`storageQuota.usedBytes` 从 tenant-scoped 未 purged 资产 metadata 计算；引用图上传和 Worker 输出资产持久化都会在写入前执行配额校验。
7. `P13-FE-SYSTEM-SETTINGS`
   - 已完成并合并。前端 admin settings tab 仅展示 active runtime-backed 设置，并按分组发送 CSRF-protected patch；未暴露 log retention、orphan cleanup、manual cleanup、MinIO listing 或 Provider secrets。
8. `R13`
   - 已完成。完整 P13 范围通过前端 lint/type-check/test/build、后端 test/race/vet/build、Compose config、whitespace 检查，以及前端 Provider 直连、Provider Key 存储、task polling、deferred settings、bucket/object key 和敏感 auth 字符串扫描。

### P14：Provider、模型、用量与成本运营

目标：强化运营数据完整性，并让用量/成本统计对实际运营有用。

建议任务：

1. `P14-BE-PROVIDER-MODEL-INTEGRITY`
   - 已完成并合并。Provider delete/disable、model create/update/enable 和 taskDefaults 读取现在会保持 Provider/model 生命周期一致性；同 Provider `model_name` 唯一约束暂不落地。
2. `P14-BE-USAGE-COST-REPORTING`
   - 已完成并合并。后端成本估算确定、非负、8 位小数稳定；usage summary 支持 tenant/user/project/Provider/model 聚合、过滤、分页、多币种和 exact decimal cost。
3. `P14-FE-COST-OBSERVABILITY`
   - 已完成并合并。前端成本/用量看板现在消费后端 tenant/user/project/Provider/model summary，支持 tenant totals、filters、drilldown、多币种展示和 stale response 防护。
4. `R14`
   - 已完成。完整 P14 范围通过 frontend/backend/Compose/whitespace/禁止模式回归，未发现阻塞问题。

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

## 下一个任务包：P15-E2E-CORE-FLOWS

### 调度决策

- 本任务串行执行，不与最终安全回归或部署 runbook 并行。
- 理由：P15 的第一步应先把核心平台流转固化为可重复验证的 E2E/集成回归，后续安全和发布验证才能复用同一套检查入口。
- 本任务以测试和验证脚本为主，不开发新业务能力，不调用真实 AI Provider，不改公共合同文档或 Agent 规则。

### 任务信息

- 任务名称：`P15-E2E-CORE-FLOWS`
- 目标：新增可重复执行的核心平台端到端验证，覆盖 init admin/login、Provider/model 配置、项目、参考图上传、任务创建、Worker fake 执行或测试替身、SSE 事件/回放、输出资产、下载、再次编辑入口所需数据、history、usage/cost、operation/API logs 的关键合同。
- 推荐线程名：`P15-E2E-CORE-FLOWS`
- 推荐分支名：`codex/p15-e2e-core-flows`
- 起始分支：已完成 R14 的最新 `main`
- 前置依赖：P14 全部代码和 R14 文档已合并；共享本地 MySQL/Redis/MinIO 使用规则见 `docs/local-development.md`。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P15-E2E-CORE-FLOWS`。

你必须在分支 `codex/p15-e2e-core-flows` 上工作；如果当前不在该分支，先执行 `git switch codex/p15-e2e-core-flows`，确认 `git branch --show-current` 后再继续。起始点必须包含完成 R14 后的最新 `main`；如果 `git merge-base --is-ancestor main codex/p15-e2e-core-flows` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
为完整平台补齐第一批 P15 核心 E2E/集成验证。验证必须证明平台关键合同可以串起来，而不是只测孤立函数：
- 管理员初始化或登录链路可取得 HttpOnly Cookie 会话和内存 CSRF。
- Provider/model 管理合同可创建启用的 Provider/model，并且 Provider API Key 不会完整返回。
- 项目创建、参考图上传、资产 metadata、授权下载合同可用，上传仍走真实文件校验。
- 任务创建进入后端任务合同，Worker 路径使用测试替身或 fake Provider Adapter 完成成功任务，不允许调用真实 OpenAI/Gemini/中转站。
- SSE 事件、Last-Event-ID 回放或任务事件历史读取能看到排队、开始、输出、usage、完成等关键事件。
- 输出图片进入项目资产库，项目 history 能看到生成/编辑结果，下载必须经后端鉴权。
- usage/cost summary、usage records、operation logs、API call logs 可按租户读取并保持脱敏。

必须遵守：
1. 不修改 `docs/**`、`AGENTS.md`、`agent-instructions/**`。
2. 不开发新业务能力，不改变公开 API/SSE 合同；如发现必须改合同，停止并报告主 agent。
3. 不调用真实 AI Provider，不访问 OpenAI、Gemini 或任何外部中转站；Provider 调用必须用测试替身、fake adapter、httptest 或已有测试 helper。
4. 不把真实本地 MySQL/Redis/MinIO 凭据、Provider Key、Authorization、Cookie、JWT、图片 base64、bucket 或 object_key 写入仓库、快照、日志或最终交付。
5. 不使用前端轮询、`setInterval` 或循环 fetch 来模拟任务状态；任务状态验证必须走 SSE、任务事件历史或 Worker 测试事件。
6. 如使用共享本地 MySQL/Redis/MinIO，只能创建任务命名空间测试数据，例如 `codex_p15_e2e_*`，不得 drop 数据库、删除共享 bucket、flush Redis 或删除无关数据。
7. 测试必须可重复执行；失败时要清楚指出失败阶段和关键对象 ID，但不得泄露敏感字段。

建议实现：
1. 先阅读：
   - `backend/internal/api/*_test.go`
   - `backend/internal/task/worker_test.go`
   - `backend/internal/sse/*_test.go`
   - `frontend/src/test/taskWorkbench.test.tsx`
   - `frontend/src/test/historyAssetSource.test.tsx`
   - `frontend/src/test/adminObservabilitySettingsPanel.test.tsx`
   - `docs/api-contract.md`
   - `docs/sse-contract.md`
   - `docs/security.md`
   - `docs/local-development.md`
2. 优先新增后端集成测试文件，例如 `backend/internal/api/e2e_core_flow_test.go`，复用现有 test router/helper。测试应串联真实路由、真实 DB metadata、真实 MinIO/storage 测试替身或现有 upload helper、真实 task/event/history/usage/audit 查询；Provider/AI 调用必须 fake。
3. 如单个后端测试难以跨包驱动 Worker，可在 `backend/internal/task/e2e_core_flow_test.go` 补 Worker fake execution 测试，并在 API 层测试验证创建、history、SSE replay、资产、usage/log reads 合同。不要为测试方便改生产代码。
4. 可新增一个轻量脚本 `scripts/e2e-core-flow.sh`，作为 operator 本地验证入口。脚本只能编排已有测试命令或可选检查共享本地服务连通性；不得写死本机真实密码，不得启动新 MySQL/Redis/MinIO，不得清空共享服务。
5. 如需要前端合同覆盖，可新增 `frontend/src/test/e2eCoreFlowContracts.test.tsx` 或扩展现有测试，验证 workbench/history/admin observability 对后端核心合同的串联假设，但不要重写 UI。

验收命令：
```bash
cd backend
go test ./internal/api ./internal/task ./internal/sse ./internal/audit -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
cd frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ..
docker compose -f deploy/docker-compose.yml config
git diff --check main...HEAD

# 如新增脚本：
bash scripts/e2e-core-flow.sh --help
```

最终交付必须包含：
- 修改文件清单。
- 执行的测试命令和结果。
- 每个 failure mode 对应的测试文件/测试名映射。
- 安全自查结果，明确没有真实 AI Provider 调用、Provider Key 存储、task polling、localStorage/sessionStorage/IndexedDB 敏感数据、Authorization/Cookie/JWT/base64/object key 暴露。
- 刻意未修改范围。
- 如使用共享本地服务，说明创建/修改/清理了哪些 `codex_p15_e2e_*` 测试数据。
- 如发现公共合同缺口，只报告给主 agent，不修改 docs。
```

### 允许修改文件

- `backend/internal/api/*core_flow*_test.go`
- `backend/internal/task/*core_flow*_test.go`
- `backend/internal/sse/*core_flow*_test.go`
- `backend/internal/audit/*core_flow*_test.go`
- `frontend/src/test/*coreFlow*.test.tsx`
- `scripts/e2e-core-flow.sh`
- 为复用现有测试 helper，可小范围修改现有 `*_test.go`；不得改生产代码。

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/internal/**` 非测试生产文件，除非发现测试无法表达真实合同且先停止报告；默认不改
- `frontend/src/**` 非测试生产文件
- `deploy/**`
- `frontend/src/lib/taskSseClient.ts`
- 任何 Provider 直连、AI relay、Provider Key 存储、task polling、IndexedDB 敏感数据或真实外部网络调用路径

### 具体开发内容

1. 建立 P15 核心流转测试入口，优先为后端 API/Worker/SSE/audit 增加端到端或准端到端测试。
2. 测试中创建命名空间化租户、管理员、Provider、模型、项目、参考图资产、任务和输出资产。
3. 使用 fake Provider/Worker 执行路径产生输出图片、usage record、API call log 和 task events；不得访问外部网络。
4. 验证 SSE 或 task event replay 的 Last-Event-ID/历史补发关键行为能覆盖该任务。
5. 验证项目 history、资产 detail/download、usage summary/records、operation logs、API call logs 均只返回当前 tenant 数据并保持脱敏。
6. 若新增脚本，只做验证编排，不写业务逻辑，不保存 secrets，不创建额外服务。

### 必须保持的现有行为

- 现有单元、集成和前端测试不回归。
- 生产代码行为不因测试任务改变。
- 任务状态仍通过 SSE/事件历史验证，不引入 polling。
- Provider key、auth cookie、CSRF、JWT、object_key、bucket、image bytes 仍不进入日志、快照或前端可见状态。
- Docker Compose 配置不被本任务改动。

### 允许的中间态

- 本任务可以先以自动化测试和验证脚本覆盖核心合同，不要求启动完整真实外部 AI。
- 如某些真实浏览器交互无法稳定自动化，可先用前端组件合同测试加后端 API/Worker 集成测试组合覆盖，但必须说明缺口。
- 允许使用共享本地服务做额外手工验证，但自动化测试不得依赖本机私密凭据。

### 禁止的半迁移状态

- 只新增脚本但没有任何自动化断言。
- 测试绕过权限、租户过滤、上传校验或 Worker/任务状态机，导致无法证明真实合同。
- 用真实 Provider Key 或外部 AI 调用“验证成功”。
- 通过改生产代码来适配测试，但没有产品需求或合同依据。
- 留下不可清理的本地服务数据、Redis 队列消息或 MinIO 对象。

### 失败模式与边界场景

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| 未登录访问核心接口 | 返回 401，不创建业务数据 | 是 |
| 无权限或跨租户读取项目/资产/任务/history/usage/log | 返回 403/404，不泄露存在性 | 是 |
| Provider API Key 创建后读取 | 只返回 hint/metadata，不返回完整 key | 是 |
| 参考图上传 | 校验真实图片类型、大小和 metadata，禁止 SVG/base64 直存 | 是 |
| 任务执行成功 | 产生 task events、output asset、usage record、API call log、operation log | 是 |
| SSE replay / Last-Event-ID | 能补发历史事件或从事件历史验证等价合同 | 是 |
| 输出资产下载 | 必须经后端鉴权，跨租户不可下载 | 是 |
| usage/cost summary | 按 tenant 隔离，成本使用后端字符串，不重新计算 | 是 |
| 敏感日志/响应 | 不包含 Provider Key、Authorization、Cookie、JWT、base64、bucket、object_key | 是 |
| 测试失败清理 | 不留下无归属 Redis/MinIO/MySQL 测试数据 | 是 |

### 安全要求

- 所有测试数据必须包含 tenant/project/task/asset 级别隔离，跨租户断言必须覆盖至少一个核心资源。
- Provider/AI 调用必须 fake；测试替身返回的错误和 metadata 也要覆盖脱敏。
- 任何脚本不得读取或打印真实 `.env` secrets；如需环境变量，只打印变量名是否存在，不打印值。
- 不得新增 browser storage、polling、Provider direct call、MinIO direct download 或 object key 暴露。

### 必须新增或更新的回归测试

- 后端核心流转测试：覆盖 auth、Provider/model、project、asset upload/download、task create、fake Worker success、events/SSE replay/history、usage/log reads。
- 跨租户/无权限负向测试：至少覆盖 project/history/asset download/usage 或 log 中的两个资源类型。
- 敏感信息负向测试：响应与日志 payload 不含 Provider Key、Authorization、Cookie、JWT、image base64、bucket、object_key。
- 如新增前端合同测试：覆盖 workbench/history/admin observability 对核心后端合同的调用顺序和 forbidden patterns。

### 验收标准

- 至少有一个可重复执行的核心流转自动化测试入口，且失败信息能定位阶段。
- 测试证明核心 seller/admin flow 可以从认证走到资产、任务、事件、history、usage/cost 和 logs。
- 无真实 AI Provider 调用，无敏感数据泄漏，无轮询，无浏览器 Provider Key 存储。
- 现有前端、后端、Compose 配置验证全部通过。
- 未修改公共合同文档、Agent 规则、部署配置或无关生产代码。

### 测试命令

```bash
cd backend
go test ./internal/api ./internal/task ./internal/sse ./internal/audit -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
cd frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ..
docker compose -f deploy/docker-compose.yml config
git diff --check main...HEAD

if [ -f scripts/e2e-core-flow.sh ]; then
  bash scripts/e2e-core-flow.sh --help
fi
```

如使用共享本地服务做额外功能验证，必须按 `docs/local-development.md` 使用任务命名空间数据并在交付中记录。

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
