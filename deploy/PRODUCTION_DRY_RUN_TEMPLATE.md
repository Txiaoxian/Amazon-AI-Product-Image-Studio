# 生产试运行证据模板

使用此模板进行 R18 Go/No-Go 审核。只保留已脱敏阶段
结果。不要附加 env 文件、转储、秘密、Provider 响应、图像
输出、存储桶名称、对象密钥、签名 URL 或服务。

## 发布候选版本

- commit：
- 运维人员：
- 环境：
- 日期：

## 命令

默认不产生费用的演练：

```bash
bash scripts/prod-dry-run.sh
```

显式生产环境预检，使用外部现有的受限文件
存储库：

```bash
bash scripts/prod-dry-run.sh \
  --production-env-file /secure/runtime/production.env
```

可选的实时 Compose 演练和范围清理：

```bash
bash scripts/prod-dry-run.sh --live-compose
docker compose -f deploy/docker-compose.yml ps -a
docker volume ls --format '{{.Name}}' | rg '^amazon-ai-product-image-studio_' || true
```

## 已脱敏证据

|检查 |结果 | 已脱敏注|
| --- | --- | --- |
|生产环境预检| PASS / FAIL / NOT RUN |决不能在此处复制值。 |
|部署发布验证 | PASS / FAIL | |
|安全回归| PASS / FAIL | |
| 真实 Provider 冒烟测试保护机制试运行 | PASS / FAIL | |
| Backup/restore演练保护机制试运行 | PASS / FAIL | |
| Compose 配置验证 | PASS / FAIL | |
|活Compose健康之路 ​​| PASS / FAIL / NOT RUN | |
|实际 Compose清理| PASS / FAIL / NOT RUN |仅记录容器/数据卷不存在。 |
|可选真实Provider冒烟测试| PASS / FAIL / NOT RUN |手动计费步骤； 仅已脱敏状态。 |

## 备份与恢复演练

使用 [PRODUCTION_BACKUP_RESTORE_TEMPLATE.md](./PRODUCTION_BACKUP_RESTORE_TEMPLATE.md)
为隔离现场演练和生产运维人员提供程序证据。

- [ ] MySQL 和 MinIO 备份被视为一个一致性点。
- [ ] 备份目标位于存储库之外并且访问受到限制。
- [ ] 恢复演练前停止或隔离的目标。
- [ ] 恢复过程使用匹配的 MySQL 和 MinIO 备份集。
- [ ] 发布验证在恢复后重新开放流量之前运行。
- [ ] 未附加转储、对象列表、对象密钥、签名URL或附件。

## 回滚演练

- [ ] 先前发布的commit或产物已被识别。
- [ ] 无需复制值即可识别匹配的运行时配置位置。
- [ ] 已识别维护或写停止程序。
- [ ] 识别出匹配的MySQL和MinIO还原点。
- [ ] 分配运行状况、前端代理、SSE边界、登录、上传、任务和下载检查。

## Go / No-Go

Go only when:

- 所有必需的默认检查均通过。
- 显式生产环境预检目标运行时文件。
- 任何实际 Compose演练完成的清理工作，没有项目容器或
  留下的卷。
- 备份、恢复和回滚演练清单已完成。
- 可选真实Provider冒烟测试要么批准并通过，要么明确
  记录为未运行。

决定：GO / NO-GO

阻塞问题：
