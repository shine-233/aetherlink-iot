/*
 * 鏂囦欢鐢ㄩ€旓細绠＄悊 ECharts 瀹炰緥闆嗗悎锛屾彁渚涚粺涓€娉ㄥ唽銆佽幏鍙栧拰閲婃斁鑳藉姏銆? * 鏍稿績閫昏緫锛氬洿缁?chart id 鎴?DOM 缁存姢瀹炰緥寮曠敤锛岄伩鍏嶉噸澶嶅垵濮嬪寲銆? * 鍏抽敭娉ㄦ剰浜嬮」锛氬繀椤诲湪缁勪欢閿€姣佹椂閲婃斁瀹炰緥锛岄槻姝㈠唴瀛樻硠婕忓拰 resize 鐩戝惉娈嬬暀銆? * 閲嶆瀯寤鸿锛氬悗缁彲涓庡浘琛?Hook 鍚堝苟鐢熷懡鍛ㄦ湡杈圭晫銆? */
/**
 * ECharts 鍏ㄥ眬绠＄悊鍣? * 瑙ｅ喅 ECharts 缁勪欢閲嶅娉ㄥ唽闂
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

// 鍏ㄥ眬鏍囪瘑锛岀‘淇濆彧娉ㄥ唽涓€娆?let isEChartsRegistered = false

// 鍩虹蹇呴渶缁勪欢 - 棣栨鍔犺浇鏃舵敞鍐?const BASIC_COMPONENTS = [
  // 鏈€甯哥敤鐨勫浘琛ㄧ被鍨?  BarChart,
  LineChart,
  PieChart,

  // 鍩虹缁勪欢
  TitleComponent,
  LegendComponent,
  TooltipComponent,
  GridComponent,
  DatasetComponent,
  TransformComponent,

  // 鍩虹鍔熻兘
  LabelLayout,
  UniversalTransition,

  // 娓叉煋鍣?  CanvasRenderer
]

type EChartsExtensionLoader = () => Promise<any[]>

// 鎵╁睍缁勪欢鏄犲皠琛?- 鐪熸鎸夐渶鍔犺浇锛岄伩鍏嶆櫘閫氬浘琛ㄥ叆鍙ｉ潤鎬佸紩鍏ュ叏閮ㄦ墿灞曘€?const EXTENDED_COMPONENT_LOADERS: Record<string, EChartsExtensionLoader> = {
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

// 宸叉敞鍐岀殑鎵╁睍缁勪欢
const registeredExtensions = new Set<string>()
const pendingExtensionRegistrations = new Map<string, Promise<void>>()

/**
 * 鍒濆鍖?ECharts 鍩虹缁勪欢娉ㄥ唽
 * 鍙敞鍐屾渶甯哥敤鐨勭粍浠讹紝鍑忓皯鍒濆鍐呭瓨鍗犵敤
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
    // 鎹曡幏閲嶅娉ㄥ唽閿欒锛屼絾涓嶅奖鍝嶇▼搴忔墽琛?    if (error instanceof Error && error.message.includes('exists')) {
      isEChartsRegistered = true
    } else {
      throw error
    }
  }
}

/**
 * 鎸夐渶娉ㄥ唽鎵╁睍缁勪欢
 * @param componentTypes 闇€瑕佹敞鍐岀殑缁勪欢绫诲瀷鏁扮粍
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
          console.error('鈿狅笍 ECharts 鎵╁睍缁勪欢娉ㄥ唽璀﹀憡:', error)
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
      console.error('鈿狅笍 ECharts 鎵╁睍缁勪欢娉ㄥ唽璀﹀憡:', error)
    }
  }
}

/**
 * 鑾峰彇 ECharts 瀹炰緥
 * 纭繚缁勪欢宸叉敞鍐屽悗鍐嶅垱寤哄疄渚? */
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
  // 纭繚缁勪欢宸叉敞鍐?  initEChartsComponents()

  // 鍒涘缓瀹炰緥锛堥粯璁や娇鐢?AetherLink 鍝佺墝涓婚锛?  return echarts.init(dom, theme || 'aetherlink', opts)
}

/**
 * 瀹夊叏鍦颁娇鐢?ECharts
 * 鎻愪緵缁熶竴鐨?ECharts 璁块棶鎺ュ彛
 */
export function useEChartsInstance() {
  // 纭繚缁勪欢宸叉敞鍐?  initEChartsComponents()

  return {
    echarts,
    createInstance: createEChartsInstance,
    isRegistered: () => isEChartsRegistered
  }
}

/**
 * 閲嶇疆娉ㄥ唽鐘舵€侊紙浠呯敤浜庢祴璇曪級
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