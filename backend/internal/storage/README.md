# 后端存储辅助层

`backend/internal/storage` 保存运行期存储抽象与存储辅助逻辑，负责上传、生成文件或本地运行期产物相关的存储协作。

## 目录职责

- 封装运行期文件或对象存储相关的基础能力。
- 为业务层提供更稳定的存储入口，而不是直接在业务文件里散落路径和存储细节。

## 遥测独立文件 spool

- `telemetry_file_spool.go` 是主 PostgreSQL 与库内 `telemetry_dead_letters` 同时不可写时的最后一道本机持久边界。每个历史点一个 JSON 文件，先写同目录临时文件并 `fsync`，再原子重命名；启动时会把内容完整且校验通过的中断临时文件提升为正式记录，只清理不完整或损坏的临时文件。
- 文件名身份与数据库唯一键 `(device_id, key, ts)` 一致，内容有 SHA-256 校验。后台重放使用历史 `ON CONFLICT DO NOTHING` 和当前值单调 upsert；只有事务成功后才删除文件，所以崩溃后的重复重放是安全的。
- JSON、身份或校验和损坏的正式记录不会自动删除；系统会原子重命名为 `.json.corrupt*` 隔离，继续处理同批后续正常记录，并以 `corrupt` 数量和哈希文件名告警。隔离文件仍计入容量且不会自动重放或清理，运维应复制留证后人工修复、归档或删除；系统不会把它当作已重放。
- 默认总容量为 512 MiB 或 100000 条，新写单条 JSON 上限 16 MiB；任一上限先到即拒绝新记录，绝不通过淘汰旧点制造“成功”。读取兼容安全上限固定为 512 MiB，与当前新写上限解耦，降低 `max_record_bytes` 不会把旧的完整记录误判为损坏。`Storage.GetMetrics()` 的进程内快照包含累计落盘/重放/失败/损坏及当前 backlog 条数、字节数和 quarantine 占用；启用 `storage.enable_metrics` 时，同一组累计值、容量、backlog 和 quarantine gauges 会暴露在现有 `/metrics` Prometheus registry 中。对应名称是 `AetherLinkIoT_storage_telemetry_spooled_total`、`AetherLinkIoT_storage_telemetry_spool_replayed_total`、`AetherLinkIoT_storage_telemetry_spool_failures_total`、`AetherLinkIoT_storage_telemetry_spool_corrupt_total`、`AetherLinkIoT_storage_telemetry_spool_backlog_records`、`AetherLinkIoT_storage_telemetry_spool_bytes`、`AetherLinkIoT_storage_telemetry_spool_quarantine_records`、`AetherLinkIoT_storage_telemetry_spool_quarantine_bytes`、`AetherLinkIoT_storage_telemetry_spool_capacity_records` 和 `AetherLinkIoT_storage_telemetry_spool_capacity_bytes`；这些名称尚未通过实际抓取验收。
- spool 文件包含原始遥测值，只申请目录 `0700`、文件 `0600`，不把值写入应用日志。权限不等于加密：生产部署必须使用受限、持久、必要时加密的独立卷；多个后端实例必须各用自己的目录，不能把同一个目录作为并发共享队列。
- Compose 已把 `/go/src/app/data/telemetry-spool` 挂载到独立于 `postgres-data` 的 `backend-telemetry-spool` 卷。spool 必须保持在公开 `/files/*filepath` 根目录之外；容量需按最坏数据库故障时长和点速率估算。Prometheus 指标只证明本进程观测到的落盘/重放/容量状态，仍需部署侧配置告警规则和磁盘/卷健康检查。

## 遥测 writer 转换边界

- `telemetry_writer.go` 负责批次接收、分块和数据库写入编排；`telemetry_writer_transform.go` 集中 wire payload 转换、历史点去重、current 最新值选择、预览行生成与值列转换。拆分保留原内部函数名和存储契约，`buildTelemetryCurrentChunk` 只为 history 中出现且 current lookup 已存在的 `(device_id, key)` 生成一条最新 current 行。
- 这一区域的 focused 测试源码覆盖重复 history/current 行与 history-only key。`GOMAXPROCS=1 go test -p=1 -gcflags=all='-N -l' ./internal/storage` 已通过；Windows `go1.26.2` 的默认优化编译仍会在 `cmd/compile/internal/ssa.(*factsTable).update` 发生 access violation，因此默认优化构建继续记为工具链阻断，不能用降优化结果替代生产构建验收。

## Attribute/event durable envelope 与 PostgreSQL dead-letter

- `attribute_event_envelope.go` 集中 version 2 canonical envelope 的类型、构建、规范化、校验、身份与 fingerprint helper；`attribute_event_ingress.go` 只保留数据库写入、dead-letter 与 replay 编排，避免持久化流程和序列化契约继续混在同一大文件。envelope 包含 `message_id`、完整 SHA-256 `fingerprint`、设备/租户、类型、正时间戳和 canonical JSON payload；属性点按 key 排序且拒绝空/重复 key，事件保留 `identify` 与 canonical data。重放入口再次执行同一校验，拒绝未知字段、非 canonical payload、身份碰撞、空设备/租户和非正 timestamp。该拆分已通过 `go test ./internal/storage`。
- 协议来源的 opaque message identity 只以 tenant、已认证来源设备、目标设备和数据类型参与派生 UUID-shaped identity；原始 MQTT identity 不写入 receipt、dead-letter、spool 或日志。没有协议 identity 的内部事件每次生成新的 occurrence identity，不把相同时间戳/值错误合并。
- 持久化顺序是“主表事务 -> `uplink_storage_dead_letters` -> 独立私有 file spool”。attribute receipt 与整批属性行同一事务，event 使用 envelope ID 作为幂等主键；只有三层中至少一层返回 durable receipt，heartbeat、automation、RDI/SW3/OTA 等业务副作用才允许继续。receipt 表示耐久接纳，不等于最终主表已完成，也不等于 exactly-once。
- `39.sql` 为 PostgreSQL dead-letter 增加 `claim_token` 与 `lease_until`。后台和人工 replay 都先按状态/重试时间或过期租约候选，再用 `pending|retrying -> processing` 的 CAS 领取；`resolved`、`retrying`、`dead` 的结算必须同时匹配 `id + processing + claim_token`。过期租约可被另一实例安全接管，旧实例的迟到写入会被 fencing 拒绝；取消/超时会以短 detached context 尝试结算。`attribute_event_dead_letter_operator.go` 现提供 metadata-only 分页、tenant/device/type/status 过滤、单条 retry/replay/resolve/ignore 和最多 100 条 bounded drain；HTTP 路由为 `/telemetry/datas/uplink-dead-letters`。单条动作必须携带 `expected_status` 并在同一数据库更新中做状态 CAS；人工 drain 会先在调用者 tenant/owner scope 内回收已耗尽且租约过期的 `processing` 行，即使候选筛选为其他状态也不会跳过，同时不会顺带修改别的租户。这些入口仅完成源码静态接线，尚未运行真实 API/PostgreSQL/权限和并发验收。
- file spool 保存完整 envelope，采用私有目录、受限权限、临时文件 `fsync`、原子重命名、容量上限、校验和、损坏隔离和成功后删除；DB replay 与 file replay 都只重做存储，不重复触发业务副作用。多实例不要共享同一个 file-spool 目录；PostgreSQL claim 可多实例共享，但必须使用独立租约和持久卷。
- 本轮只完成源码、SQL、文本和静态合同同步；未执行 PostgreSQL 39 迁移、时钟偏差/租约抢占、主表/同库故障、磁盘满/只读/断电、进程重启重放、真实 MQTT retransmission、Prometheus 抓取、API/E2E 或副作用时序验收。

## 审查与重构建议

- 问题：存储路径或清理逻辑一旦混乱，容易把本地产物、生成文件和公开源码边界搞混。
- 改进方案：继续明确哪些路径是运行期、哪些文件应被忽略、哪些产物属于可公开源码之外的内容。
- 预期效果：提升部署清洁度，减少把本地产物误带进仓库的风险。
