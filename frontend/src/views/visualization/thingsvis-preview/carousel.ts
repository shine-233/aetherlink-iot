/**
 * TP-22 大屏轮播纯逻辑模块（/tv-preview 多屏轮播投屏）。
 *
 * 与组件解耦的原因：轮播定时器和 URL 参数解析是"电视墙黑屏/跳错屏"最沉默的失效点，
 * 必须能在无 DOM、无真实时钟的测试里逐毫秒验证。页面只消费这里的行为，不自带定时逻辑。
 *
 * 设计规则：
 *  - interval 是展示参数，只存在于投屏页 URL（后端公开端点刻意不收展示参数，
 *    见 backend/internal/api/board_carousel.go），非法值回落默认而不是报错：
 *    电视墙 URL 往往手写或被投屏器截断，参数坏掉应当"还能播"，而不是黑屏。
 *  - 轮播数据面按 share token 解析（公开面凭证语义，不认内部 board id），
 *    token 清洗在后端 BoardCarouselMaxTokens 内闭环，这里只做 URL 形态归一。
 */

/** 默认切换间隔（秒）：电视墙一屏看清单大约 10-20s 的习惯值。 */
export const CAROUSEL_DEFAULT_INTERVAL_SECONDS = 15
/** 下限：低于 3s 人眼无法完成"看懂这一屏"，且会放大公开端点的取数频率。 */
export const CAROUSEL_MIN_INTERVAL_SECONDS = 3
/** 上限：一小时还没切下一屏的"轮播"实际上是单屏投放。 */
export const CAROUSEL_MAX_INTERVAL_SECONDS = 3600

export interface CarouselPlaylistQuery {
  tokens: string[]
  intervalSeconds: number
}

function asStringArray(value: unknown): string[] {
  if (typeof value === 'string') return [value]
  if (Array.isArray(value)) return value.filter((item): item is string => typeof item === 'string')
  return []
}

/** 规范化切换间隔：非正数/非有限数字一律回落默认，越界收敛到上下限。 */
export function normalizeCarouselIntervalSeconds(raw: unknown): number {
  const value = typeof raw === 'string' ? Number(raw) : raw
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    return CAROUSEL_DEFAULT_INTERVAL_SECONDS
  }
  const floored = Math.floor(value)
  if (floored < CAROUSEL_MIN_INTERVAL_SECONDS) return CAROUSEL_MIN_INTERVAL_SECONDS
  if (floored > CAROUSEL_MAX_INTERVAL_SECONDS) return CAROUSEL_MAX_INTERVAL_SECONDS
  return floored
}

/**
 * 解析 /tv-preview 的轮播 URL 参数。
 * `?tokens=a,b,c` 与 `?tokens=a&tokens=b` 两种形态等价（与后端 QueryArray 口径一致），
 * 空段跳过、去重保序。没有可用 token 时返回 null：调用方回落单看板预览模式。
 */
export function parseCarouselQuery(query: Record<string, unknown>): CarouselPlaylistQuery | null {
  const seen = new Set<string>()
  const tokens: string[] = []
  for (const group of asStringArray(query.tokens)) {
    for (const token of group.split(',')) {
      const trimmed = token.trim()
      if (!trimmed || seen.has(trimmed)) continue
      seen.add(trimmed)
      tokens.push(trimmed)
    }
  }
  if (tokens.length === 0) return null
  return { tokens, intervalSeconds: normalizeCarouselIntervalSeconds(query.interval) }
}

export interface CarouselTimerOptions {
  /** 切换间隔毫秒数（intervalSeconds * 1000）。 */
  intervalMs: number
  /** 轮播清单长度；运行中清单可能被重载，所以是函数而不是数字。 */
  count: () => number
  /** 每次切换回调，参数为新激活下标（已按 count 取模）。 */
  onAdvance: (index: number) => void
  /** 可注入的调度器（测试注入假时钟）；默认 setTimeout/clearTimeout。 */
  schedule?: (callback: () => void, ms: number) => unknown
  cancel?: (handle: unknown) => void
}

export interface CarouselTimer {
  /** 开始轮播。清单少于 2 块时不轮播（单屏投放无需计时器）。幂等。 */
  start: () => void
  /** 停止轮播并取消待执行的切换。幂等。 */
  stop: () => void
  /** 立即切到下一屏并按原节奏重新计时（供"当前屏加载失败，跳过等待"使用）。 */
  advanceNow: () => void
  /** 当前激活下标。 */
  readonly index: number
  /** 是否已有待执行的切换。 */
  readonly running: boolean
}

/**
 * 轮播计时器：到点切下一屏（取模循环）、切换后继续按原间隔计时。
 * 计时器永不因"某屏加载失败"停摆——电视墙的第一美德是一直在播，失败屏
 * 由页面渲染可诊断的错误态，下一屏到点照常切入。
 */
export function createCarouselTimer(options: CarouselTimerOptions): CarouselTimer {
  const schedule = options.schedule ?? ((callback: () => void, ms: number) => setTimeout(callback, ms))
  const cancel = options.cancel ?? ((handle: unknown) => clearTimeout(handle as ReturnType<typeof setTimeout>))

  let index = 0
  let handle: unknown = null

  function tick() {
    handle = null
    const count = options.count()
    if (count < 2) return
    index = (index + 1) % count
    options.onAdvance(index)
    scheduleNext()
  }

  function scheduleNext() {
    if (handle !== null) return
    handle = schedule(tick, options.intervalMs)
  }

  return {
    start() {
      if (handle !== null) return
      if (options.count() < 2) return
      scheduleNext()
    },
    stop() {
      if (handle !== null) {
        cancel(handle)
        handle = null
      }
    },
    advanceNow() {
      if (handle !== null) {
        cancel(handle)
        handle = null
      }
      tick()
    },
    get index() {
      return index
    },
    get running() {
      return handle !== null
    }
  }
}
