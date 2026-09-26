# 协议接入传输深度：TCP 入站网关（TP-03）

## 适用版本与状态

- 适用版本：2026-09 批次三（TP-03 后端面 + 收尾面交付）。
- 当前状态：**已实现（后端）**；TLS 封装、跨副本会话共享、真实设备联调仍为已知边界（见文末）。
- 相关源码：`backend/internal/protocolgw/tcp/`（framing.go / registry.go / ingest.go / downlink.go /
  gateway.go）、`backend/internal/app/tcp_gateway.go`、`backend/internal/app/downlink.go:48-55`、
  `backend/main.go:63`（装配点）；复用 `backend/internal/protocolgw/telemetry.go` 的
  `DBNumberResolver`（device_number → 设备/租户凭证映射）与 uplink 总线发布面。
- 验证方式：`backend/internal/protocolgw/tcp` 包 37 个 Go 单测（帧解析粘包/半包、注册映射、
  断网缓冲续传、真实 socket 端到端）；契约测试 `automation_tests/tests/90_tcp_gateway.test.js`
  （默认关闭面 + 启用路径，待活栈运行）。

## 一、接入拓扑与配置面

TCP 网关是进程内入站监听（`net.Listen`），与 CoAP/LwM2M UDP 面平行，**默认关闭**：

| 配置键（viper） | 含义 | 默认 |
| --- | --- | --- |
| `protocols.tcp.enabled` | 总开关 | `false`（关闭时不建监听、不占端口） |
| `protocols.tcp.host` | 监听地址 | `0.0.0.0` |
| `protocols.tcp.port` | 监听端口 | `9877`（避开平台已占端口 9999/1883/8080/8082/5683/18881） |
| `protocols.tcp.frame-mode` | 帧格式 | `length-prefix`（可选 `line`） |
| `protocols.tcp.max-frame-size` | 单帧 payload 上限（字节） | `4096` |
| `protocols.tcp.command-buffer-limit` | 单设备下行缓冲上限（条） | `1000` |
| `protocols.tcp.idle-timeout-seconds` | 注册后读空闲超时（秒，0=不启用） | `300` |

装配语义（`app/tcp_gateway.go`）：`WithTCPGateway()` 在 `protocols.tcp.enabled=true` 时启动；
DB/uplink 服务未就绪则降级不启动（比 CoAP 的"纯接入层降级"更严格——无凭证映射的纯 TCP 监听
无法完成注册，放行等于摆设）。`Application.Shutdown` 显式 `Stop`（踢全部会话+关监听），
TCP 会话是真实连接资源，不走 CoAP 的进程退出回收路径。

## 二、帧协议契约（framing.go）

两种帧格式，由 `protocols.tcp.frame-mode` 选择，上行下行同格式：

1. **length-prefix（默认）**：4 字节大端长度 + payload。二进制安全。
2. **line**：`'\n'` 分隔文本帧（兼容尾部 `\r`，即 CRLF）；空行视为心跳跳过。
   **下行 payload 含 `'\n'` 直接报错**（`ErrInvalidPayload`），绝不静默改写帧边界语义。

**首帧即注册帧**：连接建立后的第一帧承载设备号（后续帧全部是遥测），两种写法等价：

- 纯文本：`dev-01`（首尾空白容忍）；
- JSON 信封：`{"device_number":"dev-01"}`（为未来携带凭证字段预留扩展，不破坏纯文本设备）。

设备号字符白名单：字母/数字/`. _ : @ -`，长度 ≤128——防换行注入（line 模式帧边界）、
控制字符与空白注入。解析失败或超长一律断连。

**遥测帧**：必须是**非空 JSON 对象**（键值集），原样透传进 `mqttadapter.UplinkMessage.Payload`，
metadata 携带 `source_protocol=tcp`；非对象/空对象/发布失败 fail-closed 丢弃并计数，
绝不阻塞读循环。

**流式与中毒语义**：每连接独占 `FrameAccumulator`，`Feed` 消化粘包（一次多帧）与半包
（不完整帧留存待续）。超长帧（声明长度或无换行残留超过 `max-frame-size`）使流**中毒**：
无法在二进制流内重同步，此后 `Feed` 恒返回同时匹配 `ErrStreamPoisoned` 与原始违规错误
的包装错误（`errors.Is` 双命中），调用方必须关闭连接——防半帧静默拼接回归。

## 三、会话注册簿与凭证映射（registry.go / ingest.go）

参照 `internal/lwm2m` 注册簿 + `protocolgw.TelemetryBridge` 凭证映射模式：

1. **注册**：首帧解析出设备号后，经 `protocolgw.DBNumberResolver.ResolveByNumber` 做凭证映射
   （`devices.device_number` + `is_enabled=enabled`），身份（DeviceID/TenantID/DeviceNumber）
   在注册时解析并**固写到会话**——运行期每帧不再查库，租户归属取自 DB 记录、不信任帧内容。
2. **fail-closed**：未知/禁用设备号、非法注册帧一律断连（弱凭证边界与 CoAP 面一致：
   `device_number` 即准入凭证，PSK/TLS 升级为后续安全增强）。
3. **顶替**：同号新会话胜出，旧连接被关闭；旧会话随后的去注册请求按**会话身份指针**校验，
   不会误删新会话——这是断网缓冲正确性的前提。
4. **Known 留存**：注册簿只保存**在线**会话；`Known` 集合记录本进程注册过的设备号（含离线），
   是下行"走 TCP 通道还是回退 MQTT"的路由依据。

## 四、下行命令断网缓冲（downlink.go）

与 `internal/edgeforward` 同款环形缓冲模式（满丢最旧并计数、失败转缓冲、恢复按 FIFO 续传）：

- **在线直投**：会话存在 → `EncodeFrame` + 5s 写超时写回；写失败视为连接已死，
  去注册+关闭+命令转入缓冲。
- **离线缓冲**：设备 Known 但无会话 → 入该设备专属环形缓冲（上限
  `protocols.tcp.command-buffer-limit`，满丢最旧）。返回 nil 表示"已接受"（尽力而为语义）。
- **注册续传**：设备重连注册事件触发 `flushOnRegister`，按 FIFO 逐条投递；写失败立即停止，
  剩余命令按原序塞回**队首**（requeue 溢出时丢"最新"保"最旧"——续传顺序优先于新命令），
  全部续传完回收缓冲条目。全内存态，进程重启缓冲即失（与 edgeforward 同口径）。
- **未知设备显式报错**：从未注册过的设备号返回 `ErrUnknownDevice`，绝不静默吞掉。

**下行缝**（`app/tcp_gateway.go` + `app/downlink.go:48-55`）：`tcpFallbackPublisher` 包装原
MQTT 发布器——`HandlesDevice(deviceNumber)` 为真（本进程注册过 TCP 会话，含离线）时改走
`DeliverCommand`，否则原样回落 MQTT。非 TCP 设备**零行为变化**；未启用网关时不包装。
注意：命令经缓冲即记"接受"，`command_set_logs` 会标成功（与 edgeforward 尽力而为口径一致），
per-命令投递回执属后续迭代。

## 五、部署注意事项

- 默认端口 `9877` 与平台既有端口（HTTP 9999、broker 1883/8082、前端 8080、CoAP 5683、
  插件 gRPC 18881、PG 5432、Redis 6379）无冲突；开启后需在 docker compose/K8s 补充
  端口暴露（当前 Helm env 键集与 docker-compose 严格对齐且由 helmchart 单测强制，
  TCP 端口暴露属 TB-11 部署面后续批次）。
- 网关默认关闭：不开启时 `net.Listen` 不会执行，端口探测为 connection refused
  （契约测试用例 1 守护该默认面）。
- 启用路径的活栈验证需在栈配置 `protocols.tcp.enabled=true` 并对契约测试导出
  `AETHERLINK_TCP_GATEWAY_ENABLED=1`（可选 `AETHERLINK_TCP_GATEWAY_HOST/PORT`）。

## 六、已知边界（诚实口径）

1. **TLS 封装未做**：明文传输，与 CoAP/UDP 面同水位；公网部署需前置 TLS 终结（LB/网关），
   或后续在 `Gateway` 内引入 `tls.Listen`（配置面预留 `protocols.tcp.tls.*` 时再动）。
2. **真实 TCP 设备联调未做**：需活栈 + 设备模拟器；Go 侧已有真实 socket 端到端单测
   （127.0.0.1:0），契约测试 90 号已就位待活栈运行。
3. **单机形态**：注册簿与命令缓冲均为进程内存态；多副本部署需外置会话表/共享队列，
   当前仅单副本语义成立。
4. **无 UI 面**：网关为配置门控的进程内监听，无新增 HTTP API，前端无对应页面/菜单。
5. **`-race` 环境限制**：本机无 gcc/CGO，竞态检测未能执行；并发正确性由锁设计保证
   （缓冲单锁域、registry 事件锁外回调、会话写互斥）。
