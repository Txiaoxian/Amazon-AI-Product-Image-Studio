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

R12 已完成，未发现阻塞问题。`P13-BE-RUNTIME-DEFAULTS`、`P13-BE-RUNTIME-DEFAULTS-HARDENING`、`P13-BE-CONCURRENCY-POLICY` 与 `P13-BE-STORAGE-CLEANUP-FOUNDATION` 已 review 并合并，P13 仍在进行中。

已完成的平台基础：

- P0-P3：文档、monorepo 布局、后端/前端/部署骨架、Docker 运行时修复。
- P4-P6：数据库、租户、认证、RBAC、项目、资产、Provider/模型管理。
- P7-P8：任务队列、Worker、Provider Adapter 运行时、SSE、前端后端化。
- P9-P10：审计/用量读取、运行时生效的上传策略、生产密钥保护、安全/部署验证、Worker 并发池、SSE bridge 生命周期、Provider/模型生命周期、admin 硬化、后端统一历史查询。
- P11-P12：用户/角色管理、统一历史、卖家项目/资产流程和项目最后 `OWNER` 约束。
- P13 已合并切片：租户 `taskDefaults.{defaultProviderId,defaultModelId}` 的读写、任务创建真实消费路径、损坏持久化默认值的 fail-closed hardening、Worker 实际消费的 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`，以及 soft-delete 资产物理清理基础服务。

当前已知后续项：

- 前端历史已改为消费 `GET /api/v1/projects/{projectId}/history`。
- 前端项目/资产工作流已完成一轮产品化打磨：项目编辑、资产筛选、资产元数据编辑、项目成员入口、上传切项目 stale 保护和筛选后列表一致性均已合并。
- 后端项目成员写路径已补齐最后一个 `OWNER` 保护：不能删除或降级项目最后一个 `OWNER`，但允许先新增或提升另一个 `OWNER` 后再完成 owner 转移。
- R12 已验证 P12 范围内的卖家工作流、统一历史、项目/资产 UI、项目成员 API、最后 `OWNER` 保护、操作日志、权限边界、前端禁止模式和 Compose 配置。
- 用户/角色管理后端接口和前端管理 UI 已完成；后续租户/团队更深层能力可在新的任务中继续补齐。
- 上传策略、`taskDefaults` 与 `taskConcurrency` 已有真实运行时消费者；损坏的 `task_defaults` 与 `task_concurrency` 配置必须 fail closed，不能绕过校验、限流或 Provider 执行边界。其他运行时设置在消费者落地前不得暴露为可写配置。
- 下一串行任务只做后端存储保留运行时：增加 nullable `storageRetention.deletedAssetRetentionDays` 设置，并让 Worker maintenance loop 真实消费该设置。
- 存储配额、缩略图策略、完整 orphan cleanup 和前端设置 UI 仍需实现。
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
   - 增加 nullable `storageRetention.deletedAssetRetentionDays` 设置，并让 Worker maintenance loop 消费该设置调用 cleanup foundation。
   - 不开放 storage quota、log retention、orphan object listing 或 frontend settings。
6. `P13-BE-STORAGE-QUOTA-ACCOUNTING`
   - 存储用量统计与配额执行；完整 orphan cleanup 需要在有明确对象枚举/命名空间边界后再实现。
7. `P13-FE-SYSTEM-SETTINGS`
   - 仅为已经运行时生效的设置提供前端 admin UI。
8. `R13`
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

## 下一个任务包：P13-BE-STORAGE-RETENTION-RUNTIME

### 调度决策

- 本任务串行执行，不与前端设置、存储配额、orphan cleanup 或发布任务并行。
- 理由：`storageRetention` 是会触发物理删除的控制面设置，必须和真实 Worker maintenance consumer 同步落地，且需要完整 failure matrix。
- 本任务只开放一个 nullable retention 字段，不开放 storage quota、log retention、orphan object listing、frontend UI 或公开 cleanup API。

### 任务信息

- 任务名称：`P13-BE-STORAGE-RETENTION-RUNTIME`
- 目标：增加 runtime-backed `storageRetention.deletedAssetRetentionDays` 设置，并在 Worker 进程中增加可停止的 maintenance loop 消费该设置，按租户调用已合并的 cleanup foundation 物理清理 soft-deleted 资产。
- 推荐线程名：`P13-BE-STORAGE-RETENTION-RUNTIME`
- 推荐分支名：`codex/p13-backend-storage-retention-runtime`
- 起始分支：已合并 `P13-BE-STORAGE-CLEANUP-FOUNDATION` 与本任务公共合同文档的最新 `main`
- 前置依赖：`P13-BE-STORAGE-CLEANUP-FOUNDATION` 已合并；`docs/api-contract.md`、`docs/database-schema.md`、`docs/storage.md` 已冻结 `storageRetention.deletedAssetRetentionDays` 的 nullable 设置和 Worker 消费合同。

### 控制面字段与运行时消费者映射

| 外部字段 | 运行时消费者 | 本任务是否实现消费者 |
| --- | --- | --- |
| `storageRetention.deletedAssetRetentionDays` | Worker maintenance loop 读取 tenant setting，计算 `cutoff = now - days`，调用 `asset.CleanupService.PurgeDeletedAssets` | 是 |
| `storageQuota.*` | 暂无；后续 quota accounting/enforcement | 否，本任务禁止暴露 |
| `logRetention.*` | 暂无；后续日志清理任务 | 否，本任务禁止暴露 |

### 允许修改文件

- `backend/internal/asset/**`
- `backend/internal/settings/**`
- `backend/internal/database/**`，仅限补齐 `ImageAsset.PurgedAt` model 字段、tenant/settings 查询所需最小 repository/helper 和测试；不得新增 quota 表
- `backend/internal/config/**`，仅限 Worker retention maintenance interval、batch limit、range bounds 等后端内部配置；不得新增 Provider 或 task 状态配置
- `backend/cmd/worker/**`
- `backend/internal/api/*system_settings*_test.go`
- `backend/internal/asset/*_test.go`
- `backend/internal/database/*_test.go`
- `backend/internal/api/router.go`，仅限 settings service wiring 所必需的最小修改

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/internal/task/**`，除非只改测试 helper 且不触碰任务状态机；默认不允许
- `backend/internal/provider/**`
- `backend/internal/provideradapter/**`
- `backend/internal/model/**`
- `backend/internal/queue/**`
- `backend/internal/sse/**`
- 任何新的 public cleanup trigger API、storage quota API、log retention API 或 MinIO object listing API
- 前端 UI、部署拓扑、Provider/model/task/SSE 行为或无关重构
- 硬删除 `image_assets` 行、删除未 soft-delete 的对象、或绕过 `tenant_id` 的清理

### 具体开发内容

1. 先写 settings API、Worker maintenance 和 cleanup failure matrix 测试，再做最小实现。
2. 补齐 `database.ImageAsset` 的 nullable `PurgedAt` 字段，使 GORM model 与已合并 migration 对齐；不新增新 migration，除非发现当前 migration 无法在真实 MySQL 上执行。
3. 在 `GET/PATCH /api/v1/admin/system-settings` 中增加 `storageRetention`：
   - 响应形态：`storageRetention: { "deletedAssetRetentionDays": null | positive_integer }`
   - `null` 表示禁用自动物理清理；没有 tenant override 时默认返回 `null`。
   - PATCH 可设置正整数或 `null` 清除；省略字段保持当前值。
   - 建议合法范围：`1..3650` 天，超出、零值、负值、小数、字符串、未知字段均返回 `422 VALIDATION_ERROR`。
4. Worker 进程增加 retention maintenance loop：
   - 随 Worker 生命周期启动和停止，尊重 context cancellation 和 shutdown timeout。
   - 只扫描有 non-null `storage_retention` 配置且配置合法的 tenant。
   - 对每个 tenant 计算 cutoff，并调用 `asset.CleanupService.PurgeDeletedAssets(ctx, tenantID, cutoff, PurgeOptions{BatchLimit: ...})`。
   - 单个 tenant 清理失败不得阻塞其他 tenant；错误日志必须脱敏，只记录 tenant、计数、error_kind。
   - 持久化设置损坏时 fail closed：跳过该 tenant cleanup，记录脱敏错误，不做任何删除。
5. 操作日志只记录 settings key、changed fields 和非敏感 retention 天数或 cleared 状态；不得记录 raw JSON、bucket、object key、MinIO URL、图片 base64 或内部错误栈。
6. 不新增前端 UI；前端设置页是否展示该字段留给 `P13-FE-SYSTEM-SETTINGS`。

### 必须保持的现有行为

- `uploadPolicy`、`taskDefaults`、`taskConcurrency` 的 API、运行时消费、hardening 语义不变。
- 已合并的 upload rollback cleanup 和 `asset.CleanupService` tenant/cutoff/batch/idempotency 行为不变。
- Worker 任务 claim、Redis lease、Provider execution、SSE、task events、outputs、usage、API-call logs、cancel/retry/timeout/recovery 状态机不变。
- 资产下载、详情、列表、收藏、软删、项目权限和 object-level authorization 行为不变。
- MySQL 仍只保存 metadata/object key，不保存图片 blob；MinIO bucket 创建仍是环境/部署责任。

### 允许的中间态

- 后端 API 和 Worker 已支持 `storageRetention`，但前端 admin 设置页仍暂不展示。
- `storageRetention.deletedAssetRetentionDays = null` 是合法禁用状态，不会触发任何自动物理删除。
- Storage quota、orphan object discovery 和 log retention 继续留给后续任务。

### 禁止的半迁移状态

- 暴露 `storageRetention` 但 Worker 不消费，或 Worker 使用另一个未受 settings/RBAC/tenant 约束的配置源。
- 默认启用自动物理删除。无 override 必须是 disabled/null，不能突然删除历史 soft-deleted 资产。
- 暴露 storage quota、log retention、orphan cleanup、manual purge API 或前端 UI。
- 清理未 soft-delete、未到 cutoff、已 purged 或跨 tenant 的对象。
- 清理失败仍标记 `purged_at`，或用 hard-delete DB 行掩盖对象删除失败。
- 日志、响应或 operation log 泄漏 object key、bucket、MinIO URL、Authorization、Cookie、API Key、图片 base64 或内部错误栈。

### 失败模式与边界场景

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| GET 无 `storage_retention` row | 返回 `deletedAssetRetentionDays: null`；Worker 不清理 | 是 |
| admin PATCH 设置合法天数 | 当前 tenant 生效，写脱敏 operation log；Worker 下一轮使用该天数计算 cutoff | 是 |
| admin PATCH 清除为 `null` | 当前 tenant 禁用自动清理；Worker 跳过 | 是 |
| non-admin、缺 CSRF、跨 tenant 探测 | 既有 `403`/授权行为；不得读写其他 tenant 设置 | 是 |
| 零值、负值、小数、字符串、超范围、未知字段、`storageQuota` 字段 | `422 VALIDATION_ERROR`；原设置和 operation log 不变 | 是 |
| 手工损坏 `storage_retention.value_json` | API GET/PATCH 返回 sanitized failure 或按既有 settings 错误语义处理；Worker fail closed 跳过删除 | 是 |
| Worker loop context canceled/shutdown | 停止新一轮 cleanup，退出不阻塞正常 shutdown | 是 |
| 单 tenant cleanup 失败 | 记录脱敏错误，继续其他 tenant；失败 tenant 后续可重试 | 是 |
| tenant A 设置 retention，tenant B 无设置或设置无效 | 只清理 tenant A 符合条件的 soft-deleted asset | 是 |
| cleanup foundation 原有 not-found/idempotency/storage error | 行为不回退 | 回归现有测试 |

### 安全要求

- `storageRetention` 只能由 tenant admin 且具备 `system:settings:manage` 的用户修改，写请求继续走 CSRF。
- Worker 必须以 tenant-scoped metadata 为删除来源，不能从请求或设置中接收 object key。
- 所有设置读取、tenant 枚举、cleanup 查询和 purge update 必须带 tenant 边界。
- 不公开 bucket、object key、MinIO URL、图片 base64、Authorization、Cookie、JWT、Provider API Key 或内部错误栈。
- 不引入 frontend Provider 直连、轮询、浏览器敏感存储或 public object URL。

### 必须新增或更新的回归测试

- `backend/internal/api/*system_settings*_test.go` 覆盖 GET fallback null、合法 PATCH、clear null、RBAC/CSRF、非法/未知字段、脱敏 operation log。
- `backend/internal/settings/**` 测试覆盖 `storage_retention` JSON 解析、nullable 语义、非法持久化 fail closed、tenant 隔离。
- `backend/cmd/worker/**` 或新建后端 maintenance 测试覆盖 loop 启停、只处理有合法 retention 的 tenant、单 tenant 失败继续、context cancel。
- `backend/internal/asset/**` 回归 cleanup foundation：tenant/cutoff/batch/idempotency、not-found success、storage error retry、`purged_at` 不误标。
- 全量后端测试证明任务 Worker 主流程、SSE、Provider runtime、task outputs、usage/API logs 不回归。

### 验收标准

- `storageRetention.deletedAssetRetentionDays` 是唯一新增的公开设置字段，且有 Worker runtime consumer。
- 无 override 时自动物理清理默认禁用；设置合法天数后 Worker 按 tenant/cutoff/batch 调用 cleanup foundation。
- 损坏设置、settings 存储错误、cleanup 错误或 Worker shutdown 均不能导致跨 tenant 删除、未到期删除、未 soft-delete 删除或敏感信息泄漏。
- 未新增 storage quota、log retention、orphan cleanup、frontend UI、manual cleanup API 或无关范围修改。

### 测试命令

```bash
cd backend
go test ./internal/api ./internal/settings ./internal/asset ./internal/database ./cmd/worker -count=1
go test ./... -count=1
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
git diff --check main...HEAD
```

如使用共享本地 MySQL、Redis、MinIO 做额外功能验证，必须按 `docs/local-development.md` 使用任务命名空间数据和对象前缀，并在交付中记录创建、删除和残留情况；本任务不得另起项目专属依赖容器，不得删除共享 bucket 或无关对象。

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
