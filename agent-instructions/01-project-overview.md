# 项目概况

## 产品

Amazon AI Product Image Studio 是面向 Amazon 卖家的 AI 产品图片生成与编辑平台。

平台帮助卖家：

- 使用 AI 生成产品图片。
- 使用 AI 编辑现有产品图片。
- 上传产品参考图。
- 按产品管理独立项目。
- 在项目素材库中保存生成图、编辑图和参考图。
- 跟踪提示词、任务历史、用量、成本和审计事件。

## 当前状态

仓库已完成截至 R12 的平台化建设：

- 原有 React + TypeScript + Vite + Tailwind 前端现位于 `frontend/`。
- Go + Gin + GORM 后端现位于 `backend/`，并提供 API 和 Worker 入口。
- Docker Compose 部署资源位于 `deploy/`。
- 已实现认证、RBAC、租户隔离、租户用户/角色管理、项目、项目成员、素材、Provider/模型管理、任务 API、Redis 队列、Worker 执行、SSE、统一项目历史、用量/审计查询、上传策略设置，以及截至 P12 的运行时加固。
- 生产环境的生成/编辑流程经过后端任务 API、后端 Provider Adapter、Redis 队列、Worker 执行、MinIO 素材和 SSE；浏览器不得直接调用 AI Provider。
- 浏览器 Provider Adapter、普通用户的 Provider API Key/API URL 设置，以及基于 IndexedDB 的生产历史/图片路径已从生产导入图中移除或停用。
- 面向卖家的项目流程现已包含后端驱动的项目编辑、素材筛选、素材元数据编辑、项目成员入口、统一历史、授权详情/下载/再次编辑，以及后端对成员写入的最后一个 `OWNER` 保护。
- IndexedDB 仍可用于非敏感的本地提示词模板便捷数据和测试，但不得重新作为平台历史或图片的事实来源。

应继续保留并演进现有前端 UI 概念；除非后续任务明确授权替换，否则不得从头重写 UI。

## 目标技术栈

- 前端：React + TypeScript + Vite + Tailwind CSS。
- 后端：Go + Gin + GORM。
- 数据库：MySQL 8。
- 队列和缓存：Redis。
- 对象存储：MinIO。
- 认证：JWT + HttpOnly Cookie。
- 授权：RBAC + `tenant_id` 隔离。
- 任务状态：仅使用 SSE。
- 部署：Docker Compose。

## 历史 P0 非目标

P0 已完成，其旧约束仅作为历史记录：P0 未实现后端业务代码、移动前端文件、重构 React 组件或替换本地存储路径。当前任务必须遵循 `docs/development-plan.md` 和 `docs/codex-agent-tasks.md` 中的最新阶段计划。

## 实施原则

- 优先采用具有明确契约的增量修改。
- 保留现有面向用户的概念，同时确保当前后端驱动的生产路径真实有效。
- 除非后端等价路径已经存在、完成验证，且任务明确负责该迁移，否则不得删除或替换现有生产路径。
