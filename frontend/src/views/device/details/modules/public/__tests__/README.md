# __tests__

## 目录职责

公共展示组件测试目录。

## 文件关系

- 测试文件对应 `../public` 下地图和分布表格组件。
- 外部地图、图表或浏览器 API 应使用 mock，避免测试依赖真实网络。

## 重点文件

- `tencent-map.test.ts`: 地图组件测试。
- `distribution-and-table.test.ts`: 分布与表格组件测试。

## 审查建议

建议补齐无坐标、SDK 失败、空分布和筛选联动场景。
