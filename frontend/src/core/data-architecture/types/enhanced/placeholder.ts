/**
 * 文件用途：动态参数与占位符解析相关类型。
 * 关键注意事项：由 enhanced-types.ts 拆分而来，enhanced/index.ts 统一重导出，外部引用路径保持不变。
 */
// ==================== 动态参数系统与占位符解析 ====================

/**
 * 动态参数定义（预留接口）
 * 用于HTTP动态参数系统
 */
export interface DynamicParam {
  /** 参数名称 */
  name: string

  /** 参数类型 */
  type: 'string' | 'number' | 'boolean' | 'object'

  /** 当前参数值 */
  currentValue: unknown

  /** 种子值 */
  exampleValue?: unknown

  /** 参数描述 */
  description?: string

  /** 是否必填参数 */
  required?: boolean

  /** 参数验证规则 */
  validation?: {
    /** 最小值/最小长度 */
    min?: number
    /** 最大值/最大长度 */
    max?: number
    /** 正则表达式 */
    pattern?: string
    /** 枚举值 */
    enum?: unknown[]
  }
}

/**
 * 占位符配置接口
 * 用于{{variableName}}占位符系统
 */
export interface PlaceholderConfig {
  /** 占位符名称（不包含{{}}） */
  name: string

  /** 占位符当前值 */
  value: unknown

  /** 数据类型 */
  dataType: 'string' | 'number' | 'boolean' | 'json'

  /** 是否为必填占位符 */
  required: boolean

  /** 占位符描述 */
  description?: string

  /** 默认值 */
  defaultValue?: unknown

  /** 验证规则 */
  validation?: PlaceholderValidationRule
}

/**
 * 占位符验证规则
 */
export interface PlaceholderValidationRule {
  /** 最小值/最小长度 */
  min?: number

  /** 最大值/最大长度 */
  max?: number

  /** 正则表达式（字符串类型时使用） */
  pattern?: string

  /** 枚举值列表 */
  enum?: unknown[]

  /** 自定义验证函数名称 */
  customValidator?: string
}

/**
 * 占位符依赖分析结果
 */
export interface PlaceholderDependencyAnalysis {
  /** 分析的配置对象标识 */
  configId: string

  /** 发现的所有占位符名称列表 */
  placeholders: string[]

  /** 占位符详细信息映射 */
  placeholderDetails: Map<string, PlaceholderDependencyDetail>

  /** 是否检测到循环依赖 */
  hasCircularDependency: boolean

  /** 循环依赖路径（如果存在） */
  circularDependencyPaths?: string[][]
}

/**
 * 单个占位符的依赖详情
 */
export interface PlaceholderDependencyDetail {
  /** 占位符名称 */
  name: string

  /** 在配置中出现的位置路径 */
  occurrences: PlaceholderOccurrence[]

  /** 依赖的其他占位符 */
  dependencies: string[]

  /** 被依赖的占位符 */
  dependents: string[]
}

/**
 * 占位符出现位置信息
 */
export interface PlaceholderOccurrence {
  /** 配置对象路径（如: "config.url", "config.headers[0].value"） */
  path: string

  /** 原始值（包含占位符的字符串） */
  originalValue: string

  /** 在该值中的占位符位置 */
  position: {
    start: number
    end: number
  }
}
