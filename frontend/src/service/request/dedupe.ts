/*
 * 文件用途：为只读 GET 提供进程内 in-flight 去重。
 * 核心逻辑：按「URL + 规范化 query」生成 key，重复命中时复用同一个 in-flight promise，
 *   但每个调用方拿到的是独立副本，避免多方共享同一个可变对象导致互相污染。
 * 关键注意事项：
 *   - 只对 GET（幂等读）生效；写请求绝不能去重。
 *   - 请求 settle 后立刻从表中移除，因此不存在"永久缓存"导致的脏读。
 *   - 非可克隆载荷（Blob/FormData/函数）直接返回原值，不做拷贝。
 * 重构建议：若后续要加 TTL 缓存，应另建模块，不要与本文件的 in-flight 语义混用。
 */
import type { CustomAxiosRequestConfig, FlatResponseData } from '@aetherlink/axios'

type QueryValue = unknown
function buildDedupeKey(url: string, params?: unknown): string {
  if (!params) return url

  if (typeof URLSearchParams !== 'undefined' && params instanceof URLSearchParams) {
    // entries 必须先按 key 排序：URLSearchParams 保留插入序，同参数不同顺序会生成不同键，
    // 导致去重失效（2026-09-30 去重键稳定性测试抓出的回归）。
    const entries = [...params.entries()].sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
    return `${url}?${entries.map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(value)}`).join('&')}`
  }

  return `${url}?${normalizeQueryValue(params)}`
}
const inFlight = new Map<string, Promise<FlatResponseData<unknown>>>()

/** 当前正在去重的请求数（测试与诊断用）。 */
export function getInFlightRequestCount(): number {
  return inFlight.size
}

/** 清空去重表（仅测试用）。 */
export function resetInFlightRequests(): void {
  inFlight.clear()
}

/**
 * 执行一个可去重的 GET。
 *
 * @param key 去重键，通常由「URL + 规范化 params」构成
 * @param run 真正发起请求的函数
 */
export async function dedupeGet<T>(key: string, run: () => Promise<FlatResponseData<T>>): Promise<FlatResponseData<T>> {
  const pending = inFlight.get(key)
  if (pending) {
    const shared = await pending
    // 复用结果但返回独立副本，避免调用方 A 的修改影响调用方 B。
    return cloneResponseValue(shared) as FlatResponseData<T>
  }

  const promise = run().finally(() => {
    // settle 后移除：只有"真正并发"的请求才共享，后续请求会重新打网络。
    if (inFlight.get(key) === promise) {
      inFlight.delete(key)
    }
  })

  inFlight.set(key, promise as Promise<FlatResponseData<unknown>>)

  // 失败结果与成功结果一样向外抛，交给调用方/全局 onError 处理。
  return await promise
}

/** 从 url + config 推导去重键。 */
export function buildGetDedupeKey(url: string, config?: CustomAxiosRequestConfig): string {
  return buildDedupeKey(url, config?.params)
}

/** 规范化 query 参数为稳定字符串（对象键排序），保证同参数生成同键。 */
function normalizeQueryValue(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (Array.isArray(value)) return value.map((item) => normalizeQueryValue(item)).join('&')
  if (typeof value === 'object') {
    return Object.entries(value as Record<string, unknown>)
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
      .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(normalizeQueryValue(v))}`)
      .join('&')
  }
  return encodeURIComponent(String(value))
}

/**
 * 深拷贝响应值，让去重命中的调用方各持独立副本。
 * Blob/FormData/ArrayBuffer/函数等不可安全克隆的载荷原样返回。
 */
export function cloneResponseValue<T>(value: T): T {
  if (value === null || typeof value !== 'object') return value
  if (typeof structuredClone === 'function') {
    try {
      return structuredClone(value)
    } catch {
      return value
    }
  }
  return JSON.parse(JSON.stringify(value)) as T
}
