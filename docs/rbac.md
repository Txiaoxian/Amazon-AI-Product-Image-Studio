#RBAC计划

## 原则

- RBAC 属于租户范围。
- `tenant_id` 强制隔离。
- RBAC 不取代对象级授权。
- 项目会员资格可以进一步限制项目和资产访问。

## 内置角色

### 管理员

租户管理员。

典型权限：

- 管理用户。
- 管理角色。
- 管理Provider和模型。
- 管理系统设置。
- 查看审核日志。
- 访问所有租户项目。

### 卖家

运营卖家用户。

典型权限：

- 创建和管理自己或分配的项目。
- 上传资产。
- 创建、取消和重试生成任务。
- 下载可见资产。
- 查看可见项目的使用情况。

### 观众

只读用户。

典型权限：

- 查看分配的项目。
- 查看资产和任务历史记录。
- 如果明确允许则下载。
- 没有Provider、模型、用户或系统设置访问权限。

## 权限代码组

用户和角色：

- `user:read`
- `user:create`
- `user:update`
- `user:disable`
- `role:read`
- `role:manage`

P11 用户-管理员角色映射：

- 用户list/detail需要租户管理员访问权限或`user:read`。
- 用户创建需要租户管理员访问权限或`user:create`。
- 创建具有一个或多个`roleIds`的用户还需要租户管理员访问权限或`role:manage`。
- 用户安全字段更新需要租户管理员访问权限或`user:update`。
- 通过PATCH、`/disable`或`/enable`更改用户`status`需要租户管理员访问权限或`user:disable`。
- 角色替换需要租户管理员访问权限或`role:manage`。
- 角色和权限读取需要租户管理员访问权限或`role:read`。
- 用户管理对象 API 必须始终按 `tenant_id` 进行过滤；跨租户用户和角色 ID 不得泄漏存在。
- 后端必须拒绝自我禁用以及任何会删除租户最后一个活动管理员的更新。
- 前端user/role管理面板必须镜像这些边界：没有`user:read`就不要加载`/users`，没有`role:read`就不要加载`/roles`或`/permissions`，不要提交没有`role:manage`的`roleIds`，没有`user:disable`的情况下不要公开状态操作，并在UI中禁用当前用户状态操作。
- 创建的用户密码是暂时的UI输入。它们不得写入 localStorage、sessionStorage、IndexedDB、日志或在成功创建后渲染。

租户和自定义角色操作：

- 创建额外租户是运维人员CLI的责任。租户HTTP API 绝不能从租户管理会话推断出平台范围的超级管理员。
- `GET /tenants/current` 仅读取经过身份验证的租户。
- `PATCH /tenants/current`仅更新经过身份验证的租户名称，并需要租户管理员访问权限加上`system:settings:manage`。
- 保留内置`admin`、`seller`和`viewer`角色。租户HTTP API 不得改变、禁用、删除或替换其授权。
- 自定义角色create/update/delete和权限替换需要租户管理员访问权限或`role:manage`。
- 自定义角色对象API必须按`tenant_id`过滤；跨租户 ID 返回现有的已脱敏未找到形状。
- 当用户仍然引用角色时，自定义角色删除必须失败。补助金替换和成功删除是事务性的且可审计的。

P20状态：

- 额外租户配置仅由运维人员`backend/cmd/provision-tenant`CLI实施。
- 当前租户read/name更新和自定义角色CRUD/permission替换在后端实现。
- 前端tenant/custom-role管理UI已实施。租户名称写入和自定义角色写入保持同源并受到 CSRF 保护；内置角色呈现为只读。

项目：

- `project:read`
- `project:create`
- `project:update`
- `project:delete`
- `project:member:manage`

资产：

- `asset:read`
- `asset:upload`
- `asset:update`
- `asset:delete`
- `asset:download`

任务：

- `task:read`
- `task:create`
- `task:cancel`
- `task:retry`

Provider和型号：

- `provider:read`
- `provider:manage`
- `model:read`
- `model:manage`

审核和设置：

- `usage:read`
- `audit:read`
- `system:settings:manage`

## 对象级授权

每个接收对象 ID 的API都必须验证：

1. 对象存在。
2. 对象属于当前租户。
3. 用户通过角色、项目成员身份或所有权获得权限。

当揭示存在会泄漏跨租户数据时，首选返回`404`。

## 项目成员资格

项目成员可以具有项目级别的角色，例如：

- `OWNER`
- `EDITOR`
- `VIEWER`

项目角色检查应结合租户RBAC。例如，用户需要 `task:create` 和项目编辑器访问权限才能提交该项目中的任务。

项目成员写入需要租户管理员访问权限或相关租户RBAC权限加上项目`OWNER`访问权限。任何成员写入路径都不能离开没有 `OWNER` 的项目：删除或降级最终的 `OWNER` 必须因冲突而失败，并且必须首先通过添加或提升另一个 `OWNER` 来进行所有者转移。

当前 P5 project/asset 角色映射：

- 项目创建需要租户RBAC`project:create`；创建者成为项目`OWNER`。
- 项目读取接受租户管理员访问权限或`project:read`加上项目成员资格。
- 项目update/delete需要租户管理员访问权限或匹配的RBAC权限加上项目`OWNER`。
- 资产read/download接受租户管理员访问权限或`asset:read`/`asset:download`加上项目`OWNER`、`EDITOR`或`VIEWER`。
- 资产upload/update需要租户管理员访问权限或`asset:upload`/`asset:update`加上项目`OWNER`或`EDITOR`。
- 资产删除需要租户管理员访问权限或`asset:delete`加上项目`OWNER`。
- 资产对象 API 首先解析 `asset -> project`，然后应用租户、RBAC 和项目成员资格检查。

P6 Provider/model角色映射：

- Provider list/detail/create/update/delete/enable/disable/test 仅限租户管理员。自定义 `provider:*` 授权不会将普通用户转变为 Provider 管理员。
- 型号create/update/delete/enable/disable仅供租户管理员使用。自定义 `model:manage` 不授予非管理员 CRUD 访问权限。
- 非管理模型list/detail需要模型读取功能加上显式的`(tenant_id, user_id, model_id)`分配。未分配的模型被排除在列表之外，并被未找到的详细响应隐藏。
- `GET /users/{userId}/ai-access` 和 `PUT /users/{userId}/ai-access` 仅限租户管理员，并以事务方式替换模型分配。
- 分配模型隐式分配其拥有的Provider的使用；普通用户不独立管理或直接选择Provider。
- 前端必须对普通用户隐藏Provider/model管理和分配页面。他们唯一的模型交互是在生成参数中选择分配的启用模型。
- Provider/model/grant 对象 API 仍必须按 `tenant_id` 进行过滤；仅靠角色检查是不够的。

P7任务角色映射：

- 任务创建需要`task:create`加上项目`OWNER`或`EDITOR`，除非租户管理员。
- 非管理任务创建者还必须拥有所选模型的明确授权。后端在创建任务状态或将工作排队之前会重新检查这一点。
- 任务list/detail需要`task:read`加上项目可见性，除非租户管理员。
- 任务取消需要`task:cancel`加上项目`OWNER`或`EDITOR`，除非租户管理员。
- 任务重试需要`task:retry`加上项目`OWNER`或`EDITOR`，除非租户管理员。
- 任务事件SSE流需要与任务读取相同的可见性规则。事件过滤必须应用于每个租户和每个project/task对象，而不仅仅是在连接打开时。
- Worker仅使用后端服务权限执行；在读取与任务相关的资产和元数据时，它不得绕过 tenant/project 检查。

P9 audit/settings角色映射：

- 使用摘要和使用记录读取需要租户管理员访问权限加上`usage:read`。
- 操作日志和API-呼叫日志list/detail读取需要租户管理员访问权限加上`audit:read`。
- 新版经营总览和用量费用聚合、对应导出需要租户管理员访问权限加上`usage:read`。
- 用户活跃列表、用户统计详情和用户导出需要租户管理员访问权限加上`user:read`；预计费用、币种和计费用量额外需要`usage:read`，缺少该权限时必须返回脱敏空费用。用户创建、停用和角色修改仍分别使用既有写权限。
- 生图任务统计、模型调用统计、任务/调用导出和调用技术详情需要租户管理员访问权限加上`audit:read`。若调用者没有`usage:read`，响应必须隐藏预计费用、币种和费用状态。
- 中转站健康统计来自模型调用日志，因此查看运行数据需要`audit:read`；配置读写仍分别使用`provider:read|provider:manage`和`model:read|model:manage`。
- 所有统计、详情和导出继续按`tenant_id`限定范围；RBAC 检查不能替代对象与租户隔离。
- 跨租户 log/detail 探针必须不返回任何行或 `404` 且不存在存在泄露。
- 系统设置读取和写入必须需要租户管理员访问权限加上`system:settings:manage`。

## 审核要求

记录操作日志：

- 角色和权限更改。
- 用户创建、禁用和角色分配。
- 用户安全字段更新和启用。
- Provider 和模型更改。
- 项目成员变更。
- 根据政策要求删除和下载资产。
- 根据策略要求，任务创建、取消、重试、失败、超时和终端完成。
