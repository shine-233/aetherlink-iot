/**
 * 状态 → 视觉语气（Naive UI tag/alert `type`）的统一分类。
 *
 * 业务模块里反复出现两类内联三元：
 * - 进度三态 done/active/todo → success/warning/default（首页首设备引导、闭环画布）
 * - 业务状态 → success/info/warning/error 四态（指令中心任务、进度步骤）
 *
 * 这里把映射收敛为表驱动的解析器：业务侧只声明「状态 → 语气」表和兜底语气，
 * 不再手写嵌套三元；未知状态一律落到兜底值，保证 UI 不会拿到 undefined。
 */

/** 四态告警语气，对应 NAlert / NTag 的 type。 */
export type StatusTone = 'success' | 'info' | 'warning' | 'error'

/** 进度标签语气：完成 / 进行中 / 未开始。 */
export type ProgressTone = 'success' | 'warning' | 'default'

/** 线性引导中单个步骤的进度状态。 */
export type ProgressState = 'done' | 'active' | 'todo'

/**
 * 创建表驱动的状态语气解析器。
 *
 * @param table    已知状态到语气的映射；可以只列出非兜底的状态
 * @param fallback 未命中（含 null/undefined）时返回的语气
 */
export const createToneResolver = <S extends string, T extends string>(table: Partial<Record<S, T>>, fallback: T) => {
  const lookup = new Map<string, T>(Object.entries(table) as Array<[string, T]>)
  return (state: S | string | null | undefined): T => (state == null ? fallback : (lookup.get(state) ?? fallback))
}

const PROGRESS_TONE_TABLE: Record<ProgressState, ProgressTone> = {
  done: 'success',
  active: 'warning',
  todo: 'default'
}

/** done → success，active → warning，其余 → default。 */
export const resolveProgressTone = createToneResolver<ProgressState, ProgressTone>(PROGRESS_TONE_TABLE, 'default')

/**
 * 进度状态的完整展示描述：语气 + 文案。
 * 文案因页面而异（「已完成/现在做」 vs 「已通过/当前卡点」），由调用方传入。
 */
export const describeProgressState = (state: ProgressState, labels: Record<ProgressState, string>) => ({
  label: labels[state] ?? labels.todo,
  tone: resolveProgressTone(state)
})

/** 从「是否完成 + 是否为当前第一个卡点」推导进度状态。 */
export const resolveProgressState = (done: boolean, isCurrent: boolean): ProgressState =>
  done ? 'done' : isCurrent ? 'active' : 'todo'

/** 语气严重度：error 最需要处理，success 最不需要；用于把问题项排在前面。 */
export const STATUS_TONE_SEVERITY: Readonly<Record<StatusTone, number>> = Object.freeze({
  error: 0,
  warning: 1,
  info: 2,
  success: 3
})

/** 排序比较器：严重的语气在前；非四态值视为最不严重。 */
export const compareToneSeverity = (a: string, b: string) =>
  (STATUS_TONE_SEVERITY[a as StatusTone] ?? 4) - (STATUS_TONE_SEVERITY[b as StatusTone] ?? 4)

/** 按「是否就绪」二选一的语气，覆盖 ready ? 'success' : 'warning' 这一高频写法。 */
export const readinessTone = (ready: boolean): 'success' | 'warning' => (ready ? 'success' : 'warning')
