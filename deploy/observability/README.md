# Durable spool observability

This directory ships separate per-instance Prometheus rule groups for the two
private filesystem durability tiers:

- `telemetry-spool-alerts.yml` covers point-level telemetry fallback.
- `attribute-event-spool-alerts.yml` covers complete attribute/event envelopes.

The lightweight root Compose stack still does not deploy Prometheus or
Alertmanager. Loading these source files into a real monitoring stack, scraping
every backend instance, validating the rules, and routing notifications remain
deployment responsibilities.

## Attribute/event spool runbook

The attribute/event spool contains complete canonical envelopes and is separate
from the telemetry point spool. Each backend instance must use its own private,
persistent directory. Preserve files and logs before manual intervention; never
delete backlog merely to clear an alert. The database dead-letter worker and
file-spool replay repair primary storage only. They do not make MQTT ACK equal a
durable receipt and do not replay automation, email, SW3, OTA, or other business
side effects.

### Attribute/event spool activated

Confirm the fallback counter increased on the alerted instance, then inspect
both the primary PostgreSQL write and `uplink_storage_dead_letters`. File
fallback means those two database tiers failed; it is a durability safety net,
not evidence that normal writes are healthy. After recovery, verify replay
increases and backlog decreases on that same instance.

### Attribute/event spool write failure

Treat any increase as critical. Inspect configured record/byte limits, volume
free space and inodes, mount read-only state, directory ownership, and backend
logs. A file-spool failure occurs after the primary transaction and PostgreSQL
dead-letter path have already failed. Preserve the error evidence and do not
claim every accepted consumer message has a durable copy until the failure is
explained.

### Attribute/event spool corruption

Retain restricted copies of `.corrupt*` files and correlate them with host I/O,
power loss, filesystem, and mount events. Quarantined envelopes are excluded
from automatic replay. Any repair, replay, rewrite, or deletion needs an
auditable data-owner decision because an envelope can contain customer device
attributes or events.

### Attribute/event spool backlog

Compare fallback and replay counter rates, database availability, the configured
replay interval/batch/timeout, and quarantine count. A non-zero backlog made only
of quarantine records cannot drain automatically. Keep observing until backlog
returns to zero; one successful replay log is not closure.

### Attribute/event spool capacity

Check both record and byte ratios for the alerted instance. Restore database and
replay capacity first. If temporary expansion is required, confirm real volume
space and inode headroom before raising `max_records` or `max_bytes`, then record
the exported capacity gauges after a controlled restart. The spool never evicts
old envelopes to admit new ones.

### Attribute/event spool quarantine

Open an operator incident, preserve the files and validation logs, and have the
data owner decide whether a valid envelope can be reconstructed and replayed.
Quarantine consumes both record and byte capacity. Repeated corruption after
manual cleanup is a storage-integrity incident, not a reason for automatic
deletion.

本目录提供 AetherLink backend 独立 telemetry 文件 spool 的 Prometheus 告警规则和运维接入说明。

- `telemetry-spool-alerts.yml` 是可挂载到 Prometheus 的规则文件。
- 当前根 `docker-compose.yml` 没有部署 Prometheus 或 Alertmanager；把规则提交到仓库并不等于生产告警已经生效。
- 本说明只覆盖 spool 指标。抓取目标存活、Prometheus 自身、Alertmanager 通知链路和宿主机磁盘/inode 告警仍应由部署平台统一提供。

## Preconditions

告警生效前必须同时满足：

1. backend 的 `storage.telemetry_spool.enabled` 为 `true`。
2. backend 的 `storage.enable_metrics` 为 `true`。Compose 环境也可显式设置 `GOTP_STORAGE_ENABLE_METRICS=true`。
3. Prometheus 能抓取 backend 的 `/metrics`。根 Compose 默认将 backend 暴露在宿主机 `9999` 端口；同一 Compose 网络内的 Prometheus 可使用 `backend:9999`。
4. 每个 backend 实例的 spool 目录位于独立持久卷上，并限制访问。文件含原始 telemetry 值，不应复制到公开 `/files` 路径。

`storage.enable_metrics=false` 时这些指标仍可能以零值存在于默认 registry，但不会随 spool 更新；因此“没有告警”不能替代对有效配置和抓取目标的检查。spool 未启用或尚未初始化时，capacity gauge 也可能是零，容量规则会主动忽略零分母。

## Prometheus integration

把规则文件只读挂载到已有 Prometheus，并将它加入 `rule_files`。以下只是需合并到部署方 Prometheus 配置的片段，不代表本仓库已经新增 Prometheus 服务：

```yaml
rule_files:
  - /etc/prometheus/rules/telemetry-spool-alerts.yml
  - /etc/prometheus/rules/attribute-event-spool-alerts.yml

scrape_configs:
  - job_name: aetherlink-backend
    metrics_path: /metrics
    static_configs:
      - targets:
          - backend:9999
```

对应的容器挂载示例：

```yaml
volumes:
  - ./deploy/observability/telemetry-spool-alerts.yml:/etc/prometheus/rules/telemetry-spool-alerts.yml:ro
  - ./deploy/observability/attribute-event-spool-alerts.yml:/etc/prometheus/rules/attribute-event-spool-alerts.yml:ro
```

如果 Prometheus 不在根 Compose 网络中，请将 target 换成它实际可达的 backend 地址。不要为了抓取指标而把 `/metrics` 无认证暴露到公网；优先使用内网、反向代理访问控制或监控专网。Alertmanager 路由至少应根据 `severity` 和 `component=telemetry-spool` 分流。

接入或修改规则后，由部署方使用其 Prometheus 版本自带的 `promtool check rules` 做语义校验并安全 reload。目标环境还应确认：

- `/metrics` 中实际出现下面列出的大小写完全一致的指标名；
- `capacity_records` 和 `capacity_bytes` 大于零，且等于该实例的有效配置；
- Prometheus 的 Rules 页面或 API 显示本组 9 条规则；
- Alertmanager 的测试通知能抵达值班渠道。

本次仓库变更没有启动服务、抓取指标、运行 `promtool` 或发送测试通知。

## Shipped alerts

| Alert | 条件 | 持续门槛 | Severity | 目的 |
| --- | --- | --- | --- | --- |
| `AetherLinkTelemetrySpoolActivated` | `spooled_total` 在 10 分钟窗口内有增量 | 2 分钟 | warning | 尽早提示 PostgreSQL 与数据库 dead-letter 已同时失败，文件兜底正在接管 |
| `AetherLinkTelemetrySpoolWriteFailure` | `failures_total` 在 10 分钟窗口内有增量 | 2 分钟 | critical | spool 自身无法落盘，存在未获得耐久副本的风险 |
| `AetherLinkTelemetrySpoolCorruptionDetected` | `corrupt_total` 在 15 分钟窗口内有增量 | 2 分钟 | warning | 尽快调查文件损坏和底层存储 |
| `AetherLinkTelemetrySpoolBacklogPersistent` | backlog 非零 | 15 分钟 | warning | 区分短时数据库抖动与持续重放积压 |
| `AetherLinkTelemetrySpoolRecordsNearCapacity` | records / configured records capacity >= 80% | 10 分钟 | warning | 避免记录数上限耗尽 |
| `AetherLinkTelemetrySpoolBytesNearCapacity` | bytes / configured bytes capacity >= 80% | 10 分钟 | warning | 避免逻辑字节上限耗尽 |
| `AetherLinkTelemetrySpoolRecordsCriticalCapacity` | records / configured records capacity >= 95% | 2 分钟 | critical | 记录数上限即将耗尽，新的 fallback 写入可能被拒绝 |
| `AetherLinkTelemetrySpoolBytesCriticalCapacity` | bytes / configured bytes capacity >= 95% | 2 分钟 | critical | 逻辑字节上限即将耗尽，新的 fallback 写入可能被拒绝 |
| `AetherLinkTelemetrySpoolQuarantinePresent` | quarantine records 非零 | 5 分钟 | critical | 隔离记录不会自动重放，需要人工处置 |

窗口和 `for` 都是为了抑制一次 scrape 抖动或可自行恢复的短暂 backlog。spool 写失败和损坏本身是离散事件，因此规则用 counter 增量而不是 counter 当前总值，避免进程整个生命周期永久告警。

## Capacity calibration

容量规则不硬编码默认的 512 MiB 或 100000 条，而是分别除以每个实例实际导出的：

- `AetherLinkIoT_storage_telemetry_spool_capacity_bytes`
- `AetherLinkIoT_storage_telemetry_spool_capacity_records`

这些 gauge 在 spool 启动时由有效的 `storage.telemetry_spool.max_bytes` 和 `storage.telemetry_spool.max_records` 设置。根 Compose 分别通过 `AETHERLINK_TELEMETRY_SPOOL_MAX_BYTES` 和 `AETHERLINK_TELEMETRY_SPOOL_MAX_RECORDS` 映射到 backend；非 Compose 部署可以使用 `GOTP_STORAGE_TELEMETRY_SPOOL_MAX_BYTES`、`GOTP_STORAGE_TELEMETRY_SPOOL_MAX_RECORDS` 或配置文件键。

生产定值应至少考虑：预期最长 PostgreSQL 故障时间、峰值 telemetry 点速率、单条落盘后的平均逻辑字节、重放吞吐、卷内其他文件占用以及磁盘安全余量。可用下式估算最低需求，再加入运维余量：

```text
required_records ~= peak_points_per_second * tolerated_outage_seconds
required_bytes   ~= required_records * measured_average_spool_record_bytes
```

`max_record_bytes` 是单条保护上限，不是总容量，不能拿它替代 `max_bytes`。提高 spool 上限前必须先确认持久卷真实可用空间和 inode；容量 gauge 反映逻辑 spool 字节，不等同于文件系统实际占用，也不能替代宿主机磁盘告警。

默认 80%/10 分钟 warning 与 95%/2 分钟 critical 是初始双门槛。若目标环境的恢复时间较长，可把 warning 提前到 60%-70%；若 telemetry 峰值很短且卷余量充足，可延长 warning 的 `for`，但不应把 critical 阈值抬高到来不及处理。80% 与 95% 会同时满足时，应由 Alertmanager 用同一实例和容量维度的 critical 抑制 warning，避免重复通知。backlog 的 15 分钟门槛也应大于正常 replay 恢复时间；默认 replay 是每 30 秒最多 100 条，因此应根据实际积压量和数据库恢复吞吐重新校准。

## Multi-instance boundary

这些 Go 指标没有 tenant、device 或业务分片 label；它们只继承 Prometheus 抓取时添加的 `job`、`instance` 以及部署方 relabel 后的 `cluster`、`namespace`、`pod` 等 label。规则没有执行跨实例 `sum`，会保留每条源时间序列的完整 label 集，因此一个实例接近容量不会被其他空闲实例稀释。

- 每个 backend 实例应拥有自己的 spool 目录/卷，并以唯一 `instance` 或 `pod` 被抓取。
- 不要让多个 backend 进程共享同一 spool 目录；当前文件 spool 的多进程共享和 fencing 没有运行证明。
- fleet dashboard 可对独立实例的 backlog 求和，但容量告警应看“每实例比例的最大值”，不要用 `sum(usage) / sum(capacity)` 掩盖热点实例。
- 若 Prometheus HA、副本抓取或 federation 会复制同一 target 的时间序列，应保留可去重的 `cluster`/`prometheus_replica` 等外部 label，并在 Alertmanager 层按部署规范去重。

## Metric contract

规则只引用 `backend/internal/storage/metrics.go` 已定义的真实指标：

- `AetherLinkIoT_storage_telemetry_spool_failures_total`
- `AetherLinkIoT_storage_telemetry_spool_corrupt_total`
- `AetherLinkIoT_storage_telemetry_spool_backlog_records`
- `AetherLinkIoT_storage_telemetry_spool_bytes`
- `AetherLinkIoT_storage_telemetry_spool_quarantine_records`
- `AetherLinkIoT_storage_telemetry_spool_capacity_records`
- `AetherLinkIoT_storage_telemetry_spool_capacity_bytes`

排障时还可以关联下面三个已存在的指标：

- `AetherLinkIoT_storage_telemetry_spooled_total`
- `AetherLinkIoT_storage_telemetry_spool_replayed_total`
- `AetherLinkIoT_storage_telemetry_spool_quarantine_bytes`

指标没有设备或 payload label，不应为排障临时加入高基数或敏感 label。

## Write failure

1. 先看同实例的 backlog、bytes、两项 capacity 和宿主卷的可用空间/inode。
2. 检查 backend 日志中的 `store telemetry file spool record`，区分容量耗尽、目录权限、只读文件系统和 I/O 错误。
3. 同时检查主 PostgreSQL 与数据库 dead-letter 写入；只有两者都失败后才进入文件 spool。
4. 不要通过删除旧 spool/quarantine 文件来让告警变绿。先保全受限副本，再恢复卷和数据库写入能力。
5. 恢复后确认 failure counter 不再增加、replayed counter 增长且 backlog 下降。counter 本身不会回到零。

## Spool activated

1. 先确认告警实例的 `spooled_total` 确有增量，并检查同实例 backlog 是否开始增长。
2. 同时调查主 PostgreSQL 写入和 `telemetry_dead_letters` 写入；只有两者都失败后，文件 spool 才会接管该数据点。
3. spool 激活本身说明耐久 fallback 已生效，不等于实时写入正常，也不能静默关闭事件。
4. 数据库恢复后确认 `replayed_total` 增长、backlog 回到零，并持续观察 `spooled_total` 不再增加。

## Corruption detected

1. 记录告警实例和时间，关联 `corrupt_total` 增量与 backend 损坏/隔离日志。
2. 保留 `.corrupt*` 文件的受限副本，检查宿主磁盘、文件系统错误、异常断电和卷挂载变化。
3. 损坏文件包含原始 telemetry，分析和传输时沿用生产数据访问控制。
4. 自动 replay 不会读取 quarantine；任何手工恢复、重写或删除都必须经过数据负责人确认并留下审计记录。

## Persistent backlog

1. 先检查 `quarantine_records`。backlog 包含 quarantine，全部积压若都是 quarantine，转到 quarantine runbook。
2. 对比同实例 `spooled_total` 与 `replayed_total` 的增速，确认 backlog 是继续增长、稳定还是正在排空。
3. 检查 PostgreSQL 可用性以及 `replay_interval`、`replay_batch_size`、`replay_timeout` 的有效配置和 backend 日志。
4. 数据库恢复后持续观察到 backlog 归零；不要仅凭一次 replay 成功日志关闭事件。

## Capacity near limit

1. 同时查看 records 与 bytes 比例，确认先触及哪个限制，并检查 quarantine 占用。
2. 先恢复 PostgreSQL/重放能力；若必须临时扩容，确保实际卷空间和 inode 足够，再调整实例对应的 `max_bytes`/`max_records` 并按正常发布流程重启。
3. 记录扩容前后的 capacity gauge。若 gauge 未随配置变化，说明有效配置或实例重启仍有问题。
4. spool 达到任一上限时不会淘汰旧点；后续 fallback 写入会失败并触发 critical write-failure 告警。

## Quarantine present

1. quarantine 不会自动重放且持续占用 records/bytes 容量，应由当班人员创建人工处置记录。
2. 先保全文件和相关日志，再由数据负责人判断能否恢复有效 payload、是否允许重放以及何时删除。
3. 人工处置后同时确认 `quarantine_records`、`quarantine_bytes` 和总 backlog/bytes 已按预期下降。
4. 若 quarantine 在清理后再次出现，按底层存储或写入完整性故障升级，而不是重复静默删除。
