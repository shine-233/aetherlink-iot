/**
 * 文件用途: 表达 Fleet 批量选择的两种语义（仅当页勾选 / 全部匹配筛选结果）并计算真实作用范围。
 * 核心逻辑: 全量选择走 device_filter 交接，作用台数受安全上限截断，截断量必须如实呈现给操作员。
 * 关键注意事项: 上限默认值必须与 Command Center 的 DEFAULT_FILTER_JOB_MAX_DEVICES 保持一致，否则 UI 会谎报。
 */

export type FleetSelectionMode = 'current_page' | 'all_matching'

/** 与 Command Center DEFAULT_FILTER_JOB_MAX_DEVICES 及后端 defaultFleetCommandDeviceFilterMaxDevices 对齐。 */
export const FLEET_SELECT_ALL_DEFAULT_MAX_DEVICES = 200
/** 与后端 maxFleetCommandDeviceFilterMaxDevices 对齐。 */
export const FLEET_SELECT_ALL_HARD_MAX_DEVICES = 1000

export type FleetSelectionScope = {
  mode: FleetSelectionMode
  /** 当前筛选条件匹配到的设备总数（来自设备列表 total）。 */
  matchedTotal: number
  /** 当前页加载的设备数。 */
  currentPageCount: number
  /** 当页真正被勾选的行数。 */
  checkedCount: number
  /** 下一个批量动作实际会作用到的设备数。 */
  effectiveCount: number
  maxDevices: number
  /** 匹配总数超过安全上限时为 true。 */
  truncated: boolean
  /** 因为上限而不会被作用到的设备数。 */
  skippedCount: number
}

export type FleetSelectionScopeMessage = {
  key: string
  params: Record<string, number>
}

export function normalizeFleetSelectAllMaxDevices(value?: number | null) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    return FLEET_SELECT_ALL_DEFAULT_MAX_DEVICES
  }
  return Math.min(Math.floor(value), FLEET_SELECT_ALL_HARD_MAX_DEVICES)
}

export function canSelectAllMatchingFleetDevices(matchedTotal?: number | null) {
  return typeof matchedTotal === 'number' && Number.isFinite(matchedTotal) && matchedTotal > 0
}

export function buildFleetSelectionScope(input: {
  mode: FleetSelectionMode
  matchedTotal?: number | null
  currentPageCount?: number | null
  checkedCount?: number | null
  maxDevices?: number | null
}): FleetSelectionScope {
  const matchedTotal = canSelectAllMatchingFleetDevices(input.matchedTotal) ? Number(input.matchedTotal) : 0
  const currentPageCount = Math.max(Number(input.currentPageCount) || 0, 0)
  const checkedCount = Math.max(Number(input.checkedCount) || 0, 0)
  const maxDevices = normalizeFleetSelectAllMaxDevices(input.maxDevices)

  if (input.mode !== 'all_matching') {
    return {
      mode: 'current_page',
      matchedTotal,
      currentPageCount,
      checkedCount,
      effectiveCount: checkedCount,
      maxDevices,
      truncated: false,
      skippedCount: 0
    }
  }

  const effectiveCount = Math.min(matchedTotal, maxDevices)

  return {
    mode: 'all_matching',
    matchedTotal,
    currentPageCount,
    checkedCount,
    effectiveCount,
    maxDevices,
    truncated: matchedTotal > maxDevices,
    skippedCount: Math.max(matchedTotal - maxDevices, 0)
  }
}

/**
 * 输出用于展示的 i18n key 与参数：当页语义、全量语义、以及全量被上限截断三种情况分别有独立文案，
 * 避免操作员把「当页勾选」误当成「全部匹配」。
 */
export function formatFleetSelectionScopeText(template: string, params: Record<string, number>) {
  return Object.entries(params).reduce(
    (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)),
    template
  )
}

export function buildFleetSelectionScopeMessage(scope: FleetSelectionScope): FleetSelectionScopeMessage {
  if (scope.mode !== 'all_matching') {
    return {
      key: 'custom.devicePage.fleetSelectionScopeCurrentPage',
      params: {
        checked: scope.checkedCount,
        page: scope.currentPageCount,
        matched: scope.matchedTotal
      }
    }
  }

  if (scope.truncated) {
    return {
      key: 'custom.devicePage.fleetSelectionScopeCapped',
      params: {
        matched: scope.matchedTotal,
        effective: scope.effectiveCount,
        max: scope.maxDevices,
        skipped: scope.skippedCount
      }
    }
  }

  return {
    key: 'custom.devicePage.fleetSelectionScopeAllMatching',
    params: {
      matched: scope.matchedTotal,
      effective: scope.effectiveCount,
      max: scope.maxDevices
    }
  }
}
