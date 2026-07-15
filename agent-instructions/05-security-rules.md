# 安全规则

## 身份认证与会话

- 在 HttpOnly Cookie 中使用 JWT。
- 生产环境 Cookie 必须启用 Secure 和 SameSite 保护。
- 前端 JavaScript 不得读取认证令牌。
- 使用 Cookie 认证时，改变状态的端点必须增加 CSRF 防护。

## 授权

- 使用 RBAC 和租户隔离。
- 所有对象 ID 端点必须执行对象级授权。
- 访问项目和资产时必须检查项目成员身份。
- 仅管理员可用的 Provider、模型和系统设置 API 必须要求明确权限。

## 密钥

- API Key 存入 MySQL 前必须加密。
- 绝不能向前端完整返回 API Key。
- 响应只能包含脱敏后的密钥元数据，例如末 4 位和更新时间。
- 日志不得包含 API Key、Authorization 请求头、Cookie、密码或图片 base64 数据。

## Provider SSRF 防护

Provider `base_url` 在保存前和使用前都必须校验：

- 默认只允许 `https://`。
- 阻止 localhost、回环地址、私网网段、链路本地网段、组播网段和 Docker 内部主机名。
- 解析 DNS 并校验解析后的 IP。
- 阻止重定向到禁止目标。

## 上传安全

- 校验声明的 MIME 类型和文件魔数。
- 只允许 JPEG、PNG 和 WebP。
- 禁止 SVG。
- 强制执行文件大小、尺寸和像素总数限制。
- 上传文件存入 MinIO，不得存入 MySQL。
- 下载必须经过后端鉴权。

## 审计

对敏感操作和业务操作记录操作日志：

- 登录和退出。
- 用户、角色、Provider、模型和系统设置变更。
- 项目和资产变更。
- 任务创建、取消、重试、失败和完成。
