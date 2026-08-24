# gen 继承链风险收敛清单（P1 后续）

## 背景

2026-08-23 P1 专项确认：gorm/gen 包级表单例的继承式语句根在高并发下会跨请求残留
`Statement.Model/Dest`，导致 UPDATE/DELETE 注入陈旧 `id=` 条件、SELECT 读到旧快照
（症状：假成功删除、record-not-found、空列表）。机制与修复样例见 VALIDATION.md
2026-08-23 P1 记录与提交 c6f24ff / b3235ee / 3316b51。

## 已收敛面

- `device_config` 全部读写（raw 链 + RowsAffected 守卫）
- `users`：登录选择器（email/phone）、UpdateLastVisitTime、列表 count/find、user/detail 重写
- `data_script`：UpdateDataScript（显式列赋值 map）
- 回归守卫：`backend/internal/dal/device_config_isolation_test.go`

## 剩余风险清单（静态扫描，按文件内链式调用数排序）

Device(3) / User(3) / DataPolicy(2) / AlarmHistory(2) / Group(2) /
DeviceModelCommand(2) / DeviceTemplate(2) / NotificationGroup(2) / Board(2) /
DeviceModelCustomControl(2) / DataScript(2, 已部分收敛) / ExpectedData(2) /
DeviceModelCustomCommand(2) / SysUIElement(2) / DeviceModelAttribute(2) …

均为低频路径（无实测症状），无需立即全量改造。建议触发条件任一满足时继续：

1. 对应表出现负载下偶发 record-not-found / 更新后读旧值工单；
2. 该表进入高频调用路径（如新遥测聚合端点）；
3. gorm/gen 升级引入语句克隆语义变化时全量复查。

## 收敛方法（同构模板）

1. 将该表热点函数的 `query.<T>.Where(...).Xxx()` 改为 `global.DB.Table("<t>")...`
   或 `global.DB.Model(&model.T{}).Where(...)`——clone==1 根，每次起点全新 Statement；
2. 写操作显式构造列赋值（struct 非零语义用 if-nil 判断展开为 map），并检查 RowsAffected；
3. 参照 `device_config_isolation_test.go` 补确定性回归测试；
4. 同步 VALIDATION.md 与本清单勾销。
