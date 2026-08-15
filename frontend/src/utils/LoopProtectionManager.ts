/*
 * 文件用途：提供循环调用保护管理器，防止组件方法高频递归或互相触发形成死循环。
 * 核心逻辑：跟踪活跃调用、调用历史、递归深度和黑名单，并包装同步/异步方法进行清理。
 * 关键注意事项：误判会阻断合法交互，漏判会导致页面卡死，阈值配置需要谨慎。
 * 重构建议：可继续拆分检测策略和包装执行器，提升可测试性。
 */
/**
 * 🔥 全局循环检测和防护管理器
 *
 * 解决200+组件场景下的循环触发问题：
 * 1. 检测配置变更的循环依赖
 * 2. 防止数据源的递归执行
 * 3. 监控属性绑定的循环更新
 * 4. 提供性能统计和调试信息
 */

export interface LoopDetectionConfig {
  maxDepth: number // 最大递归深度
  timeWindow: number // 时间窗口 (ms)
  maxCallsInWindow: number // 时间窗口内最大调用次数
  enableDebug: boolean // 是否启用调试输出
}

export interface CallRecord {
  timestamp: number
  depth: number
  source: string
  componentId?: string
  action?: string
}

class LoopProtectionManager {
  private static instance: LoopProtectionManager | null = null

  // 配置
  private config: LoopDetectionConfig = {
    maxDepth: 10,
    timeWindow: 5000, // 5秒
    maxCallsInWindow: 50, // 5秒内最多50次调用
    enableDebug: process.env.NODE_ENV === 'development'
  }

  // 调用栈跟踪
  private callStacks = new Map<string, CallRecord[]>() // key: functionName
  private activeCallCounts = new Map<string, number>() // 当前活跃调用计数

  // 时间窗口内的调用统计
  private callHistory = new Map<string, CallRecord[]>() // key: functionName

  // 黑名单：被检测到循环的函数暂时禁用
  private blacklistedFunctions = new Set<string>()
  private blacklistTimeouts = new Map<string, NodeJS.Timeout>()

  // 性能统计
  private performanceStats = {
    totalCallsBlocked: 0,
    totalLoopsDetected: 0,
    averageCallsPerSecond: 0,
    lastResetTime: Date.now()
  }

  private constructor() {
    this.setupGlobalErrorHandling()
    this.startPerformanceMonitoring()
  }

  public static getInstance(): LoopProtectionManager {
    if (!LoopProtectionManager.instance) {
      LoopProtectionManager.instance = new LoopProtectionManager()
    }
    return LoopProtectionManager.instance
  }

  /**
   * 🔥 核心方法：检查函数调用是否应该被允许
   */
  public shouldAllowCall(functionName: string, componentId?: string, action?: string, source = 'unknown'): boolean {
    const callKey = componentId ? `${functionName}:${componentId}` : functionName

    // 1. 检查黑名单
    if (this.blacklistedFunctions.has(callKey)) {
      this.performanceStats.totalCallsBlocked++
      if (this.config.enableDebug) {
        console.warn(`🚫 [LoopProtection] 阻止黑名单函数调用: ${callKey}`)
      }
      return false
    }

    // 2. 检查递归深度
    const currentDepth = this.getCurrentDepth(callKey)
    if (currentDepth >= this.config.maxDepth) {
      this.addToBlacklist(callKey, `递归深度超过${this.config.maxDepth}`)
      return false
    }

    // 3. 检查时间窗口内的调用频率
    if (this.isCallFrequencyTooHigh(callKey)) {
      this.addToBlacklist(callKey, `调用频率过高`)
      return false
    }

    // 4. 记录这次调用
    this.recordCall(callKey, source, componentId, action)

    return true
  }

  /**
   * 🔥 标记函数调用开始
   */
  public markCallStart(functionName: string, componentId?: string, source = 'unknown'): string {
    const callKey = componentId ? `${functionName}:${componentId}` : functionName
    const callId = `${callKey}:${Date.now()}:${Math.random().toString(36).slice(2, 7)}`

    if (!this.shouldAllowCall(functionName, componentId, 'start', source)) {
      return '' // 空字符串表示调用被阻止
    }

    // 增加活跃调用计数
    const currentCount = this.activeCallCounts.get(callKey) || 0
    this.activeCallCounts.set(callKey, currentCount + 1)

    if (this.config.enableDebug && currentCount > 3) {
      console.warn(`⚠️ [LoopProtection] 高并发调用检测: ${callKey} (${currentCount + 1} 个并发)`)
    }

    return callId
  }

  /**
   * 🔥 标记函数调用结束
   */
  public markCallEnd(callId: string, functionName: string, componentId?: string): void {
    if (!callId) return // 调用被阻止的情况

    const callKey = componentId ? `${functionName}:${componentId}` : functionName

    // 减少活跃调用计数
    const currentCount = this.activeCallCounts.get(callKey) || 0
    if (currentCount > 0) {
      this.activeCallCounts.set(callKey, currentCount - 1)
    }

    // 清理调用栈
    this.cleanupCallStack(callKey)
  }

  /**
   * 🔥 获取当前递归深度
   */
  private getCurrentDepth(callKey: string): number {
    const stack = this.callStacks.get(callKey) || []
    return stack.length
  }

  /**
   * 🔥 检查调用频率是否过高
   */
  private isCallFrequencyTooHigh(callKey: string): boolean {
    const now = Date.now()
    const history = this.callHistory.get(callKey) || []

    // 清理过期的历史记录
    const validHistory = history.filter(record => now - record.timestamp <= this.config.timeWindow)
    this.callHistory.set(callKey, validHistory)

    return validHistory.length >= this.config.maxCallsInWindow
  }

  /**
   * 🔥 记录函数调用
   */
  private recordCall(callKey: string, source: string, componentId?: string, action?: string): void {
    const now = Date.now()
    const record: CallRecord = {
      timestamp: now,
      depth: this.getCurrentDepth(callKey),
      source,
      componentId,
      action
    }

    // 更新调用栈
    const stack = this.callStacks.get(callKey) || []
    stack.push(record)
    this.callStacks.set(callKey, stack)

    // 更新历史记录
    const history = this.callHistory.get(callKey) || []
    history.push(record)
    this.callHistory.set(callKey, history)
  }

  /**
   * 🔥 添加到黑名单
   */
  private addToBlacklist(callKey: string, reason: string): void {
    this.blacklistedFunctions.add(callKey)
    this.performanceStats.totalLoopsDetected++

    if (this.config.enableDebug) {
      console.error(`🚫 [LoopProtection] 检测到循环，已加入黑名单: ${callKey}`, {
        reason,
        callHistory: this.callHistory.get(callKey)?.slice(-5), // 最后5次调用
        currentDepth: this.getCurrentDepth(callKey)
      })
    }

    // 设置自动解除黑名单的定时器
    const existingTimeout = this.blacklistTimeouts.get(callKey)
    if (existingTimeout) {
      clearTimeout(existingTimeout)
    }

    const timeout = setTimeout(() => {
      this.removeFromBlacklist(callKey)
    }, 10000) // 10秒后自动解除黑名单

    this.blacklistTimeouts.set(callKey, timeout)
  }

  /**
   * 🔥 从黑名单移除
   */
  private removeFromBlacklist(callKey: string): void {
    this.blacklistedFunctions.delete(callKey)
    this.blacklistTimeouts.delete(callKey)

    // 清理相关的调用历史
    this.callStacks.delete(callKey)
    this.callHistory.delete(callKey)
    this.activeCallCounts.delete(callKey)

    if (this.config.enableDebug) {
      console.info(`✅ [LoopProtection] 已从黑名单移除: ${callKey}`)
    }
  }

  /**
   * 🔥 清理调用栈
   */
  private cleanupCallStack(callKey: string): void {
    const stack = this.callStacks.get(callKey) || []
    if (stack.length > 0) {
      stack.pop() // 移除最后一个调用记录
      this.callStacks.set(callKey, stack)
    }
  }

  /**
   * 🔥 设置全局错误处理
   */
  private setupGlobalErrorHandling(): void {
    // 监听未捕获的异常，可能是循环调用导致的栈溢出
    if (typeof window !== 'undefined') {
      window.addEventListener('error', event => {
        if (event.error && event.error.message.includes('Maximum call stack size exceeded')) {
          console.error('🚫 [LoopProtection] 检测到栈溢出，可能存在无限递归')
          this.performanceStats.totalLoopsDetected++

          // 清空所有活跃调用，防止系统崩溃
          this.activeCallCounts.clear()
          this.callStacks.clear()
        }
      })
    }
  }

  /**
   * 🔥 启动性能监控
   */
  private startPerformanceMonitoring(): void {
    setInterval(() => {
      const now = Date.now()
      const timeDiff = now - this.performanceStats.lastResetTime
      const totalCalls = Array.from(this.callHistory.values()).reduce((sum, history) => sum + history.length, 0)

      this.performanceStats.averageCallsPerSecond = totalCalls / (timeDiff / 1000)
      this.performanceStats.lastResetTime = now

      if (this.config.enableDebug && totalCalls > 0) {
        /* intentionally empty */
      }

      // 清理过期的历史记录
      this.cleanupExpiredHistory()
    }, 30000) // 每30秒统计一次
  }

  /**
   * 🔥 清理过期的历史记录
   */
  private cleanupExpiredHistory(): void {
    const now = Date.now()
    for (const [callKey, history] of this.callHistory.entries()) {
      const validHistory = history.filter(
        record => now - record.timestamp <= this.config.timeWindow * 2 // 保留2倍时间窗口的历史
      )
      if (validHistory.length !== history.length) {
        this.callHistory.set(callKey, validHistory)
      }
    }
  }

  /**
   * 🔥 获取性能统计信息
   */
  public getPerformanceStats() {
    return {
      ...this.performanceStats,
      blacklistedFunctionsCount: this.blacklistedFunctions.size,
      activeCallsCount: Array.from(this.activeCallCounts.values()).reduce((a, b) => a + b, 0),
      totalTrackedFunctions: this.callHistory.size
    }
  }

  /**
   * 🔥 获取当前黑名单
   */
  public getBlacklistedFunctions(): string[] {
    return Array.from(this.blacklistedFunctions)
  }

  /**
   * 🔥 手动清理（用于测试或重置）
   */
  public reset(): void {
    this.callStacks.clear()
    this.callHistory.clear()
    this.activeCallCounts.clear()
    this.blacklistedFunctions.clear()

    // 清理所有定时器
    for (const timeout of this.blacklistTimeouts.values()) {
      clearTimeout(timeout)
    }
    this.blacklistTimeouts.clear()

    this.performanceStats = {
      totalCallsBlocked: 0,
      totalLoopsDetected: 0,
      averageCallsPerSecond: 0,
      lastResetTime: Date.now()
    }
  }

  /**
   * 🔥 更新配置
   */
  public updateConfig(newConfig: Partial<LoopDetectionConfig>): void {
    this.config = { ...this.config, ...newConfig }
  }
}

// 导出单例实例
export const loopProtectionManager = LoopProtectionManager.getInstance()

/**
 * 🔥 装饰器：自动添加循环保护
 */
export function loopProtection(functionName?: string) {
  return function (target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    const originalMethod = descriptor.value
    const fnName = functionName || `${target.constructor.name}.${propertyKey}`

    descriptor.value = function (...args: any[]) {
      const componentId = (this as any).componentId || (this as any).nodeId || 'unknown'
      const callId = loopProtectionManager.markCallStart(fnName, componentId, 'decorator')

      if (!callId) {
        // 调用被阻止
        return Promise.resolve()
      }

      try {
        const result = originalMethod.apply(this, args)

        if (result instanceof Promise) {
          return result.finally(() => {
            loopProtectionManager.markCallEnd(callId, fnName, componentId)
          })
        } else {
          loopProtectionManager.markCallEnd(callId, fnName, componentId)
          return result
        }
      } catch (error) {
        loopProtectionManager.markCallEnd(callId, fnName, componentId)
        throw error
      }
    }

    return descriptor
  }
}
