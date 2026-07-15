# 架构规则

## 目标目录结构

仓库根目录是当前平台根目录：

```text
gpt-image/
  frontend/
  backend/
  deploy/
  docs/
  agent-instructions/
  AGENTS.md
  README.md
  .env.example
```

P1 前端机械迁移已完成。不得把后端代码移动到 `frontend/` 下，也不得重新混合应用根目录。新增前端工作放在 `frontend/`，后端工作放在 `backend/`，部署工作放在 `deploy/`，公开契约放在 `docs/`。

## 服务边界

- 前端负责 UI、本地草稿状态、API 客户端、SSE 客户端和浏览器交互。
- 后端 API 负责认证、授权、业务 API、上传授权、任务创建、Provider/模型管理和 SSE 交付。
- 后端 Worker 负责队列任务执行、Provider 调用、输出上传、用量记录和任务事件创建。
- MySQL 是用户、租户、项目、素材、任务、任务事件、日志和用量的最终事实来源。
- Redis 仅用于队列、锁、并发限制、缓存、速率限制和临时状态。
- MinIO 存储图片对象和缩略图。

## 必需数据流

1. 用户通过后端完成认证并接收 HttpOnly Cookie。
2. 前端从 `/api/v1` 加载项目、素材、Provider 和模型能力。
3. 用户创建生成任务。
4. 后端把任务持久化到 MySQL，并将其加入 Redis 队列。
5. Worker 领取任务、应用运行时并发限制、调用选定的 Provider Adapter、把输出存入 MinIO，并向 MySQL 写入任务事件。
6. 前端通过 SSE 接收任务事件，无需轮询即可更新 UI。
7. 项目历史必须使用后端统一历史接口；不得在浏览器中拼接任务列表和素材列表来重建生产历史流。

## 强制架构规则

- 前端不得调用 AI Provider。
- 前端不得存储 API Key。
- 前端不得轮询任务状态。
- 业务 SQL 查询不得缺少租户范围。
- MySQL 不得存储图片二进制数据。
- 业务代码不得直接依赖具体 Provider SDK 或 URL，必须使用 Provider Adapter 接口。
