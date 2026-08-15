/*
 * 文件用途：提供前端共享 ECharts 生命周期 hook。
 * 核心逻辑：创建图表实例，绑定容器尺寸、主题变化、渲染更新和销毁回调。
 * 关键注意事项：该 hook 会影响所有复用图表入口；修改尺寸监听、主题刷新或销毁顺序时需防止内存泄漏。
 * 重构建议：建议继续补充图表初始化、主题切换、容器 resize 和销毁回调的单元测试。
 */
import { computed, effectScope, nextTick, onScopeDispose, ref, watch } from 'vue'
import type { ComposeOption, ECharts } from 'echarts/core'
import type {
  BarSeriesOption,
  GaugeSeriesOption,
  LineSeriesOption,
  PictorialBarSeriesOption,
  PieSeriesOption,
  RadarSeriesOption,
  ScatterSeriesOption
} from 'echarts/charts'
import type {
  DatasetComponentOption,
  GridComponentOption,
  LegendComponentOption,
  TitleComponentOption,
  ToolboxComponentOption,
  TooltipComponentOption
} from 'echarts/components'
import type { MaybeComputedElementRef, MaybeElement } from '@vueuse/core'
import { useElementSize } from '@vueuse/core'
import { useThemeStore } from '@/store/modules/theme'

export type ECOption = ComposeOption<
  | BarSeriesOption
  | LineSeriesOption
  | PieSeriesOption
  | ScatterSeriesOption
  | PictorialBarSeriesOption
  | RadarSeriesOption
  | GaugeSeriesOption
  | TitleComponentOption
  | LegendComponentOption
  | TooltipComponentOption
  | GridComponentOption
  | ToolboxComponentOption
  | DatasetComponentOption
>

export interface ChartHooks {
  onRender?: (chart: ECharts) => void | Promise<void>
  onUpdated?: (chart: ECharts) => void | Promise<void>
  onDestroy?: (chart: ECharts) => void | Promise<void>
}

type EChartsInitOptions = {
  devicePixelRatio?: number
  renderer?: 'canvas' | 'svg'
  useDirtyRect?: boolean
  useCoarsePointer?: boolean
  pointerSize?: number
  ssr?: boolean
  width?: number
  height?: number
  locale?: string
}

export interface EChartsHookOptions {
  initOptions?: EChartsInitOptions
  hideLoadingAfterDefaultRender?: boolean
  requiredExtensions?: string[]
}

/**
 * create shared echarts lifecycle hook
 *
 * @param optionsFactory echarts options factory function
 * @param hooks
 * @param hookOptions entrypoint-specific compatibility options
 */
export function createEChartsHook<T extends ECOption>(
  optionsFactory: () => T,
  hooks: ChartHooks = {},
  hookOptions: EChartsHookOptions = {}
) {
  const scope = effectScope()

  const themeStore = useThemeStore()
  const darkMode = computed(() => themeStore.darkMode)

  const domRef = ref<HTMLElement | null>(null)
  const initialSize = { width: 0, height: 0 }
  const { width, height } = useElementSize(domRef as unknown as MaybeComputedElementRef<MaybeElement>, initialSize)

  let chart: ECharts | null = null
  let chartCreation: Promise<void> | null = null
  let active = true
  const chartOptions: T = optionsFactory()

  const {
    onRender = instance => {
      const textColor = darkMode.value ? 'rgb(224, 224, 224)' : 'rgb(31, 31, 31)'
      const maskColor = darkMode.value ? 'rgba(0, 0, 0, 0.4)' : 'rgba(255, 255, 255, 0.8)'

      instance.showLoading({
        color: themeStore.themeColor,
        textColor,
        fontSize: 14,
        maskColor
      })

      if (hookOptions.hideLoadingAfterDefaultRender) {
        instance.hideLoading()
      }
    },
    onUpdated = instance => {
      instance.hideLoading()
    },
    onDestroy
  } = hooks

  /**
   * whether can render chart
   *
   * when domRef is ready and initialSize is valid
   */
  function canRender() {
    return domRef.value && initialSize.width > 0 && initialSize.height > 0
  }

  /** is chart rendered */
  function isRendered() {
    return Boolean(domRef.value && chart)
  }

  /**
   * update chart options
   *
   * @param callback callback function
   */
  async function updateOptions(callback: (opts: T, optsFactory: () => T) => ECOption = () => chartOptions) {
    if (!isRendered()) return

    const updatedOpts = callback(chartOptions, optionsFactory)

    Object.assign(chartOptions, updatedOpts)

    if (isRendered()) {
      chart?.clear()
    }

    chart?.setOption({ ...updatedOpts, backgroundColor: 'transparent' })

    await onUpdated?.(chart!)
  }

  /** render chart */
  async function render() {
    if (isRendered()) return
    if (chartCreation) {
      await chartCreation
      return
    }

    chartCreation = (async () => {
      const chartTheme = darkMode.value ? 'dark' : 'light'

      await nextTick()

      if (!active || !domRef.value || chart) {
        return
      }

      const { createEChartsInstance, registerEChartsExtensions } = await import('@/utils/echarts/echarts-manager')
      if (!active || !domRef.value || chart) {
        return
      }

      if (hookOptions.requiredExtensions?.length) {
        await registerEChartsExtensions(hookOptions.requiredExtensions)
      }
      if (!active || !domRef.value || chart) {
        return
      }

      chart = createEChartsInstance(domRef.value as unknown as HTMLElement, chartTheme, hookOptions.initOptions)
      chart.setOption({ ...chartOptions, backgroundColor: 'transparent' })

      await onRender?.(chart)
    })().finally(() => {
      chartCreation = null
    })

    await chartCreation
  }

  /** resize chart */
  function resize() {
    chart?.resize()
  }

  /** destroy chart */
  async function destroy() {
    if (!chart) return

    await onDestroy?.(chart)
    chart?.dispose()
    chart = null
  }

  /** change chart theme */
  async function changeTheme() {
    await destroy()
    await render()
    await onUpdated?.(chart!)
  }

  /**
   * render chart by size
   *
   * @param w width
   * @param h height
   */
  async function renderChartBySize(w: number, h: number) {
    initialSize.width = w
    initialSize.height = h

    // size is abnormal, destroy chart
    if (!canRender()) {
      await destroy()

      return
    }

    // resize chart
    if (isRendered()) {
      resize()
    }

    // render chart
    await render()
  }

  scope.run(() => {
    watch([width, height], ([newWidth, newHeight]) => {
      renderChartBySize(newWidth, newHeight)
    })

    watch(darkMode, () => {
      changeTheme()
    })
  })

  onScopeDispose(() => {
    active = false
    destroy()
    scope.stop()
  })

  return {
    domRef,
    updateOptions
  }
}

/**
 * use echarts
 *
 * @param optionsFactory echarts options factory function
 * @param hooks
 */
export function useEcharts<T extends ECOption>(optionsFactory: () => T, hooks: ChartHooks = {}) {
  return createEChartsHook(optionsFactory, hooks)
}
