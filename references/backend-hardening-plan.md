# 后端加固计划（2026-08）

本文档记录安全/健壮性加固的已完成项与后续车道。来源：2026-08 全仓审查 + references/source-quality-review.md 挂账项闭环。

## 已完成（本轮）

| 项 | 位置 | 说明 |
| --- | --- | --- |
| voucher 缓存键跨服务对齐 | backend/pkg/utils/vouchercache.go；device_voucher.go、device_delete.go、rdi.go（复用）、api/protocol_plugin.go | 修复现行 bug：backend 曾用明文 voucher 删 broker 的 SHA-256 缓存键，换凭证/删设备后旧凭证最长 1h 仍可认证。限流键同步脱敏（不再把明文 voucher 写入 Redis keyspace）。双端契约测试锁定同一已知向量 |
| 统一 MQTT 拨号地址助手 | backend/pkg/utils/mqtt_broker_address.go；接入 telemetry_simulation.go ×3、simulation_publish/send.go、mqtt/publish/mqtt_client.go、internal/app/mqtt_service.go | 根治 CI "::1 dial" 失败模式：回环地址依次回退 GOTP_MQTT_BROKER / AETHERLINK_MQTT_INNER_BROKER，最终 localhost 归一化为 127.0.0.1（规避 RFC6724 对 ::1 的偏好） |
| broker 认证失败限速 | mqtt-broker/plugin/aetherlink/auth_ratelimit.go + hooks_auth.go 接线 | 每 IP 滑动窗口 60s 内最多 30 次【认证失败】（成功不计、被拒不延长封禁），超限 CONNECT 直接拒绝；配置项 `auth_ratelimit.max_failures_per_minute`（env `GMQTT_AUTH_RATELIMIT_MAX_FAILURES_PER_MINUTE`），非法值回退默认 |
| WS/SSE 监听协程可取消 | pkg/global/WSManager.go、SSEManager.go；internal/app/application.go Shutdown 接线 | Redis Pub/Sub 监听循环支持 context 取消 + 幂等启动守卫（重复 Init 不再叠加监听协程）+ StopListen 有界等待；停机序列显式调用两个 Stop |
| 心跳监视器幂等 Start / 有界 Stop | internal/service/heartbeat_monitor.go | 修复重复 Start 叠加过期事件消费协程的确定性 bug；Stop 等待监听协程真正退出 |
| WS close 所有权收敛 | internal/api/telemetry_ws_stream.go（newWSConnCloser sync.Once）+ 四个端点接入 | 消除双重 close panic 面；书面约定先 CloseSend 后 conn.Close |
| 8082 指标端口防外暴 | docker-compose.yml mqtt-broker 服务 + deploy/tests/host-port-binding-contract.test.sh 契约断言 | 指标无认证，发布绑定钉死 127.0.0.1，不再跟随 AETHERLINK_BIND_ADDRESS |
| TLS 开发路径 | deploy/gen-mqtt-certs.ps1/.sh + deploy/README.md 启用小节 + .gitignore certs | 默认仍关闭；三步启用 8883（gen → 配置取消注释+挂载 → doctor） |
| /files 路径穿越回归锁定 | backend/router/files_traversal_test.go | 判定**不可穿越**：resolver 拒绝 `..`/`\`/`:`/`//` + os.OpenRoot 系统调用级限制 + 仅常规文件下发；14 向量 httptest + AST 契约钉死 handler 不得退化为 c.File/ServeFile |

## 车道 1：设备凭证哈希存储（设计就绪，待排期）

目标：devices.voucher 列从明文 JSON 改为哈希存储。当前为审查确认的高危面（DB/备份泄露=全部设备可冒充）。

关键决策点：
- 迁移：50.sql 加 `voucher_hash varchar(64)` 列 + 索引；**回填建议走 Go 迁移而非 pgcrypto**——broker 存在多候选键序兼容匹配（mqtt-broker/plugin/aetherlink/db.go:227-265 deviceVoucherLookupCandidates），纯 SQL 无法复刻语义等价展开。
- 兼容期：**双模式匹配窗口**（哈希优先、明文列只读兜底 + 观测计数），归零后后续迁移置空并 DROP 明文列。不可照搬 49.sql open_api_keys 的硬切先例（API key 可重生成，设备凭证烧在固件里）。
- 展示面产品决策清单（哈希化后无法回显，需改为"仅创建时一次性展示+轮换"语义）：
  - frontend/src/views/device/details/modules/join.vue:262-291（详情回显/编辑流）
  - frontend/src/views/device/details/modules/DeviceAccessGuide.vue:224-233（密码展示+复制）
  - frontend/src/views/device/details/modules/device-access-guide-state.ts:638-686（测试命令嵌入密码）
  - frontend/src/views/device/manage/modules/add-devices-step2.vue:119-121
  - frontend/src/views/home/useHomeFirstDeviceWorkbench.ts:286（首台设备工作台）
  - backend 详情响应 device.ALL 全列扫描（dal/device_query_reads.go:339 需改投影）
  - 更新凭证响应回显（api/device.go:327-339）、网关注册重复回显（device_gateway_register.go:110-114）、插件回执（protocol_plugin.go:137-143）、预注册 Excel 全量导出（device_preregister_export.go:58,79,100）
- 写入点改造：device_create.go:123-141、device_batch_create.go:106、device_voucher.go:93、device_auth.go:169-181、device_gateway_register.go:37-43,60,129。
- 匹配点改造：broker GetDeviceByVoucher（候选→哈希→voucher_hash 查询）、backend dal.GetDeviceByVoucher（device_query_reads.go:355-368）、CheckVoucherExists（device_identity_queries.go:56-58）。
- 附带清理：死字段 UpdateDeviceReq.Voucher（model/devices.http.go:48，被绑定但从不落库，应显式移除或报错）。
- 生成文件：model/devices.gen.go、query/devices.gen.go 需重新生成以纳入新列。
- 关联审计来源：backend/internal/service 各文件行号见上；完整影响面清单以 2026-08 审计报告为准。

## 车道 2：脚本沙箱 Worker 化

现状：用户脚本在主 JS 线程执行（后端内嵌引擎），正则/静态检查提供不了真抢占与隔离。

方案选项：
- A. goja + 时间片中断：同进程，改造小，但 CPU/内存隔离弱；
- B. 独立进程池 + 资源限额（cgroup/ulimit + 墙钟超时）：隔离最强，运维复杂度中等；
- C. AST 白名单（只放行安全语法子集）：静态可靠，需维护白名单。

建议 B+C 组合：进程池承载执行 + AST 预检拒绝危险构造。工作量粗估：B 约 1-2 周（含 IPC 协议与超时回收），C 约 3-5 天。

## 车道 3：service_access.voucher 同类哈希化

HTTP 接入与服务插件的凭证存于 service_access.voucher（另一列）。校验侧已用 SHA-256 摘要常数时间比较（backend/internal/httpaccess/voucher.go:69-76），但 DB 存储仍明文。devices.voucher 车道完成后按同法迁移。

## 未修挂账（定点处理轮确认存在）

- **croninit 可重入叠加**（中危）：initialize/croninit/cron.go:17-61 包级单例 cron 无重入守卫，重启场景任务会重复注册双份执行。建议 sync.Once 或每次 Init 替换全局 runner。
- SSE 半开连接依赖代理层超时行为（低危）：internal/api/sseapi/sseapi.go:55-58，需运维侧确认代理读超时配置。
- WS 握手形态差异（信息级）：批量状态端点走 header 凭证注入首帧、单设备端点走 readTelemetryWSHandshake；认证入口已统一复用 ValidateJWTUserStatus，握手形态合并需更大改动，暂保留差异。
