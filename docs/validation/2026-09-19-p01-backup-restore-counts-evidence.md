# P0.1 backup/restore 计数一致性证据（2026-09-19）

> 对应路线图：§1.2 P0.1 剩余四项之一「backup/restore 计数一致性」
> 结论：**该项闭环**——真实 pg_dump → manifest 校验 → psql 恢复到全新空库 →
> 五张核心表行数逐一比对一致，sys_version 保留。

## 一、执行链路（全部走脚本契约，非手工拼凑）

`scripts/roadmap/backup-restore.ps1` 三步：

1. **backup**：`pg_dump --no-owner`（plain SQL 格式，刻意如此：恢复只需 psql）
   → `aetherlink-20260919T022857Z.dump`（11,128,456 bytes）→ manifest（SHA-256 逐文件哈希）。
   `[PASS] backup-exec :: dump written`，**VERDICT=PASS**。
2. **restore**：先做 manifest 完整性校验（拒绝把损坏备份写进目标库），再以
   `-Force` 恢复到**全新空库** `aetherlink_p01_restore`（隔离集群 55433 上新建，
   不碰业务库）→ `[PASS] restore-exec`，**VERDICT=PASS**。
3. **计数一致性**：源库（aetherlink_go99）与恢复库逐表 count 比对：

| 表 | 源库 | 恢复库 |
| --- | --- | --- |
| devices | 32 | 32 |
| scene_info | 10 | 10 |
| alarm_config | 10 | 10 |
| telemetry_datas | 85,869 | 85,869 |
| users | 7 | 7 |

`COUNT_CONSISTENCY=PASS`；恢复库 `sys_version = 115`（= VERSION_NUMBER，迁移状态保留）。

## 二、P0.1 剩余（如实）

四项中的另三项仍卡真实部署环境：目标服务器 HTTPS/TLS、MQTTS 设备上报/下发、
公网 MQTT。backup/restore 项此后不再阻塞 P0.1 的推进评估。
