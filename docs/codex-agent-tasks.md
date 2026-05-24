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

R12 已完成，未发现阻塞问题。`P13-BE-RUNTIME-DEFAULTS`、`P13-BE-RUNTIME-DEFAULTS-HARDENING`、`P13-BE-CONCURRENCY-POLICY`、`P13-BE-STORAGE-CLEANUP-FOUNDATION`、`P13-BE-STORAGE-RETENTION-RUNTIME` 与 `P13-BE-STORAGE-QUOTA-ACCOUNTING` 已 review 并合并，P13 仍在进行中。

已完成的平台基础：

- P0-P3：文档、monorepo 布局、后端/前端/部署骨架、Docker 运行时修复。
- P4-P6：数据库、租户、认证、RBAC、项目、资产、Provider/模型管理。
- P7-P8：任务队列、Worker、Provider Adapter 运行时、SSE、前端后端化。
- P9-P10：审计/用量读取、运行时生效的上传策略、生产密钥保护、安全/部署验证、Worker 并发池、SSE bridge 生命周期、Provider/模型生命周期、admin 硬化、后端统一历史查询。
- P11-P12：用户/角色管理、统一历史、卖家项目/资产流程和项目最后 `OWNER` 约束。
- P13 已合并切片：租户 `taskDefaults.{defaultProviderId,defaultModelId}` 的读写、任务创建真实消费路径、损坏持久化默认值的 fail-closed hardening、Worker 实际消费的 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`、soft-delete 资产物理清理基础服务、Worker 实际消费的 `storageRetention.deletedAssetRetentionDays`，以及引用图上传/Worker 输出实际消费的 `storageQuota.maxBytes` 与只读 `storageQuota.usedBytes`。

当前已知后续项：

- 前端历史已改为消费 `GET /api/v1/projects/{projectId}/history`。
- 前端项目/资产工作流已完成一轮产品化打磨：项目编辑、资产筛选、资产元数据编辑、项目成员入口、上传切项目 stale 保护和筛选后列表一致性均已合并。
- 后端项目成员写路径已补齐最后一个 `OWNER` 保护：不能删除或降级项目最后一个 `OWNER`，但允许先新增或提升另一个 `OWNER` 后再完成 owner 转移。
- R12 已验证 P12 范围内的卖家工作流、统一历史、项目/资产 UI、项目成员 API、最后 `OWNER` 保护、操作日志、权限边界、前端禁止模式和 Compose 配置。
- 用户/角色管理后端接口和前端管理 UI 已完成；后续租户/团队更深层能力可在新的任务中继续补齐。
- 上传策略、`taskDefaults`、`taskConcurrency`、`storageRetention` 与 `storageQuota` 已有真实运行时消费者；损坏的 `task_defaults`、`task_concurrency`、`storage_retention` 与 `storage_quota` 配置必须 fail closed，不能绕过校验、限流、Provider 执行边界、清理边界或资产写入边界。其他运行时设置在消费者落地前不得暴露为可写配置。
- 下一串行任务只做前端系统设置 UI：展示并编辑已经后端生效的设置，不新增后端合同或未生效设置入口。
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
   - 下一个任务。仅为已经运行时生效的设置提供前端 admin UI。
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

## 下一个任务包：P13-FE-SYSTEM-SETTINGS

### 调度决策

- 本任务串行执行，不与后端 settings、orphan cleanup、log retention 或发布任务并行。
- 理由：前端系统设置页要跟随已经合并的后端运行时设置合同，不能抢先暴露未生效字段。
- 本任务只改前端 admin 设置 UI、前端类型/API 合同和前端测试，不改后端、部署、公共文档或 Agent 规则。

### 任务信息

- 任务名称：`P13-FE-SYSTEM-SETTINGS`
- 目标：扩展现有 admin 设置页，使 tenant admin 能查看和编辑已经 runtime-backed 的设置：`uploadPolicy`、`taskDefaults`、`taskConcurrency`、`storageRetention`、`storageQuota`。继续隐藏 log retention、orphan cleanup、manual cleanup、MinIO listing、Provider secrets 等未开放能力。
- 推荐线程名：`P13-FE-SYSTEM-SETTINGS`
- 推荐分支名：`codex/p13-frontend-system-settings`
- 起始分支：已合并 `P13-BE-STORAGE-QUOTA-ACCOUNTING` 与本任务公共合同文档的最新 `main`
- 前置依赖：`P13-BE-RUNTIME-DEFAULTS`、`P13-BE-CONCURRENCY-POLICY`、`P13-BE-STORAGE-RETENTION-RUNTIME`、`P13-BE-STORAGE-QUOTA-ACCOUNTING` 均已合并；`docs/api-contract.md` 已确认可前端展示/编辑的 active settings 字段。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P13-FE-SYSTEM-SETTINGS`。

你必须在分支 `codex/p13-frontend-system-settings` 上工作；如果当前不在该分支，先执行 `git switch codex/p13-frontend-system-settings`，确认 `git branch --show-current` 后再继续。起始点必须包含最新 `main`，若不包含先停止并报告，不要自行合并公共合同。

任务目标：
扩展现有前端 admin observability/settings 面板，让具备 `system:settings:manage` 的 tenant admin 能查看并编辑后端已经真正 runtime-backed 的设置：
- `uploadPolicy.{maxFileSizeBytes,maxWidth,maxHeight,maxPixels}`
- `taskDefaults.{defaultProviderId,defaultModelId}`
- `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`
- `storageRetention.deletedAssetRetentionDays`
- `storageQuota.maxBytes`，并显示 read-only `storageQuota.usedBytes`

必须遵守：
1. 不修改后端、部署、docs、AGENTS.md、agent-instructions。
2. 不暴露 log retention、orphan cleanup、manual cleanup trigger、MinIO listing、bucket/object key、Provider API Key 或任何未 runtime-backed 的设置。
3. 前端不得直连 OpenAI/Gemini/任何 AI Provider，不得保存 Provider API Key，不得新增轮询、setInterval 或循环 fetch。
4. 所有写请求继续使用现有 `adminApi.updateSystemSettings(request, csrfToken)` 和 CSRF token。
5. PATCH 请求必须只发送当前用户正在保存的设置分组，避免把未编辑字段、read-only 字段或后端额外字段回写。
6. `storageQuota.usedBytes` 永远只读；`storageQuota.maxBytes = null` 与 `storageRetention.deletedAssetRetentionDays = null` 都是合法禁用状态。
7. 如果需要 Provider/model 下拉数据，复用现有 frontend API client，不新增 Provider 直连或敏感字段展示。Provider/model 选择只使用安全展示字段和 ID，不显示 API Key。
8. UI 风格延续现有 `AdminObservabilitySettingsPanel`，不要重写整个 admin 面板，不做无关视觉重构。

建议实现：
1. 更新 `frontend/src/types/admin.ts` 的 `SystemSettings` 与 `UpdateSystemSettingsRequest` 类型，覆盖 active settings 字段，并保持 deferred 字段不存在。
2. 扩展 `frontend/src/components/admin/AdminObservabilitySettingsPanel.tsx`：
   - 保留现有 upload policy 表单行为。
   - 为 task defaults 增加 Provider/model 选择或 ID 输入；保存时成对提交，支持清除为 `null`。
   - 为 task concurrency 增加正整数输入。
   - 为 storage retention 增加正整数输入和清除/禁用状态。
   - 为 storage quota 增加 max bytes 输入、清除/禁用状态，并显示 read-only used bytes。
   - 每个设置分组应有独立保存状态、错误状态和成功后刷新/本地更新，避免一个分组的无效草稿阻塞其他分组。
3. 更新 `frontend/src/api/admin.ts` 与测试，确保 GET/PATCH 路径、CSRF header、请求 body 均正确。
4. 更新 `frontend/src/test/adminObservabilitySettingsPanel.test.tsx`：
   - 验证 active settings 能显示。
   - 验证保存每个分组时只 PATCH 对应分组。
   - 验证 `storageQuota.usedBytes` 不可编辑且不会出现在 PATCH body。
   - 验证 null/clear 行为。
   - 验证 deferred 字段仍不显示、不发送。
   - 验证无 `system:settings:manage` 时不会加载或写 settings。
5. 更新必要的 legacy/forbidden-pattern 测试，确保不引入 Provider 直连、API Key 浏览器存储或 task polling。

验收命令：
```bash
cd frontend
npm run lint
npm run type-check
npm run test -- adminApi adminObservabilitySettingsPanel legacyRetirement
npm run test
npm run build

cd ..
git diff --check main...HEAD
! rg -n "api\\.openai\\.com|generativelanguage|localhost:|127\\.0\\.0\\.1|setInterval\\(|localStorage\\.|sessionStorage\\.|indexedDB|api[_-]?key|Authorization" frontend/src --glob '!**/test/**'
```

最终交付必须包含：
- 修改文件清单。
- 执行的测试命令和结果。
- 每个 failure mode 对应的测试文件/测试名映射。
- 安全自查结果，明确没有新增 Provider 直连、API Key 存储、轮询、未生效设置 UI、object key/bucket/MinIO URL 展示。
- 刻意未修改范围。
- 如发现公共合同缺口，只报告给主 agent，不修改 docs。
```

### 允许修改文件

- `frontend/src/types/admin.ts`
- `frontend/src/api/admin.ts`
- `frontend/src/components/admin/AdminObservabilitySettingsPanel.tsx`
- `frontend/src/components/admin/AdminApiCallLogsView.tsx`，仅限必要的类型/布局兼容；默认不改
- `frontend/src/api/providers.ts`、`frontend/src/api/models.ts`，仅限复用 Provider/model select 所需的安全类型或 helper；默认不改
- `frontend/src/types/platform.ts`，仅限复用安全 Provider/model 展示类型；默认不改
- `frontend/src/test/adminApi.test.ts`
- `frontend/src/test/adminObservabilitySettingsPanel.test.tsx`
- `frontend/src/test/legacyRetirement.test.tsx`
- `frontend/src/test/setup.ts`，仅限测试必要 mock；默认不改

### 禁止修改文件

- `backend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/src/api/tasks.ts`
- `frontend/src/lib/taskSseClient.ts`
- `frontend/src/hooks/useGeneration.ts`
- `frontend/src/db/**`
- Provider/model admin 主流程文件，除非只是复用安全类型且在交付中说明
- 任何 OpenAI/Gemini/AI relay 直连、Provider API Key 浏览器存储、task polling、MinIO public URL、object key/bucket 展示、log retention UI、orphan cleanup UI、manual cleanup UI

### 具体开发内容

1. 先更新或新增前端测试，明确 active settings 与 deferred settings 的边界。
2. 扩展前端 system settings 类型：
   - `uploadPolicy`
   - `taskDefaults`
   - `taskConcurrency`
   - `storageRetention`
   - `storageQuota`
3. 扩展 settings tab UI：
   - 用紧凑工作型布局，不做营销式重排。
   - 每个设置分组独立保存，PATCH body 只包含对应 top-level 字段。
   - 保留 loading、empty、error、disabled、saving 状态。
4. 处理可空字段：
   - task defaults 支持 provider/model 成对设置或成对清除。
   - storage retention 支持设置天数或清除为 `null`。
   - storage quota 支持设置 max bytes 或清除为 `null`，显示 `usedBytes` 和当前状态。
5. 保持已合并的 upload policy UI 行为不回归。
6. 更新 forbidden-pattern 测试或断言，防止重新出现 deferred 字段、Provider 直连、敏感存储、轮询。

### 必须保持的现有行为

- 无 `system:settings:manage` 权限时，settings tab 不加载、不渲染、不写入 settings。
- usage、operation logs、API call logs 三个 observability tab 行为不变。
- Provider/model 管理 UI、用户角色 UI、项目/资产/任务工作台不变。
- 前端任务状态仍只通过 SSE，不新增 polling。
- 前端不保存 Provider API Key，不展示后端脱敏之外的 secret。

### 允许的中间态

- 前端可展示 active settings，但更深层的成本看板、orphan cleanup、log retention、thumbnail policy 继续留给后续任务。
- task defaults 可以先用安全 Provider/model 选择器；如果现有 API 不足以提供选择数据，可使用 ID 输入并在交付中说明限制，不能新增后端合同。

### 禁止的半迁移状态

- UI 展示某个设置但保存时没有调用真实后端 `PATCH /admin/system-settings`。
- PATCH 回写整个 GET response，导致 `usedBytes`、未知字段或 deferred 字段被发送。
- 展示或提交 `logRetention`、`orphanCleanup`、`manualCleanup`、`allowedMimeTypes`、`storageQuotaBytes` 等未开放字段。
- 因保存一个分组，把另一个分组的无效草稿或旧值一起发送。
- 引入 Provider 直连、Provider Key 存储、task polling、object key/bucket/MinIO URL 暴露。

### 失败模式与边界场景

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| 用户无 settings 权限 | 不加载 settings，不显示 settings tab，不发 PATCH | 是 |
| GET 返回全部 active settings | UI 正确显示 upload policy、defaults、concurrency、retention、quota | 是 |
| 保存 upload policy | 只 PATCH `uploadPolicy` | 是 |
| 保存 task defaults | 只 PATCH `taskDefaults`；provider/model 成对提交或成对清除 | 是 |
| 保存 task concurrency | 只 PATCH `taskConcurrency` 正整数字段 | 是 |
| 保存 storage retention | 支持正整数和 `null` 清除 | 是 |
| 保存 storage quota | 只 PATCH `storageQuota.maxBytes`；`usedBytes` 不可写不发送 | 是 |
| backend validation error | 保留当前分组草稿并显示错误，不污染其他分组 | 是 |
| deferred/legacy 字段出现在 mock response | 不显示、不发送 | 是 |

### 安全要求

- 所有 settings 写请求必须带 CSRF token。
- 不在 localStorage、sessionStorage、IndexedDB 或源码中保存 Provider API Key。
- 不显示 bucket、object key、MinIO URL、Authorization、Cookie、JWT、Provider API Key、图片 base64。
- 不新增 setInterval、循环 fetch 或 task polling。
- 不新增任何 AI Provider 直连 URL。

### 必须新增或更新的回归测试

- `frontend/src/test/adminApi.test.ts`：系统设置类型和 PATCH body 覆盖 active settings，证明 CSRF header 保持存在。
- `frontend/src/test/adminObservabilitySettingsPanel.test.tsx`：覆盖五个 active settings 分组、只 PATCH 当前分组、null clear、read-only usedBytes、权限 gating、backend error 保留草稿、deferred 字段不显示不发送。
- `frontend/src/test/legacyRetirement.test.tsx` 或等价 forbidden-pattern 测试：确认没有 Provider 直连、API Key 存储、task polling、legacy/deferred settings 重新出现。

### 验收标准

- Admin settings tab 能查看并编辑所有已 runtime-backed 设置。
- 每个保存动作只发送一个 top-level settings patch，且不会发送 read-only/deferred 字段。
- `storageQuota.usedBytes` 只读展示；`storageQuota.maxBytes = null` 和 `storageRetention.deletedAssetRetentionDays = null` 能表达 disabled 状态。
- 前端权限、CSRF、错误态、loading 态和现有 observability tab 行为不回归。
- 未修改后端、部署、公共合同文档或 Agent 规则。

### 测试命令

```bash
cd frontend
npm run lint
npm run type-check
npm run test -- adminApi adminObservabilitySettingsPanel legacyRetirement
npm run test
npm run build

cd ..
git diff --check main...HEAD
! rg -n "api\\.openai\\.com|generativelanguage|setInterval\\(|localStorage\\.|sessionStorage\\.|api[_-]?key|Authorization" frontend/src --glob '!**/test/**'
```

如使用共享本地服务做额外前后端功能验证，必须按 `docs/local-development.md` 使用任务命名空间数据并在交付中记录；本任务默认不需要写共享 MySQL/Redis/MinIO 数据。

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
