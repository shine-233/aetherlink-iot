/**
 * Command Job 视图层的基础层：共享类型、语气（tone）分类与纯文本格式化。
 *
 * 该文件不依赖任何其他 command-center 视图模块，progress / outcome / history
 * 三个领域模块都只向下依赖这里，保证没有循环引用。
 */

export type Translate = (key: string) => string

/** Naive UI tag/alert 使用的四级语气。 */
export type CommandJobTone = 'success' | 'info' | 'warning' | 'error'

export interface CommandJobLabelValueRow {
  key?: string
  label: string
  value: string
}

export interface CommandJobStatusCountRow {
  status: string
  label: string
  count: number
}

// ---------------------------------------------------------------------------
// Tone 分类
// ---------------------------------------------------------------------------

/** 严重程度排序：数值越小越需要先处理。 */
const COMMAND_JOB_TONE_SEVERITY: Readonly<Record<CommandJobTone, number>> = {
  error: 0,
  warning: 1,
  info: 2,
  success: 3
}

export function commandJobToneSeverity(tone: CommandJobTone) {
  return COMMAND_JOB_TONE_SEVERITY[tone]
}

export type CommandJobToneTable = Readonly<Record<string, CommandJobTone>>

/**
 * 由「状态值 -> 语气」表构建解析器。未知值落到 fallback，
 * 各领域的 fallback 不同（进度健康未知视为异常，治理未知视为提示）。
 */
export function createCommandJobToneResolver(table: CommandJobToneTable, fallback: CommandJobTone) {
  return (value: string | undefined | null): CommandJobTone =>
    value && Object.hasOwn(table, value) ? table[value] : fallback
}

/** 后端 progress_health.state；canceled / timed_out / needs_attention 走 error 兜底。 */
export const COMMAND_JOB_PROGRESS_HEALTH_TONES: CommandJobToneTable = {
  complete: 'success',
  scheduled: 'info',
  running: 'info',
  timeout_risk: 'warning'
}
export const resolveCommandJobProgressHealthTone = createCommandJobToneResolver(
  COMMAND_JOB_PROGRESS_HEALTH_TONES,
  'error'
)

/** 后端 governance_summary.level。 */
export const COMMAND_JOB_GOVERNANCE_LEVEL_TONES: CommandJobToneTable = {
  success: 'success',
  warning: 'warning',
  error: 'error',
  blocked: 'error'
}
export const resolveCommandJobGovernanceLevelTone = createCommandJobToneResolver(
  COMMAND_JOB_GOVERNANCE_LEVEL_TONES,
  'info'
)

/** 后端 governance_summary.items[].state。 */
export const COMMAND_JOB_GOVERNANCE_STATE_TONES: CommandJobToneTable = {
  done: 'success',
  blocked: 'error',
  watch: 'warning',
  todo: 'warning'
}
export const resolveCommandJobGovernanceStateTone = createCommandJobToneResolver(
  COMMAND_JOB_GOVERNANCE_STATE_TONES,
  'info'
)

/** 单设备进度轨道中每一步（preview/dispatch/ack/evidence）的状态。 */
export const COMMAND_JOB_DEVICE_STEP_TONES: CommandJobToneTable = {
  done: 'success',
  blocked: 'error',
  failed: 'error',
  waiting: 'warning',
  missing: 'warning'
}
export const resolveCommandJobDeviceStepTone = createCommandJobToneResolver(COMMAND_JOB_DEVICE_STEP_TONES, 'info')

/** 计数型指标：为 0 即健康，大于 0 时取该指标自身的告警语气。 */
export function resolveCommandJobCountTone(count: number | undefined, activeTone: CommandJobTone): CommandJobTone {
  return (count ?? 0) > 0 ? activeTone : 'success'
}

/** 按顺序匹配的规则表，首个命中的规则决定结果。 */
export interface CommandJobRule<TInput, TResult> {
  when: (input: TInput) => boolean
  then: TResult
}

export function matchCommandJobRule<TInput, TResult>(
  rules: ReadonlyArray<CommandJobRule<TInput, TResult>>,
  input: TInput,
  fallback: TResult
): TResult {
  return rules.find((rule) => rule.when(input))?.then ?? fallback
}

// ---------------------------------------------------------------------------
// 文本格式化
// ---------------------------------------------------------------------------

/** 翻译 key；若 i18n 缺失（返回 key 本身）则使用 fallback。 */
export function translateCommandJobOr(t: Translate, key: string, fallback: string) {
  const translated = t(key)
  return translated === key ? fallback : translated
}

/** 用 `{name}` 占位符填充翻译模板；split/join 避免 String.replace 的 `$&` 特殊语义。 */
export function fillCommandJobTemplate(template: string, values: Record<string, string | number>) {
  return Object.entries(values).reduce((text, [name, value]) => text.split(`{${name}}`).join(String(value)), template)
}

export function formatCommandJobStatus(status: string | undefined, t: Translate) {
  if (!status) return '-'
  return translateCommandJobOr(t, `custom.commandCenter.jobStatus.${status}`, status)
}

export function formatCommandJobResponseStatus(statusLabel: string | undefined, t: Translate) {
  if (!statusLabel) return t('custom.commandCenter.responseStatus.awaiting')
  return translateCommandJobOr(t, `custom.commandCenter.responseStatus.${statusLabel}`, statusLabel)
}

export function formatCommandJobReadiness(readiness: string[] | undefined) {
  const items = readiness?.filter(Boolean) ?? []
  return items.length ? items.join(', ') : '-'
}

export function formatCommandJobDateTime(value?: string) {
  if (!value) return '--'
  return value
    .replace('T', ' ')
    .replace(/\.\d+Z$/, ' UTC')
    .replace(/Z$/, ' UTC')
}

export function formatCommandJobDuration(seconds: number | undefined, t: Translate) {
  if (typeof seconds !== 'number' || !Number.isFinite(seconds)) return '--'
  if (seconds < 0) return t('custom.commandCenter.progressHealthExpired')
  if (seconds < 60)
    return fillCommandJobTemplate(t('custom.commandCenter.progressHealthSeconds'), { seconds: Math.round(seconds) })
  return fillCommandJobTemplate(t('custom.commandCenter.progressHealthMinutes'), { minutes: Math.ceil(seconds / 60) })
}

export function formatCommandJobYesNo(value: boolean | undefined, t: Translate) {
  return value ? t('common.yesOrNo.yes') : t('common.yesOrNo.no')
}
