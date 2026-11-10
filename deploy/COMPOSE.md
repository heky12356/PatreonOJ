# Docker Compose 部署

当前部署域名：`oj.example.com`。Cloudflare 提供访问端 HTTPS，Caddy 同时提供源站 HTTP 80 与 HTTPS 443。443 用于兼容 Cloudflare 当前的 HTTPS 回源；证书由 Caddy 自动申请和续期，无需手工管理。

## 服务与数据

`compose.yaml` 从源码构建四个镜像：

| 服务 | 构建内容 | 持久化 |
| --- | --- | --- |
| web | Node 构建 React，Caddy 提供静态文件及 `/api`、`/oss` 代理 | `patreonoj_caddy_data`（证书与账户） |
| api | Go 原生 Linux/CGO 构建、单元测试、API 与管理员命令 | `patreonoj_database`，SQLite |
| oss | 固定 MinIO 源码版本构建 | `patreonoj_objects` |
| judge | 固定 go-judge v1.14.0，包含 Go/C++/Java/Python 编译运行环境 | 临时共享内存，自动清理编译缓存 |

API、对象存储和判题服务只连接内部网络，不发布宿主端口。API 不挂载 Docker socket；判题服务使用独立的特权容器和 cgroup，要求资源隔离可用，禁用无 cgroup 的降级执行。日志有滚动上限，服务配置自动重启。

Go 标准库冷编译使用独立的 45 秒 CPU / 90 秒墙钟上限，判题客户端请求超时为 120 秒；题目本身的运行时限仍由题目配置决定。Java 配置文件在镜像内实体化，jar 临时文件放在工作目录，避免沙箱符号链接缺失或跨挂载移动失败。

## 目录和凭据

两个项目必须为同级目录，前端目录名为 `yjls`：

```text
/opt/patreonoj/
  releases/<release>/PatreonOJ/
  releases/<release>/yjls/
  current -> releases/<release>
  shared/compose.env
  backups/
```

初次部署从 `compose.env.example` 创建 `/opt/patreonoj/shared/compose.env`，权限设为 600，填写域名、发布标识及两个不同的随机密钥。正式凭据仅保存在服务器，不提交 Git。

首次发布使用源码快照，不包含 `.git`。后续版本可从后端 GitHub 的 `master` 和前端的 `labVersion` 分支取新快照，放入新的同级 `PatreonOJ`、`yjls` 目录；不要在当前运行目录覆盖私有环境文件或删除持久卷。

`SITE_DOMAIN` 为 Cloudflare 已代理的域名；公网链接和浏览器 Cookie 使用 HTTPS。Cloudflare 可使用 Full / Full (strict) 回源，Caddy 通过 HTTP-01 验证域名并申请证书；不使用会被 Cloudflare 截获的 TLS-ALPN 验证。证书与 ACME 账户保存在 `patreonoj_caddy_data` 卷中。Caddy 只信任 Cloudflare 官方地址范围的访客 IP 头，并将确认后的客户端 IP 传给 API，避免整站用户共用代理 IP 限流。

若出现 521，先核对 Cloudflare 回源端口：只监听 80 无法接收 HTTPS 回源。本次排查已抓到 Cloudflare 对源站 443 的连接请求，因此保留 80 并补齐自动 HTTPS 入口。80 不强制跳转，避免切换 Cloudflare 模式时出现回源重定向循环。

## 构建和启动

安装 Docker Engine、Buildx 和 Compose v2。进入本次后端发布目录：

```bash
cd /opt/patreonoj/current/PatreonOJ
bash deploy/compose.sh config --quiet
# 2 核 / 4 GB 主机逐个服务构建，避免 Bake 多目标并行编译。
for service in api web judge oss; do
  bash deploy/compose.sh --progress plain build "$service"
done
bash deploy/compose.sh up -d --wait --wait-timeout 180
bash deploy/compose.sh ps
curl -fsS http://127.0.0.1/api/healthz
curl -fsS https://oj.example.com/api/healthz
```

没有 `current` 链接时，在首次发布目录执行相同命令，确认健康后创建链接。镜像在服务器构建；后端 Dockerfile 会执行 Linux 单元测试，前端会执行 Node 测试和生产构建。配置校验使用 `--quiet`，避免把展开后的真实密钥输出到终端。

Cloudflare 域名路径请求不再带构建时后端地址：浏览器统一请求 `/api` 与 `/oss`。`public_address` 使用访问端 HTTPS 域名，Caddy 保留 Host 并去掉 `/oss` 前缀，确保 S3 预签名上传正确。

## 首次管理员

先注册账号，再运行：

```bash
bash deploy/compose.sh exec -T api /app/patreonoj-admin -username YOUR_USERNAME
```

重新登录刷新管理权限。部署验收账号使用随机密码，凭据若由部署脚本生成，应放在 `/opt/patreonoj/shared/` 下权限为 600 的文件中。不要执行会重建数据库的 `seed_demo`。

## 维护和回滚

```bash
bash deploy/compose.sh ps
bash deploy/compose.sh logs --tail=100 api
bash deploy/compose.sh logs --tail=100 judge
bash deploy/compose.sh restart api
```

升级前备份 SQLite 数据卷、对象存储卷和私有环境文件，并记录当前发布目录和镜像标签。备份数据卷时先停止 API 写入，备份对象存储时同时停止 oss，以取得一致快照。不要使用 `down -v` 或删除持久卷。

新版本放在新发布目录，沿用同一 Compose 项目名和两个数据卷，保持原有随机密钥。更新 `RELEASE_TAG` 后依次构建，再执行 `up -d --wait`；完成验收后切换 `current` 链接。回滚前确认数据库迁移兼容，必要时恢复配套快照。

验收应覆盖：HTTPS 域名首页、健康检查、注册登录与注销、头像/预签名上传、管理员创建题目和测试点、四种语言正确代码通过、错误代码不通过、隐藏测试输出遮蔽，以及重启后数据保留。

2026-11-10 已完成公网验收：首页与 API 健康检查正常；管理员登录、资料更新、匿名邮箱遮蔽、签名上传、公开头像读取、Python/C++/Java/Go 判题和旧会话注销均通过。初始管理员和 A+B 示例题由 `smoke.py` 创建，凭据保存在服务器 `/opt/patreonoj/shared/admin-credentials.json`，权限为 600，不写入日志或仓库。
