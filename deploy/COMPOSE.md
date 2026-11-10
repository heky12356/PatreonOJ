# Docker Compose 部署

完整步骤见 [部署指南](../部署指南.md)，从 GitHub 克隆代码开始，使用同级的 `PatreonOJ`、`PatreonOJ-UI` 目录。

`compose.yaml` 构建和启动 `web`、`api`、`mysql`、`oss`、`judge` 五个服务。生产数据库为 MySQL 8.4，持久化到 `patreonoj_mysql_data`；对象存储与证书使用独立卷。仅 Web 发布 80/443。

按指南创建受保护的环境文件，填写自己的域名及随机凭据，再在后端目录执行：

```bash
bash deploy/compose.sh config --quiet
for service in mysql api web judge oss; do
  bash deploy/compose.sh --progress plain build "$service"
done
bash deploy/compose.sh up -d --wait --wait-timeout 240
bash deploy/compose.sh ps
```

管理员授权、验收、备份、升级、回滚和代理配置均以部署指南为准。升级保留原凭据和数据卷，不使用 `down -v`。
