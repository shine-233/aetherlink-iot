# Thing Model Component Tests

## 目录职责

覆盖物模型组件容器层的弹窗向导行为，重点保证新增/编辑模式、步骤组件映射和弹窗显隐状态稳定。

## 文件关系

- `template-modal.test.ts` 对应 `../template-modal.vue`。
- 测试通过 stub 步骤组件和 Naive UI 组件，把断言聚焦在向导容器本身。

## 重点文件

- `template-modal.test.ts`: 模板向导弹窗的核心容器测试。

## 审查建议

- 增减步骤时，必须同步更新组件映射断言。
- 修改弹窗标题或模式枚举时，确认新增和编辑模式都仍有覆盖。
- 子步骤复杂逻辑应放在 `step/__tests__/`，不要把步骤内部行为塞进容器测试。
