# 设备配置详情内部组件

## 目录职责

`frontend/src/views/device/config-detail/modules/components` 保存设备配置详情模块内部复用组件，主要服务于连接信息和 Topic 映射等配置场景。

## 文件关系

- `topic-mapping-modal.vue`
  - 服务于连接信息模块，用于维护 Topic 映射配置。
- `__tests__`
  - 保存组件级测试资源。

## 静态审查建议

- 问题：Topic 映射弹窗属于协议接入敏感面，重复 Topic、空映射和编辑态回填都容易影响保存结果。
- 改进：继续保持 README 与组件文件头同步更新，明确字段约定、校验规则和回填边界。
- 预期效果：后续修改 Topic 映射逻辑时更容易快速评估影响面。
