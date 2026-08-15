# 交互系统快速开始

## 目录定位

这是一份面向接入方的最小启动说明，适合先确认“从哪里导入、能做什么、哪些地方要小心”。

## 先看结论

当前目录真正值得优先使用的能力是：

- `InteractionCardWizard`：简化版配置入口。
- `InteractionTemplateSelector`：模板选择入口。
- `InteractionPreview`：交互效果预览入口。
- `configRegistry`：自定义配置视图注册表。

补充说明：

- `components/InteractionTemplatePreview.vue` 目前存在于目录中，但未在 `index.ts` 统一导出，建议按内部文件使用，不要把它当作稳定公共 API。

## 最小接入

```ts
import {
  InteractionCardWizard,
  InteractionTemplateSelector,
  InteractionPreview,
  configRegistry,
  initializeSettings
} from '@/core/interaction-system'

initializeSettings()
```

```vue
<template>
  <n-card title="交互设置">
    <InteractionCardWizard
      v-model="interactions"
      :component-id="componentId"
      :component-type="componentType"
    />
  </n-card>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { InteractionCardWizard } from '@/core/interaction-system'

const interactions = ref([])
const componentId = ref('component-001')
const componentType = ref('chart-component')
</script>
```

## 常见组合

### 1. 模板选择

```vue
<template>
  <InteractionTemplateSelector @select="applyTemplate" @cancel="closeSelector" />
</template>

<script setup lang="ts">
import { InteractionTemplateSelector } from '@/core/interaction-system'

const applyTemplate = (template) => {
  interactions.value.push(template)
}
</script>
```

### 2. 效果预览

```vue
<template>
  <InteractionPreview
    :interactions="interactions"
    :component-id="componentId"
    @close="closePreview"
  />
</template>
```

### 3. 自定义配置注册

```ts
import { configRegistry } from '@/core/interaction-system'

configRegistry.register('my-component', {
  component: MyConfigView,
  props: {},
  validators: {}
})
```

## 使用注意事项

1. 当前目录的导出以 `index.ts` 为准，不要把历史文档中的旧组件名当作稳定接口。
2. 接入前先确认组件 ID 和组件类型已经在你的编辑器上下文中定义好。
3. 模板选择和预览更适合编辑器/配置页，不适合直接放到业务提交链路里。
4. 如果你要做自定义配置注册，记得提前设计好清理策略。

## 静态审查建议

- 先看导入路径是否统一走 `@/core/interaction-system`。
- 先看是否真的需要注册表，能用现成模板时不要过度扩展。
- 先看组件预览是否会被重复挂载，避免状态污染。

## 后续改进方向

- 给 `InteractionTemplateSelector` 和 `InteractionPreview` 补一张完整流程图。
- 为模板和注册表增加接入约定文档，减少不同页面的接法差异。
- 如果交互配置越来越大，可以把模板定义和运行时执行逻辑拆分成更清晰的子包。
