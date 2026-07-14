# 极空间 X86 NAS 单镜像部署

这份镜像面向 16 GB 内存的 X86 极空间 NAS。前端、Go API、Go Worker、MySQL、Redis、MinIO 和 Nginx 都运行在同一个 `linux/amd64` 容器中。

极空间创建容器时只需要配置两项：

- 一个对外端口：宿主机端口映射到容器 `8080/TCP`。
- 一个数据目录：外部硬盘目录映射到容器 `/data`，权限为读写。

不需要填写数据库密码、JWT 密钥或其他环境变量。镜像首次启动会生成独立随机密钥，并保存到 `/data/config/runtime-secrets.env`。只要保留原 `/data` 映射，删除或升级容器后仍能继续使用原数据和密钥。

> 单容器适合家用 NAS 和局域网使用。多人高并发或公网生产环境仍建议使用仓库原有的多容器部署，以便数据库、队列和对象存储独立升级与故障隔离。

## 1. 在 Mac mini M4 构建镜像包

启动 Docker Desktop，在仓库根目录执行：

```bash
bash deploy/nas-x86/build-package.sh v1
```

脚本会运行后端测试、前端 lint、类型检查、测试和生产依赖审计，然后固定构建 `linux/amd64` 镜像。输出文件：

```text
dist/nas-x86/gpt-image-nas-amd64-v1.docker.tar
dist/nas-x86/gpt-image-nas-amd64-v1.docker.tar.sha256
dist/nas-x86/gpt-image-nas-amd64-v1.README.md
```

后续构建只需更换版本号，例如：

```bash
bash deploy/nas-x86/build-package.sh v2
```

## 2. 在极空间 Docker 界面导入

把 `.docker.tar` 和 `.sha256` 复制到 NAS。可先通过终端校验：

```bash
sha256sum -c gpt-image-nas-amd64-v1.docker.tar.sha256
```

然后在极空间 Docker 界面中：

1. 打开“镜像”，选择“导入镜像”。
2. 选择 `gpt-image-nas-amd64-v1.docker.tar`，等待导入完成。
3. 使用导入后的 `amazon-ai-product-image-studio-nas:v1` 创建容器。
4. 端口映射设置为“NAS 端口 `8080` → 容器端口 `8080/TCP`”。NAS 端口也可以改成其他未占用端口。
5. 文件夹映射设置为“外部硬盘目录 → `/data`”，并选择读写权限。例如：

   ```text
   /volume2/docker-data/gpt-image  →  /data
   ```

6. 建议将重启策略设置为“除非手动停止，否则自动重启”。
7. 不需要配置环境变量，直接创建并启动容器。

首次启动需要初始化 MySQL 和 MinIO，通常会比后续启动慢。容器健康后访问：

```text
http://NAS_IP:8080
```

如果 NAS 侧使用了其他端口，就把地址中的 `8080` 换成对应端口。

## 3. 命令行启动方式（可选）

不使用极空间界面时，也可以执行：

```bash
docker load -i gpt-image-nas-amd64-v1.docker.tar

docker run -d \
  --name gpt-image-studio \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /volume2/docker-data/gpt-image:/data \
  amazon-ai-product-image-studio-nas:v1
```

这条命令同样不需要传入环境变量。

## 4. 初始化管理员

容器健康后，只执行一次：

```bash
curl -fsS -X POST http://NAS_IP:8080/api/v1/auth/init-admin \
  -H 'Content-Type: application/json' \
  -d '{
    "tenantName": "我的团队",
    "email": "你的管理员邮箱",
    "password": "替换为至少 12 位的强密码",
    "displayName": "管理员"
  }'
```

成功后即可在浏览器登录。重复初始化会返回冲突，不会覆盖原管理员。中转站 URL 和 API Key 请在登录后的后台页面配置；API Key 会使用 `/data/config` 中的主密钥加密后存入 MySQL。

## 5. 外部数据目录

必须备份映射到 `/data` 的整个外部目录：

| `/data` 子目录 | 内容 | 是否必须备份 |
| --- | --- | --- |
| `config/` | MySQL、Redis、MinIO、JWT 和 Provider Key 加密密钥 | 是 |
| `mysql/` | 用户、权限、产品、任务、模型、中转站和审计数据 | 是 |
| `mysql-tmp/` | MySQL 运行期临时文件 | 否，可清理 |
| `minio/` | 上传素材、生成图片和缩略图 | 是 |
| `redis/` | 任务队列、SSE 辅助状态、登录限流和验证码短期状态 | 建议 |
| `tmp/` | Provider 大图响应临时文件 | 否，可清理 |

不要单独删除或修改 `config/runtime-secrets.env`。密钥丢失后，数据库密码和已加密的 Provider API Key 无法自动恢复。

## 6. 备份、升级与回滚

备份时先停止容器，再使用极空间的快照或备份功能复制完整外部数据目录。不要只复制运行中的 MySQL 文件。

升级步骤：

1. 停止旧容器并备份完整 `/data` 外部目录。
2. 导入新版本 `.docker.tar`。
3. 删除旧容器，但不要删除外部数据目录。
4. 使用新镜像创建容器，继续映射原目录到 `/data`，继续映射容器 `8080` 端口。
5. 启动并等待健康检查通过。

回滚时应恢复升级前的数据快照并使用匹配的旧镜像。数据库迁移可能是前向迁移，不能只切回旧镜像而不恢复数据。

## 7. 安全与常见问题

- 镜像默认按可信局域网 HTTP 使用。不要在路由器上直接把端口映射到公网；公网访问请使用极空间 HTTPS 反向代理。
- `exec format error`：导入的不是 `linux/amd64` 镜像，需重新使用本项目构建脚本生成。
- `/data 不可写`：确认文件夹映射为读写，并检查外部硬盘目录权限。
- 日志反复停在“首次启动，正在初始化 MySQL 数据目录”：旧版镜像可能在异常中断后留下半初始化目录，导致后续重启继续失败。新版会先在 `/data/.mysql-initialize` 中完成初始化，再切换到正式目录，并自动重建不完整且尚未包含 MySQL 系统表的目录；`/data/config` 中的持久化密钥不会被删除。
- 容器长时间处于启动中：查看容器日志，重点检查 MySQL、Redis、MinIO 或 API 初始化错误。新版会把 MySQL 初始化错误的末尾内容直接输出到容器日志；完整日志仍保存在 `/data/mysql/error.log`，初始化尚未完成时保存在 `/data/.mysql-initialize/error.log`。
- MySQL 报文件系统或权限错误：确认 `/data` 映射为读写，且所在存储支持 Linux 文件所有者、文件锁和同一文件系统内重命名。不要删除 `/data/config/runtime-secrets.env`；如果已经产生业务数据，也不要手工清空 `/data/mysql`。
- 页面可打开但无法生图：在管理页面检查中转站、模型、用户模型授权和 API 调用日志。
- 16 GB NAS 默认生成并发为 `2`；如果同时运行很多其他服务，可在极空间容器高级设置中把内存上限设为约 `12 GB`。

可在终端查看内部服务状态：

```bash
docker exec gpt-image-studio supervisorctl status
docker exec gpt-image-studio /usr/local/bin/healthcheck.sh
```
