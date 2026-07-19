# 部署说明

一期部署在单台服务器 `/root/works/tanban`：

- `tanban-api`：Docker 容器，主机网络监听 `127.0.0.1:18090`。
- MySQL 5.7：使用服务器现有 `tanban` 数据库。
- 平台管理端：静态文件发布到 `/www/wwwroot/tanban-platform`。
- 商户后台：静态文件发布到 `/www/wwwroot/tanban-merchant`。
- Nginx：继续由宝塔安装的 Nginx 管理三个域名及 TLS。
- 缓存：一期使用 API 进程内缓存。缓存接口保留 Redis 实现边界，但生产配置不启用 Redis。

## 域名

| 域名 | 服务 |
| --- | --- |
| `tbapi.666qwe.cn` | REST API、支付回调、健康检查 |
| `tbadmin.666qwe.cn` | SaaS 平台管理端 |
| `mysales.666qwe.cn` | 商户运营后台 |

## 生产配置

复制 `.env.production.example` 为 `.env.production`，填写数据库口令、JWT 密钥和首次启动账号。`.env.production` 已被 Git 忽略，不得提交，并必须限制为仅 root 可读：

```bash
cp .env.production.example .env.production
chmod 600 .env.production
```

示例文件默认启用 Mock 支付、公开 Mock 确认和演示数据，只适合联调。真实营业前至少需要切换为已经验签联调通过的真实 Provider，并设置：

```dotenv
TB_ALLOW_MOCK_CONFIRMATION=false
TB_SEED_DEMO=false
```

首次账号已经创建且密码已在后台修改后，应清空 `TB_BOOTSTRAP_ADMIN_PASSWORD`、`TB_DEMO_MERCHANT_PASSWORD` 等一次性明文，避免它们长期留在容器环境中。

## 首次 TLS/ACME 引导

正式 vhost 直接引用三个 Certbot 证书，所以全新服务器不能先运行正式部署脚本。先确认 DNS 已指向服务器、80/443 端口可访问，再安装临时 HTTP vhost：

```bash
cd /root/works/tanban
install -d -m 0755 \
  /www/wwwroot/tanban-api-acme \
  /www/wwwroot/tanban-platform \
  /www/wwwroot/tanban-merchant
install -m 0644 infra/nginx/acme-bootstrap.conf \
  /www/server/panel/vhost/nginx/tanban-acme-bootstrap.conf
nginx -t
nginx -s reload
```

分别申请三个独立证书；将示例邮箱换成实际运维邮箱：

```bash
certbot certonly --webroot --non-interactive --agree-tos \
  --email ops@example.com \
  -w /www/wwwroot/tanban-api-acme -d tbapi.666qwe.cn
certbot certonly --webroot --non-interactive --agree-tos \
  --email ops@example.com \
  -w /www/wwwroot/tanban-platform -d tbadmin.666qwe.cn
certbot certonly --webroot --non-interactive --agree-tos \
  --email ops@example.com \
  -w /www/wwwroot/tanban-merchant -d mysales.666qwe.cn
```

安装续签后的 Nginx reload hook，并启用当前发行版提供的 Certbot timer（本服务器单元名为 `certbot-renew.timer`）：

```bash
install -d -m 0755 /etc/letsencrypt/renewal-hooks/deploy
install -m 0755 /dev/stdin /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh <<'HOOK'
#!/usr/bin/env bash
nginx -t && nginx -s reload
HOOK
systemctl enable --now certbot-renew.timer
systemctl list-timers --all | grep -i certbot
```

证书存在后再执行正式部署；部署脚本会用 HTTPS vhost 替换并删除临时引导配置。

## MySQL 备份

[`scripts/mysql-backup.sh`](../scripts/mysql-backup.sh) 从 root-only 的 `.env.production` 读取 `TB_DATABASE_DSN`，为 `mysqldump` 创建临时 `0600` client option file，因此密码不会出现在命令行参数中。备份经 `gzip -t` 和 dump 完成标记双重校验后才原子改名，最终文件权限为 `0600`。默认目录是 `/var/backups/tanban/mysql`，默认保留 14 天。

先手工验证一次：

```bash
cd /root/works/tanban
bash scripts/mysql-backup.sh
find /var/backups/tanban/mysql -maxdepth 1 -type f -name 'tanban-*.sql.gz' -ls
gzip -t /var/backups/tanban/mysql/tanban-<label>-<timestamp>.sql.gz
```

安装每日 03:17 的 cron：

```bash
install -m 0644 infra/cron/tanban-mysql-backup /etc/cron.d/tanban-mysql-backup
```

用 `journalctl -t tanban-mysql-backup` 检查运行结果。可通过 `BACKUP_DIR`、`BACKUP_RETENTION_DAYS` 和 `BACKUP_LOCK_TIMEOUT` 覆盖默认值。脚本自带文件锁，不会并发生成两份 dump。

每次正式部署也会先创建一份带发布号的已校验备份；备份失败会中止发布。该备份只保护数据库，不会让破坏性 DDL 自动回滚。

> 本机备份不等于容灾：服务器或磁盘整体损坏时，本机数据库和 `/var/backups` 可能同时丢失。真实营业前必须把备份加密同步到异机/对象存储，启用并归档 binlog，并至少完成一次隔离库恢复演练。

## 发布

首次部署：

```bash
cd /root/works/tanban
bash scripts/server-deploy.sh
```

后续升级仍执行同一脚本。数据库 migration 由 API 在启动前按版本串行执行。

部署脚本按以下顺序执行，任一步骤失败都会停止后续发布：

1. 校验 `.env.production` 中 `TB_HTTP_ADDR` 必须为 `127.0.0.1:18090`，避免 API 直接暴露到公网。
2. 在任何 API/数据库变更前创建并校验 MySQL 备份。
3. 给当前 API 镜像创建带发布号的回滚标签，再构建并启动新容器，持续检查 `http://127.0.0.1:18090/readyz`。只有数据库迁移完成且 API 就绪后才继续；新 API 未就绪时自动恢复上一镜像，并再次等待旧 API 就绪。
4. 构建两个 Web 前端，将产物写入带版本号的 release 目录，再通过符号链接切换生效。已有符号链接的后续发布是原子切换；第一次接管旧的实体目录时，脚本会先将旧目录改名保留。
5. 在临时 Nginx include 目录中独立执行配置预检，通过后才备份并安装正式 vhost。正式配置还会再次执行 `nginx -t`，成功后才 reload。
6. 发布成功后清理旧的静态 release、Nginx 配置备份和 API rollback image，默认各保留最近 5 个。

Nginx 备份默认保存在 `/var/backups/tanban/nginx/<release-id>/`。如果正式配置检查或 reload 失败，脚本会恢复旧的 vhost 和静态目录，并重新加载旧配置。可通过 `NGINX_BACKUP_ROOT` 更换备份根目录；用 `RELEASE_RETENTION_COUNT=10` 可调整发布产物保留数。

API 就绪等待时间默认为 180 秒，可在迁移耗时较长时临时调整：

```bash
API_READY_TIMEOUT=300 bash scripts/server-deploy.sh
```

容器使用只读根文件系统、移除全部 Linux capabilities，并限制为 2 CPU、512 MiB 内存和 256 个进程。若实际负载超过这一上限，应先通过监控确认瓶颈，再在 `infra/deploy/docker-compose.prod.yml` 中调整。

当前是单实例原地替换：API 容器启动期间可能出现短暂 502，新 API 与旧前端也会短暂并存。所有接口和 migration 必须向后兼容；需要真正无损发布时再增加第二实例和流量切换。

## Migration 约束

MySQL 5.7 的 DDL 即使写在事务中也会隐式提交，`.down.sql` 不会被部署脚本自动执行。所有生产 migration 必须遵循 expand/contract：

1. **Expand**：先添加允许旧代码继续运行的表、可空列、带默认值列或新索引；不得在本步删除/重命名旧列，也不得直接收窄类型。
2. 发布同时兼容旧、新 schema 的代码；大数据回填使用独立、可恢复、限速的任务，不能把长回填塞进 API 启动。
3. 验证新代码稳定、备份可恢复并超过回滚窗口后，再在另一次发布执行 **Contract**，删除已经无人读取的旧结构。
4. 每个 migration 都需评估 MySQL 5.7 元数据锁、表重建时间和磁盘空间；大表 DDL 应使用在线变更方案和维护窗口。
5. 如果新 schema 不兼容旧镜像，禁止依赖自动镜像回滚；应先停止发布并制定数据库恢复/前向修复步骤。

## 成功发布后的人工回滚

脚本内自动回滚只处理“当前发布过程失败”。若部署已经成功、随后才发现业务缺陷，先确认坏版本发布号、目标旧静态版本和对应数据库 migration 是否向后兼容。默认只保留 5 个版本，超出保留窗口不能使用此流程。

以下示例把 `BAD_RELEASE_ID` 和 `PREVIOUS_RELEASE_ID` 换成实际目录名；`rollback-$BAD_RELEASE_ID` 保存的是发布坏版本前的 API 镜像：

```bash
cd /root/works/tanban
BAD_RELEASE_ID=20260719T130213Z-3526266
PREVIOUS_RELEASE_ID=20260719T125320Z-3520991

docker image inspect "tanban-api:rollback-$BAD_RELEASE_ID"
test -d "/www/wwwroot/tanban-platform.releases/$PREVIOUS_RELEASE_ID"
test -d "/www/wwwroot/tanban-merchant.releases/$PREVIOUS_RELEASE_ID"
test -d "/var/backups/tanban/nginx/$BAD_RELEASE_ID"

docker tag "tanban-api:rollback-$BAD_RELEASE_ID" tanban-api:local
docker compose -f infra/deploy/docker-compose.prod.yml up -d --no-build api
timeout 180 bash -c 'until curl --fail --silent --max-time 3 http://127.0.0.1:18090/readyz >/dev/null; do sleep 2; done'

ln -s "/www/wwwroot/tanban-platform.releases/$PREVIOUS_RELEASE_ID" "/www/wwwroot/tanban-platform.rollback"
mv -Tf /www/wwwroot/tanban-platform.rollback /www/wwwroot/tanban-platform
ln -s "/www/wwwroot/tanban-merchant.releases/$PREVIOUS_RELEASE_ID" "/www/wwwroot/tanban-merchant.rollback"
mv -Tf /www/wwwroot/tanban-merchant.rollback /www/wwwroot/tanban-merchant

for name in tbapi.666qwe.cn.conf tbadmin.666qwe.cn.conf mysales.666qwe.cn.conf tanban-acme-bootstrap.conf; do
  backup="/var/backups/tanban/nginx/$BAD_RELEASE_ID/$name"
  target="/www/server/panel/vhost/nginx/$name"
  if test -e "$backup"; then
    cp -a -- "$backup" "$target"
  elif test -f "/var/backups/tanban/nginx/$BAD_RELEASE_ID/.missing-$name"; then
    rm -f -- "$target"
  fi
done
nginx -t
nginx -s reload
```

回滚后再次检查三个域名、登录、下单和订单状态。上述步骤**不会回滚数据库**；只在 migration 向后兼容时使用。若必须恢复数据库，应先停止写流量，使用已验证 dump/binlog 在隔离库演练并明确数据丢失窗口，不能直接在营业库盲目导入。

## TLS

Nginx 配置使用 Certbot 证书目录：

```text
/etc/letsencrypt/live/tbapi.666qwe.cn/
/etc/letsencrypt/live/tbadmin.666qwe.cn/
/etc/letsencrypt/live/mysales.666qwe.cn/
```

证书到期续签由服务器 Certbot timer 负责，deploy hook 会在续签成功后校验并 reload Nginx。应定期检查 `systemctl list-timers` 和 `journalctl -u certbot-renew.service`。小程序发布前必须确保 API HTTPS 证书有效，并在微信公众平台添加 request 合法域名。
