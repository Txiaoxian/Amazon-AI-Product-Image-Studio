# 安全计划

## P21 运营强化期间的当前转型风险

当前 `main` 分支已完成 P15 发布强化、P16 生产启动强化、P17 存储治理与诊断、P18 生产试运行、合并 P19/P20 操作强化分片，以及多个 P21 生产可靠性分片。浏览器端 AI Provider 执行、浏览器端 Provider 凭据持久化，以及由 IndexedDB 支撑生成图片和历史记录的生产路径，均不再是可接受的平台行为。下表记录了已解决的转型风险及其当前状态，以避免未来再次引入这些问题：

|风险|以前的位置 |现状 |验收检查|
| --- | --- | --- | --- |
|前端商店 Provider API 输入 localStorage | `frontend/src/hooks/useSettings.ts` |已解决。删除了正常的Provider设置； Provider密钥仅通过后端Provider管理表单提交，不会持久保存在浏览器存储中。 |静态扫描和测试必须继续显示 Provider API key/API URL 持久存在于 localStorage、sessionStorage、IndexedDB、 URL params，或客户端可见的配置。 |
|前端直接调用OpenAI、Gemini、relay API | `frontend/src/providers/**`，旧浏览器Provider适配器 |已解决。浏览器Provider适配器文件和前端Providerregistry/types已删除；工作台生成仅创建后端任务。 |浏览器生成流程仅创建后端任务；生产前端代码中没有 Provider `Authorization` 标头或直接 Provider 主机出现。 |
| 图片二进制数据和历史是IndexedDB中的原始数据 | `frontend/src/db/**` |已解决。后端项目资产和任务历史是最终事实来源；剩余IndexedDB使用仅限于提示模板。 Dexie v2 在升级过程中删除了已停用的 image/history 存储。 |项目资产和任务历史API是主要数据源；旧的本地 blob 不会以静默方式上传，并且不得重新进入生产历史路径。 |
|旧版本地上传验证是客户端且仅基于 MIME | `frontend/src/lib/file.ts` 和本地老一代路径 |解决了生成路径。参考上传经过后端资产上传验证；前端预检查仅保留UX。 |后端资源上传拒绝伪造MIME、无效魔法字节、SVG、尺寸过大、像素数过多。 |

剩余的安全和强化风险：

- Provider/model生命周期完整性现已强化：Provider在存在同租户未删除链接模型时阻止删除，Provider在启用链接模型存在时阻止禁用，模型写入路径拒绝不可用Provider，失败的生命周期写入不会记录成功的操作日志，以及P18 写入路径序列化会拒绝重复的同租户 Same-Provider 未删除的 `model_name` 值，而无需进行破坏性迁移。
- Provider/model 管理仅限租户管理员。普通用户只能发现和使用显式分配的模型，任务创建会在持久或队列副作用之前重新检查分配。
- 普通用户登录使用存储在Redis中的短暂一次性验证码。验证码验证以原子方式消耗记录，仅在密码验证后才在服务器端决定管理员绕过，并且所有credential/captcha失败都使用相同的已脱敏响应。
- 如果需要精确的读取时间清理，包含非启发式秘密的历史脏行仍然需要未来的设计； P9 审核读取故意不会将 Provider 明文密钥解密扩大到管理读取路径，而无需可信的最小秘密源和生命周期。
- 可写系统设置仍然仅限于具有实时运行时消费者的字段。租户上传策略由资产验证支持，任务默认值由任务创建支持，`taskConcurrency`由Worker信号量获取支持，`storageRetention`由Worker维护清理支持，`storageQuota`由参考上传和Worker输出持久性检查，`logRetention`由Worker数据库日志清理支持。前端设置可能仅公开具有合并的后端消费者的设置；它不得公开手动清理触发器、原始 MinIO 列表或任何没有运行时使用者的字段。

已解决的过渡项目：

- P3删除了前端 Nginx AI中继路由。前端容器必须继续仅代理 `/api/` `backend-api`，并且不得代理 AI Provider。
- P5后端资源上传现在会在将参考图像存储在MinIO之前验证MIME、魔法字节、大小、尺寸和像素数。
- P5前端project/assetUI现在使用经过身份验证的后端项目和资产 API 来进行参考上传、元数据、favorite/delete和下载。它不直接与 MinIO 通信，也没有添加新的 AI Provider 直接调用、Provider API 密钥持久性、身份验证令牌持久性或任务轮询。
- P6后端Provider/model管理现在存储Provider API静态加密的密钥，仅返回屏蔽密钥元数据，验证Providersave/update/test的URL，记录已脱敏操作日志，并公开租户范围的 Provider/model API。
- P6前端Provider/model管理现在仅向后端API提交ProviderAPI密钥，仅显示屏蔽元数据，清除已提交和未提交的密钥草稿，并且不在浏览器存储中保留Provider密钥。
- P7Provider运行时现在在真正的Provider调用之前使用连接时SSRF安全的出站传输，并在持久化之前递归地编辑运行时元数据。查看修复明确涵盖显示为值和嵌套 JSON 映射键的 API 键。
- P7前端任务客户端工作现在使用EventSource/SSE合约，并且没有引入轮询、新的Provider直接调用或新的ProviderAPI密钥持久化。
- P8前端后端化将生产工作台替换为后端任务API + SSE + 授权后端资产，删除普通浏览器Provider设置，删除浏览器Provider适配器，删除`legacyFile`引用有效负载，并将history/detail/download/re-edit移动到后端资产和任务。
- R8验证了前端、后端和Compose配置回归。敏感前端静态扫描未返回浏览器 Provider 桌面、Provider 授权标头、直接 Provider 主机、任务轮询或敏感浏览器存储的生产代码命中。 Provider静态扫描命中仅限于后端Provider管理API消费者。
- P9 audit/usage 读取 API 现在使用共享递归编辑、租户范围查询、管理 RBAC 和确定性分页。审查修复集中了修订实施，并通过受控注入缝证明了精确的已知秘密清理，而无需扩大生产Provider密钥解密范围。
- P9生产启动强化现在拒绝占位符`JWT_SIGNING_SECRET`和占位符`API_KEY_ENCRYPTION_KEY`，在API或Worker启动可以在生产中继续进行，同时保持非生产默认值可用。
- P9 运行时设置现在仅公开租户上传策略并在后端资产上传验证中强制执行。延迟设置不会出现在响应中，并且会在写入时被拒绝，直到其运行时使用者存在为止。
- P9 前端管理端的可观测性/设置 UI 现在只使用后端管理契约，通过 `usage:read`、`audit:read` 和 `system:settings:manage` 控制各区域权限，保持列表分页，通过共享 API 客户端携带 CSRF 执行 PATCH 设置，并且不会在浏览器存储中保留 Provider 密钥、认证令牌、日志元数据或系统设置载荷。
- P9安全回归添加了针对SSRF、编辑、tenant/object授权、上传验证、task/SSE重播可见性、生产秘密卫士、前端生产导入安全的针对性测试，并删除了在期间识别的无法访问的遗留历史记录display/storage助手P8/R8.
- R9验证了完整的P9代码范围，前端lint/type-check/test/build，后端tests/race/vet/build，Docker Composeconfig/build/up/health，API健康状况，前端静态路线和Compose清理。 R9未发现阻塞安全问题。
- P10Worker池、SSE桥生命​​周期和Provider/model生命周期强化无需更改租户、Provider适配器、SSE重播、任务状态或敏感日志记录合约。
- P10前端管理可观察性强化为API调用详细信息添加了陈旧响应保护，保留详细元数据bounded/redacted，保留仅上传策略的系统设置，并且不会将Provider密钥、身份验证令牌、日志元数据或设置有效负载写入浏览器存储。
- P10后端历史记录查询现在提供只读、租户范围、项目授权的`GET /projects/{projectId}/history`端点。它使用后端拥有的任务输出、资产和任务连接​​；仅返回未删除的 generated/edited 输出资产；排除孤立行和跨租户行；并且不公开对象键、MinIO URL、图像字节、Provider秘密、身份验证标头、cookie 或 API 调用元数据。
- R10审查了完整的P10代码范围，发现没有阻塞安全问题。验证通过了前端lint/type-check/test/build、后端tests/race/vet/build、Docker Compose配置、禁止前端Provider/polling/storage扫描和空格检查。
- P11 后端用户管理 API 现在强制执行租户范围的用户和角色查询，每个操作都需要 RBAC，散列新创建的用户密码，编辑响应和操作日志中的敏感字段，阻止自我禁用，阻止最后一个活动租户管理员的丢失，需要 `role:manage` 在创建或角色替换期间分配角色，并要求`user:disable` 用于状态更改。该任务通过了后端测试、竞态测试、审查、API/Worker构建、Compose配置和空格检查。
- P11前端user/role管理UI现在控制进入、用户读取、role/permission读取、create/update/disable/enable以及通过当前会话权限进行角色分配。它通过共享的 API 客户端发送 CSRF 标头，在没有权限时不调用用户管理端点，禁用当前用户状态操作，避免呈现不安全的响应字段，并将创建的用户密码排除在localStorage、sessionStorage之外， IndexedDB，以及成功后UI。该任务通过了前端 lint、类型检查、目标测试、完整测试、构建和空格检查。
- R11审查了完整的P11代码范围，没有发现阻塞安全问题。验证已通过前端 lint/type-check/test/build、后端 tests/race/vet/build、Docker Compose 配置、P11 frontend/backend 敏感模式扫描和空格检查。
- P12前端统一历史迁移现在直接使用后端拥有的、租户范围的`GET /projects/{projectId}/history` feed。浏览器不再通过将任务列表与 generated/edited 资产列表连接来构建生产历史记录。 UI保留项目切换过时响应保护、授权detail/download/re-edit、后端`editSourceAssetId`、非泄漏历史错误和不安全的响应字段抑制。
- P12前端project/asset工作流程优化现在使用后端API进行项目编辑、资产过滤、资产元数据编辑、成员list/add/update/remove入口点、参考上传、下载、删除、收藏和用作参考。它保留了 CSRF 写入请求、过时的项目切换保护和过滤列表一致性，而无需添加 Provider 直接调用、浏览器 Provider 密钥存储或任务轮询。
- P12后端项目成员强化现在拒绝删除或降级最终项目`OWNER`，仅在另一个`OWNER`保留后才允许所有者转移，保留tenant/RBAC/project-role检查，并验证被拒绝的写入不会创建成功的操作日志。
- R12审查了完整的P12代码范围，没有发现阻塞安全问题。验证通过了前端lint/type-check/test/build、后端tests/race/vet/build、Docker Compose配置、禁止前端Provider/polling/storage扫描和空格检查。
- P13后端运行时默认值现在仅存储租户范围的Provider/modelID，在设置写入时验证启用的同租户所有权，重新验证默认支持的任务请求，并拒绝缺失、清除、过时、禁用、删除、跨租户或功能不兼容的默认值，而不会产生排队或成功的任务审核副作用。 Focused/full后端测试、竞态测试、审查、构建、Compose配置以及合并之前通过的空格检查。
- P13 运行时默认值强化现在会针对使用默认值的请求，将格式错误的持久化 `task_defaults` 行转换为已脱敏的 `422 VALIDATION_ERROR`，且不产生 task/event/enqueue/success-audit 副作用；显式 Provider/model 请求不依赖未使用的默认值；真实设置存储故障仍返回已脱敏的 `500 INTERNAL_ERROR`。
- P13 任务并发策略现在仅对 Worker 运行时使用者公开 tenant/user/Provider/model 限制。租户值可能仅缩小或匹配环境硬上限，全局并发仍然由环境拥有，Provider行限制仍然是额外的更严格的上限，并且格式错误的持久`task_concurrency`在Provider执行或成功output/usage/API-call副作用之前以失败方式关闭（fail closed）。
- P13 存储清理基础现在使用独立的有界清理上下文在对象写入后进行上传回滚，并为软删除资产添加内部租户范围的物理清理服务。它仅删除元数据选择的 original/thumbnail 早于调用者提供的截止时间的对象，将丢失的对象视为幂等成功，使失败的删除可重试，并使用 `purged_at` 跟踪物理清理。
- P13 存储保留运行时现在仅通过 Worker 维护使用者公开可空的 `storageRetention.deletedAssetRetentionDays`。未设置或清除的保留被禁用，格式错误的持久设置以失败方式关闭（fail closed），跳过不活动的租户，清理日志避免对象键、存储桶名称、MinIO URL、图像字节、授权、Cookie、JWT和ProviderAPI键。
- P14 Provider/model 生命周期完整性可防止启用的模型通过普通管理 API 指向禁用或删除的 Provider，重新验证加载的任务默认值，并使生命周期冲突响应保持不敏感。
- P14后端usage/cost报告保持成本估算的确定性和非负性，将无效定价视为零成本，而不会成功完成Provider任务，使使用摘要保持在跨tenant/user/project/Provider/model维度的租户范围内，并保留原始的递归编辑usage/API元数据。
- P14前端成本可观察性仅消耗同源后端管理使用API，通过`usage:read`控制使用视图，根据后端字符串保持成本显示，避免轮询和浏览器Provider密钥存储，并且不暴露原始Provider秘密， Authorization/Cookie/JWT 值、图像 base64、存储桶名称或对象键。
- P15核心流E2E覆盖范围现在可以验证init-admin、HttpOnly会话cookie、可读CSRFcookie、Provider/model设置、上传验证、假Worker执行的happy-path安全信封， SSE重放，输出资产下载、历史、使用和日志，无需调用外部AIProvider。
- P15最终安全回归现在提供`scripts/security-regression.sh`，并通过针对输出资产下载、项目历史记录、使用读取、操作日志、API调用日志和API调用详细信息的低权限否定断言扩展核心流程测试。该脚本还运行重点安全测试、前端生产禁止模式扫描、后端敏感标记扫描、前端`/api/`代理安全检查、Compose配置验证和空格检查。
- R15发布准备审查在最新的`main`上重新运行了最终的安全回归和部署验证；没有发现阻塞安全问题，并且实时 Compose 清理没有留下任何项目容器或项目卷。
- P16部署脚本强化现在向`scripts/deploy-release-validation.sh --up --down`添加了范围内的清理陷阱，包括失败和SIGINT/SIGTERM路径。验证确认在实时部署检查后没有项目Compose容器或卷剩余。
- P16后端日志保留现在仅通过Worker运行时使用者公开`logRetention`。清理是租户范围内的、批量限制的，跳过格式错误的设置失败关闭、保留SSE/recovery的非终端任务事件，并且仅记录已脱敏聚合审计元数据。
- P16 缩略图政策现在由后端拥有。新的缩略图仅从通过后端验证的图像生成，存储在MinIO中，并在登录、租户、project/member、RBAC和对象授权检查后通过`GET /api/v1/assets/{assetId}/thumbnail`访问。响应和日志必须继续避免存储桶名称、对象键、MinIO URL、图像 base64、授权、Cookie、JWT 和 Provider 秘密。
- P17孤儿清理是保守的。它仅通过后端代码扫描存储，删除资格需要识别的后端对象键模式以及可信MySQL元数据的缺失；原始的桶列表是不够的。 试运行和执行响应使用已脱敏聚合计数和hashes/opaque ID，而不是原始对象键。
- P13 存储配额核算现在仅向真正的后端消费者公开可空的 `storageQuota.maxBytes`。 `storageQuota.usedBytes` 是只读的，根据租户范围内未清除的资产元数据计算得出；上传和Worker输出配额失败不得创建成功的元数据、输出事件、使用记录、成功日志或泄漏对象标识符。
- P17 存储配额预留现在使配额执行在并发编写者下安全。预订 ID 和柜台内部仅适用于服务器； responses/logs/audit元数据不得泄漏它们或任何对象key/bucket/MinIOURL。失败的预留、过期的预留、清理失败、通过积极的最终确定尝试释放的预留以及格式错误的计数器必须在不创建成功的资产元数据的情况下关闭。
- P17 生产诊断作为仅管理、租户范围、只读、仅聚合的后端端点来实现。诊断不得暴露原始 Provider 有效负载、operation/API 日志元数据、队列有效负载内容、Redis 键、对象键、存储桶名称、MinIO URL、签名 URL、图像 base64、授权、Cookie、JWT 或Provider秘密。维护元数据解析按字段类型失败关闭：只有数字聚合计数、布尔标志、枚举状态和RFC3339时间戳可以生存。
- P18Provider/model/default-setting序列化添加行锁和写入路径重复模型名称拒绝，因此并发管理更改不能默默地留下不可用的默认引用或重复的活动模型名称。
- P18可选真实Provider冒烟测试工具仍然是手动且受成本限制的。它的默认help/dry-run模式不调用任何API，显式`--run`需要`REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS`，直接AIProviderAPI碱基被拒绝。
- P18生产试运行修复了临时文件清理注册错误，添加了故障路径回归，并通过了安全默认加实际 Compose演练以及项目范围的清理。
- P19主机TLS反向代理强化添加了可审计的Nginx模板和静态保护机制。公共流量必须终止TLS并且仅代理到环回前端`127.0.0.1:8080`；它绝不能直接路由到后端 api、AI Provider 或中继。
- P19 API 启动现在可以协调现有租户缺少的内置角色和授权，而无需删除自定义角色或授权。
- P20将CSRF请求标头合约固定到`X-CSRF-Token`，跨前端、后端、CORS、Compose和生产环境预检。自定义标头别名被拒绝，以失败方式关闭（fail closed）。
- P20前端tenant/custom-role管理仅使用具有CSRF保护写入的同源后端API。内置角色在 UI 中保持只读状态，并且角色草稿、密码、令牌和敏感响应不会保留在浏览器存储中。
- P21 Provider 桌面生命周期强化现在可以在Provider软删除期间加密擦除`encrypted_api_key`、`api_key_hint`和`api_key_updated_at`。 Provider主密钥轮换应用还会清除历史软删除的Provider仍包含凭据材料的行，而试运行报告仅计数擦除候选者。
- P21生产CSRF强化现在在后端启动和生产试运行预检时拒绝`APP_ENV=production`中的`CSRF_ENABLED=false`。
- P21 可靠的队列强化现在使用Redis Lua 原子迁移来进行重试、ack、死信、过时恢复和延迟升级。 MySQL支持的queued/retrying交付协调修复了缺失的Redis交付状态，同时保留MySQL作为任务最终事实来源。
- P21部署强化现在通过Compose/release-validation命令传播单个生产环境文件，编辑健康故障日志，为长期运行的服务配置有界`json-file`日志记录，并在backup/restore中保持准确的MinIO恢复语义演练。
- P21前端工作台强化现在通过后端任务`imageType`提交选定的亚马逊图像类型，在重新编辑时保留后端历史图像类型，并在提交前规范化无效草稿。
- P21 登录强化现在使用 Redis 支持的失败登录速率限制，由租户的不透明哈希、标准化电子邮件和 IP 控制。限制检查发生在用户查找之前，失败在无效后进行计数，成功登录会在成功 audit/session 响应之前重置计数器，Redis 限制器失败失败会关闭而不回显凭据。
- P21 迁移启动强化现在使用 MySQL 路径上的 MySQL 咨询锁序列化 API/Worker 迁移，仅跳过 SQLite/unit-test 路径的锁定，并且在应用的迁移的预期架构对象丢失时以失败方式关闭（fail closed）。
- P21 配额维护现在可以通过有界轮换活动租户批次来协调 MySQL 元数据中的租户存储配额计数器。它不使用 MinIO 列表作为配额真相，仅记录已脱敏聚合错误类型。
- P21Provider尝试账本强化现在会在外部Provider执行之前保留`ATTEMPTING`API调用行，并在成功、失败、超时或取消后完成它。预写失败会阻止 Provider 调用；最终确定失败使任务关闭失败，没有output/usage副作用。 Request/response元数据递归已脱敏并删除对象键、存储桶、MinIO URL、签名 URL、Authorization/Cookie/JWT/API-key/base64 字段。
- P21前端遗留清理删​​除了旧浏览器image/history存储库、遗留Blobresult/detail分支、对象URL结果挂钩以及未使用的base64/object-url文件助手。生产history/detail/download/re-edit仅保留后端asset/task/history； IndexedDB仅保留提示模板，并通过Dexie架构升级主动删除已退役的image/history商店。
- P21SSE弹性强化现在限制重放，即使没有Redis唤醒，也可以在心跳期间捕获持续的MySQL事件，并在Redis通知通道关闭时重新订阅。 Redis 仅保留唤醒路径。
- P21会话撤销强化现在存储用户`session_version`，将其包含在JWT中，拒绝身份验证中间件中的过时会话，在密码更改时轮换会话，在注销时撤销旧cookie，使禁用用户会话无效，并关闭会话版本或用户状态不再是当前的SSE流。
- P21并发租约续订强化现在可以在长时间Provider执行期间续订WorkerRedis信号量租约。更新失败会取消执行，并在成功输出、使用、API-调用成功或可写入完成任务副作用之前将任务关闭为`CONCURRENCY_LEASE_LOST`失败。
- P21Worker就绪强化现在仅在数据库、Redis和MinIO依赖项检查通过后写入Compose健康检查文件，报告脱敏依赖级失败，并在已完成时立即删除该文件`Worker.Run`退出。

存储和P5审查强化积压：

- 前端设置UI现在仅公开活动的运行时支持的设置。它显示`storageQuota.maxBytes`，只读`storageQuota.usedBytes`，可为空`logRetention`； UI 仍必须隐藏孤立清理、手动清理触发器、MinIO 对象列表、存储桶名称、对象键、Provider 秘密以及任何没有后端使用者的设置。
- 在 API 启动期间，为现有租户协调内置 `asset:*` 权限和其他内置授权。
- MinIO 存储桶创建或验证仍然是environment/deployment 的责任。
- 前端上传预检查限制目前仅限UX，而不是平台安全边界。在系统上传限制暴露给前端之前，后端上传验证仍然具有权威性。

P20操作控制：

- Provider主密钥轮换使用仅运维人员`backend/cmd/provider-key-rotation`CLI，默认试运行，显式应用确认，序列化事务处理，对任何坏行进行完全回滚，以及仅已脱敏计数输出。
- 第二个及以后的租户配置仅使用运维人员`backend/cmd/provision-tenant`CLI，直到存在有意的平台级管理模型。它的应用路径是明确确认和事务性的。
- 租户HTTP API 仍属于租户范围；租户管理会话绝不意味着平台范围的超级管理员权限。
- 自定义角色 HTTP 写入属于租户范围，CSRF 受内置角色的保护、审核和阻止。当用户分配存在时，删除失败。
- Backup/restore/rollback演练仅针对一次性动态命名的Compose项目运行，绝不针对共享开发或生产服务。

R20 发布阻止安全后续措施：

- JWT会话撤销、SSEreplay/catch-up、并发租赁生命周期和Worker准备情况已在P21中实现，并在R21中进行了充分验证frontend/backend回归、安全回归、默认部署验证、实际 Composehealth/proxy检查和清理检查。生产部署仍然需要运维人员提供生产秘密、目标环境backup/restore演练、可选的真实Provider冒烟测试以及明确的成本确认、远程CI以及外部发布批准。

## 身份验证

使用存储在 HttpOnly Cookie 中的JWT。

Cookie 要求：

- 仅限 Http。
- 生产安全。
- SameSite 保护。
- 合理的到期时间。

前端 JavaScript 不得读取身份验证令牌。

## 面向用户的语言和错误处理

- 平台拥有的UI文本、配置说明、验证消息和非技术错误应尽可能使用简体中文。
- 前端代码不得在用户可见文本中显示原始后端堆栈跟踪、原始 Provider 响应、原始基础设施错误、Authorization/Cookie/JWT 值、Provider API 键、MinIO 对象标识符或图像 base64。
- 后端路由处理程序应返回已脱敏消息。如果第三方Provider或基础设施组件返回英文文本，平台应将其包装或映射为简洁的中文解释，同时仅在授权日志中保留详细的已脱敏诊断。
- 精度所需的技术标识符，例如枚举值、MIME类型、模型 ID、请求字段名称和Provider类型代码可能保持不变。

## 授权

使用 RBAC 加上租户和对象级别的检查。

每个对象ID端点都必须验证：

- 租户所有权。
- 所需的权限代码。
- 项目成员身份或管理员权限（如果适用）。

## API Key 保护

- 在存储密钥之前对其进行加密 Provider API 密钥。
- 切勿将完整的API密钥返回到前端。
- 仅显示隐藏的元数据，例如提示和上次更新时间。
- 不要记录秘密。
- 前端只能在后端收集ProviderAPI密钥，以便立即提交管理表单。它绝不能保留 Provider API 键。旧版本地设置流程已在 P8 中删除； Provider 凭据只能通过后端 API 进行管理。

P6 Provider 管理层还必须强制执行：

- API密钥可以被Providercreate/update表单接受，仅用于立即提交到后端。
- 后端响应不得包含明文 API 密钥或加密的密钥材料。
- 更新没有 `apiKey` 的 Provider 必须保留现有的加密密钥。
- 旋转API密钥必须创建仅包含已脱敏元数据的操作日志。
- Provider测试必须仅在后端内存中使用解密的凭据，并且必须在日志或API响应之前编辑出站请求元数据。

当前 P6 Provider 后端状态：

- 实现了ProviderAPI密钥加密、屏蔽响应、旋转元数据、仅后端Provider测试以及递归操作日志元数据编辑。
- Provider 测试不会创建任务、资产或使用记录。
- 前端Provider/model管理已实施，不会保留ProviderAPI密钥或创建Provider直接浏览器调用。
- P9生产启动强化在API或Worker生产启动之前拒绝默认占位符`API_KEY_ENCRYPTION_KEY`。

## 敏感日志记录策略

不记录：

- 完整的API按键。
- 授权标头。
- 饼干。
- 密码。
- 图像base64。
- 原始上传图像字节。

记录已脱敏请求 ID、Provider ID、模型 ID、任务 ID、持续时间、状态和已脱敏错误。

## SSRF 防御

Provider 基础 URL 验证必须阻止：

- `localhost`。
- `127.0.0.0/8`.
- `::1`.
- RFC1918私人范围。
- 链接本地范围。
- 多播范围。
- Docker 内部服务名称。
- 重定向到被阻止的目标。

默认策略仅限HTTPS。

P6 必须在持久时间和出站 test/use 时间验证 Provider URL。仅保存时间验证是不够的，因为在保存 Provider 后，DNS 可能会发生变化。

Provider URL 验证必须拒绝：

- 非HTTP(S) 计划。
- 普通HTTP，除非未来明确记录的本地开发例外情况得到批准。
- 嵌入了凭据的 URL。
- 主机名是Docker Compose服务名称或解析为阻止的IP范围。
- 重定向落在被阻止目标上的链。

SSRF 需要测试Provider save/update/test 和真实的运行时执行路径。

当前 P6 Provider 后端状态：

- Providersave/update/testSSRF测试涵盖阻止方案、嵌入式、本地主机、环回、私有范围、本地链路、多播、Docker服务名称、DNS解析到阻止的范围以及重定向到阻止的目标。
- P7真实Provider适配器执行使用连接时IP验证/SSRF安全传输来防止DNS在验证和连接之间重新绑定。

P7 Provider 运行时要求：

- 在出站 HTTP 传输验证连接时的最终拨号目标之前，真实的 Provider generation/edit 呼叫不得开始。
- Provider运行时日志和api_call_logs必须递归地编辑授权、Cookie、API密钥、不记名令牌、图像 base64 和原始图像字节。
- Provider 运行时必须将模型功能行视为可信参数白名单。

当前 P7 Provider 运行时状态：

- 实现并合并上述运行时要求。
- 配置的 Provider API 密钥仅在后端内存中解密，作为已知秘密传递到编辑器中，并从持久元数据中删除，无论它们显示为值还是嵌套的 JSON 映射密钥。
- 剩余边界：既不作为已知秘密提供也不由启发式规则匹配的未知秘密不能被自动识别。这是编辑的一般限制，而不是当前配置的 Provider API 密钥的未覆盖路径。

当前P9审核读取状态：

- Audit/usage 读取响应使用与 Provider 运行时相同的共享编辑实现，而不是仅分叉的启发式副本。
- 当显式注入受信任的编辑器时，支持精确的已知秘密value/key删除。
- 生产管理员读取 API 目前默认仅进行启发式编辑。这是故意的：Provider明文API密钥不会被广泛解密，只是为了清除历史脏行。
- 如果未来的需求需要对历史非启发式秘密进行精确的读取时清理，首先设计范围狭窄的秘密来源、授权生命周期和保留策略；不要在读取处理程序中临时扩大解密范围。

P8迁移安全要求：

- 浏览器generation/edit流必须仅创建后端任务，而不能创建Provider`Authorization`标头。
- 任何在 P8 中保留的浏览器持久设置都必须是非敏感的 UI 首选项；禁止使用 Provider API 键和 Provider API URL。
- 如果保留，现有的本地历史记录 blob 可能仅保留为显式兼容性数据；它们不得默默上传到租户存储中或保留为正常的生产历史源。
- 工作台状态必须仅消耗SSE。即使在迁移回退处理期间，轮询仍然被禁止。

当前R12前端安全状态：

- 生产工作台generation/edit流程创建后端任务并使用SSE作为状态。
- 截至 R12 的静态扫描未发现生产代码直接访问 Provider 主机、设置 Provider `Authorization` 请求头、持久化 Provider 密钥、使用敏感浏览器存储或引入轮询循环。
- 静态扫描中剩余的 `providers` 命中项是后端 Provider 管理 API 路径，不是浏览器 AI Provider 调用。
- 剩余的 IndexedDB 使用必须仅限于提示模板。前端生产历史记录直接使用统一后端历史端点，旧 image/history 存储由 Dexie 升级路径删除。

## 上传防御

上传必须验证真实的文件类型和图像属性。 SVG 被禁止，因为它可以包含脚本和外部引用。

验证必须包括：

- MIME 允许名单。
- 魔术字节检测。
- 尺寸限制。
- 宽度和高度限制。
- 像素数限制。

客户端检查可能会保留用于用户体验，但它们不是安全边界。在任何对象存储在MinIO中之前，必须进行后端验证。

P5 上传实现必须在写入 MinIO 之前验证以下所有内容：

- 通过身份验证的用户可以访问当前租户中的目标项目。
- 请求 MIME 类型位于允许列表中。
- Magic bytes 解码为 JPEG、PNG 或 WebP。
- 解码图像尺寸和像素数在配置的限制内。
- 文件名仅被视为不受信任的元数据，并且不会直接在 MinIO 对象键中使用。

## CSRF 和 CORS

由于 auth 使用 Cookie，因此状态更改 API 需要 CSRF 保护。 CORS 必须仅限于已配置的前端来源，并且仅对可信来源启用了凭据。

## 速率和并发限制

将限制应用于：

- 登录尝试。
- 上传。
- 任务创建。
- Provider调用。
- SSE 连接。

全局、租户、用户、Provider 和模型维度必须存在并发限制。

## 审计

记录安全敏感操作的操作日志。元数据必须对审计有用，但对秘密来说已脱敏。

## 安全验收检查

- 无前端AIProvider调用。
- 无前端API密钥存储。
- 没有完整的 API 输入 API 回复。
- 日志中没有敏感数据。
- 对每个对象IDAPI进行对象级检查。
- 上传拒绝SVG和无效的图像字节。
- Docker前端没有AI中继路由。
- 任务状态使用SSE，而不是轮询。
