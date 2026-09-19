# P0.4 场景与 Flow 服务重启后调度不丢任务实测演练证据（2026-09-19）

> 对应路线图：§3 P0.4「场景与 Flow 语义」——门禁第四条：**服务重启后调度不丢任务**
> 演练脚本：`automation_tests/scripts/p04_restart_drill.js`
> 对应工件：`automation_tests/scratch/p04_state.json`
> 运行结果：**VERDICT=PASS**

---

## 一、验证背景

P0.4 场景与 Flow 语义在此前的权威状态表中已处于 `done`（真实 E2E 31 号用例全绿），但路线图注记中曾如实写明：
> "重启不丢调度"由 DB 持久化窗口（91.sql）+ 启动 cron 重载结构性保证（无专门重启演练，如实注明）。

为彻底消除这一口径层面的保留注记，本次采用真实两阶段独立脚本执行真实后端进程重启演练。

---

## 二、两阶段演练流程

1. **第一阶段（Arm）**：
   - 调用 `node scripts/p04_restart_drill.js arm --state-file scratch/p04_state.json --fire-in 120`；
   - 登录租户管理员，通过标准 API `POST /scene_automations` 创建一次性定时触发任务（`trigger_conditions_type: '20'`，`execution_time: now + 120s`，`expiration_time: 10`，`action_type: '20'` 激活普通场景种子）；
   - 将生成的 `automation_id`、计划触发时刻写入状态文件。

2. **第二阶段（Restart 后端）**：
   - 强制停止运行中的后端进程（`Stop-Process -Force`），记录重启基线时刻（`2026-09-19T14:24:28.000Z`）；
   - 重新启动编译后的最新 `backend.exe`，等待 9999 端口和 `/health` 端点重新恢复健康（200 OK）；
   - 后端启动时执行 `croninit`，从 PostgreSQL 重新加载未执行的计划任务窗口进调度引擎。

3. **第三阶段（Verify）**：
   - 调用 `node scripts/p04_restart_drill.js verify --state-file scratch/p04_state.json --restart-at 2026-09-19T14:24:28.000Z`；
   - 持续轮询 `/scene_automations/log`，断言必须捕获到触发时刻落于重启完成之后、且与原计划时刻高度吻合的执行日志。

---

## 三、实测日志与输出

```
ARM 阶段：
ARMED automation_id=33b1f75b-3dec-c11a-eb11-8ac05339fdfc fire_at=2026-09-19T14:26:01.434Z -> scratch/p04_state.json

VERIFY 阶段：
VERIFY automation_id=33b1f75b-3dec-c11a-eb11-8ac05339fdfc restart_at=2026-09-19T14:24:28.000Z
FIRED executed_at=2026-09-19T22:26:17.053027+08:00
VERDICT=PASS scheduled task survived the backend restart
```

---

## 四、结论

真实后端在被彻底 Kill 并重新冷启动后，预先持久化在数据库中的一次性定时场景联动在到达预定触发时间后准确执行，并生成了有效的执行流水日志。至此，P0.4「服务重启后调度不丢任务」从原先的"结构性保证"正式升级为**具备运行期实测演练证据支持的闭环事实**。
