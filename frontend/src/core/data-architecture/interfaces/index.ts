/**
 * 文件用途: 数据架构纯类型导出入口。
 * 核心逻辑: 聚合编辑器、组件数据和组件配置管理契约，不加载运行时实现。
 * 关键注意事项: 运行时桥接器和适配器应从各自模块显式导入，避免接口引用扩大依赖图。
 * 重构建议: 按领域继续拆分 public/internal 类型，减少临时契约被外部长期依赖。
 */


// 编辑器大数据管理
export type {
  IEditorDataManager,
  EditorData,
  EditorDataChangeEvent
} from '@/core/data-architecture/interfaces/IEditorDataManager'

// 组件配置管理
export type {
  IComponentConfigManager,
  WidgetConfiguration,
  ConfigLayer,
  BaseConfig,
  ComponentConfig,
  DataSourceConfig,
  InteractionConfig,
  ConfigChangeEvent,
  ConfigValidationResult
} from './IComponentConfigManager'

// 运行时数据管理
export type {
  IComponentDataManager,
  ComponentDataRequirement,
  DataSourceDefinition,
  DataSourceType,
  DataExecutionResult,
  ComponentDataState,
  DataUpdateEvent
} from './IComponentDataManager'

/**
 * 数据架构设计原则：
 *
 * 1. **职责分离**：三个数据层各司其职，不互相调用
 *    - EditorDataManager：管理组件树和画布配置
 *    - ComponentConfigManager：管理组件四层配置
 *    - ComponentDataManager：管理运行时数据
 *
 * 2. **事件驱动**：层间通过事件通信，不直接调用方法
 *    - 配置变更 → 发出事件 → 数据层监听并更新
 *    - 数据更新 → 发出事件 → UI层监听并重渲染
 *
 * 3. **简单直接**：避免复杂的状态管理和依赖链
 *    - 不做轮询、连接池等重型功能
 *    - 错误容忍，不阻塞界面
 *    - 缓存策略简单明了
 *
 * 4. **渐进重构**：接口先行，实现逐步替换
 *    - 先定义清晰接口
 *    - 新实现与旧系统并存
 *    - 逐步切换到新架构
 */
