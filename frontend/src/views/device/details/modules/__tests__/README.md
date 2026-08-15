# __tests__

## 目录职责

设备详情业务 tab 测试目录。

## 文件关系

- 每个 `*.test.ts` 对应同名 `../*.vue` 模块，覆盖状态、命令、消息、告警、RDI 入口等低层面板。
- 共享设备 fixture 应保持字段和详情页父组件传参一致。

## 重点文件

- `RdiDeviceOperationsView.test.ts`: RDI 综合操作视图测试。
- `command-delivery.test.ts`: 命令下发风险路径测试。
- `telemetry-chart.test.ts`: 遥测图表入口测试。

## 审查建议

重点检查测试是否断言业务结果和失败提示，而不是只验证组件可挂载。
