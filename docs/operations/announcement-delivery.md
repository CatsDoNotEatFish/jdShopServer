# 公告投放与回执

## 目标

公告系统用于向全部账号或指定账号发送运营通知，不参与账号授权、客户端停用、任务暂停或强制更新。版本更新继续由 `app_versions` 管理。

## 管理端配置

- 级别：`info` 普通、`warning` 重要、`critical` 紧急。
- 展示：`center` 公告中心、`banner` 顶部横幅、`modal` 弹窗。
- 策略：`once` 每个修订提醒一次、`every_start` 每次客户端启动提醒、`require_ack` 必须明确确认。
- 时间：`starts_at`、`ends_at` 使用 RFC3339；空值分别表示立即生效和长期有效。
- 投放：`all` 全部账号或 `users` 指定账号；可继续按平台和客户端版本码过滤。
- 操作：可配置按钮文字和 HTTPS 链接。

公告先保存为草稿。发布时修订号从0变为1；重新发布或修改已发布公告后修订号递增。回执按修订号保存，因此旧修订已读不会让新修订被跳过。

## 投放链路

1. 管理员发布公告。
2. 服务端保存发布状态和修订号。
3. 全量公告调用 `ControlHub.Broadcast`；指定账号逐用户发送 `announcement_published`。
4. 在线客户端从 SSE 收到事件后重新调用 `GET /api/v1/user/announcements`。
5. 离线客户端登录时拉取；在线客户端每分钟轮询兜底，定时生效公告无需额外调度器。
6. 查询会写入 `delivered_at`；用户查看写入 `read_at`；明确确认写入 `acknowledged_at`。

SSE 只携带 `type`、`announcement_id`、`revision`，公告正文始终通过鉴权接口读取，避免长连接消息成为权威状态。

## 状态和统计

管理列表显示当前修订的送达人数、已读人数和确认人数。下架、过期或不符合账号/平台/版本条件的公告不会出现在用户列表中。

公开 `GET /api/v1/announcements` 仅兼容旧客户端，只返回面向全部账号且当前生效的公告；定向公告只能通过 JWT 用户接口读取。

## 数据库迁移

`006_announcement_delivery.sql`：

- 扩展 `announcements` 的展示、策略、时间、目标、版本和修订字段。
- 新增 `announcement_targets`。
- 新增 `announcement_receipts`。

部署新二进制后启动程序会自动执行迁移。迁移前应按常规流程备份 `data/app.db`。

## 验证

```powershell
C:\Go\bin\go.exe test ./...
```

集成测试覆盖指定账号隔离、版本过滤、送达/已读/确认回执、管理统计和已发布公告修订后重新未读。
