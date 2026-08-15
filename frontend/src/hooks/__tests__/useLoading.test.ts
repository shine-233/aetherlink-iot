/*
 * 文件用途：验证 loading/empty Hook 的默认状态、状态切换和组合初始化行为。
 * 核心逻辑：通过 Vitest 直接调用 Hook，断言 loading、empty 及对应 setter 的响应式变化。
 * 关键注意事项：测试关注公开返回值，不应依赖内部实现顺序或 Vue 调度细节。
 * 重构建议：如 Hook 增加异步能力，应补充异步状态和错误分支断言。
 */
import { describe, expect, it, vi } from 'vitest'

vi.mock('@aetherlink/hooks', () => {
  const ref = (val: boolean) => ({ value: val })
  return {
    useBoolean: (initValue = false) => {
      const bool = { value: initValue }
      return {
        bool,
        setBool: (val: boolean) => { bool.value = val },
        setTrue: () => { bool.value = true },
        setFalse: () => { bool.value = false }
      }
    }
  }
})

import useLoadingEmpty from '../common/use-loading-empty'

describe('useLoadingEmpty hook', () => {
  it('returns loading, startLoading, endLoading, empty, setEmpty', () => {
    const result = useLoadingEmpty()
    expect(result.loading.value).toBe(false)
    expect(result.empty.value).toBe(false)
    expect(typeof result.startLoading).toBe('function')
    expect(typeof result.endLoading).toBe('function')
    expect(typeof result.setEmpty).toBe('function')
  })

  it('initializes loading to false by default', () => {
    const result = useLoadingEmpty()
    expect(result.loading.value).toBe(false)
  })

  it('initializes loading to true when initLoading is true', () => {
    const result = useLoadingEmpty(true)
    expect(result.loading.value).toBe(true)
  })

  it('initializes empty to false by default', () => {
    const result = useLoadingEmpty()
    expect(result.empty.value).toBe(false)
  })

  it('initializes empty to true when initEmpty is true', () => {
    const result = useLoadingEmpty(false, true)
    expect(result.empty.value).toBe(true)
  })

  it('startLoading sets loading to true', () => {
    const result = useLoadingEmpty()
    result.startLoading()
    expect(result.loading.value).toBe(true)
  })

  it('endLoading sets loading to false', () => {
    const result = useLoadingEmpty(true)
    result.endLoading()
    expect(result.loading.value).toBe(false)
  })

  it('setEmpty sets empty to the given value', () => {
    const result = useLoadingEmpty()
    result.setEmpty(true)
    expect(result.empty.value).toBe(true)
    result.setEmpty(false)
    expect(result.empty.value).toBe(false)
  })

  it('can initialize both loading and empty', () => {
    const result = useLoadingEmpty(true, true)
    expect(result.loading.value).toBe(true)
    expect(result.empty.value).toBe(true)
  })
})
