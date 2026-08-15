# 核心交互系统

## 模块定位

`interaction-system` 负责 AetherLink IoT 前端可视化编辑器中的组件交互配置、模板选择、交互预览和配置组件注册。它不直接承载业务页面，而是为编辑器和组件配置区提供可复用的交互能力。

## 目录结构

```text
interaction-system/
├── index.ts                         # 模块统一导出和初始化状态
├── interaction-engine.ts            # 交互动作用例执行入口
├── managers/
│   └── ConfigRegistry.ts            # 自定义配置组件注册表
└── components/
    ├── InteractionCardWizard.vue    # 简化交互配置向导
    ├── InteractionPreview.vue       # 交互配置预览视图
    ├── InteractionTemplatePreview.vue
    └── InteractionTemplateSelector.vue
```

## 核心职责

- 提供交互卡片的新增、编辑、启用、禁用和预览能力。
- 按模板分类展示预置交互，并支持模板预览、选择和导入。
- 执行交互动作用例，包括跨组件属性更新和导航类动作。
- 通过 `ConfigRegistry` 维护组件 ID 与自定义配置区的映射关系。

## 使用入口

```ts
import {
  InteractionCardWizard,
  InteractionPreview,
  InteractionTemplateSelector,
  configRegistry
} from '@/core/interaction-system'
```

## 维护注意事项

- 交互配置字段会被模板、预览、导入导出和运行时执行共同使用，调整字段时需要同步更新所有链路。
- 跨组件更新依赖编辑器节点和组件暴露能力，必须保留目标组件不存在或能力缺失时的保护逻辑。
- 模板导入需要持续校验 JSON 结构，避免把无效配置写入可视化编辑器状态。
- 本目录只维护交互系统本身，不应把页面级业务流程、布局逻辑或通用工具沉入这里。

## 重构方向

- 将交互动作用例拆成策略表或独立 handler，降低新增动作类型时的主流程修改成本。
- 抽出模板仓库、导入校验和预览执行器，减少 Vue 组件内职责耦合。
- 沉淀测试 fixture 和 Naive UI mock，降低交互组件单测重复度。
