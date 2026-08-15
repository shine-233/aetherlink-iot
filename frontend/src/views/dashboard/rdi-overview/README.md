# frontend/src/views/dashboard/rdi-overview 目录说明

## 目录职责

本目录存放 `frontend/src/views/dashboard/rdi-overview` 对应的前端页面、局部组件或页面级支撑文件，属于 `frontend/src/views` 文档化第一分片。

## 内容结构

- `index.vue`：设备状态总览、告警历史、设备快照展示，以及可选择年份的 12 个月告警次数趋势。
- `useRdiDeviceSnapshots.ts`：设备快照分页请求、受限并发补数、竞态隔离、空闲调度与卸载失效处理。
- `rdiOverviewState.ts`：页面与快照 composable 共用的纯转换、格式化和月度趋势归一化 helper。
- 子目录：`__tests__/`

## 维护规则

- 只在本目录内维护与相邻页面直接相关的展示、交互和测试内容。
- 修改页面行为时，同步检查相邻 `__tests__` 是否需要补充断言。
- 保持路由名、用户可见文案和测试选择器稳定；必要调整时要同步更新说明和测试。
- 年度趋势通过 `GET /alarm/info/history/monthly?year=YYYY&timezone=Asia/Shanghai` 获取；页面传浏览器 IANA 时区，后端按同一时区计算年界和月份。普通租户用户必须沿用设备 `owner_user_id` 可见范围，不能退回租户全量统计。

## 已知缺口

- 本 README 只说明静态职责，不代表已完成真实后端联调或 E2E 业务闭环。
- focused Vitest 已覆盖页面与设备快照 composable；图表视觉细节、真实后端权限链路和跨页导航仍需更高层级验证。
