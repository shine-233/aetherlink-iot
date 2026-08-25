/*
 * ECharts 实例管理器：
 * 按 chart id 或 DOM 元素持有实例，统一处理 resize 与销毁；
 * 业务组件通过 Hook 消费，不直接接触 echarts/core 单例状态。
 */

import * as echarts from 'echarts/core'
import { BarChart, LineChart, PieChart } from 'echarts/charts'
import {
  TitleComponent,
  LegendComponent,
  TooltipComponent,
  GridComponent,
  DatasetComponent,
  TransformComponent
} from 'echarts/components'
import { aetherLinkTheme } from './aetherlink-theme'
import { LabelLayout, UniversalTransition } from 'echarts/features'
import { CanvasRenderer } from 'echarts/renderers'

// 全局标识，确保只注册一次
let isEChartsRegistered = false

// 基础必需组件 - 首次加载时注册
const BASIC_COMPONENTS = [
  BarChart,
  LineChart,
  PieChart,

  // 渲染组件
  TitleComponent,
  LegendComponent,
  TooltipComponent,
  GridComponent,
  DatasetComponent,
  TransformComponent,

  // 布局与过渡
  LabelLayout,
  UniversalTransition,

  // 渲染器
  CanvasRenderer
]

type EChartsExtensionLoader = () => Promise<any[]>

// 扩展组件 - 首次用到对应图表类型时按需动态加载
const EXTENDED_COMPONENT_LOADERS: Record<string, EChartsExtensionLoader> = {
  scatter: async () => {
    const { ScatterChart } = await import('echarts/charts')
    return [ScatterChart]
  },
  gauge: async () => {
    const [{ GaugeChart }, { PolarComponent }] = await Promise.all([
      import('echarts/charts'),
      import('echarts/components')
    ])
    return [GaugeChart, PolarComponent]
  },
  radar: async () => {
    const [{ RadarChart }, { RadarComponent }] = await Promise.all([
      import('echarts/charts'),
      import('echarts/components')
    ])
    return [RadarChart, RadarComponent]
  },
  pictorial: async () => {
    const { PictorialBarChart } = await import('echarts/charts')
    return [PictorialBarChart]
  },
  funnel: async () => {
    const { FunnelChart } = await import('echarts/charts')
    return [FunnelChart]
  },
  sankey: async () => {
    const { SankeyChart } = await import('echarts/charts')
    return [SankeyChart]
  },
  tree: async () => {
    const { TreeChart } = await import('echarts/charts')
    return [TreeChart]
  },
  treemap: async () => {
    const { TreemapChart } = await import('echarts/charts')
    return [TreemapChart]
  },
  graph: async () => {
    const { GraphChart } = await import('echarts/charts')
    return [GraphChart]
  },
  boxplot: async () => {
    const { BoxplotChart } = await import('echarts/charts')
    return [BoxplotChart]
  },
  candlestick: async () => {
    const { CandlestickChart } = await import('echarts/charts')
    return [CandlestickChart]
  },
  effectScatter: async () => {
    const { EffectScatterChart } = await import('echarts/charts')
    return [EffectScatterChart]
  },
  heatmap: async () => {
    const { HeatmapChart } = await import('echarts/charts')
    return [HeatmapChart]
  },
  lines: async () => {
    const { LinesChart } = await import('echarts/charts')
    return [LinesChart]
  },
  map: async () => {
    const [{ MapChart }, { GeoComponent }] = await Promise.all([
      import('echarts/charts'),
      import('echarts/components')
    ])
    return [MapChart, GeoComponent]
  },
  parallel: async () => {
    const [{ ParallelChart }, { ParallelComponent, SingleAxisComponent }] = await Promise.all([
      import('echarts/charts'),
      import('echarts/components')
    ])
    return [ParallelChart, ParallelComponent, SingleAxisComponent]
  },
  sunburst: async () => {
    const { SunburstChart } = await import('echarts/charts')
    return [SunburstChart]
  },
  themeRiver: async () => {
    const { ThemeRiverChart } = await import('echarts/charts')
    return [ThemeRiverChart]
  },
  toolbox: async () => {
    const { ToolboxComponent } = await import('echarts/components')
    return [ToolboxComponent]
  },
  dataZoom: async () => {
    const { DataZoomComponent } = await import('echarts/components')
    return [DataZoomComponent]
  },
  visualMap: async () => {
    const { VisualMapComponent } = await import('echarts/components')
    return [VisualMapComponent]
  },
  timeline: async () => {
    const { TimelineComponent } = await import('echarts/components')
    return [TimelineComponent]
  },
  calendar: async () => {
    const { CalendarComponent } = await import('echarts/components')
    return [CalendarComponent]
  },
  graphic: async () => {
    const { GraphicComponent } = await import('echarts/components')
    return [GraphicComponent]
  },
  markLine: async () => {
    const { MarkLineComponent } = await import('echarts/components')
    return [MarkLineComponent]
  },
  markPoint: async () => {
    const { MarkPointComponent } = await import('echarts/components')
    return [MarkPointComponent]
  },
  markArea: async () => {
    const { MarkAreaComponent } = await import('echarts/components')
    return [MarkAreaComponent]
  },
  brush: async () => {
    const { BrushComponent } = await import('echarts/components')
    return [BrushComponent]
  },
  axisPointer: async () => {
    const { AxisPointerComponent } = await import('echarts/components')
    return [AxisPointerComponent]
  },
  svg: async () => {
    const { SVGRenderer } = await import('echarts/renderers')
    return [SVGRenderer]
  }
}

// 已注册的扩展组件
const registeredExtensions = new Set<string>()
const pendingExtensionRegistrations = new Map<string, Promise<void>>()

/**
 * 鍒濆鍖?ECharts 基础组件注册
 * 只注册最常用的组件，减少初始内存占用
 */
export function initEChartsComponents() {
  if (isEChartsRegistered) {
    return
  }

  try {
    echarts.use(BASIC_COMPONENTS)
    echarts.registerTheme('aetherlink', aetherLinkTheme as any)
    isEChartsRegistered = true
  } catch (error) {
    // 重复注册同一主题时 echarts 会抛 already exists 错误，这里视为已注册
    if (error instanceof Error && error.message.includes('exists')) {
      isEChartsRegistered = true
    } else {
      throw error
    }
  }
}

/**
 * 按需注册扩展组件
 * @param componentTypes 需要注册的组件类型数组
 */
export async function registerEChartsExtensions(componentTypes: string[]) {
  const registrationTasks = componentTypes
    .map(type => {
      if (registeredExtensions.has(type)) return null

      const inFlightRegistration = pendingExtensionRegistrations.get(type)
      if (inFlightRegistration) return inFlightRegistration

      const loader = EXTENDED_COMPONENT_LOADERS[type]
      if (!loader) return null

      const registrationPromise = loader()
        .then(components => {
          if (components.length > 0) {
            echarts.use(components)
          }
          registeredExtensions.add(type)
          if (process.env.NODE_ENV === 'development') {
            /* intentionally empty */
          }
        })
        .catch(error => {
          console.error('鈿狅笍 ECharts 扩展组件注册警告:', error)
        })
        .finally(() => {
          pendingExtensionRegistrations.delete(type)
        })

      pendingExtensionRegistrations.set(type, registrationPromise)
      return registrationPromise
    })
    .filter(Boolean) as Array<Promise<void>>

  if (registrationTasks.length > 0) {
    try {
      await Promise.all(registrationTasks)
    } catch (error) {
      console.error('鈿狅笍 ECharts 扩展组件注册警告:', error)
    }
  }
}

/**
 * 获取 ECharts 实例
 * 默认使用 AetherLink 品牌主题，可传自定义主题与初始化参数。
 */
export function createEChartsInstance(
  dom: HTMLElement,
  theme?: string | object,
  opts?: {
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
): echarts.ECharts {
  // 确保基础组件已注册（幂等）
  initEChartsComponents()

  // 未指定主题时回退到 AetherLink 品牌主题
  return echarts.init(dom, theme || 'aetherlink', opts)
}

/**
 * 瀹夊叏鍦颁娇鐢?ECharts
 * 鎻愪緵缁熶竴鐨?ECharts 访问接口
 */
export function useEChartsInstance() {
  // 确保基础组件已注册（幂等）
  initEChartsComponents()

  return {
    echarts,
    createInstance: createEChartsInstance,
    isRegistered: () => isEChartsRegistered
  }
}

/**
 * 重置注册状态（仅用于测试）
 */
export function resetEChartsRegistration() {
  isEChartsRegistered = false
  registeredExtensions.clear()
  pendingExtensionRegistrations.clear()
  if (process.env.NODE_ENV === 'development') {
    /* intentionally empty */
  }
}

// 娉ㄦ剰锛氫笉鍦ㄦ澶勮嚜鍔ㄥ垵濮嬪寲锛屾敼鐢?main.ts 鍦?requestIdleCallback 涓欢杩熷姞杞斤紝浠ュ噺灏戝惎鍔ㄥ唴瀛樺崰鐢?
export default {
  initEChartsComponents,
  registerEChartsExtensions,
  createEChartsInstance,
  useEChartsInstance,
  resetEChartsRegistration
}
