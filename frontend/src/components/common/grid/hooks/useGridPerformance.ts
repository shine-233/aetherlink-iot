/**
 * 文件用途：提供网格交互的性能辅助 hooks。
 * 核心逻辑：封装节流、批处理、可见性或缓存相关逻辑，降低拖拽缩放时的频繁更新成本。
 * 关键注意事项：性能优化不能改变布局事件顺序，调整节流窗口时需验证拖拽和历史记录行为。
 * 重构建议：可按事件节流、渲染调度和缓存策略继续拆分，便于独立测试。
 */
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import type { Ref } from 'vue'
import type { GridLayoutPlusItem, PerformanceConfig } from '../gridLayoutPlusTypes'
import { debounce, throttle } from '../gridLayoutPlusUtils'
import { createLogger } from '../../../../utils/logger'

const gridPerformanceLogger = createLogger('GridPerformance')
const MAX_HISTORY_LENGTH = 100
const DEFAULT_VIRTUALIZATION_THRESHOLD = 100
const DEFAULT_CONFIG: PerformanceConfig = {
  enableVirtualization: false,
  virtualizationThreshold: DEFAULT_VIRTUALIZATION_THRESHOLD,
  debounceDelay: 100,
  throttleDelay: 16,
  enableLazyLoading: false,
  lazyLoadingBuffer: 5
}

export interface UseGridPerformanceOptions {
  performanceConfig?: Partial<PerformanceConfig>
  enableMonitoring?: boolean
  monitoringInterval?: number
  autoOptimize?: boolean
}

export interface PerformanceMetrics {
  renderTime: number
  layoutTime: number
  itemCount: number
  memoryUsage: number
  fps: number
  lastUpdated: number
}

interface PerformanceReport {
  current: PerformanceMetrics
  average: Pick<PerformanceMetrics, 'renderTime' | 'layoutTime' | 'fps' | 'memoryUsage'>
  score: number
  suggestions: string[]
  historyLength: number
}

interface PerformanceMonitorOptions {
  enabled: boolean
  interval: number
  isMonitoring: Ref<boolean>
  metrics: Ref<PerformanceMetrics>
  history: Ref<PerformanceMetrics[]>
  onSample: () => void
}

function createPerformanceConfig(overrides: Partial<PerformanceConfig>): PerformanceConfig {
  return {
    ...DEFAULT_CONFIG,
    ...overrides,
    virtualizationThreshold: overrides.virtualizationThreshold ?? DEFAULT_VIRTUALIZATION_THRESHOLD
  }
}

function createInitialMetrics(): PerformanceMetrics {
  return {
    renderTime: 0,
    layoutTime: 0,
    itemCount: 0,
    memoryUsage: 0,
    fps: 60,
    lastUpdated: Date.now()
  }
}

function getVirtualizationThreshold(config: PerformanceConfig): number {
  return config.virtualizationThreshold ?? DEFAULT_VIRTUALIZATION_THRESHOLD
}

function calculatePerformanceScore(metrics: PerformanceMetrics): number {
  const { renderTime, layoutTime, fps, itemCount } = metrics
  let score = 100

  if (renderTime > 16) score -= Math.min(30, (renderTime - 16) / 2)
  if (layoutTime > 10) score -= Math.min(20, (layoutTime - 10) / 2)
  if (fps < 60) score -= Math.min(25, (60 - fps) / 2)
  if (itemCount > 50) score -= Math.min(25, (itemCount - 50) / 10)

  return Math.max(0, Math.floor(score))
}

function buildOptimizationSuggestions(metrics: PerformanceMetrics, config: PerformanceConfig): string[] {
  const suggestions: string[] = []
  const { renderTime, layoutTime, itemCount, fps } = metrics

  if (itemCount >= getVirtualizationThreshold(config) && !config.enableVirtualization) {
    suggestions.push('建议启用虚拟化以提高大数据集性能')
  }
  if (renderTime > 16) {
    suggestions.push('渲染时间过长，考虑减少DOM操作或启用防抖')
  }
  if (layoutTime > 10) {
    suggestions.push('布局计算耗时较长，考虑优化布局算法')
  }
  if (fps < 45) {
    suggestions.push('帧率较低，建议减少动画效果或优化渲染')
  }
  if (!config.enableLazyLoading && itemCount > 30) {
    suggestions.push('考虑启用懒加载以改善初始加载性能')
  }

  return suggestions
}

function appendPerformanceHistory(history: Ref<PerformanceMetrics[]>, metrics: PerformanceMetrics) {
  history.value.push({ ...metrics })
  if (history.value.length > MAX_HISTORY_LENGTH) {
    history.value.shift()
  }
}

function estimateLayoutMemory(layout: GridLayoutPlusItem[]): number {
  const itemSize = 200
  const layoutSize = JSON.stringify(layout).length * 2
  return (layout.length * itemSize + layoutSize) / 1024
}

function averagePerformanceHistory(history: PerformanceMetrics[]) {
  const totals = history.reduce(
    (acc, metric) => ({
      renderTime: acc.renderTime + metric.renderTime,
      layoutTime: acc.layoutTime + metric.layoutTime,
      fps: acc.fps + metric.fps,
      memoryUsage: acc.memoryUsage + metric.memoryUsage
    }),
    { renderTime: 0, layoutTime: 0, fps: 0, memoryUsage: 0 }
  )

  return {
    renderTime: totals.renderTime / history.length,
    layoutTime: totals.layoutTime / history.length,
    fps: totals.fps / history.length,
    memoryUsage: totals.memoryUsage / history.length
  }
}

function buildPerformanceReport(
  history: PerformanceMetrics[],
  current: PerformanceMetrics,
  score: number,
  suggestions: string[]
): PerformanceReport | null {
  if (history.length === 0) return null

  return {
    current,
    average: averagePerformanceHistory(history),
    score,
    suggestions,
    historyLength: history.length
  }
}

function createMeasurementTools(metrics: Ref<PerformanceMetrics>) {
  const measureRenderTime = async (renderFn: () => Promise<void> | void) => {
    const startTime = performance.now()

    try {
      await renderFn()
      await nextTick()
    } catch (error) {
      console.error('[GridPerformance] Render function error:', error)
    }

    const renderTime = performance.now() - startTime
    metrics.value.renderTime = renderTime
    metrics.value.lastUpdated = Date.now()
    return renderTime
  }

  const measureLayoutTime = (layoutFn: () => void) => {
    const startTime = performance.now()

    try {
      layoutFn()
    } catch (error) {
      console.error('[GridPerformance] Layout function error:', error)
    }

    const layoutTime = performance.now() - startTime
    metrics.value.layoutTime = layoutTime
    metrics.value.lastUpdated = Date.now()
    return layoutTime
  }

  const estimateMemoryUsage = (layout: GridLayoutPlusItem[]) => {
    try {
      const totalSize = estimateLayoutMemory(layout)
      metrics.value.memoryUsage = totalSize
      return totalSize
    } catch (error) {
      console.error('[GridPerformance] Failed to estimate memory usage:', error)
      return 0
    }
  }

  return {
    measureRenderTime,
    measureLayoutTime,
    estimateMemoryUsage
  }
}

function createOptimizationTools(config: Ref<PerformanceConfig>, metrics: Ref<PerformanceMetrics>) {
  const applyAutoOptimizations = () => {
    const { itemCount, renderTime } = metrics.value
    let optimized = false

    if (renderTime > 20 && config.value.debounceDelay < 200) {
      config.value.debounceDelay = Math.min(200, config.value.debounceDelay + 50)
      optimized = true
      gridPerformanceLogger.debug(`Increased debounce delay to ${config.value.debounceDelay}ms`)
    }
    if (itemCount > 30 && !config.value.enableLazyLoading) {
      config.value.enableLazyLoading = true
      optimized = true
      gridPerformanceLogger.debug('Auto-enabled lazy loading')
    }

    return optimized
  }

  const createDebouncedFunction = <T extends (...args: any[]) => any>(fn: T, customDelay?: number) => {
    return debounce(fn, customDelay || config.value.debounceDelay)
  }

  const createThrottledFunction = <T extends (...args: any[]) => any>(fn: T, customDelay?: number) => {
    return throttle(fn, customDelay || config.value.throttleDelay)
  }

  return {
    applyAutoOptimizations,
    createDebouncedFunction,
    createThrottledFunction
  }
}

function createFpsMonitor(isMonitoring: Ref<boolean>, metrics: Ref<PerformanceMetrics>) {
  let frameCount = 0
  let startTime = 0
  let animationFrameId = 0

  const measure = () => {
    const now = performance.now()
    if (startTime === 0) {
      startTime = now
      frameCount = 0
    }

    frameCount++
    if (now - startTime >= 1000) {
      metrics.value.fps = Math.round((frameCount * 1000) / (now - startTime))
      startTime = now
      frameCount = 0
    }

    if (isMonitoring.value) {
      animationFrameId = requestAnimationFrame(measure)
    }
  }

  return {
    start() {
      startTime = 0
      frameCount = 0
      measure()
    },
    stop() {
      if (animationFrameId) {
        cancelAnimationFrame(animationFrameId)
        animationFrameId = 0
      }
    }
  }
}

function createPerformanceMonitor(options: PerformanceMonitorOptions) {
  const fpsMonitor = createFpsMonitor(options.isMonitoring, options.metrics)
  let timer: ReturnType<typeof setInterval> | null = null

  const startMonitoring = () => {
    if (options.isMonitoring.value || !options.enabled) return

    options.isMonitoring.value = true
    fpsMonitor.start()
    timer = setInterval(() => {
      appendPerformanceHistory(options.history, options.metrics.value)
      options.onSample()
    }, options.interval)

    gridPerformanceLogger.debug('Monitoring started')
  }

  const stopMonitoring = () => {
    if (!options.isMonitoring.value) return

    options.isMonitoring.value = false
    fpsMonitor.stop()
    if (timer) {
      clearInterval(timer)
      timer = null
    }

    gridPerformanceLogger.debug('Monitoring stopped')
  }

  return {
    startMonitoring,
    stopMonitoring
  }
}

export function useGridPerformance(options: UseGridPerformanceOptions = {}) {
  const { performanceConfig = {}, enableMonitoring = true, monitoringInterval = 1000, autoOptimize = false } = options
  const config = ref<PerformanceConfig>(createPerformanceConfig(performanceConfig))
  const metrics = ref<PerformanceMetrics>(createInitialMetrics())
  const isMonitoring = ref(false)
  const performanceHistory = ref<PerformanceMetrics[]>([])

  const needsVirtualization = computed(() => {
    return metrics.value.itemCount >= getVirtualizationThreshold(config.value)
  })
  const performanceScore = computed(() => calculatePerformanceScore(metrics.value))
  const optimizationSuggestions = computed(() => buildOptimizationSuggestions(metrics.value, config.value))
  const measurementTools = createMeasurementTools(metrics)
  const optimizationTools = createOptimizationTools(config, metrics)
  const monitor = createPerformanceMonitor({
    enabled: enableMonitoring,
    interval: monitoringInterval,
    isMonitoring,
    metrics,
    history: performanceHistory,
    onSample: () => {
      if (autoOptimize && performanceScore.value < 60) {
        optimizationTools.applyAutoOptimizations()
      }
    }
  })

  const getPerformanceReport = () => {
    return buildPerformanceReport(
      performanceHistory.value,
      metrics.value,
      performanceScore.value,
      optimizationSuggestions.value
    )
  }

  onMounted(() => {
    if (enableMonitoring) {
      monitor.startMonitoring()
    }
  })

  onUnmounted(() => {
    monitor.stopMonitoring()
  })

  return {
    config,
    metrics,
    performanceScore,
    needsVirtualization,
    optimizationSuggestions,
    isMonitoring,
    startMonitoring: monitor.startMonitoring,
    stopMonitoring: monitor.stopMonitoring,
    measureRenderTime: measurementTools.measureRenderTime,
    measureLayoutTime: measurementTools.measureLayoutTime,
    estimateMemoryUsage: measurementTools.estimateMemoryUsage,
    createDebouncedFunction: optimizationTools.createDebouncedFunction,
    createThrottledFunction: optimizationTools.createThrottledFunction,
    applyAutoOptimizations: optimizationTools.applyAutoOptimizations,
    getPerformanceReport,
    performanceHistory
  }
}
