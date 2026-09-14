/**
 * 文件用途：P1.6 打包导入（bundle import）的纯逻辑层——解析、闸门判定、请求构造、结果汇总。
 *
 * 关键注意事项：
 * 1. **前端不验签，也不得假装验签**。签名密钥在服务端（`market.bundle_signing_keys`），
 *    前端拿不到也绝不能拿。前端只做"包里有没有签名三字段"的**预检**，
 *    真正的验签由后端 `VerifyMarketBundle` fail closed 执行。
 *    把预检说成"验签通过"是假安全——所以决策值叫 `unsigned` 而不是 `valid`。
 * 2. **阻断项（blocking）不可被用户确认绕过**。preview.blocking 非空时
 *    后端不会导入（依赖自洽检查失败），前端给确认框等于骗用户。
 * 3. **覆盖项（overwrite）必须显式确认，且换文件时确认态必须重置**。
 *    否则用户勾一次之后导入任何包都会静默覆盖已有模板，
 *    最终库状态取决于操作顺序——这正是路线图门禁要防的事。
 * 4. 本文件不依赖 Vue，便于纯函数表驱动测试。
 */

/** 后端 `MarketBundle` 的前端投影（只取前端需要判定的字段）。 */
export interface MarketBundlePayload {
  type_key?: string
  exported_at?: number
  count?: number
  templates?: unknown[]
  /** 内容摘要（hex SHA-256）。 */
  digest?: string
  /** HMAC-SHA256 签名（hex）。 */
  signature?: string
  /** 签名所用密钥 ID。 */
  signed_key_id?: string
  [key: string]: unknown
}

/** 后端 `MarketBundleImportPreview`。数组一律按"可能缺失"处理。 */
export interface MarketBundlePreview {
  total?: number
  create?: string[]
  overwrite?: string[]
  blocking?: string[]
}

export type BundleImportOutcome = 'created' | 'idempotent' | 'rejected'

export interface BundleImportItemResult {
  name?: string
  version?: string
  outcome?: BundleImportOutcome | string
  template_id?: string
  reason?: string
}

export interface BundleImportResponse {
  preview?: MarketBundlePreview
  applied?: boolean
  results?: BundleImportItemResult[]
}

/**
 * 导入闸门决策。顺序即优先级，调用方不应重排：
 * - `invalid`：载荷不是合法 JSON 对象（连预检都做不了）
 * - `empty`：包内没有模板（导入是空操作，不必发请求）
 * - `unsigned`：缺签名三字段，**后端会拒绝**；前端提前告知，别让用户白等一次往返
 * - `blocked`：preview 含阻断项，禁止导入
 * - `needs-confirm`：含覆盖项，需显式确认
 * - `ready`：可直接导入
 */
export type BundleImportDecision = 'invalid' | 'empty' | 'unsigned' | 'blocked' | 'needs-confirm' | 'ready'

/** 文件解析结果。 */
export type ParseBundleResult =
  | { ok: true; bundle: MarketBundlePayload }
  | { ok: false; errorKey: string }

/** 解析打包文件文本。只做结构检查，不做业务校验。 */
export function parseBundleText(text: string): ParseBundleResult {
  if (!text || !text.trim()) return { ok: false, errorKey: 'page.marketBrowse.importErrorEmpty' }

  let parsed: unknown
  try {
    parsed = JSON.parse(text)
  } catch {
    return { ok: false, errorKey: 'page.marketBrowse.importErrorJson' }
  }

  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return { ok: false, errorKey: 'page.marketBrowse.importErrorShape' }
  }

  return { ok: true, bundle: parsed as MarketBundlePayload }
}

/** 包是否带完整签名三字段。**存在不等于有效**——有效性只有后端能判。 */
export function hasBundleSignature(bundle: MarketBundlePayload | null): boolean {
  if (!bundle) return false
  return Boolean(bundle.digest && bundle.signature && bundle.signed_key_id)
}

function countTemplates(bundle: MarketBundlePayload): number {
  if (Array.isArray(bundle.templates)) return bundle.templates.length
  if (typeof bundle.count === 'number' && Number.isFinite(bundle.count)) return bundle.count
  return 0
}

/** 阻断项是否非空（null 安全）。 */
export function hasBlocking(preview: MarketBundlePreview | null | undefined): boolean {
  return Boolean(preview && Array.isArray(preview.blocking) && preview.blocking.length > 0)
}

/** 覆盖项是否非空（null 安全）。 */
export function hasOverwrite(preview: MarketBundlePreview | null | undefined): boolean {
  return Boolean(preview && Array.isArray(preview.overwrite) && preview.overwrite.length > 0)
}

/**
 * 闸门判定。
 * preview 为 null 表示还没拿到预览结果——此时只能做本地可见的判定
 * （解析 / 空包 / 未签名），不得放行到 `ready`。
 */
export function decideBundleImport(bundle: MarketBundlePayload | null, preview: MarketBundlePreview | null): BundleImportDecision {
  if (!bundle) return 'invalid'
  if (!hasBundleSignature(bundle)) return 'unsigned'
  if (countTemplates(bundle) === 0) return 'empty'
  if (!preview) return 'blocked' // 尚未预览：保守拒绝，宁可多一次往返
  if (hasBlocking(preview)) return 'blocked'
  if (hasOverwrite(preview)) return 'needs-confirm'
  return 'ready'
}

/**
 * 是否可以提交导入。
 * `needs-confirm` 下必须显式确认；`blocked`/`unsigned`/`empty` 下确认也无效——
 * 用户勾了勾却照样被拒，比一开始就禁用更糟。
 */
export function canSubmitBundleImport(decision: BundleImportDecision, confirmed: boolean): boolean {
  if (decision === 'ready') return true
  if (decision === 'needs-confirm') return confirmed
  return false
}

/**
 * 换文件时重置确认态。
 * fileKey 用 "文件名|字节数|最后修改时间" 之类能区分文件的指纹；
 * 同名同大小但内容不同的极端情况由后端预览结果兜住——勾选后再换包，
 * 只要指纹变了就重新要求确认。
 */
export function nextConfirmOverwrite(previousFileKey: string | null, nextFileKey: string | null): boolean {
  if (!previousFileKey || !nextFileKey) return false
  return false // 文件指纹变化即重置；首次选择同样为 false
}

/** 构造请求体。`confirm_overwrite` 只在真正需要时发送，避免给后端传噪声。 */
export function buildBundleImportPayload(
  bundle: MarketBundlePayload,
  options: { preview?: boolean; confirmOverwrite?: boolean } = {}
) {
  const payload: { bundle: MarketBundlePayload; preview?: boolean; confirm_overwrite?: boolean } = { bundle }
  if (options.preview) payload.preview = true
  if (options.confirmOverwrite) payload.confirm_overwrite = true
  return payload
}

export interface BundleImportSummary {
  total: number
  created: number
  idempotent: number
  rejected: number
}

/** 结果汇总。未知 outcome 计入 rejected 而非静默丢弃——宁可多报，不可漏报。 */
export function summarizeBundleImportResults(results: BundleImportItemResult[] | null | undefined): BundleImportSummary {
  if (!Array.isArray(results)) return { total: 0, created: 0, idempotent: 0, rejected: 0 }
  const summary: BundleImportSummary = { total: results.length, created: 0, idempotent: 0, rejected: 0 }
  for (const item of results) {
    if (item?.outcome === 'created') summary.created += 1
    else if (item?.outcome === 'idempotent') summary.idempotent += 1
    else summary.rejected += 1
  }
  return summary
}

/**
 * 预览里"会被改动的名字"列表（覆盖项优先展示）。
 * 阻断项单独返回，UI 必须分开渲染——混在一起会让用户以为勾一下就能过。
 */
export function previewNameLists(preview: MarketBundlePreview | null) {
  return {
    create: Array.isArray(preview?.create) ? preview!.create! : [],
    overwrite: Array.isArray(preview?.overwrite) ? preview!.overwrite! : [],
    blocking: Array.isArray(preview?.blocking) ? preview!.blocking! : []
  }
}
