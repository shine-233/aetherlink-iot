/**
 * 文件用途: Twin Lite 卡片的纯函数视图模型。
 * 核心逻辑: 从 TwinLiteState 推导漂移/缺失/命令行分组、修复摘要文本、指引条目、收敛确认提示和行元数据文本，
 * 供 useTwinLite 与各展示子组件共享，替代原 TwinLiteCard.vue 内联的 computed 规则。
 * 关键注意事项: 文案 key 与原组件保持一致；这里不访问接口、不读 window。
 */
import {
  getTwinLiteConvergenceStatus,
  normalizeTwinLiteConvergenceStatus,
  type TwinLiteConvergenceStatus,
  type TwinLiteRow,
  type TwinLiteState
} from './twin-lite-normalizer'

type Translate = (key: string) => string
type AlertType = 'warning' | 'info' | 'success'

const DETAILS = 'custom.device_details'

export function formatTwinValue(value: unknown) {
  if (value === null || value === undefined || value === '') return '--'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

/** 编辑器文本序列化：字符串原样，其它值 2 空格 JSON。 */
export function serializeTwinValue(value: unknown) {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

/** 期望值输入：空值报错；合法 JSON 按 JSON 解析，否则按原始字符串提交。 */
export function parseTwinDesiredInput(value: string, emptyMessage: string): unknown {
  const trimmed = value.trim()
  if (!trimmed) throw new Error(emptyMessage)
  try {
    return JSON.parse(trimmed)
  } catch {
    return trimmed
  }
}

export function isTwinStatePayload(payload: unknown): payload is TwinLiteState {
  if (!payload || typeof payload !== 'object') return false
  const candidate = payload as TwinLiteState
  return Array.isArray(candidate.rows) && typeof candidate.summary === 'object' && candidate.summary !== null
}

export function groupTwinRows(rows: TwinLiteRow[]) {
  const drift = rows.filter((row) => row.comparable && !row.matched)
  return {
    drift,
    unavailable: rows.filter((row) => row.comparable && row.reported === null),
    repairable: drift.filter((row) => row.source !== 'command' && row.reported !== null && row.reported !== undefined),
    command: rows.filter((row) => row.source === 'command')
  }
}

export type TwinRowGroups = ReturnType<typeof groupTwinRows>

export function twinRepairAlertType(groups: TwinRowGroups): AlertType {
  if (groups.drift.length) return 'warning'
  if (groups.unavailable.length) return 'info'
  return 'success'
}

export function twinRepairHeadlineKey(groups: TwinRowGroups) {
  if (groups.drift.length) return `${DETAILS}.twinRepairNeedsAction`
  if (groups.unavailable.length) return `${DETAILS}.twinRepairWaitingReported`
  return `${DETAILS}.twinRepairAllGood`
}

export function buildTwinRepairSummary(
  deviceId: string,
  state: TwinLiteState,
  drift: TwinLiteRow[],
  sourceLabels: Record<string, string>,
  t: Translate
) {
  return [
    `Device: ${deviceId}`,
    `${t(`${DETAILS}.twinMatched`)}: ${state.summary.matchedCount}`,
    `${t(`${DETAILS}.twinDelta`)}: ${state.summary.deltaCount}`,
    `${t(`${DETAILS}.twinUnavailable`)}: ${state.summary.unavailableCount}`,
    '',
    'Delta rows:',
    ...(drift.length
      ? drift.map(
          (row, index) =>
            `${index + 1}. [${sourceLabels[row.source] || row.source}] ${row.label || row.key}: desired=${formatTwinValue(
              row.desired
            )}; reported=${formatTwinValue(row.reported)}`
        )
      : ['--'])
  ].join('\n')
}

export function buildTwinGuidanceItems(groups: TwinRowGroups, totalRows: number, t: Translate) {
  const items: Array<{ type: AlertType; label: string; text: string }> = []
  const push = (type: AlertType, name: string, count?: number) =>
    items.push({
      type,
      label: t(`${DETAILS}.twinGuidance${name}Label`),
      text:
        count === undefined
          ? t(`${DETAILS}.twinGuidance${name}Text`)
          : t(`${DETAILS}.twinGuidance${name}Text`).replace('{count}', String(count))
    })

  if (groups.drift.length) push('warning', 'Drift', groups.drift.length)
  if (groups.unavailable.length) push('info', 'Unavailable', groups.unavailable.length)
  if (groups.command.length) push('info', 'Command', groups.command.length)
  if (!items.length && totalRows) push('success', 'Matched')
  return items
}

export function resolveTwinConvergenceStatus(state: TwinLiteState): TwinLiteConvergenceStatus {
  const { summary } = state
  const derived = getTwinLiteConvergenceStatus(summary.desiredCount, summary.deltaCount, summary.unavailableCount)
  return normalizeTwinLiteConvergenceStatus(summary.convergenceStatus, derived)
}

export function twinConvergenceAlertType(status: TwinLiteConvergenceStatus): AlertType {
  if (status === 'ready') return 'success'
  if (status === 'waiting_reported' || status === 'no_desired') return 'info'
  return 'warning'
}

export function twinConfirmationBoundaryKey(state: TwinLiteState) {
  return state.summary.evidenceBoundary === 'platform_visible_evidence_only'
    ? `${DETAILS}.twinConfirmationBoundaryPlatform`
    : `${DETAILS}.twinConfirmationBoundaryDefault`
}

function formatTwinTimestamp(value?: string) {
  if (!value) return ''
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}

export function twinMetadataLines(row: TwinLiteRow, t: Translate) {
  const lines: string[] = []
  const desiredUpdatedAt = formatTwinTimestamp(row.desired_updated_at)
  const desiredExpiresAt = formatTwinTimestamp(row.desired_expires_at)
  const reportedAt = formatTwinTimestamp(row.reported_at)
  if (desiredUpdatedAt) lines.push(`${t(`${DETAILS}.twinDesiredUpdatedAt`)}: ${desiredUpdatedAt}`)
  if (desiredExpiresAt) lines.push(`${t(`${DETAILS}.twinDesiredExpiresAt`)}: ${desiredExpiresAt}`)
  if (reportedAt) lines.push(`${t(`${DETAILS}.twinReportedAt`)}: ${reportedAt}`)
  if (row.desired_revision) lines.push(`${t(`${DETAILS}.twinDesiredRevision`)}: ${row.desired_revision}`)
  if (row.last_write_source) {
    lines.push(
      `${t(`${DETAILS}.twinLastWriteSource`)}: ${t(`${DETAILS}.twinLastWriteSource.${row.last_write_source}`)}`
    )
  }
  return lines
}

/** 最近一次保存期望值后的观察结果：未找到行 / 不可比较 / 尚无上报为 info，其余按是否匹配。 */
export function desiredObservationType(row: TwinLiteRow | null): AlertType {
  if (!row) return 'info'
  if (!row.comparable || row.reported === null || row.reported === undefined) return 'info'
  return row.matched ? 'success' : 'warning'
}
