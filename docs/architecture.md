# 架构

## 原始前端基线

该存储库最初是一个纯前端本地应用程序：

- React + TypeScript + Vite。
- TailwindCSS。
- Dexie / IndexedDB 用于图像二进制数据、历史记录和提示模板。
- localStorage 用于设置和 Provider API 键。
- 用于OpenAI、Gemini/Nano Banana 和OpenAI兼容中继调用的前端Provider适配器。
- 静态 Nginx Docker 部署。

该基线在早期平台化阶段得到保留，因此现有的UI概念可以迁移而不是重写。

## P21 期间的当前平台状态

仓库结构上分为`frontend/`、`backend/`、`deploy/`、`docs/`，已完成P18生产试运行、P19/P20操作强化以及多项P21生产可靠性切片。该平台现在拥有backend/frontend基础设施、authentication/RBAC、租户用户和角色管理、项目管理、具有最后`OWNER`保护的项目成员管理、MinIO支持的reference/generated/edited资产、后端为新资产生成的授权缩略图、卖家project/asset工作流程完善，Provider/model管理，任务API，SSE交付，可靠Redis排队，Worker处理，真正的后端Provider适配器运行时，统一后端项目历史记录，usage/audit读取，独立的`/admin/*`运营控制台，后端聚合统计与中文CSV导出，用量成本快照与时间索引，运行时支持upload-policy/task-default/task-concurrency/storage-retention/storage-quota/log-retention设置，保守的孤立清理，严格配额保留，管理诊断，发布验证，安全回归，部署运行手册，可选真实Provider冒烟测试工具，主机TLS代理模板检查，前端依赖审核门，现有租户内置角色协调、固定CSRF标头合约、Redis支持的登录速率限制、序列化启动迁移、Worker配额协调运行时、Provider尝试分类账、前端遗留IndexedDB image/history清理，以及Docker Compose发布检查。

当前重要事实：

- 生产前端工作台现在使用后端模型功能、后端任务创建、SSE任务状态、授权后端资产和后端统一项目历史记录。
- 浏览器Provider适配器、前端Providerregistry/types和正常本地ProviderAPIKey/APIURL设置已被删除。
- IndexedDB不再是生成图像或历史记录的生产源。它仅支持非敏感提示模板，Dexie v2 升级路径会删除已退役的image/history 商店。
- 后端目前有配置、日志记录、路由器、运行状况、响应助手、中间件、显式 MySQL/GORM 迁移，`backend/internal/database`、身份验证、RBAC、user/role 管理 API、项目 API、项目成员 API、资产 API、MinIO存储抽象，上传验证，授权下载，ProviderAPI，模型API，API密钥加密，SSRF验证Provider测试，任务API，SSE重播，可靠Redis排队，Worker状态转换，后端Provider适配器运行时，MinIO输出资产，使用记录，API调用日志，操作日志，audit/usage读取API，`/admin/analytics/*`聚合与导出 API，费用状态和定价快照，运行时支持upload-policy/task-default/task-concurrency/storage-retention/storage-quota/log-retention设置、生产秘密守卫、Worker进程并发、API Redis订阅者生命周期所有权、保守的孤儿清理、严格的配额保留、租户范围的诊断、Provider/model/default-setting写入序列化、安全回归入口点、部署验证脚本和可选真正的Provider冒烟测试工具。
- 前端有API客户端、task/SSE客户端合约、身份验证集成、user/role管理UI、项目selection/creation/editing、项目成员入口点、项目资产upload/list/filter/favorite/delete/download/detail/metadata-edit UI，后端项目范围的参考选择`assetId`，独立的`/admin/*`管理控制台（经营总览、用量与费用、用户与活跃、任务与调用、中转站与模型、操作审计、系统设置），模板库，后端任务支持的工作台提交，后端结果渲染和后端unified-history/detail/download/re-edit流动。
- 前端消费后端拥有的项目历史查询`GET /api/v1/projects/{projectId}/history`；它不得通过在浏览器中加入任务和generated/edited资产列表来重建生产历史记录。
- 后端现在公开租户`taskDefaults`，并且仅当任务创建忽略两个Provider/model ID 时才使用它们。对于默认支持的请求，格式错误的持久默认值以失败方式关闭（fail closed），而不会产生任务创建副作用。
- 后端现在仅通过 Worker 运行时使用者公开租户 `taskConcurrency`。租户值可以缩小环境硬上限，全局并发仍然由部署拥有，并且格式错误的持久并发设置在 Provider 执行之前以失败方式关闭（fail closed）。
- 后端具有内部资产清理基础，用于上传回滚和软删除对象的物理清除。 Worker 维护现在消耗可空租户 `storageRetention.deletedAssetRetentionDays` 和 `logRetention` 设置； unset/null/malformed 设置关闭失败并且不删除任何内容。
- Docker Compose具有可构建的运行时基础，P15发布验证，P16清理陷阱，P18现场试运行清理证据，部署运行手册，外部TLS反向代理template/static检查器，以及可选的真实Provider冒烟测试工具。日常开发仍然使用`docs/local-development.md`中记录的共享本地MySQL/Redis/MinIO服务。

剩余的后续行动记录在`docs/development-plan.md`和`docs/security.md`中。 P21仍然需要SSE弹性、可撤销会话、并发租约续订、Worker准备以及最终稳定生产Go/No-Go审查。可写系统设置仅通过租户管理员和`system:settings:manage`公开，并限制在已有运行时消费者支持的字段内。

## 管理控制台与运营统计

- `/admin/*` 与 `/studio` 工作台分离。工作台负责生图交互，管理控制台负责运营数据、Provider/模型、审计和系统设置。
- 管理控制台固定为七个模块：经营总览、用量与费用、用户与活跃、任务与调用、中转站与模型、操作审计、系统设置；模块入口按当前管理员权限收敛。
- `/admin/analytics/*` 由后端直接从 MySQL 聚合任务、实际出图、用户活跃、模型调用和持久化用量，不由浏览器下载明细后临时重算。前端筛选条件写入 URL，导出由后端生成并限制最大行数。
- 预计费用只汇总任务产生时保存的用量记录和定价快照，按币种分开返回；历史记录不会使用当前模型价格回算。缺少有效定价时保留任务和出图，但费用标记为不可用。
- 运营查询、详情和导出仍按`tenant_id`隔离，并分别受`usage:read`、`user:read`、`audit:read`和既有 Provider/模型/系统设置权限控制；不具备用量权限的管理员不得看到费用字段。

## 目标平台架构

目标平台是一个多用户、多租户的图像生成平台：

- 前端：React + TypeScript + Vite + Tailwind CSS。
- 后端API：Go + Gin + GORM。
- 后端Worker：GoWorker进程。
- 数据库：MySQL8。
- 队列/缓存/锁：Redis。
- 对象存储：MinIO。
- 部署：Docker Compose。

## 目标存储库布局

```text
gpt-image/
  frontend/
    src/
    public/
    package.json
    vite.config.ts
    Dockerfile
    nginx.conf
  backend/
    cmd/
      api/
      worker/
    internal/
      auth/
      tenant/
      rbac/
      project/
      asset/
      task/
      sse/
      queue/
      provider/
      provideradapter/
      model/
      audit/
      settings/
      storage/
      database/
  deploy/
    docker-compose.yml
    mysql/
    minio/
    nginx/
  docs/
  agent-instructions/
  AGENTS.md
  .env.example
  README.md
```

此布局处于活动状态。显式迁移目前位于`backend/internal/database/migrations.go`。

## 服务边界

前端拥有演示、用户交互、本地草稿状态、API客户端调用和SSE消费。它不拥有 Provider 凭据、Provider调用、任务执行或持久业务数据。

API服务拥有身份验证、租户上下文、RBAC、业务 API、上传验证、任务创建、Provider/model管理、SSE交付、审核日志记录和使用查询。

Worker服务拥有队列消费、并发控制、Provider适配器执行、输出上传、使用情况提取、任务状态转换和任务事件持久化。

MySQL是最终事实来源。 Redis仅用于队列、锁、缓存、速率限制、并发信号量和临时加速。 MinIO 存储图像字节。

## 主要数据流

1. 用户通过`/api/v1/auth/login`登录。
2. 后端设置HttpOnly Cookie并返回当前用户元数据。
3. 前端加载项目、资产、可用的Provider以及启用的模型功能。
4. 用户提交生成或编辑请求。
5. API 验证权限和输入，创建 MySQL 任务，写入初始任务事件，并将 Redis 作业入队。
6. 前端打开或重用SSE任务流。
7. Worker声明作业，写入任务事件，调用Provider适配器，上传输出到MinIO，记录使用情况和API调用日志，并标记任务终端。
8. SSE 将排队、运行、输出、使用、完成、失败、取消、重试和心跳事件推送到前端。

## 兼容性策略

- 保留现有UI组件并适配后端API。
- 现有的提示、上传、参数、结果和历史UI概念保留。
- 浏览器Provider代码已从生产导入图中删除，不得重新引入。
- 如果获得批准，现有的本地历史数据只能通过明确的未来import/compatibility功能使用；它不是主要平台数据。

## 过渡保护机制

- 不要添加新的浏览器端 AI Provider 调用。
- 不要添加新的前端API密钥存储。
- 不要扩展旧版 Nginx AI 中继行为。
- 不要将IndexedDB作为任何新平台功能的最终事实来源。
- 新的平台功能必须围绕后端API、MySQL、Redis、MinIO、Provider适配器和SSE合约进行设计。
