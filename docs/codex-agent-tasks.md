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

P0-P18 已完成。P19 已完成生产配置 fail-closed、CI 门禁、前端依赖审计修复与持续门禁、主机 TLS Nginx 模板/静态检查、前端 `logRetention` 设置和既有租户内置角色授权 reconciliation。P20 后端与部署切片已完成：固定 `X-CSRF-Token` 合同、Provider 主密钥轮换 CLI、第二租户开通 CLI、当前租户资料 API、自定义角色 CRUD/权限替换、备份恢复回滚演练。当前仅剩前端租户/自定义角色管理和最终 Go/No-Go。

已完成的平台基础：

- P0-P3：文档、monorepo 布局、后端/前端/部署骨架、Docker 运行时修复。
- P4-P6：数据库、租户、认证、RBAC、项目、资产、Provider/模型管理。
- P7-P8：任务队列、Worker、Provider Adapter 运行时、SSE、前端后端化。
- P9-P10：审计/用量读取、运行时生效的上传策略、生产密钥保护、安全/部署验证、Worker 并发池、SSE bridge 生命周期、Provider/模型生命周期、admin 硬化、后端统一历史查询。
- P11-P12：用户/角色管理、统一历史、卖家项目/资产流程和项目最后 `OWNER` 约束。
- P13 已合并切片：租户 `taskDefaults.{defaultProviderId,defaultModelId}` 的读写、任务创建真实消费路径、损坏持久化默认值的 fail-closed hardening、Worker 实际消费的 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`、soft-delete 资产物理清理基础服务、Worker 实际消费的 `storageRetention.deletedAssetRetentionDays`、引用图上传/Worker 输出实际消费的 `storageQuota.maxBytes` 与只读 `storageQuota.usedBytes`，以及只展示 active runtime-backed settings 的前端 admin 设置页。
- P14 已合并切片：Provider/model 生命周期完整性、后端确定性 usage/cost reporting、前端 admin 成本可观测性和 R14。Worker 成本估算使用稳定 decimal，非法 pricing 归零且不失败成功任务；admin usage summary 支持 tenant/user/project/Provider/model 维度、tenant isolation、多币种分组和 exact decimal cost；前端 usage tab 支持 tenant totals、过滤、drilldown、多币种展示和 stale response 防护。
- P15 已合并切片：`P15-E2E-CORE-FLOWS`、`P15-SECURITY-FINAL-REGRESSION` 和 `P15-DEPLOY-RUNBOOK-FINAL`。后端集成测试已串联 init-admin、Provider/model、project、reference upload、task create、fake Worker success、SSE replay、output asset download、history、usage records/summary、operation logs 和 API call logs，且不调用真实 AI Provider；安全回归脚本已覆盖 focused tests、forbidden-pattern scans、敏感日志扫描、Compose config 和 `/api/` 代理安全检查；部署脚本和 runbook 已覆盖 Compose config/build/up/health/proxy/down cleanup、init-admin、MinIO bootstrap、SSE proxy、backup/restore、upgrade/rollback、日志排查和清理。

当前已知后续项：

- 前端历史已改为消费 `GET /api/v1/projects/{projectId}/history`。
- 前端项目/资产工作流已完成一轮产品化打磨：项目编辑、资产筛选、资产元数据编辑、项目成员入口、上传切项目 stale 保护和筛选后列表一致性均已合并。
- 后端项目成员写路径已补齐最后一个 `OWNER` 保护：不能删除或降级项目最后一个 `OWNER`，但允许先新增或提升另一个 `OWNER` 后再完成 owner 转移。
- R12 已验证 P12 范围内的卖家工作流、统一历史、项目/资产 UI、项目成员 API、最后 `OWNER` 保护、操作日志、权限边界、前端禁止模式和 Compose 配置。
- 用户/角色管理后端接口和前端管理 UI 已完成；后续租户/团队更深层能力可在新的任务中继续补齐。
- 上传策略、`taskDefaults`、`taskConcurrency`、`storageRetention` 与 `storageQuota` 已有真实运行时消费者；损坏的 `task_defaults`、`task_concurrency`、`storage_retention` 与 `storage_quota` 配置必须 fail closed，不能绕过校验、限流、Provider 执行边界、清理边界或资产写入边界。其他运行时设置在消费者落地前不得暴露为可写配置。
- P16 已完成切片：`P16-DEPLOY-SCRIPT-HARDENING`、`P16-BE-LOG-RETENTION` 和 `P16-BE-THUMBNAIL-POLICY`。`scripts/deploy-release-validation.sh --up --down` 已具备失败、错误退出、SIGINT、SIGTERM 的项目 Compose cleanup trap；`logRetention` 已有 backend GET/PATCH 与 Worker runtime consumer，范围限定为现有数据库日志：`operation_logs`、`api_call_logs`、终态任务的 `task_events`；新 reference upload 和 Worker 输出资产会生成 MinIO thumbnail object 并通过后端鉴权访问。
- R16 已完成：完整 P16 范围通过后端、前端、Compose、安全回归、部署验证脚本、live Compose `--up --down` 和 post-cleanup 检查。
- P17 已完成切片：`P17-BE-ORPHAN-CLEANUP`、`P17-BE-STORAGE-QUOTA-RESERVATION` 和 `P17-BE-OBSERVABILITY-METRICS`。MinIO orphan object 已有 conservative scan、dry-run、confirmed cleanup、retry-safe 失败处理、bounded listing、opaque cursor 和 sanitized audit；reference upload 与 Worker output 已接入 tenant-scoped quota reservation/counter/reconciliation，解决并发写入下的 optimistic quota race；backend admin diagnostics 已提供 queue depth、task aggregates、Provider failure rate、storage usage 和 sanitized maintenance result。
- R17 已完成：完整 P17 范围通过后端、前端、Compose、安全回归和默认 deployment release validation，未发现阻塞问题。
- P18 已完成切片：`P18-BE-PROVIDER-MODEL-SERIALIZATION` 和 `P18-E2E-REAL-PROVIDER-SMOKE`。Provider/model/default settings 写路径已补强 MySQL 行锁、模型写路径目标行锁、`taskDefaults` 保存前锁定 Provider/model，并拒绝同租户同 Provider 非删除 `modelName` 重复；可选真实 Provider smoke 脚本已具备 help/dry-run/explicit run、费用控制、直接 Provider API base 拒绝、临时文件清理和 fake-curl 安全测试。
- `P18-PROD-DRY-RUN` 已完成：默认 safe dry-run、生产 env preflight、live Compose rehearsal 和项目范围 cleanup 均通过；无真实 Provider 调用。
- P19 已完成：生产配置门禁、CI、依赖审计、外层 TLS 模板、前端日志保留设置和既有租户内置角色补齐均已合并。
- P20 当前优先级：前端租户/自定义角色管理和最终 Go/No-Go。

## P20：稳定生产运营收口

### 调度顺序

1. 公共合同由主 agent 串行冻结。
2. 可并行：`P20-BE-PROVIDER-KEY-ROTATION` 与 `P20-DEPLOY-BACKUP-RESTORE-REHEARSAL`，写入范围互不重叠。
3. 串行：`P20-BE-TENANT-PROVISIONING`。
4. 串行：`P20-BE-TENANT-ROLE-ADMIN`，依赖租户合同和既有角色 reconciliation。
5. 串行：`P20-FE-TENANT-ROLE-ADMIN`，依赖后端合同。
6. 主 agent 执行 `R20-STABLE-PRODUCTION-GO-NO-GO`。

### 已完成切片

- `P20-CSRF-CONTRACT-HARDENING`
  - 后端、Compose、CORS 和 production-env preflight 已固定使用 `X-CSRF-Token`；别名 fail closed。
- `P20-BE-PROVIDER-KEY-ROTATION`
  - 已提供 `backend/cmd/provider-key-rotation`。默认 dry-run；`--apply` 需要强确认；未删除 Provider 在串行化事务内重加密；坏行整体回滚；输出仅 sanitized 计数；payload key ID 错配 fail closed。
- `P20-DEPLOY-BACKUP-RESTORE-REHEARSAL`
  - 已提供 `scripts/backup-restore-rehearsal.sh` 和 sanitized evidence 模板。默认 guardrail-only；显式 live 模式只操作动态隔离 Compose project；真实 matching MySQL/MinIO restore 和 rollback rehearsal 已通过并完成 scoped cleanup。
- `P20-BE-TENANT-PROVISIONING`
  - 已提供 `backend/cmd/provision-tenant`。默认 dry-run；`--apply` 需要强确认；单事务创建第二及后续 tenant、内置角色/grants、首任 admin 和脱敏审计记录。
- `P20-BE-TENANT-ROLE-ADMIN`
  - 已实现 tenant-scoped `GET/PATCH /tenants/current` 与 custom role create/update/delete/permission replacement；写路径走 CSRF、RBAC、事务和 sanitized audit；内置角色不可变；被用户引用的 custom role 不可删除。

### 待完成切片

#### `P20-FE-TENANT-ROLE-ADMIN`

- 目标：在现有身份管理 UI 中增加当前租户名称编辑和自定义角色管理。
- 允许修改：frontend API/types/components/tests。
- 禁止修改：后端、公共合同、Provider/task 工作台。
- 安全要求：只调用同源后端；写操作带 CSRF；按权限隐藏入口；不持久化密码、token、角色草稿或敏感响应。
- 验收：loading/empty/error、权限门禁、租户名更新、自定义角色 CRUD、权限替换、后端错误保留草稿、stale response 保护。

#### `R20-STABLE-PRODUCTION-GO-NO-GO`

- 主 agent 执行 requirement-by-requirement 审计、全量前后端回归、脚本测试、`npm audit`、Compose config/build/live cleanup、TLS 静态检查、备份恢复 rehearsal、git whitespace、远程 push 和 hosted CI 核验。

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
   - 已完成并合并。核心 API/Worker/SSE/history/usage/log 自动化集成路径已落地，且使用 fake Worker/Provider，不调用真实 AI Provider。
2. `P15-SECURITY-FINAL-REGRESSION`
   - 已完成并合并。最终禁止模式扫描、安全回归入口和安全失败模式到测试的映射已落地。
3. `P15-DEPLOY-RUNBOOK-FINAL`
   - 已完成并合并。Compose 发布验证、运维手册、备份/恢复说明和健康检查最终化已落地。
4. `R15`
   - 已完成。最终发布就绪 review 已通过。

## 最近已完成任务包：P15-SECURITY-FINAL-REGRESSION

本任务已合并到 `main`，保留在本文档中仅作为安全回归任务包审计记录。

### 调度决策

- 本任务串行执行，不与部署 runbook 或 R15 并行。
- 理由：最终安全回归必须基于已合并的 P15 core-flow 覆盖，先把安全失败模式、禁止模式扫描和可复用命令入口稳定下来，后续部署验证和 R15 才能引用。
- 本任务以测试、扫描脚本和安全回归映射为主。默认不改生产代码；如果发现真实安全缺陷，先停止并报告主 agent，不要扩大范围直接修业务逻辑。

### 任务信息

- 任务名称：`P15-SECURITY-FINAL-REGRESSION`
- 目标：新增最终安全回归入口，整合并补齐 auth、tenant isolation、RBAC、object authorization、upload validation、Provider SSRF、sensitive redaction、task state/SSE replay、frontend forbidden patterns、deployment config safety 的自动化验证和测试映射。
- 推荐线程名：`P15-SECURITY-FINAL-REGRESSION`
- 推荐分支名：`codex/p15-security-final-regression`
- 起始分支：已合并 `P15-E2E-CORE-FLOWS` 的最新 `main`
- 前置依赖：P15 core-flow E2E 已合并；现有 P9/P12/P14/P15 focused tests 可复用；共享本地环境规则见 `docs/local-development.md`。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P15-SECURITY-FINAL-REGRESSION`。

你必须在分支 `codex/p15-security-final-regression` 上工作；如果当前不在该分支，先执行 `git switch codex/p15-security-final-regression`，确认 `git branch --show-current` 后再继续。起始点必须包含已合并 `P15-E2E-CORE-FLOWS` 的最新 `main`；如果 `git merge-base --is-ancestor main codex/p15-security-final-regression` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
为 P15 补齐最终安全回归入口和映射：
- 建立一个可复用命令或脚本，能运行最终安全回归所需的后端/前端 focused tests、forbidden-pattern scans、Compose config 检查和 whitespace 检查。
- 补齐高价值安全缺口测试，尤其是 P15 core-flow review 提到的跨租户/无权限负向场景映射不足。
- 明确 auth、tenant isolation、RBAC、object authorization、upload validation、Provider SSRF、sensitive redaction、task state/SSE replay、frontend forbidden patterns、deployment config safety 对应的测试文件/测试名或扫描命令。
- 保持生产代码不变；本任务是安全回归与验证能力建设，不是功能开发。

必须遵守：
1. 不修改 `docs/**`、`AGENTS.md`、`agent-instructions/**`。
2. 不新增业务功能，不改变公开 API/SSE/RBAC/DB 合同；如测试暴露真实安全缺陷，停止并报告主 agent。
3. 不调用真实 AI Provider，不访问 OpenAI、Gemini 或任何外部中转站。
4. 不把真实本地 MySQL/Redis/MinIO 凭据、Provider Key、Authorization、Cookie、JWT、图片 base64、bucket 或 object_key 写入仓库、快照、日志、脚本输出或最终交付。
5. 不使用前端轮询、`setInterval` 或循环 fetch 来模拟任务状态。
6. 如果新增脚本，脚本必须只编排安全测试/扫描，不启动新的 MySQL/Redis/MinIO，不清空共享服务，不删除无关数据。
7. 任何 forbidden-pattern scan 必须避免把测试中的“用于断言禁止泄漏的字符串”误判为生产违规；输出需要区分生产代码和测试断言。

建议实现：
1. 先阅读：
   - `backend/internal/api/e2e_core_flow_test.go`
   - `backend/internal/api/auth_routes_test.go`
   - `backend/internal/api/asset_routes_test.go`
   - `backend/internal/api/task_routes_test.go`
   - `backend/internal/api/task_history_routes_test.go`
   - `backend/internal/api/audit_usage_routes_test.go`
   - `backend/internal/api/provider_routes_test.go`
   - `backend/internal/provider/ssrf_test.go`
   - `backend/internal/provideradapter/redaction_test.go`
   - `backend/internal/sse/no_polling_test.go`
   - `frontend/src/test/taskWorkbench.test.tsx`
   - `frontend/src/test/historyAssetSource.test.tsx`
   - `frontend/src/test/adminObservabilitySettingsPanel.test.tsx`
   - `frontend/src/test/authFlow.test.tsx`
   - `frontend/src/test/adminProviderModelPanel.test.tsx`
   - `docs/security.md`
   - `docs/sse-contract.md`
   - `docs/local-development.md`
2. 新增一个轻量安全回归脚本，推荐 `scripts/security-regression.sh`：
   - 默认执行 focused backend security tests。
   - 默认执行 focused frontend security tests 或全量 frontend tests 中安全相关文件。
   - 执行 frontend production-code forbidden-pattern scans。
   - 执行 backend sensitive marker scans，重点检查非测试代码。
   - 执行 `docker compose -f deploy/docker-compose.yml config` 和 `git diff --check main...HEAD`。
   - 支持 `--help`，输出命令说明，不打印任何 secret 值。
3. 补齐缺口测试时优先新增或扩展测试文件，不改生产代码。重点补：
   - P15 core-flow 后新增一组同一 flow 下的跨租户或低权限负向断言，至少覆盖 asset download、project history、usage/log reads 中两个资源类型。
   - 如已有测试已覆盖，允许只新增脚本和最终映射，但最终交付必须把场景映射到真实测试名。
4. forbidden-pattern scans 建议按范围拆分：
   - frontend production files：禁止 `setInterval` polling、AI Provider direct strings、relay paths、Provider key storage、browser sensitive storage。
   - backend production files：禁止完整 Authorization/Cookie/JWT/base64/object key logging patterns，允许安全 redaction utilities 和 tests 中的 marker 字符串。
   - deploy config：确认 frontend 只代理 `/api/` 到 backend，不代理 AI Provider。
5. 不要把本任务变成大规模安全重构；如发现问题，输出明确 failure 和复现命令，由主 agent 决定是否拆修复任务。

验收命令：
```bash
cd backend
go test ./internal/api ./internal/provider ./internal/provideradapter ./internal/sse ./internal/task ./internal/audit -count=1
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
bash scripts/security-regression.sh --help
bash scripts/security-regression.sh
```

最终交付必须包含：
- 修改文件清单。
- 执行的测试命令和结果。
- 每个 failure mode 对应的测试文件/测试名映射。
- 安全自查结果，明确没有真实 AI Provider 调用、Provider Key 存储、task polling、localStorage/sessionStorage/IndexedDB 敏感数据、Authorization/Cookie/JWT/base64/object key 暴露。
- 刻意未修改范围。
- forbidden-pattern scan 命中项说明：哪些是生产违规、哪些是测试断言或 redaction helper 的允许命中。
- 如使用共享本地服务，说明创建/修改/清理了哪些 `codex_p15_security_*` 测试数据；默认优先不依赖共享服务。
- 如发现公共合同缺口，只报告给主 agent，不修改 docs。
```

### 允许修改文件

- `backend/internal/api/*_test.go`
- `backend/internal/provider/*_test.go`
- `backend/internal/provideradapter/*_test.go`
- `backend/internal/sse/*_test.go`
- `backend/internal/task/*_test.go`
- `backend/internal/audit/*_test.go`
- `frontend/src/test/*.test.ts`
- `frontend/src/test/*.test.tsx`
- `scripts/security-regression.sh`

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/internal/**` 非测试生产文件，除非发现真实安全缺陷且先停止报告；默认不改
- `frontend/src/**` 非测试生产文件
- `deploy/**`
- `frontend/src/lib/taskSseClient.ts`
- 任何 Provider 直连、AI relay、Provider Key 存储、task polling、IndexedDB 敏感数据或真实外部网络调用路径

### 具体开发内容

1. 新增 `scripts/security-regression.sh`，提供 `--help` 和默认执行路径。
2. 梳理现有安全测试，确定脚本覆盖的 focused backend/frontend test files。
3. 补齐缺口测试，优先覆盖 P15 core-flow 下跨租户/低权限负向读取和敏感响应排除。
4. 增加或固化 frontend/backend/deploy forbidden-pattern scans。
5. 保持脚本可在普通开发机执行，失败时输出失败命令，不打印 secret 值。

### 必须保持的现有行为

- 现有单元、集成和前端测试不回归。
- 生产代码行为不因本任务改变。
- 任务状态仍通过 SSE/事件历史验证，不引入 polling。
- Provider key、auth cookie、CSRF、JWT、object_key、bucket、image bytes 仍不进入日志、快照、脚本输出或前端可见状态。
- Docker Compose 配置不被本任务改动。

### 允许的中间态

- 安全回归脚本可以先编排 focused tests 和 scans，不要求启动完整 Compose runtime。
- 部分已有安全场景可通过映射既有测试名完成，不必重复写测试。
- 允许新增少量测试 helper，但不能改生产路径来方便测试。

### 禁止的半迁移状态

- 只新增脚本但脚本没有运行任何真实测试或扫描。
- 脚本扫描范围只扫测试文件，漏掉生产前端/后端代码。
- 将测试 marker 字符串误报为生产安全失败却没有说明。
- 为了让测试通过而放松认证、租户过滤、RBAC、上传校验、SSRF、SSE 或 redaction 行为。
- 使用真实 Provider Key、真实外部 AI 调用或真实 object storage 直链完成验证。

### 失败模式与边界场景

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| 未登录访问核心接口 | 返回 401，不创建业务数据 | 是 |
| 无权限访问管理或项目资源 | 返回 403/404，不泄露敏感数据 | 是 |
| 跨租户读取/写入项目、资产、任务、history、usage/log | 返回 404/空结果，不泄露存在性 | 是 |
| Provider API Key 创建、测试、更新、读取 | 只返回 hint/metadata，日志和响应不含完整 key | 是 |
| Provider base URL SSRF | localhost/private/link-local/Docker internal 被拒绝 | 是 |
| 上传伪造文件/SVG/超限图片 | 被拒绝，不写 MinIO，不写成功 metadata | 是 |
| Task/SSE replay | Last-Event-ID 补发正确，不用 polling，不跨租户 | 是 |
| Worker/API metadata redaction | 不含 Authorization/Cookie/JWT/base64/b64_json/object_key/bucket/API key | 是 |
| Frontend production forbidden patterns | 无 Provider direct calls、AI relay、Provider Key browser storage、task polling | 是 |
| Deploy config | frontend 只代理 `/api/` 到 backend，不代理 AI Provider | 是 |

### 安全要求

- 所有新增测试必须使用测试替身或内存测试环境；不得调用真实 AI Provider。
- Forbidden-pattern scan 必须覆盖生产前端文件，不得仅依赖测试。
- 脚本必须 `set -euo pipefail`，失败时直接退出。
- 脚本不得打印 `.env` 内容、cookie、JWT、Provider Key、MinIO secret、Redis/MySQL password。
- 不得新增 browser storage、polling、Provider direct call、MinIO direct download 或 object key 暴露。

### 必须新增或更新的回归测试

- `backend/internal/api/e2e_core_flow_test.go` 或相关 API 测试：补齐 P15 flow 级别的跨租户/低权限负向断言，至少覆盖两个资源类型。
- 如缺口已由既有测试覆盖，必须在最终交付中列出真实测试名，不允许只写“已有覆盖”。
- `scripts/security-regression.sh`：脚本自身通过 `--help` 和默认执行验证。
- 如发现 frontend forbidden pattern 缺口，新增或更新相应 frontend test；否则用扫描证明无新增生产违规。

### 验收标准

- 存在一个可重复执行的最终安全回归脚本或命令入口。
- 失败模式表中的每一项都有测试名或扫描命令映射。
- 跨租户/无权限负向场景在 P15 final security regression 中有明确覆盖。
- 前端生产代码 forbidden-pattern scan、后端生产代码 sensitive-pattern scan、Compose config 检查均通过或有清晰允许项说明。
- 未修改公共合同文档、Agent 规则、部署配置或无关生产代码。

### 测试命令

```bash
cd backend
go test ./internal/api ./internal/provider ./internal/provideradapter ./internal/sse ./internal/task ./internal/audit -count=1
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

if [ -f scripts/security-regression.sh ]; then
  bash scripts/security-regression.sh --help
  bash scripts/security-regression.sh
fi
```

如使用共享本地服务做额外功能验证，必须按 `docs/local-development.md` 使用任务命名空间数据并在交付中记录。

## 最近已完成任务包：P15-DEPLOY-RUNBOOK-FINAL

本任务已合并到 `main`，保留在本文档中仅作为部署 runbook 任务包审计记录。新的下一个任务是 `R15`，由主 agent 直接执行，不创建子 agent worktree。

## R15：最终发布就绪 Review

### 调度决策

- 本任务为主 agent 串行 review，不创建新分支，不交给子 agent。
- Review 范围是完整 P15 代码范围：`P15-E2E-CORE-FLOWS`、`P15-SECURITY-FINAL-REGRESSION`、`P15-DEPLOY-RUNBOOK-FINAL` 及其配套 docs。
- Review 后由主 agent 更新公共文档中的实际结果和遗留风险。

### 必须检查

- P15 core-flow E2E 是否仍覆盖 init-admin、Provider/model、project、reference upload、task create、fake Worker success、SSE replay、output asset download、history、usage records/summary、operation logs 和 API call logs。
- `scripts/security-regression.sh` 是否可运行，且没有真实 AI Provider 调用、Provider Key 存储、task polling、敏感浏览器存储或敏感日志输出。
- `scripts/deploy-release-validation.sh` 和 `deploy/RUNBOOK.md` 是否覆盖 Compose config/build/up/health/proxy/down cleanup、init-admin、MinIO bootstrap、SSE proxy、backup/restore、upgrade/rollback、日志排查和清理。
- Docker Compose 验证后是否没有遗留项目容器和项目卷。
- 前端、后端、安全回归、部署验证和 whitespace 检查是否通过。

### R15 验证命令

```bash
bash scripts/security-regression.sh
bash scripts/deploy-release-validation.sh
bash scripts/deploy-release-validation.sh --up --down
docker compose -f deploy/docker-compose.yml ps -a
docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true

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
git diff --check
```

### R15 实际结果

- Review 范围：完整 P15 range `3db7980..HEAD`。
- 总体结论：通过，未发现阻塞 release-readiness 问题。
- 已通过验证：
  - `bash scripts/security-regression.sh`
  - `bash scripts/deploy-release-validation.sh`
  - `bash scripts/deploy-release-validation.sh --up --down`
  - `docker compose -f deploy/docker-compose.yml ps -a`
  - `docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true`
  - `cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./cmd/api ./cmd/worker`
  - `cd frontend && npm run lint && npm run type-check && npm run test && npm run build`
  - `docker compose -f deploy/docker-compose.yml config`
  - `git diff --check`
- Live Compose 验证确认服务健康、frontend `/api/` 代理、SSE auth boundary 和 cleanup 均正常。
- 非阻塞遗留：Provider/model 更强事务序列化仍属于 post-R15 backlog；严格并发 quota reservation 已由 `P17-BE-STORAGE-QUOTA-RESERVATION` 完成。

## 稳定生产上线路线图

P15 已达到 release candidate 状态。稳定生产上线还需要 P16-P18 三个阶段，重点从“功能可用”转向“长期运行可靠、运维可控、真实 Provider 可验证”。

### P16：生产上线前硬化

建议串行开始，第一任务为部署脚本硬化：

1. `P16-DEPLOY-SCRIPT-HARDENING`
   - 给 `scripts/deploy-release-validation.sh --up --down` 增加失败 cleanup trap。
   - 补充脚本级回归，证明失败时不会遗留项目 Compose 容器或卷。
   - 不修改业务前后端代码。
   - 已完成并合并。
2. `P16-BE-LOG-RETENTION`
   - 实现 operation logs、api call logs、task events/error logs 的 retention runtime consumer。
   - 未接入真实 consumer 前不得暴露新的 active writable settings。
   - 已完成并合并。
3. `P16-BE-THUMBNAIL-POLICY`
   - 明确并落地缩略图策略，推荐生成 MinIO thumbnail object 并经后端鉴权访问。
   - 已完成并合并。
4. `R16`
   - 主 agent review P16 全部代码和回归。
   - 已完成。未发现阻塞问题。

### P17：存储治理与生产观测

1. `P17-BE-ORPHAN-CLEANUP`
   - MinIO orphan discovery、dry-run、执行、审计、批量限制和失败重试。
   - 已完成并合并。
2. `P17-BE-STORAGE-QUOTA-RESERVATION`
   - 为并发上传和 Worker 输出增加严格 quota reservation/counter 与 reconciliation。
   - 已完成并合并。
3. `P17-BE-OBSERVABILITY-METRICS`
   - 增加 admin-only JSON diagnostics：queue depth、running/failed tasks、Provider failure rate、storage usage、maintenance job result 等。
   - 已完成并合并。
4. `R17`
   - 主 agent review P17 全部代码和回归。
   - 已完成。未发现阻塞问题。

### P18：真实上线信心与 Go/No-Go

1. `P18-BE-PROVIDER-MODEL-SERIALIZATION`
   - 强化 Provider/model enable/disable/delete/update 与默认设置交互的事务序列化。
   - 已完成并合并。
2. `P18-E2E-REAL-PROVIDER-SMOKE`
   - 新增可选真实 Provider smoke 脚本；不进默认 CI，不提交真实 key，必须有费用控制。
   - 已完成并合并。
3. `P18-PROD-DRY-RUN`
   - 按 runbook 在目标或准生产环境执行完整上线 dry-run。
   - 已完成仓库可控范围：safe default、production-env preflight、live Compose rehearsal 和 scoped cleanup 均通过。
4. `R18-STABLE-PRODUCTION-READINESS`
   - 主 agent 执行最终 Go/No-Go review。
   - 审计后转入 P19/P20 运营收口，发现并继续解决 TLS、CI、主密钥轮换、租户开通和备份恢复演练缺口。

## 历史任务包：P18-PROD-DRY-RUN

### 调度决策

- 本任务串行执行，不与 R18 并行。
- 理由：production dry-run 是最终 Go/No-Go 前的证据收集任务，必须基于已合并的部署 runbook、release validation、安全回归和可选 real Provider smoke 工具完成。
- 本任务以部署验证脚本、runbook 证据模板和本地/准生产 dry-run 为主。默认不得调用真实 AI Provider，不得提交真实密钥、真实响应体、真实输出图片或环境凭据。

### 任务信息

- 任务名称：`P18-PROD-DRY-RUN`
- 目标：新增并执行一个稳定上线 dry-run 入口，串联部署前检查、Compose release validation、security regression、runbook checklist、backup/restore rehearsal 说明、optional real Provider smoke dry-run 和 sanitized evidence 输出，为 R18 Go/No-Go review 提供可复现证据。
- 推荐线程名：`P18-PROD-DRY-RUN`
- 推荐分支名：`codex/p18-prod-dry-run`
- 起始分支：已合并 `P18-E2E-REAL-PROVIDER-SMOKE` 的最新 `main`
- 前置依赖：P18 Provider/model/default-setting serialization 已合并；`scripts/real-provider-smoke.sh`、`scripts/deploy-release-validation.sh`、`scripts/security-regression.sh` 和 `deploy/RUNBOOK.md` 均可用。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P18-PROD-DRY-RUN`。

你必须在分支 `codex/p18-prod-dry-run` 上工作；如果当前不在该分支，先执行 `git switch codex/p18-prod-dry-run`，确认 `git branch --show-current` 后再继续。起始点必须包含已合并 `P18-E2E-REAL-PROVIDER-SMOKE` 的最新 `main`；如果 `git merge-base --is-ancestor main codex/p18-prod-dry-run` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
新增稳定上线 dry-run 验证入口和证据模板，用于 R18 前的 operator rehearsal：
- 默认模式只运行不会产生真实 AI 费用的检查：deploy release validation、安全回归、Compose config/build/health 路径、real Provider smoke dry-run、runbook checklist。
- 允许显式 opt-in 的真实 Provider smoke，但必须复用 `scripts/real-provider-smoke.sh --run` 的确认变量、费用上限和脱敏行为；本任务不得新增第二套真实 Provider 调用路径。
- 输出的 evidence 必须是 sanitized summary，不包含真实密钥、Cookie、Authorization、JWT、CSRF、图片 base64、MinIO bucket/object_key、signed URL、真实 Provider response body、数据库密码或本地服务密钥。
- 不修改后端/前端生产代码，不改变部署拓扑，不把 dry-run 纳入默认 CI。

允许修改文件：
- `scripts/**`
- `deploy/RUNBOOK.md`
- 可新增 `deploy/PRODUCTION_DRY_RUN.md` 或 `deploy/PRODUCTION_DRY_RUN_TEMPLATE.md`
- 可新增 `scripts/*dry-run*.sh`、`scripts/*dry-run*test*.sh`
- 不改 `docs/**`，公共文档由主 agent 在 review/merge 后更新

禁止修改文件：
- `AGENTS.md`
- `agent-instructions/**`
- `docs/**`
- `backend/internal/**`
- `backend/cmd/**`
- `frontend/**`
- `deploy/docker-compose.yml`，除非发现 runbook 无法验证当前拓扑；如确需改 Compose，先停止报告
- `.env`、真实密钥文件、真实 Provider 响应、真实输出图片、数据库 dump、MinIO 对象或任何本地环境凭据

前置阅读：
- `AGENTS.md`
- `agent-instructions/01-project-overview.md`
- `agent-instructions/02-architecture-rules.md`
- `agent-instructions/05-security-rules.md`
- `agent-instructions/06-testing-and-delivery.md`
- `agent-instructions/07-task-package-and-review-rules.md`
- `docs/development-plan.md`
- `docs/codex-agent-tasks.md`
- `docs/deployment.md`
- `docs/security.md`
- `docs/local-development.md`
- `deploy/RUNBOOK.md`
- `scripts/deploy-release-validation.sh`
- `scripts/deploy-release-validation-test.sh`
- `scripts/security-regression.sh`
- `scripts/real-provider-smoke.sh`
- `scripts/real-provider-smoke-test.sh`

具体开发内容：
1. 新增 production dry-run 脚本，建议命名 `scripts/prod-dry-run.sh`：
   - 支持 `--help`、默认安全检查、可选 `--live-compose`、可选 `--real-provider-smoke`。
   - 默认不启动持久服务、不调用真实 AI Provider、不写真实业务数据。
   - 默认串联：
     - `bash scripts/deploy-release-validation.sh`
     - `bash scripts/security-regression.sh`
     - `bash scripts/real-provider-smoke.sh --dry-run`
     - `docker compose -f deploy/docker-compose.yml config`
   - `--live-compose` 可以调用 `bash scripts/deploy-release-validation.sh --up --down`，并必须继承其 cleanup trap 行为。
   - `--real-provider-smoke` 只能调用 `bash scripts/real-provider-smoke.sh --run`，不得直接实现 Provider 调用；必须检测并提示确认变量和费用风险。
   - 所有输出必须是阶段化、脱敏、可复制到 R18 review 的摘要。
2. 新增脚本测试，建议命名 `scripts/prod-dry-run-test.sh`：
   - 用 fake PATH 注入模拟 `bash`/`docker`/子脚本或通过轻量 wrapper 验证调用顺序。
   - 覆盖默认模式不会调用 `real-provider-smoke.sh --run`。
   - 覆盖 `--real-provider-smoke` 缺确认变量 fail closed，且不打印 fake secret。
   - 覆盖 `--live-compose` 会调用 release validation 的 `--up --down`，不直接执行 broad docker prune。
   - 覆盖子命令失败时退出非 0，并输出 sanitized 阶段名。
3. 更新 `deploy/RUNBOOK.md` 或新增 `deploy/PRODUCTION_DRY_RUN_TEMPLATE.md`：
   - 给出 dry-run 执行步骤、证据清单、Go/No-Go 条件、rollback/backup rehearsal 检查项。
   - 明确真实 Provider smoke 是可选人工步骤，不属于默认 dry-run。
   - 明确不要保存真实 secret、Provider response、图片输出、object key 或 signed URL。
4. 实际执行本地验证：
   - 默认 `scripts/prod-dry-run.sh`。
   - `scripts/prod-dry-run.sh --live-compose` 如本机环境允许；执行后必须确认项目 Compose 容器/卷已清理。
   - 不执行真实 Provider `--run`，除非主 agent/用户另行提供真实密钥并明确授权。

安全要求：
- 不得新增任何直接调用 OpenAI、Gemini、OpenAI-Compatible 中转站或自定义 AI Provider 的路径。
- 不得打印、写入、提交或快照 Provider API Key、Authorization、Cookie、JWT、CSRF、图片 base64、MinIO bucket/object_key、signed URL、数据库密码、Redis/MinIO 凭据。
- `--real-provider-smoke` 必须复用 `scripts/real-provider-smoke.sh --run`，并保留其确认变量、API base 校验、费用上限和临时文件清理。
- live Compose dry-run cleanup 必须只使用 `deploy/docker-compose.yml`，不得使用 broad Docker prune、`docker system prune`、`docker volume prune` 或删除无关容器/卷。

必须保持的现有行为：
- `scripts/deploy-release-validation.sh` 默认行为不变。
- `scripts/security-regression.sh` 默认行为不变。
- `scripts/real-provider-smoke.sh` 默认 help/dry-run 行为不变。
- 后端、前端、Compose topology 不变。
- 默认验证仍不调用真实 Provider。

允许的中间态：
- Production dry-run 可以先作为手动 operator entry point 存在，不纳入默认 CI。
- 真实 Provider smoke 可以作为 dry-run 的显式可选步骤存在，但默认只执行 dry-run。
- Evidence 可以是模板或 sanitized summary，不需要提交真实环境输出。

禁止的半迁移状态：
- 默认 dry-run 或默认 release validation 触发真实 AI 调用。
- 新增第二套真实 Provider 调用逻辑，绕过 `scripts/real-provider-smoke.sh` 或 Go 后端。
- 脚本失败后留下项目 Compose 容器/卷而未报告。
- 输出或文档要求用户把真实 secret 写进命令历史、仓库、日志或模板。
- 为了 dry-run 修改后端/前端生产代码或数据库 schema。

失败模式与边界场景：

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| `--help` | 输出用法，退出 0，不执行检查 | 脚本测试 |
| 默认模式 | 运行安全检查与 smoke dry-run，不调用真实 Provider | 脚本测试/实际命令 |
| 子检查失败 | 输出 sanitized 阶段名，退出非 0 | 脚本测试 |
| `--real-provider-smoke` 缺确认变量 | fail closed，不调用真实 Provider，不打印 secret | 脚本测试 |
| `--real-provider-smoke` 有确认变量 | 只委托 `real-provider-smoke.sh --run`，不直接 Provider call | 脚本测试 |
| `--live-compose` | 调用 release validation `--up --down` 并依赖 cleanup trap | 脚本测试/可选实际命令 |
| fake secret 出现在子命令错误中 | dry-run 输出不包含完整 secret | 脚本测试 |
| 备份/恢复 rehearsal | 只记录 checklist 和命令入口，不提交 dump 或真实数据 | 文档检查 |

必须新增或更新的回归测试：
- `scripts/prod-dry-run.sh --help`
- 默认 `scripts/prod-dry-run.sh` 调用顺序测试
- `scripts/prod-dry-run.sh --real-provider-smoke` 缺确认变量 fail closed 测试
- `scripts/prod-dry-run.sh --live-compose` 委托 `deploy-release-validation.sh --up --down` 测试
- 子命令失败和 fake secret 脱敏测试

验收标准：
- 新 dry-run 入口默认安全，不可能误触真实 AI 调用。
- live Compose 模式仍受现有 cleanup trap 保护。
- 可选真实 Provider smoke 只通过已合并 smoke 脚本执行。
- runbook/checklist 足以支持 R18 Go/No-Go review。
- 现有安全回归、部署发布验证、真实 Provider smoke 脚本测试和 Compose config 均通过。

测试命令：
```bash
bash scripts/prod-dry-run.sh --help
bash scripts/prod-dry-run.sh
bash scripts/prod-dry-run-test.sh
bash scripts/deploy-release-validation-test.sh
bash scripts/deploy-release-validation.sh --help
bash scripts/deploy-release-validation.sh
bash scripts/security-regression.sh
bash scripts/real-provider-smoke-test.sh
bash scripts/real-provider-smoke.sh --dry-run
docker compose -f deploy/docker-compose.yml config
git diff --check main...HEAD
```

如执行 live Compose：
```bash
bash scripts/prod-dry-run.sh --live-compose
docker compose -f deploy/docker-compose.yml ps -a
docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true
```

交付要求：
- 提交到分支 `codex/p18-prod-dry-run`。
- 最终说明必须包含：修改文件清单、执行命令结果、failure mode 到测试名映射、安全自查、刻意未修改范围、是否实际调用真实 Provider、live Compose 是否执行以及 cleanup 结果。
```

## 最近已完成任务包：P18-E2E-REAL-PROVIDER-SMOKE

### 调度决策

- 本任务串行执行，不与 production dry-run 或 R18 并行。
- 理由：真实 Provider smoke 是显式 opt-in 的上线信心验证入口，必须先稳定安全门禁、成本门禁和脚本行为，再允许后续生产 dry-run 引用。
- 本任务以脚本和脚本测试为主。默认模式不得调用真实 Provider，不得创建 Provider/model/project/task，不得消耗费用。

### 任务信息

- 任务名称：`P18-E2E-REAL-PROVIDER-SMOKE`
- 目标：新增一个手动执行的真实 Provider smoke 脚本，用来在部署环境中验证“登录/初始化、Provider/model 配置、项目创建、任务提交、SSE/状态观察、输出资产检查”的最小真实 AI 调用路径；脚本必须有费用控制、密钥保护、默认 dry-run 和清理说明。
- 推荐线程名：`P18-E2E-REAL-PROVIDER-SMOKE`
- 推荐分支名：`codex/p18-real-provider-smoke`
- 起始分支：已合并 `P18-BE-PROVIDER-MODEL-SERIALIZATION` 的最新 `main`
- 前置依赖：P18 Provider/model/default-setting serialization 已合并；P15 deploy runbook、P15 security regression、P17 diagnostics 均可用。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P18-E2E-REAL-PROVIDER-SMOKE`。

你必须在分支 `codex/p18-real-provider-smoke` 上工作；如果当前不在该分支，先执行 `git switch codex/p18-real-provider-smoke`，确认 `git branch --show-current` 后再继续。起始点必须包含已合并 `P18-BE-PROVIDER-MODEL-SERIALIZATION` 的最新 `main`；如果 `git merge-base --is-ancestor main codex/p18-real-provider-smoke` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
新增可选真实 Provider smoke 验证入口：
- 默认执行只显示帮助或 dry-run 校验，不调用真实 AI Provider，不消耗费用。
- 只有显式 `--run` 加确认环境变量时才允许创建真实 Provider/model/project/task 并触发真实 AI 调用。
- 必须保护 API Key、Cookie、Authorization、CSRF、Provider response、图片 base64、MinIO object key 和 signed URL，不打印、不写入仓库、不进入测试快照。
- 必须有费用控制：默认 `n=1`、有限 prompt、有限 timeout、显式确认变量、可配置但有上限的输出数量。
- 必须使用现有后端 API，不直接调用 OpenAI/Gemini/中转站。
- 必须观察任务状态通过 SSE 或后端已有事件/状态接口；不得实现前端轮询模式，不得使用 `setInterval`，脚本内如需等待只能作为一次性 CLI smoke 的 bounded wait，不得新增生产代码轮询。

允许修改文件：
- `scripts/**`
- `scripts/*test*.sh` 或新增脚本测试文件
- `deploy/RUNBOOK.md` 仅限增加“如何手动运行 real Provider smoke”的简短章节
- `docs/deployment.md` 仅限主 agent 指定时修改；本任务默认不要改公共 docs
- 不改后端/前端生产代码，除非发现脚本无法使用现有公开 API 完成最小 smoke；若需要后端改动，先停止报告

禁止修改文件：
- `AGENTS.md`
- `agent-instructions/**`
- `docs/**`，除非主 agent 明确允许的 `deploy/RUNBOOK.md` 不属于 docs
- `backend/internal/**`
- `backend/cmd/**`
- `frontend/**`
- `deploy/docker-compose.yml`
- Provider Adapter runtime、Provider/model 管理生产逻辑、认证/JWT/Cookie 主流程、SSE handler、Worker 状态机、task execution 主流程、storage/quota/cleanup 逻辑
- 不提交 `.env`、真实 API Key、真实 Cookie、真实输出图片、真实 Provider response body 或本地测试数据

前置阅读：
- `AGENTS.md`
- `agent-instructions/01-project-overview.md`
- `agent-instructions/02-architecture-rules.md`
- `agent-instructions/05-security-rules.md`
- `agent-instructions/06-testing-and-delivery.md`
- `agent-instructions/07-task-package-and-review-rules.md`
- `docs/development-plan.md`
- `docs/codex-agent-tasks.md`
- `docs/deployment.md`
- `docs/security.md`
- `deploy/RUNBOOK.md`
- `scripts/deploy-release-validation.sh`
- `scripts/security-regression.sh`

具体开发内容：
1. 新增脚本，建议命名 `scripts/real-provider-smoke.sh`：
   - 支持 `--help`、`--dry-run`、`--run`。
   - 默认无参数等价于 `--help` 或安全 dry-run，不调用真实 Provider。
   - `--run` 必须要求确认变量，例如 `REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS`。
   - 必须要求通过环境变量提供目标 API base URL、admin 登录或 init-admin 信息、Provider 类型/base URL/API Key、模型名、尺寸/质量/输出格式等。
   - 必须使用临时 cookie jar，并在退出时清理。
   - 日志输出必须脱敏，只显示 Provider key hint 或固定 `[REDACTED]`。
   - 失败时退出非零并给出 sanitized 失败阶段。
2. Smoke 流程：
   - 连接后端 health endpoint。
   - 登录现有 admin；可选支持在明确变量开启时调用 init-admin。
   - 创建或复用 smoke 专用 Provider/model/project，命名必须带 `codex-smoke` 或类似前缀。
   - 提交最小 generation task，默认 `n=1`，prompt 明确为 smoke test。
   - 用 SSE 或现有 task/event API 观察 bounded timeout 内的状态；不要新增生产轮询逻辑。
   - 成功后验证至少一个输出 asset 通过后端授权元数据可见；默认不下载原图，除非显式变量要求。
   - 输出清理提示；如果脚本执行自动清理，只能清理自己创建的 `codex-smoke` 数据。
3. 新增脚本测试：
   - 覆盖 `--help`。
   - 覆盖无确认变量时 `--run` fail closed。
   - 覆盖 dry-run 不需要真实 key、不调用 curl 写接口或 Provider。
   - 覆盖日志脱敏：测试输出不能包含 fake key 全值、Authorization、Cookie、JWT、base64、object key。
   - 可使用 fake `curl`/PATH 注入方式，参考 `scripts/deploy-release-validation-test.sh`。
4. 如修改 `deploy/RUNBOOK.md`：
   - 只增加简短手动运行章节。
   - 明确该脚本不是默认发布门禁，不应在 CI 默认执行，不应保存真实 key。

安全要求：
- 脚本不得直接请求 OpenAI/Gemini/AI relay；所有调用都必须是目标平台 `/api/v1` 后端 API。
- 不得打印、持久化或提交完整 Provider API Key、Cookie、Authorization、CSRF token、JWT、图片 base64、MinIO bucket/object key、signed URL。
- 默认模式不得产生费用。
- `--run` 必须有显式确认和 bounded timeout；输出数量默认 1，最大值必须受脚本限制。
- 不得使用共享本地真实密钥；若使用共享本地 MySQL/Redis/MinIO 验证脚本逻辑，只能用 fake curl 或 dry-run，不写真实业务数据。

必须保持的现有行为：
- `scripts/security-regression.sh` 和 `scripts/deploy-release-validation.sh` 行为不回归。
- 默认发布验证仍不调用真实 Provider。
- 后端/前端生产代码不变。
- Docker Compose config 不变。

允许的中间态：
- 新脚本可以先作为手动 smoke 入口存在，不纳入默认 CI 和默认 release validation。
- 真实 Provider smoke 的实际执行依赖操作者提供有效部署环境和真实 Provider key；自动化测试只验证脚本门禁和脱敏行为。

禁止的半迁移状态：
- 默认测试或默认 deploy validation 触发真实 AI 调用。
- 脚本没有确认变量就能执行真实 Provider 任务。
- 脚本输出完整 API Key、Cookie、Authorization、JWT、base64、object key 或 signed URL。
- 脚本直接调用 AI Provider，绕过 Go 后端。
- 脚本创建不可识别、不可清理的业务数据。
- 为了脚本修改后端生产 API 或前端生产代码。

失败模式与边界场景：

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| `--help` | 输出用法，退出 0，不需要 secrets | 脚本测试 |
| 无参数 | 安全输出帮助或 dry-run，不调用真实 Provider | 脚本测试 |
| `--run` 缺确认变量 | fail closed，退出非 0，不打印 secret | 脚本测试 |
| 缺少必需环境变量 | fail closed，列出缺失字段名，不打印 secret 值 | 脚本测试 |
| dry-run | 校验参数形状，不调用写接口，不调用 Provider | 脚本测试 |
| fake curl 返回 auth/validation/server error | sanitized 阶段失败，退出非 0 | 脚本测试 |
| fake output 含 secret/base64/object key | 输出脱敏或不输出 | 脚本测试 |
| 真实任务超时 | bounded timeout 后退出非 0，给出 sanitized task id/status | 脚本或文档说明 |
| 真实任务成功 | 输出 task id、状态、输出 asset 数量，不输出 object key 或 signed URL | 脚本说明/可测试 fake |

必须新增或更新的回归测试：
- `scripts/real-provider-smoke.sh --help`
- `scripts/real-provider-smoke.sh --run` 缺确认变量失败
- `scripts/real-provider-smoke.sh --dry-run` 不调用写接口
- fake `curl` 场景下 secret redaction 和阶段失败
- 现有 deploy/security 脚本测试继续通过

验收标准：
- 新脚本默认安全，不可能误触真实 AI 调用。
- `--run` 具备显式确认、费用上限、bounded timeout、脱敏输出和临时 cookie cleanup。
- 现有默认 release validation/security regression 不被改成真实 Provider 调用。
- 脚本测试通过，Compose config 和安全回归通过。

测试命令：
```bash
bash scripts/real-provider-smoke.sh --help
bash scripts/real-provider-smoke.sh --dry-run
bash scripts/real-provider-smoke.sh --run
bash scripts/real-provider-smoke-test.sh
bash scripts/deploy-release-validation.sh --help
bash scripts/deploy-release-validation.sh
bash scripts/security-regression.sh
docker compose -f deploy/docker-compose.yml config
git diff --check main...HEAD
```

交付要求：
- 提交到分支 `codex/p18-real-provider-smoke`。
- 最终说明必须包含：修改文件清单、执行命令结果、failure mode 到测试名映射、安全自查、刻意未修改范围、是否实际调用真实 Provider。默认应为“未调用真实 Provider”。
```

## 最近已完成任务包：P18-BE-PROVIDER-MODEL-SERIALIZATION

### 调度决策

- 本任务串行执行，不与 real Provider smoke、production dry-run、前端任务或部署任务并行。
- 理由：本任务会修改 Provider/model/default settings 的共享控制面写路径，是 P18 后续真实 Provider smoke 和上线 dry-run 的前置一致性保障。
- 本任务不新增 API 字段，不新增前端 UI，不触发真实 Provider 调用，不解密 Provider API Key，不修改 Worker/SSE/task execution 主流程。

### 任务信息

- 任务名称：`P18-BE-PROVIDER-MODEL-SERIALIZATION`
- 目标：强化 Provider/model enable/disable/delete/update 与 `taskDefaults` 写入之间的事务序列化，确保并发 admin 操作后不会留下“启用模型挂在禁用/删除 Provider 下”“默认 Provider/model 指向不可用对象”“同一 Provider 下重复 active model name”等不一致状态。
- 推荐线程名：`P18-BE-PROVIDER-MODEL-SERIALIZATION`
- 推荐分支名：`codex/p18-backend-provider-model-serialization`
- 起始分支：完成 R17 后的最新 `main`
- 前置依赖：P14 Provider/model lifecycle integrity、P13 runtime task defaults、P17 R17 回归均已合并。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P18-BE-PROVIDER-MODEL-SERIALIZATION`。

你必须在分支 `codex/p18-backend-provider-model-serialization` 上工作；如果当前不在该分支，先执行 `git switch codex/p18-backend-provider-model-serialization`，确认 `git branch --show-current` 后再继续。起始点必须包含完成 R17 后的最新 `main`；如果 `git merge-base --is-ancestor main codex/p18-backend-provider-model-serialization` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
强化 Provider/model/default settings 控制面一致性：
- 统一 Provider/model/default settings 写路径的事务锁顺序，避免并发 admin 操作留下不可用引用。
- Provider disable/delete 与 model create/update/enable/disable/delete、taskDefaults update 必须在同一 tenant 范围内序列化相关行。
- 禁止启用模型挂在 disabled/deleted Provider 下。
- 禁止 taskDefaults 写入 disabled/deleted/cross-tenant/mismatched Provider/model。
- 对同一 tenant + same Provider + active(non-deleted) `modelName` 重复问题做明确数据完整性处理；优先在写路径 fail closed，并用测试固定行为。不要在没有迁移/backfill 方案的情况下贸然添加破坏现有数据的唯一索引。
- 不改变外部 API 形状、错误响应格式、RBAC 合同、Provider Adapter runtime 或 Worker 执行语义。

允许修改文件：
- `backend/internal/provider/**`
- `backend/internal/model/**`
- `backend/internal/settings/**`
- `backend/internal/api/provider_routes_test.go`
- `backend/internal/api/model_routes_test.go`
- `backend/internal/api/system_settings_routes_test.go`
- `backend/internal/api/task_routes_test.go` 仅限补充默认设置相关回归
- `backend/internal/database/**` 仅限确有必要的非破坏性索引/迁移与测试；如果需要破坏性唯一约束或数据 backfill，先停止报告，不要直接落地
- backend-only 测试 helper

禁止修改文件：
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `deploy/**`
- `scripts/**`
- `backend/internal/provideradapter/**`
- `backend/internal/task/**`，除 `backend/internal/api/task_routes_test.go` 中的路由测试外
- `backend/cmd/**`
- 认证/JWT/Cookie 主流程、SSE handler、Redis queue、Worker claim/cancel/retry/timeout 状态机、task execution 主流程、MinIO/storage cleanup/quota runtime
- 不新增真实 AI Provider 调用、Provider test 调用、API key 解密、前端轮询、浏览器存储或新的 active writable settings

前置阅读：
- `AGENTS.md`
- `agent-instructions/01-project-overview.md`
- `agent-instructions/02-architecture-rules.md`
- `agent-instructions/04-backend-rules.md`
- `agent-instructions/05-security-rules.md`
- `agent-instructions/06-testing-and-delivery.md`
- `agent-instructions/07-task-package-and-review-rules.md`
- `docs/development-plan.md`
- `docs/codex-agent-tasks.md`
- `docs/api-contract.md`
- `docs/database-schema.md`
- `docs/security.md`
- `backend/internal/provider/service.go`
- `backend/internal/provider/repository.go`
- `backend/internal/model/service.go`
- `backend/internal/model/repository.go`
- `backend/internal/settings/service.go`
- `backend/internal/settings/repository.go`

具体开发内容：
1. 梳理并实现 Provider/model/default settings 写路径统一锁顺序：
   - 相关操作必须 tenant-scoped。
   - 推荐锁顺序：Provider row -> linked model rows or target model row -> system setting row。
   - 同一个事务内完成校验、写入和成功 audit log。
   - SQLite 测试环境可以跳过物理 `FOR UPDATE`，但生产 MySQL 路径必须使用行级锁或等价事务序列化。
2. Provider 写路径：
   - disable 或 `PATCH status=DISABLED` 时，在事务内锁定 Provider 并重新检查 enabled linked models。
   - delete 时，在事务内锁定 Provider 并重新检查 same-tenant non-deleted linked models。
   - 失败路径不得记录成功 operation log。
3. Model 写路径：
   - create/update/enable 时，在事务内锁定目标 Provider 并验证 Provider enabled + non-deleted + same tenant。
   - update/delete/disable/enable 目标模型时，锁定目标模型；如涉及 Provider 迁移或启用，按 Provider -> model 顺序避免死锁。
   - 新增同一 tenant + provider + modelName + non-deleted 的重复检查。create 和 update modelName/providerId 都必须覆盖；同名但 deleted 的旧模型不阻塞。
4. `taskDefaults` 写路径：
   - 写入默认 Provider/model 时，在同一事务内锁定 Provider 和 model 并验证状态、归属和 provider/model 匹配。
   - 清空默认值仍可成功，但必须保留当前 audit 行为和响应形状。
   - 损坏持久化默认值的读取 fail-closed 行为不得回归。
5. 回归测试：
   - 补齐 Provider disable/delete、model create/update/enable/delete、taskDefaults update 的一致性负向测试。
   - 如果可行，增加 MySQL 或事务级并发测试；如果当前测试基建只支持 SQLite，需要用 deterministic transaction/locking helper 或明确的顺序化测试覆盖不变量，并在交付中说明并发真实锁依赖 MySQL。

安全要求：
- 所有查询和写入必须带 `tenant_id`，对象级权限和 RBAC 不能降级。
- 不得返回或记录 Provider 完整 API Key、Authorization、Cookie、JWT、base64 图片、MinIO bucket/object key 或底层数据库错误。
- 不得因为一致性检查暴露跨租户 Provider/model 是否存在。
- SSRF、Provider URL 校验和 API Key 加密逻辑不得削弱。
- 失败写入不得记录成功 operation log；成功 audit metadata 必须保持 sanitized。

必须保持的现有行为：
- 现有 Provider/model CRUD、enable/disable/delete API 路径和响应形状保持兼容。
- `taskDefaults` 只在任务创建同时省略 providerId/modelId 时生效；显式 Provider/model 请求不读取未使用的损坏默认值。
- disabled/deleted/cross-tenant Provider 或 model 仍返回通用 validation/not-found/forbidden 语义，不泄漏内部对象详情。
- Provider test、Provider Adapter runtime、Worker 执行、SSE、storage quota/retention/orphan cleanup 不受影响。
- 已有 P13/P14/P15/P17 安全与回归测试继续通过。

允许的中间态：
- 后端一致性更严格，可能把以前可写入但会造成不一致的请求改为 `422 VALIDATION_ERROR` 或既有 conflict 类错误。
- 同一 Provider 下 active modelName 重复被写路径阻止，但历史已有重复数据不做破坏性迁移。
- 不新增前端提示；前端沿用现有错误展示。

禁止的半迁移状态：
- Provider disable/delete 成功后仍存在 enabled linked models。
- Model enable/create/update 成功后指向 disabled、deleted、cross-tenant 或不匹配 Provider。
- `taskDefaults` 成功保存后指向 disabled、deleted、cross-tenant 或不匹配 Provider/model。
- 同一 tenant + Provider 下成功创建或更新出两个 non-deleted 同名 model。
- 失败请求写入成功 operation log。
- 为了实现校验而解密 Provider API Key、调用真实 Provider、修改 Worker/SSE/task execution 主流程或放宽 RBAC。

失败模式与边界场景：

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| Provider disable，存在 enabled linked model | 拒绝，Provider 状态不变，无成功 audit | API/服务测试 |
| Provider disable，只有 disabled linked model | 允许，状态变更并记录 sanitized audit | API/服务测试 |
| Provider delete，存在任意 non-deleted linked model | 拒绝，Provider 不删除，无成功 audit | API/服务测试 |
| Provider delete，只有 deleted linked model | 允许 soft-delete Provider | API/服务测试 |
| Model create 指向 disabled/deleted/cross-tenant Provider | 拒绝，不创建 model，无成功 audit | API/服务测试 |
| Model enable 时 Provider 已 disabled/deleted | 拒绝，model 仍 disabled | API/服务测试 |
| Model update 迁移到 disabled/deleted/cross-tenant Provider | 拒绝，原 Provider 保持 | API/服务测试 |
| Model create/update 造成 same Provider active modelName 重复 | 拒绝，不创建/不更新 | API/服务测试 |
| Model create/update 与 deleted 同名旧 model 冲突 | 允许 | API/服务测试 |
| taskDefaults 指向 disabled/deleted/cross-tenant/mismatched Provider/model | 拒绝，旧值保持，无成功 audit | settings/API 测试 |
| taskDefaults 清空 | 允许，响应和 audit 保持现有形状 | settings/API 测试 |
| 损坏已持久化 task_defaults 读取 | 继续 fail closed，不创建任务副作用 | task/API 回归 |
| 并发或交错 admin 写入 | 最终状态满足上述不变量；无法在 SQLite 精确验证时交付说明 MySQL 锁语义 | 服务/API 或集成测试 |

必须新增或更新的回归测试：
- Provider disable/delete 对 linked model 状态的事务内重检。
- Model create/update/enable 对 Provider 状态和租户归属的事务内重检。
- Model create/update 对 same Provider active modelName 重复的拒绝，以及 deleted 同名模型不阻塞。
- `taskDefaults` update 对 Provider/model 状态、归属和匹配关系的锁定校验。
- 失败路径无成功 operation log。
- 原有 task defaults malformed fail-closed 和 explicit Provider/model 忽略未用损坏默认值测试继续通过。

验收标准：
- Provider/model/default settings 写路径满足失败模式矩阵。
- 未修改公共合同文档、前端、部署脚本、Worker/SSE/task execution 主流程或 Provider Adapter runtime。
- 后端测试、race、vet、build 通过。
- Docker Compose config 和安全回归脚本通过。
- 子 agent 最终回复必须列出“每个 failure mode 对应的测试文件/测试名”，并说明是否使用共享本地 MySQL/Redis/MinIO 及清理情况。

测试命令：
```bash
cd backend
go test ./internal/provider ./internal/model ./internal/settings ./internal/api ./internal/task -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
bash scripts/security-regression.sh
git diff --check main...HEAD
```

交付要求：
- 提交到分支 `codex/p18-backend-provider-model-serialization`。
- 最终说明必须包含：修改文件清单、执行命令结果、failure mode 到测试名映射、安全自查、刻意未修改范围、是否有合同缺口。
```

## 最近已完成任务包：P17-BE-OBSERVABILITY-METRICS

### 调度决策

- 本任务串行执行，不与 Provider/model serialization、real Provider smoke、前端任务或部署 dry-run 并行。
- 理由：这是首个生产 diagnostics 公共合同，会触达 admin API、queue inspection、task/provider aggregate、storage usage 和 maintenance result 读取；先稳定只读诊断边界，再进入 P18 最终上线信心任务。
- 本任务只实现 backend read-only JSON diagnostics，不新增前端 UI，不新增 Prometheus/exporter，不触发 cleanup、Provider test 或任务执行。

### 任务信息

- 任务名称：`P17-BE-OBSERVABILITY-METRICS`
- 目标：增加 admin-only、tenant-scoped、read-only production diagnostics JSON API，用于查看 queue depth、running/failed tasks、Provider failure rate、storage usage 和 recent maintenance results，提升上线运维可见性。
- 推荐线程名：`P17-BE-OBSERVABILITY-METRICS`
- 推荐分支名：`codex/p17-backend-observability-metrics`
- 起始分支：已合并 `P17-BE-STORAGE-QUOTA-RESERVATION` 的最新 `main`
- 前置依赖：P17 orphan cleanup 和 P17 strict storage quota reservation 已合并；P9/P14 audit usage reads、P13/P16 runtime settings、P7/P10 queue/Worker/SSE 基础均已可用。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P17-BE-OBSERVABILITY-METRICS`。

你必须在分支 `codex/p17-backend-observability-metrics` 上工作；如果当前不在该分支，先执行 `git switch codex/p17-backend-observability-metrics`，确认 `git branch --show-current` 后再继续。起始点必须包含已合并 `P17-BE-STORAGE-QUOTA-RESERVATION` 的最新 `main`；如果 `git merge-base --is-ancestor main codex/p17-backend-observability-metrics` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
增加 backend admin-only JSON diagnostics 能力：
- 新增 `GET /api/v1/admin/diagnostics/summary`。
- 该接口只读、tenant-scoped、admin-only，并要求 `audit:read` 权限。
- 返回 queue depth、task status aggregates、Provider/API-call failure rate、storage usage aggregate、recent maintenance result aggregate。
- 只返回聚合和 sanitized samples，不返回 Redis keys、queue payload、claim IDs、raw Provider payload、operation/API log raw metadata、bucket、object key、MinIO URL、signed URL、image base64、Authorization、Cookie、JWT 或 Provider secrets。
- Redis/queue section 不可用时优先返回 section-level `status="unavailable"` 和 sanitized reason code，不泄漏连接串或底层错误；数据库主查询失败仍按现有 API 错误形状返回 sanitized 500。

允许修改文件：
- `backend/internal/api/**`
- `backend/internal/queue/**` 仅限新增只读 queue depth inspector 和测试
- `backend/internal/task/**` 仅限只读 aggregate helper 或测试；不得改 Worker 状态机
- `backend/internal/settings/**` 仅限读取 storage quota/public usage helper 或测试
- `backend/internal/asset/**` 仅限只读 storage aggregate helper 或测试
- `backend/internal/database/**` 仅限使用既有模型/必要测试 helper；默认不新增表和 migration
- backend-only 测试文件

禁止修改文件：
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `deploy/**`
- `scripts/**`
- Provider Adapter、Provider/model 管理写路径、认证/JWT/Cookie 主流程、SSE handler、Worker claim/cancel/retry/timeout 状态机、任务创建/执行语义
- 不新增 AI Provider 调用、Provider test 调用、API key 解密、cleanup 触发、任务 enqueue、前端轮询、浏览器存储、Prometheus/exporter 或公共 MinIO/signed URL

前置阅读：
- `docs/api-contract.md` 的 P17 diagnostics contract
- `docs/security.md` 的 P17 production diagnostics 安全要求
- `docs/development-plan.md` 的 P17 当前优先级
- `backend/internal/api/audit_usage_routes.go`
- `backend/internal/api/router.go`
- `backend/internal/queue/reliable_queue.go`
- `backend/internal/task/types.go`
- `backend/internal/database/models.go`
- `backend/internal/settings/quota.go`
- `backend/internal/asset/repository.go`
- `backend/cmd/worker/log_retention_maintenance.go`
- `backend/internal/asset/orphan_cleanup.go`

具体开发内容：
1. API 合同：
   - 注册 `GET /api/v1/admin/diagnostics/summary`。
   - 使用现有认证、tenant、admin/RBAC 中间件模式。
   - 要求 tenant admin 加 `audit:read` 权限；无权限返回现有 `403 FORBIDDEN` 形状。
   - 响应走现有统一 envelope。
2. Task diagnostics：
   - 返回当前 tenant 的 task counts by status，至少覆盖 queued/running/retrying/cancelling/succeeded/failed/cancelled/timed_out。
   - 返回 bounded recent failures，字段只允许 taskId、status、errorCode、sanitized errorMessage、updatedAt/finishedAt。
   - 支持 bounded `windowHours` 和 `limit`，默认值必须小且安全，最大值必须受限。
3. Queue diagnostics：
   - 新增只读 queue depth inspector，读取 pending/processing/delayed/dead counts。
   - 不返回 Redis key 名称、payload、claim ID、task ID 列表或 Redis 原始错误。
   - Redis 不可用时 diagnostics endpoint 不应泄漏内部错误；返回 queue section `status="unavailable"` 和 `reason="queue_unavailable"`。
4. Provider diagnostics：
   - 基于 `api_call_logs` 做 tenant-scoped aggregate。
   - 返回固定/请求窗口内 totalCalls、failedCalls、failureRate，以及 bounded by Provider 聚合。
   - Provider sample 只能包含 providerId、providerName 或 display-safe name、totalCalls、failedCalls、failureRate。
   - 不返回 raw request/response JSON、request ID 列表、Provider error raw body、API key hint 以外的任何 secret。
5. Storage diagnostics：
   - 返回 `storageQuota.maxBytes`、read-only `usedBytes`、asset counts、softDeleted count、purged count。
   - 不返回 reservation IDs、counter internals、bucket、object key、MinIO URL、signed URL。
   - 不使用 MinIO listing 计算 quota 或 asset usage；MySQL metadata / settings quota helper 仍是 truth。
6. Maintenance diagnostics：
   - 基于现有 operation logs 的 sanitized aggregate metadata 返回 latest maintenance summaries when available。
   - 至少覆盖 `storage.orphan_cleanup` 和 `log_retention.cleanup`；storage retention cleanup 如没有 operation log，只返回 `status="not_recorded"` 或省略，不要伪造成功状态。
   - 不返回 operation log raw metadata 中的敏感或过大字段；只提取安全的 processed/deleted/failed/candidates/status/counts/timestamps。
7. 错误、安全与边界：
   - 所有查询必须带 tenant_id 过滤。
   - 所有 limit/window 参数必须校验并 clamp。
   - 所有 response strings 必须脱敏并限制长度。
   - 不写 operation log，因为该接口是只读 diagnostics read；如项目已有 read-log 策略则沿用，不新增高噪声日志。

安全要求：
- 不允许泄漏 Redis keys、queue payload、claim IDs、raw task params、raw prompt、raw Provider request/response metadata、API key、Authorization、Cookie、JWT、image base64、bucket、object key、MinIO URL、signed URL。
- 不允许跨 tenant 读取任何 task、asset、usage、API call、operation log、Provider、model 或 setting 数据。
- 不允许在 diagnostics 中调用 Provider、解密 Provider key、执行 cleanup、enqueue task、修改 task state、修改 settings 或写 MinIO。
- Section-level unavailable 必须是 sanitized reason code，不得包含 DSN、host、password、Redis key、SQL 或 stack trace。

必须保持的现有行为：
- 现有 `/admin/usage/*`、`/admin/operation-logs`、`/admin/api-call-logs*` 行为不变。
- 现有 `/healthz` 和 `/api/v1/healthz` 行为不变。
- 现有 task creation、queue claim、Worker execution、SSE replay、quota reservation、orphan cleanup、log retention cleanup 不变。
- Frontend 不变。

允许的中间态：
- 可以先实现 backend-only JSON diagnostics；前端 UI、Prometheus/exporter 和 alerting 留给后续任务。
- 可以对 Redis queue metrics 做 best-effort unavailable section；不能因此阻塞数据库-backed diagnostics。

禁止的半迁移状态：
- diagnostics endpoint 读取跨租户数据。
- diagnostics endpoint 返回 Redis key/payload、object key/bucket、raw log metadata、raw Provider payload 或 secrets。
- 为了展示 maintenance result 触发 cleanup 或修改任何状态。
- 为了展示 queue depth 改变 queue 数据结构或 Worker claim/ack/retry 行为。
- 新增 active writable setting 但没有 runtime consumer。
- 前端开始轮询 diagnostics。

失败模式与边界场景：

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| 未登录 | 401 | 是 |
| 非 admin 或缺少 `audit:read` | 403 | 是 |
| tenant A 请求 | 只返回 tenant A 聚合，不包含 tenant B 数据 | 是 |
| 默认查询参数 | 返回 bounded aggregates | 是 |
| `windowHours` / `limit` 非法或过大 | 422 或 clamp 到安全上限，行为需测试明确 | 是 |
| Redis/queue unavailable | endpoint 返回 200，queue section `status=unavailable`，无底层错误泄漏 | 是 |
| DB 查询失败 | sanitized 500，不泄漏 SQL/table/stack | 是 |
| Provider failure rate | 基于 tenant-scoped `api_call_logs` 聚合，不返回 raw JSON | 是 |
| Storage usage | 使用 settings/quota metadata truth，不使用 MinIO listing，不暴露 object identifiers | 是 |
| Maintenance logs absent | 返回 empty/not_recorded，不伪造成功 | 是 |
| Maintenance metadata 含敏感字段 | response 不包含敏感字段或超长 raw metadata | 是 |

必须新增或更新的回归测试：
- `backend/internal/api/**`：diagnostics auth/RBAC、tenant isolation、response shape、query validation、Redis unavailable、DB failure sanitization。
- `backend/internal/queue/**`：queue depth inspector 只读读取 pending/processing/delayed/dead counts，错误脱敏。
- `backend/internal/api/**` 或相关 backend tests：Provider failure aggregate、task status aggregate、storage usage aggregate、maintenance latest result aggregate。
- forbidden response tests：断言不包含 `tenants/`、bucket name、objectKey、Redis key、Authorization、Cookie、JWT、base64、raw request/response JSON、Provider key marker。

测试命令：
```bash
cd backend
go test ./internal/api ./internal/queue ./internal/task ./internal/settings ./internal/asset -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
bash scripts/security-regression.sh
git diff --check main...HEAD
```

最终交付必须包含：
- 修改文件清单。
- 执行的测试命令和结果。
- failure matrix 到具体测试文件/测试名的映射。
- 安全自查结果，明确无 Redis key/payload、object key/bucket、raw metadata、Provider secret、Authorization/Cookie/JWT/image base64 泄漏。
- 刻意未修改范围，特别说明 frontend UI、Prometheus/exporter、Provider/model serialization、real Provider smoke、cleanup triggers 不在本任务内。
- 如使用共享本地 MySQL/Redis/MinIO，说明创建/修改/清理了哪些 `codex_p17_observability_metrics_*` 测试数据；默认优先使用自动化测试和 fake Redis/store。
- 如发现公共合同缺口，只报告主 agent，不修改 docs。
```

### 允许修改文件

- `backend/internal/api/**`
- `backend/internal/queue/**` 仅限只读 queue depth inspector 和测试
- `backend/internal/task/**` 仅限只读 aggregate helper 或测试
- `backend/internal/settings/**` 仅限读取 storage quota/public usage helper 或测试
- `backend/internal/asset/**` 仅限只读 storage aggregate helper 或测试
- `backend/internal/database/**` 仅限既有模型使用或测试 helper；默认不新增 migration
- backend-only 测试文件

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `deploy/**`
- `scripts/**`
- Provider Adapter、Provider/model 管理写路径、认证/JWT/Cookie 主流程、SSE handler、Worker claim/cancel/retry/timeout 状态机、任务创建/执行语义

### 验收标准

- `GET /api/v1/admin/diagnostics/summary` 可返回 tenant-scoped diagnostics summary。
- 权限、tenant isolation、query bounds、Redis unavailable、DB failure、敏感字段脱敏均有测试。
- Endpoint 不修改状态、不调用 Provider、不触发 cleanup、不 enqueue task。
- 不泄漏 Redis key/payload、object key/bucket/MinIO URL、raw Provider/log metadata、Authorization/Cookie/JWT、Provider secret 或 image base64。
- 现有 admin usage/audit/settings/task/queue/Worker/SSE 行为不回归。

### 测试命令

```bash
cd backend
go test ./internal/api ./internal/queue ./internal/task ./internal/settings ./internal/asset -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
bash scripts/security-regression.sh
git diff --check main...HEAD
```

## 最近已完成任务包：P17-BE-STORAGE-QUOTA-RESERVATION

### 调度决策

- 本任务串行执行，不与 observability metrics、Provider/model serialization 或前端任务并行。
- 理由：严格 quota reservation 会触达数据库 schema、settings quota 读取、reference upload、Worker output persistence、physical purge cleanup 和系统设置 usedBytes 语义；必须先稳定写入路径，再做生产 diagnostics。
- 本任务保持现有 `storageQuota` API 形状不变，不新增前端 UI，不把内部 reservation ID 暴露给浏览器。

### 任务信息

- 任务名称：`P17-BE-STORAGE-QUOTA-RESERVATION`
- 目标：为并发 reference upload 和 Worker generated/edited output 写入增加 tenant-scoped strict storage quota reservation/counter 与 reconciliation，消除当前 metadata-sum check 的并发超额风险。
- 推荐线程名：`P17-BE-STORAGE-QUOTA-RESERVATION`
- 推荐分支名：`codex/p17-backend-storage-quota-reservation`
- 起始分支：已合并 `P17-BE-ORPHAN-CLEANUP` 的最新 `main`
- 前置依赖：P13 storage quota accounting、P13 storage cleanup foundation、P13 storage retention runtime、P16 thumbnail policy、P17 orphan cleanup 已合并。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P17-BE-STORAGE-QUOTA-RESERVATION`。

你必须在分支 `codex/p17-backend-storage-quota-reservation` 上工作；如果当前不在该分支，先执行 `git switch codex/p17-backend-storage-quota-reservation`，确认 `git branch --show-current` 后再继续。起始点必须包含已合并 `P17-BE-ORPHAN-CLEANUP` 的最新 `main`；如果 `git merge-base --is-ancestor main codex/p17-backend-storage-quota-reservation` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
当前 storage quota enforcement 通过读取 `image_assets.size_bytes` 聚合值做写入前检查；并发 reference upload 或 Worker output 可同时通过检查并共同超过 `storageQuota.maxBytes`。本任务要增加严格 reservation/counter：
- Reference upload 在写 MinIO 前 reserve 原图 bytes；成功 metadata transaction 内 finalize；失败路径 release。
- Worker output persistence 在写 generated/edited objects 前 reserve 所有 pending output 原图 bytes；成功 metadata transaction 内 finalize；失败、取消、超时、重复输出、storage/DB error 路径 release。
- `storageQuota.usedBytes` 对外仍是只读字段；内部可使用 counter/reservation 表，但不得返回 internal reservation IDs。
- MySQL `image_assets` metadata 仍是 reconciliation source of truth；MinIO listing 不得成为 quota truth。
- Soft delete 不释放 used bytes；physical purge 后才释放或通过 reconciliation 扣减 used bytes。

允许修改文件：
- `backend/internal/database/**`
- `backend/internal/settings/**`
- `backend/internal/asset/**`
- `backend/internal/task/**`
- `backend/internal/api/**` 仅限既有 quota/asset/task/settings 测试或必要错误映射
- `backend/cmd/worker/**` 仅限 cleanup/reconciliation maintenance 如确有 runtime consumer 需要
- backend-only 测试文件

禁止修改文件：
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `deploy/**`
- `scripts/**`
- Provider Adapter、Provider/model 管理、认证/JWT/Cookie 主流程、SSE handler、Redis queue、Worker claim/cancel/retry/timeout 状态机
- 不新增 AI Provider 直连、Provider key 处理、前端轮询、浏览器存储、公共 MinIO URL、signed URL、bucket/object-key 暴露

前置阅读：
- `docs/storage.md` 的 P17 storage quota reservation target
- `docs/api-contract.md` 的 `storageQuota` 合同
- `docs/database-schema.md` 的 quota reservation 规划
- `docs/security.md` 的 quota 安全要求
- `backend/internal/settings/service.go`
- `backend/internal/settings/repository.go`
- `backend/internal/settings/types.go`
- `backend/internal/asset/service.go`
- `backend/internal/asset/cleanup.go`
- `backend/internal/task/runtime_persistence.go`
- `backend/internal/task/worker.go`
- `backend/internal/database/migrations.go`
- 现有 quota 测试：`backend/internal/settings/service_test.go`、`backend/internal/api/system_settings_routes_test.go`、`backend/internal/api/asset_routes_test.go`、`backend/internal/task/worker_test.go`

具体开发内容：
1. 数据模型与迁移：
   - 可新增 tenant-scoped quota counter/reservation 表，例如 `storage_quota_counters`，字段至少应支持 `tenant_id`、`used_bytes`、`reserved_bytes`、`updated_at`、`reconciled_at`。
   - 如需要 reservation 明细表，可新增 `storage_quota_reservations`，必须包含 `tenant_id`、reservation id、bytes、status、expires_at、created_at、updated_at。
   - 所有业务表必须包含 `tenant_id`，并建立必要唯一索引/查询索引。
   - 迁移必须可重复运行，不能破坏已有 `image_assets`、`system_settings` 或任务数据。
2. Reservation service：
   - 增加明确的后端 API/内部接口，例如 `ReserveStorageQuota`、`FinalizeStorageQuotaReservation`、`ReleaseStorageQuotaReservation`、`ReconcileStorageQuotaCounter`。
   - Reserve 必须在 DB transaction 中锁定 tenant counter row，读取有效 `storage_quota.maxBytes`，校验 `used + reserved + pending <= maxBytes`。
   - `maxBytes=null` 时应保持 unlimited 行为；可以 no-op reservation，但接口行为必须清晰。
   - Malformed persisted quota 或 malformed counter 必须 fail closed。
   - Release 必须 idempotent；Finalize 必须 idempotent 或在重复调用时安全返回。
   - Reservation 过期或 cleanup 可通过 reconciliation 修复，但不得让 stale reservations 永久阻断 tenant。
3. Reference upload 接入：
   - 在 validated image bytes 和 quota pending bytes 确定后、写 MinIO 前 reserve。
   - Original + thumbnail upload 成功后，在 metadata/audit DB transaction 中创建 asset row 并 finalize reservation。
   - 任何 validation/storage/metadata/audit failure 均必须 release reservation，并保留现有 uploaded object rollback 行为。
   - Quota exceeded 仍返回现有 sanitized `409 STORAGE_QUOTA_EXCEEDED`，不写 successful asset row，不写成功 audit，不泄漏 object key。
4. Worker output 接入：
   - 在 pending outputs 和 pendingBytes 确定后、写 generated/thumbnail MinIO 前 reserve。
   - 成功 DB transaction 中创建 assets/task_outputs/events/usage 时 finalize reservation。
   - Provider output validation failure、quota exceeded、storage failure、DB failure、task no longer running、duplicate output race、context canceled/deadline exceeded 等路径必须 release 或避免创建 reservation。
   - 不改变 Worker claim/cancel/retry/timeout 状态机。
5. Physical cleanup / retention 接入：
   - Soft delete 不改变 quota used bytes。
   - P13 physical purge 成功后必须 decrement counter 或触发 reconciliation；missing object idempotent success 要保持一致。
   - Orphan cleanup 删除非 metadata 对象时不得改变 quota counter。
6. Reconciliation：
   - 增加从 MySQL metadata 重建 tenant quota counter 的能力，至少用于测试和 repair path。
   - `storageQuota.usedBytes` 应反映 counter used bytes；如果 counter 不存在，应先基于 metadata 初始化或安全回退。
   - Reconciliation 不能使用 MinIO listing 作为 truth。
7. 错误、安全与审计：
   - 内部 reservation ID、counter row、lock detail 不得出现在 API response、operation log、task event 或错误消息中。
   - 只记录 sanitized aggregate metadata，例如 `pendingBytes`、`maxBytesSet`、`reservationResult`、`reason`，不要记录 object key/bucket/MinIO URL。
   - Quota failure、release failure、reconcile failure 的日志必须脱敏。

安全要求：
- 所有 quota/counter/reservation 查询必须 tenant-scoped。
- 不能把 internal reservation ID、bucket、object key、MinIO URL、signed URL、image base64、Authorization、Cookie、JWT、Provider Key 写入响应、日志、audit metadata 或 task event。
- 不能为了 quota reservation 改成 Redis 最终状态；MySQL 仍是 quota/counter 和 asset metadata truth。
- 不允许使用 MinIO bucket listing 计算 quota used bytes。
- 不得 drop 数据库、删除共享 bucket、flush Redis 或清空 MinIO。
- 如使用共享本地 MySQL/Redis/MinIO 验证，只能创建带 `codex_p17_quota_reservation_` 前缀或明显测试路径的数据，并在最终交付说明是否清理。

必须保持的现有行为：
- `GET/PATCH /api/v1/admin/system-settings` 的 `storageQuota.maxBytes` / read-only `usedBytes` API 形状不变。
- `maxBytes=null` 仍表示 unlimited。
- Reference upload validation、SVG 禁止、thumbnail generation、rollback cleanup、asset audit、download/detail/list/favorite/update/delete 不回归。
- Worker output idempotency、usage/API logs、task events、SSE replay、不重复 output index 不回归。
- Storage retention physical purge 和 P17 orphan cleanup 不回归。
- Frontend 不变。

允许的中间态：
- 可以新增内部 quota service 和 migration，再接 reference upload，最后接 Worker output 和 purge/reconcile。
- 可以保留公共 API response 字段不变，只改变内部 `usedBytes` 来源。
- 可以新增 backend-only repair/reconcile helper，不需要新增前端 UI。

禁止的半迁移状态：
- Reference upload 使用 reservation，但 Worker output 仍绕过 reservation。
- Reservation 成功后，失败路径不 release，导致 tenant 永久被 stale reserved bytes 阻塞。
- Metadata 成功但 counter finalize 失败且无修复路径。
- Soft delete 立即释放 quota used bytes。
- Orphan cleanup 删除非 metadata object 时修改 quota counter。
- API response、audit、task event 泄漏 reservation id、bucket、object key 或 MinIO URL。
- 使用 Redis 作为 quota 最终来源。
- 使用 MinIO listing 作为 usedBytes truth。

失败模式与边界场景：

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| maxBytes=null | upload/Worker 不受 quota 限制，reservation no-op 或安全通过 | 是 |
| 单个 upload within quota | reserve -> MinIO write -> metadata transaction finalize，usedBytes 增加 | 是 |
| 单个 upload exceeds quota | 409，无 MinIO write、无 asset row、无成功 audit、无 reservation leak | 是 |
| 两个并发 uploads 各自单独可通过但合计超额 | 只有一个成功，另一个 quota exceeded 或等待后失败，最终 usedBytes 不超额 | 是 |
| thumbnail upload failure | original cleanup + reservation release，无 asset row | 是 |
| metadata/audit transaction failure after MinIO write | object cleanup + reservation release，无 stale reservation | 是 |
| Worker multi-output within quota | 一次性 reserve pending bytes，成功 finalize，assets/task_outputs/events 正常 | 是 |
| Worker output quota exceeded | task 失败为现有 sanitized quota error，无 output assets/events/usage 成功副作用 | 是 |
| Worker DB transaction failure after object writes | cleanup generated/thumbnail objects + reservation release | 是 |
| duplicate output index / already persisted output | 不重复 reserve/finalize，不重复 asset/output | 是 |
| soft delete | usedBytes 不减少 | 是 |
| physical purge success | usedBytes 减少或 reconciliation 后减少 | 是 |
| orphan cleanup deletes non-metadata object | usedBytes 不变 | 是 |
| malformed stored quota/counter | fail closed，不写 successful asset/task side effects | 是 |
| stale reservation | reconciliation 或 cleanup 后不会永久阻塞 tenant | 是 |

必须新增或更新的回归测试：
- `backend/internal/database/**`：migration/schema/index/idempotency 覆盖 quota counter/reservation 表。
- `backend/internal/settings/**`：reserve/finalize/release/reconcile、malformed counter fail-closed、usedBytes 来源、并发 reservation。
- `backend/internal/asset/**` 或 `backend/internal/api/asset_routes_test.go`：reference upload 成功、quota exceeded、并发超额、thumbnail/storage/metadata failure release。
- `backend/internal/task/worker_test.go`：Worker multi-output reservation、quota exceeded、storage/DB failure release、duplicate output idempotency。
- `backend/internal/asset/cleanup_test.go`：physical purge 后 counter/reconcile 正确，orphan cleanup 不影响 quota counter。
- 现有 `system_settings` quota API 测试需继续证明 public contract 不变。

测试命令：
```bash
cd backend
go test ./internal/database ./internal/settings ./internal/asset ./internal/api ./internal/task -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
bash scripts/security-regression.sh
git diff --check main...HEAD
```

最终交付必须包含：
- 修改文件清单。
- 执行的测试命令和结果。
- failure matrix 到具体测试文件/测试名的映射。
- 安全自查结果，明确无 reservation id/object key/bucket/MinIO URL 泄漏，无 Redis-as-truth，无 MinIO-listing-as-quota-truth，无前端轮询或浏览器存储变更。
- 刻意未修改范围，特别说明 frontend settings UI、observability metrics、Provider/model serialization、real Provider smoke 不在本任务内。
- 如使用共享本地 MySQL/Redis/MinIO，说明创建/修改/清理了哪些 `codex_p17_quota_reservation_*` 测试数据；默认优先使用自动化测试和 fake store。
- 如发现公共合同缺口，只报告主 agent，不修改 docs。
```

### 允许修改文件

- `backend/internal/database/**`
- `backend/internal/settings/**`
- `backend/internal/asset/**`
- `backend/internal/task/**`
- `backend/internal/api/**` 仅限相关后端测试和错误映射
- `backend/cmd/worker/**` 仅限必要 cleanup/reconciliation runtime
- backend-only 测试文件

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `deploy/**`
- `scripts/**`
- Provider Adapter、Provider/model 管理、认证/JWT/Cookie 主流程、SSE handler、Redis queue、Worker claim/cancel/retry/timeout 状态机

### 验收标准

- 并发 reference upload / Worker output 无法突破 tenant `storageQuota.maxBytes`。
- Reservation/finalize/release/reconcile 均 tenant-scoped、idempotent 或 retry-safe。
- 失败路径不留下 stale reservation、成功 asset row 指向缺失对象、成功 task output event 或敏感日志。
- `storageQuota.usedBytes` 保持只读且与 quota counter/reconciliation 一致。
- Soft delete、physical purge、orphan cleanup 与 quota accounting 语义正确。
- 现有前端和 API contract 不变。

### 测试命令

```bash
cd backend
go test ./internal/database ./internal/settings ./internal/asset ./internal/api ./internal/task -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
bash scripts/security-regression.sh
git diff --check main...HEAD
```

## 最近已完成任务包：P17-BE-ORPHAN-CLEANUP

### 调度决策

- 本任务串行执行，不与 quota reservation、observability metrics 或 Provider/model serialization 并行。
- 理由：orphan cleanup 会触达 MinIO listing、对象删除、资产 metadata truth、审计日志和生产运维安全边界；先把保守扫描和删除合同稳定下来，再推进严格 quota reservation。
- 本任务只实现后端 owned orphan scan / dry-run / cleanup 能力。不得把它做成 system-settings 字段，也不得暴露 bucket/object key 给前端。

### 任务信息

- 任务名称：`P17-BE-ORPHAN-CLEANUP`
- 目标：为 MinIO 中不再被 MySQL 可信 metadata 引用的对象增加 conservative discovery、dry-run、execution、retry-safe failure handling 和 sanitized audit 支持。
- 推荐线程名：`P17-BE-ORPHAN-CLEANUP`
- 推荐分支名：`codex/p17-backend-orphan-cleanup`
- 起始分支：已完成 R16 的最新 `main`
- 前置依赖：P16 thumbnail policy 已合并，`image_assets.object_key`、`image_assets.thumbnail_object_key`、P13 physical cleanup foundation、P16 thumbnail bucket、operation logs、RBAC/auth middleware 和 storage ObjectStore 均可复用。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P17-BE-ORPHAN-CLEANUP`。

你必须在分支 `codex/p17-backend-orphan-cleanup` 上工作；如果当前不在该分支，先执行 `git switch codex/p17-backend-orphan-cleanup`，确认 `git branch --show-current` 后再继续。起始点必须包含已完成 R16 的最新 `main`；如果 `git merge-base --is-ancestor main codex/p17-backend-orphan-cleanup` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
为生产存储治理增加后端 conservative orphan cleanup 能力：
- 能扫描 configured originals/generated/thumbnails buckets 中符合后端对象命名规则的对象。
- 能与 MySQL `image_assets.object_key` / `image_assets.thumbnail_object_key` 可信 metadata 对比，识别“可疑 orphan candidate”。
- 默认 dry-run，不删除对象。
- 显式 cleanup 时必须 batch-limited、age-gated、tenant-scoped、retry-safe、auditable，并且不泄漏 bucket/object key/MinIO URL。
- 删除 eligibility 必须同时满足 recognized backend object-key pattern、tenant scope、older-than grace period、未被可信 MySQL metadata 引用。禁止只因为 bucket listing 里有陌生对象就删除。

允许修改文件：
- `backend/internal/storage/**`
- `backend/internal/asset/**`
- `backend/internal/api/**`
- `backend/internal/audit/**` 仅限复用或补充测试 helper；不要改变既有 audit 公共行为
- `backend/internal/config/**` 仅限新增 orphan cleanup 的安全默认配置或测试覆盖
- `backend/internal/database/**` 仅限查询 helper 或测试 schema 补齐；默认不要新增表或迁移，除非实现无法审计/重试并先在交付中说明
- `backend/internal/httpx/**` 仅限复用现有错误响应；一般不应修改
- backend-only 测试文件

禁止修改文件：
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `deploy/**`
- `scripts/**`
- Provider Adapter、Provider/model 管理、认证/JWT/Cookie 主流程、SSE、Redis queue、Worker claim/cancel/retry/timeout 状态机、task execution 主流程、system settings、log retention runtime、thumbnail 生成算法
- 不新增 AI Provider 直连、Provider key 处理、前端轮询、浏览器存储、公共 MinIO URL、signed URL 或 object-key 暴露

前置阅读：
- `docs/api-contract.md` 的 Admin Storage Orphan APIs
- `docs/storage.md` 的 P17 orphan cleanup target
- `docs/security.md` 的 P17 orphan cleanup 安全要求
- `backend/internal/storage/store.go`
- `backend/internal/asset/cleanup.go`
- `backend/internal/asset/repository.go`
- `backend/internal/asset/service.go`
- `backend/internal/api/router.go`
- `backend/internal/api/*_routes_test.go` 中 admin/auth/RBAC 测试模式
- `backend/internal/audit/recorder.go`

具体开发内容：
1. Storage listing abstraction：
   - 为 ObjectStore 增加受控 listing 能力，或新增最小接口，只允许后端代码按 bucket/prefix/cursor/batch limit 列出对象 metadata。
   - MinIO 实现必须支持 bounded listing，不得一次性加载整个 bucket。
   - fake/in-memory store 测试实现必须覆盖 list cursor、not-found delete、delete failure 等情况。
2. Orphan candidate 识别：
   - 只识别后端生成的对象路径：
     - `tenants/{tenantId}/projects/{projectId}/assets/{assetId}/original.{ext}`
     - `tenants/{tenantId}/projects/{projectId}/assets/{assetId}/thumbnail.jpg`
     - 当前 generated/edited output object key 既有格式也必须覆盖，以代码实际格式为准。
   - 解析失败、tenant/project/asset ID 异常、bucket kind 不匹配、对象过新、tenant 不匹配、对象仍被 `image_assets.object_key` 或 `image_assets.thumbnail_object_key` 引用时，一律 skip。
   - 候选必须 older-than configurable/default grace period。建议默认最小 24 小时；测试可注入更小时间。
   - MySQL metadata 是可信 truth。不要以 MinIO listing 反推资产存在。
3. Admin API：
   - 新增 `POST /api/v1/admin/storage/orphans/scan`，dry-run only。
   - 新增 `POST /api/v1/admin/storage/orphans/cleanup`，默认 dry-run；只有 `dryRun=false` 且 `confirm="DELETE_ORPHANS"` 时才执行删除。
   - 要求 tenant admin 或 `system:settings:manage` 权限；不引入新权限码，除非发现现有 RBAC 无法表达并先报告。
   - Tenant admin 只能操作自己的 tenant；如果请求中带其他 tenant ID，必须拒绝或忽略为当前 tenant，不能跨租户扫描/删除。
   - Response 只返回 aggregate counts、bucket kind、tenantId、skipped/error categories、candidate hashes/opaque IDs。禁止返回 raw bucket/object key/MinIO endpoint。
4. Execution / retry / audit：
   - Cleanup 必须 batch-limited；达到 batch limit 时返回 `hasMore` 或 cursor 信息，供后续调用继续。
   - 删除 missing object 应视为 idempotent success。
   - 删除失败必须记录 sanitized error kind，不能中断已完成对象的结果，但整体响应要体现 failed count。
   - 失败 candidate 后续 scan 应仍可发现，方便 retry。
   - 执行 cleanup 必须写 operation log，metadata 只包含 aggregate counts、bucket kinds、tenantId、dryRun/execute、skipped/error categories 和 hash/opaque sample，不包含 raw object key。
   - Dry-run 可以写 `storage.orphan.scan` operation log；execute 写 `storage.orphan.cleanup`。
5. 错误与安全：
   - 所有错误走现有统一错误响应，保持 sanitized。
   - Storage unavailable、list failure、delete failure、malformed request、permission denied、cross-tenant request 均需有明确测试。
   - 不要把 orphan cleanup 暴露到 frontend UI；本任务是后端/API 能力。

安全要求：
- 不允许基于 bucket listing 单独删除对象。
- 不允许删除不符合后端对象命名规则的对象；陌生对象必须 skip。
- 不允许返回或记录 raw bucket name、object key、MinIO URL、signed URL、image base64、Authorization、Cookie、JWT、Provider Key。
- 所有查询必须 tenant-scoped，所有 object ID 或 candidate 操作必须做 object/tenant-level 判断。
- Dry-run 是默认行为；执行删除必须显式确认。
- 不得 drop 数据库、删除共享 bucket、flush Redis 或清空 MinIO。
- 如果使用共享本地 MySQL/Redis/MinIO 验证，只能创建带 `codex_p17_orphan_cleanup_` 前缀或明显测试路径的数据，并在交付中说明是否清理。

必须保持的现有行为：
- 正常 asset upload/download/detail/list/delete/favorite/update 不回归。
- P13 soft-delete physical cleanup foundation 不回归。
- P16 thumbnail generation/download/cleanup 不回归。
- Storage quota、storage retention、log retention、task Worker 输出、SSE、Provider Adapter 运行时不回归。
- Frontend 不变。

允许的中间态：
- 可以先实现 service + focused tests，再接 admin routes。
- 可以先只支持 originals/generated/thumbnails 当前配置 bucket；不需要支持任意 bucket 名。
- 可以不新增数据库表，使用 operation logs 记录 aggregate cleanup result。

禁止的半迁移状态：
- API 返回 candidate object keys 或 bucket names。
- Cleanup endpoint 默认执行删除。
- 只按 bucket listing 删除对象。
- 跨租户 admin 可以扫描/删除其他 tenant 对象。
- 删除仍被 MySQL metadata 引用的 original 或 thumbnail object。
- Listing 一次性读完整 bucket，缺少 batch/cursor/limit。
- Storage delete 失败后响应假装全部成功。
- 为 orphan cleanup 新增 system-settings writable 字段但没有真实 runtime consumer。

失败模式与边界场景：

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| dry-run 扫描有 orphan candidate | 返回 sanitized aggregate 和 candidate hash，不删除对象 | 是 |
| cleanup 未传 confirm 或 dryRun 省略 | 保持 dry-run，不删除对象 | 是 |
| cleanup dryRun=false 且 confirm 正确 | 只删除 eligible candidate，写 sanitized audit | 是 |
| 对象 key 格式陌生 | skip，不删除，不返回 raw key | 是 |
| 对象仍被 object_key 引用 | skip，不删除 | 是 |
| 对象仍被 thumbnail_object_key 引用 | skip，不删除 | 是 |
| 对象过新 | skip，不删除 | 是 |
| 跨租户请求 | 拒绝或限定当前 tenant，不能扫描/删除其他 tenant | 是 |
| 非 admin/无权限用户 | 403，不能泄漏 candidate 存在性 | 是 |
| storage list 失败 | sanitized error 或 failed count，无 raw storage path | 是 |
| delete not found | idempotent success | 是 |
| delete failed | failed count/error kind，candidate 后续可 retry | 是 |
| batch limit 达到 | 返回 hasMore/cursor，不超量删除 | 是 |

必须新增或更新的回归测试：
- `backend/internal/storage/**`：listing abstraction/fake store cursor、delete not-found/delete failure。
- `backend/internal/asset/**`：orphan candidate parsing、metadata exclusion、age gate、tenant scope、dry-run no delete、execute delete、failed delete retry-safe、audit metadata sanitized。
- `backend/internal/api/**`：admin scan/cleanup auth/RBAC、CSRF、dry-run default、explicit confirm execute、cross-tenant denial、response no raw object key/bucket/MinIO URL。
- 如修改 config，补 `backend/internal/config/**` 测试。

测试命令：
```bash
cd backend
go test ./internal/storage ./internal/asset ./internal/api -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
bash scripts/security-regression.sh
git diff --check main...HEAD
```

最终交付必须包含：
- 修改文件清单。
- 执行的测试命令和结果。
- failure matrix 到具体测试文件/测试名的映射。
- 安全自查结果，明确没有 raw object key/bucket/MinIO URL 泄漏，没有 bucket-listing-only deletion，没有跨租户扫描/删除，没有 Provider 直连、前端轮询或浏览器存储变更。
- 刻意未修改范围，特别说明 strict quota reservation、frontend UI、Prometheus/metrics、Provider/model serialization、real Provider smoke 不在本任务内。
- 如使用共享本地 MySQL/Redis/MinIO，说明创建/修改/清理了哪些 `codex_p17_orphan_cleanup_*` 测试数据；默认优先使用自动化测试和 fake store。
- 如发现公共合同缺口，只报告主 agent，不修改 docs。
```

### 允许修改文件

- `backend/internal/storage/**`
- `backend/internal/asset/**`
- `backend/internal/api/**`
- `backend/internal/audit/**` 仅限测试/helper
- `backend/internal/config/**` 仅限必要配置和测试
- `backend/internal/database/**` 仅限必要查询 helper 或测试 schema
- backend-only 测试文件

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `deploy/**`
- `scripts/**`
- Provider Adapter、Provider/model 管理、认证/JWT/Cookie 主流程、SSE、Redis queue、Worker claim/cancel/retry/timeout 状态机、task execution 主流程、system settings、log retention runtime、thumbnail 生成算法

### 验收标准

- Admin dry-run scan 能发现 eligible orphan candidate，但不删除对象。
- Admin cleanup 只有在 `dryRun=false` 且确认字符串正确时执行删除。
- Candidate eligibility 同时依赖 recognized backend object-key pattern、tenant scope、age gate 和 MySQL metadata exclusion。
- Response 和 operation log 不包含 raw bucket、object key、MinIO URL、signed URL 或敏感凭据。
- Listing 和 cleanup batch-limited，支持 retry-safe failed delete。
- 正常 asset upload/download/thumbnail/retention/quota/Worker 路径不回归。

### 测试命令

```bash
cd backend
go test ./internal/storage ./internal/asset ./internal/api -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
bash scripts/security-regression.sh
git diff --check main...HEAD
```

## 最近已完成任务包：P16-BE-THUMBNAIL-POLICY

### 调度决策

- 本任务串行执行，不与 orphan cleanup、quota reservation 或前端 UI 任务并行。
- 理由：缩略图会进入资产上传、Worker 输出持久化、MinIO 对象生命周期、资产响应 URL 和 cleanup rollback；先稳定后端合同，再决定是否需要前端展示优化。
- 本任务只处理新资产的缩略图生成与鉴权访问。既有资产 backfill、MinIO orphan discovery、手动 cleanup trigger 和缩略图字节 quota 计数不在本任务范围内。

### 任务信息

- 任务名称：`P16-BE-THUMBNAIL-POLICY`
- 目标：为新 reference upload 和 Worker 生成/编辑输出资产生成 MinIO thumbnail object，保存 `thumbnail_object_key`，并通过后端鉴权 endpoint 提供 `thumbnailUrl`。
- 推荐线程名：`P16-BE-THUMBNAIL-POLICY`
- 推荐分支名：`codex/p16-backend-thumbnail-policy`
- 起始分支：已合并 `P16-BE-LOG-RETENTION` 的最新 `main`
- 前置依赖：P16 deployment script hardening 和 backend log retention 已合并；现有 `image_assets.thumbnail_object_key`、`MINIO_BUCKET_THUMBNAILS`、asset cleanup foundation、reference upload、Worker output persistence、storage quota accounting 均可复用。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P16-BE-THUMBNAIL-POLICY`。

你必须在分支 `codex/p16-backend-thumbnail-policy` 上工作；如果当前不在该分支，先执行 `git switch codex/p16-backend-thumbnail-policy`，确认 `git branch --show-current` 后再继续。起始点必须包含已合并 `P16-BE-LOG-RETENTION` 的最新 `main`；如果 `git merge-base --is-ancestor main codex/p16-backend-thumbnail-policy` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
为新创建的图片资产落地生产缩略图策略：
- Reference upload 成功创建资产时，同步生成 bounded thumbnail，写入 MinIO thumbnails bucket，并把 `thumbnail_object_key` 写入 `image_assets.thumbnail_object_key`。
- Worker 持久化 generated/edited 输出资产时，同步生成 bounded thumbnail，写入 MinIO thumbnails bucket，并把 `thumbnail_object_key` 写入资产 metadata。
- 资产 list/detail/history/task output event 只在 `thumbnail_object_key` 存在时返回同源后端 `thumbnailUrl`，推荐路径 `/api/v1/assets/{assetId}/thumbnail`。
- 新增 `GET /api/v1/assets/{assetId}/thumbnail`，必须经过登录、tenant、project/member/RBAC/object authorization，再从 thumbnails bucket stream 图片。
- 不 backfill 既有资产；既有无缩略图资产保持 `thumbnailUrl=""` 或缺省空值。

允许修改文件：
- `backend/internal/asset/**`
- `backend/internal/task/runtime_persistence.go`
- `backend/internal/task/types.go`
- `backend/internal/task/worker_test.go`
- `backend/internal/api/asset_routes_test.go`
- `backend/internal/api/task_history_routes_test.go`
- `backend/internal/api/e2e_core_flow_test.go`
- 必要时新增 backend-only 测试文件，例如 `backend/internal/asset/thumbnail_test.go`
- 如确实需要共享缩略图 helper，可新增 `backend/internal/thumbnail/**`，但不要把业务授权逻辑放进去

禁止修改文件：
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `deploy/**`
- `scripts/**`
- Provider Adapter、Provider/model 管理、认证/RBAC 主流程、SSE handler、Redis queue、Worker claim/cancel 状态机、system settings、log retention runtime
- 不新增 AI Provider 直连、Provider key 处理、前端轮询、浏览器存储、公共 MinIO URL 或 object-key 暴露

前置阅读：
- `docs/api-contract.md` 的 Asset APIs 与 thumbnail 合同
- `docs/storage.md` 的 Thumbnail generation 与 MinIO 规则
- `docs/security.md` 的 P16 thumbnail 安全要求
- `backend/internal/asset/service.go`
- `backend/internal/asset/types.go`
- `backend/internal/asset/repository.go`
- `backend/internal/asset/cleanup.go`
- `backend/internal/task/runtime_persistence.go`
- `backend/internal/task/types.go`
- `backend/internal/api/asset_routes_test.go`
- `backend/internal/api/task_history_routes_test.go`
- `backend/internal/api/e2e_core_flow_test.go`
- `backend/internal/storage/store.go`
- `backend/internal/config/config.go`

具体开发内容：
1. 缩略图生成：
   - 从已通过后端验证的 JPEG/PNG/WebP 原图 bytes 生成 JPEG 缩略图。
   - 使用已存在依赖 `golang.org/x/image/draw` 进行缩放；除非确有必要，不新增第三方依赖。
   - 保持长宽比，不放大小图；建议最大边长 `512`，输出 JPEG quality 建议 `85`。
   - 输出大小必须有合理上限；如生成结果异常，返回 sanitized storage/upload/persistence error。
   - 缩略图 object key 使用 deterministic backend key：`tenants/{tenantId}/projects/{projectId}/assets/{assetId}/thumb.jpg`。
2. Reference upload 路径：
   - `UploadAsset` 在写 metadata 前生成缩略图并上传到 thumbnails bucket。
   - `image_assets.thumbnail_object_key` 必须写入。
   - 如果 original upload、thumbnail upload 或 metadata transaction 任一失败，必须尽力 cleanup 已上传的 original 和 thumbnail object，不能留下成功 asset row 指向缺失对象。
   - Storage quota 仍按现有 `size_bytes` 原图元数据 enforcement；本任务不改变 quota schema，不把 thumbnail bytes 加入 `usedBytes`。
3. Worker output 路径：
   - `persistSuccessfulResult` 为每个待持久化 output 生成 thumbnail object key 和 bytes。
   - 成功资产 metadata 必须包含 `ThumbnailObjectKey`。
   - `IMAGE_OUTPUT` 事件中的 `thumbnailUrl` 应在有缩略图时返回 `/api/v1/assets/{assetId}/thumbnail`。
   - 如果 DB transaction 失败、任务状态已变化、输出已存在或只部分持久化，cleanup 必须同时覆盖 generated original bucket 和 thumbnails bucket 中未持久化对象。
   - 不改变 Worker claim/cancel/retry/timeout 状态机。
4. Thumbnail endpoint：
   - 注册 `GET /assets/:assetId/thumbnail`。
   - 复用 asset authorization；最低应满足 `asset:read` 与 project viewer 权限。不要要求更高权限导致列表缩略图不可用。
   - 如果 asset 不存在、已删除、跨租户、无权限或无 thumbnail object，返回现有 sanitized error shape，不泄漏 object key/bucket/MinIO URL。
   - Stream 内容类型为 `image/jpeg`；不要添加下载文件名也可以，但不得暴露原始 object key。
5. 响应映射：
   - `responseFromRecord` 只在 `ThumbnailObjectKey != nil && *ThumbnailObjectKey != ""` 时返回 thumbnail URL。
   - 资产 list/detail、project history、task output event 应保持同一 URL 语义。
   - `previewUrl` 继续指向 authorized download endpoint，不改变现有下载行为。

安全要求：
- 所有 thumbnail 对象写入、读取、删除都只能使用后端生成的 object key。
- 不允许前端传入或修改 `thumbnail_object_key`。
- 不允许把 thumbnail bucket、object key、MinIO endpoint、signed URL、image base64、Authorization、Cookie、JWT、Provider Key 写入响应、operation log、task event metadata 或错误消息。
- Thumbnail endpoint 必须做登录、tenant、object/project authorization；不能只校验登录状态。
- 禁止上传 SVG 的现有行为不能回归。
- Cleanup log 只能记录 `asset_id` 和 sanitized `error_kind` 等非敏感字段。

必须保持的现有行为：
- Reference upload validation、quota enforcement、audit log、favorite/update/delete/download、project member/RBAC 行为不回归。
- Worker output idempotency：重复 queue delivery 或已存在 output index 不能创建重复资产或重复缩略图 side effects。
- Soft-delete 和 P13 physical cleanup 继续能删除 original 和 thumbnail object。
- Frontend 不修改；现有 UI 通过返回的 `thumbnailUrl` 或 `previewUrl` 兼容显示。
- No thumbnail 的既有资产仍可 detail/download/history，不应因为缺 thumbnail 变成不可用。

允许的中间态：
- 可以先实现 backend thumbnail generator/helper 和 asset upload，再接 Worker output。
- 可以在本任务只对新资产生成缩略图，不做既有资产 backfill。
- 可以保持 storage quota 只按原图 metadata 计数，但最终交付必须说明该边界。

禁止的半迁移状态：
- `thumbnailUrl` 返回非空，但 endpoint 不存在或不鉴权。
- Metadata 写入 `thumbnail_object_key`，但 thumbnail object upload 失败后仍返回成功。
- Worker 成功 task/output event 中给出 thumbnail URL，但 DB 或 MinIO 没有对应 thumbnail object。
- Cleanup 只删 original，不删本任务新写入的 thumbnail。
- 直接返回 MinIO URL、bucket、object key 或 public/signed URL 给浏览器。
- 为了缩略图而修改前端、Provider Adapter、SSE replay、queue claim 或 system settings。

失败模式与边界场景：

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| 上传 JPEG/PNG/WebP reference | 创建 original + thumbnail + asset row，响应含 thumbnailUrl | 是 |
| 上传小图 | 不放大，仍生成合法 JPEG thumbnail | 是 |
| original upload 成功但 thumbnail upload 失败 | upload 失败，清理 original，无 asset row，无成功 audit | 是 |
| thumbnail upload 成功但 metadata transaction 失败 | upload 失败，清理 original + thumbnail，无成功 asset row | 是 |
| asset 无 thumbnail_object_key | detail/list/history 可用，thumbnailUrl 为空；thumbnail endpoint 404 | 是 |
| 跨租户或无项目权限访问 thumbnail | 拒绝，不能泄漏对象存在性或 object key | 是 |
| Worker generated output success | original + thumbnail + asset row + task_output + IMAGE_OUTPUT thumbnailUrl 一致 | 是 |
| Worker output DB transaction 失败 | 清理 original + thumbnail，任务不进入错误成功态 | 是 |
| 重复 output index 已存在 | 不创建重复 original/thumbnail/asset/output | 是 |
| soft-deleted asset | thumbnail endpoint/download/detail 均不可访问，cleanup 后可删除 thumbnail | 是 |
| storage cleanup thumbnail not found | 保持幂等成功或现有 not-found 语义 | 是 |

必须新增或更新的回归测试：
- `backend/internal/api/asset_routes_test.go`：reference upload 生成 thumbnail、响应 URL、thumbnail endpoint auth/object auth/no-leak、failure cleanup。
- `backend/internal/task/worker_test.go` 或相关 task persistence 测试：Worker output 生成 thumbnail、event thumbnailUrl、失败 cleanup、重复 output idempotency。
- `backend/internal/api/task_history_routes_test.go`：history 在有 thumbnail 时返回后端同源 thumbnailUrl，仍不泄漏 object key。
- `backend/internal/asset/cleanup_test.go`：如当前 coverage 不足，补 thumbnail object cleanup/rollback 场景。
- 如新增 helper，补 helper 单元测试覆盖 JPEG/PNG/WebP、小图不放大、输出 bounds。

测试命令：
```bash
cd backend
go test ./internal/asset ./internal/api ./internal/task -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
bash scripts/security-regression.sh
git diff --check main...HEAD
```

如果你实际修改了 frontend、docs、deploy、scripts、Provider Adapter、SSE 或 queue claim/cancel 主流程，立即停止并报告；本任务默认禁止。

最终交付必须包含：
- 修改文件清单。
- 执行的测试命令和结果。
- failure matrix 到具体测试文件/测试名的映射。
- 安全自查结果，明确 thumbnail endpoint authorization、无 object key/bucket/MinIO URL 泄漏、无 Provider 直连、无前端轮询、无浏览器存储变更。
- 刻意未修改范围，特别说明 existing asset backfill、orphan cleanup、manual cleanup trigger、thumbnail-byte quota accounting、frontend thumbnail polish 不在本任务内。
- 如使用共享本地 MySQL/Redis/MinIO，说明创建/修改/清理了哪些 `codex_p16_thumbnail_*` 测试数据；默认优先使用自动化测试和内存/fake store。
- 如发现公共合同缺口，只报告主 agent，不修改 docs。
```

### 允许修改文件

- `backend/internal/asset/**`
- `backend/internal/task/runtime_persistence.go`
- `backend/internal/task/types.go`
- `backend/internal/task/worker_test.go`
- `backend/internal/api/asset_routes_test.go`
- `backend/internal/api/task_history_routes_test.go`
- `backend/internal/api/e2e_core_flow_test.go`
- 必要时新增 `backend/internal/thumbnail/**` 或 backend-only 测试文件

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `deploy/**`
- `scripts/**`
- Provider Adapter、Provider/model 管理、认证/RBAC 主流程、SSE handler、Redis queue、Worker claim/cancel 状态机、system settings、log retention runtime

### 验收标准

- 新 reference upload 和 Worker output 会生成 thumbnail object，并在成功资产 metadata 中保存 `thumbnail_object_key`。
- `thumbnailUrl` 只返回同源后端鉴权 endpoint，不返回 MinIO URL、bucket 或 object key。
- `GET /assets/{assetId}/thumbnail` 通过登录、tenant、project/member/RBAC/object authorization 后 stream JPEG thumbnail。
- 失败 rollback 会清理本任务新写入的 original/thumbnail 对象；成功 cleanup foundation 仍能删除 thumbnail。
- 既有无缩略图资产不回归，仍可 detail/download/history。
- 现有 upload validation、quota、Worker idempotency、task output event、security regression 不回归。

### 测试命令

```bash
cd backend
go test ./internal/asset ./internal/api ./internal/task -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
bash scripts/security-regression.sh
git diff --check main...HEAD
```

## 最近已完成任务包：P16-BE-LOG-RETENTION

本任务已合并到 `main`，保留在本文档中作为日志保留任务包审计记录。

### 调度决策

- 本任务串行执行，不与缩略图或 orphan cleanup 并行。
- 理由：它会把 `logRetention` 从 deferred setting 变成 active runtime-backed setting，必须先稳定系统设置合同、Worker runtime consumer 和数据删除边界。
- 本任务只处理现有数据库日志：`operation_logs`、`api_call_logs`、`task_events`。容器 stdout/stderr、宿主机日志、外部日志平台保留策略不在本任务范围内。

### 任务信息

- 任务名称：`P16-BE-LOG-RETENTION`
- 目标：为数据库日志增加租户级 retention 设置和 Worker maintenance consumer，确保日志清理由真实运行时消费、tenant-safe、batch-limited、auditable、sensitive-log safe。
- 推荐线程名：`P16-BE-LOG-RETENTION`
- 推荐分支名：`codex/p16-backend-log-retention`
- 起始分支：已合并 `P16-DEPLOY-SCRIPT-HARDENING` 的最新 `main`
- 前置依赖：P16 deployment script hardening 已合并；现有 `system_settings`、Worker storage-retention maintenance、audit recorder、API call logs、task events 均可复用。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P16-BE-LOG-RETENTION`。

你必须在分支 `codex/p16-backend-log-retention` 上工作；如果当前不在该分支，先执行 `git switch codex/p16-backend-log-retention`，确认 `git branch --show-current` 后再继续。起始点必须包含已合并 `P16-DEPLOY-SCRIPT-HARDENING` 的最新 `main`；如果 `git merge-base --is-ancestor main codex/p16-backend-log-retention` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
为现有数据库日志增加真实 runtime-backed retention：
- 新增系统设置 `logRetention`，字段为 nullable：
  - `operationLogRetentionDays`
  - `apiCallLogRetentionDays`
  - `taskEventRetentionDays`
- `null` 表示该日志类型自动清理关闭；正整数表示 Worker 按 `cutoff = now - days` 清理旧数据。
- 有效范围为 `1..3650` 天。
- Worker maintenance 必须实际消费该设置，按 active tenant、按日志类型、按 batch limit 清理。
- 清理必须 tenant-safe、batch-limited、可重复执行、可审计，并且不能输出敏感字段。

允许修改文件：
- `backend/internal/settings/**`
- `backend/internal/api/system_settings_routes_test.go`
- `backend/internal/database/models.go`
- `backend/internal/database/migrations_test.go`
- `backend/cmd/worker/main.go`
- `backend/cmd/worker/*retention*.go`
- `backend/cmd/worker/*retention*_test.go`
- 如确有必要，可新增 `backend/internal/audit` 或 `backend/internal/task` 下的测试文件；生产代码默认不要改这些包，除非实现清理服务时必须复用类型/常量，并在最终交付说明原因

禁止修改文件：
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `deploy/**`
- `scripts/**`
- Provider Adapter、任务执行主流程、SSE handler、资产上传/下载、MinIO storage 实现
- 任何 AI Provider 直连、Provider key 处理、前端轮询或浏览器存储路径

前置阅读：
- `docs/api-contract.md` 中 system settings 的 `logRetention` 合同
- `docs/database-schema.md` 中 `system_settings`、`operation_logs`、`api_call_logs`、`task_events`
- `docs/security.md` 关于敏感日志和 runtime-backed settings 的规则
- `backend/internal/settings/service.go`
- `backend/internal/settings/types.go`
- `backend/internal/settings/repository.go`
- `backend/cmd/worker/retention_maintenance.go`
- `backend/cmd/worker/retention_maintenance_test.go`
- `backend/internal/database/models.go`
- `backend/internal/database/migrations.go`

具体开发内容：
1. 设置合同实现：
   - 在 settings 类型中加入 `LogRetention` / patch / enabled config / invalid config。
   - `GET /api/v1/admin/system-settings` 返回 `logRetention`。
   - `PATCH /api/v1/admin/system-settings` 支持设置或清空三个 nullable day 字段。
   - 任一字段可单独 patch；未知字段、非整数、0、负数、超过 3650、空对象等必须返回现有 settings validation error。
   - 权限继续沿用 tenant admin + `system:settings:manage`。
   - settings update operation log 只能记录 key、changedFields、数值/null，不得记录请求原文或敏感上下文。
2. Worker runtime consumer：
   - 新增或扩展 Worker maintenance runner，读取 active tenants 的 `log_retention` 设置。
   - 对 malformed stored `log_retention` fail closed：跳过该 tenant，记录 sanitized `error_kind`，不清理任何日志。
   - 对每个 enabled tenant/category 计算 cutoff 并删除旧 rows。
   - `operation_logs`：只删除同 tenant 且 `created_at < cutoff` 的 rows；清理完成后写入一条新的 sanitized `operation_logs` audit event，记录每类 processed/deleted/failed 计数。
   - `api_call_logs`：只删除同 tenant 且 `created_at < cutoff` 的 rows。
   - `task_events`：只删除同 tenant、`created_at < cutoff`、且对应 `generation_tasks.status` 为 terminal 的 events。不要删除 queued/running/cancelling/retrying 等非终态任务事件。
   - 每类清理必须 batch-limited。单次 run 不需要清空所有历史，只要按 batch 逐步推进。
   - 清理必须幂等；重复执行不会报错或越租户删除。
3. Worker lifecycle：
   - 将 log-retention runner 接入 `backend/cmd/worker/main.go`。
   - 可以复用现有 `WORKER_RETENTION_MAINTENANCE_INTERVAL` 与 `WORKER_RETENTION_MAINTENANCE_BATCH_LIMIT`，除非有充分理由新增环境变量。
   - shutdown 必须尊重 context，不能阻塞 Worker 停止。
4. 测试：
   - settings/API 测试覆盖 GET/PATCH、权限、tenant isolation、validation、malformed stored row fail closed、operation log metadata sanitized。
   - Worker 测试覆盖 active/inactive tenant、valid/null/malformed config、per-category cutoff、batch limit、task_events 终态保护、跨租户不删除、重复执行幂等、cleanup audit log、context cancel。
   - 如新增 SQL cleanup helper，优先用 sqlite 单元测试证明 SQL 条件；如 MySQL 特性不可避免，说明原因。

必须保持的现有行为：
- 现有 `uploadPolicy`、`taskDefaults`、`taskConcurrency`、`storageRetention`、`storageQuota` 行为不回归。
- Frontend system settings 在本任务中不修改；前端是否展示 `logRetention` 由后续任务决定。
- SSE replay 对活跃任务不被破坏：非终态任务的 `task_events` 不得被删除。
- API call log read endpoints、operation log read endpoints、usage summary 不得泄漏敏感信息。
- Worker storage retention cleanup 继续工作，不因新增 log retention loop 停止。

允许的中间态：
- 可以先实现 backend GET/PATCH 和 settings tests，再接入 Worker consumer。
- 可以让 log retention 使用同一个 Worker maintenance interval/batch 配置。
- 可以把 container stdout/stderr retention 明确留给部署层，不在 backend settings 中出现。

禁止的半迁移状态：
- `GET/PATCH /admin/system-settings` 返回或接受 `logRetention`，但 Worker 不消费。
- 只删除全局日志、不带 `tenant_id` 过滤。
- 删除非终态 task events，导致运行中任务 SSE replay 断裂。
- 删除日志时记录完整 metadata、Authorization、Cookie、JWT、Provider Key、base64、bucket 或 object_key。
- 暴露容器日志、MinIO object listing、manual cleanup trigger 或 orphan cleanup 为 active settings。
- 修改前端来展示尚未验收的设置。

失败模式与边界场景：

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| GET 无 log_retention row | 返回三个字段均为 null | 是 |
| PATCH 单字段为有效天数 | 保存并返回该字段，其他字段保持当前值 | 是 |
| PATCH 字段为 null | 清空该字段并关闭该类 cleanup | 是 |
| PATCH 非整数/0/负数/超过上限/未知字段 | 400/422 validation error，不写成功 operation log | 是 |
| 非 admin 或无 `system:settings:manage` | 拒绝读写 | 是 |
| tenant A 设置 | tenant B 不可读取或消费 | 是 |
| stored malformed log_retention | API 读失败为 sanitized error；Worker 跳过该 tenant | 是 |
| operation_logs cleanup | 只删 tenant 内 cutoff 前 rows，插入 sanitized cleanup audit | 是 |
| api_call_logs cleanup | 只删 tenant 内 cutoff 前 rows | 是 |
| task_events cleanup | 只删终态任务旧 events，保留非终态任务 events | 是 |
| batch limit 小于候选数 | 单次只删 batch 内数量，可重复执行 | 是 |
| context canceled | 停止后续 tenant/category，不扩大删除 | 是 |
| cleanup SQL/storage error | sanitized warn，继续下一 tenant/category 或安全停止 | 是 |

必须新增或更新的回归测试：
- `backend/internal/api/system_settings_routes_test.go`：增加 `logRetention` API/权限/validation/tenant isolation/operation log 测试。
- `backend/internal/settings/service_test.go`：增加 decode/validate/apply/LoadEnabledLogRetentions 或等价单元测试。
- `backend/cmd/worker/*log_retention*_test.go`：增加 Worker cleanup runner 测试，映射上述 failure matrix。
- 如更新模型或 migration 断言，补 `backend/internal/database/migrations_test.go`。

测试命令：
```bash
cd backend
go test ./internal/settings ./internal/api ./cmd/worker -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
bash scripts/security-regression.sh
git diff --check main...HEAD
```

如果你实际修改了 frontend 或 deploy，立即停止并报告；本任务默认禁止。

最终交付必须包含：
- 修改文件清单。
- 执行的测试命令和结果。
- failure matrix 到具体测试文件/测试名的映射。
- 安全自查结果，明确 tenant filter、敏感日志脱敏、无 Provider 直连、无前端轮询、无 browser storage 变更。
- 刻意未修改范围，特别说明容器 stdout/stderr retention、frontend settings UI、orphan cleanup、manual cleanup trigger 不在本任务内。
- 如使用共享本地 MySQL/Redis/MinIO，说明创建/修改/清理了哪些 `codex_p16_log_retention_*` 测试数据；默认优先使用自动化测试和 sqlite。
- 如发现公共合同缺口，只报告主 agent，不修改 docs。
```

### 允许修改文件

- `backend/internal/settings/**`
- `backend/internal/api/system_settings_routes_test.go`
- `backend/internal/database/models.go`
- `backend/internal/database/migrations_test.go`
- `backend/cmd/worker/main.go`
- `backend/cmd/worker/*retention*.go`
- `backend/cmd/worker/*retention*_test.go`
- 必要时新增 backend-only 测试文件

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `frontend/**`
- `deploy/**`
- `scripts/**`
- Provider Adapter、任务执行主流程、SSE handler、资产上传/下载、MinIO storage 实现

### 验收标准

- `logRetention` 只在 backend settings/API 和 Worker runtime consumer 同时落地后变为 active。
- Worker cleanup tenant-safe、batch-limited、idempotent、context-aware、sanitized。
- `task_events` cleanup 不破坏非终态任务的 SSE replay。
- settings 更新有 sanitized operation log，cleanup run 有 aggregate audit trace。
- 所有现有 settings 行为不回归。

### 测试命令

```bash
cd backend
go test ./internal/settings ./internal/api ./cmd/worker -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api ./cmd/worker

cd ..
docker compose -f deploy/docker-compose.yml config
bash scripts/security-regression.sh
git diff --check main...HEAD
```

## 最近已完成任务包：P16-DEPLOY-SCRIPT-HARDENING

本任务已合并到 `main`，保留在本文档中作为部署脚本硬化任务包审计记录。

### 调度决策

- 本任务串行执行，不与 P16 后端任务并行。
- 理由：部署验证失败后不残留容器/卷是稳定生产上线的基础运维保障，且写入范围独立。
- 本任务只处理部署验证脚本和脚本级回归；公共 docs 由主 agent 在 review/merge 后同步。

### 任务信息

- 任务名称：`P16-DEPLOY-SCRIPT-HARDENING`
- 目标：让 `scripts/deploy-release-validation.sh --up --down` 在 live Compose 验证失败、被中断或提前退出时仍尽力执行 cleanup，避免遗留项目 Compose 容器、网络或卷。
- 推荐线程名：`P16-DEPLOY-SCRIPT-HARDENING`
- 推荐分支名：`codex/p16-deploy-script-hardening`
- 起始分支：已完成 R15 的最新 `main`
- 前置依赖：P15/R15 已完成；`scripts/deploy-release-validation.sh` 和 `deploy/RUNBOOK.md` 已存在。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P16-DEPLOY-SCRIPT-HARDENING`。

你必须在分支 `codex/p16-deploy-script-hardening` 上工作；如果当前不在该分支，先执行 `git switch codex/p16-deploy-script-hardening`，确认 `git branch --show-current` 后再继续。起始点必须包含 R15 文档提交后的最新 `main`；如果 `git merge-base --is-ancestor main codex/p16-deploy-script-hardening` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
强化部署验证脚本的失败清理能力：
- `scripts/deploy-release-validation.sh --up --down` 在 live Compose 验证失败、脚本错误、SIGINT 或 SIGTERM 时，必须尽力执行 `docker compose down -v --remove-orphans`。
- `--up` 不带 `--down` 时仍保留当前行为：验证后让 stack 留给 operator inspection。
- `--down` 不带 `--up` 时仍保持 cleanup-only 行为。
- 默认模式不应启动 Compose stack，不应删除容器/卷。
- 增加脚本级回归，证明 cleanup trap 的关键路径。

允许修改文件：
- `scripts/deploy-release-validation.sh`
- `scripts/deploy-release-validation-test.sh` 或同类脚本级测试文件

禁止修改文件：
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/**`
- `frontend/**`
- `deploy/**`
- `.env.example`
- `scripts/security-regression.sh`，除非发现必须联动的脚本 bug，且先在交付中说明理由

前置依赖：
- 先阅读 `scripts/deploy-release-validation.sh`、`scripts/security-regression.sh`、`deploy/RUNBOOK.md`、`docs/development-plan.md`、`docs/codex-agent-tasks.md`、`docs/deployment.md`。
- `docs/**` 只读，不允许修改。

具体开发内容：
1. 为 `scripts/deploy-release-validation.sh` 增加 cleanup trap：
   - 仅在 `--down` 被指定且 live stack 已经或即将启动时自动 cleanup。
   - 正常成功路径不能重复执行危险 cleanup，或重复执行也必须是幂等且输出清晰。
   - SIGINT/SIGTERM 触发时返回非 0，并尝试 cleanup。
   - cleanup 输出不得打印 secret。
2. 保持现有 CLI 行为：
   - `--help` 只输出帮助。
   - 默认模式：config、proxy scan、build、security regression，不启动 stack，不 cleanup。
   - `--down`：cleanup-only。
   - `--up`：启动并验证，保留 stack。
   - `--up --down`：启动并验证，成功或失败都 cleanup。
3. 增加脚本级回归：
   - 推荐新增 `scripts/deploy-release-validation-test.sh`，使用临时目录和 fake `docker`/`rg`/`curl` 等命令验证参数行为和 cleanup trap，不启动真实 Docker。
   - 至少覆盖：默认模式不 cleanup、`--down` cleanup-only、`--up` 不带 down 不自动 cleanup、`--up --down` 失败时 cleanup、SIGTERM/SIGINT 路径如可稳定模拟。
   - 测试不得依赖真实 MySQL/Redis/MinIO，不得调用真实 AI Provider。
4. 如发现现有脚本 redaction 不足，可在允许范围内收紧，但不要扩大到业务代码。

安全要求：
- 不打印 `.env`、JWT、Cookie、Provider Key、MinIO secret、MySQL/Redis 密码。
- 不新增 AI Provider proxy、AI relay、真实 Provider 调用、task polling 或 MinIO direct download。
- 不删除共享本地 `dev-mysql8`、`dev-redis`、`dev-minio` 数据。
- 真实 Compose cleanup 只能作用于 `deploy/docker-compose.yml` 对应项目栈；不要写 broad Docker prune、volume prune、system prune。
- 测试里的 fake secret 只能是明显测试值，不得使用真实本地凭据。

验收标准：
- `bash scripts/deploy-release-validation.sh --help` 通过。
- `bash scripts/deploy-release-validation.sh` 通过，且不启动 Compose stack。
- `bash scripts/deploy-release-validation.sh --up --down` 通过，并在结束后无项目容器/卷残留。
- 新增脚本级回归通过，并能证明失败 cleanup trap。
- `scripts/security-regression.sh` 仍通过。
- 未修改禁止范围文件。

必须保持的现有行为：
- 默认部署验证仍执行 Compose config、frontend nginx `/api/` 和 SSE proxy checks、image build、安全回归。
- `--up` 仍保留 stack 给 operator inspection。
- `--down` 仍可单独用于 cleanup-only。
- 所有输出继续避免 secret。

允许的中间态：
- 可以先添加内部 helper 函数和 fake-command 测试，再调整 trap 行为。
- 可以让 cleanup 重入，但必须幂等且不会扩大删除范围。
- 可以跳过真实 `--up --down` 的失败注入，只要 fake-command 测试覆盖失败 trap，真实 `--up --down` 成功路径仍要跑。

禁止的半迁移状态：
- 失败时仍可能遗留项目 Compose 容器/卷却没有测试覆盖。
- 默认模式或 `--up` 不带 `--down` 意外删除 stack。
- 使用 `docker system prune`、`docker volume prune` 或不限定项目的清理命令。
- 为了测试 trap 而启动真实服务后不清理。
- 修改业务后端、前端或公共 docs。

失败模式与边界场景：

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| 默认模式成功 | 不启动 stack，不 cleanup，验证现有 gates | 是 |
| `--down` without `--up` | 只执行 `compose down -v --remove-orphans` | 是 |
| `--up` without `--down` 成功 | 验证后保留 stack 给 operator | 是 |
| `--up --down` 成功 | 验证后 cleanup | 是 |
| `--up --down` live check 失败 | 返回非 0，仍 cleanup | 是 |
| SIGINT/SIGTERM during live validation | 返回非 0，尽力 cleanup | 尽量覆盖；若模拟不稳定，交付中说明 |
| cleanup 命令自身失败 | 返回非 0，输出 sanitized failure | 是 |
| 缺少 docker/rg/curl | 清晰失败，不触发无关 cleanup | 是 |

必须新增或更新的回归测试：
- 新增或更新脚本级测试，建议 `scripts/deploy-release-validation-test.sh`。
- 测试必须用 fake Docker path 覆盖 cleanup trap，不依赖真实 Docker 失败。
- 保留真实成功路径验证命令。

测试命令：
```bash
bash -n scripts/deploy-release-validation.sh
bash scripts/deploy-release-validation.sh --help
bash scripts/deploy-release-validation-test.sh
bash scripts/deploy-release-validation.sh
bash scripts/security-regression.sh
bash scripts/deploy-release-validation.sh --up --down
docker compose -f deploy/docker-compose.yml ps -a
docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true
docker compose -f deploy/docker-compose.yml config
git diff --check main...HEAD
```

如果你修改了会影响 frontend/backend build 的内容，额外运行：
```bash
cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./cmd/api ./cmd/worker
cd ../frontend && npm run lint && npm run type-check && npm run test && npm run build
```

最终交付必须包含：
- 修改文件清单。
- 执行的测试命令和结果。
- cleanup trap failure modes 到测试名/脚本断言的映射。
- 是否执行真实 Compose `--up --down`，以及是否确认无项目容器/卷残留。
- 安全自查结果，明确没有真实 secret、AI Provider 调用、AI relay、Provider proxy、task polling、broad Docker cleanup。
- 刻意未修改范围。
- 如果发现公共合同或业务代码缺口，只报告主 agent，不修改 docs 或业务代码。
```

### 允许修改文件

- `scripts/deploy-release-validation.sh`
- `scripts/deploy-release-validation-test.sh` 或同类脚本级测试文件

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/**`
- `frontend/**`
- `deploy/**`
- `.env.example`
- `scripts/security-regression.sh`，除非先说明必须联动的脚本 bug

### 验收标准

- cleanup trap 覆盖 `--up --down` 失败或中断路径。
- 默认模式、`--down`、`--up`、`--up --down` 的原有语义保持不变。
- 脚本级测试与真实部署验证成功路径均通过。
- 真实 `--up --down` 验证后无项目 Compose 容器或卷残留。

### 测试命令

```bash
bash -n scripts/deploy-release-validation.sh
bash scripts/deploy-release-validation.sh --help
bash scripts/deploy-release-validation-test.sh
bash scripts/deploy-release-validation.sh
bash scripts/security-regression.sh
bash scripts/deploy-release-validation.sh --up --down
docker compose -f deploy/docker-compose.yml ps -a
docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true
docker compose -f deploy/docker-compose.yml config
git diff --check main...HEAD
```

## 最近已完成任务包原始内容：P15-DEPLOY-RUNBOOK-FINAL

### 调度决策

- 本任务串行执行，不与 R15 并行。
- 理由：部署 runbook 和 Compose 发布验证必须基于已合并的 P15 core-flow 与最终安全回归；R15 只能 review 已落地的部署验证结果。
- 本任务以部署脚本、部署目录内 runbook、Compose 验证和运维说明为主，不开发业务功能。

### 任务信息

- 任务名称：`P15-DEPLOY-RUNBOOK-FINAL`
- 目标：最终化 Docker Compose 发布验证入口和 operator runbook，覆盖环境变量、启动、健康检查、初始化管理员、MinIO bucket/bootstrap、SSE 代理、备份/恢复、升级/回滚、日志排查和清理步骤。
- 推荐线程名：`P15-DEPLOY-RUNBOOK-FINAL`
- 推荐分支名：`codex/p15-deploy-runbook-final`
- 起始分支：已合并 `P15-SECURITY-FINAL-REGRESSION` 的最新 `main`
- 前置依赖：`P15-E2E-CORE-FLOWS`、`P15-SECURITY-FINAL-REGRESSION` 已合并；`scripts/security-regression.sh` 可作为发布前安全门禁。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P15-DEPLOY-RUNBOOK-FINAL`。

你必须在分支 `codex/p15-deploy-runbook-final` 上工作；如果当前不在该分支，先执行 `git switch codex/p15-deploy-runbook-final`，确认 `git branch --show-current` 后再继续。起始点必须包含已合并 `P15-SECURITY-FINAL-REGRESSION` 的最新 `main`；如果 `git merge-base --is-ancestor main codex/p15-deploy-runbook-final` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
最终化部署发布验证和 operator runbook：
- 提供一个可重复执行的部署发布验证入口，覆盖 Compose config、镜像 build、可选 up/down、健康检查、frontend `/api/` 代理、安全回归脚本和清理。
- 在 `deploy/` 内补齐面向运维人员的 runbook，说明环境变量、生产 secret、启动顺序、init-admin、MinIO bucket/bootstrap、SSE 代理、备份/恢复、升级/回滚、日志排查、验证命令和清理命令。
- 不改变业务运行时行为；如发现 Compose 或健康检查真实缺陷，优先做部署范围内最小修复，超出允许范围时停止并报告主 agent。

允许修改文件：
- `deploy/**`
- `scripts/*.sh`
- `.env.example`
- 根级部署辅助文件，例如 `Makefile`，仅当仓库已有或确有必要时

禁止修改文件：
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/internal/**`
- `backend/cmd/**`
- `frontend/src/**`
- `frontend/package.json`、`frontend/package-lock.json`
- `backend/go.mod`、`backend/go.sum`
- 数据库 migration/schema、API/SSE/RBAC/Provider/task/storage 公共合同

前置依赖：
- 分支必须从最新 `main` 创建，且包含 P15 安全回归合并结果。
- 先阅读 `deploy/docker-compose.yml`、`deploy/minio/README.md`、`deploy/nginx/README.md`、`.env.example`、`scripts/security-regression.sh`、`docs/deployment.md`、`docs/security.md`、`docs/local-development.md`。其中 `docs/**` 只读，不可修改。

具体开发内容：
1. 新增或完善部署发布验证脚本，推荐 `scripts/deploy-release-validation.sh`：
   - 支持 `--help`。
   - 默认执行非破坏性检查：Compose config、必要文件存在、`.env.example` placeholder 检查、frontend Nginx `/api/` 代理安全检查、AI relay/Provider proxy 禁止检查、`scripts/security-regression.sh --help`。
   - 提供显式运行模式，例如 `--build` 和 `--up`，用于执行 Compose build/up/health/ps/log-tail/check/down；默认不要意外启动或删除服务。
   - 如果执行 `up`，必须在正常结束或失败时给出清理命令；除非用户明确要求保留，不得让项目 Compose 容器和卷长期遗留。
   - 不打印 `.env` 全量内容、JWT、Cookie、Provider Key、MinIO secret、MySQL/Redis 密码。
2. 新增或完善 `deploy/` 内 runbook，例如 `deploy/RUNBOOK.md`：
   - 环境准备和变量说明，只使用 placeholder，不写真实本地凭据。
   - 首次启动、健康检查、init-admin、MinIO bucket/bootstrap。
   - 前端 `/api/` 与 SSE 代理要求，明确禁止 AI Provider relay。
   - 发布前门禁命令：backend/frontend/Compose/security/deploy validation。
   - MySQL 备份/恢复、MinIO 备份/恢复、Redis 说明（Redis 非任务最终状态源）。
   - 升级、回滚、日志排查、常见故障和清理命令。
3. 如 `.env.example` 或 `deploy/docker-compose.yml` 存在明显部署验证缺口，可做最小部署配置修复；不得引入真实 secret 或业务行为改动。
4. 如果脚本需要调用 `docker compose up -d`，必须验证：
   - `mysql`、`redis`、`minio`、`backend-api`、`backend-worker`、`frontend` 健康或运行状态。
   - API `/healthz` 可达。
   - frontend `/api/v1/healthz` 代理到 backend。
   - SSE proxy 路径不 buffering。
   - frontend Nginx 没有 OpenAI/Gemini/custom Provider/relay proxy。
5. 最终交付必须说明有没有启动 Compose、有没有清理、有没有使用共享本地服务。默认本任务优先使用 Compose 验证，不使用共享本地 MySQL/Redis/MinIO。

安全要求：
- 不调用真实 AI Provider，不配置真实 Provider Key。
- 不把真实本地服务凭据写入仓库、脚本输出、runbook 或交付说明。
- 不新增 AI Provider 代理、Nginx relay、Provider direct browser path、task polling 或 MinIO 直链下载。
- Runbook 必须强调生产 secret 不可使用 placeholder，Compose 示例不得鼓励 `.env.example` 原样用于生产。
- 备份/恢复命令不得包含真实密码；使用环境变量或 placeholder。

验收标准：
- 存在可重复执行的部署发布验证入口，且 `--help` 可用。
- `deploy/` 内存在 operator runbook，覆盖启动、健康检查、init-admin、bucket/bootstrap、SSE 代理、备份/恢复、升级/回滚、日志排查和清理。
- Compose config/build/up/health 验证路径明确；如实际执行过 up，交付中必须注明最终已清理或用户要求保留。
- frontend 只代理 `/api/` 到 backend，不代理 AI Provider。
- 没有修改业务生产代码、公共合同文档或 Agent 规则。

必须保持的现有行为：
- `scripts/security-regression.sh` 继续可运行。
- 现有 frontend/backend 构建测试不因部署脚本或 runbook 变化而回归。
- 生产 placeholder secret guard 仍由后端启动逻辑负责，本任务不得放松。
- `deploy/docker-compose.yml` 仍是部署拓扑，不替代 routine shared local development environment。

允许的中间态：
- 默认脚本可以只做非破坏性检查，完整 build/up 通过显式参数触发。
- Runbook 可以放在 `deploy/RUNBOOK.md`，公共 `docs/deployment.md` 由主 agent 在 review/merge 后同步更新。
- 允许增加部署专用脚本 helper，但必须保持可读、可审计、失败即退出。

禁止的半迁移状态：
- 写了 runbook 但没有任何可执行验证入口。
- 写了验证脚本但默认会删除共享本地服务或清空数据。
- Compose up 后不提供清理路径，或脚本失败时泄露 secret。
- 通过修改业务代码绕过健康检查、认证、SSE、Provider、任务或存储问题。
- 把 frontend 重新变成 AI relay 或 Provider proxy。

失败模式与边界场景：

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| `.env.example` 含真实 secret | 脚本或 review 能发现，仓库不得保留 | 是 |
| frontend Nginx 出现 OpenAI/Gemini/relay proxy | 验证失败 | 是 |
| `/api/` 不代理到 backend-api | 验证失败 | 是 |
| SSE 代理 buffering 未关闭 | 验证失败或 runbook 明确标红 | 是 |
| Compose config 无效 | 验证失败 | 是 |
| Compose build 失败 | 显式 build 模式失败并报告 | 是 |
| Compose up 后服务不健康 | 显式 up 模式失败并给出排查日志/清理命令 | 是 |
| minio bucket/bootstrap 缺失 | runbook 或脚本覆盖验证/说明 | 是 |
| 备份/恢复命令暴露真实密码 | 禁止提交 | 是 |
| 脚本失败 | `set -euo pipefail`，不吞错，不打印 secret | 是 |

必须新增或更新的回归测试：
- 本任务以 shell/deploy 验证为主，不要求新增 Go/TS 测试。
- 必须新增或更新脚本自检：`bash scripts/deploy-release-validation.sh --help` 和默认模式。
- 如修改 Compose/Nginx 配置，必须用命令验证对应代理和 config。

测试命令：
```bash
bash scripts/deploy-release-validation.sh --help
bash scripts/deploy-release-validation.sh
bash scripts/security-regression.sh --help
bash scripts/security-regression.sh

docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend

# 如果实现了显式 up 模式，执行：
bash scripts/deploy-release-validation.sh --up
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml down -v --remove-orphans

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
git diff --check main...HEAD
```

最终交付必须包含：
- 修改文件清单。
- 执行的测试命令和结果。
- 是否执行 Compose build/up，服务健康结果，以及最终是否清理。
- 部署验证 failure mode 到脚本检查、runbook 小节或命令的映射。
- 安全自查结果，明确没有真实 secret、AI Provider 调用、AI relay、Provider proxy、task polling、MinIO 直链或敏感日志输出。
- 刻意未修改范围。
- 如发现公共合同或业务代码缺口，只报告主 agent，不修改 docs 或业务代码。
```

### 允许修改文件

- `deploy/**`
- `scripts/*.sh`
- `.env.example`
- 根级部署辅助文件，例如 `Makefile`，仅当仓库已有或确有必要时

### 禁止修改文件

- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/internal/**`
- `backend/cmd/**`
- `frontend/src/**`
- `frontend/package.json`
- `frontend/package-lock.json`
- `backend/go.mod`
- `backend/go.sum`
- API/SSE/RBAC/Provider/task/storage/database 公共合同

### 验收标准

- `scripts/deploy-release-validation.sh --help` 和默认模式可运行。
- `deploy/` 内 runbook 覆盖发布前门禁、启动、健康、init-admin、bucket/bootstrap、SSE 代理、备份/恢复、升级/回滚、日志排查和清理。
- Compose config/build/up/health 验证路径清晰，且执行过的 Compose 资源已清理。
- 未修改业务代码、公共 docs、Agent 规则或前端/后端依赖文件。

### 测试命令

```bash
bash scripts/deploy-release-validation.sh --help
bash scripts/deploy-release-validation.sh
bash scripts/security-regression.sh --help
bash scripts/security-regression.sh
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
bash scripts/deploy-release-validation.sh --up
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml down -v --remove-orphans

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
git diff --check main...HEAD
```

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
