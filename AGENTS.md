# 项目 AGENTS.md

这是 Amazon AI Product Image Studio 的项目级主说明文件，仅作为索引使用。详细规则位于 `agent-instructions/`。

## 使用方式

- 先阅读本文件，再根据任务类型阅读匹配的子说明文件。
- 如果任务同时涉及多个主题，应阅读所有匹配的文件。
- 全局 Codex 说明仍然生效；本目录中更具体的项目规则在本仓库内优先级更高。
- 编辑项目说明时，继续保持“主 `AGENTS.md` + `agent-instructions/` 专项文件”的结构。
- 进行 worktree 任务规划、迁移任务或审查交接时，除领域专项规则外，还必须阅读 `agent-instructions/07-task-package-and-review-rules.md`。
- 选择工具或 skill 时，必须阅读 `agent-instructions/08-tool-and-skill-rules.md`；本项目不依赖已删除或当前不可用的专项 skill。

## 项目决策

目标平台采用 monorepo 风格的根目录结构：

- `frontend/`：现有 React + TypeScript + Vite + Tailwind 应用。
- `backend/`：Go + Gin + GORM API 和 Worker 服务。
- `deploy/`：Docker Compose、服务配置和部署资源。
- `docs/`：架构、契约、安全和开发计划文档。

P0/P1 已完成，`frontend/`、`backend/`、`deploy/` 和 `docs/` 的目录划分是当前正式仓库结构。

## 强制平台规则

- 前端不得直接调用 OpenAI、Gemini 或任何 AI 中转站。
- 前端不得在 localStorage、IndexedDB、sessionStorage、源代码或客户端可见配置中存储 AI Provider API Key。
- 任务状态必须使用 SSE；禁止使用轮询、`setInterval` 或重复 fetch 循环查询任务进度。
- 所有 AI 调用必须经过 Go 后端和 Provider Adapter。
- 每张业务表都必须包含 `tenant_id`，租户范围查询必须按 `tenant_id` 过滤。
- 图片必须存储在 MinIO；MySQL 只保存元数据和 `object_key`，不得保存图片二进制数据。
- API Key 必须加密存储，且不得完整返回前端。
- 日志不得包含完整 API Key、Authorization 请求头、Cookie 或图片 base64 数据。
- Provider `base_url` 必须防御 SSRF，并阻止 localhost、回环地址、私有地址、链路本地地址和 Docker 内部目标。
- 文件上传必须校验真实文件类型、大小、尺寸和像素总数，禁止上传 SVG。
- 使用对象 ID 的 API 必须检查对象级授权，不能只检查登录状态。
- 图片下载必须经过后端授权。
- 平台面向用户的文本应尽量使用简体中文，尤其是配置标签、说明文案、校验提示和错误信息；必要的技术标识、枚举值、模型 ID、API 字段名可保持原始形式。
- 项目文档、规则、任务说明和交付说明默认使用简体中文；代码、命令、路径、API 字段、枚举、模型 ID、P0/P1 等阶段编号及其他必须保持精确的技术标识可保留原样。

## 说明索引

| 文件 | 阅读场景 | 内容摘要 |
| --- | --- | --- |
| `agent-instructions/01-project-overview.md` | 所有任务 | 产品目标、当前状态、目标技术栈和非目标。 |
| `agent-instructions/02-architecture-rules.md` | 架构、目录、服务边界 | 目标 monorepo 布局、服务边界、数据流和强制架构规则。 |
| `agent-instructions/03-frontend-rules.md` | 前端代码、UI、状态、API 集成 | 在使用后端契约替换本地 AI/数据路径时保留现有 React UI。 |
| `agent-instructions/04-backend-rules.md` | Go 后端、数据库、Worker、队列 | Gin/GORM 结构、租户过滤、Redis 队列和 MySQL 最终数据源。 |
| `agent-instructions/05-security-rules.md` | 认证、RBAC、上传、Provider 配置、日志 | Cookie、SSRF、API Key、上传、审计和日志的安全要求。 |
| `agent-instructions/06-testing-and-delivery.md` | 验证、交付说明、PR 交接 | 分层风险验证、紧凑输出、简体中文 Git 提交信息和按改动类型划分的交付要求。 |
| `agent-instructions/07-task-package-and-review-rules.md` | Worktree 任务规划、迁移任务、审查交接 | 必需的任务包章节、中间态规则、失败矩阵和回归测试映射。 |
| `agent-instructions/08-tool-and-skill-rules.md` | 所有任务中的工具和 skill 选择 | 仅使用当前实际可用且适合任务的能力；专项 skill 缺失时直接使用项目内置流程，不得阻塞或提示安装。 |

## 相关规划文档

- `docs/architecture.md`
- `docs/business-requirements.md`
- `docs/database-schema.md`
- `docs/api-contract.md`
- `docs/sse-contract.md`
- `docs/rbac.md`
- `docs/provider-adapter.md`
- `docs/task-queue.md`
- `docs/storage.md`
- `docs/security.md`
- `docs/deployment.md`
- `docs/local-development.md`
- `docs/development-plan.md`
- `docs/codex-agent-tasks.md`
