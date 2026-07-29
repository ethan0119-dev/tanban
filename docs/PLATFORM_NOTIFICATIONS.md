# 平台通知与商户收件箱

## 1. 业务目标

摊伴平台可向全部或指定商户发布系统迭代、问题修复、新功能说明、注意事项和待办提醒。商户老板、店长与店员均可从运营后台顶部铃铛进入收件箱，未读数量按登录账号独立计算。

该能力是 SaaS 平台公告，不等同于订单语音提醒、微信服务号模板消息或短信。后续接入服务号时，可以复用已发布通知作为外部推送内容源，但站内信仍是可追溯的最终收件箱。

## 2. 状态与权限

- 草稿 `DRAFT`：平台管理员可创建和修改，商户不可见。
- 已发布 `PUBLISHED`：发布时固化接收商户范围，正文和范围不可再修改。
- 已撤回 `WITHDRAWN`：平台管理员可撤回；商户收件箱立即隐藏，发送与阅读记录继续保留用于审计。
- 平台运营账号可查看通知与触达数据，只有平台管理员可以创建、修改、发布和撤回。
- 商户端所有有效角色都能查看；已读记录以 `announcement_id + user_id` 唯一，互不影响。

## 3. 接收范围

- `ALL`：发布时快照所有未删除商户。发布之后新创建的商户不会收到历史通知。
- `SELECTED`：草稿保存明确的商户列表；发布时剔除已经删除的商户，若最终没有可用接收者则拒绝发布。

发布动作在一个数据库事务内完成通知状态变更和商户接收快照，避免出现“页面显示已发布但商户没有收件记录”的中间状态。

## 4. 数据结构

- `platform_announcements`：标题、摘要、正文、类型、重要程度、接收模式和生命周期。
- `platform_announcement_targets`：指定商户草稿的目标列表。
- `merchant_notification_recipients`：发布时生成的商户接收快照。
- `merchant_notification_reads`：具体商户账号的首次阅读记录。

平台“已读商户”统计按至少有一个账号读过的商户去重；商户铃铛和“只看未读”按当前账号计算。

## 5. API

平台端：

- `GET /api/v1/platform/announcements`
- `POST /api/v1/platform/announcements`
- `GET|PUT /api/v1/platform/announcements/{id}`
- `POST /api/v1/platform/announcements/{id}/publish`
- `POST /api/v1/platform/announcements/{id}/withdraw`

商户端：

- `GET /api/v1/merchant/notifications`
- `GET /api/v1/merchant/notifications/unread-count`
- `GET /api/v1/merchant/notifications/{id}`
- `POST /api/v1/merchant/notifications/{id}/read`
- `POST /api/v1/merchant/notifications/read-all`

商户后台登录后立即查询未读数量，每 60 秒及窗口重新获得焦点时刷新。阅读单条或全部已读后，通过页面事件立即刷新顶部角标，不需要等待下一轮轮询。

## 6. 后续扩展

当微信服务号、短信或 App Push 接入时，应新增异步投递任务与每通道投递状态，不要把外部通道“投递成功”与站内信“已读”混为一个状态。外部通道失败不影响站内收件快照，Worker 可独立重试并记录失败原因。
