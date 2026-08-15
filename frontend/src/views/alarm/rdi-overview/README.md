# RDI 告警概览

`rdi-overview` 目录为告警域提供 RDI 概览入口。

## 目录职责

- `index.vue` 复用 `dashboard/rdi-overview` 页面组件。
- `__tests__/` 验证包装组件挂载和透传契约。

## 维护注意

- 包装层不应改变 dashboard 组件业务逻辑。
- 若告警域需要差异化行为，应先明确 props 或组合边界，再补充测试。