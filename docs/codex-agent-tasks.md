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

R12 已完成，未发现阻塞问题。`P13-BE-RUNTIME-DEFAULTS`、`P13-BE-RUNTIME-DEFAULTS-HARDENING` 与 `P13-BE-CONCURRENCY-POLICY` 已 review 并合并，P13 仍在进行中。

已完成的平台基础：

- P0-P3：文档、monorepo 布局、后端/前端/部署骨架、Docker 运行时修复。
- P4-P6：数据库、租户、认证、RBAC、项目、资产、Provider/模型管理。
- P7-P8：任务队列、Worker、Provider Adapter 运行时、SSE、前端后端化。
- P9-P10：审计/用量读取、运行时生效的上传策略、生产密钥保护、安全/部署验证、Worker 并发池、SSE bridge 生命周期、Provider/模型生命周期、admin 硬化、后端统一历史查询。
- P11-P12：用户/角色管理、统一历史、卖家项目/资产流程和项目最后 `OWNER` 约束。
- P13 已合并切片：租户 `taskDefaults.{defaultProviderId,defaultModelId}` 的读写、任务创建真实消费路径、损坏持久化默认值的 fail-closed hardening，以及 Worker 实际消费的 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`。

当前已知后续项：

- 前端历史已改为消费 `GET /api/v1/projects/{projectId}/history`。
- 前端项目/资产工作流已完成一轮产品化打磨：项目编辑、资产筛选、资产元数据编辑、项目成员入口、上传切项目 stale 保护和筛选后列表一致性均已合并。
- 后端项目成员写路径已补齐最后一个 `OWNER` 保护：不能删除或降级项目最后一个 `OWNER`，但允许先新增或提升另一个 `OWNER` 后再完成 owner 转移。
- R12 已验证 P12 范围内的卖家工作流、统一历史、项目/资产 UI、项目成员 API、最后 `OWNER` 保护、操作日志、权限边界、前端禁止模式和 Compose 配置。
- 用户/角色管理后端接口和前端管理 UI 已完成；后续租户/团队更深层能力可在新的任务中继续补齐。
- 上传策略、`taskDefaults` 与 `taskConcurrency` 已有真实运行时消费者；损坏的 `task_defaults` 与 `task_concurrency` 配置必须 fail closed，不能绕过校验、限流或 Provider 执行边界。其他运行时设置在消费者落地前不得暴露为可写配置。
- 下一串行任务只做后端存储清理基础：修复上传后 DB 失败的取消上下文清理缺口，并建立 soft-delete 后物理删除对象的幂等基础服务。
- 存储保留周期配置、配额、缩略图策略和完整 orphan cleanup 仍需实现。
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
   - 修复上传对象写入后 metadata 持久化失败时的独立 cleanup context，避免请求取消导致 MinIO 孤儿对象。
   - 增加 soft-delete 资产物理清理基础服务：按 tenant、cutoff、batch 上限扫描，删除 original/thumbnail 对象，并用持久化 purge 标记保证幂等。
   - 不开放 storage quota、retention 或 frontend settings。
5. `P13-BE-STORAGE-QUOTA-RETENTION`
   - 在 cleanup foundation 合并后，再做存储配额、保留周期、计划任务/运维入口和更完整的 orphan cleanup。
6. `P13-FE-SYSTEM-SETTINGS`
   - 仅为已经运行时生效的设置提供前端 admin UI。
7. `R13`
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

## 下一个任务包：P13-BE-STORAGE-CLEANUP-FOUNDATION

### 调度决策

- 本任务串行执行，不与前端设置、配额、保留周期或发布任务并行。
- 理由：存储物理删除必须先把对象清理语义、tenant 过滤、幂等标记和失败重试边界固定下来，再开放可写 retention/quota 设置。
- 本任务只做后端存储清理基础，不增加新的 public admin settings 字段，不新增前端 UI，不改任务、SSE 或 Provider Adapter 业务语义。

### 任务信息

- 任务名称：`P13-BE-STORAGE-CLEANUP-FOUNDATION`
- 目标：修复资产上传后 metadata 持久化失败时可能因请求取消而遗漏对象清理的问题，并建立 tenant-scoped、batch-limited、idempotent 的软删资产物理清理基础服务，为后续 storage quota/retention 和计划任务奠定真实运行时基础。
- 推荐线程名：`P13-BE-STORAGE-CLEANUP-FOUNDATION`
- 推荐分支名：`codex/p13-backend-storage-cleanup-foundation`
- 起始分支：已合并 `P13-BE-CONCURRENCY-POLICY` 与本任务公共合同文档的最新 `main`
- 前置依赖：P13 runtime defaults、defaults hardening、task concurrency policy 均已合并；`docs/storage.md` 与 `docs/database-schema.md` 已明确 soft-delete 默认策略、MinIO object metadata 关系和物理清理基础规则。

### 允许修改文件

- `backend/internal/asset/**`
- `backend/internal/storage/**`
- `backend/internal/database/**`，仅限新增资产 purge 标记、索引、migration 和对应测试所需的最小模型变更
- `backend/internal/api/*asset*_test.go`
- `backend/internal/api/router.go`，仅在注入 asset service 依赖或测试 wiring 必需时修改
- `backend/internal/config/**`，仅在需要定义后端内部 cleanup timeout 默认值时修改；不得新增可写 system setting
- `backend/cmd/worker/**`，仅当实现内部 cleanup service 的构造测试需要时修改，不得改变 Worker 任务执行主流程

### 禁止修改文件

- `frontend/**`
- `deploy/**`
- `docs/**`
- `AGENTS.md`
- `agent-instructions/**`
- `backend/internal/settings/**`
- `backend/internal/task/**`，除非只改测试 helper 且不触碰任务状态机；默认不允许
- `backend/internal/provider/**`
- `backend/internal/provideradapter/**`
- `backend/internal/model/**`
- `backend/internal/queue/**`
- `backend/internal/sse/**`
- 任何新的 public storage quota、retention 或 log retention API
- 系统设置响应/PATCH 合同、前端 UI、部署拓扑、Provider/model/task/SSE 行为或无关重构

### 具体开发内容

1. 先写回归测试，再做最小实现；每个 failure mode 都要在交付说明中映射到测试。
2. 修复上传对象写入成功、metadata/DB 创建失败后的清理路径：
   - 继续保证校验失败时不写对象。
   - 如果对象已写入而 metadata 持久化失败，清理必须使用独立的 bounded context 或等价的 cleanup abstraction，不能依赖已取消的 HTTP request context。
   - 清理失败必须返回上传失败并记录脱敏错误；不得把 object key、bucket、请求文件名、base64 或内部堆栈暴露给前端。
3. 增加软删资产物理清理基础：
   - 查询条件必须包含 `tenant_id`。
   - 只处理 `deleted_at IS NOT NULL` 且早于调用方传入 cutoff 的资产。
   - 增加 durable purge marker，例如 `image_assets.purged_at`，防止已物理删除对象被反复处理；migration 和模型变更保持最小。
   - 批量上限必须有默认保护，避免一次扫描全表。
4. 清理 original 和 thumbnail 对象：
   - original bucket 使用资产 kind 对应的当前 storage 规则。
   - thumbnail 仅在 `thumbnail_object_key` 非空时尝试删除。
   - MinIO object not found 视为清理成功，保证幂等。
   - 任一非 not-found storage 错误时不得设置 `purged_at`，便于后续重试。
5. 不 hard-delete MySQL 行；资产 metadata 继续作为审计和历史状态来源。
6. 不新增公开 API 或 system setting。cleanup service 可以是内部服务/仓储能力，后续任务再接入 retention job、operator trigger 或 quota accounting。

### 必须保持的现有行为

- 上传校验仍然在对象写入前完成，SVG、伪造 MIME、过大文件/尺寸/像素继续被拒绝。
- 资产下载、详情、列表、收藏、软删、项目权限和 object-level authorization 行为不变。
- MySQL 仍只保存 metadata/object key，不保存图片 blob。
- MinIO bucket 创建仍是环境/部署责任；request handler 不创建 bucket。
- 任务输出资产、历史查询、SSE、Provider Adapter、usage/API-call logs、系统设置和 RBAC 语义不变。

### 允许的中间态

- 清理基础服务已存在，但尚未接入可写 retention setting、定时任务或前端 UI。
- `purged_at` 或等价 durable marker 可先只由内部 service 测试和后续任务调用。
- 缩略图生成策略仍可维持当前状态；本任务只处理已有 thumbnail object key 的删除。

### 禁止的半迁移状态

- 只新增 `storageQuota`、`retentionDays` 或类似 API 字段，但没有真实 cleanup/quota consumer。
- 物理删除对象时不带 tenant 条件，或通过前端/请求传入 object key 决定删除目标。
- 删除未 soft-delete 的资产对象，或把 DB 行 hard-delete 掩盖权限/审计历史。
- 遇到 storage delete 失败仍标记 `purged_at`。
- 使用 request context 进行上传失败后的对象清理，导致客户端断开即可留下孤儿对象。
- 返回或记录 bucket/object_key、MinIO URL、图片 base64、Authorization、Cookie 或内部错误栈。

### 失败模式与边界场景

| 场景 | 预期行为 | 必须覆盖 |
| --- | --- | --- |
| 上传 validation 失败 | 不写 MinIO object，不写 image_assets | 是 |
| 上传 object 成功后 DB create 失败且 request context 已取消 | 仍使用独立 bounded cleanup 删除刚写入对象；前端收到上传失败 | 是 |
| 上传 object 成功后 DB create 失败且 cleanup 也失败 | 返回上传失败并记录脱敏错误；不得泄漏 object key；不能创建成功 metadata | 是 |
| soft-deleted 且早于 cutoff 的资产 | 删除 original 与已有 thumbnail，成功后设置 `purged_at` | 是 |
| 非 soft-deleted、未到 cutoff、已 purged、跨 tenant 资产 | 不删除、不更新 | 是 |
| original 或 thumbnail 已不存在 | 视为成功，设置或保持 purge marker，保证幂等 | 是 |
| storage delete 发生非 not-found 错误 | 不设置 `purged_at`，返回/记录脱敏错误，后续可重试 | 是 |
| batch limit 小于待清理数量 | 只处理上限内记录，排序稳定，后续批次可继续 | 是 |
| cleanup service 重复运行 | 已 purged 资产不重复删除，未成功资产可重试 | 是 |
| 现有下载/列表/历史/任务输出 | 行为不变，软删过滤不退化 | 回归现有测试 |

### 安全要求

- 清理对象必须从后端已授权/tenant-scoped metadata 推导，不接受前端传 object key 作为删除真相源。
- 所有查询和 purge update 必须带 `tenant_id`；不得跨 tenant 扫描或删除。
- 不公开 bucket、object_key、MinIO URL、图片 base64 或内部错误栈。
- 不解密、不读取、不记录 Provider API Key、Authorization、Cookie 或 JWT。
- 不引入 frontend Provider 直连、轮询、浏览器敏感存储或 public object URL。

### 必须新增或更新的回归测试

- 上传后 DB 失败且 request context canceled 时，fake store 仍收到独立 cleanup 删除调用。
- 上传后 DB 失败且 cleanup 失败时，响应失败且错误脱敏，无成功 asset row。
- cleanup service 只处理当前 tenant、soft-deleted、早于 cutoff、未 purged 的资产。
- original 与 thumbnail 删除成功后设置 `purged_at`；object not found 仍成功；storage error 不设置 `purged_at`。
- batch limit、生效排序、重复运行幂等。
- 现有 asset list/download/delete/favorite/detail 权限测试和 task output/history 相关测试不回归。

### 验收标准

- 上传 metadata 失败后的对象 cleanup 不再依赖 request context。
- soft-delete 资产物理 cleanup foundation 已具备 tenant 过滤、cutoff、batch、original/thumbnail 删除、not-found 幂等、失败可重试和 durable purge marker。
- 没有开放 storage quota、retention、log retention、frontend settings 或 public cleanup API。
- 资产现有业务行为、任务输出资产、历史查询、下载鉴权和敏感信息脱敏无回归。

### 测试命令

```bash
cd backend
go test ./internal/asset ./internal/storage ./internal/database ./internal/api -count=1
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
