# 场景编辑页说明

## 目录职责

该目录负责普通自动化场景的新建与编辑，重点是维护动作表单状态，并在提交前转换成后端 `actions` 结构。

## 关键文件

- `index.vue`：页面入口与保存流程。
- `scene-action-form-state.ts`：动作表单联动状态辅助函数。
- `scene-action-mappers.ts`：动作分组态与接口扁平态之间的双向映射。

## 维护提示

- `buildActionsPayload` 与回显侧 mapper 需要保持对称。
- 设备动作的参数类型较多，改动时要重点确认 JSON 输入和回填逻辑。
