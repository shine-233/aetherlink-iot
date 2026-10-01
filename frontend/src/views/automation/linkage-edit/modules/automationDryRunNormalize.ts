/**
 * 联动预演响应的唯一归一化边界。
 *
 * 后端契约（backend/internal/model/scene_automations.http.go SceneAutomationDryRunResult）
 * 只输出 snake_case 字段。历史上各个视图 builder 各自猜测 camelCase / `blockers` 等变体，
 * 字段漂移因此被静默掩盖。现在所有变体兼容都收敛在本文件：
 *   - snake_case 永远是主读取路径；
 *   - 旧版 camelCase / `blockers` 兜底只出现在这里，每处都注明原因；
 *   - 下游 builder 只接受 NormalizedDryRun，不再触碰原始键名。
 */
import type { SceneAutomationDryRunResult, SceneAutomationDryRunStats } from '@/service/api/automation'

/** 后端 SceneAutomationDryRunDiagnostic 的宽松线上形态 */
export interface DryRunWireDiagnostic {
  severity?: string | null
  scope?: string | null
  message?: string | null
}

/** 后端 SceneAutomationDryRunTraceStep 的宽松线上形态 */
export interface DryRunWireTraceStep {
  index?: unknown
  phase?: string | null
  status?: string | null
  label?: string | null
  kind?: string | null
  target?: string | null
  detail?: string | null
  notes?: unknown[] | null
  [key: string]: unknown
}

/** 后端 SceneAutomationDryRunTrace 的宽松线上形态 */
export interface DryRunWireTrace {
  steps?: DryRunWireTraceStep[] | null
  step_count?: number | null
  evaluated_at?: string | null
  /** 旧版 camelCase 兜底：早期前端 mock 使用过 evaluatedAt */
  evaluatedAt?: string | null
  explanation?: string | null
  is_simulation?: boolean | null
  [key: string]: unknown
}

/**
 * 预演接口原始响应。snake_case 字段来自后端契约；camelCase 字段仅为兼容旧 mock / 旧网关，
 * 只允许 normalizeDryRunResponse 读取。
 */
export type DryRunWireResult = SceneAutomationDryRunResult & {
  execution_trace?: DryRunWireTrace | null
  /** 旧版 camelCase 兜底 */
  executionTrace?: DryRunWireTrace | null
  /** 旧版 camelCase 兜底 */
  nextSteps?: string[] | null
}

export type DryRunDiagnosticSeverity = 'success' | 'info' | 'warning' | 'error'

export interface NormalizedDryRunDiagnostic {
  severity: DryRunDiagnosticSeverity
  /** 后端原始 severity，仅用于严格区分 error/warning（未知值不会被当作 error） */
  rawSeverity: string
  scope: string
  message: string
}

export interface NormalizedDryRunTraceStep {
  /** 后端给出的 index；缺失时为 null，由视图层用位置补齐 */
  index: number | null
  phase: string
  status: string
  label: string
  kind: string
  target: string
  detail: string
  notes: string[]
}

export interface NormalizedDryRunTrace {
  steps: NormalizedDryRunTraceStep[]
  stepCount: number
  evaluatedAt: string
  explanation: string
  isSimulation: boolean
}

export interface NormalizedDryRunStats {
  conditionGroupCount: number
  conditionCount: number
  actionCount: number
  conditionTypes: Record<string, number>
  actionTypes: Record<string, number>
  targetKinds: Record<string, number>
}

export interface NormalizedDryRun {
  /** true 表示确实存在后端响应；null/undefined 输入归一化为 present=false 的空结构 */
  present: boolean
  supported: boolean | null
  valid: boolean | null
  /** can_save 优先；后端未给出时退回 valid；都没有则为 null（未知） */
  canSave: boolean | null
  summary: string
  stats: NormalizedDryRunStats
  /** 资源引用计数，后端缺失时退回 dry_run.target_kinds */
  referenceCounts: Record<string, number>
  /** 去重后的阻断原因：blocking_errors + errors + error 级诊断 */
  blockingErrors: string[]
  /** 去重后的提醒：warnings + warning 级诊断 */
  warnings: string[]
  /** 原始 errors 数组（诊断列表为空时的回退源） */
  errors: string[]
  /** 原始 warnings 数组（诊断列表为空时的回退源） */
  rawWarnings: string[]
  skippedConditions: string[]
  unavailableActions: string[]
  matchedDevices: number | null
  diagnostics: NormalizedDryRunDiagnostic[]
  nextSteps: string[]
  /** 后端未返回 execution_trace 时为 null */
  trace: NormalizedDryRunTrace | null
}

/** 已归一化结构的非枚举标记，保证 normalize 幂等且不会出现在 JSON/toEqual 中 */
const NORMALIZED_BRAND = '__aetherlinkDryRunNormalized'

export type DryRunResponseInput = DryRunWireResult | NormalizedDryRun | null | undefined

const isRecord = (value: unknown): value is Record<string, unknown> =>
  !!value && typeof value === 'object' && !Array.isArray(value)

export const isNormalizedDryRun = (value: unknown): value is NormalizedDryRun =>
  isRecord(value) && (value as Record<string, unknown>)[NORMALIZED_BRAND] === true

const brand = (value: NormalizedDryRun): NormalizedDryRun => {
  Object.defineProperty(value, NORMALIZED_BRAND, { value: true, enumerable: false })
  return value
}

const asBoolean = (value: unknown): boolean | null => (typeof value === 'boolean' ? value : null)

const asString = (value: unknown): string => (typeof value === 'string' ? value : '')

const asNumber = (value: unknown): number | null => (typeof value === 'number' && Number.isFinite(value) ? value : null)

/** 把字符串、字符串数组或 {message} 对象数组统一成字符串数组 */
export const toMessageList = (value: unknown): string[] => {
  if (!value) return []
  if (typeof value === 'string') return [value]
  if (!Array.isArray(value)) return []

  return value
    .filter((item) => item !== null && item !== undefined && item !== '')
    .map((item) => {
      if (typeof item === 'string') return item
      if (isRecord(item) && typeof item.message === 'string' && item.message) return item.message

      return String(item)
    })
}

const dedupe = (values: string[]): string[] => Array.from(new Set(values))

/** 计数映射只保留有限数值；数组、null 等非法结构归一化为空对象 */
const toCountRecord = (value: unknown): Record<string, number> => {
  if (!isRecord(value)) return {}

  const record: Record<string, number> = {}
  for (const [key, count] of Object.entries(value)) {
    const numeric = asNumber(count)
    if (numeric !== null) record[key] = numeric
  }

  return record
}

/**
 * 第一个"存在"的计数映射；用于在等价字段间做有序回退。
 * 后端显式返回的空对象 {} 视为权威结果（"没有引用"），不会继续回退到后续字段。
 */
const firstCountRecord = (...candidates: unknown[]): Record<string, number> => {
  for (const candidate of candidates) {
    if (isRecord(candidate)) return toCountRecord(candidate)
  }

  return {}
}

const toSeverity = (value: string): DryRunDiagnosticSeverity => {
  if (value === 'success' || value === 'error' || value === 'warning') return value

  return 'info'
}

const normalizeDiagnostics = (value: unknown): NormalizedDryRunDiagnostic[] => {
  if (!Array.isArray(value)) return []

  return value
    .filter((item) => item !== null && item !== undefined)
    .map((item) => {
      if (!isRecord(item)) {
        return { severity: 'info', rawSeverity: '', scope: '', message: String(item) }
      }
      const rawSeverity = asString(item.severity)

      return {
        severity: toSeverity(rawSeverity),
        rawSeverity,
        scope: asString(item.scope),
        message: asString(item.message) || String(item)
      }
    })
}

const normalizeStats = (dryRun: SceneAutomationDryRunStats): NormalizedDryRunStats => ({
  conditionGroupCount: asNumber(dryRun.condition_group_count) ?? 0,
  conditionCount: asNumber(dryRun.condition_count) ?? 0,
  actionCount: asNumber(dryRun.action_count) ?? 0,
  conditionTypes: toCountRecord(dryRun.condition_types),
  actionTypes: toCountRecord(dryRun.action_types),
  targetKinds: toCountRecord(dryRun.target_kinds)
})

const normalizeTraceStep = (step: unknown): NormalizedDryRunTraceStep => {
  const source = isRecord(step) ? (step as DryRunWireTraceStep) : {}

  return {
    index: asNumber(source.index),
    phase: asString(source.phase),
    status: asString(source.status),
    label: asString(source.label),
    kind: asString(source.kind),
    target: asString(source.target),
    detail: asString(source.detail),
    notes: Array.isArray(source.notes) ? source.notes.filter((note): note is string => typeof note === 'string') : []
  }
}

const normalizeTrace = (value: unknown): NormalizedDryRunTrace | null => {
  if (!isRecord(value)) return null

  const trace = value as DryRunWireTrace
  const steps = Array.isArray(trace.steps) ? trace.steps.map(normalizeTraceStep) : []

  return {
    steps,
    stepCount: asNumber(trace.step_count) ?? steps.length,
    // 旧版 camelCase 兜底：evaluatedAt
    evaluatedAt: asString(trace.evaluated_at) || asString(trace.evaluatedAt),
    explanation: asString(trace.explanation),
    // 后端始终是静态推演；只有显式 false 才视为非模拟
    isSimulation: trace.is_simulation !== false
  }
}

const emptyStats = (): NormalizedDryRunStats => ({
  conditionGroupCount: 0,
  conditionCount: 0,
  actionCount: 0,
  conditionTypes: {},
  actionTypes: {},
  targetKinds: {}
})

export const emptyNormalizedDryRun = (): NormalizedDryRun =>
  brand({
    present: false,
    supported: null,
    valid: null,
    canSave: null,
    summary: '',
    stats: emptyStats(),
    referenceCounts: {},
    blockingErrors: [],
    warnings: [],
    errors: [],
    rawWarnings: [],
    skippedConditions: [],
    unavailableActions: [],
    matchedDevices: null,
    diagnostics: [],
    nextSteps: [],
    trace: null
  })

/**
 * 把后端预演响应归一化为视图层唯一可读的结构。幂等：已归一化的输入原样返回。
 */
export const normalizeDryRunResponse = (raw: DryRunResponseInput | unknown): NormalizedDryRun => {
  if (isNormalizedDryRun(raw)) return raw
  if (!isRecord(raw)) return emptyNormalizedDryRun()

  const wire = raw as DryRunWireResult
  // 旧版 camelCase 兜底：dryRun
  const dryRunSource = (
    isRecord(wire.dry_run) ? wire.dry_run : isRecord(wire.dryRun) ? wire.dryRun : {}
  ) as SceneAutomationDryRunStats
  const stats = normalizeStats(dryRunSource)
  const diagnostics = normalizeDiagnostics(wire.diagnostics)
  const errors = toMessageList(wire.errors)
  const rawWarnings = toMessageList(wire.warnings)
  const valid = asBoolean(wire.valid)

  // can_save 为主；旧版 camelCase canSave 兜底；更早期后端没有 can_save 时以 valid 代替
  const canSave = asBoolean(wire.can_save) ?? asBoolean(wire.canSave) ?? valid

  // 后端会把同一条引用校验失败同时写进 errors、blocking_errors 和 error 级诊断，这里去重
  const blockingErrors = dedupe([
    ...toMessageList(wire.blocking_errors),
    // 旧版兜底：早期网关把阻断原因放在 blockers
    ...toMessageList(wire.blockers),
    ...errors,
    ...diagnostics.filter((item) => item.rawSeverity === 'error').map((item) => item.message)
  ])
  const warnings = dedupe([
    ...rawWarnings,
    ...diagnostics.filter((item) => item.rawSeverity === 'warning').map((item) => item.message)
  ])

  return brand({
    present: true,
    supported: asBoolean(wire.supported),
    valid,
    canSave,
    summary: asString(wire.summary),
    stats,
    referenceCounts: firstCountRecord(
      wire.reference_counts,
      // 旧版 camelCase 兜底
      wire.referenceCounts,
      // 旧版兜底：曾经把 reference_counts 嵌在 dry_run 内
      dryRunSource.reference_counts,
      // 后端未给出引用计数时，以目标资源种类计数近似
      dryRunSource.target_kinds
    ),
    blockingErrors,
    warnings,
    errors,
    rawWarnings,
    skippedConditions: dedupe([
      ...toMessageList(wire.skipped_conditions),
      // 旧版 camelCase 兜底
      ...toMessageList(wire.skippedConditions)
    ]),
    unavailableActions: dedupe([
      ...toMessageList(wire.unavailable_actions),
      // 旧版 camelCase 兜底
      ...toMessageList(wire.unavailableActions)
    ]),
    matchedDevices:
      asNumber(wire.matched_devices) ??
      // 旧版 camelCase 兜底
      asNumber(wire.matchedDevices) ??
      asNumber((dryRunSource as Record<string, unknown>).matched_devices) ??
      asNumber((dryRunSource as Record<string, unknown>).matchedDevices),
    diagnostics,
    nextSteps: toMessageList(
      Array.isArray(wire.next_steps)
        ? wire.next_steps
        : // 旧版 camelCase 兜底
          wire.nextSteps
    ),
    // 旧版 camelCase 兜底：executionTrace
    trace: normalizeTrace(wire.execution_trace ?? wire.executionTrace)
  })
}
