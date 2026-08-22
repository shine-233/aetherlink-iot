/*
 * 文件用途：提供前端 Logger 类和日志级别控制。
 * 核心逻辑：按配置过滤日志级别，并封装 debug/info/warn/error 等输出入口。
 * 关键注意事项：日志不能泄露 token、用户信息或设备敏感数据。
 * 重构建议：后续可增加按环境禁用和结构化上下文字段。
 */
/**
 * 统一调试日志系统 - 增强版
 * 支持开发/生产环境切换，避免生产环境中的调试信息污染
 */

// 日志级别枚举
export enum LogLevel {
  DEBUG = 0,
  INFO = 1,
  WARN = 2,
  ERROR = 3
}

// 日志配置接口
interface LoggerConfig {
  enabled: boolean
  level: LogLevel
  prefix?: string
  timestamp?: boolean
}

// 默认配置：开发环境启用所有日志，生产环境只启用警告和错误
const DEFAULT_CONFIG: LoggerConfig = {
  enabled: true,
  level: import.meta.env.DEV ? LogLevel.DEBUG : LogLevel.WARN,
  prefix: '[AetherLink IoT]',
  timestamp: true
}

export default class Logger {
  private config: LoggerConfig
  moduleName: string = ''

  constructor(moduleName = '', config?: Partial<LoggerConfig>) {
    this.moduleName = moduleName
    this.config = { ...DEFAULT_CONFIG, ...config }
  }

  /**
   * 更新日志配置
   */
  updateConfig(config: Partial<LoggerConfig>) {
    this.config = { ...this.config, ...config }
  }

  /**
   * 检查日志级别是否启用
   */
  private isLevelEnabled(level: LogLevel): boolean {
    return this.config.enabled && level >= this.config.level
  }

  /**
   * 格式化日志前缀
   */
  private formatPrefix(level: string): string {
    const prefix = this.config.prefix || '[App]'
    const timestamp = this.config.timestamp ? new Date().toLocaleTimeString() + ' -' : ''
    const moduleInfo = this.moduleName ? `[${this.moduleName}]` : ''
    return `${prefix}${moduleInfo}[${level}] ${timestamp}`
  }

  /**
   * Debug级别日志 - 只在开发环境显示
   */
  debug(...args: unknown[]): void {
    if (this.isLevelEnabled(LogLevel.DEBUG)) {
      console.log(this.formatPrefix('DEBUG'), ...args)
    }
  }

  /**
   * Info级别日志
   */
  info(...args: unknown[]): void {
    if (this.isLevelEnabled(LogLevel.INFO)) {
      console.info(this.formatPrefix('INFO'), ...args)
    }
  }

  /**
   * Warning级别日志
   */
  warn(...args: unknown[]): void {
    if (this.isLevelEnabled(LogLevel.WARN)) {
      console.warn(this.formatPrefix('WARN'), ...args)
    }
  }

  /**
   * Error级别日志
   */
  error(...args: unknown[]): void {
    if (this.isLevelEnabled(LogLevel.ERROR)) {
      console.error(this.formatPrefix('ERROR'), ...args)
    }
  }

  /**
   * 条件日志 - 只有当条件为true时才输出
   */
  debugIf(condition: boolean, ...args: unknown[]): void {
    if (condition) this.debug(...args)
  }

  /**
   * 性能计时开始
   */
  time(label: string): void {
    if (this.isLevelEnabled(LogLevel.DEBUG)) {
      console.time(`${this.formatPrefix('TIMER')} ${label}`)
    }
  }

  /**
   * 性能计时结束
   */
  timeEnd(label: string): void {
    if (this.isLevelEnabled(LogLevel.DEBUG)) {
      console.timeEnd(`${this.formatPrefix('TIMER')} ${label}`)
    }
  }
}

// 创建日志器工厂函数
export const createLogger = (moduleName: string, config?: Partial<LoggerConfig>) => new Logger(moduleName, config)

// 创建全局默认日志器
export const logger = new Logger()

// 为常用模块创建专用日志器
export const dataSourceLogger = createLogger('DataSource')
export const httpLogger = createLogger('HTTP')
export const componentLogger = createLogger('Component')
export const visualEditorLogger = createLogger('VisualEditor')
export const propertyBindingLogger = createLogger('PropertyBinding')

// 导出类型
export type { LoggerConfig }
