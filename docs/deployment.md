# 部署计划

## 中文部署说明入口

面向实际启动和部署的中文操作文档：

- `docs/mac-mini-m4-local-startup.md`：Mac mini M4 本机直接启动前端、后端 API、Worker，并连接共享本地 MySQL、Redis、MinIO。
- `docs/mac-mini-m4-docker-deployment.md`：Mac mini M4 完整 Docker Compose 部署验证。
- `docs/x86-server-docker-deployment.md`：X86 线上 Linux 服务器 Docker Compose 部署，包含 MySQL、Redis、MinIO 环境。

本文件保留为部署架构、验收标准和历史验证记录；具体操作优先阅读上述中文专项文档。

## 本地开发环境

日常开发和验证必须使用`docs/local-development.md`中记录的共享机器级服务。

- 在`127.0.0.1:3306`上使用`dev-mysql8`，而不是创建特定于项目的MySQL容器。
- 在`127.0.0.1:6379`上使用`dev-redis`，而不是创建特定于项目的Redis容器。
- 在`127.0.0.1:9000`上使用`dev-minio`，而不是创建特定于项目的MinIO容器。
- 不要commit本地服务此存储库。 驴留在全球本地开发文档中。

`deploy/docker-compose.yml` 是部署拓扑。它不是默认的常规开发环境。

如果特定于部署的验证启动项目 Compose 堆栈，请随后清理它，除非用户明确要求保留它：

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

## P21 可靠性强化后的当前状态

该存储库具有 Docker Compose 拓扑和可构建的 frontend/backend 图像，在 P9 发布准备工作后经过验证。在P12卖家工作流程和项目成员强化之后，R12重新验证了Compose配置。添加了P15安全回归`scripts/security-regression.sh`，它现在验证重点安全测试、前端禁止模式扫描、后端敏感标记扫描、前端`/api/`代理安全、Compose配置和空格检查。

平台功能工作仍应使用共享的本地开发服务进行日常开发，除非任务明确需要Compose部署验证。

当前验证状态：

- `docker compose -f deploy/docker-compose.yml config`通过。
- `docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend`通过。
- `docker compose -f deploy/docker-compose.yml up -d`在`mysql`、`redis`、`minio`、`backend-api`、`backend-worker`和`frontend`达到健康状态。
- 前端`/api/`代理达到`backend-api:8080`，并且前端Nginx配置不包含AIProvider或中继代理。
- 前端本地 lint、类型检查、测试和 R12 验证后构建通过。
- 后端`go test`、竞态测试、`go vet`、API构建和Worker在R12验证后构建通过。
- R12验证确认`docker compose -f deploy/docker-compose.yml config`在统一前端历史、卖家project/asset工作流程完善以及后端项目成员最后-`OWNER`强化后仍然通过。
- 共享本地`dev-mysql8`、`dev-redis`和`dev-minio`是预期的例行验证服务，并在R5中验证可访问。
- `scripts/real-provider-smoke.sh` 作为后端介导的真实 Provider 验证的可选手动冒烟测试入口存在。它的默认help/dry-run路径不会调用AIProvider或消耗积分。
- `scripts/prod-dry-run.sh`通过了安全默认和实际 Compose具有范围清理的演练模式；此后没有留下任何项目容器或数据卷。
- `deploy/nginx/amazon-ai-product-image-studio.conf.template`和`scripts/tls-reverse-proxy-check.sh`定义并验证主机TLS边缘。外部流量仅路由至环回前端`127.0.0.1:8080`。
- Compose和后端配置将browser/backendCSRF请求标头合约固定到`X-CSRF-Token`；部署不得覆盖它。
- 生产后端启动和生产试运行预检现在失败关闭，除非`CSRF_ENABLED=true`。
- 登录速率限制使用Redis与`AUTH_LOGIN_RATE_LIMIT_MAX_FAILURES`和`AUTH_LOGIN_RATE_LIMIT_WINDOW`；默认值是保守的，不会在 Redis 键中公开 email/IP。
- 普通用户验证码挑战使用Redis与`AUTH_CAPTCHA_ENABLED`和`AUTH_CAPTCHA_TTL`（默认`2m`）。 `AUTH_DEFAULT_TENANT_ID` 可以设置为特定于租户的登录入口点，因此浏览器永远不会要求用户输入租户 ID。
- `backend/cmd/provider-key-rotation` 为Provider 主密钥轮换提供默认安全的试运行和明确确认的事务应用路径。活跃的Provider凭据已重新加密；历史软删除 Provider 凭证残留在试运行中进行计数报告并在应用中进行加密擦除。
- `backend/cmd/provision-tenant` 为其他租户创建提供默认安全的试运行和明确确认的事务应用路径。
- `backend-api`映像捆绑了运维人员CLI二进制文件，因此Compose服务器可以通过作用域`docker compose run --rm --no-deps`命令运行它们，而无需主机Go工具链。
- `scripts/backup-restore-rehearsal.sh`通过了一个孤立的实际 Compose匹配MySQL/MinIO恢复和回滚演练与范围清理。
- 合并前端tenant/custom-role管理UI，前端测试工具链升级为`vitest@^4.1.8`；前端审计报告零漏洞。

已知的运行时注释：

- Compose 保留部署拓扑，而不是默认的常规开发环境。
- 前端容器必须继续仅代理`/api/`到`backend-api:8080`。
- 前端 Nginx 不得代理 AI Provider 流量。
- Compose堆栈包括所需存储桶的一次性`minio-bootstrap`服务。
- 未来影响发布的任务必须在声明发布准备就绪之前重新运行Composeconfig/build/up/healthcheck。
- 实施了Provider主密钥轮换和backup/restore演练。 在生产变更之前，运维人员仍必须执行默认安全检查和批准的目标环境程序。
- R20生产环境文件传播的部署后续，已脱敏健康故障日志，有界容器日志轮换，精确MinIO恢复语义，启动迁移序列化，Worker配额协调，Provider尝试分类账， Redis支持的登录速率限制、SSE弹性、会话撤销、Worker并发租约续订、Worker就绪门控和前端遗留清理由R21实现和验证。

R21部署验证结果：

- `bash scripts/deploy-release-validation.sh`通过，包括Compose配置，前端`/api/`和SSE代理检查，主机TLS模板静态检查，后端运维人员CLI镜像检查，镜像构建，以及委托安全回归。
- `bash scripts/deploy-release-validation.sh --up --down`已通过，包括实际 Compose启动、MySQL/Redis/MinIO/API/Worker/frontend运行状况、后端运行状况端点、前端根、前端`/api/`代理运行状况、SSE身份验证边界检查和范围清理。
- 清理后检查没有发现项目Compose容器，也没有`amazon-ai-product-image-studio_`Docker卷。

## 服务

Docker Compose 必须支持：

- `frontend`
- `backend-api`
- `backend-worker`
- `mysql`
- `redis`
- `minio`

所需主机服务：

- TLS 公共HTTPS路由的反向代理，使用`deploy/nginx/amazon-ai-product-image-studio.conf.template`或经`scripts/tls-reverse-proxy-check.sh`验证的等效配置。

## 目标 Compose 布局

```text
deploy/
  docker-compose.yml
  mysql/
    init/
  minio/
  nginx/
```

## 环境文件

根`.env.example`应该记录：

- MySQL数据库、用户和密码。
- Redis 地址和密码（如果启用）。
- MinIO 端点、访问密钥、秘密密钥和存储桶名称。
- JWT签名秘密。
- Cookie 域和安全模式。
- 允许的前端来源。
- API 密钥加密秘密。
- 上传限制。
- 并发限制。
- Provider 超时默认值。

从来没有commit真正的秘密。

P5 资产存储期望在执行上传之前配置的 MinIO 存储桶已存在。当前后端请求处理程序不会创建存储桶。部署或环境引导初始化必须创建或验证 `MINIO_BUCKET_ORIGINALS`、`MINIO_BUCKET_GENERATED` 和 `MINIO_BUCKET_THUMBNAILS`。

## 健康检查

所需的健康检查：

- MySQL 准备好了。
- Redis 平。
- MinIO 健康端点。
- 后端API`/healthz`。
- Worker 就绪文件由数据库门控，Redis和MinIO检查，并在`Worker.Run`退出时删除。
- 前端静态服务。

## 卷

需要持久化：

- MySQL数据。
- Redis启用持久性时的数据。
- MinIO 对象数据。

应用程序容器应该是无状态的。

## 启动顺序

API和Worker取决于MySQL、Redis和MinIO的健康状况。迁移应在 API/worker 提供流量之前运行。迁移机制可以是一次性容器或API启动门，但它必须是显式的。

## 路由

前端路线：

- 静态前端资源。
- SPA后备。
- `/api/*`代理到Compose网络内的`backend-api:8080`。

API路线：

- `/api/v1/*`到后端API。
- `/api/v1/events/*` 必须保留流式传输行为并避免缓冲。

禁止路由：

- 前端容器不得代理 OpenAI、Gemini、OpenAI兼容中继或自定义 AI Provider 流量。
- Nginx/static前端部署不得用作AI中继。

## P3 部署验收

仅当所有这些都从存储库根传递时，P3 才完成：

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
```

预期健康状况：

- `mysql`、`redis`和`minio`都很健康。
- `backend-api`通过`/healthz`保持健康。
- `backend-worker`仅在配置的准备文件存在时才健康； Worker 在依赖项检查通过后写入它，并在 `Worker.Run` 退出时删除它。
- `frontend` 健康并为应用程序提供服务。

P3 部署检查仅验证运行时接线。它不需要业务 API、数据库迁移、身份验证、任务执行、Provider调用或MinIO资产流来实现。

## 部署验证

最低限度检查：

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
```

然后验证：

- 前端负载。
- API健康恢复健康。
- Worker正在运行。
- MySQL、Redis和MinIO可到达。

本地部署验证后，删除项目特定的容器和卷，除非有意保留环境用于部署调试：

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

## P9 发布验证期望

`P9-DEPLOY-RELEASE-VALIDATION` 是特定于部署的，因此即使普通功能验证使用共享的本地开发服务，它也可能启动项目 Compose 堆栈。除非用户明确要求保留它，否则它必须随后清理堆栈。

所需检查：

- `docker compose -f deploy/docker-compose.yml config`
- `docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend`
- `docker compose -f deploy/docker-compose.yml up -d`
- `docker compose -f deploy/docker-compose.yml ps`
- API 已发布或内部路由的运行状况，具体取决于 Compose 拓扑。
- 前端仅向构建的应用程序和代理`/api/`提供服务`backend-api`。
- Worker进程保持运行并报告配置的health/readiness信号。
- MySQL、Redis和MinIO服务健康。
- MinIO 所需的存储桶已创建，或者运行手册清楚地记录了引导初始化步骤。
- SSE 路由保留流式标头，并且不被 frontend/reverse 代理路径缓冲。
- 部署示例中不使用生产占位符机密。

文档输出：

- 仅使用占位符更新`.env.example`，而不是真正的本地凭据。
- 使用实际的 P9 验证结果和剩余的操作说明更新此部署计划。
- 添加或更新发布运行手册（如果尚不存在），包括初始化管理流程、bucket/bootstrap注释、backup/restore注释和清理命令。

## P9 部署验证结果

验证日期：2026年5月20日。

除非另有说明，从存储库根目录执行的实际命令：

```bash
cd backend && go test ./...
cd backend && go test -race ./...
cd backend && go vet ./...
cd backend && go build ./cmd/api ./cmd/worker
cd frontend && npm ci
cd frontend && npm run lint
cd frontend && npm run type-check
cd frontend && npm run test
cd frontend && npm run build
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml logs --tail=120 backend-api
docker compose -f deploy/docker-compose.yml logs --tail=120 backend-worker
docker compose -f deploy/docker-compose.yml logs --tail=120 frontend
docker compose -f deploy/docker-compose.yml run --rm --no-deps minio-bootstrap
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

观察结果：

- 后端测试、竞态测试、审查和 API/Worker 构建已通过。
- 使用 `npm ci` 安装依赖项后，前端 lint、类型检查、测试和构建已通过。
- Compose 配置已通过。
- Compose 镜像构建已通过`backend-api`、`backend-worker`和`frontend`。
- `docker compose up -d` 成功启动堆栈。
- `mysql`、`redis`、`minio`、`backend-api`、`backend-worker`和`frontend`已达到`healthy`。
- `minio-bootstrap`退出，代码为`0`，并创建或验证了`product-originals`、`product-generated`和`product-thumbnails`。
- `http://127.0.0.1:8081/healthz`的直接API健康状况返回`database`、`redis`和`minio`为`ok`。
- `http://127.0.0.1:8080/api/v1/healthz` 的前端代理健康状况返回相同的后端API健康负载，证明`/api/`路由到`backend-api:8080`。
- 前端根和深层SPA路线均返回`200 text/html`；构建的JS和CSS资产返回了`200`，并且没有被SPA后备吞没。
- 运行时 Nginx 配置仅显示`/api/v1/events/`和`/api/`代理位置，两者都针对`backend-api:8080`；不存在 OpenAI、Gemini、自定义 Provider 或 AI 中继代理。
- SSE代理位置具有`proxy_buffering off`、禁用缓存、长read/send超时和`X-Accel-Buffering: no`。
- 当使用`APP_ENV=production`运行时，API和Worker都拒绝了占位符`JWT_SIGNING_SECRET`和占位符`API_KEY_ENCRYPTION_KEY`。
- 清理已完成，`docker compose -f deploy/docker-compose.yml down -v --remove-orphans`；后续检查显示没有遗留任何项目容器或项目卷。

操作注意事项：

- `.env.example` 仅包含占位符。不要将其原封不动地用于登台或生产。
- Compose堆栈现在包含一个可重复的一次性`minio-bootstrap`服务，使用`mc mb --ignore-existing`来获取所需的存储桶。
- 当前 Redis 7.4 日志可以包含 go-redis `maintnotifications` 后备警告。它不会影响验证期间的健康检查或Worker/API操作。

## P15 部署运行手册结果

验证日期：2026年5月26日。

`P15-DEPLOY-RUNBOOK-FINAL`已审核并合并。它保持了公共部署合同的稳定性，并增加了可重复的运维人员验证。

添加输出：

- `scripts/deploy-release-validation.sh`与`--help`，安全默认检查，显式`--up`，并通过`--down`进行清理。
- `deploy/RUNBOOK.md`涵盖先决条件，`.env`仅使用占位符设置，生产秘密替换，启动，运行状况检查，init-admin，MinIObucket/bootstrap，SSE代理行为，backup/restore， upgrade/rollback，日志检查和清理。

实际检查通过：

- `bash scripts/deploy-release-validation.sh --help`
- `bash scripts/deploy-release-validation.sh`
- `bash scripts/deploy-release-validation.sh --up --down`
- `bash scripts/security-regression.sh --help`
- `docker compose -f deploy/docker-compose.yml config`
- `docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend`
- 完整的前端 lint、类型检查、测试和构建
- 完整的后端测试、竞态测试、审查和 API/Worker 构建
- `git diff --check main...HEAD`

实际 Compose验证已确认：

- `mysql`、`redis`、`minio`、`backend-api`、`backend-worker`和`frontend`已达到healthy/running状态。
- `minio-bootstrap`成功完成。
- 后端`/healthz`和`/api/v1/healthz`返回HTTP200。
- 前端根返回HTTP200。
- 前端`/api/v1/healthz`代理到后端并返回HTTP200。
- 前端`/api/v1/events/tasks`到达后端身份验证边界并返回HTTP401。
- 使用`docker compose -f deploy/docker-compose.yml down -v --remove-orphans`清理已删除的项目容器和卷。

后续状态：

- P16添加了清理陷阱，因此失败或中断的`--up --down`运行仍会尝试自动Compose清理。
- P16通过Worker维护过程添加了后端数据库日志保留。 运维人员仍然在后端`logRetention`设置之外管理容器stdout/stderr和外部日志聚合保留。
- P16添加了后端生成的资源缩略图，用于新的参考上传和Worker输出。 运维人员必须保持配置的缩略图桶和originals/generated桶可用；对象访问仍然要经过后端授权。

## R15 部署准备结果

验证日期：2026年5月26日。

R15 在合并了 P15 切片之后，在最新的 `main` 上重新运行了部署就绪门。 `scripts/deploy-release-validation.sh`、实时`scripts/deploy-release-validation.sh --up --down`、完整的前端和后端回归、Docker Compose配置和空格检查均已通过。后续检查确认清理后没有留下 `amazon-ai-product-image-studio` Compose 容器或数据卷。

## P16 部署脚本强化计划

`P16-DEPLOY-SCRIPT-HARDENING`是继R15之后的第一个稳定生产任务。

所需行为：

- 当实时验证失败、脚本错误或进程收到 SIGINT/SIGTERM. 时，`scripts/deploy-release-validation.sh --up --down` 必须尝试清理
- 没有`--down`的`--up`必须保持当前运维人员检查行为并使堆栈保持运行。
- 没有`--up`的`--down`必须保持仅清理状态。
- 默认验证不得启动或删除 Compose 堆栈。
- 清理范围必须保持在`deploy/docker-compose.yml`范围内，并且不得使用广泛的Docker修剪命令。
- 脚本级测试应尽可能使用假的 Docker 命令，以便覆盖清理陷阱故障路径，而不依赖于真正的基础设施故障。

## P16 部署脚本强化结果

验证日期：2026年5月26日。

`P16-DEPLOY-SCRIPT-HARDENING`已审核并合并。部署验证脚本现在具有 `--up --down` 的范围清理陷阱，因此实时验证失败、脚本错误、SIGINT 或 SIGTERM 仍会尝试项目 Compose 清理。没有`--down`的`--up`保持运维人员检查行为，`--down`单独保持仅清理，默​​认验证仍然不会启动或删除Compose堆栈。

添加输出：

- `scripts/deploy-release-validation-test.sh`，一个用于部署脚本清理行为的假命令 shell 回归套件。

实际检查通过：

- `bash -n scripts/deploy-release-validation.sh`
- `bash -n scripts/deploy-release-validation-test.sh`
- `bash scripts/deploy-release-validation.sh --help`
- `bash scripts/deploy-release-validation-test.sh`
- `bash scripts/security-regression.sh`
- `bash scripts/deploy-release-validation.sh`
- `bash scripts/deploy-release-validation.sh --up --down`
- `docker compose -f deploy/docker-compose.yml ps -a`
- `docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true`
- `docker compose -f deploy/docker-compose.yml config`
- `git diff --check main...HEAD`

## P16 后端日志保留结果

验证日期：2026年5月27日。

`P16-BE-LOG-RETENTION` 已审核、修复并合并。后端`logRetention`仅涵盖数据库支持的`operation_logs`、`api_call_logs`和终端任务`task_events`。 Worker维护循环消耗活动租户设置，应用有界批量清理，保留SSE/recovery的非终端任务事件，并记录已脱敏聚合审计元数据。容器stdout/stderr、主机日志和外部日志聚合保留仍由deployment/operator负责。

## R16 生产启动强化结果

R16审查了合并部署脚本强化、后端日志保留和后端缩略图策略后的完整P16范围。未发现阻塞部署问题。

验证通过：

- `bash scripts/deploy-release-validation-test.sh`
- `bash scripts/security-regression.sh`
- `bash scripts/deploy-release-validation.sh`
- `bash scripts/deploy-release-validation.sh --up --down`
- `docker compose -f deploy/docker-compose.yml ps -a`
- `docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true`
- `docker compose -f deploy/docker-compose.yml config`

实际 Compose运行已验证MySQL、Redis、MinIO、MinIO引导初始化、后端API、后端Worker和前端健康检查；后端`/healthz`；前端`/api/`代理健康； SSE 身份验证边界路由；以及项目容器和卷的清理。

实时 Compose 验证确认堆栈达到 healthy/running 状态，前端 `/api/` 和 SSE 身份验证边界代理检查已通过，清理已完成，后续检查显示没有留下任何项目容器或项目卷。

## P18 可选真实 Provider 冒烟测试结果

验证日期：2026年5月29日。

`P18-E2E-REAL-PROVIDER-SMOKE`已审核并合并。它补充说：

- `scripts/real-provider-smoke.sh`，一个手动选择的冒烟测试脚本，用于后端介导的真实Provider路径。
- `scripts/real-provider-smoke-test.sh`，一个用于脚本保护机制和编辑的假命令回归套件。
- `deploy/RUNBOOK.md` 中有一个简短的可选冒烟测试章节。

安全性能：

- 没有参数，`--help`仅打印用法。
- `--dry-run`验证本地保护机制，并且不调用任何API。
- `--run`需要`REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS`、必需的环境变量、有界输出计数、有界超时和平台`/api/v1`API基础。
- 该脚本拒绝直接 AI Provider API 基地，例如 OpenAI 或 Google API 主机。
- 该脚本使用临时 cookie jar 并跟踪临时 payload/output 文件，然后在退出时删除它们。
- 脚本输出不会打印完整的 Provider API 键、授权标头、Cookie、CSRF 令牌、JWT、图像 base64、对象键、存储桶名称、签名 URL 或原始 Provider 响应。
- 默认发布验证和安全回归脚本仍然不会调用真实的AIProvider。

执行的检查：

```bash
bash scripts/real-provider-smoke.sh --help
bash scripts/real-provider-smoke.sh --dry-run
bash scripts/real-provider-smoke.sh --run
bash scripts/real-provider-smoke-test.sh
bash scripts/deploy-release-validation-test.sh
bash scripts/deploy-release-validation.sh --help
bash scripts/deploy-release-validation.sh
bash scripts/security-regression.sh
docker compose -f deploy/docker-compose.yml config
git diff --check main...HEAD
```

除了普通的 `--run` 防护检查之外，所有检查均已通过，该检查在任何 API 调用之前故意失败，因为缺少确认变量。自动验证期间没有执行真正的Provider调用。
