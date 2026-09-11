# PHASE-D-D2 交付说明(通知中心 2.0)

> 分支 `phase-d/d2`(基线 main@ac72114),主会话实现(后台 agent 因平台模型容量限制中断,由主会话接管)。

## 完成清单

1. **模板变量引擎** `notification_template_d2.go`:`{{key}}` 占位符(告警数据注入),未知占位符原样保留,不递归展开;`buildIMTextContent` 组装主题+正文,nil 安全。
2. **IM 四渠道** `notification_channels_d2.go`:钉钉/企业微信/飞书/Telegram。契约与既有 WEBHOOK 完全对齐:
   - 配置读 `notification_groups.notification_config`(顶层键 WEBHOOK/SECRET/BOT_TOKEN/CHAT_ID);
   - 历史先 PENDING 后 SUCCESS/FAILURE 回写,remark 只落受控原因码(审计红线:不落原始 URL 凭证与外部错误细节);
   - 审计目标脱敏:webhook 凭证在 query 由 `resolveWebhookEndpoint` 剥离;Telegram token 在路径,显式 REDACTED;
   - 每次发送 10s 独立超时,重试 2 次(与 sendWebhookMessage 一致);内容按 rune 截断 2000;
   - 钉钉加签(HMAC-SHA256,毫秒时间戳)/飞书加签(秒时间戳,密钥=待签名串)按官方规范实现,SECRET 留空则不签。
3. **阿里云短信** `notification_sms_aliyun_d2.go`:`NoticeType_SME_CODE` 桩位补实。RPC 签名(HMAC-SHA1 + RFC3986 percentEncode)stdlib 实现;未配置 fail-closed(SMS_GROUP_CONFIG_INVALID);手机号解析/归一/去重;模板参数 `{"content":...}` 对应控制台模板 `${content}`。
4. **dispatch 接线** `notification_execution.go`(标记 PHASE-D-D2):新增 4 个 IM case + SME_CODE 替换为实发送;删除旧 SMS 桩函数。
5. **前端**:`constants/business.ts` 渠道下拉新增 4 项;`table-action-modal.vue` 新增 IM 配置表单(webhook/加签密钥/bot token/chat id)与编辑回填(顶层键,与后端读取对齐);i18n 四语言(en-us/zh-cn/fr-fr/es-es)新增 9 键。

## 明确不做(复核裁决)

- **告警分配 assignee**:复核发现 `Alarm.UpdateAlarmInfo` 已设置 `Processor`(处理人 id)+ 处理结果(DOP/UND/IGN),单条+批量+写访问守卫齐全——"分配/认领"语义已存在,68.sql 无需建列,不再重复实现。

## 证据

- 后端:`go build ./internal/service/` ✓;`go vet ./internal/service/` ✓;`go test ./internal/service/ -count=1` ✓(全包回归,含既有通知/邮件测试零回退;新增 D2 单测:模板引擎 9 例、IM 渠道 5 形状+4 配置非法+外发失败重试+截断+签名转义、短信签名回环/OK/拒绝重试/fail-closed/手机号解析)。
- 前端:`npm run typecheck` 0 错误;`npx vitest run`(全量)3617 passed(含 hex 契约与通知组 56 例)。
- 提交:32343ec(后端渠道批)、<本次>(前端+文档)。

## 预期冲突点(集成时注意)

- `notification_execution.go` 的 switch 有 PHASE-D-D2 标记段。
- `frontend/src/constants/business.ts` notificationOptions 追加段。
- i18n generate.json ×4 追加键(无删改既有键)。

## 未尽事项(留集成阶段)

- 运行期 E2E:隔离栈 + 钉钉/飞书官方调试台(或 httptest 反代)实发验证;短信需真实 AccessKey,建议用户环境抽验。
- 通知测试发送按钮(前端"发送测试"入口)——后端已有全部能力,UI 增量留待与看板/报表批一起做。
