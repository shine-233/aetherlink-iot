/**
 * 文件用途：提供网格高频操作相关的性能辅助函数。
 * 核心逻辑：封装防抖、节流、批处理或缓存策略，降低拖拽缩放期间的更新压力。
 * 关键注意事项：性能包装必须保留取消、flush 或最终执行语义，避免事件丢失。
 * 重构建议：可将通用节流工具迁移到全局 utils，网格目录只保留布局专属优化。
 */
import type { GridLayoutPlusItem, PerformanceConfig } from '../gridLayoutPlusTypes'

function getHighResolutionTime(): number {
  // lib.dom 将 performance 标为恒存在；运行时仍可能缺失，用可空视图保留守卫语义。
  const perf = globalThis.performance as Performance | undefined
  return perf?.now ? perf.now() : Date.now()
}

/**
 * 适合输入联想、尺寸监听等场景：只保留最后一次触发。
 */
export function debounce<T extends (...args: any[]) => any>(func: T, delay: number): (...args: Parameters<T>) => void {
  let timeoutId: NodeJS.Timeout | null = null

  return function (this: ThisParameterType<T>, ...args: Parameters<T>) {
    if (timeoutId) {
      clearTimeout(timeoutId)
    }

    timeoutId = setTimeout(() => {
      func.apply(this, args)
      timeoutId = null
    }, delay)
  }
}

/**
 * 适合拖拽、滚动和 resize：限制单位时间内的调用频率，但保留尾调用补发。
 */
export function throttle<T extends (...args: any[]) => any>(func: T, delay: number): (...args: Parameters<T>) => void {
  let lastTime = 0
  let timeoutId: NodeJS.Timeout | null = null

  return function (this: ThisParameterType<T>, ...args: Parameters<T>) {
    const now = Date.now()

    if (now - lastTime >= delay) {
      lastTime = now
      func.apply(this, args)
    } else if (!timeoutId) {
      timeoutId = setTimeout(
        () => {
          lastTime = Date.now()
          func.apply(this, args)
          timeoutId = null
        },
        delay - (now - lastTime)
      )
    }
  }
}

/**
 * 针对布局数据做轻量性能处理；当前主要是惰性加载标记，保留了后续扩展入口。
 */
export function optimizeLayoutPerformance(
  layout: GridLayoutPlusItem[],
  config: PerformanceConfig
): GridLayoutPlusItem[] {
  try {
    let optimizedLayout = [...layout]

    // 懒加载处理
    if (config.enableLazyLoading) {
      optimizedLayout = applyLazyLoading(optimizedLayout, config)
    }

    return optimizedLayout
  } catch (error) {
    console.error('Failed to optimize layout performance:', error)
    return layout
  }
}

/**
 * 基于顺序给后半段组件打上惰性标记，实际渲染裁剪仍由上层消费这些 metadata。
 */
function applyLazyLoading(layout: GridLayoutPlusItem[], config: PerformanceConfig): GridLayoutPlusItem[] {
  try {
    const buffer = config.lazyLoadingBuffer || 5

    // 标记需要懒加载的项目
    return layout.map((item, index) => ({
      ...item,
      metadata: {
        ...item.metadata,
        lazy: index >= buffer,
        lazyIndex: index
      }
    }))
  } catch (error) {
    console.error('Failed to apply lazy loading:', error)
    return layout
  }
}

/**
 * 统一封装运行期性能采样：
 * 优先走浏览器 Performance API，缺失时降级到时间戳计时。
 */
export class PerformanceMonitor {
  private metrics: Map<string, number[]> = new Map()
  private timers: Map<string, number> = new Map()
  private isEnabled = true

  /**
   * 开始监控操作
   */
  start(operation: string): string {
    if (!this.isEnabled) return ''

    const id = `${operation}_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`
    const perf = globalThis.performance as Performance | undefined
    if (perf?.mark) {
      performance.mark(`${id}_start`)
    } else {
      this.timers.set(id, getHighResolutionTime())
    }

    return id
  }

  /**
   * 结束监控并记录
   */
  end(id: string, operation: string): number | null {
    if (!this.isEnabled || !id) return null

    try {
      // 在非浏览器或能力不完整环境下，退化为普通高精度时间差统计。
      const perf = globalThis.performance as Performance | undefined
      if (!perf?.mark || !perf.measure) {
        const start = this.timers.get(id)
        if (start === undefined) return null

        const duration = Math.max(0.001, getHighResolutionTime() - start)
        this.timers.delete(id)
        this.recordMetric(operation, duration)
        return duration
      }

      performance.mark(`${id}_end`)
      performance.measure(id, `${id}_start`, `${id}_end`)

      const measures = performance.getEntriesByName(id, 'measure')
      if (measures.length > 0) {
        const duration = measures[0].duration

        // 记录到指标中
        this.recordMetric(operation, duration)

        // 限制记录数量

        // 清理性能条目
        performance.clearMarks(`${id}_start`)
        performance.clearMarks(`${id}_end`)
        performance.clearMeasures(id)

        return duration
      }
    } catch (error) {
      console.error('Performance monitoring error:', error)
    }

    return null
  }

  startTimer(metric: string): void {
    if (!this.isEnabled) return
    this.timers.set(metric, getHighResolutionTime())
  }

  endTimer(metric: string): number {
    if (!this.isEnabled) return 0

    const start = this.timers.get(metric)
    if (start === undefined) return 0

    const duration = Math.max(0.001, getHighResolutionTime() - start)
    this.timers.delete(metric)
    this.recordMetric(metric, duration)
    return duration
  }

  getMetrics(): Record<string, number> {
    const result: Record<string, number> = {}

    for (const [operation, values] of this.metrics.entries()) {
      result[operation] = values.length > 0 ? values[values.length - 1] : 0
    }

    return result
  }

  /**
   * 获取操作的统计信息
   */
  getStats(operation: string): {
    count: number
    average: number
    min: number
    max: number
    recent: number
  } | null {
    const metrics = this.metrics.get(operation)
    if (!metrics || metrics.length === 0) return null

    const count = metrics.length
    const sum = metrics.reduce((a, b) => a + b, 0)
    const average = sum / count
    const min = Math.min(...metrics)
    const max = Math.max(...metrics)
    const recent = metrics[metrics.length - 1]

    return { count, average, min, max, recent }
  }

  /**
   * 获取所有统计信息
   */
  getAllStats(): Record<string, ReturnType<PerformanceMonitor['getStats']>> {
    const stats: Record<string, ReturnType<PerformanceMonitor['getStats']>> = {}

    for (const operation of this.metrics.keys()) {
      stats[operation] = this.getStats(operation)
    }

    return stats
  }

  /**
   * 清理监控数据
   */
  clear(operation?: string): void {
    if (operation) {
      this.metrics.delete(operation)
      this.timers.delete(operation)
    } else {
      this.metrics.clear()
      this.timers.clear()
    }
  }

  reset(): void {
    this.clear()
    this.isEnabled = true
  }

  /**
   * 启用/禁用监控
   */
  setEnabled(enabled: boolean): void {
    this.isEnabled = enabled
  }

  private recordMetric(operation: string, duration: number): void {
    if (!this.metrics.has(operation)) {
      this.metrics.set(operation, [])
    }

    const operationMetrics = this.metrics.get(operation)!
    operationMetrics.push(duration)

    if (operationMetrics.length > 100) {
      operationMetrics.shift()
    }
  }
}

/**
 * 全局性能监控实例
 */
export const performanceMonitor = new PerformanceMonitor()

/**
 * 内存使用监控
 */
export function getMemoryUsage(): {
  used: number
  total: number
  percentage: number
} | null {
  try {
    // 现代浏览器的内存API
    if ('memory' in performance) {
      const memory = (performance as any).memory
      return {
        used: memory.usedJSHeapSize,
        total: memory.totalJSHeapSize,
        percentage: (memory.usedJSHeapSize / memory.totalJSHeapSize) * 100
      }
    }
  } catch (error) {
    console.error('Failed to get memory usage:', error)
  }

  return null
}

/**
 * 缓存管理器
 */
/**
 * 一个带 TTL 和最少使用淘汰策略的轻量缓存，适合短时复用的布局派生数据。
 */
export class CacheManager<K, V> {
  private cache = new Map<K, { value: V; timestamp: number; hits: number }>()
  private maxSize: number
  private ttl: number // 生存时间(ms)

  constructor(maxSize = 100, ttl = 5 * 60 * 1000) {
    // 默认5分钟
    this.maxSize = maxSize
    this.ttl = ttl
  }

  /**
   * 获取缓存值
   */
  get(key: K): V | null {
    const item = this.cache.get(key)
    if (!item) return null

    // 检查是否过期
    if (Date.now() - item.timestamp > this.ttl) {
      this.cache.delete(key)
      return null
    }

    // 更新访问计数
    item.hits++
    return item.value
  }

  has(key: K): boolean {
    return this.get(key) !== null
  }

  /**
   * 设置缓存值
   */
  set(key: K, value: V): void {
    // 如果缓存已满，移除最少使用的项
    if (this.cache.size >= this.maxSize) {
      this.evictLeastUsed()
    }

    this.cache.set(key, {
      value,
      timestamp: Date.now(),
      hits: 0
    })
  }

  /**
   * 删除缓存项
   */
  delete(key: K): boolean {
    return this.cache.delete(key)
  }

  /**
   * 清空缓存
   */
  clear(): void {
    this.cache.clear()
  }

  size(): number {
    return this.cache.size
  }

  /**
   * 获取缓存统计
   */
  getStats(): {
    size: number
    maxSize: number
    hitRate: number
    totalHits: number
  } {
    let totalHits = 0
    let totalAccess = 0

    for (const item of this.cache.values()) {
      totalHits += item.hits
      totalAccess += item.hits + 1 // +1 for the initial set
    }

    return {
      size: this.cache.size,
      maxSize: this.maxSize,
      hitRate: totalAccess > 0 ? totalHits / totalAccess : 0,
      totalHits
    }
  }

  /**
   * 移除最少使用的项
   */
  private evictLeastUsed(): void {
    let leastUsedKey: K | null = null
    let minHits = Infinity
    let oldestTime = Infinity

    for (const [key, item] of this.cache.entries()) {
      if (item.hits < minHits || (item.hits === minHits && item.timestamp < oldestTime)) {
        leastUsedKey = key
        minHits = item.hits
        oldestTime = item.timestamp
      }
    }

    if (leastUsedKey !== null) {
      this.cache.delete(leastUsedKey)
    }
  }
}

/**
 * 异步队列处理器
 */
/**
 * 控制异步任务并发度，避免密集布局任务同时执行时放大主线程压力。
 */
export class AsyncQueue<T> {
  private queue: Array<() => Promise<T>> = []
  private running = false
  private concurrency: number

  constructor(concurrency = 1) {
    this.concurrency = concurrency
  }

  /**
   * 添加任务到队列
   */
  add<R>(task: () => Promise<R>): Promise<R> {
    return new Promise((resolve, reject) => {
      this.queue.push(async () => {
        try {
          const result = await task()
          resolve(result as any)
          return result as any
        } catch (error) {
          reject(error)
          throw error
        }
      })

      this.process()
    })
  }

  /**
   * 处理队列
   */
  private async process(): Promise<void> {
    if (this.running) return

    this.running = true

    try {
      while (this.queue.length > 0) {
        const batch = this.queue.splice(0, this.concurrency)
        await Promise.allSettled(batch.map(task => task()))
      }
    } finally {
      this.running = false
    }
  }

  /**
   * 清空队列
   */
  clear(): void {
    this.queue = []
  }

  /**
   * 获取队列长度
   */
  size(): number {
    return this.queue.length
  }
}
