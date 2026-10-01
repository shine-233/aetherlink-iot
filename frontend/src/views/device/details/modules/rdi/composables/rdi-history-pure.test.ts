/**
 * 文件用途: rdi-history-pure 纯函数单元测试(无 Vue / API mock)。
 * 核心逻辑: 覆盖时间范围(4 个预设 + custom)、查询参数、响应 normalize、分页状态机
 *   (空数据/分页失败/截断/重复页)、缺口检测、能耗统计、结果汇总、导出与图表配置。
 */
import { describe, expect, it, vi } from 'vitest'
import type { RDIHistoryParams } from '@/service/api/rdi'
import {
  DEFAULT_HISTORY_CHART_SERIES_KEYS,
  ENERGY_RANGE_DURATIONS,
  HISTORY_CHART_MAX_PAGES_PER_SERIES,
  HISTORY_GAP_THRESHOLD_MS,
  applyHistoryPage,
  buildCsvContent,
  buildExportFileUrl,
  buildHistoryChartData,
  buildHistoryChartOptions,
  buildHistoryChartQueryParams,
  buildHistoryExportCsvRows,
  buildHistoryExportQueryParams,
  calculateEnergyStats,
  createHistoryPaginationState,
  fetchHistoryChartData,
  fetchHistorySeriesPages,
  finalizeHistorySeriesResult,
  getExportedHistoryFilePath,
  hasRenderableHistoryChartData,
  insertHistoryGapMarkers,
  normalizeHistoryChartSeriesKeys,
  normalizeHistoryExportRows,
  normalizeHistoryPoints,
  normalizeHistoryTimestamp,
  normalizeHistoryTotal,
  normalizeHistoryValue,
  resolveHistoryRange,
  summarizeHistorySeriesResults,
  type HistoryPageResponse,
  type HistoryPaginationLimits,
  type HistoryPoint,
  type HistoryRange,
  type HistorySeriesResult,
  type PresetEnergyRange
} from './rdi-history-pure'

const NOW = 1_760_000_000_000
const RANGE: HistoryRange = [NOW - 3_600_000, NOW]
const SMALL_LIMITS: HistoryPaginationLimits = { pageSize: 3, maxPoints: 7, maxPages: 3 }

function pagePoints(startTs: number, count: number, stepMs = 30_000, startValue = 0) {
  return Array.from({ length: count }, (_, index) => ({ ts: startTs + index * stepMs, value: startValue + index }))
}

/** 按页号返回预置响应;超出预置页数时返回空页。 */
function scriptedFetcher(pages: Record<number, HistoryPageResponse | Error>) {
  return vi.fn(async (params: RDIHistoryParams): Promise<HistoryPageResponse> => {
    const scripted = pages[params.page ?? 1]
    if (scripted instanceof Error) throw scripted
    return scripted ?? { error: null, data: { list: [], total: null } }
  })
}

function seriesResult(overrides: Partial<HistorySeriesResult>): HistorySeriesResult {
  return {
    key: 'temperature_1',
    status: 'loaded',
    points: [],
    loadedCount: 0,
    expectedCount: null,
    missingCount: 0,
    invalidCount: 0,
    truncated: false,
    detectedGapCount: 0,
    ...overrides
  }
}

describe('resolveHistoryRange - 时间范围', () => {
  it.each(Object.entries(ENERGY_RANGE_DURATIONS) as [PresetEnergyRange, number][])(
    '预设 %s 以 now 为终点回推对应时长',
    (range, duration) => {
      expect(resolveHistoryRange(range, null, NOW)).toEqual([NOW - duration, NOW])
    }
  )

  it('四个预设时长分别为 1h / 24h / 7d / 30d', () => {
    expect(ENERGY_RANGE_DURATIONS).toEqual({
      last_1h: 3_600_000,
      last_24h: 86_400_000,
      last_7d: 604_800_000,
      last_30d: 2_592_000_000
    })
  })

  it('预设范围忽略残留的自定义范围', () => {
    expect(resolveHistoryRange('last_24h', [1, 2], NOW)).toEqual([NOW - 86_400_000, NOW])
  })

  it('custom 原样返回用户选择的范围', () => {
    expect(resolveHistoryRange('custom', [100, 200], NOW)).toEqual([100, 200])
  })

  it('custom 未选择时返回 null(由调用方提示)', () => {
    expect(resolveHistoryRange('custom', null, NOW)).toBeNull()
  })

  it('未知范围回退到默认 last_1h,且不会误用原型属性', () => {
    expect(resolveHistoryRange('last_99y', null, NOW)).toEqual([NOW - 3_600_000, NOW])
    expect(resolveHistoryRange('toString', null, NOW)).toEqual([NOW - 3_600_000, NOW])
  })
})

describe('查询参数构造', () => {
  it('图表查询使用 5000 条分页并携带范围与页号', () => {
    expect(buildHistoryChartQueryParams('switch_1', RANGE, 4)).toEqual({
      key: 'switch_1',
      start_time: RANGE[0],
      end_time: RANGE[1],
      page: 4,
      page_size: 5000
    })
  })

  it.each([
    ['excel', true],
    ['csv', false]
  ] as const)('导出 %s 时 export_excel=%s 且固定第一页 10000 条', (format, excel) => {
    expect(buildHistoryExportQueryParams('electricity_consumption', RANGE, format)).toEqual({
      key: 'electricity_consumption',
      start_time: RANGE[0],
      end_time: RANGE[1],
      page: 1,
      page_size: 10000,
      export_excel: excel,
      export_format: format
    })
  })

  it('分页上限为 20 页', () => {
    expect(HISTORY_CHART_MAX_PAGES_PER_SERIES).toBe(20)
  })
})
describe('响应 normalize', () => {
  it('时间戳: 秒转毫秒、毫秒保留、ISO/Date 解析、非法为 null', () => {
    expect(normalizeHistoryTimestamp(1_700_000_000)).toBe(1_700_000_000_000)
    expect(normalizeHistoryTimestamp('1700000000')).toBe(1_700_000_000_000)
    expect(normalizeHistoryTimestamp(1_700_000_000_123)).toBe(1_700_000_000_123)
    expect(normalizeHistoryTimestamp('2026-01-01T00:00:00Z')).toBe(Date.UTC(2026, 0, 1))
    expect(normalizeHistoryTimestamp(new Date(5))).toBe(5)
    expect(normalizeHistoryTimestamp('not-a-time')).toBeNull()
    expect(normalizeHistoryTimestamp(Number.NaN)).toBeNull()
    expect(normalizeHistoryTimestamp(undefined)).toBeNull()
  })

  it('值: 布尔/开关语义字符串转 1/0,数字字符串转数字,其余为 null', () => {
    expect(normalizeHistoryValue(true)).toBe(1)
    expect(normalizeHistoryValue(false)).toBe(0)
    expect(normalizeHistoryValue(' HIGH ')).toBe(1)
    expect(normalizeHistoryValue('closed')).toBe(0)
    expect(normalizeHistoryValue('12.5')).toBe(12.5)
    expect(normalizeHistoryValue(Infinity)).toBeNull()
    expect(normalizeHistoryValue('abc')).toBeNull()
    expect(normalizeHistoryValue({})).toBeNull()
  })

  it('total: 缺失/空串/负数/非数字为 null,小数向下取整', () => {
    expect(normalizeHistoryTotal({})).toBeNull()
    expect(normalizeHistoryTotal({ total: '' })).toBeNull()
    expect(normalizeHistoryTotal({ total: -1 })).toBeNull()
    expect(normalizeHistoryTotal({ total: 'x' })).toBeNull()
    expect(normalizeHistoryTotal({ total: '12.9' })).toBe(12)
    expect(normalizeHistoryTotal(null)).toBeNull()
  })

  it('空数据: null / 非对象 / 无 list 均得到空页', () => {
    for (const payload of [null, undefined, 42, 'x', {}, { list: 'nope' }]) {
      expect(normalizeHistoryPoints(payload, 'temperature_1')).toEqual({ points: [], invalidCount: 0 })
    }
  })

  it('兼容多种字段名、按时间升序并统计无效时间戳', () => {
    const page = normalizeHistoryPoints(
      [
        { x: 3_000_000_000_000, y: 3 },
        { time: 1_000_000_000_000, number_v: 1 },
        { UpdateAt: 2_000_000_000_000, bool_v: true },
        { created_at: 4_000_000_000_000, temperature_1: '4' },
        { ts: null, value: 9 },
        null
      ],
      'temperature_1'
    )
    expect(page.points).toEqual([
      { ts: 1_000_000_000_000, value: 1 },
      { ts: 2_000_000_000_000, value: 1 },
      { ts: 3_000_000_000_000, value: 3 },
      { ts: 4_000_000_000_000, value: 4 }
    ])
    expect(page.invalidCount).toBe(2)
  })

  it('无法解析的值保留为 null 点而不是丢弃时间戳', () => {
    expect(normalizeHistoryPoints({ list: [{ ts: NOW, value: 'garbage' }] }, 'switch_1').points).toEqual([
      { ts: NOW, value: null }
    ])
  })
})

describe('分页状态机 applyHistoryPage', () => {
  it('满页且 total 未满足时继续,满足 total 时停止', () => {
    const state = createHistoryPaginationState()
    expect(applyHistoryPage(state, 1, { list: pagePoints(NOW, 3), total: 5 }, 'temperature_1', SMALL_LIMITS)).toBe(true)
    expect(
      applyHistoryPage(
        state,
        2,
        { list: pagePoints(NOW + 90_000, 2, 30_000, 3), total: 5 },
        'temperature_1',
        SMALL_LIMITS
      )
    ).toBe(false)
    expect(state.points).toHaveLength(5)
    expect(state.expectedCount).toBe(5)
    expect(state.truncated).toBe(false)
  })

  it('无 total 的短页视为结束', () => {
    const state = createHistoryPaginationState()
    expect(applyHistoryPage(state, 1, pagePoints(NOW, 2), 'temperature_1', SMALL_LIMITS)).toBe(false)
    expect(state.truncated).toBe(false)
  })

  it('空页立即停止', () => {
    const state = createHistoryPaginationState()
    expect(applyHistoryPage(state, 1, { list: [], total: 10 }, 'temperature_1', SMALL_LIMITS)).toBe(false)
    expect(finalizeHistorySeriesResult('temperature_1', state)).toMatchObject({
      status: 'partial',
      missingCount: 10
    })
  })

  it('跨页边界重复点被去重', () => {
    const state = createHistoryPaginationState()
    applyHistoryPage(state, 1, { list: pagePoints(NOW, 3), total: 4 }, 'temperature_1', SMALL_LIMITS)
    applyHistoryPage(
      state,
      2,
      { list: pagePoints(NOW + 60_000, 2, 30_000, 2), total: 4 },
      'temperature_1',
      SMALL_LIMITS
    )
    expect(state.points.map((point) => point.value)).toEqual([0, 1, 2, 3])
  })

  it('无 total 时整页重复无法证明结束,标记 truncated', () => {
    const state = createHistoryPaginationState()
    const full = pagePoints(NOW, 3)
    expect(applyHistoryPage(state, 1, full, 'temperature_1', SMALL_LIMITS)).toBe(true)
    expect(applyHistoryPage(state, 2, full, 'temperature_1', SMALL_LIMITS)).toBe(false)
    expect(state.truncated).toBe(true)
  })

  it('达到点数上限时截断并在 total 更大时标记 truncated', () => {
    const state = createHistoryPaginationState()
    applyHistoryPage(state, 1, { list: pagePoints(NOW, 3), total: 50 }, 'temperature_1', SMALL_LIMITS)
    applyHistoryPage(
      state,
      2,
      { list: pagePoints(NOW + 90_000, 3, 30_000, 3), total: 50 },
      'temperature_1',
      SMALL_LIMITS
    )
    expect(
      applyHistoryPage(
        state,
        3,
        { list: pagePoints(NOW + 180_000, 3, 30_000, 6), total: 50 },
        'temperature_1',
        SMALL_LIMITS
      )
    ).toBe(false)
    expect(state.points).toHaveLength(7)
    expect(state.truncated).toBe(true)
  })

  it('最后允许的一页仍为满页且无 total 时标记 truncated', () => {
    const limits = { pageSize: 2, maxPoints: 100, maxPages: 2 }
    const state = createHistoryPaginationState()
    expect(applyHistoryPage(state, 1, pagePoints(NOW, 2), 'temperature_1', limits)).toBe(true)
    expect(applyHistoryPage(state, 2, pagePoints(NOW + 60_000, 2, 30_000, 2), 'temperature_1', limits)).toBe(false)
    expect(state.truncated).toBe(true)
  })
})
describe('fetchHistorySeriesPages - 注入 fetcher 的分页拉取', () => {
  it('空数据: 首页成功但为空时状态为 empty', async () => {
    const fetchPage = scriptedFetcher({ 1: { error: null, data: { list: [], total: 0 } } })
    const result = await fetchHistorySeriesPages(fetchPage, 'electricity_consumption', RANGE, SMALL_LIMITS)
    expect(result).toMatchObject({ status: 'empty', loadedCount: 0, failedPage: undefined, truncated: false })
    expect(fetchPage).toHaveBeenCalledTimes(1)
  })

  it('首页返回 error 时为 failed', async () => {
    const result = await fetchHistorySeriesPages(
      scriptedFetcher({ 1: { error: new Error('500'), data: null } }),
      'temperature_1',
      RANGE,
      SMALL_LIMITS
    )
    expect(result).toMatchObject({ status: 'failed', failedPage: 1, loadedCount: 0 })
  })

  it('首页抛异常时为 failed 而不是向上抛出', async () => {
    const result = await fetchHistorySeriesPages(
      scriptedFetcher({ 1: new Error('network') }),
      'temperature_1',
      RANGE,
      SMALL_LIMITS
    )
    expect(result).toMatchObject({ status: 'failed', failedPage: 1 })
  })

  it('部分分页失败: 保留已成功页数据并标记 partial + failedPage', async () => {
    const fetchPage = scriptedFetcher({
      1: { error: null, data: { list: pagePoints(NOW, 3), total: 6 } },
      2: new Error('timeout')
    })
    const result = await fetchHistorySeriesPages(fetchPage, 'temperature_1', RANGE, SMALL_LIMITS)
    expect(result).toMatchObject({
      status: 'partial',
      failedPage: 2,
      loadedCount: 3,
      expectedCount: 6,
      missingCount: 3
    })
    expect(fetchPage).toHaveBeenCalledTimes(2)
  })

  it('按页号递增请求并携带范围与页大小', async () => {
    const fetchPage = scriptedFetcher({
      1: { error: null, data: { list: pagePoints(NOW, 3), total: 4 } },
      2: { error: null, data: { list: pagePoints(NOW + 90_000, 1, 30_000, 3), total: 4 } }
    })
    const result = await fetchHistorySeriesPages(fetchPage, 'switch_2', RANGE, SMALL_LIMITS)
    expect(fetchPage.mock.calls.map(([params]) => params)).toEqual([
      { key: 'switch_2', start_time: RANGE[0], end_time: RANGE[1], page: 1, page_size: 3 },
      { key: 'switch_2', start_time: RANGE[0], end_time: RANGE[1], page: 2, page_size: 3 }
    ])
    expect(result).toMatchObject({ status: 'loaded', loadedCount: 4, missingCount: 0 })
  })

  it('页数上限之后不再请求', async () => {
    const pages: Record<number, HistoryPageResponse> = {}
    for (let page = 1; page <= 5; page += 1) {
      pages[page] = { error: null, data: pagePoints(NOW + page * 1_000_000, 3) }
    }
    const fetchPage = scriptedFetcher(pages)
    const result = await fetchHistorySeriesPages(fetchPage, 'temperature_1', RANGE, { ...SMALL_LIMITS, maxPoints: 100 })
    expect(fetchPage).toHaveBeenCalledTimes(3)
    expect(result).toMatchObject({ status: 'partial', truncated: true, loadedCount: 9 })
  })

  it('fetchHistoryChartData 按定义顺序拉取并汇总 chartData', async () => {
    const fetchPage = vi.fn(async (params: RDIHistoryParams) => ({
      error: null,
      data: { list: [{ ts: NOW, value: params.key === 'switch_1' ? 'on' : 2 }], total: 1 }
    }))
    const { chartData, seriesResults } = await fetchHistoryChartData(
      fetchPage,
      RANGE,
      ['switch_1', 'temperature_1'],
      SMALL_LIMITS
    )
    expect(seriesResults.map((item) => item.key)).toEqual(['temperature_1', 'switch_1'])
    expect(chartData).toEqual({ temperature_1: [{ ts: NOW, value: 2 }], switch_1: [{ ts: NOW, value: 1 }] })
  })
})

describe('insertHistoryGapMarkers - 缺口检测', () => {
  it('空输入没有缺口', () => {
    expect(insertHistoryGapMarkers([])).toEqual({ points: [], detectedGapCount: 0 })
  })

  it('间隔恰好等于阈值不算缺口,超过阈值插入中点 null 标记', () => {
    const exact = insertHistoryGapMarkers([
      { ts: 0, value: 1 },
      { ts: HISTORY_GAP_THRESHOLD_MS, value: 2 }
    ])
    expect(exact.detectedGapCount).toBe(0)

    const gapped = insertHistoryGapMarkers([
      { ts: 0, value: 1 },
      { ts: HISTORY_GAP_THRESHOLD_MS + 2, value: 2 }
    ])
    expect(gapped.points).toEqual([
      { ts: 0, value: 1 },
      { ts: HISTORY_GAP_THRESHOLD_MS / 2 + 1, value: null },
      { ts: HISTORY_GAP_THRESHOLD_MS + 2, value: 2 }
    ])
    expect(gapped.detectedGapCount).toBe(1)
  })

  it('原生 null 计为缺口且相邻处不重复插入标记', () => {
    const result = insertHistoryGapMarkers([
      { ts: 0, value: null },
      { ts: 500_000, value: 1 },
      { ts: 1_000_000, value: null }
    ])
    expect(result.points).toHaveLength(3)
    expect(result.detectedGapCount).toBe(2)
  })

  it('乱序输入先排序;阈值可注入', () => {
    const result = insertHistoryGapMarkers(
      [
        { ts: 20, value: 2 },
        { ts: 0, value: 1 }
      ],
      10
    )
    expect(result.points.map((point) => point.ts)).toEqual([0, 10, 20])
    expect(result.detectedGapCount).toBe(1)
  })
})
describe('calculateEnergyStats - 能耗统计', () => {
  it('空数据与只有缺口标记时返回空统计', () => {
    const empty = { sample_count: 0, latest: null, min: null, max: null, delta: null }
    expect(calculateEnergyStats()).toEqual(empty)
    expect(calculateEnergyStats([{ ts: 1, value: null }])).toEqual(empty)
  })

  it('单点时 delta 为 0', () => {
    expect(calculateEnergyStats([{ ts: 1, value: 5 }])).toEqual({
      sample_count: 1,
      latest: 5,
      min: 5,
      max: 5,
      delta: 0
    })
  })

  it('乱序输入按时间计算最新值与增量,并忽略 null', () => {
    expect(
      calculateEnergyStats([
        { ts: 30, value: 12 },
        { ts: 10, value: 10 },
        { ts: 20, value: null },
        { ts: 25, value: 15 }
      ])
    ).toEqual({ sample_count: 3, latest: 12, min: 10, max: 15, delta: 2 })
  })

  it('计数器回绕时 delta 不为负', () => {
    expect(
      calculateEnergyStats([
        { ts: 1, value: 100 },
        { ts: 2, value: 3 }
      ]).delta
    ).toBe(0)
  })

  it('20 万点不触发调用栈上限', () => {
    const points: HistoryPoint[] = Array.from({ length: 200_000 }, (_, index) => ({ ts: index, value: index }))
    expect(calculateEnergyStats(points)).toMatchObject({ sample_count: 200_000, min: 0, max: 199_999 })
  })
})

describe('序列选择与结果汇总', () => {
  it('空选择回退到全部默认序列;非空选择过滤非法值并去重', () => {
    expect(normalizeHistoryChartSeriesKeys([])).toEqual([...DEFAULT_HISTORY_CHART_SERIES_KEYS])
    expect(normalizeHistoryChartSeriesKeys(['switch_1', 'bogus' as never, 'switch_1'])).toEqual(['switch_1'])
  })

  it('summarize 区分 failed / partial / gapped 并判断能耗统计可用性', () => {
    const summary = summarizeHistorySeriesResults([
      seriesResult({ key: 'temperature_1', status: 'failed' }),
      seriesResult({ key: 'switch_1', status: 'partial', detectedGapCount: 2 }),
      seriesResult({ key: 'electricity_consumption', status: 'empty' })
    ])
    expect(summary).toEqual({
      failedLabels: ['T1'],
      partialLabels: ['SW1'],
      gappedLabels: ['SW1'],
      hasFailures: true,
      hasSuccessfulData: true,
      energyStatisticsAvailable: true
    })
  })

  it('能耗序列失败、缺失或只有缺口标记时统计不可用', () => {
    expect(summarizeHistorySeriesResults([]).energyStatisticsAvailable).toBe(false)
    expect(
      summarizeHistorySeriesResults([seriesResult({ key: 'electricity_consumption', status: 'failed' })])
        .energyStatisticsAvailable
    ).toBe(false)
    expect(
      summarizeHistorySeriesResults([
        seriesResult({ key: 'electricity_consumption', status: 'partial', points: [{ ts: 1, value: null }] })
      ]).energyStatisticsAvailable
    ).toBe(false)
  })

  it('全部失败时 hasSuccessfulData 为 false', () => {
    expect(summarizeHistorySeriesResults([seriesResult({ status: 'failed' })]).hasSuccessfulData).toBe(false)
  })

  it('图表数据只有 null 标记时视为无可渲染数据', () => {
    const results = [seriesResult({ key: 'temperature_2', points: [{ ts: 1, value: null }] })]
    expect(hasRenderableHistoryChartData(buildHistoryChartData(results))).toBe(false)
    expect(hasRenderableHistoryChartData({ temperature_2: [{ ts: 1, value: 0 }] })).toBe(true)
    expect(hasRenderableHistoryChartData({})).toBe(false)
  })
})

describe('导出', () => {
  it('导出行保留原始 ts 并过滤缺失时间戳', () => {
    expect(normalizeHistoryExportRows({ list: [{ ts: 1, value: 2 }, { x: '2026', y: 'on' }, { value: 3 }] })).toEqual([
      { ts: 1, value: 2 },
      { ts: '2026', value: 'on' }
    ])
    expect(normalizeHistoryExportRows(null)).toEqual([])
  })

  it('华氏度导出温度列换算并改表头,非温度列原样', () => {
    expect(buildHistoryExportCsvRows('temperature_1', [{ ts: 0, value: '100' }], 'F')).toEqual([
      ['time', 'key', 'value (F)'],
      ['1970-01-01T00:00:00.000Z', 'temperature_1', '212.00']
    ])
    expect(buildHistoryExportCsvRows('switch_1', [{ ts: 'raw', value: 'on' }], 'F')).toEqual([
      ['time', 'key', 'value'],
      ['raw', 'switch_1', 'on']
    ])
  })

  it('CSV 转义双引号并把 null/undefined 写成空串', () => {
    expect(
      buildCsvContent([
        ['a"b', null],
        [undefined, 1]
      ])
    ).toBe('"a""b",""\n"","1"')
  })

  it('优先使用服务端导出文件路径并拼接到服务根地址', () => {
    expect(getExportedHistoryFilePath({ file_path: 'files/a.xlsx' })).toBe('files/a.xlsx')
    expect(getExportedHistoryFilePath({ filePath: 'files/b.xlsx', file_path: 'x' })).toBe('files/b.xlsx')
    expect(getExportedHistoryFilePath(null)).toBeUndefined()
    expect(buildExportFileUrl('http://h:8080/api/v1', 'files/a.xlsx')).toBe('http://h:8080/files/a.xlsx')
  })
})

describe('buildHistoryChartOptions', () => {
  const t = (key: string) => `t:${key}`

  it('只为存在的序列生成配置,开关量走右轴并保留 null 断线', () => {
    const options = buildHistoryChartOptions(
      t as never,
      {
        switch_1: [
          { ts: 1, value: 1 },
          { ts: 2, value: null }
        ],
        temperature_1: [{ ts: 1, value: 10 }]
      },
      (key, value) => (key === 'temperature_1' ? value * 2 : value)
    ) as { color: string[]; series: Array<Record<string, unknown>> }

    expect(options.color).toEqual(['#f43f5e', '#7c3aed'])
    expect(options.series.map((item) => item.name)).toEqual(['T1', 'SW1'])
    expect(options.series[0]).toMatchObject({ yAxisIndex: 0, smooth: true, data: [[1, 20]], connectNulls: false })
    expect(options.series[1]).toMatchObject({
      yAxisIndex: 1,
      step: 'middle',
      data: [
        [1, 1],
        [2, null]
      ]
    })
  })

  it('空 chartData 生成空 series', () => {
    const options = buildHistoryChartOptions(t as never, {}, (_key, value) => value) as { series: unknown[] }
    expect(options.series).toEqual([])
  })
})
