# 后端规则

## 技术栈

后端服务使用 Go + Gin + GORM。

后端预期目录结构：

```text
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
```

当前显式迁移位于 `backend/internal/database/migrations.go`；除非后续任务明确调整迁移策略，否则不得引入第二套迁移来源。

## API 服务

- 在 `/api/v1` 下使用 Gin 路由组。
- 添加请求 ID、日志、panic 恢复、安全响应头、身份认证、租户上下文和 RBAC 中间件。
- 使用 `docs/api-contract.md` 中定义的统一响应和错误结构。
- 在路由边界校验请求体和查询参数。
- 绝不向客户端暴露内部堆栈或第三方原始错误载荷。
- 返回平台前端的人类可读 API 校验和错误信息应尽量使用简体中文。机器可读错误码保持稳定，并使用英文风格的大写标识符。
- 不得将 Provider、数据库或内部运行时原始错误直接传给用户；应返回简体中文摘要，并在日志/审计记录中保留已脱敏的详细上下文。

## 数据库

- 使用 MySQL 8 作为最终事实来源。
- 使用 GORM，并显式定义模型和迁移。
- 每张业务表都必须包含 `tenant_id`。
- 租户范围 Repository 必须要求租户上下文，并包含 `tenant_id` 过滤条件。
- 涉及任务、输出、用量和事件的状态转换必须使用事务。

## Worker

- Worker 从 Redis 处理作业，并将状态转换持久化到 MySQL。
- Worker 必须具备幂等性。队列重复投递不得产生重复输出、用量记录或终态事件。
- 任务状态转换必须遵循文档定义的状态机。
- 取消、重试、超时和恢复操作必须先读取 MySQL 状态再执行。
- Worker、队列、认证、Provider 和其他有状态后端任务在实施前必须定义失败模式或状态转换矩阵，并为范围内的每个分支添加测试。
- 如果有意推迟高风险分支，必须在任务包和最终交付说明中明确指出。

## Redis

Redis 可用于：

- 作业队列。
- 锁。
- 并发信号量。
- 限流。
- 缓存。
- 临时任务投递加速。

不得将 Redis 视为任务状态的最终事实来源。
