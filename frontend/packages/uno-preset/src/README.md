# uno-preset 预设源码

## 目录定位
本目录提供 `@aetherlink/uno-preset` 的 UnoCSS 预设，用于沉淀项目常用 shortcuts 和主题扩展。

## 主要文件
- `index.ts`：定义并导出 `presetAetherLink`，当前包含常用 flex 布局快捷类。

## 依赖关系
依赖 `@unocss/core` 和 `@unocss/preset-uno` 类型。构建插件会在 `frontend/build/plugins/unocss.ts` 中加载该预设。

## 审查发现
当前预设较轻，`// @unocss-include` 指令需要保留在文件顶部，避免影响 UnoCSS 静态扫描。

## 重构建议
后续新增 shortcuts 时建议按布局、文本、状态等分组，并补充命名规则，避免快捷类含义漂移。

## 验证建议
优先执行 `pnpm exec eslint packages/uno-preset/src --ext .ts`。样式能力变更时建议在使用页面检查生成的原子类是否符合预期。
