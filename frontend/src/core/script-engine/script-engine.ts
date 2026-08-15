/**
 * 文件用途：组合执行器、沙箱、模板管理器和上下文管理器，提供统一脚本引擎门面。
 * 核心逻辑：初始化默认配置与内置模板，对外暴露直接执行、模板执行和状态统计能力。
 * 关键注意事项：默认实例会被多个调用方共享，配置变更需要谨慎评估全局影响。
 * 重构建议：可将门面层与默认实例创建分离，便于测试和多实例注入。
 */

import type {
  IScriptEngine,
  IScriptExecutor,
  IScriptSandbox,
  IScriptTemplateManager,
  IScriptContextManager,
  ScriptEngineConfig,
  ScriptExecutionResult,
  ScriptConfig
} from './types'
import { ScriptExecutor, defaultScriptConfig } from '@/core/script-engine/executor'
import { ScriptSandbox, defaultSandboxConfig } from '@/core/script-engine/sandbox'
import { ScriptTemplateManager } from '@/core/script-engine/template-manager'
import { ScriptContextManager } from '@/core/script-engine/context-manager'
import { initializeBuiltInTemplates } from '@/core/script-engine/templates/built-in-templates'

/**
 * 主脚本引擎实现类
 */
export class ScriptEngine implements IScriptEngine {
  public executor: IScriptExecutor
  public sandbox: IScriptSandbox
  public readonly templateManager: IScriptTemplateManager
  public readonly contextManager: IScriptContextManager

  private config: ScriptEngineConfig

  constructor(config?: Partial<ScriptEngineConfig>) {
    this.config = {
      defaultScriptConfig,
      sandboxConfig: defaultSandboxConfig,
      enableCache: true,
      cacheTTL: 5 * 60 * 1000, // 5分钟
      maxConcurrentExecutions: 10,
      enablePerformanceMonitoring: true,
      ...config
    }

    // 检查与实际执行共享同一份沙箱策略，避免配置只影响预检查。
    this.executor = new ScriptExecutor(this.config.sandboxConfig)
    this.sandbox = new ScriptSandbox(this.config.sandboxConfig)
    this.templateManager = new ScriptTemplateManager()
    this.contextManager = new ScriptContextManager()

    initializeBuiltInTemplates(this.templateManager)
  }

  /**
   * 快速执行脚本
   */
  async execute<T = any>(code: string, context?: Record<string, any>): Promise<ScriptExecutionResult<T>> {
    // 创建脚本配置
    const scriptConfig: ScriptConfig = {
      ...this.config.defaultScriptConfig,
      code
    }

    // 创建或获取执行上下文
    let executionContext = undefined
    if (context) {
      executionContext = this.contextManager.createContext('临时上下文', context)
    }

    try {
      const result = await this.executor.execute<T>(scriptConfig, executionContext)

      // 清理临时上下文
      if (executionContext) {
        this.contextManager.deleteContext(executionContext.id)
      }
      return result
    } catch (error) {
      // 清理临时上下文
      if (executionContext) {
        this.contextManager.deleteContext(executionContext.id)
      }
      throw error
    }
  }

  /**
   * 使用模板执行
   */
  async executeTemplate<T = any>(
    templateId: string,
    parameters: Record<string, any>
  ): Promise<ScriptExecutionResult<T>> {
    // 根据模板生成代码
    const code = this.templateManager.generateCode(templateId, parameters)

    // 执行生成的代码
    return await this.execute<T>(code)
  }

  /**
   * 批量执行脚本
   */
  async executeBatch<T = any>(
    scripts: Array<{ code: string; context?: Record<string, any> }>
  ): Promise<ScriptExecutionResult<T>[]> {
    const promises = scripts.map(script => this.execute<T>(script.code, script.context))
    return await Promise.all(promises)
  }

  /**
   * 执行脚本并返回流式结果
   */
  async executeStream<T = any>(
    code: string,
    context?: Record<string, any>,
    onUpdate?: (result: Partial<ScriptExecutionResult<T>>) => void
  ): Promise<ScriptExecutionResult<T>> {
    // 创建脚本配置
    const scriptConfig: ScriptConfig = {
      ...this.config.defaultScriptConfig,
      code
    }

    // 创建执行上下文
    let executionContext = undefined
    if (context) {
      executionContext = this.contextManager.createContext('流式上下文', context)
    }

    try {
      // 如果提供了更新回调，先发送开始状态
      if (onUpdate) {
        onUpdate({
          success: false,
          executionTime: 0,
          logs: [
            {
              level: 'info',
              message: '脚本开始执行...',
              timestamp: Date.now()
            }
          ]
        })
      }

      const result = await this.executor.execute<T>(scriptConfig, executionContext)

      // 发送最终结果
      if (onUpdate) {
        onUpdate(result)
      }

      // 清理上下文
      if (executionContext) {
        this.contextManager.deleteContext(executionContext.id)
      }

      return result
    } catch (error) {
      // 清理上下文
      if (executionContext) {
        this.contextManager.deleteContext(executionContext.id)
      }

      throw error
    }
  }

  /**
   * 验证脚本语法
   */
  validateScript(code: string): { valid: boolean; error?: string } {
    return this.executor.validateSyntax(code)
  }

  /**
   * 检查脚本安全性
   */
  checkScriptSecurity(code: string): { safe: boolean; issues: string[] } {
    return this.sandbox.checkCodeSecurity(code)
  }

  /**
   * 获取执行统计信息
   */
  getExecutionStats() {
    return {
      executor: this.executor.getExecutionStats(),
      templates: {
        total: this.templateManager.getAllTemplates().length,
        byCategory: this.getTemplatesByCategory()
      },
      contexts: {
        total: this.contextManager.getAllContexts().length,
        active: this.contextManager.getAllContexts().filter(
          ctx => Date.now() - ctx.updatedAt < 24 * 60 * 60 * 1000 // 24小时内活跃
        ).length
      }
    }
  }

  /**
   * 获取按分类统计的模板数量
   */
  private getTemplatesByCategory(): Record<string, number> {
    const templates = this.templateManager.getAllTemplates()
    const stats: Record<string, number> = {}

    templates.forEach(template => {
      stats[template.category] = (stats[template.category] || 0) + 1
    })

    return stats
  }

  /**
   * 获取引擎配置
   */
  getConfig(): ScriptEngineConfig {
    return { ...this.config }
  }

  /**
   * 更新引擎配置。
   * 沙箱策略变更会重建安全检查器和执行器，后续执行立即采用新策略；
   * 重建执行器也会重置其进程内执行统计。
   */
  updateConfig(config: Partial<ScriptEngineConfig>): void {
    this.config = { ...this.config, ...config }

    if (config.sandboxConfig) {
      this.executor = new ScriptExecutor(this.config.sandboxConfig)
      this.sandbox = new ScriptSandbox(this.config.sandboxConfig)
    }
  }

  /**
   * 预热引擎（执行一些初始化脚本以提高后续性能）
   */
  async warmup(): Promise<void> {
    const warmupScripts = [
      'return { device_id: "replace_with_device_id", online: true }',
      'return Math.round((20 + Math.random() * 5) * 10) / 10',
      'return { timestamp: new Date().toISOString(), metric: "temperature" }',
      'return [20, 21, 22].map(value => ({ metric: "temperature", value }))'
    ]

    for (const script of warmupScripts) {
      try {
        await this.execute(script)
      } catch (error) {
        /* intentionally empty */
      }
    }
  }

  /**
   * 清理资源
   */
  cleanup(): void {
    // 清理所有上下文
    const contexts = this.contextManager.getAllContexts()
    contexts.forEach(context => {
      this.contextManager.deleteContext(context.id)
    })
  }

  /**
   * 导出引擎状态
   */
  exportState(): any {
    return {
      config: this.config,
      stats: this.getExecutionStats(),
      templates: this.templateManager.getAllTemplates(),
      contexts: this.contextManager.getAllContexts(),
      timestamp: new Date().toISOString()
    }
  }

  /**
   * 导入引擎状态
   */
  importState(state: any): boolean {
    try {
      // 导入配置
      if (state.config) {
        this.updateConfig(state.config)
      }

      // 导入模板
      if (state.templates && Array.isArray(state.templates)) {
        state.templates.forEach((template: any) => {
          if (!template.isSystem) {
            // 只导入非系统模板
            this.templateManager.createTemplate(template)
          }
        })
      }

      // 导入上下文
      if (state.contexts && Array.isArray(state.contexts)) {
        state.contexts.forEach((context: any) => {
          this.contextManager.createContext(context.name, context.variables)
        })
      }
      return true
    } catch (error) {
      return false
    }
  }
}

/**
 * 默认脚本引擎实例。
 * 预热保留为显式 warmup() API，模块导入本身不执行脚本或产生运行时副作用。
 */
export const defaultScriptEngine = new ScriptEngine()
