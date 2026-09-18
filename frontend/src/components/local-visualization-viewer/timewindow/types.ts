/**
 * 文件用途：Timewindow 2.0（时间窗口控制器）核心类型定义。
 *
 * 对标 ThingsBoard 3.8.0 / 4.0 Timewindow 规范：
 * 1. 实时模式（Realtime）：相对过去某时间跨度，支持流式/定时周期刷新；
 * 2. 历史模式（History）：快捷自然区间（今天/昨天/本周等）与精确起止时间戳；
 * 3. 聚合设置（Aggregation）：函数（none/avg/min/max/sum/count）与自动/固定分组粒度；
 * 4. 时区设置（Timezone）：跟随浏览器、UTC、指定时区；
 * 5. 小部件支持局部 Timewindow 覆盖看板全局 Timewindow。
 */

export type TimewindowType = 'realtime' | 'history'

export type RealtimeIntervalLabel =
  | '1m'
  | '5m'
  | '15m'
  | '30m'
  | '1h'
  | '2h'
  | '6h'
  | '12h'
  | '1d'
  | '7d'
  | '30d'

export type QuickHistoryInterval =
  | 'today'
  | 'yesterday'
  | 'this_week'
  | 'prev_week'
  | 'this_month'
  | 'prev_month'
  | 'last_7d'
  | 'last_30d'

export type AggregationFunc = 'none' | 'avg' | 'min' | 'max' | 'sum' | 'count'

export type RefreshInterval = 0 | 1000 | 5000 | 10000 | 30000 | 60000

export type TimezoneMode = 'browser' | 'utc' | string

export interface RealtimeConfig {
  interval: RealtimeIntervalLabel | number
}

export interface HistoryConfig {
  quickInterval?: QuickHistoryInterval
  fixedRange?: {
    startTime: number
    endTime: number
  }
}

export interface AggregationConfig {
  func: AggregationFunc
  /** 分组粒度（毫秒），不传或为 0 时表示自动推导（auto） */
  interval?: number
}

export interface TimewindowConfig {
  type: TimewindowType
  realtime?: RealtimeConfig
  history?: HistoryConfig
  aggregation?: AggregationConfig
  refreshInterval?: RefreshInterval
  timezone?: TimezoneMode
}

/** 解析完成的标准起止范围，供遥测 API / 数据查询引擎消费 */
export interface ResolvedTimeRange {
  startTime: number
  endTime: number
  groupingInterval: number
  aggregationFunc: AggregationFunc
  timezone: string
  isRealtime: boolean
}
