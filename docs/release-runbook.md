# 发布运行手册索引

规范的运维人员运行手册是[`deploy/RUNBOOK.md`](../deploy/RUNBOOK.md)。

发布命令、生产密钥处理、MinIO 引导初始化、SSE 代理要求、
备份/恢复步骤、升级/回滚说明、真实 Provider 冒烟测试指导和
生产试运行说明统一保留在该文档中，避免运维记录出现偏差。

存储库范围内的规划和验证证据保留在：

- [`docs/deployment.md`](deployment.md)
- [`docs/development-plan.md`](development-plan.md)
- [`docs/security.md`](security.md)

生产规则不变：

- Provider API 密钥仅通过后端管理 API 进行配置。
- 前端代码不得直接调用 AI Provider 或中继端点。
- 前端代码不得保留 Provider API 键。
- 任务进度使用SSE，从不轮询。
- 生产密钥不得使用占位符，也不得写入已提交文件、
  日志、屏幕截图、shell 历史记录或验证证据。
- MySQL 和 MinIO 备份必须作为一致的备份对创建和恢复。
