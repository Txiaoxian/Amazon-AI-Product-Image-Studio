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

R13 已完成，未发现阻塞问题。`P14-BE-PROVIDER-MODEL-INTEGRITY` 已 review、修复阻塞项并合并，P14 进入后端用量/成本统计切片。

已完成的平台基础：

- P0-P3：文档、monorepo 布局、后端/前端/部署骨架、Docker 运行时修复。
- P4-P6：数据库、租户、认证、RBAC、项目、资产、Provider/模型管理。
- P7-P8：任务队列、Worker、Provider Adapter 运行时、SSE、前端后端化。
- P9-P10：审计/用量读取、运行时生效的上传策略、生产密钥保护、安全/部署验证、Worker 并发池、SSE bridge 生命周期、Provider/模型生命周期、admin 硬化、后端统一历史查询。
- P11-P12：用户/角色管理、统一历史、卖家项目/资产流程和项目最后 `OWNER` 约束。
- P13 已合并切片：租户 `taskDefaults.{defaultProviderId,defaultModelId}` 的读写、任务创建真实消费路径、损坏持久化默认值的 fail-closed hardening、Worker 实际消费的 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`、soft-delete 资产物理清理基础服务、Worker 实际消费的 `storageRetention.deletedAssetRetentionDays`、引用图上传/Worker 输出实际消费的 `storageQuota.maxBytes` 与只读 `storageQuota.usedBytes`，以及只展示 active runtime-backed settings 的前端 admin 设置页。
- P14 已合并切片：Provider/model 生命周期完整性，包括 Provider delete/disable 与 model create/update/enable 的可用性边界、默认 task settings 读取重校验和失败写入不记成功日志。

当前已知后续项：

- 前端历史已改为消费 `GET /api/v1/projects/{projectId}/history`。
- 前端项目/资产工作流已完成一轮产品化打磨：项目编辑、资产筛选、资产元数据编辑、项目成员入口、上传切项目 stale 保护和筛选后列表一致性均已合并。
- 后端项目成员写路径已补齐最后一个 `OWNER` 保护：不能删除或降级项目最后一个 `OWNER`，但允许先新增或提升另一个 `OWNER` 后再完成 owner 转移。
- R12 已验证 P12 范围内的卖家工作流、统一历史、项目/资产 UI、项目成员 API、最后 `OWNER` 保护、操作日志、权限边界、前端禁止模式和 Compose 配置。
- 用户/角色管理后端接口和前端管理 UI 已完成；后续租户/团队更深层能力可在新的任务中继续补齐。
- 上传策略、`taskDefaults`、`taskConcurrency`、`storageRetention` 与 `storageQuota` 已有真实运行时消费者；损坏的 `task_defaults`、`task_concurrency`、`storage_retention` 与 `storage_quota` 配置必须 fail closed，不能绕过校验、限流、Provider 执行边界、清理边界或资产写入边界。其他运行时设置在消费者落地前不得暴露为可写配置。
- 下一任务是 `P14-BE-USAGE-COST-REPORTING`，串行强化后端用量/成本估算和聚合。
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
   - 下一个任务。改进后端确定性成本估算和按租户、用户、项目、Provider、模型聚合的用量/成本统计。
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

## 下一个任务包：P14-BE-USAGE-COST-REPORTING

### 调度决策

- 本任务串行执行，不与前端成本看板或 R14 review 并行。
- 理由：成本估算和用量汇总是前端成本看板的后端合同基础，必须先保证后端聚合、分页、红线和成本计算稳定。
- 本任务只改后端用量/成本估算、admin usage API 和测试，不改前端、部署、公共合同文档或 Agent 规则。

### 任务信息

- 任务名称：`P14-BE-USAGE-COST-REPORTING`
- 目标：强化后端用量/成本统计，使 Worker 持久化的 `usage_records.estimated_cost` 确定、非负且 8 位小数稳定，并让 admin usage summary 支持 tenant/user/project/Provider/model 维度的租户内聚合，为后续前端成本看板提供可信合同。
- 推荐线程名：`P14-BE-USAGE-COST-REPORTING`
- 推荐分支名：`codex/p14-backend-usage-cost-reporting`
- 起始分支：已合并 `P14-BE-PROVIDER-MODEL-INTEGRITY` 与本任务公共合同文档的最新 `main`
- 前置依赖：P14 Provider/model integrity 已完成；P9 admin usage read APIs、P7 Worker usage record persistence、P6/P14 model pricing metadata 均已存在。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P14-BE-USAGE-COST-REPORTING`。

你必须在分支 `codex/p14-backend-usage-cost-reporting` 上工作；如果当前不在该分支，先执行 `git switch codex/p14-backend-usage-cost-reporting`，确认 `git branch --show-current` 后再继续。起始点必须包含最新 `main`；如果 `git merge-base --is-ancestor main codex/p14-backend-usage-cost-reporting` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
强化后端用量/成本统计：
- Worker 持久化 `usage_records.estimated_cost` 时必须使用确定性、非负、8 位小数格式，不允许因为 float 漂移导致不稳定结果。
- 缺失、非法、负数或不完整的模型 pricing 配置必须产生 `0.00000000` 估算成本，而不能让已经成功的 Provider 任务失败。
- Admin usage summary 必须支持 `dimension=tenant|user|project|provider|model`，其中 `tenant` 只返回当前租户聚合，不允许跨租户。
- 用量/成本读取必须保持分页、时间范围、过滤、排序、红线脱敏和 RBAC 行为不回归。

必须遵守：
1. 不修改 frontend、deploy、docs、AGENTS.md、agent-instructions。
2. 不修改 Provider Adapter outbound runtime、SSE、Redis queue、task state machine 或 asset persistence 主流程；如确实必须越界，停止并报告。
3. 不解密 Provider API Key，不把 Provider API Key、Authorization、Cookie、JWT、图片 base64、MinIO object key 或 raw Provider payload 写入日志、响应或测试快照。
4. 所有 usage/API-call/operation 查询必须继续 tenant_id 过滤并保持 admin + RBAC 权限要求。
5. API 响应路径和已有字段保持兼容；新增 `dimension=tenant` 必须使用现有分页 envelope 和错误风格。
6. 如果新增数据库索引或 migration，必须兼容已有数据并只服务本任务查询性能。
7. 可以使用 `docs/local-development.md` 中的共享 MySQL/Redis/MinIO 做功能验证，但只允许操作任务自有测试数据，并在交付中说明。

建议实现：
1. 先阅读：
   - `backend/internal/task/runtime_persistence.go` 中 usage record 创建和当前 `estimateCost` 逻辑。
   - `backend/internal/audit/**` 中 usage records 和 summary 查询/响应。
   - `backend/internal/api/audit_usage_routes.go` 与 `backend/internal/api/audit_usage_routes_test.go`。
   - `backend/internal/model/**` 中 pricing JSON 校验。
2. 先补测试，再实现：
   - 将成本估算逻辑收敛到可测试的后端函数或小包，避免复制在 API 层和 Worker 层。
   - 用确定性 decimal/整数/`math/big` 方案计算成本，避免直接 float 累加造成 8 位小数不稳定。不要为了这个任务引入重型依赖，除非你能证明标准库无法满足。
   - 支持当前 pricing key：`inputToken`/`input_token`/`inputTokens`/`input_tokens`，`outputToken`/`output_token`/`outputTokens`/`output_tokens`，`image`/`images`/`outputImage`/`output_image`。
   - currency 继续规范化为 3 位大写代码，非法或缺失默认 `USD`。
   - 负数、NaN、非法 JSON、非法 unit price、缺失 unit price 都不得产生负成本或任务失败。
   - Admin usage summary 增加 `dimension=tenant`，保持现有 user/project/provider/model 维度兼容。
   - Usage summary 的 `total`、分页、排序、同 timestamp 稳定性和多 currency 分组保持清楚。
3. 保持 `rawUsage` 递归脱敏，不能为了成本统计返回未脱敏 Provider metadata。
4. 如果查询需要索引优化，只新增最小必要索引，并补 migration/schema 测试。

验收命令：
```bash
cd backend
go test ./internal/audit ./internal/api ./internal/task ./internal/database -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
git diff --check main...HEAD
docker compose -f deploy/docker-compose.yml config
```

最终交付必须包含：
- 修改文件清单。
- 执行的测试命令和结果。
- 每个 failure mode 对应的测试文件/测试名映射。
- 安全自查结果，明确没有泄露 Provider Key、Authorization、Cookie、JWT、跨租户数据、图片 base64 或 MinIO object key。
- 刻意未修改范围。
- 成本计算规则说明，包括支持的 pricing key、非法 pricing 行为和 rounding/format 策略。
- 如发现公共合同缺口，只报告给主 agent，不修改 docs。
```

### 允许修改文件

- `backend/internal/task/runtime_persistence.go`
- `backend/internal/task/*test.go`
- `backend/internal/audit/**`
- `backend/internal/api/audit_usage_routes.go`
- `backend/internal/api/audit_usage_routes_test.go`
- `backend/internal/model/**`，仅限 pricing validation/fixtures/tests 必要调整
- `backend/internal/database/**`，仅限 usage/cost 查询所需最小 migration/model/index/helper 与测试
- 可新增 `backend/internal/usagecost/**` 或等价小包，用于确定性成本估算

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/internal/provider/**`，除非只读测试 fixture 需要；默认不改
- `backend/internal/provideradapter/**`
- `backend/internal/queue/**`
- `backend/internal/sse/**`
- `backend/internal/asset/**`
- `backend/cmd/**`，除非编译因新增 migration wiring 必须调整；默认不改
- 任何 AI Provider outbound runtime、API Key 解密路径、task status state machine、SSE/queue 主流程

### 具体开发内容

1. 抽出或重构成本估算函数，供 Worker usage persistence 使用，并提供直接单元测试。
2. 确认 `usage_records.estimated_cost` 始终为 `0.00000000` 或正数 8 位小数字符串。
3. 增加 `dimension=tenant` summary 查询和响应，保持租户内过滤、分页、时间范围和排序。
4. 保持现有 `dimension=user|project|provider|model` 响应兼容。
5. 强化测试覆盖 raw usage redaction、跨租户隔离、多 currency 分组、同 timestamp 稳定分页、非法 query validation。
6. 如新增索引，更新 migration 测试并确认 MySQL/SQLite 测试兼容。

### 必须保持的现有行为

- `GET /admin/usage/records` 响应字段保持兼容。
- `GET /admin/usage/summary` 现有维度、分页、过滤和排序保持兼容。
- Usage、operation log、API call log 读取仍只允许 tenant admin + 对应 RBAC 权限。
- `rawUsage`、operation metadata、API call metadata 继续递归脱敏。
- Worker 成功任务不因 pricing 缺失或非法而失败。

### 允许的中间态

- 后端可先提供更准确的 usage/cost API；前端成本看板留给 `P14-FE-COST-OBSERVABILITY`。
- 价格配置仍来自模型 `pricing_json`，不新增 billing account 或 invoice 表。
- 不要求历史脏数据重算成本；如发现历史成本有 float 误差，只记录为后续 migration/maintenance 事项。

### 禁止的半迁移状态

- API summary 声称支持 tenant 维度但仍返回跨租户数据或未过滤数据。
- Worker 因 pricing 缺失/非法而把成功 Provider 任务标记失败。
- 成本估算直接使用不稳定 float 累加并导致不可预测的 8 位小数。
- 为了成本展示把 raw Provider usage、Authorization、Cookie、Provider Key 或 image base64 暴露给前端。
- 改动前端成本 UI 或部署配置。

### 失败模式与边界场景

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| pricing 缺失或空 | 成本 `0.00000000`，任务成功路径不失败 | 是 |
| pricing 非法 JSON 或非法数字 | 成本 `0.00000000`，不 panic，不泄露内部错误 | 是 |
| pricing 为负数 | 负数忽略或归零，成本不为负 | 是 |
| 多种 supported pricing key | input/output/image 都按兼容 key 计算 | 是 |
| 小数价格和大 token/image 数 | 8 位小数确定性 rounding/format | 是 |
| summary `dimension=tenant` | 只聚合当前租户，`dimensionId` 为当前 tenant ID | 是 |
| summary 多 currency | 按 dimension + currency 分组，total 与分页稳定 | 是 |
| 过滤 user/project/provider/model/date | 过滤仍 tenant-scoped 且不串租户 | 是 |
| raw usage 含 secret/base64 | 响应递归脱敏 | 是 |
| 非 admin 或缺 `usage:read` | 拒绝读取 | 是 |

### 安全要求

- 所有 usage/cost 查询必须带 tenant_id 过滤。
- 不引入任何 Provider API Key 解密路径。
- `rawUsage` 和 API call metadata 必须继续递归脱敏。
- 不返回 MinIO object keys、bucket names、image base64、Authorization、Cookie、JWT 或 Provider API keys。
- 错误响应保持稳定泛化，不泄露 SQL、pricing 原文、Provider 原始响应。

### 必须新增或更新的回归测试

- 成本估算单元测试：缺失/非法/负数 pricing、多 key 兼容、decimal rounding、currency fallback。
- Worker usage persistence 测试：成功任务在 pricing 异常时仍成功，usage record 成本为零且不重复创建。
- Admin usage summary 测试：`dimension=tenant`、现有维度不回归、多 currency、分页稳定、过滤与跨租户隔离。
- Admin usage records 测试：raw usage redaction 不回归。
- 如果新增 migration/index：数据库 migration 测试。

### 验收标准

- Worker usage cost 估算确定、非负、8 位小数稳定。
- Admin usage summary 支持 tenant/user/project/provider/model 维度并保持 tenant isolation。
- 现有 usage records、operation logs、API call logs 读取不回归。
- 未修改前端、部署、公共合同文档或 Agent 规则。

### 测试命令

```bash
cd backend
go test ./internal/audit ./internal/api ./internal/task ./internal/database -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
git diff --check main...HEAD
docker compose -f deploy/docker-compose.yml config
```

如使用共享本地服务做额外功能验证，必须按 `docs/local-development.md` 使用任务命名空间数据并在交付中记录；本任务默认可以只用自动化测试完成。

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
