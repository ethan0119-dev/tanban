# 摊伴正式环境部署

本文档记录正式环境的唯一基准。生产部署与遗留测试环境相互独立；发布生产环境时，不修改或停止测试服务器上的现有容器、Nginx 站点和数据库。

## 环境与域名

| 用途 | 地址 |
| --- | --- |
| 产品官网 | `https://tanban.com.cn` |
| 官网重定向 | `https://www.tanban.com.cn` → `https://tanban.com.cn` |
| REST API、媒体与支付回调 | `https://api.tanban.com.cn` |
| SaaS 平台管理端 | `https://admin.tanban.com.cn` |
| 商户运营后台 | `https://b.tanban.com.cn` |

生产服务器为 `39.96.16.153`，日常部署用户为 `deploy`。所有域名的公网 A 记录必须直接解析到该地址，且 80/443 端口对公网开放。不要只依赖云厂商内网 DNS 的同地域解析结果；签发证书前应从权威 DNS 和至少一个公共递归 DNS 复核。

遗留测试服务器为 `192.144.213.94`。正式发布不覆盖该服务器的配置，测试服务继续独立运行。

## 目录约定

```text
/srv/tanban/
  current -> releases/<release-id>
  releases/
  incoming/
  artifacts/
  shared/
    acme/.well-known/acme-challenge/
    api-public/
    media/
    uploads/
    static/
      platform/current
      merchant/current

/etc/tanban/
  env/production.env
  secrets/wechat-pay/
    apiclient_cert.pem
    apiclient_key.pem
    wechatpay_public_key.pem
    api_v2.key
    api_v3.key

/var/backups/tanban/
  mysql/
  media/
  config/
```

代码和静态资源归 `deploy:deploy` 管理。`/etc/tanban` 归 `root:tanban-secrets` 管理，目录权限 `0750`；微信支付 PEM/KEY 文件权限 `0640`，APIv2/APIv3 对称密钥文件权限 `0640`。密钥不得提交 Git、写入镜像、放到网站静态目录或经管理 API 返回。

## 生产环境变量

`/etc/tanban/env/production.env` 至少包含：

```dotenv
TB_HTTP_ADDR=127.0.0.1:18090
TB_DATABASE_DSN=tanban_app:<password>@tcp(127.0.0.1:3306)/tanban?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai
TB_MIGRATIONS_DIR=/app/migrations
TB_MEDIA_STORAGE_DIR=/var/lib/tanban/media
TB_MEDIA_PUBLIC_BASE_URL=https://api.tanban.com.cn/api/v1/public/media
TB_CORS_ALLOWED_ORIGINS=https://tanban.com.cn,https://www.tanban.com.cn,https://admin.tanban.com.cn,https://b.tanban.com.cn
NEXT_PUBLIC_TANBAN_API_URL=https://api.tanban.com.cn/api/v1
VITE_API_BASE_URL=https://api.tanban.com.cn/api/v1
TB_AUTO_MIGRATE=true
TB_SEED_DEMO=false
TB_ALLOW_MOCK_CONFIRMATION=false

TB_WECHAT_PAY_BASE_URL=https://api.mch.weixin.qq.com
TB_WECHAT_PAY_SP_MCH_ID=1748591603
TB_WECHAT_PAY_SP_APP_ID=wx087d633542ae8d0b
TB_WECHAT_PAY_SERVER_IP=39.96.16.153
TB_WECHAT_PAY_API_CERT=file:/run/secrets/wechat-pay/apiclient_cert.pem
TB_WECHAT_PAY_PRIVATE_KEY=file:/run/secrets/wechat-pay/apiclient_key.pem
TB_WECHAT_PAY_API_V2_KEY=file:/run/secrets/wechat-pay/api_v2.key
TB_WECHAT_PAY_API_V3_KEY=file:/run/secrets/wechat-pay/api_v3.key
TB_WECHAT_PAY_PUBLIC_KEY_ID=<微信支付公钥ID>
TB_WECHAT_PAY_PUBLIC_KEY=file:/run/secrets/wechat-pay/wechatpay_public_key.pem
TB_WECHAT_PAY_NOTIFY_URL=https://api.tanban.com.cn/api/v1/payments/wechat-partner/callback
TB_WECHAT_PAY_REFUND_NOTIFY_URL=https://api.tanban.com.cn/api/v1/payments/wechat-partner/refund-callback
```

在 APIv3 小程序下单、通知验签解密、主动查单、退款和退款通知全部完成生产联调前，`TB_PAYMENT_PROVIDER` 必须保持 `mock`，不得切换为 `wechat_partner`。

## 首次部署

1. 将代码发布到 `/srv/tanban/releases/<release-id>`，并令 `/srv/tanban/current` 原子指向该目录。
2. 安装 Node.js 22 到 `/opt/node-v22`；安装并启用 `infra/systemd/tanban-website.service`。
3. 安装 `infra/nginx/acme-bootstrap.conf` 为 `/etc/nginx/conf.d/tanban-acme-bootstrap.conf`，执行 `nginx -t` 后 reload。
4. 公网 DNS 正确后签发包含全部域名的一张证书：

```bash
certbot certonly --webroot \
  -w /srv/tanban/shared/acme \
  -d tanban.com.cn -d www.tanban.com.cn -d api.tanban.com.cn \
  -d admin.tanban.com.cn -d b.tanban.com.cn
```

5. 安装 `infra/nginx/{tanban,api,admin,b}.tanban.com.cn.conf`，移除 bootstrap 配置，执行 `nginx -t` 后 reload。
6. 以 root 执行 `scripts/server-deploy.sh`。脚本会备份数据库、构建并启动 API、构建两个后台、发布官网并做就绪检查。
7. 安装 `infra/cron/tanban-mysql-backup`，验证一次手工备份及 `gzip -t`。

## 数据迁移与校验

源库使用 `mysqldump --single-transaction --routines --events --triggers --hex-blob` 生成压缩快照，并记录 SHA-256。目标库导入前必须为空；导入后至少核对表数量、`orders`、`order_items`、`media_assets` 和 `schema_migrations` 的精确行数。

媒体目录使用 rsync 迁移。源端与目标端均按“相对路径 + 文件内容”生成排序后的 SHA-256 清单并再次求摘要，文件数和摘要必须完全相同。旧测试环境继续写入时，正式库只代表迁移快照时点；此后两个环境的数据不再自动双向同步。

## 验收

- `http://127.0.0.1:18090/readyz` 返回 2xx。
- 官网根路径包含“摊伴”，关键图片返回 `image/*`。
- `admin` 和 `b` 域名能打开 SPA，刷新子路由不返回 404。
- `api` 域名的健康检查、登录、媒体文件、CORS 与微信域名校验文件正常。
- 五个 HTTPS 域名证书 SAN 完整，HTTP 自动跳转 HTTPS，`www` 自动跳转主域名。
- `certbot renew --dry-run` 成功。
- 公网 DNS、正式 API、小程序合法域名和微信支付通知地址全部只使用正式域名。
