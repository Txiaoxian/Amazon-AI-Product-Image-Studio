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

R12 已完成，未发现阻塞问题。`P13-BE-RUNTIME-DEFAULTS`、`P13-BE-RUNTIME-DEFAULTS-HARDENING`、`P13-BE-CONCURRENCY-POLICY`、`P13-BE-STORAGE-CLEANUP-FOUNDATION` 与 `P13-BE-STORAGE-RETENTION-RUNTIME` 已 review 并合并，P13 仍在进行中。

已完成的平台基础：

- P0-P3：文档、monorepo 布局、后端/前端/部署骨架、Docker 运行时修复。
- P4-P6：数据库、租户、认证、RBAC、项目、资产、Provider/模型管理。
- P7-P8：任务队列、Worker、Provider Adapter 运行时、SSE、前端后端化。
- P9-P10：审计/用量读取、运行时生效的上传策略、生产密钥保护、安全/部署验证、Worker 并发池、SSE bridge 生命周期、Provider/模型生命周期、admin 硬化、后端统一历史查询。
- P11-P12：用户/角色管理、统一历史、卖家项目/资产流程和项目最后 `OWNER` 约束。
- P13 已合并切片：租户 `taskDefaults.{defaultProviderId,defaultModelId}` 的读写、任务创建真实消费路径、损坏持久化默认值的 fail-closed hardening、Worker 实际消费的 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`、soft-delete 资产物理清理基础服务，以及 Worker 实际消费的 `storageRetention.deletedAssetRetentionDays`。

当前已知后续项：

- 前端历史已改为消费 `GET /api/v1/projects/{projectId}/history`。
- 前端项目/资产工作流已完成一轮产品化打磨：项目编辑、资产筛选、资产元数据编辑、项目成员入口、上传切项目 stale 保护和筛选后列表一致性均已合并。
- 后端项目成员写路径已补齐最后一个 `OWNER` 保护：不能删除或降级项目最后一个 `OWNER`，但允许先新增或提升另一个 `OWNER` 后再完成 owner 转移。
- R12 已验证 P12 范围内的卖家工作流、统一历史、项目/资产 UI、项目成员 API、最后 `OWNER` 保护、操作日志、权限边界、前端禁止模式和 Compose 配置。
- 用户/角色管理后端接口和前端管理 UI 已完成；后续租户/团队更深层能力可在新的任务中继续补齐。
- 上传策略、`taskDefaults`、`taskConcurrency` 与 `storageRetention` 已有真实运行时消费者；损坏的 `task_defaults`、`task_concurrency` 与 `storage_retention` 配置必须 fail closed，不能绕过校验、限流、Provider 执行边界或清理边界。其他运行时设置在消费者落地前不得暴露为可写配置。
- 下一串行任务只做后端存储配额统计与执行：增加 nullable `storageQuota.maxBytes` 设置、read-only `storageQuota.usedBytes`，并让引用图上传和 Worker 输出资产持久化真实消费该配额。
- 缩略图策略、完整 orphan cleanup、log retention 和前端设置 UI 仍需实现。
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
   - 下一个任务。增加 nullable `storageQuota.maxBytes`、read-only `storageQuota.usedBytes`，并在引用图上传和 Worker 输出资产持久化前执行配额校验。
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

## 下一个任务包：P13-BE-STORAGE-QUOTA-ACCOUNTING

### 调度决策

- 本任务串行执行，不与前端设置、orphan cleanup、log retention 或发布任务并行。
- 理由：`storageQuota` 是会拒绝资产写入的控制面设置，必须和真实上传/Worker 输出消费者同步落地，且需要完整 failure matrix。
- 本任务只开放 nullable quota 上限和 read-only usage，不开放 log retention、orphan object listing、frontend UI、manual cleanup API 或 MinIO bucket listing。

### 任务信息

- 任务名称：`P13-BE-STORAGE-QUOTA-ACCOUNTING`
- 目标：增加 runtime-backed nullable `storageQuota.maxBytes` 设置和 read-only `storageQuota.usedBytes` 统计，并在引用图上传与 Worker 输出资产持久化前执行 tenant-scoped quota 校验。
- 推荐线程名：`P13-BE-STORAGE-QUOTA-ACCOUNTING`
- 推荐分支名：`codex/p13-backend-storage-quota-accounting`
- 起始分支：已合并 `P13-BE-STORAGE-RETENTION-RUNTIME` 与本任务公共合同文档的最新 `main`
- 前置依赖：`P13-BE-STORAGE-RETENTION-RUNTIME` 已合并；`docs/api-contract.md`、`docs/database-schema.md`、`docs/storage.md` 已冻结 `storageQuota.maxBytes` / `storageQuota.usedBytes` 合同和资产写入消费者范围。

### 控制面字段与运行时消费者映射

| 外部字段 | 运行时消费者 | 本任务是否实现消费者 |
| --- | --- | --- |
| `storageQuota.maxBytes` | 引用图上传和 Worker 输出资产持久化前读取 tenant setting，计算 `usedBytes + pendingBytes <= maxBytes` | 是 |
| `storageQuota.usedBytes` | 只读 API 字段，从 tenant-scoped `image_assets.size_bytes` 计算，`purged_at IS NULL` 计入 | 是，只读 |
| `storageRetention.deletedAssetRetentionDays` | 已合并 Worker maintenance consumer | 保持现有行为，不改语义 |
| `logRetention.*` | 暂无；后续日志清理任务 | 否，本任务禁止暴露 |

### 允许修改文件

- `backend/internal/asset/**`
- `backend/internal/settings/**`
- `backend/internal/task/**`，仅限 Worker 输出资产持久化前 quota 校验、错误映射和相关测试；不得修改队列/SSE/状态机语义
- `backend/internal/database/**`，仅限 `image_assets` quota 聚合所需 model/helper/index/migration 测试；默认不得新增 quota 表，除非实现严格并发需要并在交付中说明
- `backend/internal/config/**`，仅限 storage quota 上限/hard cap 配置（如确有必要）；不得新增 Provider 或任务状态配置
- `backend/internal/api/*system_settings*_test.go`
- `backend/internal/asset/*_test.go`
- `backend/internal/database/*_test.go`
- `backend/internal/task/*_test.go`
- `backend/internal/api/asset_routes_test.go`
- `backend/internal/api/task_routes_test.go`
- `backend/internal/api/router.go`，仅限 settings/quota service wiring 所必需的最小修改

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
- 任何新的 public cleanup trigger API、独立 storage quota API、log retention API 或 MinIO object listing API
- 前端 UI、部署拓扑、Provider/model/task/SSE 行为或无关重构
- 删除现有资产、硬删除 `image_assets` 行、修改已合并 retention cleanup 语义、或绕过 `tenant_id` 的 quota 查询

### 具体开发内容

1. 先写 settings API、asset upload、Worker output persistence 和 quota failure matrix 测试，再做最小实现。
2. 在 `GET/PATCH /api/v1/admin/system-settings` 中增加 `storageQuota`：
   - 响应形态：`storageQuota: { "maxBytes": null | positive_integer, "usedBytes": non_negative_integer }`
   - `maxBytes = null` 表示不启用 storage quota；没有 tenant override 时默认返回 `null`。
   - `usedBytes` 只读，PATCH 中出现必须返回 `422 VALIDATION_ERROR`。
   - PATCH 可设置正整数 `maxBytes` 或 `null` 清除；省略字段保持当前值。
   - 建议合法范围：`1..109951162777600` bytes（100 TiB）或等价明确 hard cap；超出、零值、负值、小数、字符串、未知字段均返回 `422 VALIDATION_ERROR`。
3. 实现 tenant-scoped usage 计算：
   - 从 `image_assets.size_bytes` 聚合，必须带 `tenant_id`。
   - `purged_at IS NULL` 的资产计入；soft-deleted 但未 purged 的资产仍计入；`purged_at IS NOT NULL` 不计入。
   - 不使用 MinIO bucket listing 作为 quota 统计来源。
4. 引用图上传 quota consumer：
   - 在成功创建资产 metadata 前校验 `usedBytes + uploadSize <= maxBytes`。
   - 超额返回 `409 STORAGE_QUOTA_EXCEEDED` 或等价稳定错误码；不得创建 asset row、成功 operation log 或残留已上传对象。
   - `maxBytes = null` 时不改变现有上传行为。
5. Worker 输出资产 quota consumer：
   - 在 Provider 输出图片校验后、成功持久化 output asset metadata 前校验本次待写入输出总 bytes。
   - 超额时任务必须以 sanitized failure 结束或按现有 Worker 错误合同处理；不得创建 output asset row、task output row、image-output success event 或成功 usage/output side effects。
   - 已上传但 DB 持久化失败或 quota 失败的对象必须按现有 cleanup pattern 清理，不泄漏 object key。
6. 操作日志只记录 settings key、changed fields、quota max/cleared 状态和只读 usage 数字；不得记录 raw JSON、bucket、object key、MinIO URL、图片 base64 或内部错误栈。
7. 不新增前端 UI；前端设置页是否展示 quota 留给 `P13-FE-SYSTEM-SETTINGS`。

### 必须保持的现有行为

- `uploadPolicy`、`taskDefaults`、`taskConcurrency`、`storageRetention` 的 API、运行时消费、hardening 语义不变。
- 已合并的 upload rollback cleanup、retention maintenance 和 `asset.CleanupService` tenant/cutoff/batch/idempotency 行为不变。
- Worker 任务 claim、Redis lease、Provider execution、SSE、task events、outputs、usage、API-call logs、cancel/retry/timeout/recovery 状态机不变。
- 资产下载、详情、列表、收藏、软删、项目权限和 object-level authorization 行为不变。
- MySQL 仍只保存 metadata/object key，不保存图片 blob；MinIO bucket 创建仍是环境/部署责任。

### 允许的中间态

- 后端 API、引用图上传和 Worker 输出资产持久化已支持 `storageQuota`，但前端 admin 设置页仍暂不展示。
- `storageQuota.maxBytes = null` 是合法禁用状态，不会拒绝资产写入。
- Orphan object discovery、log retention、thumbnail size accounting 和 frontend settings UI 继续留给后续任务。

### 禁止的半迁移状态

- 暴露 `storageQuota.maxBytes` 但引用图上传或 Worker 输出资产持久化不消费。
- 默认启用 quota。无 override 必须是 unlimited/null，不能突然拒绝现有租户写入。
- 将 `usedBytes` 做成可写字段，或把 quota 状态存成第二真相源而不从 `image_assets` 计算。
- 只限制手动上传、不限制 Worker 输出资产，或只限制 Worker、不限制手动上传。
- 因 quota 设置删除、隐藏、硬删除或自动 purge 现有资产。
- 暴露 log retention、orphan cleanup、manual purge API、MinIO object listing API 或前端 UI。
- 日志、响应或 operation log 泄漏 object key、bucket、MinIO URL、Authorization、Cookie、API Key、图片 base64 或内部错误栈。

### 失败模式与边界场景

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| GET 无 `storage_quota` row | 返回 `maxBytes: null` 和当前 computed `usedBytes`；写入不受 quota 限制 | 是 |
| admin PATCH 设置合法 maxBytes | 当前 tenant 生效，写脱敏 operation log；后续上传/Worker 输出使用该值 | 是 |
| admin PATCH 清除为 `null` | 当前 tenant 禁用 quota；后续资产写入恢复现有行为 | 是 |
| PATCH 包含 `usedBytes`、未知字段、零值、负值、小数、字符串、超范围 | `422 VALIDATION_ERROR`；原设置和 operation log 不变 | 是 |
| non-admin、缺 CSRF、跨 tenant 探测 | 既有 `403`/授权行为；不得读写其他 tenant quota | 是 |
| 手工损坏 `storage_quota.value_json` | API read/write 返回 sanitized failure 或按既有 settings 错误语义处理；资产写入 fail closed | 是 |
| usedBytes 统计遇到 active、soft-deleted、purged、cross-tenant rows | active/soft-deleted 未 purged 计入，purged 和跨租户不计入 | 是 |
| 引用图上传在 quota 内 | 行为与现有上传一致，创建 asset 和 operation log | 是 |
| 引用图上传超过 quota | 返回稳定 quota 错误；不创建 asset row/成功 log；不残留对象 | 是 |
| Worker 输出资产在 quota 内 | 行为与现有输出持久化一致 | 是 |
| Worker 输出资产超过 quota | 不创建 output asset/task output/image-output success event；任务按现有失败合同结束，错误脱敏 | 是 |
| `maxBytes` 小于当前 `usedBytes` | 允许保存但不删除现有资产；后续新增资产写入被拒绝直到低于 quota 或清除 quota | 是 |
| settings storage failure | 不绕过 quota；上传/Worker 写入返回/进入可恢复或失败路径且不创建成功副作用 | 是 |

### 安全要求

- `storageQuota` 只能由 tenant admin 且具备 `system:settings:manage` 的用户修改，写请求继续走 CSRF。
- Quota usage 必须以 tenant-scoped MySQL metadata 为来源，不能从浏览器请求、MinIO listing 或对象 key 输入推导。
- 所有设置读取、usage 聚合、quota 校验和 asset metadata 写入必须带 tenant 边界。
- Quota failure 不得泄漏当前 object key、bucket、MinIO URL、Provider payload、图片 base64 或内部错误栈。
- 不公开 bucket、object key、MinIO URL、图片 base64、Authorization、Cookie、JWT、Provider API Key 或内部错误栈。
- 不引入 frontend Provider 直连、轮询、浏览器敏感存储或 public object URL。

### 必须新增或更新的回归测试

- `backend/internal/api/*system_settings*_test.go` 覆盖 GET computed usedBytes、合法 PATCH、clear null、RBAC/CSRF、非法/未知字段、`usedBytes` 不可写、脱敏 operation log。
- `backend/internal/settings/**` 测试覆盖 `storage_quota` JSON 解析、nullable 语义、非法持久化 fail closed、tenant 隔离和 maxBytes 范围。
- `backend/internal/asset/**` 和/或 `backend/internal/api/asset_routes_test.go` 覆盖引用图上传 quota 内/超额、soft-deleted 未 purged 计入、purged 不计入、跨租户不计入、失败不残留对象。
- `backend/internal/task/**` 测试覆盖 Worker 输出资产 quota 内/超额、批量输出总 bytes、失败无 output asset/task output/image-output success event、错误脱敏。
- 全量后端测试证明 upload policy、task defaults、task concurrency、storage retention、SSE、Provider runtime、task outputs、usage/API logs 不回归。

### 验收标准

- `storageQuota.maxBytes` 是唯一新增的公开可写设置字段，且有引用图上传和 Worker 输出资产两个 runtime consumers。
- `storageQuota.usedBytes` 是只读 computed 字段，统计口径为 tenant-scoped、`purged_at IS NULL` 的 `image_assets.size_bytes`。
- 无 override 时 quota 默认禁用；设置合法 maxBytes 后新增资产写入按 tenant quota 校验。
- 损坏设置、settings 存储错误、quota 超额或 Worker 输出失败均不能导致跨 tenant 写入、成功副作用、对象泄漏或敏感信息泄漏。
- 未新增 log retention、orphan cleanup、frontend UI、manual cleanup API、MinIO listing API 或无关范围修改。

### 测试命令

```bash
cd backend
go test ./internal/api ./internal/settings ./internal/asset ./internal/database ./internal/task -count=1
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
