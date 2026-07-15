# 存储计划

## 原则

- MinIO存储所有图像字节。
- MySQL存储元数据和`object_key`。
- MySQL从不存储图像二进制数据。
- 下载需要后端授权。
- SVG禁止上传。

## 桶

推荐桶：

- `product-images`：原始参考、生成和编辑的图像。
- `product-image-thumbnails`：生成的缩略图。

存储桶可以使用前缀或特定于环境的存储桶名称按环境分隔。

P5 应使用现有的 MinIO 环境变量和共享本地 `dev-minio` 服务进行例行验证。不要为普通的 P5 开发创建特定于项目的 MinIO 容器或卷。

当前P5实现使用配置的原始存储桶（默认为`product-originals`）来上传参考图像。存储桶创建不是由请求处理程序执行的；共享本地和部署环境必须在上传测试之前创建或验证所需的存储桶。

## 对象键命名

使用确定性的、不可猜测的对象键：

```text
tenants/{tenantId}/projects/{projectId}/assets/{assetId}/original.{ext}
tenants/{tenantId}/projects/{projectId}/assets/{assetId}/thumbnail.jpg
```

永远不要相信对象密钥的用户文件名。原始文件名在清理后可以存储为元数据。

## 上传验证

需要校验：

- 声明 MIME 类型。
- 文件魔数。
- MIME验证后扩展。
- 文件大小。
- 宽度和高度。
- 像素数。

允许的类型：

- JPEG。
- PNG。
- WebP。

禁止的类型：

- SVG。
- 未知的二进制文件。
- 带有图像扩展名但图像魔法无效的文件。

P5 验证必须在写入任何对象之前进行。如果验证失败，则不应保留任何图像元数据行或 MinIO 对象。如果对象上传后 DB 写入失败，则实现必须删除刚刚上传的对象或记录足够的信息以进行确定性清理。

当前后端行为在对象写入之前进行验证，并在对象上传后使用 P13 清理基础。如果元数据持久化失败，清理将使用独立的有界上下文，因此请求取消无法阻止尽力删除；确定性的retention/orphan清理路径仍然可用于以后的恢复。

当前P5前端行为仅通过后端多部分端点上传参考图像。浏览器不会直接上传到MinIO，前端MIME/size检查只是UX提示；后台验证权威。

## 元数据

`image_assets`商店：

- 租户ID。
- 项目ID。
- 资产种类。
- 类别。
- 对象键。
- 缩略图对象键。
- MIME类型。
- 尺寸。
- 宽度和高度。
- SHA-256。
- 最喜欢的旗帜。
- 源任务ID。
- 创建者。

## 下载

下载必须：

1. 验证用户身份。
2. 检查租户。
3. 检查项目或资产权限。
4. 通过后端流式传输对象或发出短暂的签名URL。

私有租户资产不允许使用公共永久对象 URL。

P5 可以作为默认实现通过后端进行流式传输。仅在身份验证、租户过滤和对象级授权通过后才允许使用短期签名 URL。

当前P5后端行为在`asset -> project`授权后通过后端流式下载。公共永久 MinIO URL 仍然被禁止。

当前的 P5 前端行为通过 `GET /assets/{assetId}/download` 下载并将响应作为浏览器 blob 进行处理。前端代码不得构造 MinIO URL 或将对象键公开为可下载 URL。

P8前端迁移规则：

- 生成和编辑的工作台结果必须从后端任务输出/授权资产下载中读取，而不是从IndexedDB图片二进制数据读取。
- 如果稍后获得批准，现有的浏览器历史记录 blob 可能仅保留用于显式 compatibility/import 流。它们不是平台资产，不得默默提升为 MinIO 支持的租户存储。

## 生成和编辑的输出

P7 Worker 输出处理必须：

1. 从后端Provider适配器接收标准化图像字节。
2. 如果可能，在持久化之前验证MIME、尺寸和像素数。
3. 使用后端生成的对象键将输出对象写入MinIO。
4. 使用 `kind=GENERATED` 或 `kind=EDITED`、`task_id`、`project_id` 和 `tenant_id` 创建 `image_assets` 行。
5. 使用 `task_id`、`asset_id` 和稳定的 `output_index` 创建 `task_outputs` 行。
6. 提交元数据后写入`IMAGE_OUTPUT`任务事件。

Worker 重试和重复队列传递不得为同一 task/output 索引创建重复的输出资产。

## 缩略图生成

参考上传和 generated/edited Worker 输出创建缩略图作为新资产后端持久路径的一部分。

P16缩略图政策状态：

- 后端现在从已验证的图像字节生成有界的 JPEG 缩略图。
- 将缩略图字节存储在配置的缩略图存储桶中，默认为`product-thumbnails`。
- 仅将`thumbnail_object_key`存储在MySQL中。不要将缩略图块存储在MySQL中。
- 仅通过`GET /api/v1/assets/{assetId}/thumbnail`公开缩略图访问，它使用与资产读取相同的经过身份验证的tenant/object授权信封。
- 对于没有`thumbnail_object_key`的资产，将`thumbnailUrl`留空；不要公开 MinIO 存储桶名称、对象键、永久公共 URL 或浏览器生成的缩略图数据。
- 新的参考上传和新的Worker输出要么一致地保留原始对象、缩略图对象、元数据、任务输出和任务事件，要么回滚上传的对象。资产元数据不得指向丢失的缩略图。
- 不回填没有缩略图的现有资源。回填和孤儿发现仍然是存储治理工作。
- 缩略图字节是有意限制此阶段的操作开销。配额执行仍使用现有元数据最终事实来源，直到稍后的 schema/counter 任务显式添加缩略图字节记帐。

P17孤儿清理状态：

- 默认情况下，孤立清理由后端拥有且保守。
- 试运行和执行使用已识别的后端对象键模式加上MySQL元数据（而不是单独的存储桶列表）来识别候选者。
- 清理是租户范围内的、批次限制的、重试安全的、可审计的且已脱敏。
- 响应和日志不得公开完整的存储桶名称、对象键、MinIO URL、签名 URL、图像 base64、授权、Cookie、JWT 或 Provider 秘密。

## 删除

MySQL中默认删除是软删除。为租户启用保留规则后，必须通过受控后端清理路径来处理物理 MinIO 删除。

当前后端行为软删除资产元数据以进行正常删除。 `P13-BE-STORAGE-CLEANUP-FOUNDATION` 添加了内部清理基础，用于物理删除已软删除的资产对象，`P13-BE-STORAGE-RETENTION-RUNTIME` 添加了 Worker 当租户明确启用保留时的维护计划。

P13存储清理基础状态：

- 对象写入后，上传回滚清理不再依赖于HTTP请求上下文。如果对象上传后元数据持久化失败，后端会尝试使用独立的有界上下文进行对象清理。
- 物理清理是租户范围和元数据驱动的。清理代码绝不能接受来自浏览器或其他不受信任的调用者的对象密钥作为删除的最终事实来源。
- 清理可能仅删除已软删除且早于调用者提供的截止时间的资产。
- 清理是批量有限且幂等的。丢失 MinIO 对象算作成功清理；非未找到存储错误使资产有资格重试。
- `image_assets.purged_at` 记录物理清理完成情况，因此重复的清理运行不会重复删除已清除的对象。
- 不要在此基础任务中硬删除图像资产行。元数据对于audit/history和未来的会计仍然有用。

P13 保留运行时规则：

- `storageRetention.deletedAssetRetentionDays` 可为空。 `null` 禁用该租户的自动物理清理。
- Worker维护循环是`storageRetention`的运行时使用者。它根据配置的天数计算租户截止时间并调用清理基础。
- Worker 必须跳过保留设置不存在、为空、格式错误或不受支持的租户。它不得删除这些租户的任何内容。
- 存储配额accounting/enforcement在后端处于活动状态。前端只能通过系统设置API显示或编辑配额，并且仅适用于具有`system:settings:manage`的租户管理员。
- 实现了数据库日志保留、保守的孤立清理和前端运行时支持的设置。原始MinIO列表和不受限制的手动清理触发器故意不公开。

P13/P17存储配额规则：

- `storageQuota.maxBytes` 可为空。 `null` 表示不强制执行租户存储配额。
- `storageQuota.usedBytes` 是只读的，反映初始化或与 `image_assets` 元数据协调后的租户范围的配额计数器。软删除但尚未清除的资产仍然有效；带有 `purged_at IS NOT NULL` 的行不算数。
- 配额强制执行必须在创建新的参考上传资产之前以及在 Worker 持续存在 generated/edited 输出资产之前运行。如果没有成功的资产元数据、成功的任务输出事件或泄漏的对象密钥，则超出配额必须失败。
- 配额核算不得使用MinIO存储桶列表作为其最终事实来源。 MySQL 元数据仍然是权威的协调来源。
- P17严格reservation/counter行为对于参考上传和Workergenerated/edited输出处于活动状态，因此并发编写者无法独立通过乐观元数据总和检查并超过`storageQuota.maxBytes`。

P17存储配额预留结果：

- 租户范围 `storage_quota_counters` 和 `storage_quota_reservations` 存在严格的 reservation/finalization/release.
- 参考上传和Workergenerated/edited输出在MinIO写入之前保留原始图像字节。
- 成功的元数据事务通过 `image_assets` / `task_outputs` 创建完成预留，因此成功的行和配额计数器在正常操作下保持对齐。
- 验证、存储、DB交易、重复输出、取消、超时和清理失败路径释放或避免保留，并且不会留下成功的asset/task-output副作用。
- 协调从MySQL元数据重建配额计数器，因为MySQL仍然是预期对象所有权的权威来源。 MinIO 上市不能成为配额真理。
- 软删除不会减少已用字节。仅当对象不再存在时，保留清理后的物理清除才会减少或协调已使用的字节。
- `storageQuota.usedBytes`反映了initialization/reconciliation之后的权威计数器，并且在API/UI.中保持只读状态
- 已发布、过时、格式错误或不匹配的预留在未创建成功的资产元数据的情况下以失败方式关闭（fail closed）。
- 响应、日志、审计元数据和任务事件不得暴露内部预留 ID、对象键、存储桶名称、MinIO URL、图像 base64、授权、Cookie、JWT 或 Provider 秘密。
