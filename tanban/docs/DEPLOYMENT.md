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

装修图片上传使用 API 容器的持久卷，生产配置必须明确写入：

```dotenv
TB_MEDIA_STORAGE_DIR=/var/lib/tanban/media
TB_MEDIA_PUBLIC_BASE_URL=https://tbapi.666qwe.cn/api/v1/public/media
```

`infra/deploy/docker-compose.prod.yml` 将该目录挂载到 `tanban_media` 命名卷，因此重建 API 容器不会丢图。数据库备份只保存素材元数据，不包含图片文件；正式营业前应把该卷与数据库备份按同一恢复点同步到对象存储，并演练“数据库 + 素材卷”一起恢复。

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
3. 给当前 API 镜像创建带发布号的回滚标签，再构建并启动新容器，持续检查 `http://127.0.0.1:18090/readyz`。只有数据库迁移完成且 API 就绪后才继续；新 API 未就绪时可恢复兼容扩展后 schema 的上一镜像。新 API 已通过就绪检查后，若后续前端或 Nginx 阶段失败，脚本只恢复静态文件和 Nginx，保留已兼容新 schema 的健康 API 并要求前向修复。
4. 安装两个 Web workspace 及其根级共享构建插件，构建前端，将产物写入带版本号的 release 目录，再通过符号链接切换生效。已有符号链接的后续发布是原子切换；第一次接管旧的实体目录时，脚本会先将旧目录改名保留。
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

`009_payment_attempt_identity` 会把“每订单一条支付记录”升级为追加式支付尝试并移除旧唯一键。Migration runner 会把 MySQL `1060/1061/1091` 视为 DDL 已执行后的安全重放信号，但 down migration 只有在同一订单尚未产生多条尝试时才能重新建立旧唯一键；一旦已有重试交易，必须前向修复，禁止直接 down 或回滚到旧支付代码。

`010_dine_in_tables_and_print_templates` 以增量方式增加桌台区域、桌码、订单类型/桌台快照和分场景打印模板，并把历史订单按 `fulfillment_type` 回填为 `DINE_IN/TAKEOUT/DELIVERY`。API 与商户前端必须一起发布；数据库迁移完成后不要回滚到不识别这些字段的旧 API。正式微信小程序码仍需在配置 `AppID/AppSecret` 后接入微信 `getUnlimited`，当前后台生成的是联调二维码。

`011_print_template_copies_layout` 为打印模板增加商家联、顾客联、厨房联和商品标签的 `copy_role`，并固化纸宽、布局及打印任务来源。旧 `RECEIPT/LABEL` 会迁移为 `MERCHANT/ITEM`；新 API 首次读取时再幂等创建默认关闭的顾客联与厨房联。为兼容启动失败时的旧 API，`copy_role` 保持可空，并用无特权的生成列把旧写入缺失的联次映射为历史 `MERCHANT/ITEM` 唯一键；新 API 一旦通过就绪检查，部署脚本不再因后续静态/Nginx失败而恢复旧 API。该迁移会改变模板唯一键；发布后若已经创建多联次任务，不要直接回滚到只识别一张整单模板的旧 API。

`012_print_outbox` 新增支付/退款/下单事实到打印任务之间的事务型 outbox。支付与退款事务不再直接解析模板或创建打印任务；worker 负责幂等消费、失败退避和达到上限后的 `DEAD` 记录。生产验收必须同时检查 `print_outbox` 是否持续出现非预期的 `PENDING/DEAD`，但不得为了消除积压而回滚或篡改已经确认的资金状态。

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
