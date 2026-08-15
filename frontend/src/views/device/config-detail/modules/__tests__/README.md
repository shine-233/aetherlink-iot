# __tests__

## 目录职责

设备配置详情模块测试目录。

## 文件关系

- 每个测试对应同名 `../*.vue` 模块，覆盖配置展示、保存、副作用和列表行为。
- 共享配置 fixture 应覆盖协议、模板、设备类型和扩展信息。

## 重点文件

- `connection-info.test.ts`: 连接配置核心测试。
- `form.test.ts`: 动态表单测试。
- `associated-devices.test.ts`: 关联设备列表测试。
- `alarm-info.test.ts`: 告警配置测试。

## 审查建议

重点审查保存回填、字段缺失、接口失败和批量关联设备影响。
