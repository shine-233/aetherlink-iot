/*
 * 文件用途：管理 ECharts 实例集合，提供统一注册、获取和释放能力。
 * 核心逻辑：围绕 chart id 或 DOM 维护实例引用，避免重复初始化。
 * 关键注意事项：必须在组件销毁时释放实例，防止内存泄漏和 resize 监听残留。
 * 重构建议：后续可与图表 Hook 合并生命周期边界。
 */
/**
 * ECharts 全局管理器
 * 解决 ECharts 组件重复注册问题
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
  // 最常用的图表类型
  BarChart,
  LineChart,
  PieChart,

  // 基础组件
  TitleComponent,
  LegendComponent,
  TooltipComponent,
  GridComponent,
  DatasetComponent,
  TransformComponent,

  // 基础功能
  LabelLayout,
  UniversalTransition,

  // 渲染器
  CanvasRenderer
]

type EChartsExtensionLoader = () => Promise<any[]>

// 扩展组件映射表 - 真正按需加载，避免普通图表入口静态引入全部扩展。
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
 * 初始化 ECharts 基础组件注册
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
    // 捕获重复注册错误，但不影响程序执行
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
          console.error('⚠️ ECharts 扩展组件注册警告:', error)
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
      console.error('⚠️ ECharts 扩展组件注册警告:', error)
    }
  }
}

/**
 * 获取 ECharts 实例
 * 确保组件已注册后再创建实例
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
  // 确保组件已注册
  initEChartsComponents()

  // 创建实例（默认使用 AetherLink 品牌主题）
  return echarts.init(dom, theme || 'aetherlink', opts)
}

/**
 * 安全地使用 ECharts
 * 提供统一的 ECharts 访问接口
 */
export function useEChartsInstance() {
  // 确保组件已注册
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

// 注意：不在此处自动初始化，改由 main.ts 在 requestIdleCallback 中延迟加载，以减少启动内存占用

export default {
  initEChartsComponents,
  registerEChartsExtensions,
  createEChartsInstance,
  useEChartsInstance,
  resetEChartsRegistration
}
