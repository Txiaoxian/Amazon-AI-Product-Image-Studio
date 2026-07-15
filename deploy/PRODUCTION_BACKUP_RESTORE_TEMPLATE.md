# 生产环境备份/恢复/回滚证据模板

使用此模板记录经运维人员审核的生产就绪证据。只保留已脱敏的
阶段结果。不得附加数据转储、存储列表、存储路径、凭据、密钥值、
证书、Provider 响应、图片输出或签名 URL。

仓库中的演练脚本仅限用于一次性隔离的 Compose 项目。
生产环境备份与恢复必须使用运维人员批准的运行时工具，
并采用有记录的一致性时间点。

## 发布候选版本

- commit：
- 运维人员：
- 环境：
- 日期：

## 隔离的 Compose 演练

默认仅检查保护机制：

```bash
bash scripts/backup-restore-rehearsal.sh
```

显式现场演练：

```bash
BACKUP_RESTORE_REHEARSAL_CONFIRM=I_UNDERSTAND_DATA_REPLACEMENT \
  bash scripts/backup-restore-rehearsal.sh --live
docker compose -f deploy/docker-compose.yml ps -a
docker volume ls --format '{{.Name}}' |
  rg '^amazon-ai-product-image-studio-backup-restore-rehearsal-' || true
```

## 已脱敏证据

| 检查项 | 结果 | 已脱敏说明 |
| --- | --- | --- |
| 默认仅保护机制模式 | PASS / FAIL | 不执行 Docker 命令或数据替换。 |
| 隔离 Compose 启动 | PASS / FAIL / NOT RUN | 只使用一次性演练项目。 |
| 匹配的 MySQL 与 MinIO 备份对 | PASS / FAIL / NOT RUN | 不得附加备份内容或列表。 |
| 测试数据销毁与恢复 | PASS / FAIL / NOT RUN | 只记录状态。 |
| 从匹配备份对执行回滚恢复 | PASS / FAIL / NOT RUN | 只记录状态。 |
| 范围内清理 | PASS / FAIL / NOT RUN | 只记录容器和数据卷不存在。 |
| 真实 Provider 调用 | NOT RUN | 演练不得调用 Provider。 |

## 生产环境运维流程

- [ ] 维护窗口或停止写入流程已获批准。
- [ ] 已确定获准使用的平台备份工具。
- [ ] MySQL 和 MinIO 在同一个有记录的一致性时间点完成备份。
- [ ] 备份存储已限制访问，并且位于 Compose 主机之外。
- [ ] 恢复目标已停止或被隔离。
- [ ] 选择匹配的 MySQL 和 MinIO 恢复集。
- [ ] 使用获准的平台恢复工具。
- [ ] 重新开放流量前，已安排发布健康检查、前端代理、SSE 边界、
      登录、上传、任务和下载检查。
- [ ] 已确定回滚发布产物和对应运行时配置的位置，且未复制配置值。

## 决策

决策：GO / NO-GO

阻塞问题：
