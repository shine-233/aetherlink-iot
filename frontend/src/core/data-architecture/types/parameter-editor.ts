/**
 * 文件用途: 定义 DynamicParameterEditor 及其参数编辑模式使用的类型。
 * 核心逻辑: 描述动态参数、模板参数、编辑状态和 UI 元数据，支撑参数表单渲染。
 * 关键注意事项: 参数默认值、种子值和兼容字段会影响导入模板时的值保留行为。
 * 重构建议: 将 API 参数、设备参数和 UI 状态类型分层，减少编辑器组件对复合类型的耦合。
 */
/**
 * @file parameter-editor.ts
 * @description Defines types for the DynamicParameterEditor component.
 */

/**
 * EnhancedParameter - The data structure for a parameter in the dynamic editor.
 * It extends the basic parameter concept with UI-specific properties for different editing modes.
 */
export interface EnhancedParameter {
  /** The parameter key or name. */
  key: string

  /** The parameter value, which can be of any type depending on the context. */
  value: any

  /** Whether the parameter is currently enabled and should be included in the final output. */
  enabled: boolean

  /** Whether the parameter is dynamic (for property binding) */
  isDynamic?: boolean

  /** The editing mode for the parameter's value. */
  valueMode: 'manual' | 'dropdown' | 'property' | 'component'

  /** The ID of the selected template, if any. */
  selectedTemplate?: string

  /** The data type of the parameter's value. */
  dataType: 'string' | 'number' | 'boolean' | 'json'

  /** The variable name for property binding, automatically generated in most cases. */
  variableName?: string

  /** Unique identifier for Vue tracking (internal use) */
  _id?: string

  /** A user-provided description for the parameter. */
  description?: string

  /** Default value to use when the main value is empty (for property binding) */
  defaultValue?: any

  /** Device selection context (for device-generated parameters) */
  deviceContext?: {
    sourceType: 'device-selection' | 'manual' | 'template'
    selectionConfig?: any
    timestamp: number
  }

  /** Parameter group information (for grouped parameters) */
  parameterGroup?: {
    groupId: string
    role: 'primary' | 'secondary' | 'derived' | 'optional'
    isDerived: boolean
  }
}
