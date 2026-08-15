# script-engine 组件目录

## 目录定位

`frontend/src/core/script-engine/components` 现在只保留轻量脚本编辑入口，是脚本引擎面向页面和配置面板的精简 UI 层。

## 文件用途

- `SimpleScriptEditor.vue` 提供轻量编辑入口。
- `index.ts` 统一导出当前仍保留的组件。

## 维护边界

组件负责展示、输入和事件派发，不应绕过脚本引擎直接执行不受控代码。安全检查、上下文管理和沙箱执行应留在 `script-engine` 核心文件中。
