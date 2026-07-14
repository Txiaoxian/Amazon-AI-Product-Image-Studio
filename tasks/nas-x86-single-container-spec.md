# X86 NAS 单容器部署规格

## 目标与边界

- 在 Apple Silicon Mac 上构建 `linux/amd64` 镜像，供 16 GB 内存的 X86 极空间 NAS 离线导入。
- 交付一个镜像和一个 Compose 服务；容器内运行前端、Go API、Go Worker、MySQL、Redis 与 MinIO。
- 仅公开 Web 端口；数据库、队列、对象存储和后端端口不映射到宿主机。
- MySQL、Redis、MinIO 及大图临时文件统一位于容器 `/data`，由 NAS 外部硬盘目录绑定挂载。
- 自动生成的内部密钥与数据目录都保留在 NAS 的 `/data` 映射中；Provider API Key 只通过后台页面写入并加密存储，不出现在镜像或构建日志中。

## 交付物

- 多阶段单镜像 Dockerfile。
- 可选的单服务 `docker-compose.yml`，以及无需环境变量即可启动的镜像入口。
- Mac M4 构建脚本，直接输出极空间 Docker 界面可导入的镜像归档。
- 可重复运行的部署契约检查和运行健康检查。
- 中文部署、备份、升级、回滚及故障处理说明。

## 验收标准

- `docker buildx` 明确使用 `--platform linux/amd64`，归档经 `docker load` 后架构为 `amd64`。
- Compose 配置只有一个服务、一个 `/data` 数据根挂载和一个 Web 端口。
- 首次启动自动初始化内部 MySQL、Redis、MinIO 存储与三个 MinIO bucket；API 自动迁移数据库。
- 容器健康检查同时覆盖 Nginx、API、MySQL、Redis、MinIO 与 Worker 就绪文件。
- 重启或重建容器后，用户、配置、任务、队列和图片仍存在。
- 缺少密钥、占位符密钥、非法数据目录或内部依赖启动失败时，容器必须明确失败，不得带病运行。

## 失败模式

| 场景 | 预期处理 |
| --- | --- |
| 在 M4 上直接构建成 arm64 | 构建脚本固定 `linux/amd64`，产物校验不符即失败 |
| NAS 外部目录不可写 | 入口脚本在启动服务前失败并输出目录错误 |
| 未配置内部密钥 | 镜像首次启动生成独立随机值并持久化到 `/data/config`，入口脚本 fail-closed 校验 |
| MySQL/Redis/MinIO 未就绪 | API/Worker 等待依赖，健康检查保持失败 |
| 某个内部进程退出 | Supervisor 重启该进程，容器健康检查反映未就绪状态 |
| 更新镜像失败 | 保留旧镜像归档和同一数据目录，恢复旧 `STUDIO_IMAGE` 后重建容器 |
| Provider 返回超大图片 | Worker 临时文件写入 `/data/tmp`，避免占满容器可写层；仍遵守应用大小上限 |

## 非目标与权衡

- 单容器符合本次便捷部署要求，但数据库、队列和对象存储无法独立升级、扩容或故障隔离；面向多人或公网的大规模生产环境仍应使用现有多容器拓扑。
- 默认面向 NAS 局域网 HTTP，因此以 `staging` 模式运行；公网访问必须在 NAS 反向代理启用 HTTPS，并切换生产安全配置。
