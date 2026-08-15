# 交互系统 API

## 目录定位

`frontend/src/core/interaction-system/` 是交互系统的核心目录。这里既包含对外导出的组件，也包含注册表和初始化入口。当前目录的实际导出以 `index.ts` 为准，`API.md` 只描述这份导出面。

## 文件关系

- `index.ts` 是统一出口。
- `components/` 放组件实现。
- `managers/ConfigRegistry.ts` 放配置注册表。
- `interaction-engine.ts` 提供交互执行相关能力。
- `README.md` 适合做目录总览，`API.md` 适合做接口参考。

## 对外导出

通过 `@/core/interaction-system` 可以直接拿到以下内容：

- `InteractionCardWizard`
- `InteractionTemplateSelector`
- `InteractionPreview`
- `configRegistry`
- `ConfigRegistry`
- `initializeSettings`
- `getInteractionSystemStatus`

## 组件 API

### `InteractionCardWizard`

简化版交互配置向导，适合在卡片式编辑场景中使用。

#### Props

| 属性 | 类型 | 默认值 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| `modelValue` | `any[]` | `[]` | 否 | 当前交互配置列表；未传入时使用空列表。 |
| `componentId` | `string` | - | 否 | 组件 ID；缺省时相关配置查询会走安全降级。 |
| `componentType` | `string` | - | 否 | 组件类型。 |

#### Events

| 事件 | 参数 | 说明 |
| --- | --- | --- |
| `update:modelValue` | `any[]` | 配置变更时触发。 |

#### 说明

这个组件面向“快速创建、编辑、删除、启停交互”的轻量场景，适合移动端或简化配置页。

### `InteractionTemplateSelector`

交互模板选择器，用于从内置模板中快速生成配置。

#### Props

无需显式传入。

#### Events

| 事件 | 参数 | 说明 |
| --- | --- | --- |
| `select` | `InteractionConfig` | 选择模板时触发。 |
| `cancel` | 无 | 取消选择时触发。 |

### `InteractionPreview`

交互预览组件，用于在页面中验证交互配置是否按预期触发。

#### Props

| 属性 | 类型 | 默认值 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| `interactions` | `InteractionConfig[]` | - | 是 | 待预览的交互配置。 |
| `componentId` | `string` | - | 否 | 关联组件 ID；用于需要组件上下文的预览动作。 |

#### Events

| 事件 | 参数 | 说明 |
| --- | --- | --- |
| `close` | 无 | 关闭预览时触发。 |

## 注册表 API

### `configRegistry`

用于注册、查询和移除自定义配置视图的注册表实例。

#### 方法

| 方法 | 参数 | 返回值 | 说明 |
| --- | --- | --- | --- |
| `register` | `(componentId: string, configComponent: IConfigComponent)` | `void` | 注册配置组件。 |
| `get` | `(componentId: string)` | `IConfigComponent \| undefined` | 获取配置组件。 |
| `has` | `(componentId: string)` | `boolean` | 判断是否已注册。 |
| `getAll` | `()` | `ConfigComponentRegistration[]` | 获取全部注册项。 |
| `clear` | `()` | `void` | 清空全部注册项。 |
| `unregister` | `(componentId: string)` | `boolean` | 移除指定注册项。 |

### 初始化状态

`initializeSettings()` 会初始化系统状态并返回快照。`getInteractionSystemStatus()` 则用于读取当前状态。

返回值中包含：

- `initialized`
- `initializedAt`
- `components`
- `registeredConfigComponents`

## 推荐接入方式

```ts
import {
  InteractionCardWizard,
  InteractionTemplateSelector,
  InteractionPreview,
  configRegistry,
  initializeSettings
} from '@/core/interaction-system'

const status = initializeSettings()

if (!configRegistry.has('my-component')) {
  configRegistry.register('my-component', {
    component: MyConfigView,
    props: {},
    validators: {}
  })
}
```

## 使用注意事项

1. 这份 API 文档以 `index.ts` 的真实导出为准，旧文档里出现但当前目录并未导出的组件，不应再继续当作稳定接口使用。
2. 注册表适合管理“按组件 ID 绑定的配置视图”，不要拿它做运行时业务状态仓库。
3. 如果项目中存在热更新、懒加载或动态注册，务必确认 `initializeSettings()` 只在需要的时候调用一次。
4. 模板选择器和预览组件更适合编辑器场景，不适合直接承担业务提交逻辑。

## 静态审查建议

- 检查所有调用方是否都从统一出口导入，避免深路径依赖分散。
- 检查注册表使用方是否有清理逻辑，防止重复注册。
- 检查组件预览和模板能力是否与当前目录真实导出一致，避免文档先于代码漂移。
- 检查对外文档中是否还残留旧版组件名或旧版示例代码。

## 后续改进方向

- 将组件 API、注册表 API 和初始化 API 拆成独立章节或独立文档。
- 为注册表补充生命周期示意图，降低接入方误用概率。
- 为预览和模板选择器补充更完整的事件流说明。
