/**
 * 文件用途：处理网格在不同断点和列数下的响应式布局。
 * 核心逻辑：监听容器或断点变化，将布局映射到当前 breakpoint 对应的列配置。
 * 关键注意事项：断点切换不应丢失用户已有布局，也不应污染原始持久化结构。
 * 重构建议：可补充断点转换矩阵测试，并把断点计算抽为纯函数。
 */
import { computed, onUnmounted, ref, watch } from 'vue'
import type { ComputedRef, Ref } from 'vue'
import type { GridLayoutPlusItem, ResponsiveLayout } from '../gridLayoutPlusTypes'
import { createResponsiveLayout, transformLayoutForBreakpoint } from '../gridLayoutPlusUtils'
import { createLogger } from '../../../../utils/logger'

const gridResponsiveLogger = createLogger('GridResponsive')

export interface UseGridResponsiveOptions {
  /** 是否启用响应式 */
  responsive?: boolean
  /** 断点配置 */
  breakpoints?: Record<string, number>
  /** 列数配置 */
  cols?: Record<string, number>
  /** 初始响应式布局 */
  initialResponsiveLayouts?: ResponsiveLayout
  /** 断点变化回调 */
  onBreakpointChange?: (breakpoint: string, layout: GridLayoutPlusItem[]) => void
}

interface ResponsiveSettings {
  responsive: boolean
  breakpoints: Record<string, number>
  cols: Record<string, number>
  initialResponsiveLayouts: ResponsiveLayout
  onBreakpointChange?: (breakpoint: string, layout: GridLayoutPlusItem[]) => void
}

interface ResponsiveState {
  currentBreakpoint: Ref<string>
  containerWidth: Ref<number>
  responsiveLayouts: Ref<ResponsiveLayout>
  isResponsive: Ref<boolean>
}

function createResponsiveSettings(options: UseGridResponsiveOptions): ResponsiveSettings {
  const {
    responsive = false,
    breakpoints = { lg: 1200, md: 996, sm: 768, xs: 480, xxs: 0 },
    cols = { lg: 12, md: 10, sm: 6, xs: 4, xxs: 2 },
    initialResponsiveLayouts = {},
    onBreakpointChange
  } = options

  return {
    responsive,
    breakpoints,
    cols,
    initialResponsiveLayouts,
    onBreakpointChange
  }
}

function createResponsiveState(settings: ResponsiveSettings): ResponsiveState {
  return {
    currentBreakpoint: ref<string>('lg'),
    containerWidth: ref(0),
    responsiveLayouts: ref<ResponsiveLayout>(settings.initialResponsiveLayouts),
    isResponsive: ref(settings.responsive)
  }
}

function createResponsiveComputed(state: ResponsiveState, settings: ResponsiveSettings) {
  const { currentBreakpoint } = state
  const { breakpoints, cols } = settings

  const currentCols = computed(() => {
    return cols[currentBreakpoint.value] || cols.lg || 12
  })

  const sortedBreakpoints = computed(() => {
    return Object.entries(breakpoints).sort(([, a], [, b]) => b - a) // 从大到小排序
  })

  const breakpointInfo = computed(() => {
    const bp = currentBreakpoint.value
    return {
      name: bp,
      width: breakpoints[bp] || 0,
      cols: cols[bp] || 12,
      isXs: bp === 'xxs' || bp === 'xs',
      isSm: bp === 'sm',
      isMd: bp === 'md',
      isLg: bp === 'lg',
      isXl: bp === 'xl'
    }
  })

  return {
    currentCols,
    sortedBreakpoints,
    breakpointInfo
  }
}

function findBreakpoint(width: number, sortedBreakpoints: [string, number][]): string {
  for (const [bp, minWidth] of sortedBreakpoints) {
    if (width >= minWidth) {
      return bp
    }
  }

  const fallback = sortedBreakpoints[sortedBreakpoints.length - 1] as [string, number]
  return fallback[0] || 'xs'
}

function createResponsiveLayouts(
  state: ResponsiveState,
  settings: ResponsiveSettings,
  currentCols: ComputedRef<number>
) {
  const { currentBreakpoint, responsiveLayouts, isResponsive } = state
  const { breakpoints, cols, onBreakpointChange } = settings

  const transformLayoutForCurrentBreakpoint = (
    sourceLayout: GridLayoutPlusItem[],
    fromBreakpoint: string = 'lg'
  ): GridLayoutPlusItem[] => {
    if (!isResponsive.value) return sourceLayout

    try {
      const targetCols = currentCols.value
      const sourceCols = cols[fromBreakpoint] || 12

      return transformLayoutForBreakpoint(sourceLayout, sourceCols, targetCols)
    } catch (error) {
      console.error('[GridResponsive] Failed to transform layout:', error)
      return sourceLayout
    }
  }

  const setResponsiveLayout = (breakpoint: string, layout: GridLayoutPlusItem[]) => {
    if (!isResponsive.value) return

    try {
      responsiveLayouts.value[breakpoint] = [...layout]
      gridResponsiveLogger.debug(`Set layout for breakpoint: ${breakpoint}`)
    } catch (error) {
      console.error('[GridResponsive] Failed to set responsive layout:', error)
    }
  }

  const getResponsiveLayout = (breakpoint: string): GridLayoutPlusItem[] | null => {
    return responsiveLayouts.value[breakpoint] || null
  }

  const getCurrentResponsiveLayout = (): GridLayoutPlusItem[] | null => {
    return getResponsiveLayout(currentBreakpoint.value)
  }

  const hasResponsiveLayout = (breakpoint: string): boolean => {
    return !!responsiveLayouts.value[breakpoint]
  }

  const createFullResponsiveLayout = (baseLayout: GridLayoutPlusItem[]): ResponsiveLayout => {
    if (!isResponsive.value) return {}

    try {
      return createResponsiveLayout(baseLayout, breakpoints, cols)
    } catch (error) {
      console.error('[GridResponsive] Failed to create responsive layout:', error)
      return {}
    }
  }

  const handleBreakpointChange = (newBreakpoint: string, currentLayout: GridLayoutPlusItem[]) => {
    const previousBreakpoint = currentBreakpoint.value

    gridResponsiveLogger.debug(`Breakpoint changed: ${previousBreakpoint} -> ${newBreakpoint}`)

    if (currentLayout.length > 0) {
      setResponsiveLayout(previousBreakpoint, currentLayout)
    }

    currentBreakpoint.value = newBreakpoint

    let newLayout = getResponsiveLayout(newBreakpoint)

    if (!newLayout) {
      newLayout = transformLayoutForCurrentBreakpoint(currentLayout, previousBreakpoint)
      setResponsiveLayout(newBreakpoint, newLayout)
    }

    if (onBreakpointChange) {
      onBreakpointChange(newBreakpoint, newLayout)
    }

    return newLayout
  }

  return {
    transformLayoutForCurrentBreakpoint,
    setResponsiveLayout,
    getResponsiveLayout,
    getCurrentResponsiveLayout,
    hasResponsiveLayout,
    createFullResponsiveLayout,
    handleBreakpointChange
  }
}

function createContainerObserver(state: ResponsiveState) {
  const { containerWidth, isResponsive } = state
  let resizeObserver: ResizeObserver | null = null

  const observeContainer = (element: HTMLElement) => {
    if (!isResponsive.value) return

    const rect = element.getBoundingClientRect()
    containerWidth.value = rect.width

    if (window.ResizeObserver) {
      resizeObserver = new ResizeObserver((entries) => {
        for (const entry of entries) {
          const newWidth = entry.contentRect.width
          if (Math.abs(newWidth - containerWidth.value) > 10) {
            containerWidth.value = newWidth
          }
        }
      })

      resizeObserver.observe(element)
    } else {
      const handleResize = () => {
        const rect = element.getBoundingClientRect()
        containerWidth.value = rect.width
      }

      window.addEventListener('resize', handleResize)

      return () => window.removeEventListener('resize', handleResize)
    }
  }

  const unobserveContainer = () => {
    if (resizeObserver) {
      resizeObserver.disconnect()
      resizeObserver = null
    }
  }

  return {
    observeContainer,
    unobserveContainer
  }
}

function watchContainerBreakpoint(state: ResponsiveState, calculateBreakpoint: (width: number) => string) {
  const { containerWidth, currentBreakpoint, isResponsive } = state

  watch(containerWidth, (newWidth) => {
    if (!isResponsive.value || newWidth <= 0) return

    const newBreakpoint = calculateBreakpoint(newWidth)
    if (newBreakpoint !== currentBreakpoint.value) {
      gridResponsiveLogger.debug(`Container width changed: ${newWidth}px, new breakpoint: ${newBreakpoint}`)
    }
  })
}

function createBreakpointTools(
  state: ResponsiveState,
  settings: ResponsiveSettings,
  sortedBreakpoints: ComputedRef<[string, number][]>
) {
  const { currentBreakpoint, containerWidth } = state
  const { breakpoints, cols } = settings

  const calculateBreakpoint = (width: number): string => {
    return findBreakpoint(width, sortedBreakpoints.value)
  }

  const getBreakpointConfig = () => ({
    breakpoints,
    cols,
    current: currentBreakpoint.value,
    containerWidth: containerWidth.value
  })

  const isBreakpoint = (bp: string) => currentBreakpoint.value === bp

  const isBreakpointOrSmaller = (bp: string) => {
    const currentIndex = sortedBreakpoints.value.findIndex(([name]) => name === currentBreakpoint.value)
    const targetIndex = sortedBreakpoints.value.findIndex(([name]) => name === bp)
    return currentIndex >= targetIndex
  }

  const isBreakpointOrLarger = (bp: string) => {
    const currentIndex = sortedBreakpoints.value.findIndex(([name]) => name === currentBreakpoint.value)
    const targetIndex = sortedBreakpoints.value.findIndex(([name]) => name === bp)
    return currentIndex <= targetIndex
  }

  return {
    calculateBreakpoint,
    getBreakpointConfig,
    isBreakpoint,
    isBreakpointOrSmaller,
    isBreakpointOrLarger
  }
}

/**
 * 响应式网格布局管理Hook
 * 提供断点监听和响应式布局转换功能
 */
export function useGridResponsive(options: UseGridResponsiveOptions = {}) {
  const settings = createResponsiveSettings(options)
  const state = createResponsiveState(settings)
  const computedState = createResponsiveComputed(state, settings)
  const breakpointTools = createBreakpointTools(state, settings, computedState.sortedBreakpoints)
  const responsiveLayouts = createResponsiveLayouts(state, settings, computedState.currentCols)
  const containerObserver = createContainerObserver(state)

  watchContainerBreakpoint(state, breakpointTools.calculateBreakpoint)

  // 生命周期清理
  onUnmounted(() => {
    containerObserver.unobserveContainer()
  })

  return {
    // 状态
    currentBreakpoint: state.currentBreakpoint,
    containerWidth: state.containerWidth,
    responsiveLayouts: state.responsiveLayouts,
    isResponsive: state.isResponsive,

    // 计算属性
    currentCols: computedState.currentCols,
    breakpointInfo: computedState.breakpointInfo,
    sortedBreakpoints: computedState.sortedBreakpoints,

    // 方法
    observeContainer: containerObserver.observeContainer,
    unobserveContainer: containerObserver.unobserveContainer,
    handleBreakpointChange: responsiveLayouts.handleBreakpointChange,
    transformLayoutForCurrentBreakpoint: responsiveLayouts.transformLayoutForCurrentBreakpoint,

    // 布局管理
    setResponsiveLayout: responsiveLayouts.setResponsiveLayout,
    getResponsiveLayout: responsiveLayouts.getResponsiveLayout,
    getCurrentResponsiveLayout: responsiveLayouts.getCurrentResponsiveLayout,
    hasResponsiveLayout: responsiveLayouts.hasResponsiveLayout,
    createFullResponsiveLayout: responsiveLayouts.createFullResponsiveLayout,

    // 工具方法
    getBreakpointConfig: breakpointTools.getBreakpointConfig,
    isBreakpoint: breakpointTools.isBreakpoint,
    isBreakpointOrSmaller: breakpointTools.isBreakpointOrSmaller,
    isBreakpointOrLarger: breakpointTools.isBreakpointOrLarger,
    calculateBreakpoint: breakpointTools.calculateBreakpoint
  }
}
