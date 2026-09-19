# P1.5 真实边缘节点联调与断云演练证据（2026-09-19）

> 对应路线图：§1.2 P1.5 剩余项「真实边缘节点联调与断云演练（需真机/活栈）」
> 结论：**该项闭环**——一个真实运行的边缘客户端进程对平台真实 HTTP API 完成
> 注册→稳态→断云→恢复全周期，P1.3 式四面对齐后 **P1.5 翻 `done`**。

## 一、演练形态

`automation_tests/scripts/edge_node_simulator.js`：**独立运行的边缘客户端进程**
（子进程，stdio 隔离），以租户 x-token 认证，全部走平台真实 HTTP API：
注册（POST /edge/nodes）→ 心跳循环（POST /edge/nodes/:id/heartbeat）→
Reconcile（POST /edge/nodes/:id/reconcile，上报本地资源版本、接收编排计划）。
回执 JSONL（逐行）为唯一判定事实源，测试进程只读回执不下场干预。

## 二、断云演练时间线（64_edge_node_outage_drill.test.js，1/1，25.3s）

| 阶段 | 边缘侧行为 | 回执证据 |
| --- | --- | --- |
| 注册 | node_id + 版本 + 能力上报 | `registered ok=true` |
| 稳态 | 心跳 + 隔拍 Reconcile，health=online | 多条 `heartbeat ok=true` + `reconcile` |
| **断云窗口**（8s 起，6s） | API 基址切至不可达端口——心跳失败、**指数退避重试**、本地状态保留（节拍计数不丢） | `outage_start` → 多条 `heartbeat ok=false`（含失败原因）→ `outage_end` |
| **云恢复** | 切回真实基址 | `recovered`（恢复后首次心跳成功）→ `heartbeat ok=true` → `reconcile` 重新收敛 |
| 平台侧终态 | 权威健康判定在平台 | GET /edge/nodes 中该节点 health ≠ offline |

断云门禁逐条对账：**断云自治不丢本地数据**✓（窗口内状态保留、恢复后无丢失）；
**重连后按版本同步**✓（Reconcile 上报本地 revision 重新收敛）；**冲突进人工
可见状态**✓（DetectEdgeSyncConflict 只检测不合并，单测锁定）；**节点离线和
升级失败产生告警**✓（ClassifyEdgeNodeHealth + 升级流水，48 组 13/13，09-15）。

## 三、边界（如实）

- 边缘侧为**真实客户端进程（模拟器）**，非物理硬件——与 P0.3 的 OTA 设备模拟器
  同一先例（"真实设备/broker 或协议 stub"的 stub 语义）；断云从边缘视角是
  云不可达，后端进程本身不重启（其可用性由窗口外健康检查保证）。
- Reconcile 的网关设备 ID 来自种子设备；编排计划内容依赖平台资源清单，
  本演练断言收敛行为而非具体下发清单。

## 四、工程记录

- 边缘端点要求认证（401 missing authentication）；模拟器以 `x-token` 携带
  登录令牌——**getToken 是 async，首版忘记 await 把 Promise 传成了令牌**
  （平台如实返回 "token has expired"），修复后通过。
