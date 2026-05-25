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

R13 已完成，未发现阻塞问题。`P13-BE-RUNTIME-DEFAULTS`、`P13-BE-RUNTIME-DEFAULTS-HARDENING`、`P13-BE-CONCURRENCY-POLICY`、`P13-BE-STORAGE-CLEANUP-FOUNDATION`、`P13-BE-STORAGE-RETENTION-RUNTIME`、`P13-BE-STORAGE-QUOTA-ACCOUNTING` 与 `P13-FE-SYSTEM-SETTINGS` 已 review、合并并完成整批回归。

已完成的平台基础：

- P0-P3：文档、monorepo 布局、后端/前端/部署骨架、Docker 运行时修复。
- P4-P6：数据库、租户、认证、RBAC、项目、资产、Provider/模型管理。
- P7-P8：任务队列、Worker、Provider Adapter 运行时、SSE、前端后端化。
- P9-P10：审计/用量读取、运行时生效的上传策略、生产密钥保护、安全/部署验证、Worker 并发池、SSE bridge 生命周期、Provider/模型生命周期、admin 硬化、后端统一历史查询。
- P11-P12：用户/角色管理、统一历史、卖家项目/资产流程和项目最后 `OWNER` 约束。
- P13 已合并切片：租户 `taskDefaults.{defaultProviderId,defaultModelId}` 的读写、任务创建真实消费路径、损坏持久化默认值的 fail-closed hardening、Worker 实际消费的 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`、soft-delete 资产物理清理基础服务、Worker 实际消费的 `storageRetention.deletedAssetRetentionDays`、引用图上传/Worker 输出实际消费的 `storageQuota.maxBytes` 与只读 `storageQuota.usedBytes`，以及只展示 active runtime-backed settings 的前端 admin 设置页。

当前已知后续项：

- 前端历史已改为消费 `GET /api/v1/projects/{projectId}/history`。
- 前端项目/资产工作流已完成一轮产品化打磨：项目编辑、资产筛选、资产元数据编辑、项目成员入口、上传切项目 stale 保护和筛选后列表一致性均已合并。
- 后端项目成员写路径已补齐最后一个 `OWNER` 保护：不能删除或降级项目最后一个 `OWNER`，但允许先新增或提升另一个 `OWNER` 后再完成 owner 转移。
- R12 已验证 P12 范围内的卖家工作流、统一历史、项目/资产 UI、项目成员 API、最后 `OWNER` 保护、操作日志、权限边界、前端禁止模式和 Compose 配置。
- 用户/角色管理后端接口和前端管理 UI 已完成；后续租户/团队更深层能力可在新的任务中继续补齐。
- 上传策略、`taskDefaults`、`taskConcurrency`、`storageRetention` 与 `storageQuota` 已有真实运行时消费者；损坏的 `task_defaults`、`task_concurrency`、`storage_retention` 与 `storage_quota` 配置必须 fail closed，不能绕过校验、限流、Provider 执行边界、清理边界或资产写入边界。其他运行时设置在消费者落地前不得暴露为可写配置。
- 下一任务是 `P14-BE-PROVIDER-MODEL-INTEGRITY`，串行强化 Provider/model 生命周期完整性。
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

## 下一个任务包：P14-BE-PROVIDER-MODEL-INTEGRITY

### 调度决策

- 本任务串行执行，不与用量/成本统计、前端成本看板或发布任务并行。
- 理由：Provider/model 生命周期是后续成本统计和任务默认模型选择的基础数据，必须先保证并发写入和删除/禁用边界不会制造不一致状态。
- 本任务只改后端 Provider/model 生命周期相关实现和测试，不改前端、部署、公共合同文档或 Agent 规则。

### 任务信息

- 任务名称：`P14-BE-PROVIDER-MODEL-INTEGRITY`
- 目标：强化 Provider 与模型管理的生命周期完整性，重点处理 Provider 删除/禁用与模型创建、更新、启用之间的并发和事务边界，避免出现可用模型指向已删除 Provider、默认模型指向不可用 Provider/model、跨租户泄漏或成功审计日志误记。
- 推荐线程名：`P14-BE-PROVIDER-MODEL-INTEGRITY`
- 推荐分支名：`codex/p14-backend-provider-model-integrity`
- 起始分支：已完成 R13 文档更新的最新 `main`
- 前置依赖：P13 已完成；系统设置中的 `taskDefaults` 已依赖 enabled same-tenant Provider/model；Provider/model 后端管理接口和 admin UI 已存在。

### 子 agent 完整启动 prompt

```text
你是本项目的子 agent，负责 `P14-BE-PROVIDER-MODEL-INTEGRITY`。

你必须在分支 `codex/p14-backend-provider-model-integrity` 上工作；如果当前不在该分支，先执行 `git switch codex/p14-backend-provider-model-integrity`，确认 `git branch --show-current` 后再继续。起始点必须包含最新 `main`；如果 `git merge-base --is-ancestor main codex/p14-backend-provider-model-integrity` 不通过，先停止并报告，不要自行修改公共合同。

任务目标：
强化后端 Provider/model 生命周期完整性，确保 Provider 删除、禁用、模型创建、模型更新、模型启用在事务和并发边界下不会产生不一致业务状态。重点覆盖：
- 模型不能创建、迁移或启用到已删除、跨租户或不可用 Provider。
- Provider 删除不能与模型创建/更新/启用并发穿透，不能留下未删除模型指向已删除 Provider。
- Provider 禁用后，后续模型创建/启用和 `taskDefaults` 解析不能把不可用 Provider 当成可用运行时入口。
- 失败写入不得记录成功 operation log，错误响应不得泄露跨租户 Provider/model 名称或 secret。

必须遵守：
1. 不修改 frontend、deploy、docs、AGENTS.md、agent-instructions。
2. 不修改 Provider Adapter runtime、task execution 主流程、SSE、Redis queue，除非发现编译必须调整测试 helper；如需越界，停止并报告。
3. 不解密 Provider API Key，不把 Provider API Key、Authorization、Cookie、图片 base64 写入日志、响应或测试快照。
4. 所有业务查询必须继续带 tenant_id/object-level authorization。
5. API 响应和错误码尽量保持兼容；新增错误必须复用现有稳定错误风格。
6. 若需要数据库约束或索引，必须通过现有 database/migration 机制实现，并提供兼容已有数据的策略；不要破坏已有测试数据。
7. 可以使用 `docs/local-development.md` 中的共享 MySQL/Redis/MinIO 做功能验证，但只允许操作任务自有测试数据，并在交付中说明。

建议实现：
1. 先阅读 `backend/internal/provider/**`、`backend/internal/model/**`、`backend/internal/settings/**` 中 `taskDefaults` Provider/model 校验路径，以及现有 `backend/internal/api/*provider*test.go`、`backend/internal/api/*model*test.go`。
2. 先写或补充回归测试，再实现：
   - Provider 删除时如果同租户存在未删除模型，必须稳定返回冲突，并且不删除 Provider、不写成功 operation log。
   - Provider 删除与模型创建/更新并发时必须有事务序列化或数据库约束兜底；不能出现模型成功指向刚被删除的 Provider。
   - 模型创建、Provider 迁移、模型启用必须拒绝 disabled/deleted/cross-tenant Provider；如果现有产品允许 disabled Provider 下保留 disabled 模型，明确只允许“保留/编辑非运行时字段”，不允许启用为可运行模型。
   - Provider 禁用后，已存在模型的状态处理要与任务创建和 `taskDefaults` 解析一致：不能被运行时当作可用模型。
   - 失败路径不得记录 `provider.delete`、`model.create`、`model.update`、`model.enable` 成功日志。
   - 跨租户 Provider/model ID 在错误响应和 operation log 中不得泄露对方名称、模型名或租户信息。
3. 评估是否需要 `(tenant_id, provider_id, model_name)` 对未删除模型的唯一性约束：
   - 如果实现，必须覆盖重复创建、soft delete 后重建、跨租户同名、同租户不同 Provider 同名等场景。
   - 如果不实现，必须在最终交付中说明原因和残余风险。
4. 需要锁或事务时，优先使用现有 GORM transaction/repository 模式；如使用 row lock 或 DB-specific 行为，测试必须能在当前单元测试环境和 MySQL 语义下保持清楚。
5. 保持 Provider API Key 加密/脱敏、SSRF URL 校验、RBAC 和租户隔离现有行为不回归。

验收命令：
```bash
cd backend
go test ./internal/provider ./internal/model ./internal/settings ./internal/api -count=1
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
- 安全自查结果，明确没有泄露 Provider Key、Authorization、Cookie、跨租户名称或图片 base64。
- 刻意未修改范围。
- 是否实现模型名称唯一约束的决策说明。
- 如发现公共合同缺口，只报告给主 agent，不修改 docs。
```

### 允许修改文件

- `backend/internal/provider/**`
- `backend/internal/model/**`
- `backend/internal/settings/**`，仅限验证 Provider/model 可用性和默认模型一致性所需；默认不改
- `backend/internal/database/**`，仅限必要 migration/model/index/helper 与测试
- `backend/internal/api/*provider*test.go`
- `backend/internal/api/*model*test.go`
- `backend/internal/api/*settings*test.go`，仅限 taskDefaults 与 Provider/model lifecycle 交叉测试
- `backend/internal/api/**` 中 Provider/model route wiring 或测试 helper 必要的小范围调整

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/internal/task/**`
- `backend/internal/provideradapter/**`
- `backend/internal/queue/**`
- `backend/internal/sse/**`
- `backend/internal/asset/**`
- `backend/cmd/**`，除非编译因新增 migration wiring 必须调整；默认不改
- 任何 AI Provider runtime 调用逻辑、API Key 解密路径、task/SSE/queue 主流程

### 具体开发内容

1. 梳理现有 Provider/model 生命周期规则，记录你实际采用的产品语义：
   - 删除 Provider 是否只允许在无未删除模型时发生。
   - 禁用 Provider 是否允许保留模型但禁止新增/启用可运行模型。
   - 模型名称唯一性是否需要本次落地。
2. 补充 Provider 删除与模型 create/update/enable 的边界测试。
3. 如需要，增加 repository/service 层锁定、事务内二次校验或数据库约束，保证并发下数据库最终状态一致。
4. 确保 taskDefaults 的 Provider/model 解析继续拒绝 disabled/deleted/cross-tenant Provider 或模型，不因本次改动回归。
5. 确保失败路径不写成功 operation log，错误响应稳定且不泄露对象详情。
6. 保持现有 Provider 响应不返回 API Key；保持 Provider URL SSRF 校验不回归。

### 必须保持的现有行为

- Provider CRUD、enable/disable/test 的现有响应结构保持兼容，且不返回完整 API Key。
- Provider base URL SSRF 防护保持现有覆盖。
- Model capability 校验、enabled model list、RBAC、tenant scope、CSRF route behavior 不回归。
- `taskDefaults` 仍只能解析 enabled same-tenant Provider/model；显式 task 请求仍走现有 task validation。
- 成功 operation log 只在事务成功后写入；被拒绝的写入不记录成功 action。

### 允许的中间态

- 可以先通过服务层事务和锁解决并发完整性；数据库唯一约束如风险过大可推迟，但必须说明原因。
- 可以保留 disabled Provider 下已有 disabled 模型的管理能力，只要不能被运行时用作 enabled model。
- 成本统计、前端 UI 和 Provider Adapter runtime 不在本任务范围内。

### 禁止的半迁移状态

- Provider 删除成功后仍存在未删除模型指向该 Provider。
- 模型创建、迁移或启用成功后指向 disabled/deleted/cross-tenant Provider。
- 失败写入返回错误但已经写了成功 operation log。
- 为解决管理接口问题而修改 task execution、Provider Adapter runtime、SSE 或 queue 主流程。
- 通过错误信息、日志或测试快照暴露 Provider API Key、跨租户对象名称、Authorization、Cookie 或图片 base64。

### 失败模式与边界场景

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| 删除仍有关联未删除模型的 Provider | `409`/稳定冲突错误；Provider/model 不变；无成功 delete log | 是 |
| 删除 Provider 与创建模型并发 | 最终不能同时出现 Provider deleted 且模型未删除指向它 | 是 |
| 更新模型 Provider 到 deleted/cross-tenant Provider | 拒绝，响应不泄露目标对象详情，无成功 update log | 是 |
| 创建模型到 disabled Provider | 拒绝或仅允许明确非运行时安全语义；不能创建 enabled 可运行模型 | 是 |
| 启用模型但 Provider disabled/deleted | 拒绝，无成功 enable log | 是 |
| Provider 禁用后 taskDefaults 仍指向该 Provider/model | 后续任务默认解析 fail closed | 是 |
| 重复模型名称 | 若本次实现唯一约束，应稳定拒绝；若不实现，应说明风险 | 按决策覆盖 |
| 跨租户模型/Provider ID | 404/422/403 风格保持稳定，不泄露名称或租户信息 | 是 |

### 安全要求

- 所有 Provider/model 查询和写入必须带 tenant_id 过滤。
- 只授权 tenant admin 或具备对应 `provider:*` / `model:*` 权限的用户。
- Provider API Key 仍只加密存储，不返回、不解密、不写日志。
- Provider base_url SSRF 校验不得弱化。
- Operation log metadata 必须脱敏，且不能包含跨租户对象详情。
- 错误响应使用稳定错误码和泛化消息。

### 必须新增或更新的回归测试

- Provider route/service 测试：Provider delete linked models、delete/create race 或等价事务边界、禁用 Provider 后模型启用/创建边界、失败日志不存在。
- Model route/service 测试：create/update/enable 对 deleted/disabled/cross-tenant Provider 的拒绝和脱敏响应。
- Settings/taskDefaults 交叉测试：Provider 或 model disabled/deleted 后，默认任务创建 fail closed，无 task/event/enqueue/success-audit 副作用。
- 如果实现模型名称唯一约束：覆盖重复创建、soft delete 后重建、跨租户同名和不同 Provider 同名。

### 验收标准

- 并发或交错写入下，数据库不会产生 enabled/active 模型指向不可用 Provider 的状态。
- Provider 删除、禁用、模型创建、模型更新、模型启用的边界行为有测试覆盖。
- RBAC、租户隔离、Provider Key 脱敏、SSRF 校验、operation log 行为不回归。
- 未修改前端、部署、公共合同文档或 Agent 规则。

### 测试命令

```bash
cd backend
go test ./internal/provider ./internal/model ./internal/settings ./internal/api -count=1
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
