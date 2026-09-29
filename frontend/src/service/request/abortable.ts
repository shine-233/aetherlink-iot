/*
 * 文件用途：为"被取代的列表查询"提供 AbortController 生命周期管理。
 * 核心逻辑：同一 runner 内每次 run 都会先 abort 上一次仍在飞行的请求；被取代的那次
 *   返回 null（而不是抛错），调用方据此跳过过期结果，避免旧分页覆盖新分页。
 * 关键注意事项：
 *   - 取消不是错误：不会抛异常，更不会冒到全局 message。
 *   - 只有支持 AbortSignal 的 API 包装函数才能真正中断网络请求；不支持的旧包装
 *     退化为"结果被丢弃"，行为仍然正确（只是省不了带宽）。
 * 重构建议：若列表页统一迁移到 resource helper，可移除对"结果丢弃"的兜底依赖。
 */

/**
 * 判断是否取消语义（AbortController / CancelToken）。
 *
 * 兼容两种形状：
 * - 原始 AxiosError：`{ code: 'ERR_CANCELED', name: 'CanceledError' }`
 * - flatRequest 归一化后的拒绝值：`{ data: null, error: { code, message } }`
 */
export function isCanceledError(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false

  const candidate = error as { code?: unknown; name?: unknown; error?: { code?: unknown; name?: unknown } }
  const nested = candidate.error

  return isCancelCode(candidate.code) || isCancelName(candidate.name) || isCancelCode(nested?.code) || isCancelName(nested?.name)
}

function isCancelCode(code: unknown): boolean {
  // 20 是 axios v0.x CancelToken 的历史码，兜底。
  return code === 'ERR_CANCELED' || code === 20
}

function isCancelName(name: unknown): boolean {
  return name === 'CanceledError' || name === 'AbortError'
}

export interface LatestQueryRunner<T> {
  /** 执行查询；若被后续 run 取代则返回 null。 */
  run: (factory: (signal: AbortSignal) => Promise<T>) => Promise<T | null>
  /** 主动取消当前查询（组件卸载、路由切换等）。 */
  cancel: () => void
  /** 当前是否有未 settle 的查询。 */
  isActive: () => boolean
}

/**
 * 创建一个"只保留最后一次查询"的运行器。
 */
export function createLatestQueryRunner<T = unknown>(): LatestQueryRunner<T> {
  let controller: AbortController | null = null
  let generation = 0

  const cancel = () => {
    controller?.abort()
    controller = null
    // 即使底层不支持 signal，也要让仍在飞行的旧结果被判定为过期。
    generation += 1
  }

  const isActive = () => controller !== null

  const run = async (factory: (signal: AbortSignal) => Promise<T>): Promise<T | null> => {
    // 取代上一次：先 abort 再建立新的 controller。
    cancel()

    const next = new AbortController()
    controller = next
    const currentGeneration = ++generation

    try {
      const result = await factory(next.signal)
      // 期间又发起了新查询 —— 本次结果已过期，直接丢弃。
      if (currentGeneration !== generation) return null
      return result
    } catch (error) {
      if (isCanceledError(error)) return null
      // 被取代但底层不支持 signal：generation 已变，同样按过期处理。
      if (currentGeneration !== generation) return null
      throw error
    } finally {
      if (controller === next) {
        controller = null
      }
    }
  }

  return { run, cancel, isActive }
}
