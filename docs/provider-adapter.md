# Provider 适配器计划

## 目标

所有AI调用都必须经过后端Provider适配器。业务服务不得硬编码 OpenAI、Gemini 或中继特定的请求逻辑。

## Provider 类型

- `OPENAI`：官方OpenAI图像API。
- `GEMINI`：官方Gemini图像API。
- `OPENAI_COMPATIBLE`：OpenAI兼容中继或自定义AI网关。

## Provider 配置

Provider 唱片店：

- 显示名称。
- 类型。
- 基地URL。
- 加密的API密钥。
- API关键提示。
- 启用/禁用状态。
- 暂停。
- 并发限制。
- 可选标头或兼容性配置（如果稍后批准）。

API 密钥必须静态加密，并且永远不会完整返回到前端。

Provider超时政策：

- `timeout_seconds` 受后端 Provider API 限制，目前允许 `1..600` 秒。
- 十分钟超时仅适用于缓慢的图像生成调用。它们不会取代任务超时、Worker 租赁、取消、重试或 SSE 生命周期控制。
- 前端必须在超时字段附近以简体中文显示此说明。

Provider 主密钥轮换是仅限运维人员的维护工作流程。它必须使用具有安全试运行默认值和显式应用确认的后端 CLI。轮换必须在一个数据库事务中重新加密所有符合条件的活动 Provider 会计，保持活动 Provider 会计提示和会计更新时间戳不变，如果无法处理任何活动行，则完全回滚，从软删除的 Provider 行中加密擦除会计材料，并且永远不要打印明文密钥、加密的有效负载、提示、基础URL 或 tenant/object 详细信息。

Provider软删除必须加密擦除存储在同一写入路径中的凭据。删除的Provider记录可能会保留为metadata/audit历史记录，但在删除成功之前必须清除`encrypted_api_key`、`api_key_hint`和`api_key_updated_at`。在此策略之前创建的历史软删除行将由 Provider 密钥轮换应用工作流擦除。

目前的实施：

- `backend/cmd/provider-key-rotation`实施运维人员工作流程。
- 默认模式无需写入即可验证所有符合条件的行。 `--apply`还需要`PROVIDER_KEY_ROTATION_CONFIRM=I_UNDERSTAND_PROVIDER_KEY_ROTATION`。
- 该服务序列化数据库事务，锁定符合条件的 Provider 行，重新加密活动 Provider `encrypted_api_key` 值，并在任何活动行无法解密或重新加密时使完整轮换失败。
- 软删除的 Provider 行不会重新加密。如果任何已删除的行仍然包含加密的密钥材料或密钥元数据，则应用路径会将其清除，并且试运行路径会将其报告为已删除-Provider擦除候选者。
- 加密的有效负载密钥 ID 必须与解密密钥 ID 匹配。不匹配的密钥ID以失败方式关闭（fail closed）。
- 运维人员必须在批准的Provider写入维护时段内运行应用路径，然后使用新的活动密钥和密钥ID部署API和Worker。

## P6 管理边界

P6仅实施Provider/model管理和安全Provider测试。它不实现 generation/edit 执行、Redis 任务处理、输出资产创建或前端工作台后端化。

P6 必须生成这些后端基础：

- 租户范围ProviderCRUD，带有enable/disable和软删除。
- API密钥encryption/decryption服务仅供后端使用。
- 使用 `apiKeyHint` 屏蔽 Provider 响应并仅更新关键元数据。
- SSRF验证器在Providercreate/update之前和Providertest/use.之前使用
- 已脱敏Providertest/probe路径有超时、操作日志和已脱敏错误。
- 租户范围的模型功能CRUD，带有enable/disable和结构化功能验证。

P7在实现真正的Provider适配器generation/edit调用时将使用这些记录和助手。

当前P6Provider安全结果：

- 后端Provider安全基础已实施并合并。
- Provider 记录是租户范围内的，API 密钥存储为加密的有效负载。
- Provider 响应仅公开屏蔽的密钥元数据，并且从不返回加密的密钥材料。
- Provider测试仅在后端进行，写入已脱敏操作日志，而不创建任务、资产或使用记录。
- SSRF验证已针对Providersave/update/test实现，涵盖被阻止的主机名、被阻止的IP范围、不支持的方案、嵌入的凭据以及重定向到被阻止的目标。
- P7真实Provider适配器执行添加了所需的SSRF安全出站传输，并在真正generation/edit调用之前进行连接时IP验证。

当前P6模型能力结果：

- 后端模型能力管理实现并合并。
- 模型记录是租户范围内的，在同一租户中引用 Provider，并公开 generation/edit 功能、多引用支持、`n` 支持、最大输出计数、支持的大小、支持的质量、支持的输出格式、定价元数据和 enabled/disabled 状态。
- 功能和定价JSON在持久化之前经过验证。
- P7 Provider 适配器执行必须使用这些后端模型记录作为允许的任务参数的可信来源；它不得从前端常量推断允许的图像参数。
- 当前P7运行时使用稳定的`modelId`引用。 P18 通过序列化 Provider/model/default-setting 写入并拒绝模型写入路径中重复的同租户 Same-Provider 未删除 `model_name` 值来增强控制平面完整性。
- 实施并合并P14Provider/model生命周期策略：Provider删除被阻止，而任何未删除的同租户模型仍然引用Provider； Provider 当启用的链接模型存在时，禁用被阻止；模型create/update/enable拒绝禁用、删除或跨租户Provider；软删除模型不会阻止 Provider 删除。
- P21Provider 装备生命周期强化已实施：Provider删除立即加密擦除加密的密钥材料和密钥元数据，Provider主密钥轮换应用会清除任何历史软删除的Provider仍包含装备材料的行。

当前P6前端管理结果：

- 前端Provider/model管理已实施并合并。
- Provider API 密钥仅通过经过身份验证的后端 Provider API 提交，仅显示为屏蔽元数据，并在保存或模式关闭后从表单草稿中清除。
- P6前端未添加浏览器Provider直接调用、Provider授权标头、任务轮询或工作台生成后端。

P7运行时边界：

- `P7-BE-PROVIDER-ADAPTER-RUNTIME`是允许执行真实后端Providergeneration/edit调用的第一阶段。
- 运行时执行必须使用ProviderAdapter接口和后端模型能力表作为允许参数的可信来源。
- 运行时执行在`P7-BE-WORKER-QUEUE`合并可靠队列消耗、Worker状态处理、RedisSSE唤醒和fake/stub执行后开始。
- `frontend/src/providers/**`下的浏览器Provider适配器已在P8中删除，并且不得重新引入生产生成路径。

当前P7运行时结果：

- 为OpenAI、Gemini和OpenAI兼容Provider实现和合并真正的后端Provider适配器执行。
- 包含`referenceAssetIds`的业务层`IMAGE_GENERATION`任务仍然是公共API中的生成任务，但Worker将它们映射到Provider编辑操作，因此实际图像字节被发送到`/images/edits`。 OpenAI兼容的编辑请求对每个引用使用多部分`image[]`字段。
- 除了 save/use-time URL 验证之外，运行时执行还会在连接前使用 SSRF 安全传输来验证最终的出站拨号目标。
- 成功的运行时执行将generated/edited图像写入MinIO，创建资产和任务输出，记录usage/API调用日志，并发出output/usage/terminal任务事件。
- Provider错误和运行时元数据递归已脱敏。当解密的 Provider API 密钥同时显示为值和嵌套 JSON 映射密钥时，审核修复明确涵盖该密钥。
- 已知的Provider配额失败标准化为`PROVIDER_INSUFFICIENT_QUOTA`，忽略persisted/user-facing消息中的帐户余额，并且不会自动重试。
- 未提供给编辑者且不符合启发式规则的未知秘密仍处于自动检测之外；配置的 Provider API 密钥作为活动运行时路径中的已知秘密提供。

当前 P21 Provider 尝试账本结果：

- 运行时执行在调用 Provider 适配器运行时之前写入 `ATTEMPTING` API 调用分类帐。
- 成功、Provider失败、超时或取消后，同一账本行将被最终确定。
- 预写失败会阻止任何外部调用。 Finalize 失败使任务关闭失败，没有 output/usage success 副作用。
- 账本元数据使用运行时编辑器加上额外的Provider运行时元数据过滤器，该过滤器在持久化之前删除对象键、存储桶、MinIO URL、签名 URL、Authorization/Cookie/JWT/API-key 标记和图像 base64 字段。

P8前端迁移结果：

- 生产工作台仅消耗后端 Provider/model/task API。
- `frontend/src/providers/**` 不再存在于生产前端源代码树中。
- 浏览器设置可能会保留非敏感的 UI 首选项（如果仍然有用），但不得保留 Provider API 键或 Provider API URL。

## 适配器接口

后端应定义一个相当于以下内容的内部接口：

```go
type ImageProviderAdapter interface {
    Generate(ctx context.Context, req ImageGenerateRequest) (ImageGenerateResult, error)
    Edit(ctx context.Context, req ImageEditRequest) (ImageGenerateResult, error)
    Test(ctx context.Context, provider ProviderConfig) error
}
```

适配器输入使用标准化平台类型。适配器输出使用标准化图像字节、元数据、用法和Provider请求 ID。

P6最初仅暴露probe/test能力。 P7通过后端Provider适配器实现了`Generate`和`Edit`。业务代码必须继续依赖于接口，而不是具体的 Provider URL 或 SDK 调用。

Provider测试行为：

- OpenAI官方和OpenAI兼容Provider应在可用时使用轻量级身份验证端点，例如模型列表或配置的健康兼容路径。
- Gemini官方Provider应该使用轻量级的经过身份验证的端点（如果可用）。
- 如果 Provider 类型缺乏可靠的低成本探针，P6 可能会验证配置并执行最小的 HTTP 请求，以证明 DNS、TLS、SSRF 验证、超时和无需创建图像即可进行身份验证处理。
- 测试响应必须在持久性或API响应之前标准化并已脱敏。

## 模型能力配置

模型定义：

- 支持生成。
- 支持编辑。
- 支持多个参考图像。
- 支持`n`。
- 最大输出数量。
- 支持的尺寸。
- 支持的质量值。
- 支持的输出格式。
- 定价配置。
- 启用/禁用状态。

前端渲染来自模型能力响应的参数，而不是硬编码的 Provider 常量。

前端模型管理提供Provider特定的功能助手，而不是分辨率和比率值的叉积：

- OpenAI和OpenAI兼容`gpt-image-2`模型公开产品友好的纵横比列表，并使用官方生成质量值（`auto`、`low`、`medium`、`high`）。UI以简体中文显示“自动、低质量（1K）、中等质量（2K）、高质量（4K）”，并按官方顺序提交原始协议值。
- OpenAI适配器在生成和编辑请求之前结合比例与质量档位生成规范的`WIDTHxHEIGHT`，同时保持质量值不变地传给Provider。`low`使用1K基础尺寸，`medium`将宽高各放大2倍，`high`在保持比例的前提下使用不超过最长边`3840`且不超过`8,294,400`总像素的最大4K档尺寸；`auto`质量使用1K基础尺寸。
- Gemini图像模型将宽高比与输出分辨率分开存储（`1k`、`2k`、`4k`）。适配器将它们分别映射为`generationConfig.imageConfig.aspectRatio`和大写`imageSize`。

规范`gpt-image-2`比例与分辨率档位映射：

| 能力值 | 低质量（1K） | 中等质量（2K） | 高质量（4K） |
| --- | --- | --- | --- |
| `auto` | `auto` | `auto` | `auto` |
| `1:1` | `1024x1024` | `2048x2048` | `2880x2880` |
| `1.62:1` | `1296x800` | `2592x1600` | `3645x2250` |
| `2:3` | `1024x1536` | `2048x3072` | `2350x3525` |
| `3:2` | `1536x1024` | `3072x2048` | `3525x2350` |
| `3:4` | `1152x1536` | `2304x3072` | `2493x3324` |
| `4:3` | `1536x1152` | `3072x2304` | `3324x2493` |
| `4:5` | `1024x1280` | `2048x2560` | `2572x3215` |
| `5:4` | `1280x1024` | `2560x2048` | `3215x2572` |
| `9:16` | `864x1536` | `1728x3072` | `2160x3840` |
| `16:9` | `1536x864` | `3072x1728` | `3840x2160` |
| `21:9` | `1792x768` | `3584x1536` | `3836x1644` |

1K/2K/4K是平台档位名称，不是Provider枚举。4K档按比例使用Provider允许的最大尺寸，因此正方形等比例不会强制把单边扩展到4096。来自旧模型行的显式像素值不变地通过。该转换仅适用于`gpt-image-2`和日期为`gpt-image-2-*`的快照，因此其他OpenAI兼容的模型合约不会被自动重写。

模型编辑器和任务表单必须为这些语义使用匹配的人类可读标签。预设是管理员数据输入的便捷模板；持久模型功能JSON仍然具有权威性，运行时任务验证继续使用后端模型行而不是前端常量。

## 日志记录

每个Provider调用都会写下`api_call_logs`：

- ProviderID。
- 型号ID。
- 任务ID。
- 期间。
- 地位。
- HTTP可用时的状态。
- 已脱敏错误。
- 已脱敏请求和响应元数据。

切勿记录 API 密钥、授权标头、Cookie 或图像 base64。

## 大图像响应处理

OpenAI兼容的中继可能会返回大的`b64_json`有效负载，并且不需要压缩它们。 Worker 将 `data` 数组一次解码一张图像到本地临时文件中，从这些文件中进行验证，并将原始文件流式传输到 MinIO。它不能同时将完整的JSON响应和解码的图像字节加载到内存中。

- `PROVIDER_MAX_RESPONSE_SIZE_MB`限制完整的JSON响应；默认`1024`。
- `PROVIDER_MAX_OUTPUT_IMAGE_SIZE_MB` 限制一张解码图像；默认 `512` 并且不得超过响应限制。
- 这些限制独立于`UPLOAD_MAX_FILE_SIZE_MB`，适用于用户上传。
- 成功、失败、取消、超时或重试切换后，临时文件将被删除。
- 响应正文、Base64 值和临时文件路径绝不能保留在 API 调用日志中。

请参阅 [ADR-001](decisions/001-stream-provider-image-responses.md) 了解理由和被拒绝的替代方案。

## SSRF 保护

Provider 基本 URL 必须在保存时和使用前进行验证：

- HTTPS 仅默认情况下。
- 拒绝带有嵌入的凭据或不受支持的方案的 URL。
- 阻止本地主机和环回。
- 阻止私人IP范围。
- 阻止本地链路和多播范围。
- 阻止Docker-内部主机名。
- 验证 DNS 解析的 IP。
- 拒绝重定向到禁止的目标。
- 强制请求超时。
- 在出站呼叫之前重新检查已解析的 IP；仅在保存时进行验证是不够的。

推荐P6测试：

- 拒绝 `http://localhost`、`http://127.0.0.1`、`http://[::1]`、私人 RFC1918 地址、链接本地地址和 Docker 服务名称，例如 `backend-api`， `mysql`、`redis`和`minio`。
- 拒绝解析为阻止范围的 DNS 名称。
- 拒绝从允许的 URL 重定向到被阻止的目标。
- 接受语法上有效的 public HTTPS Provider URL。

额外P7要求：

- 用于实际 Provider generation/edit 调用的实际 HTTP 传输必须在连接时验证最终拨号目标。不要仅依赖于请求流中之前执行的 URL 验证。

## 删除前端 Provider 代码

旧浏览器Provider`frontend/src/providers/**`下的适配器文件已在P8中删除。如果以后的工作需要Provider行为引用，请使用git历史记录或后端Provider适配器测试；不要重新创建浏览器端 Provider 调用。
