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

R13 已完成，未发现阻塞问题。`P14-BE-PROVIDER-MODEL-INTEGRITY` 与 `P14-BE-USAGE-COST-REPORTING` 已 review、修复阻塞项并合并，P14 进入前端成本可观测性切片。

已完成的平台基础：

- P0-P3：文档、monorepo 布局、后端/前端/部署骨架、Docker 运行时修复。
- P4-P6：数据库、租户、认证、RBAC、项目、资产、Provider/模型管理。
- P7-P8：任务队列、Worker、Provider Adapter 运行时、SSE、前端后端化。
- P9-P10：审计/用量读取、运行时生效的上传策略、生产密钥保护、安全/部署验证、Worker 并发池、SSE bridge 生命周期、Provider/模型生命周期、admin 硬化、后端统一历史查询。
- P11-P12：用户/角色管理、统一历史、卖家项目/资产流程和项目最后 `OWNER` 约束。
- P13 已合并切片：租户 `taskDefaults.{defaultProviderId,defaultModelId}` 的读写、任务创建真实消费路径、损坏持久化默认值的 fail-closed hardening、Worker 实际消费的 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`、soft-delete 资产物理清理基础服务、Worker 实际消费的 `storageRetention.deletedAssetRetentionDays`、引用图上传/Worker 输出实际消费的 `storageQuota.maxBytes` 与只读 `storageQuota.usedBytes`，以及只展示 active runtime-backed settings 的前端 admin 设置页。
- P14 已合并切片：Provider/model 生命周期完整性，以及后端确定性 usage/cost reporting。Worker 成本估算使用稳定 decimal，非法 pricing 归零且不失败成功任务；admin usage summary 支持 tenant/user/project/Provider/model 维度、tenant isolation、多币种分组和 exact decimal cost。

当前已知后续项：

- 前端历史已改为消费 `GET /api/v1/projects/{projectId}/history`。
- 前端项目/资产工作流已完成一轮产品化打磨：项目编辑、资产筛选、资产元数据编辑、项目成员入口、上传切项目 stale 保护和筛选后列表一致性均已合并。
- 后端项目成员写路径已补齐最后一个 `OWNER` 保护：不能删除或降级项目最后一个 `OWNER`，但允许先新增或提升另一个 `OWNER` 后再完成 owner 转移。
- R12 已验证 P12 范围内的卖家工作流、统一历史、项目/资产 UI、项目成员 API、最后 `OWNER` 保护、操作日志、权限边界、前端禁止模式和 Compose 配置。
- 用户/角色管理后端接口和前端管理 UI 已完成；后续租户/团队更深层能力可在新的任务中继续补齐。
- 上传策略、`taskDefaults`、`taskConcurrency`、`storageRetention` 与 `storageQuota` 已有真实运行时消费者；损坏的 `task_defaults`、`task_concurrency`、`storage_retention` 与 `storage_quota` 配置必须 fail closed，不能绕过校验、限流、Provider 执行边界、清理边界或资产写入边界。其他运行时设置在消费者落地前不得暴露为可写配置。
- 下一任务是 `P14-FE-COST-OBSERVABILITY`，串行强化前端 admin 用量/成本看板和明细钻取。
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
   - 下一个任务。前端成本/用量看板和明细钻取。
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

## 下一个任务包：P14-FE-COST-OBSERVABILITY

### 调度决策

- 本任务串行执行，不与 R14 review 或其他前端 admin 重构并行。
- 理由：它消费刚合并的后端 usage/cost 合同，是 R14 前最后一个 P14 功能切片。
- 本任务只改前端 admin observability usage/cost 相关类型、API wrapper、UI 和测试，不改后端、部署、公共合同文档或 Agent 规则。

### 任务信息

- 任务名称：`P14-FE-COST-OBSERVABILITY`
- 目标：在现有 `AdminObservabilitySettingsPanel` 的 usage tab 中接入 P14 后端成本统计能力，展示 tenant totals、维度汇总、过滤后的 usage records 和明细钻取，同时保持权限、分页、错误、空态、脱敏显示边界和 no-polling/no-Provider-direct-call 规则。
- 推荐线程名：`P14-FE-COST-OBSERVABILITY`
- 推荐分支名：`codex/p14-frontend-cost-observability`
- 起始分支：已合并 `P14-BE-USAGE-COST-REPORTING` 与本任务公共合同文档的最新 `main`
- 前置依赖：P14 backend usage/cost reporting 已完成；现有前端 admin observability/settings panel、admin API wrapper 和 tests 已存在。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P14-FE-COST-OBSERVABILITY`。

你必须在分支 `codex/p14-frontend-cost-observability` 上工作；如果当前不在该分支，先执行 `git switch codex/p14-frontend-cost-observability`，确认 `git branch --show-current` 后再继续。起始点必须包含最新 `main`；如果 `git merge-base --is-ancestor main codex/p14-frontend-cost-observability` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
在现有前端 admin observability usage tab 中接入后端 P14 usage/cost reporting：
- 支持 `dimension=tenant|user|project|provider|model`，默认展示 tenant totals 或清晰的 tenant-level 总览。
- 增加成本/用量过滤控件：时间范围、taskId、userId、projectId、providerId、modelId。过滤必须同时作用于 summary 和 usage records。
- 增加从汇总卡片到用量记录的 drilldown：点击 user/project/provider/model 汇总时填入对应过滤条件并刷新 records；tenant 维度不应制造无效过滤。
- 成本展示必须使用后端返回的 `estimatedCost` 和 `currency` 字符串，不做前端权威成本重算。
- 保持 usage records 原有分页、rawUsage 脱敏预览、错误、loading、empty 和权限隐藏行为。

必须遵守：
1. 不修改 backend、deploy、docs、AGENTS.md、agent-instructions。
2. 不新增任何 OpenAI/Gemini/AI relay 直连，不新增 Provider API Key 输入、存储、读取、localStorage/sessionStorage/IndexedDB 持久化。
3. 不使用轮询、`setInterval` 或循环 fetch。Usage/cost 数据只在打开 tab、切换维度/过滤/分页、点击刷新或 drilldown 时读取。
4. 不把 raw usage 展开成不受控的大段敏感文本；继续通过现有 `MetadataPreview` 或同等安全预览展示后端已脱敏数据。
5. 不在前端重新计算权威成本，不把不同 currency 强行合并成一个总数；如果展示汇总，只能按 currency 分组或明确标注当前页/当前筛选。
6. 只按 `usage:read` 权限展示 usage/cost UI；无权限时不得发起 usage API 请求。
7. 可以使用 `docs/local-development.md` 中的共享本地服务做手工验证，但本任务默认应以 mock API 的前端自动化测试为主。

建议实现：
1. 先阅读：
   - `frontend/src/types/admin.ts`
   - `frontend/src/api/admin.ts`
   - `frontend/src/components/admin/AdminObservabilitySettingsPanel.tsx`
   - `frontend/src/test/adminApi.test.ts`
   - `frontend/src/test/adminObservabilitySettingsPanel.test.tsx`
2. 先补测试，再实现：
   - 将 `UsageSummaryDimension` 扩展为 `tenant | user | project | provider | model`。
   - 更新 admin API wrapper 测试，证明 `dimension=tenant` 和 usage filters 会正确序列化。
   - 在 usage tab 中增加 tenant totals 区域。建议单独请求 `getUsageSummary({ dimension: 'tenant', filters... })`，并按 currency 展示后端返回的 totals。
   - 保留现有 summary 维度选择，并加入 `tenant` 选项；切换维度时重置 summary 页码。
   - 增加过滤表单，字段至少包括：`createdAtFrom`、`createdAtTo`、`taskId`、`userId`、`projectId`、`providerId`、`modelId`。应用过滤时重置 summary 和 records 页码；清空过滤时恢复空过滤。
   - 点击 summary card 时执行 drilldown：根据当前 row 的 dimension 写入对应 filter（user/project/provider/model），重置 records 页码并刷新；不要为 tenant row 写入 `tenantId` 或任何后端不支持的过滤参数。
   - Usage records 表格继续显示 `estimatedCost currency`、tokens、imageCount、rawUsage 预览、时间和对象 ID。
   - 避免 UI 文字溢出，表格/卡片在桌面与窄屏下保持可扫描。
3. 不要把本任务做成新的 admin 页面。沿用现有 observability/settings 面板和 tab 结构。

验收命令：
```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ..
git diff --check main...HEAD
docker compose -f deploy/docker-compose.yml config
```

最终交付必须包含：
- 修改文件清单。
- 执行的测试命令和结果。
- 每个 failure mode 对应的测试文件/测试名映射。
- 安全自查结果，明确没有新增 Provider 直连、Provider Key 存储、task polling、localStorage/sessionStorage/IndexedDB 敏感数据、Authorization/Cookie/JWT/base64/object key 暴露。
- 刻意未修改范围。
- 成本展示规则说明，包括 currency 分组、tenant totals 来源、drilldown 行为和不做前端权威成本重算。
- 如发现公共合同缺口，只报告给主 agent，不修改 docs。
```

### 允许修改文件

- `frontend/src/types/admin.ts`
- `frontend/src/api/admin.ts`
- `frontend/src/components/admin/AdminObservabilitySettingsPanel.tsx`
- `frontend/src/test/adminApi.test.ts`
- `frontend/src/test/adminObservabilitySettingsPanel.test.tsx`
- 可新增 `frontend/src/components/admin/*Usage*` 小组件文件，但优先保持现有面板内局部拆分，避免大规模重构

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/src/lib/taskSseClient.ts`，除非类型编译需要且不改变 SSE 行为；默认不改
- `frontend/src/api/providerModel.ts`、`frontend/src/api/task.ts`、`frontend/src/api/projectAsset.ts`
- 任何旧 Provider 直连、AI relay、Provider Key 存储、task polling 或 IndexedDB 敏感数据路径

### 具体开发内容

1. 扩展 usage summary 类型和 query serialization。
2. 在 usage tab 顶部增加成本/用量 totals，清楚展示当前筛选下 tenant totals，支持多 currency 分组。
3. 增加过滤控件和应用/清空行为，所有过滤参数同时作用于 tenant totals、summary 和 records。
4. 增加 summary card drilldown 到 records 的行为。
5. 更新 loading/error/empty/pagination 状态，避免 stale response 覆盖最新筛选结果；如新增请求序号保护，应只用于本 tab。
6. 更新测试覆盖 API serialization、权限 gating、filters、tenant dimension、drilldown、multi-currency display、error/empty/loading 和 no unauthorized request。

### 必须保持的现有行为

- `AdminObservabilitySettingsPanel` 仍按权限显示 usage、operation logs、API call logs、settings tabs。
- 无 `usage:read` 时不调用 usage summary 或 usage records API。
- operation logs、API call logs、system settings UI 不因本任务回归。
- 现有 usage records 分页、刷新、错误、空态和 raw usage preview 保持可用。
- API wrapper 使用 same-origin `/api/v1/admin/...`，不新增外部请求。

### 允许的中间态

- 本任务只提供 admin usage/cost observability，不新增 billing、invoice、预算告警或导出功能。
- 前端可以显示对象 ID 而不是对象名称；名称映射/搜索选择器可留给后续体验任务。
- 图表不是硬要求；如果使用简单 cards/tables 更符合现有 UI，可以不引入 chart 依赖。

### 禁止的半迁移状态

- UI 显示 tenant/cost 过滤控件，但实际 API 请求没有带对应 query。
- 点击 drilldown 后视觉筛选状态和 records 请求不一致。
- 将不同 currency 合并为一个未标注总成本。
- 使用 `setInterval`、循环 fetch 或后台自动刷新 usage/cost。
- 为了展示成本而在前端读取或保存 Provider secret、raw Authorization/Cookie/JWT/base64/object key。

### 失败模式与边界场景

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| 无 `usage:read` | 不显示 usage tab，不发 usage 请求 | 是 |
| 打开 usage tab | 加载 tenant totals、当前维度 summary、usage records | 是 |
| `dimension=tenant` | UI 可选择并显示 tenant rows，不写无效 tenant filter | 是 |
| 多 currency summary | 按 currency 分开展示，不合并成单一金额 | 是 |
| 应用 filters | summary、tenant totals、records 请求都带相同过滤条件并重置页码 | 是 |
| 清空 filters | 请求恢复无过滤状态并重置页码 | 是 |
| 点击 provider/model/project/user summary row | 写入对应 filter，刷新 records，视觉状态与请求一致 | 是 |
| 点击 tenant summary row | 不写后端不支持的 tenantId filter，不产生错误请求 | 是 |
| usage API 失败 | 显示错误，不清空其他 tabs，不泄露内部对象 | 是 |
| stale response | 后发请求结果不被先发慢请求覆盖，或实现保持现有无 stale 风险 | 是 |

### 安全要求

- 前端只调用后端 `/api/v1/admin/usage/*`，不调用 AI Provider 或 MinIO。
- 不持久化 usage filters 到 localStorage/sessionStorage/IndexedDB，除非后续有明确产品要求和安全设计。
- Raw usage 只显示后端返回的 redacted payload 预览，不增加复制完整 payload、下载 raw payload 或扩大预览上限。
- 错误信息使用现有 `formatAdminError` 风格，不显示 stack trace、Authorization、Cookie、JWT、Provider Key、image base64、bucket 或 object key。

### 必须新增或更新的回归测试

- `adminApi.test.ts`：`dimension=tenant` 和全部 usage filters/query serialization。
- `adminObservabilitySettingsPanel.test.tsx`：tenant totals、dimension selector 包含 tenant、filters apply/clear、summary drilldown、multi-currency display、usage permission gating、API failure state。
- 如拆出小组件，增加对应组件行为测试或保持通过面板集成测试覆盖。

### 验收标准

- 前端 usage/cost UI 明确消费 `dimension=tenant|user|project|provider|model` 后端合同。
- 过滤和 drilldown 请求参数与 UI 状态一致。
- 多 currency 不被错误合并，成本值使用后端字符串展示。
- 无 `usage:read` 时不会发 usage 请求。
- operation logs、API call logs、system settings 原有测试不回归。
- 未修改后端、部署、公共合同文档或 Agent 规则。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build

cd ..
git diff --check main...HEAD
docker compose -f deploy/docker-compose.yml config
```

如使用共享本地服务做额外功能验证，必须按 `docs/local-development.md` 使用任务命名空间数据并在交付中记录；本任务默认可以只用前端自动化测试完成。

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
