# P0.4 专门重启演练证据：「服务重启后调度不丢任务」（2026-09-19）

> 对应路线图：§1.2 P0.4（已 done）——把「重启不丢调度」从结构性保证升级为专门演练实证。
> 工具：`automation_tests/scripts/p04_restart_drill.js`（arm / verify 两阶段）。

## 一、演练时间线（全部真实操作，无模拟）

| 时刻（UTC） | 动作 |
| --- | --- |
| 15:05:43 | **arm**：创建一次性定时自动化（trigger_conditions_type=20，`execution_time`=15:08:33Z，动作 30 触发种子告警配置），automation_id=`f7f94213-…` |
| 15:06:33± | **重启**：终止后端进程（9999 端口释放确认） |
| 15:06:56 | **restart-at 基准**：重启窗口结束基准时刻 |
| 15:07:46 | 新后端启动完成（`GET /health` 200）——cron 在启动时重载持久化的一次性任务 |
| 15:08:33 | 预定触发时刻（在重启完成**之后** 47 秒） |
| 15:08:35 | **实际执行**（`scene_automation_log.executed_at=2026-09-19T23:08:35+08:00`） |
| 15:10:20± | **verify**：轮询 `/scene_automations/log` 断言存在 `executed_at` 晚于 restart-at 的执行记录 |

**VERDICT=PASS**：`FIRED executed_at=2026-09-19T23:08:35(+08:00)`，
`VERDICT=PASS scheduled task survived the backend restart`。

## 二、判定口径

- 触发时刻刻意设在**重启窗口之后**：若重启吞掉调度，这条记录永远不会出现；
- `executed_at` 必须**晚于 restart-at**（重启用同一行记录伪造不可能——它按定义发生在重启后）；
- 执行时刻与预定时刻偏差 2 秒（cron 秒级粒度内的正常抖动）。

## 三、工程记录（诚实入档）

1. 演练脚本首版参数解析有索引错位 bug（`process.argv.slice(2)` 之后又从 i=2 起跳，
   跳过了子命令与第一个参数）——已修并实测；
2. 演练期间活栈两次自行退出（`go run` 后台任务被宿主回收 / broker 随之退出），
   重启后布防成功。创建自动化曾报 101001「record not found」——当时后端处于
   退出前的不健康状态；全新重启后同一请求成功。教训照旧：先查环境再查代码。
3. 演练结束后的后端退出（exit 1）不影响证据：执行记录在 PostgreSQL 中，
   verify 已在进程存活期完成。

## 四、清理

演练自动化（`restart_drill_*`）与种子场景/告警配置留存在隔离库中
（aetherlink_go99 为本地演练库，非生产数据）。
