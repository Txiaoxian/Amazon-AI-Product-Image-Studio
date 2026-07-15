#SSE合约

## 端点

```text
GET /api/v1/events/tasks
```

查询参数：

- `projectId`：可选的项目过滤器。
- `taskId`：可选任务过滤器。
- `lastEventId`：当`Last-Event-ID`标头不可用时可选的后备。

身份验证使用普通的 HttpOnly Cookie。

P7实施范围：

- `P7-BE-SSE-STREAM`在`task_events`存在后实现此端点。它与MySQL重放和API进程内唤醒合并。
- `P7-BE-WORKER-QUEUE`添加了基于Redis的跨进程Worker到API在Worker保留任务事件后唤醒。 Redis仅进行最小程度的唤醒； MySQL 仍然是重播源。
- `P7-FE-TASK-CLIENT-SSE`与前端SSE客户端类型、`lastEventId`后备处理、心跳处理和减速器实用程序合并。它不会取代主要的生成工作台流程； P8拥有工作台后端。

P10生命周期更新：

- `P10-BE-SSE-BRIDGE-LIFECYCLE`已合并。 API Redis 任务事件订阅者使用 API 生命周期上下文启动，并在 API 关闭时停止。
- Redis 任务事件唤醒仍然仅按顺序进行。它们不携带租户 ID、任务 ID、项目 ID、完整事件负载、授权标头、Cookie、Provider/API 密钥或图像 base64。
- Redis仅保留唤醒路径。在写入帧之前，SSE服务仍然会从MySQL重新加载可见事件。

P21弹性更新：

- 历史重播受到每次流尝试的限制，因此过时的光标无法在实时流开始之前强制执行无限制的响应。
- 心跳处理在写入 `HEARTBEAT` 之前执行MySQL追赶传递，因此即使Redis唤醒延迟或错过，持久事件也可以到达客户端。
- 如果 Redis 通知通道关闭，API SSE 服务会重新订阅，而不是永久丢失跨进程唤醒。
- 长期存在的SSE流定期重新验证经过身份验证的用户的会话版本和活动状态。注销、密码更改、用户禁用或其他会话版本更改会关闭陈旧的流，而不是继续发出事件。

## 浏览器规则

- 前端必须使用 EventSource 或等效的 SSE 客户端。
- 前端不得轮询任务状态。
- 前端不得使用`setInterval`或重复的获取循环来获取任务进度。

## 重新连接并重播

服务器必须支持：

- `Last-Event-ID`标题。
- `lastEventId`查询回退。
- MySQL`task_events`的历史事件重播。
- 保持连接活动的心跳事件。
- 网络中断后安全重新连接。

如果客户端重新连接已知事件 ID，服务器会在流式传输实时事件之前发送此后的所有可见事件 ID。

重放游标合约：

- `task_events.sequence`是持久的单调重放光标。
- `task_events.id`是SSE`id`派生自`sequence`，格式为`evt_`加上零填充的十进制序列。
- `Last-Event-ID` 和 `lastEventId` 必须解析回序列游标。
- 历史回放必须使用`sequence > cursor`查询可见事件，按`sequence ASC`排序。
- 在打开流之前，应拒绝格式错误的事件 ID，并显示已脱敏验证错误。

## 事件框架

示例：

```text
id: evt_000000000001
event: TASK_STARTED
data: {"taskId":"task_...","status":"RUNNING","startedAt":"2026-05-09T07:00:00Z"}
```

`id`是耐用的`task_events.id`；服务器必须使用其底层 `task_events.sequence` 进行重播排序。

## 事件类型

- `TASK_QUEUED`：任务已创建并排队。
- `TASK_STARTED`：Worker开始执行。
- `TASK_PROGRESS`：可选的进度更新。
- `IMAGE_OUTPUT`：已创建输出图像资源。
- `USAGE_RECORDED`：已创建usage/cost记录。
- `TASK_FAILED`：任务失败。
- `TASK_COMPLETED`：任务成功。
- `TASK_CANCELLED`：任务已取消。
- `TASK_RETRIED`：已安排重试。
- `TASK_TIMED_OUT`：任务超时。
- `HEARTBEAT`：连接保持活动状态。

状态映射：

- `TASK_COMPLETED`是一种事件类型，而不是规范的终端任务状态。
- 成功的任务记录使用状态`SUCCEEDED`。
- Failed/cancelled/timed-out任务记录使用`FAILED`、`CANCELLED`和`TIMED_OUT`。

## 负载原理

- 有效负载使用camelCase。
- 有效负载必须包含`taskId`。
- 项目范围内的事件包括`projectId`。
- 资产输出事件包括`assetId`、`thumbnailUrl`或授权预览URL、尺寸、MIME类型和输出索引。
- 错误负载包括已脱敏`errorCode`和`message`。
- 有效负载不得包含 API 密钥、授权标头、Cookie 或图像 base64。

## 持久化规则

对于任务事件：

1. 将事件写入MySQL。
2. 发布或扇出给活跃的 SSE 客户。

MySQL是重放源。 Redis pub/sub 可以加速实时交付，但不能是唯一的事件源。

Worker进程不得通过Redis发送完整的事件有效负载作为最终事实来源。他们应该发布一个事件 ID/sequence 或最小唤醒消息，并且 API SSE 服务必须在写入帧之前从 MySQL 加载可见事件。

## 授权

该流仅发出对经过身份验证的用户可见的事件：

- 租客必须匹配。
- 用户必须具有项目可见性或管理员权限。
- 任务过滤器仍必须通过对象级检查。

P7 测试必须证明：

- 与`Last-Event-ID`重新连接仅重播ID之后的可见事件。
- `lastEventId` 查询回退的行为与标头相同。
- 不发出跨租户和非成员项目事件。
- 心跳帧不会泄漏任务元数据。
