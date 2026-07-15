# 任务队列计划

## 原则

- 任务创建仅在MySQL持久化和Redis入队成功之前同步。
- MySQL是任务状态的最终来源。
- Redis是队列、锁、并发和速率限制层。
- Worker 执行必须是幂等的。
- SSE事件在交付前保留在MySQL中。
- P7分为基础、SSE、Worker队列、Provider适配器运行时、前端任务客户端和R7审查。不要在一个worktree中实现所有问题。

## 任务生命周期

状态：

- `QUEUED`
- `RUNNING`
- `SUCCEEDED`
- `FAILED`
- `CANCELLED`
- `RETRYING`
- `TIMED_OUT`

预期的转变：

- `QUEUED`至`RUNNING`
- `QUEUED`至`CANCELLED`
- `RUNNING`至`SUCCEEDED`
- `RUNNING`至`FAILED`
- `RUNNING`至`TIMED_OUT`
- `RUNNING`至`CANCELLED`（在Provider完成之前取消）
- `FAILED`至`RETRYING`
- `RETRYING`至`QUEUED`

终端状态：`SUCCEEDED`、`FAILED`、`CANCELLED`、`TIMED_OUT`。

状态命名说明：

- `SUCCEEDED` 是成功完成的规范任务状态。
- SSE事件类型`TASK_COMPLETED`表示过渡到`SUCCEEDED`。
- 在生产使用任务状态之前，必须对齐现有前端过渡状态名称UI。

## 队列设计

使用 Redis 进行持久队列交付和 Worker 协调。该实现可以使用 Redis Streams 或可靠的列表模式，但它必须支持：

- 申请工作。
- Worker崩溃后重新交付。
- 退避重试。
- 死信处理。
- 待处理作业的可见性。

队列有效负载应仅包含任务ID。 Worker从MySQL加载完整任务状态。

P7基础要求：

- `P7-BE-TASK-FOUNDATION`已创建队列抽象，并在MySQL持久化后将任务ID写入Redis。
- 入队失败会用已脱敏`ENQUEUE_FAILED`元数据标记任务`FAILED`，而不是为未入队的任务返回成功。
- `P7-BE-WORKER-QUEUE`实现了可靠的队列声明、可见性超时、确认、延迟重试升级、过时声明恢复和死信处理。

## 并发限制

在这些维度上强制并发：

- 全球。
- 租户。
- 用户。
- Provider。
- 模型。

Redis 信号量或锁可以强制执行活动计数。 MySQL 仍然必须检查状态才能在崩溃后恢复。

P13并发策略合约：

- `TASK_GLOBAL_CONCURRENCY` 仍由部署拥有，租户永远不可写入。
- `TASK_TENANT_CONCURRENCY`、`TASK_USER_CONCURRENCY`、`TASK_PROVIDER_CONCURRENCY`和`TASK_MODEL_CONCURRENCY`仍然是环境硬上限以及租户没有优先权时的后备限制。
- 租户设置切片`taskConcurrency.{tenantLimit,userLimit,providerLimit,modelLimit}`只能缩小或匹配这些硬上限。
- Worker 必须在加载租户范围的任务执行上下文之后和获取 Redis 信号量租约之前解析租户策略。成功获得的新租约使用有效的政策；现有租赁不会追溯改变。
- Provider `concurrencyLimit`，当为正时，仍然是一个额外的更严格的Provider维度上限；有效的Provider限制是环境上限、租户政策和Provider限制的最小值。
- 格式错误的存储并发策略必须在 Provider 执行、输出、使用或 API 调用日志记录之前使符合条件的任务失败，且已脱敏 `TASK_CONFIGURATION_INVALID`。设置 storage/infrastructure 读取失败必须使任务有资格重试，并且不得绕过并发执行。
- 实施状态：`P13-BE-CONCURRENCY-POLICY`已合并。未来队列或Worker更改必须保留此策略解析顺序、失败关闭行为和租用释放语义。

## Worker幂等性

在执行之前，Worker必须：

1. 通过 ID 使用租户上下文加载任务。
2. 检查任务状态和尝试。
3. 如果符合条件，可通过交易方式过渡到`RUNNING`。
4. 写入`TASK_STARTED`事件。

在创建输出资产之前，Worker必须防止同一任务和输出索引出现重复的输出记录。

P7实现边界：

- `P7-BE-WORKER-QUEUE` 通过 fake/stub 执行验证幂等性和状态转换。
- `P7-BE-PROVIDER-ADAPTER-RUNTIME`在Worker状态机稳定后添加真实的Provider调用、MinIO输出、task_outputs、usage_records和api_call_logs。

P7 Worker 队列结果：

- Worker队列执行被合并，并在每次声明和转换之前使用MySQL任务状态作为权限。
- Redis有效负载仅包含任务ID； Worker 从 MySQL 重新加载租户、项目、Provider、模型、提示和任务参数。
- Worker写入的事件发布最少的Redis唤醒，因此APISSE流可以重放持久的MySQL事件。
- 全局、租户、用户、Provider 和模型维度存在并发限制，并具有过时锁清理功能。
- P10 SSE 桥生命周期后续功能已合并：API Redis 订阅生命周期与 API 服务器关闭相关联，而不是与无限的后台上下文相关联。

P7 Provider 运行时结果：

- 真正的Provider执行现在合并在Worker状态机后面。
- 成功运行创建 MinIO 对象、generated/edited 资产、`task_outputs`、`usage_records` 和 `api_call_logs`，然后发出 `IMAGE_OUTPUT`、`USAGE_RECORDED` 和终端事件通过现有的持久事件流。
- Provider运行时使用SSRF-安全的出站传输和持久化之前的递归编辑。审核修复关闭了当前的 API 键值泄漏和当前的 API key-as-map-key 泄漏路径。
- P10Worker池合并后的上一个运行时结转项目已解决：APIRedis订阅生命周期现在绑定到API服务器关闭。

P21 Provider尝试账本结果：

- Provider 运行时现在会在外部 Provider 调用开始之前写入状态为 `ATTEMPTING` 的租户范围`api_call_logs` 行。
- 成功、Provider失败、任务超时和上下文取消使用已脱敏request/response元数据完成同一账本行。
- 尝试预写失败会阻止 Provider 调用，并使任务有资格重试。
- 尝试最终确定失败导致正在运行的任务因 `PROVIDER_ATTEMPT_LEDGER_FINALIZE_FAILED` 关闭而失败，并且不会保留输出资产、使用记录或成功的终端副作用。
- Worker 持久性将 APICall ID 视为幂等账本更新，因此重复交付不会创建重复的 Provider 调用或重复的 API 调用行。

P21 Worker配额对账结果：

- Worker 维护现在使用现有的 MySQL 元数据和配额 counter/reservation 表消耗存储配额协调。
- 活动租户按有界轮换批次进行处理。固定的首页批次不能让后来的租户挨饿。
- 协调是租户范围内的，对于格式错误的settings/counters/reservations会失败关闭，并且仅记录已脱敏聚合元数据。

P21并发续租结果：

- 当 Worker 正在执行 Provider 任务时，Redis 信号量租约会更新。
- 续订发生在任务被声明为`RUNNING`之后且在Provider执行之前，可以比原始租约TTL更长久。
- 如果续订失败或Redis报告不再持有租约，Worker将取消执行并导致任务失败，关闭为`CONCURRENCY_LEASE_LOST`。
- 丢失的租约后面不得有成功的输出资产、使用记录、成功的API调用完成或`SUCCEEDED`任务状态。
- Worker在执行或失败关闭处理后仍然释放租约。

P21 Worker 准备结果：

- Worker 准备文件不再是静态启动标记。
- 仅在数据库、Redis和MinIO依赖项检查通过后，Worker才会写入文件。
- 当`Worker.Run`退出时，Worker会立即删除就绪文件，包括错误和正常关闭路径。
- 依赖项准备就绪错误在依赖项名称级别报告，并且不得记录密码、Redis密钥、对象密钥、存储桶 URL、Provider秘密、授权标头、Cookie、JWT 或图像 base64。

P10 Worker 池结果：

- `P10-BE-WORKER-POOL`已合并。
- `WORKER_CONCURRENCY`控制进程内Worker处理循环的数量。
- Worker 进程并发性与 global/tenant/user/Provider/model 执行限制不同。 Worker循环计数控制一个Worker进程可以并行处理多少个队列声明； Redis 并发限制仍然决定已声明的任务是否可以运行。
- Worker池保留现有队列合约：Redis有效负载仅包含任务ID，MySQL在每次状态转换之前重新加载，每个声明发生队列最终确定，并且重复声明不得重复输出资产、使用记录、API调用日志或终端事件。
- 每个 Worker 进程的恢复仍然是一个循环，因此多个处理循环不会重复 timeout/recovery 工作。
- `P10-BE-SSE-BRIDGE-LIFECYCLE`已合并。 `P10-BE-PROVIDER-MODEL-LIFECYCLE`也被合并； Provider 现在，当存在同租户未删除链接模型时，删除会被阻止。

## 取消

取消请求：

1. 检查租户和对象授权。
2. 标记符合条件的任务已取消或已请求取消。
3. 当到达终端取消时写入`TASK_CANCELLED`事件。

如果 Provider 调用无法被中断，且 MySQL 状态已被终端取消，那么 Worker 必须忽略 Provider 输出。

## 重试

当策略允许时，重试会为符合条件的失败、超时或取消的任务创建新的尝试。重试必须保留原始提示和参数，除非 API 明确接受替换。

## 超时和恢复

任务有`timeout_at`。恢复循环应该：

- 将过期的运行任务标记为`TIMED_OUT`。
- 释放过时的并发锁。
- 当状态和尝试允许时重新排队安全任务。

## SSE 活动

每个有意义的转变都会写入`task_events`：

- 排队。
- 开始了。
- 进步。
- 创建输出。
- 记录使用情况。
- 失败的。
- 完全的。
- 取消了。
- 重试了。
- 超时。

P7 SSE边界：

- `P7-BE-SSE-STREAM`仅消耗持久的`task_events`和实时扇出。
- 重播必须使用 `task_events.sequence` 作为光标，并发出 `task_events.id` 作为 SSE `id`。
- MySQL 仍然是重播源。 Redis pub/sub 或进程内扇出可能会加速实时交付，但不能取代 MySQL 事件持久性。
- 合并的SSE实现使用API进程内代理加上Redis跨进程唤醒。 SSE API 在发送事件之前仍必须从 MySQL 重新加载事件。

P10 SSE桥梁生命周期计划：

- 完成并合并。
- Redis任务事件pub/sub仍然是仅携带事件序列ID的唤醒通道。
- API订阅者随着API服务器生命周期停止，而不是使用无限制的后台上下文。
- 订阅者关闭会关闭 Redis pub/sub 路径，并且 router/API 测试证明订阅者可以停止，而无需将 `context.Canceled` 记录为意外故障。
- SSE重播语义、心跳、`Last-Event-ID`和前端EventSource行为没有改变。

P10 Provider/model生命周期计划：

- Provider 当同一租户中存在任何未删除的链接模型时，必须阻止删除。
- 软删除模型不再阻止Provider删除。
- 跨租户模型不得阻止或泄露其他租户的 Provider 删除。

P14 Provider/model生命周期更新：

- 当启用的同租户链接模型存在时，必须阻止Provider通过`/disable`和`PATCH status=DISABLED`禁用。
- 模型create/update/enable必须拒绝禁用、删除或跨租户Provider。
- 已加载的`taskDefaults`必须在创建任务之前重新验证，因此disabled/deletedProvider或模型引用以失败方式关闭（fail closed）并且不会将任务排队。
- P18 写入路径完整性拒绝重复的同租户 Same-Provider 未删除的 `model_name` 值，而任务执行继续使用稳定的 `modelId` 引用。
