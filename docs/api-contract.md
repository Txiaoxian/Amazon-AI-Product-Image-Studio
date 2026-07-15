#API合约

## API 基础路径

所有平台 API 均使用：

```text
/api/v1
```

身份验证使用 HttpOnly Cookie。前端请求必须包含凭据。

## 响应形状

成功响应：

```json
{
  "data": {},
  "requestId": "req_..."
}
```

分页成功：

```json
{
  "data": {
    "records": [],
    "total": 0,
    "pageNum": 1,
    "pageSize": 20
  },
  "requestId": "req_..."
}
```

错误响应：

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request.",
    "details": {}
  },
  "requestId": "req_..."
}
```

## 常见HTTP语义

- `400`：格式错误的请求。
- `401`：未经过身份验证。
- `403`：已验证但未授权。
- `404`：资源未找到或对此不可见tenant/user.
- `409`：冲突。
- `422`：验证失败。
- `429`：速率限制或并发限制。
- `500`：已脱敏消息的内部错误。

## 命名和格式

- JSON字段使用camelCase。
- 枚举值使用UPPER_SNAKE。
- 时间字段使用 ISO 8601 字符串。
- 分页查询参数：`pageNum`、`pageSize`。
- 排序查询参数：`sortBy`、`sortOrder`。

## 身份验证 API

- `POST /auth/init-admin`：初始化第一个管理员。
- `POST /auth/captcha`：创建一次性验证码挑战。
- `GET /auth/captcha/{captchaId}/image`：读取短暂的PNG验证码图像。
- `POST /auth/login`：登录。
- `POST /auth/logout`：注销。
- `GET /me`：当前用户、租户、角色和权限。
- `PATCH /me/password`：更改密码。

登录合约：

- 前端登录表单发送`email`、`password`和可选的`captchaId` / `captchaCode`；它不会要求用户输入`tenantId`。
- `tenantId` 仍然是 API/operator 客户端的可选兼容性字段。具有多个活动租户的部署应为特定于租户的登录入口点配置 `AUTH_DEFAULT_TENANT_ID`。
- 验证码挑战是短暂的，存储在服务器端Redis中，并在第一次验证尝试时自动消耗。
- 活跃租户管理员可以省略验证码字段。非管理员用户必须在`AUTH_CAPTCHA_ENABLED=true`时提交有效的挑战。
- 密码无效、验证码丢失、验证码无效、验证码过期使用相同的已脱敏`401 INVALID_CREDENTIALS`响应；验证码和角色状态不得泄露。

## 租户和用户 API

- `GET /tenants/current`
- `PATCH /tenants/current`
- `GET /users`
- `POST /users`
- `GET /users/{userId}`
- `PATCH /users/{userId}`
- `POST /users/{userId}/disable`
- `POST /users/{userId}/enable`
- `POST /users/{userId}/roles`
- `GET /users/{userId}/ai-access`
- `PUT /users/{userId}/ai-access`

所有用户 API 都需要租户范围和适当的 RBAC 权限。

租客合同：

- 额外的租户配置仅是运维人员CLI工作流程，而不是未经身份验证的HTTP端点，也不是隐式平台超级管理员API。 CLI 以事务方式创建一个租户、其内置的 roles/grants 和一个初始租户管理员。
- `GET /tenants/current` 仅返回经过身份验证的调用者的租户元数据。
- `PATCH /tenants/current` 只能更新当前租户`name`。它不得接受`tenantId`、`status`、凭据、设置或任意元数据。
- 租户元数据写入需要租户管理员访问权限加上`system:settings:manage`，使用经过身份验证的租户范围，并写入已脱敏操作日志。

P20租户实施状况：

- `backend/cmd/provision-tenant` 将额外租户配置实现为默认安全的运维人员CLI，并明确确认交易应用。
- `GET /tenants/current`和`PATCH /tenants/current`是在经过身份验证的租户范围内实现的。 PATCH仅接受`name`。

P11实施情况：

- `GET /users`、`GET /users/{userId}`、`POST /users`、`PATCH /users/{userId}`、`POST /users/{userId}/disable`、`POST /users/{userId}/enable`、`POST /users/{userId}/roles`、 `GET /roles`和`GET /permissions`已实施。
- 用户响应仅包含安全公共字段：`id`、`tenantId`、`email`、`displayName`、`status`、`lastLoginAt`、时间戳和角色摘要。它们不得包含 `passwordHash`、JWT、CSRF 令牌、Cookie、授权标头或内部敏感字段。
- `POST /users`需要`user:create`；在创建过程中分配任何`roleIds`还需要租户管理员访问权限或`role:manage`。
- `PATCH /users/{userId}`可能会更新安全字段，例如`displayName`；更改`status`还需要租户管理员访问权限或`user:disable`。
- `/disable`和`/enable`需要租户管理员访问权限或`user:disable`。
- `POST /users/{userId}/roles` 需要租户管理员访问权限或 `role:manage`，验证所有角色 ID 都是调用者租户中的活动角色，并以事务方式替换角色。
- 后端拒绝自禁用以及任何会删除租户中最后一个活动管理员的操作。
- 用户创建、更新、disable/enable、角色替换写入已脱敏操作日志。
- 前端user/role管理UI通过共享API客户端使用这些端点，在写入时发送CSRF标头，通过权限控制数据加载和控制，并将初始密码仅保留为临时表单输入。
- `GET /users/{userId}/ai-access` 和 `PUT /users/{userId}/ai-access` 仅限租户管理员。 PUT以事务方式替换`modelIds`，拒绝missing/cross-tenant/deleted模型，并写入已脱敏操作日志。
- 普通用户不会收到自助服务Provider/model授予端点或管理控制权。他们仅从生成参数中指定的启用模型中进行选择。

## RBAC API 接口

- `GET /roles`
- `POST /roles`
- `PATCH /roles/{roleId}`
- `DELETE /roles/{roleId}`
- `GET /permissions`
- `PUT /roles/{roleId}/permissions`

角色管理合同：

- 内置 `admin`、`seller` 和 `viewer` 角色通过租户 HTTP API 保留且不可变。启动协调拥有其所需的补助金。
- 租户管理员只能在经过身份验证的租户内创建、更新、禁用和删除自定义角色。
- `PUT /roles/{roleId}/permissions` 使用全局权限字典中的权限 ID 以事务方式替换一个自定义角色的授权。
- 当自定义角色分配给任何用户时，删除该角色必定会失败。成功删除仅删除该角色的租户范围授权和角色行。
- 角色写入端点需要租户管理员访问权限或`role:manage`，强制对象级租户范围，使用CSRF保护，并记录已脱敏操作日志。

P20角色管理实施状况：

- 实现自定义角色create/update/delete和权限替换。
- 内置角色突变、跨租户角色 ID、无效权限 ID 以及分配的自定义角色的删除在没有部分写入的情况下失败。

## 项目 API

- `GET /projects`
- `POST /projects`
- `GET /projects/{projectId}`
- `PATCH /projects/{projectId}`
- `DELETE /projects/{projectId}`
- `GET /projects/{projectId}/members`
- `GET /projects/{projectId}/member-candidates`
- `POST /projects/{projectId}/members`
- `PATCH /projects/{projectId}/members/{userId}`
- `DELETE /projects/{projectId}/members/{userId}`

项目对象 API 需要租户范围和项目成员资格或管理员权限。

P5的项目负载规则：

- Create/update字段：`name`、`brand`、`asin`、`site`、`notes`、`status`、 `sortOrder`。
- 需要`name`。
- `status`值：`ACTIVE`、`ARCHIVED`。
- `sortOrder` 是卖家工作区项目选项卡使用的整数。列表按 `sortOrder ASC` 排序，并具有确定性 timestamp/ID 决胜局。
- 回复记录包括`id`、`tenantId`、产品字段、`status`、`sortOrder`、`createdBy`、`createdAt`、`updatedAt`。
- 删除的项目被软删除并从正常列表中排除。

P5的项目会员规则：

- 会员角色值：`OWNER`、`EDITOR`、`VIEWER`。
- 项目成员API需要`project:member:manage`或租户管理员权限。
- 项目对象 API 必须结合 RBAC 权限和项目成员资格。例如，资产上传需要`asset:upload`和项目`OWNER`或`EDITOR`。
- 一个项目必须至少保留一个`OWNER`。更新或删除最终的`OWNER`返回`409 CONFLICT`，并且不得写入成功的项目成员操作日志。
- 在降级或删除原始`OWNER`之前添加或升级另一个`OWNER`，支持所有者转让。
- `GET /projects/{projectId}/member-candidates` 返回可能添加到项目中的活跃同租户平台用户。它支持有界搜索，例如`q`，仅返回安全显示字段，并且不得要求调用者输入原始用户 ID。
- 项目成员响应包括安全用户显示字段，例如 `userName`、`userEmail` 和 `userStatus`，因此前端可以显示名称而不是不透明的 ID。

目前P5实施状态：

- 后端实现项目CRUD、项目成员API、租户范围的对象授权、操作日志以及对成员update/delete路径的last-`OWNER`保护。
- 前端使用项目 API 进行项目选择、项目creation/editing以及卖家工作区项目成员管理入口点。
- 项目成员管理已为日常卖家工作流程做好了后端准备；稍后的前端改进可以添加更细粒度的成员突变错误状态，而无需更改此契约。
- 当前卖家工作区UI将项目显示为顶部选项卡，其中包含项目名称和品牌，按`sortOrder`排序。拖动选项卡可通过项目更新 API 更新项目顺序。项目edit/member管理是从二级管理模式打开的，而不是内联原始ID形式。

## 资产 API

- `GET /projects/{projectId}/assets`
- `POST /projects/{projectId}/assets/uploads`：上传参考图片。
- `GET /assets/{assetId}`
- `PATCH /assets/{assetId}`
- `DELETE /assets/{assetId}`
- `POST /assets/{assetId}/favorite`
- `DELETE /assets/{assetId}/favorite`
- `GET /assets/{assetId}/download`
- `GET /assets/{assetId}/thumbnail`：通过后端授权流式传输生成的缩略图。仅当资产已存储`thumbnail_object_key`时，该端点才可用；它不得公开 MinIO 存储桶名称、对象密钥、签名 URL 或公共对象 URL。
- `POST /assets/{assetId}/edit-source`：可选的未来便利端点，用于准备资产作为编辑参考。 P8不需要；后端工作台应直接通过任务创建提交`referenceAssetIds`和`editSourceAssetId`。

下载必须通过后端授权进行流式传输或使用授权后创建的短期签名 URL。

P5的资产负载规则：

- 上传使用`multipart/form-data`。
- 必填文件字段：`file`。
- 可选字段：`kind`、`category`、`filename`、`isFavorite`。 `category` 是资产的当前图像类型，仅接受 `MAIN`、`A_PLUS`、`SCENE`、`DETAIL`、`DIMENSION`、`SELLING_POINT`、 `PROMOTION`，或`COMPARISON`；省略时上传默认为`MAIN`。
- P5上传仅接受`kind=REFERENCE`。生成和编辑的资源由稍后的 task/worker 流创建。
- 资产响应字段：`id`、`tenantId`、`projectId`、`kind`、`category`、`filename`、`mimeType`、 `fileSize`、`width`、`height`、`thumbnailUrl`、`previewUrl`、`isFavorite`、`imageType`、 `createdBy`，`createdAt`，`updatedAt`。 `category` 和 `imageType` 都公开资源的当前图像类型。对于具有无效或空类别的旧行，后端会回退到源任务图像类型，然后回退到 `MAIN`。
- 资产列表支持`kind`、`category`、`favorite`、`imageType`、`pageNum`和`pageSize`查询参数。 `category` 保留为 `imageType` 的兼容性别名；两者都按资产的当前分类进行过滤，并且可以与 `favorite=true` 组合。
- `PATCH /assets/{assetId}`可能会更新`category`、`filename`和`isFavorite`；更改 `category` 会立即在产品材料和生成历史图像类型过滤器之间移动资产，而无需重写源任务。它不得更改 `tenantId`、`projectId`、`objectKey`、图像尺寸、MIME 类型或基础文件格式。当更新 `filename` 时，后端始终将其扩展名替换为根据存储的 MIME 类型（`.jpg`、`.png` 或 `.webp`）确定的扩展名。
- `DELETE /assets/{assetId}`是软删除。
- `GET /assets/{assetId}/download` 必须需要后端授权，并且不得暴露永久公共 MinIO URL。
- `GET /assets/{assetId}/thumbnail` 必须需要与资产读取相同的 tenant/object 授权信封。如果不存在缩略图，则返回已脱敏 404 并在 list/detail/history 负载中将 `thumbnailUrl` 保留为空，直到可用于该资产生成缩略图。
- 当前P5后端实现提供列表、上传、详细信息、更新、软删除、favorite/unfavorite和下载。 P8可以通过任务创建来后端编辑流程，而无需添加`edit-source`。
- 当前的P5前端实现通过经过身份验证的API客户端使用`credentials: include`消耗项目范围的资产list/upload和资产detail/update/delete/favorite/download。
- P5 前端下载使用后端下载端点作为 blob 响应；浏览器不得直接与 MinIO 对话。
- P5前端不能依赖于`POST /assets/{assetId}/edit-source`；选择资产作为本地引用是过渡 UI 直到P8后端化。
- P8工作台任务应直接在`referenceAssetIds` / `editSourceAssetId`中使用项目资产ID；上传的文件必须进入后端资源库才能成为持久的任务输入。
- P16缩略图政策处于活动状态：仅在存储后端生成的缩略图对象并且`thumbnail_object_key`之后，新的参考上传和Worker创建的generated/edited输出才会使用`/api/v1/assets/{assetId}/thumbnail`填充`thumbnailUrl`被坚持下来。没有缩略图的现有资源将保持空的 `thumbnailUrl`，直到存在显式回填任务。

## 管理存储孤立 API

P17 引入了保守的仅管理存储孤儿操作。这些 API 不是通用的 MinIO 浏览器，不得公开存储桶名称、对象键、MinIO URL、签名 URL、图像 base64、授权、Cookie、JWT 或 Provider 秘密。

- `POST /admin/storage/orphans/scan`：仅试运行。它返回聚合候选、跳过和错误计数以及已脱敏候选样本。
- `POST /admin/storage/orphans/cleanup`：仅当调用者显式设置`dryRun=false`并提供确认字符串如`DELETE_ORPHANS`时才执行删除；否则它表现为试运行。
- 所需权限：租户管理员或管理设置运维人员已可用的显式 storage/system-management 权限，例如 `system:settings:manage`，直到引入专用 `storage:manage` 权限。
- 请求字段应包含`bucketKinds`、`tenantId`（仅适用于系统范围的超级管理员使用（如果存在此类角色）、`minAgeHours`、`batchLimit`和可选光标）。租户管理员的范围必须限于他们自己的租户。
- 候选人资格必须要求具有公认的后端对象密钥模式并且不存在受信任的 MySQL 元数据。仅仅列出存储桶清单永远不足以成为删除的理由。
- 响应样本必须使用哈希值或不透明的候选 ID，而不是原始对象键。
- 对于已经丢失的对象，清理必须是幂等的，对于存储错误，清理必须是重试安全的。失败的删除必须在以后的扫描中保持可发现性。
- 每次执行都要记录已脱敏聚合操作日志。
- 实施状态：合并于`P17-BE-ORPHAN-CLEANUP`。 Scan/cleanup 是仅限后端的管理 API，具有试运行默认值、显式清理确认、有界列表、不透明连续游标、元数据排除和已脱敏审核。

## 任务 API

- `POST /projects/{projectId}/tasks`：创建generation/edit任务。
- `GET /projects/{projectId}/tasks`
- `GET /tasks/{taskId}`
- `POST /tasks/{taskId}/cancel`
- `POST /tasks/{taskId}/retry`

任务创建会保留任务并将 Redis 工作排入队列。任务进度必须通过SSE消耗。

P7 的任务请求字段：

- `type`：`IMAGE_GENERATION`或`IMAGE_EDIT`。
- `prompt`：必填文字提示。
- `providerId`：当前租户中的ProviderID。从P13开始，只能与`modelId`一起省略，在这种情况下，后端解析租户`taskDefaults`。
- `modelId`：当前租户中的型号ID，由Provider拥有。从P13开始，只能与`providerId`一起省略。
- `imageType`：可选的电子商务图片类别，例如`MAIN`、`A_PLUS`、`SCENE`、`DETAIL`、`DIMENSION`、`SELLING_POINT`、 `PROMOTION`，或`COMPARISON`。
- `referenceAssetIds`：用于generation/edit引用的项目资产 ID 的可选列表。
- `editSourceAssetId`：当所选模型需要编辑源时编辑任务所需。
- `parameters`：仅包含模型支持的值的结构化对象，例如大小、质量、输出格式、输出计数、宽高比和图像类型设置。

P7 的任务响应字段：

- `id`、`tenantId`、`projectId`、`type`、`status`、`prompt`、`providerId`、 `modelId`、`imageType`、`parameters`、`inputAssetIds`、`outputAssetIds`、`attempt`、`maxAttempts`、 `queuedAt`、`startedAt`、`finishedAt`、`timeoutAt`、`errorCode`、`errorMessage`、`createdBy`、 `createdAt`，`updatedAt`。

P7的任务状态：

- `QUEUED`、`RUNNING`、`SUCCEEDED`、`FAILED`、`CANCELLED`、`RETRYING`、`TIMED_OUT`。

P7任务API要求：

- 任务 API 需要 Cookie 身份验证和 CSRF 来进行状态更改请求。
- 项目范围的任务 API 必须检查租户、RBAC 以及项目成员资格或管理员访问权限。
- 任务创建必须在入队之前验证 Provider/model 启用状态、同租户所有权、模型功能、参考资产所有权和参数值。 `IMAGE_GENERATION` 和 `referenceAssetIds` 需要生成和编辑功能，因为 Worker 将参考引导生成映射到 Provider 图像编辑操作。
- 前端不得提供或覆盖 `tenantId`、`createdBy`、`status`、`attempt`、`queuedAt`或其他服务器拥有的字​​段。
- Redis入队有效负载应仅包含任务ID； Worker从MySQL重新加载完整状态。
- P7可能会向前端添加任务API包装器，但P8拥有替换主生成工作台流程的能力。

目前P7实施状态：

- 后端实现任务 create/list/detail/cancel/retry API、MySQL task/event/output/log/usage 架构、操作日志和 Redis 入队抽象。
- 任务事件重播光标为`task_events.sequence`； `task_events.id` 源自该序列，可以安全地用作 SSE `id`。
- 后端现在实现了SSE长连接、Worker执行、真实Provider运行时执行、generated/edited输出资产创建、使用记录和API调用日志。
- 前端现在有任务 API 包装器和 SSE client/reducer 合约实用程序。工作台后端化仍推迟到P8。
- P13 使用运行时支持的租户默认值扩展任务创建：当两个 Provider/model ID 均不存在时，后端解析 `taskDefaults` 并应用相同的验证和排队合约；明确的请求仍然有效，无需咨询默认值。

P8工作台合约：

- 工作台列出已启用的后端模型并使用功能响应来呈现允许的任务参数。
- 当选定的 Provider/model 被禁用、删除或在 UI 加载后变得不可用时，任务创建仍然是最终验证器。
- 浏览器代码不得构造超出平台任务合约的面向Provider的请求有效负载。

## 历史 API

- `GET /projects/{projectId}/history`：将后端生成的项目历史记录作为与其源任务配对的generated/edited资产的单个分页提要列出。

P10的历史查询参数：

- `pageNum`：默认`1`，正整数。
- `pageSize`：默认遵循后端分页默认值，并且必须受到 task/assets 列表使用的相同最大页面大小策略的限制。
- `kind`：可选`GENERATED`或`EDITED`；缺席意味着生成和编辑的输出资产。
- `imageType`：可选`MAIN`、`A_PLUS`、`SCENE`、`DETAIL`、`DIMENSION`、`SELLING_POINT`、 `PROMOTION`，或`COMPARISON`；如果存在，则仅返回为该图像类型创建的结果。

P10 的历史响应字段：

- 标准页信封：`records`、`total`、`pageNum`、`pageSize`。
- 每个`records[]`项目包含：
  - `asset`：与`GET /assets/{assetId}`相同的安全资产响应形状，包括后端download/previewURL，但从不`objectKey`、MinIOURL、图像字节，或 Blob 数据。
  - `task`：与`GET /tasks/{taskId}`相同的任务响应形状，包括`outputAssetIds`。

历史API要求：

- 端点是只读的，需要 Cookie 身份验证。
- 端点必须使用 `task:read` 和项目任务读取使用的相同 project-member/admin 访问语义来授权项目。
- 查询必须按 `tenant_id`、`project_id`、`image_assets.deleted_at IS NULL`、generated/edited 资产类型和同租户链接任务进行过滤。
- 后端必须从后端拥有的关系构建提要，最好是`task_outputs -> image_assets -> generation_tasks`；客户端提供的任务 ID、租户 ID 或 asset/task 联接永远不会被信任。
- 排序必须是确定性的，首先是最新的输出资产，并具有稳定的决胜局，例如资产ID。
- 没有同租户可见 task/output 链接的孤立 generated/edited 资产不得出现在历史记录中。
- 此端点不创建任务，不接触Redis/SSE，不读取或写入MinIO对象，并且不公开operation/API调用日志元数据。
- P10后端历史查询实现并合并。
- P12前端统一历史迁移已实现并合并。生产前端历史提要直接使用此端点，并且不得通过在浏览器中加入任务和generated/edited资产列表来重建提要。

## Provider API 接口

- `GET /providers`
- `POST /providers`
- `GET /providers/{providerId}`
- `PATCH /providers/{providerId}`
- `DELETE /providers/{providerId}`
- `POST /providers/{providerId}/test`
- `POST /providers/{providerId}/enable`
- `POST /providers/{providerId}/disable`

Provider访问边界：

- 所有 Provider list/detail 和突变端点仅限租户管理员。非管理员无法仅通过自定义 `provider:*` 权限获得 Provider CRUD 访问权限。
- 普通用户不需要Provider端点；分配的模型响应提供了生成选择器所需的安全`providerName`。

Provider API 需要 `provider:read` 或 `provider:manage`（视情况而定）。 Provider 对象 API 必须按 `tenant_id` 过滤；跨租户 Provider ID 应返回 `404` 或非公开授权失败。

Provider P6 的请求字段：

- `type`：`OPENAI`、`GEMINI`或`OPENAI_COMPATIBLE`。
- `name`：显示名称，必填。
- `baseUrl`：custom/OpenAI-compatibleProvider所需；官方Provider默认值可能由后端配置提供，但在使用前仍然通过SSRF验证。
- `apiKey`：仅在创建或显式时接受 rotation/update. 如果在更新时省略，则保留现有的加密密钥。
- `timeoutSeconds`：有界正整数。当前最大值为 `600` 秒。 UI 应该解释长超时是为了缓慢的图像生成而设计的，并且不能替代 Worker/task 超时策略。
- `concurrencyLimit`：有界非负整数。 `0` 表示使用 global/system 默认值，除非实现选择更严格的显式默认值。
- `status`：`ENABLED`或`DISABLED`； enable/disable端点是首选状态转换API。

Provider P6 的响应字段：

- `id`、`tenantId`、`type`、`name`、`baseUrl`、`status`、`timeoutSeconds`、 `concurrencyLimit`、`apiKeyHint`、`apiKeyUpdatedAt`、`lastTestStatus`、`lastTestedAt`、`createdAt`、`updatedAt`。
- Provider 响应绝不会包含完整的 API 密钥、加密的 API 密钥值、授权标头或原始 Provider 响应。

Provider P6 测试合约：

- `POST /providers/{providerId}/test` 执行仅后端的 connectivity/authentication 探测，具有超时和 SSRF 保护。
- 返回已脱敏字段，例如`status`、`durationMs`、`checkedAt`、`httpStatus`、`requestId`和`message`。
- 它不得创建生成任务、上传输出资产、写入使用记录或暴露原始 Provider 有效负载。
- 它应该写入操作日志并可能更新已脱敏`lastTest*`Provider元数据。

当前 P6 Provider 后端实现状态：

- 后端实现ProviderCRUD、软删除、enable/disable、Provider测试、租户范围查询、RBAC、操作日志、API密钥加密和屏蔽Provider 回复。
- Provider 测试仅用于后端，不会创建任务、资产或使用记录。
- 前端Provider/model管理已实施并合并。 UI 仅将 Provider API 键作为即时表单提交发送，仅显示屏蔽元数据，清除已提交和未提交的键草稿，并且不会在浏览器存储中保留 Provider 键。

P10 Provider 生命周期策略：

- 当同一租户中任何未删除的模型仍然引用 Provider 时，`DELETE /providers/{providerId}` 必须失败并显示 `409 CONFLICT`。
- 响应必须使用标准错误信封和非敏感代码，例如`PROVIDER_HAS_LINKED_MODELS`；它可能包括链接模型计数，但不得从其他租户泄漏模型名称。
- 软删除模型不会阻止Provider删除。
- 跨租户模型绝不能阻止或泄露其他租户的 Provider 删除。
- Provider 在此阶段删除不得级联删除或级联禁用模型。
- 当前P10实施状态：此删除政策已实施并合并。冲突响应使用 `PROVIDER_HAS_LINKED_MODELS`，测试涵盖同租户 enabled/disabled 链接模型、软删除链接模型、跨租户链接模型、RBAC/not-found 行为和非泄漏 responses/logs. Provider 禁用行为，后来通过P14。

P14 Provider 生命周期策略：

- `POST /providers/{providerId}/disable` 和 `PATCH /providers/{providerId}` 以及 `status=DISABLED` 必须失败并出现 `409 CONFLICT`，而同租户未删除的启用模型仍引用 Provider。
- 冲突响应使用 `PROVIDER_HAS_ENABLED_MODELS`，并且不得泄露模型名称、Provider 秘密、跨租户标识符、授权标头、Cookie 或原始 Provider 有效负载。
- Provider 当链接的同租户模型已被禁用时，禁用可能会成功；它不会级联删除或级联禁用模型。
- 模型创建、模型Provider迁移和模型启用必须拒绝禁用、删除或跨租户Provider。失败的写入不得记录成功的`provider.*`或`model.*`操作日志。
- 当前P18实施状态：Provider/model生命周期完整性已实施并合并。 Provider/model/default-setting 写入使用更强的行锁定，并且模型 create/update 路径拒绝重复的同租户 Same-Provider 未删除的 `model_name` 值，而无需进行破坏性的唯一索引迁移。

## 模型 API

- `GET /models`
- `POST /models`
- `GET /models/{modelId}`
- `PATCH /models/{modelId}`
- `DELETE /models/{modelId}`
- `POST /models/{modelId}/enable`
- `POST /models/{modelId}/disable`

模型访问边界：

- 租户管理员可以列出和管理所有同租户模型。
- 非管理模型list/detail读取需要现有的读取能力和`user_model_access_grants`中的显式同租户行；未分配的详细信息将作为未找到而返回。
- 即使自定义角色包含 `model:manage`，模型 create/update/delete/enable/disable 操作也仅限租户管理员。
- 在创建任何任务、事件、审核成功行或队列副作用之前，任务创建会在同一事务中重新检查选定的模型授权。

模型 API 需要 `model:read` 或 `model:manage`（视情况而定）。模型对象 API 必须按 `tenant_id` 进行过滤，且 `providerId` 必须属于同一租户。

P6 的模型请求字段：

- `providerId`：必填。
- `modelName`：Provider面向模型id/name.
- `displayName`：面向用户的模型名称。
- `supportsGenerate`、`supportsEdit`、`supportsMultiReference`、`supportsN`。
- `maxOutputCount`：正整数，受`supportsN`约束。
- `supportedSizes`、`supportedQualities`、`supportedOutputFormats`：字符串数组，按结构化JSON进行验证和存储。
- `supportedQualities` accepts normalized values for two Provider-specific meanings. OpenAI `gpt-image-2` uses the official ordered values `auto`, `low`, `medium`, `high`; the frontend displays them as “自动、低质量、中等质量、高质量”, while the adapter sends the original protocol values unchanged. Gemini output resolution remains `1k`, `2k`, `4k`, stored in lowercase and converted to Provider-required uppercase at request time. Legacy `standard`/`hd` values remain readable for existing rows but are not offered by the `gpt-image-2` preset.
- 对于OpenAI和OpenAI兼容`gpt-image-2`、`supportedSizes`存储平台的有序宽高比选择：`auto`、`1:1`， `1.62:1`、`2:3`、`3:2`、`3:4`、`4:3`、`4:5`、`5:4`、 `9:16`，`16:9`，`21:9`。在Provider调用时，后端适配器将非自动比率转换为OpenAI兼容的`WIDTHxHEIGHT`值。现有的显式像素值仍然被接受并保持不变以实现向后兼容性。
- Frontend model editing and generation use matching labels for those semantics: OpenAI-style `gpt-image-2` models show “图片比例 / 生成质量”; Gemini models show “画面比例 / 输出分辨率”. Presets are UI helpers only; the backend model row remains the trusted persisted capability source.
- `pricing`：结构化JSON，包含货币和单价；准确的Provider计费解释可以在P7使用量核算之前细化。
- `status`：`ENABLED`或`DISABLED`； enable/disable端点是首选状态转换API。

P6 的模型响应字段：

- `id`、`tenantId`、`providerId`、`providerName`、`providerType`、`modelName`、`displayName`、能力字段、 `pricing`、`status`、`createdAt`、`updatedAt`。 `providerType` 允许生成参数使用相同的 Provider 特定标签作为模型配置，而无需根据旧功能值进行猜测。

当前P6模型后端实现状态：

- 后端实现模型CRUD、软删除、enable/disable、租户范围查询、同租户Provider检查、RBAC、操作日志、能力验证、定价元数据验证以及前端动态参数渲染的模型响应。
- 当前模型列表过滤器包括状态、启用的速记、ProviderID和generation/edit功能过滤。
- 前端Provider/model管理已实施并合并。模型功能表单管理generate/edit、多引用、`n`、最大输出计数、支持的尺寸、质量、格式、定价元数据和状态。
- 当前前端Provider/model管理在实用的情况下使用简体中文标签，并通过预设复选框而不是仅自由格式的文本输入来公开size/ratio和质量功能。
- 当前P7任务执行使用稳定的`modelId`引用。尽管如此，P18仍然通过拒绝写入路径中未删除模型的重复`(tenant_id, provider_id, model_name)`值来收紧admin/data完整性。
- P10实现Provider删除的链接模型行为：未删除的链接模型阻止Provider删除；管理员必须首先软删除链接模型。
- P14收紧模型写入行为：create/update/enable必须拒绝禁用、删除或跨租户Provider；当引用的 Provider/model 不再启用且不再处于同一租户时，从持久设置加载的任务默认值也必须以失败方式关闭（fail closed）。 P18 添加行锁序列化并拒绝写入路径中重复的同一 Provider未删除`model_name`值。

前端使用启用的模型功能字段来渲染动态参数。 P6仅管理能力； P8 在后端任务创建且 SSE 存在后将这些功能应用到生成工作台。

## 使用和审核 API

- `GET /admin/usage/summary`
- `GET /admin/usage/records`
- `GET /admin/operation-logs`
- `GET /admin/api-call-logs`
- `GET /admin/api-call-logs/:id`
- `GET /admin/diagnostics/summary`

当前P9后端合约：

- 上述所有路由都需要租户管理员访问权限以及匹配的RBAC权限：`usage:read`用于使用端点，`audit:read`用于operation/API呼叫日志。
- 列表端点返回带有`records`、`total`、`pageNum`和`pageSize`的标准信封。
- 共享查询参数：`pageNum`、`pageSize`、`sortBy=createdAt`、`sortOrder=asc|desc`、`createdAtFrom`、`createdAtTo`。
- 使用过滤器：`taskId`、`userId`、`projectId`、`providerId`、`modelId`；摘要接受`dimension=tenant|user|project|provider|model`。
- 操作日志过滤器：`actorUserId`、`action`、`resourceType`、`resourceId`。
- API 调用日志过滤器：`taskId`、`userId`、`projectId`、`providerId`、`modelId`、`status=SUCCESS|FAILURE`、 `requestId`。
- Usage/raw元数据、操作元数据、API调用request/response有效负载和Provider错误在序列化之前递归已脱敏。
- `tenantId` 仅针对已限定调用者租户范围的行显示；跨租户详细信息探测返回 `404`，但不披露存在性。
- 前端实现状态：`P9-FE-ADMIN-OBSERVABILITY-SETTINGS`通过`frontend/src/api/admin.ts`使用这些路由，保持列表读取分页，并通过`usage:read`或`audit:read`控制可见部分。

P17诊断合同：

- `GET /admin/diagnostics/summary` 是一个仅限管理、只读的 JSON 生产诊断端点。它需要租户管理员访问权限加上`audit:read`，除非稍后的任务故意添加更窄的诊断权限。
- 回复必须使用标准信封并且仅包含聚合部分：
  - `tasks`：按active/terminal状态、最近失败、retrying/timeout计数以及仅包含任务ID和已脱敏错误codes/messages的有界最近失败样本进行计数。
  - `queue`：pending/processing/delayed/dead计数和部分级别的可用性状态；它不得公开 Redis 键、队列负载内容、声明 ID 或任务负载主体。
  - `providers`：有界Provider/API-call固定或查询有界时间窗口的聚合计数和失败率；不得返回原始 request/response 元数据。
  - `storage`：`storageQuota.maxBytes`、只读`usedBytes`、资产计数、soft-deleted/purged计数和cleanup/orphan聚合状态，不含bucket/object-key/MinIOURL曝光。
  - `maintenance`：最新已脱敏聚合操作日志摘要，用于存储保留、日志保留和孤立清理（如果可用）。
- 查询参数应限制为安全控件，例如`windowHours`和`limit`；默认值必须是有界的。
- 数据库支持的部分是租户范围内的。 Redis 或存储检查失败应尽可能表示为已脱敏节级 `status=unavailable`，而不是泄漏基础设施详细信息。
- 诊断不得改变状态、排队任务、运行清理、测试 Provider、调用 AI Provider 或 read/decrypt Provider API 键。
- 实施状态：合并于`P17-BE-OBSERVABILITY-METRICS`。端点返回任务 status/recent 失败聚合、Redis 队列深度以及不可用队列检查时的 `reason="queue_unavailable"`、未截断的 Provider/API-call 总计以及有界 Provider 故障、存储 quota/asset 聚合，以及fail-close 已脱敏维护总结。一般不会传递维护元数据字符串：`status`仅是枚举，`completedAt`是RFC3339/RFC3339Nano-only，数字字段必须是JSON数字，并且unknown/array/map/string有效负载被丢弃而不是已脱敏到客户端可见的原始内容中。

P14 usage/cost 报告合同：

- 后端成本估算是确定性的，并使用模型`pricing.unitPrices`加上Provider使用元数据，而不依赖于前端计算的成本。
- 持久化 `usage_records.estimated_cost` 的格式为小数点后 8 位且绝不为负数。缺失、无效、负数或不完整的定价会产生零估计成本，而不是使原本成功的 Provider 任务失败。
- 使用情况摘要包括租户范围的聚合视图，以及用户、项目、Provider和模型，以及`dimension=tenant`。租户视图的`dimensionId`是当前租户ID。
- Usage/cost查询保持租户范围，分页，在相同时间戳下稳定，并且已脱敏。仅在递归编辑后才能返回原始使用情况。
- 汇总成本字符串保留精确的小数值，并且不会通过浮点转换进行舍入。多币种结果按维度和币种分组。
- 当前P14实施状态：后端usage/cost报告、前端成本可观察性和R14已合并和审查。前端管理使用选项卡使用此合同来获取租户总数、tenant/user/project/Provider/model摘要、过滤器、深入分析、多货币显示和使用记录，而无需客户端权威成本重新计算。

当前设置合同：

- `GET /admin/system-settings`
- `PATCH /admin/system-settings`

活动的运行时支持的设置切片故意变窄：

```json
{
  "uploadPolicy": {
    "maxFileSizeBytes": 26214400,
    "maxWidth": 8192,
    "maxHeight": 8192,
    "maxPixels": 40000000
  },
  "taskDefaults": {
    "defaultProviderId": "provider_123",
    "defaultModelId": "model_123"
  },
  "taskConcurrency": {
    "tenantLimit": 2,
    "userLimit": 2,
    "providerLimit": 2,
    "modelLimit": 2
  },
  "storageRetention": {
    "deletedAssetRetentionDays": null
  },
  "storageQuota": {
    "maxBytes": null,
    "usedBytes": 0
  },
  "logRetention": {
    "operationLogRetentionDays": null,
    "apiCallLogRetentionDays": null,
    "taskEventRetentionDays": null
  }
}
```

- 两条路线都需要租户管理员访问权限加上`system:settings:manage`。
- `GET /admin/system-settings` 当租户还没有覆盖行时，使用环境配置的上传限制返回有效的租户上传策略。
- `PATCH /admin/system-settings`可能会更新`uploadPolicy`下的一个或多个字段；省略的字段保留其当前有效值。
- 租户覆盖必须保持积极，并且只能缩小或匹配环境配置的上传硬上限。运行时资产验证仍然是安全边界，并使用有效的租户策略来进行请求正文大小、维度和像素计数检查。
- 允许的MIME类型保留配置拥有的安全策略；设置 API 不得使 SVG 或任何非允许类型可写。
- P13添加了`taskDefaults`，因为任务创建是运行时消费者：
  - `GET /admin/system-settings`返回`taskDefaults`，其中`defaultProviderId`和`defaultModelId`可为空。
  - 仅当两个 ID 一起提供时，`PATCH /admin/system-settings` 才可以更新`taskDefaults`，或者使用 `null` 值清除两者。
  - 后端必须在保存默认值之前验证租户所有权、启用Provider、启用模型以及模型所有权Provider。
  - 创建任务时可能会省略`providerId`和`modelId`；在这种情况下，后端会解析租户默认值，然后运行与显式请求相同的 Provider/model/capability 验证。
  - 仅省略 `providerId` 或 `modelId` 之一的任务创建仍然无效，以避免模糊的混合默认请求。
  - 如果默认值不存在、过时、已禁用、已删除、跨租户或功能不兼容，则默认支持的任务创建将返回验证失败，并且不得将任务排队或写入成功的任务操作日志。
  - 格式错误的存储 `task_defaults` 值（包括无效的 JSON、未知字段、空白 ID 或仅填充了一个 ID）是无效的服务器端配置。默认支持的任务请求必须以 `422 VALIDATION_ERROR` 的形式失败关闭，而不创建任务、事件、入队操作或成功操作日志。
  - 提供有效 `providerId` 和 `modelId` 的任务请求不得依赖或因未使用的格式错误的 `task_defaults` 行而失败。
- P13添加了`taskConcurrency`及其Worker运行时消费者：
  - 后端切片合并后，`GET /admin/system-settings`返回有效`tenantLimit`、`userLimit`、`providerLimit`和`modelLimit`。
- `PATCH /admin/system-settings`可能会更新`taskConcurrency`下的正整数字段；省略的字段保留当前有效值。
  - 值只能缩小或匹配环境配置的tenant/user/Provider/model硬上限。全局并发不是租户可见或租户可写的字段。
  - Worker在获取新的Redis信号量租约时应用有效值。正的 Provider `concurrencyLimit` 仍然是一个额外的更严格的 Provider 上限。
  - 格式错误的持久化`task_concurrency`配置会导致受影响的合格执行在Provider调用或output/usage/API-call持久化之前发生脱敏任务配置已失败；实际设置存储失败重试而不绕过限制器。
- P13存储清理基础合并为后端内部清理功能：
  - 上传回滚清理不再依赖于对象写入后取消的请求上下文。
  - 软删除的图像资产具有内部清理服务，包括租户、截止、批量、未发现幂等性、存储错误重试和持久的`purged_at`跟踪。
  - 它不会公开公共清理API。
- P13仅与其Worker维护运行时消费者一起添加了`storageRetention`：
  - `GET /admin/system-settings`返回`storageRetention.deletedAssetRetentionDays`。
  - 该值可以为空。 `null` 表示对租户禁用软删除资产的自动物理清理。
  - 无租户覆盖默认为`null`；后端不得意外启用物理删除。
  - `PATCH /admin/system-settings`可以设置正整数天数或使用`null`清除它；省略的字段保留当前值。
  - 有效范围为`1..3650`天，除非后来的公共合同故意更改范围。
  - Worker维护解析租户设置，计算`cutoff = now - deletedAssetRetentionDays`，并调用该租户的资产清理基础。
  - 格式错误的持续存在`storage_retention`必须以失败方式关闭（fail closed）：Worker跳过该租户的清理并仅记录已脱敏元数据。 API reads/writes 在现有设置错误形状下必须返回已脱敏错误。
- P13添加了`storageQuota`以及后端配额消费者：
  - `GET /admin/system-settings`返回`storageQuota.maxBytes`和只读`storageQuota.usedBytes`。
  - `maxBytes` 可为空。 `null` 表示不强制执行租户存储配额。
- `usedBytes` 是根据租户范围的 `image_assets.size_bytes` 计算得出，其中 MinIO 对象仍预计存在。软删除但未清除的行数；已清除的行则不会。
  - `PATCH /admin/system-settings`可以设置正整数`maxBytes`或用`null`清除； `usedBytes` 永远不可写。
  - 参考上传和Worker输出资产持久性必须拒绝超出配额的写入，并且不得在responses/logs.中留下成功的资产元数据、成功的任务输出事件或敏感对象标识符
  - 格式错误的持续存在`storage_quota`对于新资产写入必须以失败方式关闭（fail closed）。不得因配额设置而删除或隐藏现有资产。
  - P17配额预留使此公共API形状保持稳定，同时添加服务器端reservation/counter行为。内部预留 ID、计数器行、锁定密钥和对帐详细信息不得返回到前端。
  - `usedBytes`反映来自MySQL`image_assets`元数据的租户范围配额计数器initialized/reconciled。 MinIO 桶清单并不是配额真理。
- P16 合约 `logRetention` 仅与 Worker 运行时消费者一起使用：
  - `GET /admin/system-settings`返回可为空的`operationLogRetentionDays`、`apiCallLogRetentionDays`和`taskEventRetentionDays`。
  - `PATCH /admin/system-settings`可以将每个字段设置为正整数天数或使用`null`清除它；省略的字段保留其当前值。
  - `null` 表示禁用该日志类别的自动保留清理。
  - 有效范围是`1..3650`天，除非后来的公共合同故意更改它。
  - Worker维护解析活动租户设置，计算每个类别的截止值，仅删除早于截止值的行，限制每个批次，并在清理后写入已脱敏聚合审计元数据。
  - `taskEventRetentionDays` 只能删除早于截止时间的终端任务的事件。它必须保留queued/running/cancelling/retrying任务的事件，以便实时SSE和恢复语义不会被破坏。
  - 该设置仅涵盖现有数据库支持的日志：`operation_logs`、`api_call_logs`和`task_events`。容器stdout/stderr和外部日志聚合保留仍然是部署责任。
  - 格式错误的持久存在`log_retention`必须以失败方式关闭（fail closed）：Worker跳过该租户的清理并仅记录已脱敏元数据。 API reads/writes 在现有设置错误形状下必须返回已脱敏错误。
- `system-settings` 中仍然不存在手动清理触发器。 P17 专用管理存储孤立端点具有真正的运行时行为，并且不得镜像为可写设置。
- 实现状态：后端`GET/PATCH /admin/system-settings`和资源上传运行时消耗合并到`P9-BE-RUNTIME-SETTINGS-CONTRACT`；后端`taskDefaults`write/read、任务创建运行时消耗和畸形行失败关闭强化合并在`P13-BE-RUNTIME-DEFAULTS`和`P13-BE-RUNTIME-DEFAULTS-HARDENING`中；后端`taskConcurrency`read/write和Worker消费合并到`P13-BE-CONCURRENCY-POLICY`；后端存储清理基础合并于`P13-BE-STORAGE-CLEANUP-FOUNDATION`；后端`storageRetention`read/write和Worker维护消耗合并到`P13-BE-STORAGE-RETENTION-RUNTIME`；后端`storageQuota`read/write，计算使用、参考上传强制和Worker输出强制合并在`P13-BE-STORAGE-QUOTA-ACCOUNTING`中；后端严格配额reservation/counter/reconciliation合并到`P17-BE-STORAGE-QUOTA-RESERVATION`；后端 `logRetention` read/write 和 Worker 维护清理合并在 `P16-BE-LOG-RETENTION` 中；后端缩略图生成和授权缩略图流合并在`P16-BE-THUMBNAIL-POLICY`；后端管理存储孤儿scan/cleanup合并到`P17-BE-ORPHAN-CLEANUP`。
- 前端实现状态：`P13-FE-SYSTEM-SETTINGS`公开活动的运行时支持设置：`uploadPolicy`、`taskDefaults`、`taskConcurrency`、`storageRetention`和`storageQuota`； P19 在后端消费者合并和审核后添加了运行时支持的可空 `logRetention` 控件。每次保存都会为每个设置组发送一个受 CSRF 保护的顶级补丁，保持 `storageQuota.usedBytes` 只读，并保持 UI 和请求中不存在未使用的设置。
