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

`P15-E2E-CORE-FLOWS`、`P15-SECURITY-FINAL-REGRESSION`、`P15-DEPLOY-RUNBOOK-FINAL`、`R15`、`P16-DEPLOY-SCRIPT-HARDENING` 和 `P16-BE-LOG-RETENTION` 已完成。P15 核心 API/Worker/SSE/history/usage/log 集成路径、最终安全回归入口、部署 release validation 脚本、operator runbook 和最终 release-readiness review 均已落地；P16 部署脚本失败 cleanup trap 已合并并通过真实 Compose `--up --down` 验证，数据库日志保留已接入 backend settings 与 Worker maintenance consumer。

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
- P16 已完成切片：`P16-DEPLOY-SCRIPT-HARDENING` 和 `P16-BE-LOG-RETENTION`。`scripts/deploy-release-validation.sh --up --down` 已具备失败、错误退出、SIGINT、SIGTERM 的项目 Compose cleanup trap；`logRetention` 已有 backend GET/PATCH 与 Worker runtime consumer，范围限定为现有数据库日志：`operation_logs`、`api_call_logs`、终态任务的 `task_events`。
- P16 下一切片：`P16-BE-THUMBNAIL-POLICY`。需要为新 reference upload 和 Worker 输出资产生成 MinIO thumbnail object，通过后端鉴权访问，不暴露 bucket/object key/MinIO URL。
- 完整 orphan cleanup 仍需实现。
- Provider/模型并发管理操作可能需要更强的事务序列化。
- 最终发布验证已完成；后续工作属于 post-R15 产品/运维 backlog。

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
- 非阻塞遗留：缩略图策略、完整 orphan cleanup、Provider/model 更强事务序列化和严格并发 quota reservation 仍属于 post-R15 backlog；其中缩略图策略是当前 P16 下一任务。

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
4. `R16`
   - 主 agent review P16 全部代码和回归。

### P17：存储治理与生产观测

1. `P17-BE-ORPHAN-CLEANUP`
   - MinIO orphan discovery、dry-run、执行、审计、批量限制和失败重试。
2. `P17-BE-STORAGE-QUOTA-RESERVATION`
   - 为并发上传和 Worker 输出增加严格 quota reservation/counter 与 reconciliation。
3. `P17-BE-OBSERVABILITY-METRICS`
   - 增加 admin-only JSON diagnostics：queue depth、running/failed tasks、Provider failure rate、storage usage、maintenance job result 等。
4. `R17`
   - 主 agent review P17 全部代码和回归。

### P18：真实上线信心与 Go/No-Go

1. `P18-BE-PROVIDER-MODEL-SERIALIZATION`
   - 强化 Provider/model enable/disable/delete/update 与默认设置交互的事务序列化。
2. `P18-E2E-REAL-PROVIDER-SMOKE`
   - 新增可选真实 Provider smoke 脚本；不进默认 CI，不提交真实 key，必须有费用控制。
3. `P18-PROD-DRY-RUN`
   - 按 runbook 在目标或准生产环境执行完整上线 dry-run。
4. `R18-STABLE-PRODUCTION-READINESS`
   - 主 agent 执行最终 Go/No-Go review。

## 下一个任务包：P16-BE-THUMBNAIL-POLICY

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
