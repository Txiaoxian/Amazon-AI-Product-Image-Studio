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

`P15-E2E-CORE-FLOWS` 已 review 并合并。P15 核心 API/Worker/SSE/history/usage/log 集成路径已有自动化覆盖，下一步进入最终安全回归切片。

已完成的平台基础：

- P0-P3：文档、monorepo 布局、后端/前端/部署骨架、Docker 运行时修复。
- P4-P6：数据库、租户、认证、RBAC、项目、资产、Provider/模型管理。
- P7-P8：任务队列、Worker、Provider Adapter 运行时、SSE、前端后端化。
- P9-P10：审计/用量读取、运行时生效的上传策略、生产密钥保护、安全/部署验证、Worker 并发池、SSE bridge 生命周期、Provider/模型生命周期、admin 硬化、后端统一历史查询。
- P11-P12：用户/角色管理、统一历史、卖家项目/资产流程和项目最后 `OWNER` 约束。
- P13 已合并切片：租户 `taskDefaults.{defaultProviderId,defaultModelId}` 的读写、任务创建真实消费路径、损坏持久化默认值的 fail-closed hardening、Worker 实际消费的 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`、soft-delete 资产物理清理基础服务、Worker 实际消费的 `storageRetention.deletedAssetRetentionDays`、引用图上传/Worker 输出实际消费的 `storageQuota.maxBytes` 与只读 `storageQuota.usedBytes`，以及只展示 active runtime-backed settings 的前端 admin 设置页。
- P14 已合并切片：Provider/model 生命周期完整性、后端确定性 usage/cost reporting、前端 admin 成本可观测性和 R14。Worker 成本估算使用稳定 decimal，非法 pricing 归零且不失败成功任务；admin usage summary 支持 tenant/user/project/Provider/model 维度、tenant isolation、多币种分组和 exact decimal cost；前端 usage tab 支持 tenant totals、过滤、drilldown、多币种展示和 stale response 防护。
- P15 已合并切片：`P15-E2E-CORE-FLOWS`。后端集成测试已串联 init-admin、Provider/model、project、reference upload、task create、fake Worker success、SSE replay、output asset download、history、usage records/summary、operation logs 和 API call logs，且不调用真实 AI Provider。

当前已知后续项：

- 前端历史已改为消费 `GET /api/v1/projects/{projectId}/history`。
- 前端项目/资产工作流已完成一轮产品化打磨：项目编辑、资产筛选、资产元数据编辑、项目成员入口、上传切项目 stale 保护和筛选后列表一致性均已合并。
- 后端项目成员写路径已补齐最后一个 `OWNER` 保护：不能删除或降级项目最后一个 `OWNER`，但允许先新增或提升另一个 `OWNER` 后再完成 owner 转移。
- R12 已验证 P12 范围内的卖家工作流、统一历史、项目/资产 UI、项目成员 API、最后 `OWNER` 保护、操作日志、权限边界、前端禁止模式和 Compose 配置。
- 用户/角色管理后端接口和前端管理 UI 已完成；后续租户/团队更深层能力可在新的任务中继续补齐。
- 上传策略、`taskDefaults`、`taskConcurrency`、`storageRetention` 与 `storageQuota` 已有真实运行时消费者；损坏的 `task_defaults`、`task_concurrency`、`storage_retention` 与 `storage_quota` 配置必须 fail closed，不能绕过校验、限流、Provider 执行边界、清理边界或资产写入边界。其他运行时设置在消费者落地前不得暴露为可写配置。
- 下一任务是 `P15-SECURITY-FINAL-REGRESSION`，串行补齐最终安全回归入口、缺口测试和失败模式映射。
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
   - 已完成并合并。核心 API/Worker/SSE/history/usage/log 自动化集成路径已落地，且使用 fake Worker/Provider，不调用真实 AI Provider。
2. `P15-SECURITY-FINAL-REGRESSION`
   - 下一个任务。补齐最终禁止模式扫描、安全回归入口和安全失败模式到测试的映射。
3. `P15-DEPLOY-RUNBOOK-FINAL`
   - Compose 发布验证、运维手册、备份/恢复说明和健康检查最终化。
4. `R15`
   - 最终发布就绪 review。

## 下一个任务包：P15-SECURITY-FINAL-REGRESSION

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
