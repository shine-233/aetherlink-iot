/**
 * 文件用途：验证增强网格工具函数在布局、碰撞、历史和错误边界上的行为。
 * 核心逻辑：构造网格项和配置输入，断言工具函数返回的布局结果、错误处理和副作用。
 * 关键注意事项：测试应覆盖异常输入和边界尺寸，避免只验证常规成功路径。
 * 重构建议：可按工具模块拆分测试文件，让布局算法、校验和导入导出各自拥有独立用例。
 */
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { GridLayoutPlusItem } from './gridLayoutPlusTypes'
import {
  cloneGridItem,
  cloneLayout,
  compactLayout,
  createResponsiveLayout,
  debounce,
  exportLayout,
  filterLayout,
  findAvailablePosition,
  generateId,
  getItemAtBreakpoint,
  getLayoutBounds,
  getLayoutStats,
  importLayout,
  isItemsOverlapping,
  optimizeLayoutPerformance,
  searchLayout,
  sortLayout,
  throttle,
  transformLayoutForBreakpoint,
  validateGridItem,
  validateLayout
} from './gridLayoutPlusUtils'

const item = (overrides: Partial<GridLayoutPlusItem> = {}): GridLayoutPlusItem => ({
  i: 'card-a',
  x: 0,
  y: 0,
  w: 2,
  h: 2,
  type: 'chart',
  title: 'Temperature Card',
  props: { color: 'red', nested: { unit: 'C' } },
  data: { value: 22 },
  style: { width: '100%' },
  metadata: { source: { id: 'device-001' } },
  ...overrides
})

describe('gridLayoutPlusUtils', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('validates grid items and layout uniqueness', () => {
    expect(validateGridItem(item())).toMatchObject({ success: true, data: true })
    expect(validateGridItem(item({ i: '' }))).toMatchObject({ success: false })
    expect(validateGridItem(item({ x: -1 }))).toMatchObject({ success: false })
    expect(validateGridItem(item({ w: 0 }))).toMatchObject({ success: false })
    expect(validateGridItem(item({ w: 1, minW: 2 }))).toMatchObject({ success: false })
    expect(validateGridItem(item({ h: 5, maxH: 4 }))).toMatchObject({ success: false })

    expect(validateLayout([item(), item({ i: 'card-b', x: 3 })])).toMatchObject({ success: true })
    expect(validateLayout([item(), item()])).toMatchObject({
      success: false,
      message: '发现重复ID'
    })
  })

  it('detects collisions and finds the first free grid position', () => {
    const placed = [item({ i: 'left', x: 0, y: 0, w: 2, h: 2 }), item({ i: 'right', x: 2, y: 0, w: 2, h: 2 })]

    expect(isItemsOverlapping(placed[0], placed[1])).toBe(false)
    expect(isItemsOverlapping(placed[0], item({ i: 'overlap', x: 1, y: 1 }))).toBe(true)
    expect(findAvailablePosition(2, 2, placed, 4)).toEqual({ x: 0, y: 2 })
  })

  it('clones layout data without sharing nested business payload objects', () => {
    const source = item()
    const cloned = cloneGridItem(source)

    expect(cloned).toEqual(source)
    expect(cloned.props).not.toBe(source.props)
    expect(cloned.data).not.toBe(source.data)
    expect(cloned.metadata).not.toBe(source.metadata)

    const clonedLayout = cloneLayout([source])
    expect(clonedLayout).toHaveLength(1)
    expect(clonedLayout[0]).not.toBe(source)
  })

  it('computes bounds, compacts layout, sorts, filters, searches, and reports stats', () => {
    const layout = [
      item({ i: 'b', x: 3, y: 2, w: 2, h: 1, title: 'Humidity Card', type: 'table' }),
      item({ i: 'a', x: 0, y: 3, w: 3, h: 2, title: 'Temperature Card', type: 'chart' }),
      item({ i: 'c', x: 0, y: 0, w: 1, h: 1, title: 'Status Card', type: 'status' })
    ]

    expect(getLayoutBounds([])).toEqual({ minX: 0, minY: 0, maxX: 0, maxY: 0, width: 0, height: 0 })
    expect(getLayoutBounds(layout)).toMatchObject({ minX: 0, minY: 0, maxX: 5, maxY: 5, width: 5, height: 5 })
    expect(compactLayout(layout).map(card => [card.i, card.y])).toEqual([
      ['c', 0],
      ['b', 0],
      ['a', 1]
    ])
    expect(sortLayout(layout, 'id').map(card => card.i)).toEqual(['a', 'b', 'c'])
    expect(sortLayout(layout, 'size').map(card => card.i)).toEqual(['a', 'b', 'c'])
    expect(filterLayout(layout, card => card.type === 'chart').map(card => card.i)).toEqual(['a'])
    expect(searchLayout(layout, 'humidity').map(card => card.i)).toEqual(['b'])

    expect(getLayoutStats(layout)).toMatchObject({
      totalItems: 3,
      totalRows: 5,
      totalCells: 25,
      usedCells: 9,
      utilization: 36,
      largestItem: expect.objectContaining({ i: 'a' }),
      smallestItem: expect.objectContaining({ i: 'c' }),
      averageSize: 3
    })
  })

  it('transforms layouts for responsive breakpoints', () => {
    const base = [item({ i: 'wide', x: 6, w: 6 }), item({ i: 'narrow', x: 0, w: 3 })]

    expect(transformLayoutForBreakpoint(base, 12, 6)).toEqual([
      expect.objectContaining({ i: 'wide', x: 3, w: 3 }),
      expect.objectContaining({ i: 'narrow', x: 0, w: 2 })
    ])
    expect(transformLayoutForBreakpoint(base, 12, 12)).not.toBe(base)
    const responsive = createResponsiveLayout(base, { lg: 1200, sm: 768 }, { lg: 12, sm: 6 })
    expect(responsive.lg).toHaveLength(2)
    expect(responsive.sm).toHaveLength(2)
    expect(responsive.lg?.find(card => card.i === 'wide')).toMatchObject({ x: 6, w: 6 })
    expect(responsive.sm?.find(card => card.i === 'wide')).toMatchObject({ x: 3, w: 3 })
  })

  it('exports and imports layout data in json and csv formats', () => {
    const layout = [item({ i: 'csv-card', x: 1, y: 2, w: 3, h: 4, title: 'CSV Card', type: 'chart' })]

    expect(importLayout(exportLayout(layout, 'json'), 'json')).toEqual(layout)
    expect(exportLayout(layout, 'csv')).toContain('i,x,y,w,h,type,title')
    expect(importLayout(exportLayout(layout, 'csv'), 'csv')).toEqual([
      expect.objectContaining({ i: 'csv-card', x: 1, y: 2, w: 3, h: 4, type: 'chart', title: 'CSV Card' })
    ])
    expect(importLayout('not json', 'json')).toEqual([])
  })

  it('returns responsive item overrides and leaves untouched layouts unchanged', () => {
    const base = item({ i: 'card-a', x: 0, w: 4 })
    const sm = item({ i: 'card-a', x: 1, w: 2 })

    expect(getItemAtBreakpoint(base, 'sm', { sm: [sm] })).toBe(sm)
    expect(getItemAtBreakpoint(base, 'md', { sm: [sm] })).toBe(base)
    expect(
      optimizeLayoutPerformance([base], {
        debounceDelay: 100,
        throttleDelay: 100,
        enableLazyLoading: true,
        lazyLoadingBuffer: 2,
        enableVirtualization: true,
        virtualizationThreshold: 0
      })
    ).toEqual([base])
  })

  it('generates ids and controls delayed layout callbacks', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-27T00:00:00.000Z'))
    vi.spyOn(Math, 'random').mockReturnValue(0.123456789)

    expect(generateId('card')).toMatch(/^card-1782518400000-/)

    const debounced = vi.fn()
    const throttled = vi.fn()
    const debouncedFn = debounce(debounced, 100)
    const throttledFn = throttle(throttled, 100)

    debouncedFn('first')
    debouncedFn('second')
    expect(debounced).toHaveBeenCalledTimes(0)
    vi.advanceTimersByTime(100)
    expect(debounced).toHaveBeenCalledOnce()
    expect(debounced).toHaveBeenCalledWith('second')

    throttledFn('first')
    throttledFn('second')
    expect(throttled).toHaveBeenCalledOnce()
    expect(throttled).toHaveBeenCalledWith('first')
    vi.advanceTimersByTime(100)
    throttledFn('third')
    expect(throttled).toHaveBeenCalledTimes(2)
    expect(throttled).toHaveBeenLastCalledWith('third')
  })
})
