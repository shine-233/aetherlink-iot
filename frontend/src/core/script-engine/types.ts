/**
 * 文件用途：定义脚本引擎的执行结果、配置、模板、沙箱和上下文类型。
 * 核心逻辑：集中声明跨执行器、沙箱、模板管理器和上下文管理器共享的 TypeScript 契约。
 * 关键注意事项：这里的类型会影响多个核心模块，字段变更需要同步实现与测试。
 * 重构建议：可按执行、模板、沙箱、上下文拆分类型文件，再由入口统一导出。
 */

// 脚本执行结果
export interface ScriptExecutionResult<T = unknown> {
  /** 执行是否成功 */
  success: boolean
  /** 执行结果数据 */
  data?: T
  /** 错误信息 */
  error?: Error
  /** 执行时间（毫秒） */
  executionTime: number
  /** 执行上下文快照 */
  contextSnapshot?: Record<string, unknown>
  /** 日志输出 */
  logs: ScriptLog[]
}

// 脚本日志
export interface ScriptLog {
  level: 'log' | 'warn' | 'error' | 'info' | 'debug'
  message: string
  timestamp: number
  args?: unknown[]
}

// 脚本配置
export interface ScriptConfig {
  /** 脚本代码 */
  code: string
  /** 执行超时时间（毫秒） */
  timeout?: number
  /** 兼容字段：本地执行始终使用严格模式，不能通过此字段关闭 */
  strictMode?: boolean
  /** 兼容字段：本地执行固定使用异步函数包装 */
  asyncSupport?: boolean
  /** 保留契约：同线程沙箱无法可靠施加内存上限（字节） */
  maxMemory?: number
  /** 自定义全局变量 */
  globals?: Record<string, unknown>
  /** 保留契约：网络仍需可取消、可审计的宿主适配器，布尔值不会直接放行 */
  allowNetworkAccess?: boolean
  /** 保留契约：浏览器本地执行不提供文件系统访问 */
  allowFileSystemAccess?: boolean
}

/**
 * 脚本函数契约。
 * 参数使用 never 以保持逆变安全：任何具体签名的函数都可赋值，
 * 但引擎本身不直接调用这些函数（它们由用户脚本在沙箱内动态调用）。
 */
export type ScriptFunction = (...args: never[]) => unknown

// 脚本执行上下文
export interface ScriptExecutionContext {
  /** 上下文ID */
  id: string
  /** 上下文名称 */
  name: string
  /** 上下文变量 */
  variables: Record<string, unknown>
  /** 内置函数 */
  functions: Record<string, ScriptFunction>
  /** 上下文创建时间 */
  createdAt: number
  /** 上下文最后更新时间 */
  updatedAt: number
}

// 脚本模板
export interface ScriptTemplate {
  /** 模板ID */
  id: string
  /** 模板名称 */
  name: string
  /** 模板描述 */
  description?: string
  /** 模板分类 */
  category: string
  /** 模板代码 */
  code: string
  /** 模板参数 */
  parameters: ScriptTemplateParameter[]
  /** 使用片段 */
  usageSnippet?: string
  /** 是否为系统模板 */
  isSystem?: boolean
  /** 创建时间 */
  createdAt: number
  /** 更新时间 */
  updatedAt: number
}

// 脚本模板参数
export interface ScriptTemplateParameter {
  /** 参数名称 */
  name: string
  /** 参数类型 */
  type: 'string' | 'number' | 'boolean' | 'object' | 'array' | 'function'
  /** 参数描述 */
  description?: string
  /** 是否必需 */
  required: boolean
  /** 默认值 */
  defaultValue?: unknown
  /** 参数验证规则 */
  validation?: {
    min?: number
    max?: number
    pattern?: string
    enum?: unknown[]
  }
}

// 沙箱配置
export interface SandboxConfig {
  /** 兼容字段：本地硬安全策略始终启用，false 不会关闭防护 */
  enabled: boolean
  /** 允许暴露的安全全局对象；网络能力仍由专用适配器控制 */
  allowedGlobals: string[]
  /** 兼容字段：本地硬阻断集合不会因移除此列表项而放宽 */
  blockedGlobals: string[]
  /** 兼容字段：eval 始终阻断，true 不会放行 */
  allowEval: boolean
  /** 兼容字段：Function 构造器始终阻断，true 不会放行 */
  allowFunction: boolean
  /** 兼容字段：原型与构造器访问始终阻断，true 不会放行 */
  allowPrototypePollution: boolean
  /** 自定义安全策略 */
  customSecurityPolicy?: (code: string) => boolean
}

// 脚本引擎配置
export interface ScriptEngineConfig {
  /** 默认脚本配置 */
  defaultScriptConfig: ScriptConfig
  /** 沙箱配置 */
  sandboxConfig: SandboxConfig
  /** 保留契约：当前本地执行器不提供结果缓存 */
  enableCache: boolean
  /** 保留契约：仅在未来启用结果缓存后生效（毫秒） */
  cacheTTL: number
  /** 保留契约：当前调用由宿主调度，执行器不实现并发队列 */
  maxConcurrentExecutions: number
  /** 保留契约：当前仅始终收集基础执行统计 */
  enablePerformanceMonitoring: boolean
}

/**
 * 沙箱内部定时器记录
 */
export interface ScriptSandboxTimerRecord {
  type: 'timeout' | 'interval'
  id: ReturnType<typeof setTimeout> | ReturnType<typeof setInterval>
}

/**
 * 脚本沙箱实例。
 * `with` 作用域使用的自由键值对象；`_timers`/`_utils` 由宿主维护，其余键由脚本自由读写。
 */
export type ScriptSandboxInstance = Record<string, unknown> & {
  _timers?: ScriptSandboxTimerRecord[]
}

// 脚本执行器接口
export interface IScriptExecutor {
  /** 执行脚本 */
  execute<T = unknown>(config: ScriptConfig, context?: ScriptExecutionContext): Promise<ScriptExecutionResult<T>>
  /** 验证脚本语法 */
  validateSyntax(code: string): { valid: boolean; error?: string }
  /** 获取执行统计 */
  getExecutionStats(): ExecutionStats
}

// 脚本沙箱接口
export interface IScriptSandbox {
  /** 创建沙箱环境 */
  createSandbox(config: SandboxConfig): ScriptSandboxInstance
  /** 执行沙箱代码 */
  executeInSandbox(code: string, sandbox: ScriptSandboxInstance, timeout?: number): Promise<unknown>
  /** 销毁沙箱 */
  destroySandbox(sandbox: ScriptSandboxInstance): void
  /** 检查代码安全性 */
  checkCodeSecurity(code: string): { safe: boolean; issues: string[] }
}

// 脚本模板管理器接口
export interface IScriptTemplateManager {
  /** 获取所有模板 */
  getAllTemplates(): ScriptTemplate[]
  /** 根据分类获取模板 */
  getTemplatesByCategory(category: string): ScriptTemplate[]
  /** 获取指定模板 */
  getTemplate(id: string): ScriptTemplate | null
  /** 创建模板 */
  createTemplate(template: Omit<ScriptTemplate, 'id' | 'createdAt' | 'updatedAt'>): ScriptTemplate
  /** 更新模板 */
  updateTemplate(id: string, updates: Partial<ScriptTemplate>): boolean
  /** 删除模板 */
  deleteTemplate(id: string): boolean
  /** 根据模板生成代码 */
  generateCode(templateId: string, parameters: Record<string, unknown>): string
}

// 上下文管理器接口
export interface IScriptContextManager {
  /** 创建执行上下文 */
  createContext(name: string, variables?: Record<string, unknown>): ScriptExecutionContext
  /** 获取上下文 */
  getContext(id: string): ScriptExecutionContext | null
  /** 更新上下文 */
  updateContext(id: string, updates: Partial<ScriptExecutionContext>): boolean
  /** 删除上下文 */
  deleteContext(id: string): boolean
  /** 克隆上下文 */
  cloneContext(id: string, newName: string): ScriptExecutionContext | null
  /** 合并上下文 */
  mergeContexts(sourceId: string, targetId: string): boolean
}

// 脚本引擎主接口
export interface IScriptEngine {
  /** 脚本执行器 */
  executor: IScriptExecutor
  /** 脚本沙箱 */
  sandbox: IScriptSandbox
  /** 模板管理器 */
  templateManager: IScriptTemplateManager
  /** 上下文管理器 */
  contextManager: IScriptContextManager

  /** 快速执行脚本 */
  execute<T = unknown>(code: string, context?: Record<string, unknown>): Promise<ScriptExecutionResult<T>>
  /** 使用模板执行 */
  executeTemplate<T = unknown>(
    templateId: string,
    parameters: Record<string, unknown>
  ): Promise<ScriptExecutionResult<T>>
  /** 获取引擎配置 */
  getConfig(): ScriptEngineConfig
  /** 更新引擎配置 */
  updateConfig(config: Partial<ScriptEngineConfig>): void
}

// 执行统计
export interface ExecutionStats {
  /** 总执行次数 */
  totalExecutions: number
  /** 成功执行次数 */
  successfulExecutions: number
  /** 失败执行次数 */
  failedExecutions: number
  /** 平均执行时间（毫秒） */
  averageExecutionTime: number
  /** 最长执行时间（毫秒） */
  maxExecutionTime: number
  /** 最短执行时间（毫秒） */
  minExecutionTime: number
  /** 当前并发执行数 */
  currentConcurrentExecutions: number
}

// 引擎统计快照
export interface ScriptEngineStatsSnapshot {
  executor: ExecutionStats
  templates: {
    total: number
    byCategory: Record<string, number>
  }
  contexts: {
    total: number
    active: number
  }
}

// 引擎状态导出快照
export interface ScriptEngineStateSnapshot {
  config: ScriptEngineConfig
  stats: ScriptEngineStatsSnapshot
  templates: ScriptTemplate[]
  contexts: ScriptExecutionContext[]
  timestamp: string
}

// 内置工具函数类型
export interface BuiltinUtilities {
  /** 数据生成工具 */
  mockData: {
    randomNumber: (min?: number, max?: number) => number
    randomString: (length?: number) => string
    randomBoolean: () => boolean
    randomDate: (start?: Date, end?: Date) => Date
    randomArray: <T>(items: T[], count?: number) => T[]
    randomObject: (template: Record<string, unknown>) => Record<string, unknown>
  }

  /** 数据处理工具 */
  dataUtils: {
    deepClone: <T>(obj: T) => T
    merge: (...objects: unknown[]) => Record<string, unknown>
    pick: <T extends Record<string, unknown>>(obj: T, keys: (keyof T)[]) => Partial<T>
    omit: <T extends Record<string, unknown>>(obj: T, keys: (keyof T)[]) => Partial<T>
    groupBy: <T>(array: T[], key: keyof T | ((item: T) => unknown)) => Record<string, T[]>
    sortBy: <T>(array: T[], key: keyof T | ((item: T) => unknown)) => T[]
  }

  /** 时间工具 */
  timeUtils: {
    now: () => number
    format: (date: Date | number, format: string) => string
    addDays: (date: Date, days: number) => Date
    diffDays: (date1: Date, date2: Date) => number
    startOfDay: (date: Date) => Date
    endOfDay: (date: Date) => Date
  }

  /** 网络工具 */
  networkUtils: {
    httpGet: (url: string, options?: RequestInit) => Promise<unknown>
    httpPost: (url: string, data: unknown, options?: RequestInit) => Promise<unknown>
    httpPut: (url: string, data: unknown, options?: RequestInit) => Promise<unknown>
    httpDelete: (url: string, options?: RequestInit) => Promise<unknown>
  }

  /** 日志工具 */
  logger: {
    log: (...args: unknown[]) => void
    warn: (...args: unknown[]) => void
    error: (...args: unknown[]) => void
    info: (...args: unknown[]) => void
    debug: (...args: unknown[]) => void
  }
}

// 预定义模板分类
export enum TemplateCategory {
  DATA_GENERATION = 'data-generation',
  DATA_PROCESSING = 'data-processing',
  API_INTEGRATION = 'api-integration',
  TIME_SERIES = 'time-series',
  MATHEMATICAL = 'mathematical',
  VALIDATION = 'validation',
  TRANSFORMATION = 'transformation',
  UTILITY = 'utility',
  CUSTOM = 'custom'
}
