# 发展计划

## 开发环境规则

日常开发和功能验证必须使用`docs/local-development.md`中记录的现有共享本地服务：

- MySQL: `dev-mysql8`
- Redis：`dev-redis`
- MinIO: `dev-minio`

用户授权未来的开发任务使用这些共享的本地服务进行功能验证，包括创建、更新、删除、入队、上传、下载和清理任务拥有的测试数据。测试数据必须明确归属于任务或分支，除非明确指示，否则代理不得删除项目数据库、删除共享存储桶、刷新共享Redis或删除不相关的数据。

不要为普通功能工作创建特定于项目的 MySQL、Redis 或 MinIO 容器。 `deploy/docker-compose.yml`保留用于部署验证；如果它启动项目容器，请随后清理它们，除非用户明确要求保留它们：

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

## P21 可靠性强化后的现状

该项目已从纯粹的前端本地应用程序转变为后端支持的多用户平台基础。

阶段状态：

|相|状态 |结果 |
| --- | --- | --- |
| P0 |完成 |创建规划文档和项目代理规则。 |
| P1 |完成 | Monorepo 结构已建立：`frontend/`、`backend/`、`deploy/`、`docs/`。 |
| P2 |完成 |后端API基础设施和前端API/SSE客户端基础。 |
| P3 |完成 | Docker Compose运行时修复，后端API/Worker图像，前端`/api/`代理，无AI中继。 |
| P4 |完成 | MySQL/GORM、租户、身份验证、RBAC基金会、HttpOnly Cookie JWT、CSRF、前端登录。 |
| P5 |完成 |项目、项目成员、MinIO支持的资产、上传验证、前端project/assetUI。 |
| P6 |完成 | Provider 以及使用加密密钥的模型管理、SSRF 防御、功能 API、管理 UI。 |
| P7 |完成 |任务 API、Redis 队列、Worker 执行、Provider 适配器运行时、SSE 重放、usage/API 日志。 |
| P8 |完成 |前端generation/edit/history生产流程转移到后端任务、SSE、资产和模型。 |
| P9 |完成 | Audit/usage读取、上传策略设置、生产秘密守卫、security/deploy回归。 |
| P10 |完成 | Worker池、SSE桥生命周期、Provider/model生命周期、管理UI强化、后端历史查询。 |
| P11 |完成 |后端和前端租户 user/role 管理已合并：用户 list/create/update/disable/enable、角色分配、role/permission 读取、RBAC UI 门控和 password/secret 安全检查。 |
| P12 |完成 |卖方工作流程审核已完成。前端统一历史记录、project/asset工作流程完善、后端项目成员不变强化进行合并和回归。 |
| P13 |完成 |运行时支持的租户任务默认值、格式错误的行强化、任务并发策略、存储清理基础、存储保留运行时、存储配额核算、前端系统设置和R13回归已完成。 |
| P14 |完成 | Provider/model生命周期完整性、后端usage/cost报告、前端成本可观察性和R14回归已完成。 |
| P15 |完成 |释放强化已完成。核心流程E2E、最终安全回归、部署运行手册验证以及R15发布准备审查已通过。 |
| P16 |完成 |生产启动强化已完成：部署清理陷阱、运行时数据库日志保留、后端缩略图策略和R16回归已通过。 |
| P17 |完成 |存储治理和可观察性已完成。保守的孤儿清理、严格的配额预留、后端生产诊断以及R17回归通过了。 |
| P18 |完成 | Provider/model/default-setting序列化，选择加入真实Provider冒烟测试工具，已脱敏生产试运行，实际 Compose演练，清理检查已完成。 |
| P19 |完成 |生产配置防护、CI质量门、前端依赖项审核修复和门、主机TLS反向代理template/checks、前端日志保留控制以及现有租户内置角色协调已完成。 |
| P20 |完成 |稳定运营基础已合并：固定 CSRF 标头合约、事务性 Provider 主密钥轮换 CLI、事务性租户配置 CLI、当前租户 API、自定义角色 CRUD/permission 替换、前端tenant/custom-role行政、运维人员CLI图像捆绑、隔离backup/restore/rollback演练。 |
| P21 |完成 | R20生产审计后续实施和R21Go/No-Go回归已完成：CSRF故障关闭、部署env/log强化、队列持久性、Provider手段加密擦除、精确MinIO恢复、工作台图像类型、迁移序列化、配额协调运行时、Provider尝试账本、Redis支持的登录速率限制、前端遗留IndexedDBBlob清理、SSE弹性、会话撤销、并发租约续订、和 Worker 准备情况已合并并验证。 |

P18/P19/P20操作强化结果：

- `scripts/prod-dry-run.sh`现在提供安全默认、生产环境预检、带有范围清理的可选实际 Compose以及可选的计费Provider 冒烟测试委托。实际 Compose演练已完成，没有留下任何项目容器或卷。
- 生产配置现在拒绝占位符或丢失的数据库、Redis、MinIO、JWT和Provider加密秘密；不安全的cookie；不安全CORS起源；和非默认 CSRF 标头别名。
- CI现在运行前端lint/type-check/test/build、后端test/race/vet/build、存储库security/deploy门、Compose配置和`npm audit --audit-level=moderate`。
- 主机TLS部署现在具有可审核的Nginx模板和静态检查器。公共流量终止TLS，仅路由至环回前端`127.0.0.1:8080`； SSE 缓冲仍然处于禁用状态。
- 前端系统设置现在公开后端消耗的可为空`logRetention`。
- API 启动可协调现有租户缺少的内置角色和授权，而无需删除自定义角色或授权。
- 平台CSRF标头在前端、后端、Compose、CORS和生产环境预检中固定为`X-CSRF-Token`。
- `backend/cmd/provider-key-rotation`提供默认试运行并明确确认交易申请Provider备用主密钥轮换。现在，当有效负载密钥 ID 与解密密码不匹配时，它们将以失败方式关闭（fail closed）。活动Provider行被重新加密；历史软删除的 Provider 带有剩余的凭据材料的行在试运行中进行计数报告并在应用中进行加密擦除。
- `backend/cmd/provision-tenant` 提供默认试运行，并通过内置 roles/grants 和一个初始租户管理员明确确认第二个及后续租户的事务创建。
- 租户范围`GET/PATCH /api/v1/tenants/current`和自定义角色CRUD/permission替换 API 已合并。内置角色通过 HTTP API 保持不可变。
- 前端身份管理现在支持租户名称更新以及自定义角色 create/update/delete 以及通过同源 CSRF 保护的后端 API 进行权限替换。内置角色明显保持只读​​状态。
- `scripts/backup-restore-rehearsal.sh`提供默认安全的保护机制模式和明确确认的隔离Compose演练。实时匹配MySQL/MinIO恢复加回滚演练已完成，没有留下任何项目容器或卷。
- 前端测试工具链现在使用`vitest@^4.1.8`；前端 lint、类型检查、工作台图像类型覆盖、构建和 `npm audit` 通过后进行了 133 次测试，报告的漏洞为零。

R20尚未批准最终生产启动。审计确定了P21后续工作必须在最终Go/No-Go之前落地：

- 生产现在在后端启动配置和生产试运行预检中拒绝`CSRF_ENABLED=false`。
- 生产试运行现在通过委托的 Compose/release 验证操作传递一个生产环境文件，编辑健康故障日志，并为长期运行的 Compose 服务配置有界的 `json-file` 日志轮换。
- Redis队列状态迁移现在使用Lua原子移动进行retry/ack/dead-letter/stalerecovery/delayed升级，以及MySQL支持的queued/retrying协调修复丢失的Redis交付状态，无需处理Redis 作为最终状态。
- API/Worker 迁移启动通过 MySQL 路径上的 MySQL 咨询锁进行序列化，现在在不完整的应用架构状态下以失败方式关闭（fail closed）。 SQLite/unit-test路径绕过MySQL咨询锁。
- Worker 维护现在通过有界轮换租户批次调用租户范围的存储配额协调。主代理审查修复了饥饿风险，即固定的首页租户列表可能会无限期地阻止后来的租户。
- Provider尝试持久性现在会在外部Provider执行之前预先写入`ATTEMPTING`API调用账本行，并使用递归已脱敏元数据最终确定成功、失败、超时和取消。还实现了Provider删除加密擦除、精确MinIO恢复语义、工作台图像类型提交和Redis支持的登录速率限制。
- 前端遗留IndexedDBimage/history清理已完成。生产 result/detail/history 路径现在仅使用后端任务、资产和历史数据；旧的本地 image/history 存储库文件、Blob 对象 URL 结果处理和 Base64 转换助手已被删除。 Dexie仅保留提示模板，并通过删除已停用的image/history存储来升级现有数据库。
- SSE catch-up/resubscribe 边界、可撤销会话、并发租约续订和 Worker 准备情况在 R21 中实现和验证。

R21验证了完整的生产可靠性关闭。前端lint/type-check/test/build/audit通过；后端tests/race/vet/API-Worker-operator构建已通过； `scripts/security-regression.sh`，`scripts/deploy-release-validation.sh`，实际运行的 `scripts/deploy-release-validation.sh --up --down`已经过去了。实际 Compose达到健康MySQL、Redis、MinIO、后端API、后端Worker和前端状态；后端运行状况端点、前端`/api/`代理和SSE身份验证边界已通过；清理后没有留下任何项目容器或项目卷。

R11 在整个 P11 代码范围内没有发现阻塞问题。修复 role/status 权限边界后，对`P11-BE-USER-ROLE-ADMIN`进行了审查和合并。在验证前端权限门控、CSRF写入请求、密码非持久性和当前用户禁用保护后，对`P11-FE-USER-ROLE-ADMIN`进行了审查和合并。

R11验证通过：

- `cd frontend && npm run lint`
- `cd frontend && npm run type-check`
- `cd frontend && npm run test`
- `cd frontend && npm run build`
- `cd backend && go test ./...`
- `cd backend && go test -race ./...`
- `cd backend && go vet ./...`
- `cd backend && go build ./cmd/api ./cmd/worker`
- `docker compose -f deploy/docker-compose.yml config`
- `git diff --check 2b186fb..HEAD`
- 重点扫描P11frontend/backend敏感模式扫描禁止的浏览器存储、轮询、Provider直接调用、不安全的响应字段和秘密标记。

`P12-FE-UNIFIED-HISTORY`已审核并合并。前端历史记录生成路径现在使用`GET /api/v1/projects/{projectId}/history`，而不是在浏览器中连接任务和资产列表。该任务通过了前端 lint、类型检查、目标 history/API 测试、完整前端测试、构建和空格检查。非阻塞后续：历史缩略图当前使用授权的资产下载URL；稍后的抛光者可以更喜欢安全的同源缩略图 URL（如果可用）。

`P12-FE-PROJECT-WORKFLOW-POLISH` 已审核、修复并合并。卖家工作区现在支持项目编辑、资产过滤器、资产元数据编辑、由真实成员 API 支持的项目成员入口点、upload/project-switch 陈旧保护以及收藏夹或元数据突变后过滤列表的一致性。该任务通过了前端 lint、类型检查、目标 project/asset 测试、完整前端测试、构建和空白检查。非阻塞后续：当项目成员后端不变量得到强化时，成员删除成功和成员update/remove错误路径可以接受更细粒度的前端测试。

`P12-BE-PROJECT-MEMBER-HARDENING` 已审核、修复并合并。后端项目成员写入路径现在可以防止删除或降级最终的 `OWNER`，在另一个 `OWNER` 保留时保持所有者转移路径有效，保留 tenant/RBAC/project-role 授权，并验证阻止的写入不会创建成功的操作日志。验证通过了后端项目路由集中测试、完整后端测试、竞态测试、审查、API/Worker 构建和空白检查。非阻塞后续：MySQL支持的并发所有者突变覆盖可以在以后的集成或E2E工作中添加。

R12审查了`f843b1e..HEAD`的完整P12代码范围，发现没有阻塞问题。验证已通过前端 lint、类型检查、测试、构建、后端测试、竞态测试、审查、API/Worker 构建、Docker Compose 配置、空格检查和针对 Provider 直接调用、Provider 密钥存储、任务轮询和敏感的前端禁止模式扫描浏览器存储。保留非阻塞后续操作：在可用时首选历史卡的安全同源缩略图 URL，在稍后的 integration/E2E 工作期间添加 MySQL 支持的并发所有者突变覆盖，并保持 P13 可写设置被阻止，直到每个设置都有真正的运行时使用者。

`P13-BE-RUNTIME-DEFAULTS`已审核并合并。后端现在通过现有的系统设置合约公开租户 `taskDefaults.{defaultProviderId,defaultModelId}`，并且仅在任务创建忽略两个 Provider/model ID 时才使用它。显式对保持不变；混合 explicit/default 请求、缺少或清除默认值、无效 Provider/model 所有权或启用状态以及功能无效的默认支持提交将失败，而没有任务创建、排队或成功的 `task.create` 日志。验证通过了重点 settings/task/API 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置和空格检查。

`P13-BE-RUNTIME-DEFAULTS-HARDENING`已审核并合并。对于默认支持的任务创建，手动损坏或旧版 `task_defaults` 行中的无效 JSON、部分 ID、未知字段和空白 ID 现在以失败方式关闭（fail closed）为 `422 VALIDATION_ERROR`，且没有 task/event/enqueue/success-audit 副作用。显式有效的Provider/model提交不会读取未使用的损坏的默认值，并且真正的设置存储故障仍然已脱敏`500 INTERNAL_ERROR`。验证通过了重点 API/task/settings 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置和空白检查。

`P13-BE-CONCURRENCY-POLICY` 已审核、修复并合并。后端现在仅向实时 Worker 消费者公开租户 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`。租户覆盖可以缩小或匹配环境硬上限，全局并发仍然由环境拥有，Provider行`concurrencyLimit`仍然是一个额外的更严格的上限，并且格式错误的持久并发设置在Provider执行或output/usage/API-call成功副作用之前以失败方式关闭（fail closed）。验证通过了重点设置和 Worker 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置和空格检查。

`P13-BE-STORAGE-CLEANUP-FOUNDATION`已审核并合并。现在，当元数据持久性失败时，对象写入后的上传回滚将使用独立的有界清理上下文，并且后端资产清理具有租户范围、批量限制、幂等的基础，可通过持久的 `purged_at` 跟踪来物理删除软删除的原始对象和缩略图对象。验证通过了重点 asset/storage/database/API 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置和空格检查。

`P13-BE-STORAGE-RETENTION-RUNTIME`已审核并合并。后端现在公开可为 null 的 `storageRetention.deletedAssetRetentionDays`，默认情况下禁用自动物理清理，并运行 Worker 维护循环，读取有效的活动租户保留设置并使用 tenant/cutoff/batch 边界调用清理基础。 Malformed/null/inactive 设置以失败方式关闭（fail closed），并且仅使用已脱敏元数据记录清理错误。验证通过了重点 system-settings/settings/asset/database/worker 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置和空格检查。后来P13/P17切片添加了存储配额核算和保守的孤儿发现。

`P13-BE-STORAGE-QUOTA-ACCOUNTING`已审核并合并。后端现在公开可空的 `storageQuota.maxBytes` 以及只读计算的 `storageQuota.usedBytes`，从租户范围的 `image_assets.size_bytes`（其中 `purged_at IS NULL`）计算使用情况，并在引用上传和 Worker 输出资产持久性之前强制执行配额。配额失败返回已脱敏稳定错误，并避免成功的资产行、任务输出、输出事件、使用记录、成功操作日志和对象泄漏。验证通过了重点 system-settings/settings/asset/database/task 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置和空格检查。后来的 P17 工作用严格的租户范围配额计数器以及并发上传和 Worker 输出的预留取代了这种乐观检查。

`P13-FE-SYSTEM-SETTINGS`已审核并合并。前端管理设置选项卡现在仅显示和编辑运行时支持的设置：上传策略、任务默认值、任务并发、存储保留和存储配额。每次保存都会发送一个带有 CSRF 的顶级设置补丁，`storageQuota.usedBytes` 保持只读状态，并且延迟设置（例如日志保留、孤立清理、手动清理、MinIO 列表和 Provider 秘密）仍然隐藏。验证通过了前端 lint、类型检查、目标管理设置测试、完整前端测试、构建、空格检查和禁止模式扫描。

合并前端系统设置后，R13审查了`eeba51f..HEAD`的完整P13代码范围。没有发现阻塞问题。验证已通过前端 lint、类型检查、测试、构建；后端测试、竞态测试、`go vet`、API/Worker 构建； Docker Compose配置；空格检查；前端禁止模式扫描直接 AI Provider 调用、浏览器Provider密钥存储、任务轮询、延迟设置、bucket/object-key 暴露和敏感身份验证字符串。后期阶段完成了缩略图策略、日志保留、保守的孤立清理和严格的配额保留。其余的后续措施包括可选的`WORKER_RETENTION_MAINTENANCE_INTERVAL`最低限度、现有租户的内置`asset:*`权限协调，以及更强大的Provider/model交易序列化。

`P14-BE-PROVIDER-MODEL-INTEGRITY` 已审核、修复并合并。 Provider/model 管理现在拒绝以禁用、删除或跨租户 Provider 为目标的模型 create/update/enable 路径；默认任务设置在加载时重新生效； Provider 删除会获取行锁并在同租户未删除链接模型存在时保持阻塞状态； Provider 通过 `/disable` 和 `PATCH status=DISABLED` 禁用将被拒绝，而启用的链接模型仍然存在。失败的写入不会记录成功的操作日志，并且冲突响应保持不敏感。验证通过了重点 Provider/model/settings/API 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置和空格检查。同一 Provider`model_name`此时未强制执行唯一性，因为运行时任务执行使用稳定的`modelId`引用； P18 后来通过事务序列化和重复拒绝收紧了写入路径。

`P14-BE-USAGE-COST-REPORTING` 已审核、修复并合并。 Worker 使用持久性现在使用具有稳定的 8 位十进制格式的确定性十进制成本估计，无效或不完整的定价会产生零成本，而不会失败成功的 Provider 任务，并且管理使用摘要支持 tenant/user/project/Provider/model 维度，具有租户隔离、多货币分组、稳定分页和精确的大十进制成本保留。验证通过了重点 audit/API/task/database 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置和空格检查。非阻塞后续：使用情况摘要目前在Go中执行每页精确成本聚合；稍后的大批量调整可能会添加更高效的query/index策略。

`P14-FE-COST-OBSERVABILITY` 已审核、修复并合并。管理使用选项卡现在使用 tenant/user/project/Provider/model 维度的后端 usage/cost 摘要，显示租户总计，将 date/task/user/project/Provider/model 过滤器一致地应用于总计、摘要和记录，支持汇总深入到记录过滤器，防止陈旧的使用响应，并显示后端成本字符串，而无需客户端权威重新计算。验证通过了前端 lint、类型检查、测试、构建、Compose 配置、空格检查和重点禁止模式扫描。

合并前端成本可观察性后，R14审查了从`5585d99..HEAD`开始的完整P14范围。没有发现阻塞问题。验证通过了前端lint/type-check/test/build、后端完整tests/race/vet/build、P14集中后端测试，使用`-count=1`、Docker Compose配置、空白检查和集中前端扫描以直接进行AI Provider调用、浏览器Provider-密钥存储、任务轮询、AI中继路径、图像base64持久性和对象存储泄漏。同样的-Provider `model_name`后续问题后来通过P18写路径序列化解决。剩余的非阻塞后续行动是使用摘要大容量调整以及最终删除旧 `404` 合约的前端租户总数兼容性回退。

`P15-E2E-CORE-FLOWS`已审核并合并。后端现在有一个 API 级核心流程集成测试，从 init-admin 开始，验证 HttpOnly 会话和 CSRF 行为，通过真实路由创建 Provider/model/project/reference asset/task 数据，拒绝伪装成 SVG PNG，在没有外部AI调用的情况下运行假Worker执行，验证任务事件和SSELast-Event-ID重播，确认生成的资产下载和项目历史记录，并读取使用情况summary/records加上operation/API 排除敏感标记的调用日志。验证通过了重点后端 API/task/SSE/audit 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、完整前端 lint/type-check/test/build、Docker Compose 配置、空格检查和更改文件禁止模式扫描。

`P15-SECURITY-FINAL-REGRESSION`已审核并合并。该存储库现在将 `scripts/security-regression.sh` 作为最终的安全回归入口点。它运行集中的后端和前端安全测试、生产禁止模式扫描、后端敏感标记扫描、前端`/api/`代理安全检查、Docker Compose配置验证和空格检查。 P15核心流测试通过低权限否定断言进行了扩展，用于输出资产下载、项目历史记录、使用读取、操作日志、API调用日志和API调用详细信息。验证通过了安全回归脚本、完整后端tests/race/vet/build、完整前端lint/type-check/test/build、Compose配置和空格检查。无阻塞：跨租户负面覆盖率仍然主要映射到现有的重点测试。

`P15-DEPLOY-RUNBOOK-FINAL`已审核并合并。该存储库现在包括`deploy/RUNBOOK.md`和`scripts/deploy-release-validation.sh`。部署验证脚本支持`--help`、安全默认验证、显式`--up`以及通过`--down`进行清理；它验证Compose配置、前端`/api/`和SSE代理安全、图像构建、安全回归、实时运行状况检查、前端代理运行状况和清理。验证通过了默认部署发布验证、实时 `--up --down` Compose health/proxy 检查、完整后端 tests/race/vet/build、完整前端 lint/type-check/test/build、Compose 配置和空格检查。 P16 后来添加了针对失败或中断的 `--up --down` 运行的清理陷阱。

R15审查了`3db7980..HEAD`的完整P15范围，发现没有阻塞发布准备问题。验证通过`scripts/security-regression.sh`、`scripts/deploy-release-validation.sh`、实时`scripts/deploy-release-validation.sh --up --down`、清理后container/volume检查、完整后端tests/race/vet/build、完整前端lint/type-check/test/build、 Docker Compose 配置和空格检查。实际 Compose运行已确认MySQL、Redis、MinIO、后端API、后端Worker以及前端运行状况； MinIO引导初始化完成；后端健康端点；前端`/api/`代理健康； SSE 身份验证边界路由；以及项目容器和卷的清理。

`P16-BE-LOG-RETENTION` 已审核、修复并合并。后端 `logRetention` 现在由运行时支持：管理系统设置 API 可以为 `operation_logs`、`api_call_logs` 和 `task_events` read/write 可以为空的保留天数，以及Worker 维护循环会消耗每个活动租户的这些设置。清理是批量限制的、租户范围的、对格式错误的设置进行故障关闭、保留SSE/recovery的非终端任务事件，并记录已脱敏聚合清理审计元数据。验证通过了重点 settings/API/Worker 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置、安全回归和空白检查。

`P16-BE-THUMBNAIL-POLICY` 已审核、修复并合并。新的参考图上传和 Worker generated/edited 输出现在会生成尺寸受限的后端 JPEG 缩略图，将其存入配置的 MinIO 缩略图存储桶，持久化 `thumbnail_object_key`，仅在缩略图存在时返回 `/api/v1/assets/{assetId}/thumbnail`，并通过后端 `asset:read` 授权流式返回缩略图。没有缩略图的现有资产仍可使用，此时 `thumbnailUrl` 为空；在后续明确的 schema/counter 任务完成前，缩略图字节不计入配额。

R16 查看了 `4b1913e..HEAD` 的完整 P16 范围，没有发现阻塞问题。验证通过了后端集中测试、完整后端测试、竞态测试、审查、API/Worker构建、前端lint/type-check/test/build、Docker Compose配置、安全回归、默认部署发布验证、实时`scripts/deploy-release-validation.sh --up --down`和清理后container/volume检查。

`P17-BE-ORPHAN-CLEANUP` 已审核、修复并合并。后端现在具有仅限管理员的存储孤立扫描和清理端点，具有试运行默认值、显式清理确认、tenant/admin权限检查、有界MinIO列表、不透明连续光标、识别的后端对象键模式检查、MySQL元数据排除、年龄门控、重试安全删除失败处理和已脱敏聚合操作日志。验证通过了重点 storage/asset/API 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置、安全回归和空白检查。

`P17-BE-STORAGE-QUOTA-RESERVATION` 已审核、修复并合并。后端现在使用租户范围的配额 counter/reservation 表进行参考上传和 Worker generated/edited 输出。资产创建路径在 MinIO 写入之前保留原始字节，在元数据事务中完成保留，在 validation/storage/DB 失败时释放，对软删除但未清除的资产进行计数，在物理清除后递减，并从 MySQL 元数据而不是 MinIO 根据 MySQL 元数据协调计数器。已发布或格式错误的预留以失败方式关闭（fail closed），且没有成功的 asset/task-output 副作用，并且并发上传测试验证组合的超限请求不能超过租户配额。验证通过了重点 database/settings/asset/API/task 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置、安全回归和空白检查。

`P17-BE-OBSERVABILITY-METRICS` 已审核、修复并合并。后端现在将 `GET /api/v1/admin/diagnostics/summary` 公开为仅限管理员、租户范围的只读诊断端点，需要 `audit:read`。它返回任务状态和最近失败的有界聚合部分、Redis队列深度、Provider/API-call故障率、存储quota/asset使用情况以及最近的维护摘要。端点不会改变状态、调用 Provider、触发清理、排队任务、解密 Provider 密钥或公开 Redis 密钥、队列负载、对象密钥、存储桶、MinIO URL、签名 URL、原始 Provider/log 元数据、 Authorization/Cookie/JWT值、Provider秘密或图像base64。审查修复添加了生产Redis队列检查器接线、未截断的Provider总计、队列`reason="queue_unavailable"`以及失败关闭的维护元数据解析。验证通过了重点 diagnostics/queue/task/settings/asset 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置、安全回归和空白检查。

R17在合并孤儿清理、严格配额预留和生产诊断后审查了完整的P17范围。没有发现阻塞存储管理或可观察性问题。验证通过了完整后端测试、后端竞态测试、审查、API/Worker构建、完整前端lint/type-check/test/build、Docker Compose配置、安全回归、空白检查以及使用映像构建的默认部署发布验证。 P18可能会从最新的 `main` 开始。

`P18-BE-PROVIDER-MODEL-SERIALIZATION`已审核并合并。 Provider/model/default 设置写入路径现在对 MySQL 路径、模型 create/update/enable/delete 路径在需要时锁定目标行使用更强的行锁定，`taskDefaults` 在持久化之前更新锁 Provider/model 行以及同租户Same-Provider未删除的`modelName`重复项将被拒绝，而不会进行破坏性的唯一索引迁移。验证通过了重点后端 Provider/model/settings/API/task 测试、完整后端测试、竞态测试、`go vet`、API/Worker 构建、Docker Compose 配置、安全回归和空白检查。现有的 Provider API 形状、前端、Provider 适配器运行时、Worker/SSE/task 执行、存储生命周期和部署脚本未更改。

`P18-E2E-REAL-PROVIDER-SMOKE`已审核并合并。该存储库现在有一个可选的手动 `scripts/real-provider-smoke.sh` 入口点以及 `scripts/real-provider-smoke-test.sh`。该脚本默认是安全的，支持`--help`、`--dry-run`和显式`--run`，在任何计费路径之前需要`REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS`，拒绝直接AIProvider API base URL，限制输出计数和超时，仅使用平台`/api/v1`后端，并将秘密保留在stdout/stderr.`deploy/RUNBOOK.md`记录手动使用情况，而不将秘密放入shell历史记录中。验证通过了脚本测试、默认部署发布验证、安全回归、Docker Compose配置、dry-run/manual防护检查和空格检查。自动验证期间没有执行真正的Provider调用。后来的生产试运行切片修复了临时文件清理注册错误并添加了故障路径回归。

## 完整的平台能力

当前的`main`分支支持：

- 通过租户上下文和 RBAC 强制执行进行身份验证的多用户访问。
- 后端租户用户管理：列出、创建、详细信息、更新安全字段、disable/enable、角色分配、角色读取和权限读取。
- 前端租户user/role管理UI由`user:*`和`role:*`权限控制，密码输入保持瞬时状态并通过受CSRF保护的后端API发送写入请求。
- 项目和项目成员管理基础，包括后端last-`OWNER`成员update/delete路径保护。
- MinIO支持的reference/generated/edited具有后端授权的图像资产和后端生成的授权JPEG新资产的缩略图。
- 管理员Provider和模型管理，加密Provider会计，SSRF安全ProviderURL，简体中文管理副本（如果可行），Provider超时长达600秒，带有解释性帮助文本， OpenAI`gpt-image-2`官方质量预设加上有序的宽高比选择，以及GeminiNano Banana 2宽高比加上独立的1K/2K/4K分辨率模板。
- 后端任务创建、Redis排队、Worker执行、Provider适配器AI调用、输出资产、使用记录、API调用日志和SSE任务更新。
- 仅通过后端任务 API 和 SSE 提交前端工作台。
- 前端历史记录从`GET /projects/{projectId}/history`读取后端拥有的项目历史记录，具有分页、generated/edited过滤、过时响应保护、授权detail/download和后端`editSourceAssetId`重新编辑。
- 卖家工作区project/asset工作流程通过后端API支持项目编辑、参考上传、资产过滤、资产元数据编辑、favorite/delete/download/detail/use-as-reference和项目成员list/add/update/remove入口点。
- 卖家工作区项目选择被组织为顶级项目选项卡，显示项目名称和品牌，按项目`sortOrder`排序，支持drag/drop重新排序，并通过辅助模式打开项目editing/member管理。
- 生成工作区采用固定的左侧图像类型菜单，显示项目参考​​缩略图以供快速参考选择，支持当前任务的临时上传，并可以将上传的参考快速保存到项目资源库中。
- 管理员可观察使用记录、操作日志、API通话日志和上传策略设置。
- 后端运行时支持的任务默认值：租户管理员可以存储启用的同租户 Provider/model 对，只有当两个 ID 都被省略时，任务创建才会解析它，并且格式错误的持久默认值将以失败方式关闭（fail closed），而不会产生创建副作用。
- 后端运行时支持的任务并发策略：租户管理员可以在环境硬上限内配置 tenant/user/Provider/model 限制，并且 Worker Redis 信号量获取在 Provider 执行之前消耗这些有效限制。
- 后端存储清理基础：上传回滚清理不再依赖于取消的请求上下文，软删除的图像资产可以通过内部租户范围、基于截止的幂等清理服务进行物理清除。
- 后端存储孤儿清理：租户管理员或设置管理员可以运行保守的试运行扫描，并确认清理与后端对象键模式匹配且不被 MySQL 元数据引用的 MinIO 对象，而不会暴露原始对象键或存储桶。
- 后端存储配额预留：参考上传和Worker输出持久化使用租户范围MySQLreservation/counter记账，以防止并发写入超过`storageQuota.maxBytes`；计数器从 `image_assets` 元数据进行协调，并且不使用 MinIO 列表作为配额真相。
- 后端运行时支持的存储保留策略：租户管理员可以选择设置`storageRetention.deletedAssetRetentionDays`； Worker 维护会消耗它，并且在未设置或清除时默认为禁用。
- 后端运行时支持的存储配额策略：租户管理员可以选择设置`storageQuota.maxBytes`； `storageQuota.usedBytes`是根据租户范围的资产元数据计算的，uploads/Worker输出持久性在创建新资产元数据之前强制执行配额。
- 前端管理系统设置UI仅适用于活动运行时支持的设置：上传策略、任务默认值、任务并发、存储保留和存储配额。
- 后端Provider/model生命周期完整性：Provider删除和禁用可防止链接的模型状态，这将使活动模型指向不可用Provider； model/default-setting写入锁定并重新验证Provider/model引用；相同的Provider未删除的模型名称会被写入路径检查拒绝。
- 后端确定性usage/cost报告：Worker使用记录使用稳定的十进制成本估算，定价失败是零成本非致命案例，管理使用摘要支持使用精确成本字符串进行tenant/user/project/Provider/model聚合。
- 前端管理成本可观察性：使用选项卡显示租户总计、维度摘要、过滤的使用记录、深入过滤器、来自后端的多货币成本字符串以及过时响应保护，无需 Provider 直接调用、浏览器 Provider 密钥存储或轮询。
- 后端核心流程E2E覆盖范围：初始化管理、Provider/model设置、project/reference资产上传、任务创建、假Worker执行、SSE重放、输出资产下载、项目历史记录、使用情况和日志现在在一个集成路径中进行验证，无需外部 AI Provider 调用。
- 最终安全回归入口点：`scripts/security-regression.sh`合并重点安全测试、前端禁止模式扫描、后端敏感标记扫描、Compose配置验证、`/api/`代理安全检查和空格检查。
- 部署发布运行手册和验证入口点：`deploy/RUNBOOK.md`文档Compose发布操作、健康检查、init-admin、MinIO引导初始化、SSE代理行为、backup/restore、 upgrade/rollback，日志故障排除和清理。 `scripts/deploy-release-validation.sh`自动化config/build/security/health/proxy检查； backup/restore 和回滚仍然需要明确的目标环境演练。
- 可选的真实Provider冒烟测试工具：`scripts/real-provider-smoke.sh`可以手动运行并明确确认以验证后端介导的Provider/task/SSE/output路径；默认 help/dry-run 模式永远不会调用 real AI Provider 或消耗积分。
- 前端Docker Compose、后端API、后端Worker、MySQL、Redis和MinIO的部署拓扑。

平台硬规则保持不变：

- 浏览器不得直接调用 AI Provider。
- 浏览器不得存储 AI Provider API 密钥。
- 任务状态必须使用SSE，而不是轮询。
- MySQL是任务状态真相； Redis 是queue/wakeup/lock/cache 基础设施。
- MinIO存储图像字节； MySQL 仅存储元数据和对象键。
- 租户过滤器、RBAC、对象授权、敏感日志编辑和ProviderSSRF防御是强制性的。

## 全平台完成目标

在这些产品功能完成并得到验证之前，该平台才算完成：

1. 租户、用户、角色和项目成员管理可从后端和前端使用。
2. 面向卖家的项目工作流程足够完善，可以重复使用：create/select产品项目、管理reference/generated/edited资产、检查详细信息、下载、收藏、删除和重新编辑。
3. 前端历史记录消耗后端统一的项目历史记录端点，而不是在客户端加入任务和资产列表。
4. 运行时设置在可写之前有真正的消费者：默认Provider/model、tenant/user/model/provider并发、上传策略、存储配额和保留。
5. 存储生命周期可操作：缩略图生成或清除缩略图策略、孤立清理、保留清理和安全 MinIO bucket/bootstrap 指导。
6. Provider/model 生命周期和数据完整性针对并发管理操作进行了强化。
7. 使用情况和成本报告对于 tenant/user/project/model/provider 浏览量和未来计费来说足够准确。
8. 安全回归涵盖身份验证、租户隔离、RBAC、对象授权、上传验证、Provider SSRF、敏感编辑、任务状态转换、SSE重放和前端禁止模式。
9. 端到端发布验证证明了核心卖家流程：初始化管理、配置Provider/model、创建项目、上传参考图片、提交generation/edit任务、接收SSE更新、查看输出资产、下载、重新编辑和检查logs/usage.

## 剩余路线图

### P11：身份、团队和RBAC管理

目标：完成multi-user/team控制平面。

建议订单：

1.`P11-BE-USER-ROLE-ADMIN`
   - 后端租户用户CRUD、disable/enable、角色分配、role/permission读取。
   - 完成并合并。它保留现有的 auth/session/RBAC 行为，阻止自我禁用和最后活跃管理员丢失，并且需要 `role:manage` 进行角色分配，并需要 `user:disable` 进行状态更改。
2.`P11-FE-USER-ROLE-ADMIN`
   - 管理员UI，用于用户、角色、权限、状态更改和角色分配。
   - 完成并合并。它使用合并的后端合约、门 UI 和按权限加载数据，避免呈现不安全的响应字段，保持创建的用户密码暂时性，并使用 `/disable`、`/enable` 和 `/roles` 使用 CSRF 写入端点。
3.`R11`
   - 已完成。全面的P11范围审查和回归没有发现阻塞问题。

并行性：P11已完成。从最新的`main`移至P12。

### P12：卖家工作流程和历史记录完成情况

目标：使 project/history/asset 工作流程连贯起来，供卖家日常使用。

建议订单：

1.`P12-FE-UNIFIED-HISTORY`
   - 将前端历史记录切换为`GET /api/v1/projects/{projectId}/history`。
   - 修复历史加载、清空、错误、分页、详细信息、下载和重新编辑状态。
   - 完成并合并。浏览器不再通过加入任务和generated/edited资产列表来构建生产历史提要。
2.`P12-FE-PROJECT-WORKFLOW-POLISH`
   - 改进项目creation/editing/member入口点和资产管理人体工程学。
   - 保留现有的UI概念；不要重写应用程序外壳。
   - 完成并合并。项目编辑、资产过滤器、资产元数据编辑、项目成员API入口点和项目切换过时状态保护现在位于前端。
3.`P12-BE-PROJECT-MEMBER-HARDENING`
   - 添加缺失的项目成员不变量，例如在适当的情况下防止丢失最后一个`OWNER`。
   - 完成并合并。后端成员update/delete路径现在至少保留一个项目`OWNER`，并在成功操作日志中保留被阻止的尝试。
4.`R12`
   - 端到端卖家工作流程审查。
   - 完全的。在统一历史记录、卖家project/asset工作流程、项目成员 API、last-`OWNER`保护、权限、操作日志和禁止的前端模式中未发现阻塞问题。

并行性：P12已完成。连续移动到P13，因为运行时设置在后端消费者存在之前不得公开。

### P13：运行时设置、配额和存储生命周期

目标：通过将每个可写字段连接到运行时使用者，使管理设置诚实且可操作。

建议订单：

1.`P13-BE-RUNTIME-DEFAULTS`
   - 完成并合并。仅当省略`providerId`和`modelId`时，租户`taskDefaults.{defaultProviderId,defaultModelId}`通过系统设置存储并由任务创建消耗。
2.`P13-BE-RUNTIME-DEFAULTS-HARDENING`
   - 完成并合并。格式错误或单方面持续存在的`task_defaults`现在无法在公共任务验证合约下关闭，而不会产生副作用；显式的Provider/model提交仍然独立于损坏的未使用的默认值。
3.`P13-BE-CONCURRENCY-POLICY`
   - 完成并合并。 `taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}` 仅在环境硬上限内可写，并由 Worker Redis 信号量获取消耗；全局并发仍然是环境所拥有的。
4.`P13-BE-STORAGE-CLEANUP-FOUNDATION`
   - 完成并合并。上传对象回滚使用独立的有界清理上下文，软删除资产具有内部租户范围的物理清除基础，并具有持久的 `purged_at` 跟踪。
5.`P13-BE-STORAGE-RETENTION-RUNTIME`
   - 完成并合并。 Nullable `storageRetention.deletedAssetRetentionDays` 只能由 Worker 维护使用者写入； unset/null 禁用自动物理清理，并且格式错误的设置以失败方式关闭（fail closed）。
6.`P13-BE-STORAGE-QUOTA-ACCOUNTING`
   - 完成并合并。 Nullable `storageQuota.maxBytes` 仅适用于引用上传和 Worker 输出消费者； `storageQuota.usedBytes` 是只读的，根据租户范围内未清除的资产元数据计算得出。
7.`P13-FE-SYSTEM-SETTINGS`
   - 完成并合并。前端管理UI仅公开活动的运行时支持的设置，并为每个设置组发送一个受CSRF保护的顶级补丁。
8.`R13`
   - 已完成。在运行时设置诚实、格式错误的设置失败关闭行为、Worker消耗的并发和保留设置、存储配额强制执行、前端设置安全和禁止的前端模式中未发现阻塞问题。

并行性：主要是串行，因为在运行时消费者存在之前不得公开设置字段。

### P14：Provider，模型和成本运营

目标：强化 Provider/model 操作和 usage/cost 实际操作报告。

建议订单：

1.`P14-BE-PROVIDER-MODEL-INTEGRITY`
   - 完成并合并。 Provider delete/disable 和模型 create/update/enable 现在保留 Provider/model 生命周期完整性。 P18后来收紧了同样的-Provider`model_name`写入交易序列化和重复拒绝。
2.`P14-BE-USAGE-COST-REPORTING`
   - 完成并合并。 Worker 成本估算是确定性的，后端使用摘要支持 tenant/user/project/Provider/model 具有精确小数成本的维度。
3.`P14-FE-COST-OBSERVABILITY`
   - 完成并合并。现有的管理可观察性使用选项卡现在公开租户总数、成本感知过滤器、深入分析、多货币显示以及由合并的 usage/cost API 支持的过时响应保护。
4.`R14`
   - 已完成。 Provider生命周期、数据完整性、usage/cost报告和前端成本可观察性经过审查和回归，没有出现阻塞问题。

并行性：P14已完成。从最新的`main`移至P15。

### P15：发布强化和端到端QA

目标：为运维人员运行的服务器部署准备完整的平台。

建议订单：

1.`P15-E2E-CORE-FLOWS`
   - 完成并合并。自动后端集成覆盖范围现在可以验证 init-admin、Provider/model 设置、项目、参考上传、任务创建、假 Worker 成功、SSE 重放、输出资产下载、历史记录、使用情况和日志，而无需外部 AI 调用。
2.`P15-SECURITY-FINAL-REGRESSION`
   - 完成并合并。添加了可重用的最终安全回归入口点、核心流程测试中的低权限否定断言、frontend/backend/deploy安全扫描以及身份验证、租户隔离、RBAC、对象授权、上传验证、ProviderSSRF、敏感编辑、任务的显式映射state/SSE 重播、前端禁止模式和部署配置检查。
3.`P15-DEPLOY-RUNBOOK-FINAL`
   - 完成并合并。新增部署发布验证脚本和运维人员运行手册，并验证Composeconfig/build/up/health/proxy/down清理工作。
4.`R15`
   - 已完成。最终的发布准备审查已通过，没有出现阻塞问题。

并行性：P15已完成。未来的后R15工作应在选择下一个范围后从最新的`main`开始。

### P16：生产启动强化

目标：在稳定的生产推出之前消除剩余的启动阻碍运营风险。

建议订单：

1.`P16-DEPLOY-SCRIPT-HARDENING`
   - 第一个任务。哈登`scripts/deploy-release-validation.sh --up --down`因此失败的实时验证仍然尝试清理，并且不会留下项目Compose容器或卷。
   - 为清理陷阱行为添加脚本级回归覆盖，无需使用真正的秘密或外部AIProvider。
   - 完成并合并。该脚本现在使用 `--up --down` 范围内的清理陷阱，对 failure/signal 清理路径具有假命令回归覆盖，并通过了实时 Compose `--up --down` 验证，没有留下任何项目容器或卷。
2.`P16-BE-LOG-RETENTION`
   - 在将保留公开为可写运行时状态之前，为 operation/API/task/error 日志保留实现真正的 backend/Worker 消费者。
   - 清理必须是租户安全（如果适用）、批次限制、可审核且敏感日志安全。
   - 完成并合并。实现的范围仅是现有数据库支持的日志：`operation_logs`、`api_call_logs`和终端任务`task_events`。容器stdout/stderr和外部日志聚合保留仍然是部署责任。
3.`P16-BE-THUMBNAIL-POLICY`
   - 决定并实施制作缩略图政策。
   - 首选目标：为上传的引用和Worker输出生成MinIO缩略图对象，仅将metadata/object键存储在MySQL中，并使前端asset/history视图使用授权的缩略图访问权限。
   - 完成并合并。新的参考上传和Worker输出在MinIO中生成有界的JPEG缩略图，保留`thumbnail_object_key`，公开授权的同源缩略图URL，保持旧的无缩略图资源可用，并保留清理回滚行为。
4.`R16`
   - 在进入长期运行的存储操作之前，批量审查生产启动强化。
   - 完全的。完整的P16审查和回归没有发现阻塞问题。

并行性：P16已完成。从最新的`main`移至P17。

### P17：存储治理和可观察性

目标：在存储增长、清理漂移和queue/runtime可见性需求下，使长期运行的生产操作更加安全。

建议订单：

1.`P17-BE-ORPHAN-CLEANUP`
   - 添加MinIO孤儿发现、试运行、执行、重试和审核支持。
   - 必须由元数据限定租户范围、批次限制、默认保守，并且绝不能仅仅因为存储桶列表看起来不熟悉而删除对象。
   - 完成并合并。管理存储孤儿scan/cleanup现在默认使用试运行、显式清理确认、有界列表、不透明游标、年龄门控、元数据排除、已脱敏审核和重试安全故障处理。
2.`P17-BE-STORAGE-QUOTA-RESERVATION`
   - 为并发上传和 Worker 输出写入添加严格的配额reservation/counter行为。
   - 包括计数器与元数据的协调以及失败预订的明确行为。
   - 完成并合并。参考上传、Worker 输出持久性、物理清除记帐、过时预留协调和失败关闭的畸形状态处理均处于活动状态。
3.`P17-BE-OBSERVABILITY-METRICS`
   - 添加仅限管理员的生产诊断，用于 API/Worker 运行状况详细信息、队列深度、running/failed 任务计数、Provider 故障率、存储使用情况和维护作业结果。
   - 这可以在 Prometheus 或外部监控集成之前作为 JSON 诊断开始。
   - 完成并合并。诊断端点是只读的、租户范围的、权限门控的、有界的和仅聚合的。
4.`R17`
   - 在最终的Provider/admin一致性工作之前检查存储治理和可观察性行为。
   - 完全的。完整的P17审查和回归发现没有阻塞问题。

并行性：P17已完成。从最新的`main`开始连续P18。

### P18：生产信心和Go/No-Go

目标：证明系统可以以真实的Provider配置和稳定的管理一致性来运行。

建议订单：

1.`P18-BE-PROVIDER-MODEL-SERIALIZATION`
   - 加强Provider/modelenable/disable/delete/update的交易序列化和默认设置交互。
   - 重新审视相同的-Provider`model_name`唯一性作为明确的product/data-integrity决定。
   - 完成并合并。写入路径现在强制执行同一 Provider非删除modelName唯一性，而无需进行破坏性迁移。
2.`P18-E2E-REAL-PROVIDER-SMOKE`
   - 添加可选的手动真实Provider冒烟测试脚本。
   - 它不得在默认 CI 中运行，不得在 commit 真实密钥下运行，并且必须具有明确的成本控制。
   - 完成并合并。该脚本是选择加入、仅后端、有成本限制、直接Provider 保护的，并由 fake-curl 安全测试覆盖。
3.`P18-PROD-DRY-RUN`
   - 针对目标或登台服务器执行运行手册：init admin、tenant/user setup、Provider/model config、假或真实任务、backup/restore、回滚和 security/deploy 门。
   - 已完成存储库控制的证据：安全默认和实际 Compose模式已通过，清理过程中没有留下任何项目容器或卷，并且没有执行任何真正的Provider调用。
4.`R18-STABLE-PRODUCTION-READINESS`
   - 主代理Go/No-Go审核以稳定生产。
   - 审计发现TLS、CI、密钥轮换、租户配置和演练差距后，继续进行P19/P20运营强化。

并行性：P18存储库控制的工作已完成。剩余的生产运营工作在P19/P20.中跟踪

### 管理控制台与运营数据整改（2026-08）

目标：将三个割裂的旧管理弹窗一次性切换为独立运营控制台，并建立后端权威统计与统一简体中文展示层。

完成范围：

1. 固化生图任务、实际出图、活跃用户、成功率、P95、预计费用、定价覆盖率和单张预计费用口径。
2. 增加用量费用状态、定价快照、时间聚合索引以及 `/admin/analytics/*` 聚合、详情和中文 CSV 接口。
3. 上线 `/admin/overview`、`/admin/usage`、`/admin/users`、`/admin/requests`、`/admin/providers`、`/admin/audit`、`/admin/settings` 七个模块。
4. 将状态、生命周期、审计动作、错误类别、单位、币种和时间格式集中到前端中文展示字典；未知值使用中文兜底。
5. 管理筛选写入 URL，图表提供中文数据表或文字摘要，详情使用侧边抽屉，技术 ID 和脱敏载荷默认收起。
6. 工作台只保留一个“管理控制台”入口，并按当前管理员权限进入首个可用模块；旧三个弹窗入口不再展示。

后续范围：供应商真实账单对账与“预计费用/实际账单”双层费用体系不在本轮实现中。

## worktree 调度策略

- 公共合同和阶段计划仅由主要代理更新。
- 新功能工作从最新的`main`开始。
- 使用“串行合同优先 -> 有限并行实施 -> 串行审查和集成”。
- 新区域的第一个任务应该是连续的。仅当写入范围独立且共享合约已合并时才允许并行。
- 从P11开始的每个worktree任务都必须包括：
  - 任务名称
  - 目标
  - 允许的文件
  - 禁止的文件
  - 依赖关系
  - 具体开发内容
  - 安全要求
  - 验收标准
  - 测试命令
  - 保留现有的行为
  - 允许的中间状态
  - 禁止半迁移状态
  - 故障模式和边缘情况
  - 所需的回归测试

## 标准回归命令

前端：

```bash
cd frontend
npm run lint
npm run type-check
npm run test
npm run build
```

后端：

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

完整部署验证保留给deployment/release任务，并且必须在之后清理Compose堆栈，除非用户要求保留它。

## 当前优先级

从最新的`main`完成P20：

1. 完成前端租户名称和自定义角色管理UI。
2. 运行最终稳定生产逐项审核、全面回归、实际 Compose演练、推送`main`后托管CI验证。
