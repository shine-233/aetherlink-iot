/**
 * 文件用途：组件映射系统相关类型。
 * 关键注意事项：由 enhanced-types.ts 拆分而来，enhanced/index.ts 统一重导出，外部引用路径保持不变。
 */
// ==================== 组件映射系统 ====================

/**
 * 组件映射配置接口
 * 用于Card2.1组件属性与HTTP参数的映射
 */
export interface ComponentMappingConfig {
  /** 映射配置唯一标识 */
  id: string

  /** 映射名称 */
  name: string

  /** 源组件信息 */
  sourceComponent: ComponentMappingSource

  /** 目标HTTP配置信息 */
  targetHttpConfig: HttpConfigMappingTarget

  /** 映射关系列表 */
  mappings: PropertyToParameterMapping[]

  /** 映射状态 */
  status: 'active' | 'inactive' | 'error'

  /** 映射创建时间 */
  createdAt: number

  /** 最后更新时间 */
  lastUpdated: number

  /** 映射描述 */
  description?: string
}

/**
 * 组件映射源信息
 */
export interface ComponentMappingSource {
  /** 组件/卡片ID */
  componentId: string

  /** 组件类型 */
  componentType: string

  /** 组件显示名称 */
  displayName?: string

  /** 可映射的属性列表 */
  availableProperties: ComponentProperty[]
}

/**
 * HTTP配置映射目标信息
 */
export interface HttpConfigMappingTarget {
  /** HTTP配置标识 */
  configId: string

  /** HTTP配置名称 */
  configName?: string

  /** 可映射的参数列表 */
  availableParameters: HttpMappableParameter[]
}

/**
 * 组件属性定义
 */
export interface ComponentProperty {
  /** 属性名称 */
  name: string

  /** 属性显示名称 */
  displayName: string

  /** 属性数据类型 */
  dataType: 'string' | 'number' | 'boolean' | 'object' | 'array'

  /** 属性当前值 */
  currentValue?: unknown

  /** 属性描述 */
  description?: string

  /** 是否为只读属性 */
  readonly?: boolean

  /** 属性路径（用于嵌套对象） */
  path?: string
}

/**
 * HTTP可映射参数信息
 */
export interface HttpMappableParameter {
  /** 参数标识（对应variableName或占位符名称） */
  parameterId: string

  /** 参数显示名称 */
  displayName: string

  /** 参数类型（header/param/url/body） */
  parameterType: 'header' | 'param' | 'url' | 'body'

  /** 数据类型 */
  dataType: 'string' | 'number' | 'boolean' | 'json'

  /** 参数描述 */
  description?: string

  /** 是否为必填参数 */
  required?: boolean

  /** 参数在配置中的路径 */
  configPath: string
}

/**
 * 属性到参数的映射关系
 */
export interface PropertyToParameterMapping {
  /** 映射关系唯一标识 */
  id: string

  /** 源组件属性 */
  sourceProperty: ComponentProperty

  /** 目标HTTP参数 */
  targetParameter: HttpMappableParameter

  /** 映射类型 */
  mappingType: 'direct' | 'transform' | 'conditional'

  /** 数据转换配置（mappingType为transform时使用） */
  transformation?: DataTransformationConfig

  /** 条件映射配置（mappingType为conditional时使用） */
  condition?: ConditionalMappingConfig

  /** 映射状态 */
  status: 'active' | 'inactive' | 'error'

  /** 最后同步时间 */
  lastSyncTime?: number
}

/**
 * 数据转换配置
 */
export interface DataTransformationConfig {
  /** 转换类型 */
  type: 'format' | 'calculate' | 'lookup' | 'script'

  /** 转换参数 */
  parameters: Record<string, unknown>

  /** 转换脚本（type为script时使用） */
  script?: string
}

/**
 * 条件映射配置
 */
export interface ConditionalMappingConfig {
  /** 条件表达式 */
  condition: string

  /** 条件成立时的值 */
  trueValue: unknown

  /** 条件不成立时的值 */
  falseValue: unknown

  /** 条件评估上下文 */
  context?: Record<string, unknown>
}
