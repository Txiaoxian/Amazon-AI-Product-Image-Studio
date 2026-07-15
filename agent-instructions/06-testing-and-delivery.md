# 测试与交付

## 通用交付规则

- 说明修改了什么、有意未修改什么，以及验证方式。
- Git 提交信息的标题和正文默认使用简体中文；Conventional Commits 的类型前缀（如 `feat`、`fix`、`docs`）可保留英文。
- 将变更限制在请求的阶段范围内。
- 除非任务明确要求组合修改，否则不要混合文档、前端重构、后端实施和部署变更。
- worktree 任务的最终交付说明必须将任务包要求的每个回归场景或失败模式，映射到实际覆盖它的测试文件和测试名称。
- 如果实施无法在允许范围内保留必需的现有行为，应停止并报告冲突，不得暗中扩大范围或交付半迁移状态。

## 验证策略

验证按风险分级。完整测试套件是交付门槛，不是每次小修改后都必须运行的命令。

- 实施期间，运行能够覆盖已修改文件、包、契约和失败模式的最小检查集合。
- 实施稳定后，对每个受影响的技术栈运行一次完整交付门槛。
- 仅后端工作不运行前端检查，仅前端工作不运行后端检查，仅规则和仅文档工作不运行应用测试套件。
- 同一任务或发布流程中，可复用针对完全相同且未变更源码版本的成功检查。检查后如果相关源码发生变化，应重新运行受影响的检查。
- 完整跨栈、竞态、部署和产物检查仅用于相应的高风险变更、主分支集成、发布或用户明确要求的场景。
- 始终说明使用的验证级别、实际运行的检查和有意不要求的检查。

### 迭代检查

范围适当的开发检查示例：

```bash
# 前端：相关 lint 和测试
cd frontend
npx eslint src/components/projects/AssetDetailModal.tsx src/hooks/useProjectAssets.ts
npm test -- src/test/projectAssetsWorkbench.test.tsx

# 后端：受影响的包或指定回归测试
cd backend
GIN_MODE=release go test ./internal/asset ./internal/task
GIN_MODE=release go test ./internal/api -run 'TestAssetRoutes|TestTaskHistoryRoutes'
```

当变更跨越 TypeScript 边界或需要诊断类型错误时，可在迭代期间运行 `npm run type-check`。任务准备交付后，不得以定向检查替代受影响技术栈的最终验证门槛。

## 本地开发环境

- 常规开发验证必须使用 `docs/local-development.md` 中的共享本地服务。
- 本地 MySQL、Redis 和 MinIO 检查使用 `dev-mysql8`、`dev-redis` 和 `dev-minio`。
- 用户已授权后续开发任务为验证而修改共享本地开发数据。Agent 可在共享本地 MySQL、Redis 和 MinIO 服务中创建、更新、删除、入队、上传、下载和清理任务自有测试数据。
- 所有本地测试数据都应使用清晰的命名空间，或通过其他方式明确归属于任务或分支。清理时必须避开无关数据。
- 常规功能验证不得启动项目专属 MySQL、Redis 或 MinIO 容器。
- 常规功能验证不得创建项目专属 Docker 数据卷。
- 除非用户明确要求，否则不得执行删除项目数据库、删除共享存储桶或运行 Redis `FLUSHALL` / `FLUSHDB` 等大范围破坏性命令。
- 不得将真实本地服务密码复制到项目文件、测试、日志或最终回复中。
- `deploy/docker-compose.yml` 只能用于部署专项验证。如果它启动了项目专属容器，除非用户要求保留，否则验证后应清理：

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

## P0 文档验证

对于 P0 文档和 Agent 规则：

```bash
find docs agent-instructions -maxdepth 2 -type f | sort
git diff -- docs AGENTS.md agent-instructions
```

P0 不修改源码，因此不需要前端构建。

## 前端验证

交付已稳定的前端代码前，在 `frontend/` 中运行：

```bash
npm run lint
npm run test
npm run build
```

`npm run build` 已包含 `tsc -b`。除非有意跳过构建或需要单独进行仅类型诊断，否则不要在同一最终验证门槛中再次运行 `npm run type-check`。

仅前端变更不要求运行后端测试套件。仅 CSS 变更使用相关组件测试和生产构建；只有变更可能影响共享样式、布局、构建输出或多个页面时，才运行完整前端验证门槛。

## 后端验证

实施期间使用受影响包或指定测试命令。交付已稳定的普通后端代码前，在 `backend/` 中运行：

```bash
GIN_MODE=release go test ./...
go vet ./...
```

竞态检测按风险决定：

- 修改 Worker 执行、队列、SSE、锁、并发租约、取消/重试状态转换、共享可变状态或 goroutine 生命周期代码时，对受影响包运行 `GIN_MODE=release go test -race`。
- 任务修改并发敏感的后端行为时，在合入主分支或发布前运行 `GIN_MODE=release go test -race ./...`；用户明确要求时也应运行。
- 普通校验、DTO、Repository 过滤、文案和独立 Handler 变更不要求全仓库竞态检查，除非同时涉及并发敏感行为。

需要 MySQL、Redis 或 MinIO 的 API 和 Worker 变更，应连接 `docs/local-development.md` 中的共享本地服务。

## 变更范围矩阵

| 变更范围 | 迭代检查 | 最终交付门槛 |
| --- | --- | --- |
| 仅规则或文档 | Diff、链接和 Markdown 结构检查 | 不运行前端或后端应用测试套件 |
| 仅前端 | 相关 ESLint 和 Vitest 文件；有需要时进行类型检查 | 前端 lint、测试和构建 |
| 普通后端 | 受影响包或指定测试 | 后端完整测试和 vet |
| 并发敏感后端 | 受影响测试加受影响包竞态检查 | 后端完整测试和 vet；主分支集成或发布时运行完整竞态检查 |
| API 或跨栈契约 | 相关后端 API 测试和前端 API/UI 测试 | 两个受影响技术栈的门槛及契约文档检查 |
| 仅部署 | 静态配置或脚本契约检查 | 部署配置校验和定向冒烟测试 |
| 发布或 NAS 产物 | 可用时复用精确 commit 的源码验证 | 产物架构、校验和、首次启动、健康检查、重启和持久化冒烟检查 |

对于部署专项变更，还应验证 Docker Compose 路径：

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml up -d
```

部署专项本地验证后，除非另有要求，应清理项目 Compose 服务栈：

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

如果从干净 commit 构建 NAS 产物，且该 commit 已在同一工作流中通过所需源码门槛，则构建可使用 `SKIP_TESTS=1`，避免重复运行未变源码的测试。使用此例外时必须记录已验证的 commit，并仍然运行 NAS 部署契约、镜像架构、校验和、首次启动、健康检查和重启检查。未提交或未经验证的工作区不得使用此选项。

## 输出与 Token 效率

- 优先使用精简 Reporter 和非详细测试命令。除非正在调查失败，否则不使用 `-v`。
- API 测试设置 `GIN_MODE=release`，避免打印完整 Gin 路由表。
- 成功时只报告命令、通过数量或包摘要、必要时的耗时和最终状态。
- 失败时将完整日志保留在本地，只返回失败测试、根本错误和有用的末尾或摘录。不要将重复的成功包输出持续发送到对话中。
- 默认复用 Go 构建/测试缓存。只有为复现非确定性问题或验证有状态行为而必须绕过缓存时，才使用 `-count=1`。
- 精简输出不得隐藏安全错误、竞态报告、迁移失败或第一个可操作的堆栈信息。

## 契约验证

修改 API、SSE、RBAC、队列、存储或安全行为时：

- 更新 `docs/` 中对应的文档。
- 为契约新增或更新测试。
- 适当时提供使用 curl、浏览器或服务日志进行手动验证的示例。

## 迁移验证

迁移任务需要验证以下三层：

1. 替换前仍必须正常工作的现有行为。
2. 任务有意引入的新中间态。
3. 不得出现的禁止半迁移状态。

当前后端路径与新路径并存时，前端迁移任务应增加可见 UI 状态与实际提交载荷之间的回归覆盖。

后端高风险任务应为任务包中指定的状态或失败矩阵增加回归覆盖，而不只是覆盖正常路径。
