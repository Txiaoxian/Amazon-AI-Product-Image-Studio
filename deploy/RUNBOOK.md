# 部署运行手册

本运行手册适用于通过 `deploy/docker-compose.yml` 部署
Amazon AI Product Image Studio 的运维人员。

除非命令另有说明，否则从存储库根运行命令。

## 安全规则

- 永远不会commit`.env`或真正的秘密值。
- 不要将ProviderAPI钥匙放入`.env`；通过后端配置它们
  管理 API，以便它们在静态时被加密。
- 保持前端仅指向后端。它不能代理或调用
  OpenAI、Gemini，或直接中继端点。
- 使用 SSE 查看任务状态。不要在生产部署路径中增加轮询检查。
- 将对象存储保存在MinIO中。 MySQL 仅存储元数据和对象键。

## 环境变量

从模板开始：

```bash
cp .env.example .env
```

所需团体：

- `COMPOSE_PROJECT_NAME`、`APP_ENV`、`LOG_LEVEL`、`TZ`。
- 主机绑定：`FRONTEND_BIND_HOST`、`FRONTEND_PORT`、
  `BACKEND_API_BIND_HOST`，`BACKEND_API_PORT`。
- 构建输入：`FRONTEND_*`、`BACKEND_*`、图像变量。
- MySQL：`MYSQL_ROOT_PASSWORD`、`MYSQL_DATABASE`、`MYSQL_USER`、
  `MYSQL_PASSWORD`，连接池设置。
- Redis：`REDIS_PASSWORD`、`REDIS_DB`、`REDIS_APPENDONLY`。
- MinIO：`MINIO_ROOT_USER`、`MINIO_ROOT_PASSWORD`、`MINIO_ACCESS_KEY`、
  `MINIO_SECRET_KEY`, `MINIO_BUCKET_ORIGINALS`,
  `MINIO_BUCKET_GENERATED`，`MINIO_BUCKET_THUMBNAILS`。
- 授权和CSRF：`JWT_SIGNING_SECRET`、`AUTH_LOGIN_RATE_LIMIT_MAX_FAILURES`、
  `AUTH_LOGIN_RATE_LIMIT_WINDOW`、`AUTH_COOKIE_NAME`、`COOKIE_DOMAIN`、
  `COOKIE_SECURE`、`COOKIE_SAME_SITE`、`CSRF_ENABLED`、`CSRF_COOKIE_NAME`、
  `CSRF_HEADER_NAME`。
- CORS：`CORS_ALLOWED_ORIGINS`。
- API-密钥加密：`API_KEY_ENCRYPTION_KEY`，
  `API_KEY_ENCRYPTION_KEY_ID`。
- 上传限制：`UPLOAD_MAX_FILE_SIZE_MB`、`UPLOAD_MAX_WIDTH`、
  `UPLOAD_MAX_HEIGHT`，`UPLOAD_MAX_PIXELS`，`UPLOAD_ALLOWED_MIME_TYPES`。
- 队列和Worker控件：`TASK_*`、`WORKER_*`、`MIGRATIONS_MODE`。
- Provider运行时默认值：`PROVIDER_TIMEOUT_SECONDS`，
  `PROVIDER_MAX_RETRIES`，`PROVIDER_MAX_RESPONSE_SIZE_MB`，
  `PROVIDER_MAX_OUTPUT_IMAGE_SIZE_MB`。

Provider 图像响应通过 Worker 临时目录进行解码，
然后流入MinIO。根据配置调整Worker临时存储的大小
任务并发和输出限制。 `PROVIDER_MAX_RESPONSE_SIZE_MB` 默认为
1024 MiB 和 `PROVIDER_MAX_OUTPUT_IMAGE_SIZE_MB` 默认为 512 MiB；后者
不得超过前者。这些设置不会更改用户上传限制。

对于生产，设置`APP_ENV=production`，`COOKIE_SECURE=true`，限制
`CORS_ALLOWED_ORIGINS`到公共前端源点，绑定公共流量
通过TLS终止反向代理。保留`FRONTEND_BIND_HOST=127.0.0.1`
和 `BACKEND_API_BIND_HOST=127.0.0.1`；任何 Compose 端口都不得作为
公共监听端口。

`CSRF_HEADER_NAME`是固定的兼容性合约，必须保留
`X-CSRF-Token`。后端和生产预检拒绝别名。

## 生产秘密

为每个环境生成独特的高熵值：

- `MYSQL_ROOT_PASSWORD`，`MYSQL_PASSWORD`。
- `REDIS_PASSWORD`。
- `MINIO_ROOT_USER`、`MINIO_ROOT_PASSWORD`、`MINIO_ACCESS_KEY`、
  `MINIO_SECRET_KEY`。
- `JWT_SIGNING_SECRET`。
- `API_KEY_ENCRYPTION_KEY`。

`API_KEY_ENCRYPTION_KEY` 必须是按预期编码的有效 32 字节密钥
后端配置。当前应用程序有一个有效的加密密钥。
在更改活动密钥之前，请使用下面的运维人员工作流程
存储 Provider 凭据的环境。

## 启动顺序

Compose 使用健康检查对启动顺序进行编码：

1. 首先开始MySQL、Redis和MinIO。
2. `minio-bootstrap` 幂等创建所需的桶。
3. `backend-api`在数据服务和存储桶引导初始化正常后启动。
4. `backend-worker`在`backend-api`之后启动，数据服务正常。
5. `frontend`在`backend-api`健康后开始。

启动堆栈：

```bash
docker compose -f deploy/docker-compose.yml up -d
```

检查状态：

```bash
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml logs --tail=100 backend-api backend-worker frontend
```

## 外部TLS反向代理

在启用主机级TLS反向代理之前，不要开放公共流量
并得到验证。使用
[`nginx/amazon-ai-product-image-studio.conf.template`](./nginx/amazon-ai-product-image-studio.conf.template)
作为可审核的生产模板。

主机代理：

- 将HTTP端口`80`重定向至HTTPS；
- 在`443`端口终止TLS；
- 添加HSTS、`X-Content-Type-Options`、`X-Frame-Options`和
  `Referrer-Policy`；
- 代理 `/`、`/api/` 和 `/api/v1/events/` 仅适用于环回绑定
  前端位于`127.0.0.1:8080`；
- 绝不将公共边缘直接代理到`backend-api`、OpenAI、Gemini，或
  一个继电器。

证书颁发和更新由运维人员或平台负责。
存储库模板包含占位符证书和私钥
仅路径。不要commit真实的证书或私钥。

准备主机配置：

```bash
cp deploy/nginx/amazon-ai-product-image-studio.conf.template \
  /etc/nginx/conf.d/amazon-ai-product-image-studio.conf
# Replace __PUBLIC_HOST__ and update operator-managed TLS paths if needed.
bash scripts/tls-reverse-proxy-check.sh \
  --config /etc/nginx/conf.d/amazon-ai-product-image-studio.conf
nginx -t
# Reload Nginx only after nginx -t succeeds.
```

在`nginx -t`之后使用平台认可的重新加载命令，例如
`systemctl reload nginx`。然后验证 HTTPS 响应标头，HTTP
重定向，以及开放流量前的SSE路线：

```bash
curl -fsSI http://studio.example.com/
curl -fsSI https://studio.example.com/
curl -N --max-time 10 https://studio.example.com/api/v1/events/tasks
```

未经身份验证的SSE请求可能会返回授权错误；它仍然
检查边缘路由。在Go/No-Go之前，使用经过身份验证的浏览器会话或
受限制的 cookie jar，确认任务事件无需批处理即可增量到达。

## 初始化管理

API健康后，创建第一个租户管理员一次：

```bash
curl -fsS -X POST http://127.0.0.1:8081/api/v1/auth/init-admin \
  -H 'Content-Type: application/json' \
  -d '{
    "tenantName": "Default Tenant",
    "email": "admin@example.com",
    "password": "replace-with-a-strong-password",
    "displayName": "Admin"
  }'
```

管理员已存在后，端点返回冲突。不要自动化
这是通过硬编码生产的。

## 额外的租户配置

未经身份验证的 init-admin 端点仅适用于第一个租户。创建
第二个及以后的租户通过运维人员CLI。默认命令验证
无需打开数据库或写入行即可输入：

```bash
export PROVISION_TENANT_NAME='Seller Team'
export PROVISION_TENANT_ADMIN_EMAIL='seller-admin@example.com'
export PROVISION_TENANT_ADMIN_DISPLAY_NAME='Seller Admin'
docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -e PROVISION_TENANT_NAME -e PROVISION_TENANT_ADMIN_EMAIL \
  -e PROVISION_TENANT_ADMIN_DISPLAY_NAME \
  --entrypoint provision-tenant backend-api
```

仅在审查试运行后才适用：

```bash
PROVISION_TENANT_CONFIRM=I_UNDERSTAND_TENANT_PROVISIONING \
  docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -e PROVISION_TENANT_NAME -e PROVISION_TENANT_ADMIN_EMAIL \
  -e PROVISION_TENANT_ADMIN_DISPLAY_NAME -e PROVISION_TENANT_CONFIRM \
  --entrypoint provision-tenant backend-api --apply
unset PROVISION_TENANT_CONFIRM
```

应用路径以事务方式创建一个租户、内置角色和授权，
一名租户管理员，一份已脱敏的审核记录。它仅打印新租户ID
需要登录。这两个命令都从容器 stdin 读取初始密码
当`PROVISION_TENANT_ADMIN_PASSWORD`未通过时没有回显。不保留
shell环境中的密码。

## Provider 主密钥轮换

仅在批准期间轮换 Provider API-密钥加密主密钥
维护时段 Provider 写入已暂停。首先运行默认的试运行
针对所有 Provider 行。试运行验证活跃 Provider 凭据和
仅报告仍需要 ASS 的软删除 Provider 行的计数
加密擦除：

```bash
export PROVIDER_KEY_ROTATION_OLD_SECRET='<current secret>'
export PROVIDER_KEY_ROTATION_OLD_KEY_ID='<current key id>'
export PROVIDER_KEY_ROTATION_NEW_SECRET='<new secret>'
export PROVIDER_KEY_ROTATION_NEW_KEY_ID='<new key id>'
docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -e PROVIDER_KEY_ROTATION_OLD_SECRET -e PROVIDER_KEY_ROTATION_OLD_KEY_ID \
  -e PROVIDER_KEY_ROTATION_NEW_SECRET -e PROVIDER_KEY_ROTATION_NEW_KEY_ID \
  --entrypoint provider-key-rotation backend-api
```

试运行成功后应用：

```bash
PROVIDER_KEY_ROTATION_CONFIRM=I_UNDERSTAND_PROVIDER_KEY_ROTATION \
  docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -e PROVIDER_KEY_ROTATION_OLD_SECRET -e PROVIDER_KEY_ROTATION_OLD_KEY_ID \
  -e PROVIDER_KEY_ROTATION_NEW_SECRET -e PROVIDER_KEY_ROTATION_NEW_KEY_ID \
  -e PROVIDER_KEY_ROTATION_CONFIRM \
  --entrypoint provider-key-rotation backend-api --apply
unset PROVIDER_KEY_ROTATION_OLD_SECRET PROVIDER_KEY_ROTATION_OLD_KEY_ID
unset PROVIDER_KEY_ROTATION_NEW_SECRET PROVIDER_KEY_ROTATION_NEW_KEY_ID
unset PROVIDER_KEY_ROTATION_CONFIRM
```

应用路径序列化一个数据库事务，重新加密所有符合条件的事务
活跃的凭据，从软删除中加密删除了凭据材料Provider
行，如果任何活动行失败，则完全回滚。它从不打印明文，
密文、提示、URL、租户或Provider详细信息。申请成功后，部署
API 和 Worker 以及新的 `API_KEY_ENCRYPTION_KEY` 和
`API_KEY_ENCRYPTION_KEY_ID`，然后运行后端介导的Provider冒烟测试检查。

## MinIO 引导初始化

`minio-bootstrap` 运行`mc mb --ignore-existing`：

- `MINIO_BUCKET_ORIGINALS`
- `MINIO_BUCKET_GENERATED`
- `MINIO_BUCKET_THUMBNAILS`

当需要重试存储桶创建时，仅重新运行引导初始化服务：

```bash
docker compose -f deploy/docker-compose.yml up minio-bootstrap
```

不要在例行部署期间删除存储桶。图像下载和任务输出
取决于存储在MySQL中的对象键。

## SSE 代理

前端图像使用`frontend/nginx.conf`：

- `/api/`代理`http://backend-api:8080`。
- `/api/v1/events/`代理`http://backend-api:8080`。
- SSE缓冲被`proxy_buffering off`禁用并且
  `X-Accel-Buffering: no`。

外部TLS反向代理模板保留流行为：

- 使用HTTP/1.1上游连接。
- 清除上游`Connection`标头。
- 禁用 `/api/v1/events/*` 的响应缓冲。
- 禁用`/api/v1/events/*`的代理缓存。
- 为SSE设置长读取超时。
- 转发`Host`、`X-Real-IP`、`X-Forwarded-For`，以及
  `X-Forwarded-Proto=https`。

## 发布验证

运行可重复释放门：

```bash
bash scripts/deploy-release-validation.sh
```

默认门包括主机TLS反向代理模板静态检查。
在重新加载 Nginx 之前，直接针对已安装的主机配置运行它：

```bash
bash scripts/tls-reverse-proxy-check.sh \
  --config /etc/nginx/conf.d/amazon-ai-product-image-studio.conf
```

对于实际 Compose冒烟测试：

```bash
bash scripts/deploy-release-validation.sh --up
docker compose -f deploy/docker-compose.yml ps
```

`--up`模式启动堆栈，等待健康检查，验证后端
健康端点，验证前端根，验证前端`/api/`代理
health，检查 SSE 代理身份验证边界，并使堆栈运行
运维人员检查。检查后清理：

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

该脚本还公开了一个仅清理的快捷方式：

```bash
bash scripts/deploy-release-validation.sh --down
```

重点安全回归：

```bash
bash scripts/security-regression.sh
```

手动部署检查：

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
curl -fsS http://127.0.0.1:8081/healthz
curl -fsS http://127.0.0.1:8081/api/v1/healthz
curl -fsS http://127.0.0.1:8080/api/v1/healthz
```

## 生产试运行

在进行 Go/No-Go 审核之前，运行已脱敏运维人员演练：

```bash
bash scripts/prod-dry-run.sh
```

默认模式运行发布验证、重点安全回归、
真正的Provider冒烟测试保护机制试运行，backup/restore演练保护机制
试运行和Compose配置验证。它不会启动持久服务，
替换数据，或者调用真实的AIProvider。

对于明确的生产环境验收检查，请将脚本指向
存储库外部现有的受限运行时环境文件：

```bash
bash scripts/prod-dry-run.sh \
  --production-env-file /secure/runtime/production.env
```

预检会读取但不会获取文件，也不会打印值。它
除非生产模式、安全 cookie、受限非本地主机，否则关闭失败
HTTPS CORS 起源，以及所需的非占位符秘密都存在。

对于现场 Compose 演练并进行清理：

```bash
bash scripts/prod-dry-run.sh --live-compose
docker compose -f deploy/docker-compose.yml ps -a
docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true
```

`--live-compose` 委托给`deploy-release-validation.sh --up --down`，所以
现有的范围清理陷阱会删除该项目的 Compose 容器，并且
即使实时验证失败也是如此。不要将其替换为广泛的Docker
修剪命令。

使用 [PRODUCTION_DRY_RUN_TEMPLATE.md](./PRODUCTION_DRY_RUN_TEMPLATE.md) 作为
R18审查证据。仅记录已脱敏阶段成绩。不要附加环境
文件、转储、秘密、Provider响应、图像输出、存储桶名称、对象
密钥、签名 URL 或服务。

## 可选真实 Provider 冒烟测试

真正的Provider冒烟测试是手动的、选择性的检查。它不是默认的一部分
发布验证或CI，因为它可以创建可计费的AI调用。

首先试运行保护机制：

```bash
bash scripts/real-provider-smoke.sh --dry-run
```

仅当您有意想要进行真正的后端介导的 AI 调用时才运行它：

```bash
export SMOKE_API_BASE_URL=http://127.0.0.1:8081/api/v1
export SMOKE_ADMIN_EMAIL=admin@example.com
export SMOKE_MODEL_NAME=your-image-model
read -rsp "Admin password: " SMOKE_ADMIN_PASSWORD && export SMOKE_ADMIN_PASSWORD
printf "\n"
read -rsp "Provider API key: " SMOKE_PROVIDER_API_KEY && export SMOKE_PROVIDER_API_KEY
printf "\n"
REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS \
bash scripts/real-provider-smoke.sh --run
```

要在生产试运行摘要中包含相同的可选检查，请保留
使用上面的隐藏输入设置并明确选择加入：

```bash
REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS \
bash scripts/prod-dry-run.sh --real-provider-smoke
```

这仅委托给`real-provider-smoke.sh --run`；没有第二个
Provider调用路径。除非运维人员已批准计费，否则跳过它
后端中转的冒烟测试。

不要将真实密钥放入提交的文件、shell 脚本、屏幕截图或
共享日志。该脚本仅调用该平台的`/api/v1`后端，创建
`codex-smoke-*` Provider/model/project/task数据，默认为一张输出图像，
并仅打印已脱敏 ID 和计数。之后查看并删除冒烟测试数据
验证环境是否应保持清洁。

## 备份

将 MySQL 和 MinIO 一起备份，以便对象键和对象保持一致。
如果可能的话，暂停写入或采取维护窗口。

在使用生产数据之前，请在一次性隔离项目中演练仓库提供的
Compose 流程：

```bash
bash scripts/backup-restore-rehearsal.sh
BACKUP_RESTORE_REHEARSAL_CONFIRM=I_UNDERSTAND_DATA_REPLACEMENT \
  bash scripts/backup-restore-rehearsal.sh --live
```

默认命令只检查保护机制。`--live` 会创建一个动态命名的
Compose 项目，只启动隔离的 MySQL 和 MinIO 服务，创建任务自有
测试数据，备份并恢复匹配的备份对，再次恢复以演练回滚，最后删除
其容器和数据卷。该命令绝不能指向共享本地开发服务或生产运行时。
使用下列模板记录已脱敏证据：
[PRODUCTION_BACKUP_RESTORE_TEMPLATE.md](./PRODUCTION_BACKUP_RESTORE_TEMPLATE.md)。

MySQL逻辑备份：

```bash
docker compose -f deploy/docker-compose.yml exec -T mysql \
  sh -c 'mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' \
  > backup/mysql.sql
```

MinIO 在堆栈运行时将对象备份到本地目录：

```bash
mkdir -p backup/minio
docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -v "$PWD/backup/minio:/backup" \
  --entrypoint /bin/sh minio-bootstrap -c '
    mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
    mc mirror --overwrite --remove local/"$MINIO_BUCKET_ORIGINALS" /backup/"$MINIO_BUCKET_ORIGINALS"
    mc mirror --overwrite --remove local/"$MINIO_BUCKET_GENERATED" /backup/"$MINIO_BUCKET_GENERATED"
    mc mirror --overwrite --remove local/"$MINIO_BUCKET_THUMBNAILS" /backup/"$MINIO_BUCKET_THUMBNAILS"
  '
```

生产环境中，运维人员必须使用平台认可的运行时备份工具，建立停止
写入或平台支持的一致性时间点，将 MySQL 和 MinIO 备份作为匹配的
一组保存，并把备份存放在 Compose 主机之外。仓库演练脚本不是
生产环境备份工具。

## 恢复

恢复到停止或隔离的堆栈：

```bash
docker compose -f deploy/docker-compose.yml down --remove-orphans
docker compose -f deploy/docker-compose.yml up -d mysql redis minio
docker compose -f deploy/docker-compose.yml exec -T mysql \
  sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' \
  < backup/mysql.sql
```

恢复到相同存储桶名称的 MinIO 对象，然后启动应用。
`--remove` 选项会删除备份中不存在的存储桶对象，确保恢复结果与
备份准确一致。该选项只能用于已停止或隔离的恢复目标：

```bash
docker compose -f deploy/docker-compose.yml up minio-bootstrap
docker compose -f deploy/docker-compose.yml run --rm --no-deps \
  -v "$PWD/backup/minio:/backup:ro" \
  --entrypoint /bin/sh minio-bootstrap -c '
    mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
    mc mirror --overwrite --remove /backup/"$MINIO_BUCKET_ORIGINALS" local/"$MINIO_BUCKET_ORIGINALS"
    mc mirror --overwrite --remove /backup/"$MINIO_BUCKET_GENERATED" local/"$MINIO_BUCKET_GENERATED"
    mc mirror --overwrite --remove /backup/"$MINIO_BUCKET_THUMBNAILS" local/"$MINIO_BUCKET_THUMBNAILS"
  '
docker compose -f deploy/docker-compose.yml up -d backend-api backend-worker frontend
```

在允许用户返回之前验证发布检查。

运维人员恢复生产环境时必须使用经批准的平台恢复工具，并从同一个
一致性时间点恢复匹配的 MySQL 和 MinIO 备份集。不要对生产运行时
使用仓库演练脚本。

## 升级

1. 拉出或检查释放装置。
2. 查看 `.env.example` 是否有新变量并更新 `.env`。
3.运行`bash scripts/deploy-release-validation.sh`。
4. 为MySQL和MinIO创建备份。
5. 构建镜像：

```bash
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
```

6.按顺序启动：

```bash
docker compose -f deploy/docker-compose.yml up -d mysql redis minio minio-bootstrap
docker compose -f deploy/docker-compose.yml up -d backend-api backend-worker frontend
```

7. 验证运行状况、日志、登录、初始化管理状态、上传、任务创建、SSE
   任务更新和下载。

## 回滚

回滚必须保持MySQL和MinIO一致：

1. 停止写入或将服务置于维护状态。
2. 查看之前的版本并恢复匹配的`.env`。
3. 如果发生过迁移或任务写入，请使用运维人员认可的工具，从同一个
   一致性时间点恢复匹配的 MySQL 和 MinIO 备份。
4. 重建并启动：

```bash
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
```

5. 在重新开放流量之前运行运行状况和代理检查。

## 日志故障排除

常用命令：

```bash
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml logs --tail=200 backend-api
docker compose -f deploy/docker-compose.yml logs --tail=200 backend-worker
docker compose -f deploy/docker-compose.yml logs --tail=200 frontend
docker compose -f deploy/docker-compose.yml logs --tail=200 mysql redis minio minio-bootstrap
```

要检查的症状：

- API不健康：检查MySQL、Redis、MinIO健康状况和存储桶引导初始化日志。
- Worker 不健康：检查 `WORKER_HEALTHCHECK_FILE`、队列设置和
  `backend-worker` 日志。
- 前端 `/api/` 失败：验证 `backend-api` 健康状态和 Nginx 代理配置。
- SSE 停顿：验证反向代理缓冲已禁用，并且读取超时时间足够长。
- 上传失败：验证上传限制、MIME 类型、图片尺寸、MinIO 存储桶和
  后端授权日志。

日志不得包含完整的 API 密钥、授权标头、Cookie、密码、
或图像 base64 数据。将任何此类日志行视为安全事件。

## 清理

停止容器但保留命名卷：

```bash
docker compose -f deploy/docker-compose.yml down --remove-orphans
```

停止容器并删除 Compose 管理的数据卷：

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

只在一次性本地验证栈中，或确认备份后使用 `down -v`。该命令会
删除这个 Compose 项目的 MySQL、Redis 和 MinIO 数据卷。
