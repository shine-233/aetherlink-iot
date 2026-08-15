# __tests__

## 目录职责

配置详情内部组件测试目录。

## 文件关系

- `topic-mapping-modal.test.ts` 对应 `../topic-mapping-modal.vue`，验证映射弹窗的表单和提交契约。
- 测试应使用协议配置 fixture，避免依赖父组件真实挂载。

## 重点文件

- `topic-mapping-modal.test.ts`: topic 映射弹窗测试。

## 审查建议

建议补重复 topic、空行删除、取消关闭和提交失败场景。
