# 数据库架构规划

## 原则

- MySQL 8 是最终的最终事实来源。
- 每张商务桌都包含`tenant_id`。
- 租户范围的查询必须包含`tenant_id`。
- ID 应该是不可枚举的字符串或 UUID 样式值。
- MySQL存储图像元数据和MinIO`object_key`，而不是图像字节。
- 软删除是用户可见业务数据的默认设置。
- 影响任务、输出、使用和事件的状态转换必须是事务性的。
- 启动迁移必须在带有数据库咨询锁的MySQL路径上序列化。如果迁移锁 acquisition/release 失败或应用的迁移所需的表、列或索引丢失，API 和 Worker 启动必须失败关闭。

## 迁移运行时策略

- MySQL启动迁移使用命名咨询锁，因此并发API/Worker进程无法同时应用DDL。
- SQLite/unit-test路径绕过MySQL咨询锁，但仍然在适用的情况下执行相同的幂等迁移运行程序。
- 增量DDL必须是重试安全的：已存在的columns/indexes将被跳过，并且在所有语句和模式检查成功之前不会将失败的迁移记录为已应用。
- 每次迁移后，架构检查都会验证所需的表、列和索引。已应用的迁移中丢失的对象是启动阻止完整性故障。
- 迁移错误不得暴露数据库 URL、密码、授权标头、Cookie、JWT、Provider 密钥或包含机密的原始 SQL 错误负载。

## 核心表

### 租户

存储tenant/team元数据。

关键字段：`id`、`name`、`status`、`created_at`、`updated_at`。

额外的租户配置是仅限运维人员的事务工作流程：创建租户根、协调内置 roles/grants、创建初始租户管理员，并自动分配内置 `admin` 角色。失败的配置尝试不得留下部分租户。

### 用户

存储用户帐户。

关键字段: `id`、`tenant_id`、`email`、`display_name`、`password_hash`、`status`、`last_login_at`、 `created_at`，`updated_at`。

### 角色和权限

数据表：

- `roles`
- `permissions`
- `user_roles`
- `role_permissions`

所有角色分配表都包含`tenant_id`。系统权限可以全局播种，但分配仍保留在租户范围内。

内置角色代码`admin`、`seller`和`viewer`由后端启动保留和协调。租户HTTP API 只能管理自定义角色；他们不得改变内置角色定义或授权。

### 项目

存储亚马逊产品项目。

关键字段: `id`、`tenant_id`、`name`、`brand`、`asin`、`site`、`notes`、 `status`、`sort_order`、`created_by`、`created_at`、`updated_at`、`deleted_at`。

P5实施说明：

- `status`值：`ACTIVE`、`ARCHIVED`。
- `sort_order` 是用于卖家工作区项目选项卡排序的整数。当客户忽略时，新项目可能会收到下一个租户范围的订单值。
- 需要`name`。
- `deleted_at`实现软删除。
- 项目list/detail查询必须始终包含`tenant_id`并默认排除软删除行。
- 建议索引：`(tenant_id, status, sort_order)`、`(tenant_id, status, created_at)`、`(tenant_id, asin)`、`(tenant_id, deleted_at)`。

### project_members

存储项目特定的成员资格。

关键字段：`id`、`tenant_id`、`project_id`、`user_id`、`role`、`created_at`、`updated_at`。

P5实施说明：

- `role`值：`OWNER`、`EDITOR`、`VIEWER`。
- `(tenant_id, project_id, user_id)` 必须是唯一的。
- 外键必须包括现有架构样式支持的租户范围。
- 项目创建者应成为`OWNER`交易会员。

### image_assets

存储图像元数据。

关键字段: `id`、`tenant_id`、`project_id`、`kind`、`category`、`object_key`、`thumbnail_object_key`、 `mime_type`、`size_bytes`、`width`、`height`、`sha256`、`is_favorite`、`source_task_id`、 `created_by`、`created_at`、`updated_at`、`deleted_at`、`purged_at`。

`kind`值：`REFERENCE`、`GENERATED`、`EDITED`。

P5实施说明：

- P5上传仅创建`REFERENCE`资产。
- `object_key`必须是唯一且不可猜测的。
- `sha256` 应编制索引以供将来的重复数据删除检查，但不得绕过 tenant/project 授权。
- `deleted_at`实现软删除。软删除的资产在正常列表和下载中是隐藏的。
- 建议索引：`(tenant_id, project_id, created_at)`、`(tenant_id, project_id, kind)`、`(tenant_id, is_favorite)`、`(tenant_id, deleted_at)`。

P13存储清理基础状态：

- `P13-BE-STORAGE-CLEANUP-FOUNDATION`添加了可为空的物理清除标记`purged_at`和索引`(tenant_id, deleted_at, purged_at)`。
- 物理清理查询必须包括`tenant_id`、`deleted_at IS NOT NULL`、`deleted_at < cutoff`和`purged_at IS NULL`。
- `purged_at` 仅记录对象清理完成情况。它不得替换 `deleted_at`，并且清理不得硬删除 `image_assets` 行。
- `thumbnail_object_key` 对于`P16-BE-THUMBNAIL-POLICY`之后后端生成的MinIO缩略图对象有效。当存储有界的 JPEG 缩略图时，新的参考上传和 Worker generated/edited 输出会保留此键。 MySQL 仍然只存储 metadata/object 键，而不是图像字节。

### prompt_templates

存储tenant/project提示模板。

关键字段: `id`、`tenant_id`、`project_id`、`title`、`prompt`、`created_by`、`created_at`、 `updated_at`，`deleted_at`。

### ai_providers

存储 Provider 配置。

关键字段: `id`、`tenant_id`、`type`、`name`、`base_url`、`encrypted_api_key`、`api_key_hint`、 `status`、`timeout_seconds`、`concurrency_limit`、`created_at`、`updated_at`。

`type`值：`OPENAI`、`GEMINI`、`OPENAI_COMPATIBLE`。

P6实施说明：

- `tenant_id` 是强制性的，所有 Provider 查询都必须通过它进行过滤。
- `status`值：`ENABLED`、`DISABLED`。
- `encrypted_api_key` 仅存储版本化的加密负载；明文 API 密钥绝不能被存储。
- `api_key_hint` 存储非敏感显示元数据，例如最后 4 个字符，并且不足以重建密钥。
- 建议的附加字段：`api_key_updated_at`、`last_test_status`、`last_tested_at`、`last_test_error`、`created_by`、`deleted_at`。
- 首选软删除，以便审核历史记录和Provider/model参考文献仍然可以解释。
- 建议索引：`(tenant_id, type)`、`(tenant_id, status)`、`(tenant_id, deleted_at)`、唯一`(tenant_id, name, deleted_at)`（如果所选 MySQL 策略支持）。
- Provider `base_url` 必须在 insert/update 和任何 test/use 路径之前进行 SSRF 验证。

### ai_models

存储模型能力元数据。

关键字段：`id`、`tenant_id`、`provider_id`、`model_name`、`display_name`、`supports_generate`、`supports_edit`、 `supports_multi_reference`、`supports_n`、`max_output_count`、`supported_sizes_json`、`supported_qualities_json`、`supported_output_formats_json`、`pricing_json`、 `status`，`created_at`，`updated_at`。

P6实施说明：

- `tenant_id` 是强制性的，所有模型查询都必须通过它进行过滤。
- `provider_id` 必须引用同一租户中的 `ai_providers` 行。
- `status`值：`ENABLED`、`DISABLED`。
- `supported_sizes_json`、`supported_qualities_json`、`supported_output_formats_json`和`pricing_json`必须经过验证结构化JSON，而不是任意无界的 blob。
- `max_output_count`必须与`supports_n`一致。
- 实施了附加字段：`created_by`、`deleted_at`。
- 已实施的索引包括`(tenant_id, provider_id)`、`(tenant_id, status)`、`(tenant_id, provider_id, model_name)`、`(tenant_id, supports_generate)`、`(tenant_id, supports_edit)`、`(tenant_id, deleted_at)`和`created_by`。
- 当前的实现不会强制`(tenant_id, provider_id, model_name)`的唯一性； R7 确认当前任务执行使用 `modelId`，因此运行时不需要该不变式。稍后management/data-integrity的决定可能仍会收紧。
- 当前的实现使模型行保持独立的软删除。 P10已解决Provider删除行为：删除被阻止，而任何未删除的同租户模型仍然引用Provider；软删除模型不会阻止删除。 P14收紧Provider禁用和模型写入行为：Provider禁用在启用的链接模型存在时被阻止，禁用的链接模型可能保留，并且模型create/update/enable拒绝禁用、删除或跨租户Provider。
- 生成和编辑的任务执行不得从P6开始； models 是P7/P8.的配置数据

### user_model_access_grants

存储管理员为非管理员用户分配的模型访问权限。

关键字段：`id`、`tenant_id`、`user_id`、`model_id`、可为 null `granted_by`、`created_at`、`updated_at`。

规则：

- `(tenant_id, user_id, model_id)`是独一无二的。
- 用户、模型和授予参与者外键包括`tenant_id`；跨租户赠款在结构上被拒绝。
- 租户管理员在管理自己的租户时绕过此关系。非管理模型发现和任务创建需要匹配的资助。
- 替换一个用户的授权是事务性的。无效、已删除、丢失或跨租户模型 ID 不得部分替换之前的集合。
- 引入的迁移使用现有的未删除的同租户模型回填现有用户，以保留已部署的行为。迁移后不会自动授予新创建的用户和模型。
- 模型软删除不会硬删除历史授权行；正常读取仅连接未删除的模型。

### generation_tasks

存储持久的任务状态。

关键字段: `id`、`tenant_id`、`project_id`、`provider_id`、`model_id`、`status`、`prompt`、 `image_type`、`params_json`、`input_asset_ids_json`、`attempt`、`max_attempts`、`queued_at`、`started_at`、 `finished_at`、`timeout_at`、`created_by`、`error_code`、`error_message`、`created_at`、`updated_at`。

P7实施说明：

- `tenant_id` 是强制性的，所有任务查询都必须通过它进行过滤。
- 规范状态值：`QUEUED`、`RUNNING`、`SUCCEEDED`、`FAILED`、`CANCELLED`、`RETRYING`、`TIMED_OUT`。
- `TASK_COMPLETED` 是 SSE 事件类型；任务行使用 `SUCCEEDED` 来表示成功的终端状态。
- 任务行应仅存储来自后端逻辑的服务拥有的字段；客户不得提供 `tenant_id`、`status`、`attempt`、时间戳或 `created_by`。
- 建议索引：`(tenant_id, project_id, created_at)`、`(tenant_id, status)`、`(tenant_id, created_by, created_at)`、`(tenant_id, provider_id, status)`、`(tenant_id, model_id, status)`和`(tenant_id, timeout_at)`。

### task_events

存储SSE-可重播的任务事件。

关键字段: `sequence`、`id`、`tenant_id`、`task_id`、`project_id`、`event_type`、`event_payload_json`、 `created_at`。

`sequence` 是 MySQL `BIGINT UNSIGNED AUTO_INCREMENT` 主键，并且是持久重播光标。 `id`是一个独特的SSE安全事件ID，源自`sequence`，格式类似于`evt_00000000000000000001`。

P7实施说明：

- `task_events`是SSE的重播源。 Redis pub/sub 或进程内扇出可能只会加速实时交付。
- 历史回放必须按`sequence`进行比较；请勿使用 `created_at` 或随机 ID 后缀进行重播排序。
- 有效负载JSON必须是有界的、结构化的、camelCase和已脱敏的。
- 实施的索引：`(tenant_id, task_id, sequence)`、`(tenant_id, project_id, sequence)`和`(tenant_id, sequence)`。

### task_outputs

将任务输出映射到图像资产。

关键字段：`id`、`tenant_id`、`task_id`、`asset_id`、`output_index`、`created_at`。

### api_call_logs

记录AIProvider调用。

关键字段: `id`、`tenant_id`、`task_id`、`provider_id`、`model_id`、`status`、`duration_ms`、 `request_id`、`http_status`、`error_code`、`error_message`、`redacted_request_json`、`redacted_response_json`、`created_at`。

不要存储 API 密钥、授权标头、Cookie 或图像 base64。

### usage_records

存储使用情况和估计成本。

关键字段: `id`、`tenant_id`、`task_id`、`user_id`、`project_id`、`provider_id`、`model_id`、 `input_tokens`、`output_tokens`、`image_count`、`estimated_cost`、`currency`、`cost_status`、`pricing_snapshot_json`、`raw_usage_json`、`created_at`。

P14 usage/cost 报告保持 `estimated_cost` 确定性、非负数，并格式化为小数点后 8 位。缺失、无效、负数或不完整的模型定价会产生零估计成本，而不是无法成功完成任务。使用记录仍然可以通过租户、用户、项目、Provider、模型和租户范围的聚合视图进行查询。摘要成本输出保留精确的十进制字符串并按货币分组。

管理统计基础迁移 `202608110001_admin_analytics_foundation` 增加：

- `cost_status`：`CALCULATED` 表示费用计算成功，`UNAVAILABLE` 表示缺少有效定价或用量，`LEGACY_UNKNOWN` 表示历史记录无法可靠判断。迁移不使用当前价格回填历史费用。
- `pricing_snapshot_json`：任务产生用量时保存经过模型配置校验的定价快照；定价缺失或无效时保存空对象，不影响成功任务和出图落库。
- 时间聚合索引：任务、任务输出、模型调用和用量记录均增加以 `tenant_id` 和 `created_at` 为前导的索引；任务/调用/用量另有中转站或模型维度的时间索引，调用日志另有状态时间索引。

所有管理统计查询仍必须显式携带 `tenant_id`。任务成功率使用终态任务，实际出图张数使用 `task_outputs` 持久化行，费用只汇总 `usage_records.estimated_cost`，不得用当前模型定价回算历史记录。

### operation_logs

存储审核事件。

关键字段: `id`、`tenant_id`、`actor_user_id`、`action`、`resource_type`、`resource_id`、`ip`、 `user_agent`，`metadata_json`，`created_at`。

### system_settings

存储租户范围的设置。

关键字段：`id`、`tenant_id`、`key`、`value_json`、`created_at`、`updated_at`。

实施注意事项：

- `(tenant_id, key)` 必须是唯一的。
- 第一个活动密钥是`upload_policy`。
- `upload_policy.value_json` 是一个有界对象，具有 `maxFileSizeBytes`、`maxWidth`、`maxHeight` 和 `maxPixels`。
- 存储的上传策略值仅供租户覆盖；当不存在替代时，有效运行时值将回落到环境配置的上传限制。
- 租户上传策略覆盖可能仅缩小或匹配环境配置的硬上限，并在文件持久性之前由后端资产上传验证使用。
- P13添加了活动键`task_defaults`，因为运行时消费者是任务创建。 `task_defaults.value_json`为租户提供`defaultProviderId`和`defaultModelId`商店。
- `task_defaults` 行必须属于租户范围。任务创建必须重新验证每个默认支持的任务创建时引用的 Provider/model，包括租户所有权、启用状态、Provider 的模型所有权以及模型功能支持。
- 对于默认支持的任务创建，格式错误、部分或以其他方式无效的存储 `task_defaults` 值必须以失败方式关闭（fail closed）；他们不得创建任务、事件、排队工作或成功操作日志。显式 Provider/model 任务请求不得要求读取未使用的默认值。
- `task_concurrency`已添加到`P13-BE-CONCURRENCY-POLICY`及其Worker运行时消费者。其有界 JSON 字段为 `tenantLimit`、`userLimit`、`providerLimit` 和 `modelLimit`。
- `task_concurrency` 值是正租户覆盖，可能会缩小或匹配环境配置的 tenant/user/Provider/model 并发硬上限。它不得存储或公开租户控制的全局限制。
- `storage_retention`已添加到`P13-BE-STORAGE-RETENTION-RUNTIME`及其Worker维护消费者。它的有界 JSON 字段可为空 `deletedAssetRetentionDays`。
- `storage_retention.deletedAssetRetentionDays = null` 表示租户禁用自动物理清理。正整数意味着 Worker 计算 `cutoff = now - days` 并调用租户范围的资产清理基础。
- 格式错误或不受支持的`storage_retention`值必须以失败方式关闭（fail closed）以进行Worker清理：跳过该租户的删除并仅记录已脱敏元数据。
- 下一个合约密钥是`storage_quota`。它可能仅保留在`P13-BE-STORAGE-QUOTA-ACCOUNTING`中，它还必须连接参考上传和Worker输出配额消费者。它的有界 JSON 字段可为空 `maxBytes`。
- `storage_quota.maxBytes = null` 表示对租户禁用存储配额强制执行。正整数意味着在成功创建资产元数据之前，新的参考上传和generated/edited输出资产必须符合租户配额。
- `P17-BE-STORAGE-QUOTA-RESERVATION`添加了租户范围`storage_quota_counters`和`storage_quota_reservations`。它们包括`tenant_id`，保留MySQL`image_assets`元数据作为协调最终事实来源，并支持原子reservation/finalization/release用于并发参考上传和Worker输出。
- `storageQuota.usedBytes`是计算的API字段，不存储在`system_settings`中。它必须从租户范围 `image_assets.size_bytes` 计算，其中 `purged_at IS NULL`；软删除但尚未清除的资产仍然有效，因为它们的 MinIO 对象仍然存在。
- 对于新资产写入，格式错误或不受支持的`storage_quota`值必须以失败方式关闭（fail closed）：以已脱敏错误拒绝写入，并且不会创建成功的asset/task-output副作用。
- `log_retention`在`P16-BE-LOG-RETENTION`之后处于活动状态，因为它有一个Worker运行时消费者。其有界 JSON 字段可为 null `operationLogRetentionDays`、`apiCallLogRetentionDays` 和 `taskEventRetentionDays`。
- `log_retention.* = null` 表示禁用数据库支持的日志类别的自动清理。正整数意味着 Worker 计算 `cutoff = now - days` 并删除在有界批次中早于截止时间的租户范围内的行。
- `taskEventRetentionDays`清理只能删除早于截止时间的终端任务的事件。它不得删除排队、正在运行、取消或可重试任务的事件，因为`task_events`是SSE重播源。
- 日志保留仅适用于现有数据库支持的日志：`operation_logs`、`api_call_logs`和`task_events`。它不管理容器stdout/stderr、外部日志聚合或MinIO对象清理。
- 格式错误或不受支持的`log_retention`值必须以失败方式关闭（fail closed）以进行Worker清理：跳过该租户的删除并仅记录已脱敏元数据。
- 实施状态：`P9-BE-RUNTIME-SETTINGS-CONTRACT`合并了`system_settings`model/migration和`upload_policy`运行时路径。 `P13-BE-RUNTIME-DEFAULTS` 和 `P13-BE-RUNTIME-DEFAULTS-HARDENING` 合并了 `task_defaults` 路径和格式错误的行失败关闭行为。 `P13-BE-CONCURRENCY-POLICY`合并了`task_concurrency`read/write和Worker消耗。 `P13-BE-STORAGE-RETENTION-RUNTIME`合并了`storage_retention`read/write和Worker维护消耗。 `P13-BE-STORAGE-QUOTA-ACCOUNTING` 合并了 `storage_quota` read/write、计算使用情况和资产写入消费者。 `P16-BE-LOG-RETENTION`合并了`log_retention`read/write和Worker维护清理。 `P16-BE-THUMBNAIL-POLICY`为新资产激活`image_assets.thumbnail_object_key`，而无需将图片二进制数据添加到MySQL。 `P17-BE-ORPHAN-CLEANUP`添加了管理存储孤儿scan/cleanup，而不添加新表。 `P17-BE-STORAGE-QUOTA-RESERVATION`添加了配额counter/reservation表并​​将其连接到资产上传、Worker输出持久性、物理清除会计和对账。

## 索引期望

- 所有业务表索引`tenant_id`。
- 普通过滤器应使用复合索引，例如`(tenant_id, project_id, created_at)`。
- 任务查询需要`(tenant_id, status)`、`(tenant_id, project_id, created_at)`和`(tenant_id, created_by, created_at)`上的索引。
- 任务事件需要`(tenant_id, task_id, sequence)`、`(tenant_id, project_id, sequence)`和`(tenant_id, sequence)`。
