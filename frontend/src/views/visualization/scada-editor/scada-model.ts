/**
 * 文件用途: SCADA 画布编辑器的纯模型层（ROADMAP P1.3）。
 * 核心逻辑: 画布解析/序列化、版本冲突识别、遥测陈旧判定、Widget 能力降级。
 *
 * 关键注意事项（每一条都对应一类真实的假成功）:
 *  1. **画布解析失败绝不能退化成空画布**。一旦解析失败返回一个"空画布"，
 *     编辑器会显示空白，用户随手一保存就把真实内容覆盖成空——
 *     这是数据丢失，不是显示问题。故解析失败必须返回错误并阻断保存。
 *  2. 版本冲突必须被**准确识别**出来。若与网络错误混为一谈，
 *     用户会反复重试，而重试永远失败（他保存的是过期版本）。
 *  3. 遥测陈旧判定与后端保持同一语义：未连接时恒为陈旧，
 *     不看最后一帧有多新——刚断开 1 秒的数据同样不是实时数据。
 *  4. 3D Widget 在无 WebGL 时降级为占位，不得影响 2D Widget 渲染，
 *     未注册 Widget 同样只降级不阻断。
 */

export type ScadaRenderCapability = '2d' | '3d'

export interface ScadaWidgetLayout {
  x: number
  y: number
  w: number
  h: number
}

export interface ScadaWidgetInstance {
  id: string
  widget_type: string
  version: string
  layout: ScadaWidgetLayout
  config?: Record<string, unknown>
}

export interface ScadaBinding {
  source: string
  target_widget_id: string
  target_property: string
  transform?: string
}

export interface ScadaCanvas {
  widgets: ScadaWidgetInstance[]
  variables?: Record<string, unknown>
  bindings?: ScadaBinding[]
}

export interface ScadaWidgetDefinition {
  type: string
  version: string
  schema: string
  capabilities: ScadaRenderCapability[]
  commands?: { name: string; requires_confirmation: boolean }[]
}

export type CanvasParseResult =
  | { ok: true; canvas: ScadaCanvas }
  | { ok: false; reason: 'empty' | 'not-object' | 'invalid-json' | 'bad-widgets' }

export const SCADA_EMPTY_CANVAS = '{}'

/** 空画布常量：与后端归一化结果一致（空白折叠为 {}）。 */
export function emptyCanvas(): ScadaCanvas {
  return { widgets: [] }
}

/**
 * 解析画布载荷。
 * 注意：返回 ok:false 时调用方**必须阻断保存**，不得回退成空画布继续编辑。
 */
export function parseCanvas(raw: string | null | undefined): CanvasParseResult {
  if (raw == null || raw.trim() === '') {
    return { ok: true, canvas: emptyCanvas() }
  }
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return { ok: false, reason: 'invalid-json' }
  }
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    return { ok: false, reason: 'not-object' }
  }
  const record = parsed as Record<string, unknown>
  const rawWidgets = record.widgets
  if (rawWidgets === undefined) {
    return { ok: true, canvas: { ...(record as object), widgets: [] } as ScadaCanvas }
  }
  if (!Array.isArray(rawWidgets)) {
    return { ok: false, reason: 'bad-widgets' }
  }
  const widgets: ScadaWidgetInstance[] = []
  for (const item of rawWidgets) {
    if (typeof item !== 'object' || item === null) {
      return { ok: false, reason: 'bad-widgets' }
    }
    const w = item as Record<string, unknown>
    if (typeof w.id !== 'string' || typeof w.widget_type !== 'string') {
      return { ok: false, reason: 'bad-widgets' }
    }
    const layout = (w.layout ?? {}) as Record<string, unknown>
    widgets.push({
      id: w.id,
      widget_type: w.widget_type,
      version: typeof w.version === 'string' ? w.version : '',
      layout: {
        x: numberOr(layout.x, 0),
        y: numberOr(layout.y, 0),
        w: numberOr(layout.w, 2),
        h: numberOr(layout.h, 2)
      },
      config: typeof w.config === 'object' && w.config !== null ? (w.config as Record<string, unknown>) : undefined
    })
  }
  return {
    ok: true,
    canvas: {
      widgets,
      variables: typeof record.variables === 'object' && record.variables !== null
        ? (record.variables as Record<string, unknown>)
        : undefined,
      bindings: Array.isArray(record.bindings) ? (record.bindings as ScadaBinding[]) : undefined
    }
  }
}

function numberOr(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

/** 序列化画布。空画布统一写成 {}，与后端归一化一致。 */
export function serializeCanvas(canvas: ScadaCanvas): string {
  if (!canvas.widgets || canvas.widgets.length === 0) {
    const hasExtras = canvas.variables || canvas.bindings
    if (!hasExtras) return SCADA_EMPTY_CANVAS
  }
  return JSON.stringify(canvas)
}

/**
 * 识别后端返回的"版本冲突"。
 * 后端以 CodeOpDenied(201002) + 含 version conflict 的文案返回。
 * 两个条件取或：只认 code 会在文案调整后漏判，只认文案会在别的拒绝场景误判为冲突。
 */
export function isVersionConflictError(error: unknown): boolean {
  const e = error as { response?: { data?: { code?: number; message?: string } } } | null
  const data = e?.response?.data
  if (!data) return false
  if (data.code === 201002 && typeof data.message === 'string' && data.message.includes('version conflict')) {
    return true
  }
  return typeof data.message === 'string' && data.message.toLowerCase().includes('version conflict')
}

/** 识别"文档已归档"：归档为终态，继续保存没有意义，应提示用户而非重试。 */
export function isArchivedError(error: unknown): boolean {
  const e = error as { response?: { data?: { message?: string } } } | null
  const message = e?.response?.data?.message
  return typeof message === 'string' && message.toLowerCase().includes('archived')
}

/** 遥测链路状态，与后端 scada_telemetry_link.go 保持一致。 */
export type TelemetryLinkState = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'reconnecting' | 'error'

/**
 * 判断画布数据是否陈旧。
 * 关键：非 connected 状态**恒为 true**，刻意不参考最后消息时间。
 */
export function isTelemetryStale(
  state: TelemetryLinkState,
  lastMessageAt: number | null,
  now: number,
  maxSilenceMs = 30_000
): boolean {
  if (state !== 'connected') return true
  if (lastMessageAt == null) return true
  return now - lastMessageAt > maxSilenceMs
}

export interface WidgetResolution {
  available: ScadaWidgetDefinition[]
  degraded: ScadaWidgetDefinition[]
  unknown: ScadaWidgetInstance[]
}

/**
 * 解析画布实例：3D 在无 WebGL 时降级，未注册类型判为 unknown。
 * 本函数**永不抛错**：任何 Widget 不可用都不应让整块看板打不开。
 */
export function resolveWidgets(
  instances: ScadaWidgetInstance[],
  registry: ScadaWidgetDefinition[],
  webglAvailable: boolean
): WidgetResolution {
  const result: WidgetResolution = { available: [], degraded: [], unknown: [] }
  for (const inst of instances) {
    const def =
      registry.find(d => d.type === inst.widget_type && d.version === inst.version) ??
      latestOfType(registry, inst.widget_type)
    if (!def) {
      result.unknown.push(inst)
      continue
    }
    if (def.capabilities.includes('3d') && !webglAvailable) {
      result.degraded.push(def)
      continue
    }
    result.available.push(def)
  }
  return result
}

function latestOfType(registry: ScadaWidgetDefinition[], type: string): ScadaWidgetDefinition | undefined {
  const same = registry.filter(d => d.type === type)
  if (same.length === 0) return undefined
  return same.reduce((best, cur) => (cur.version > best.version ? cur : best))
}

/** 文档是否可编辑：归档为终态。 */
export function canEditDocument(status: string | undefined): boolean {
  return status !== 'ARCHIVED'
}

/**
 * 命令是否需要二次确认：由注册定义决定，前端不得自行放宽。
 * 未知 Widget / 未知命令一律返回 true（fail closed）：
 * 查不到注册信息时说"不需要确认"，等于放行一个我们根本不了解危险性的操作。
 * 后端本来就会拒绝未声明的命令，这里保守取值只是让危险操作多一道提示。
 */
export function commandRequiresConfirmation(
  registry: ScadaWidgetDefinition[],
  widgetType: string,
  command: string
): boolean {
  const def = registry.find(d => d.type === widgetType)
  const cmd = def?.commands?.find(c => c.name === command)
  return cmd?.requires_confirmation ?? true
}
