# __tests__

## 目录职责

RDI composable 测试目录。

## 文件关系

- 每个测试对应 `../useRdi*.ts`，覆盖配置、遥测、历史、命令和分享组合函数。
- 测试通过 mock API、store 和定时器验证 composable 契约。

## 重点文件

- `useRdiConfig.test.ts`: 配置加载与保存测试。
- `useRdiTelemetry.test.ts`: 实时遥测与轮询测试。
- `useRdiCommands.test.ts`: 命令动作测试。

## 审查建议

重点审查失败分支、清理定时器、权限边界和 payload 与后端 RDI 合约是否一致。
