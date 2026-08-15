/**
 * 文件用途：验证 请求封装单元测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => {
  const createFlatRequest = vi.fn((_config, options) => options as never)
  const localStg = {
    get: vi.fn(),
    remove: vi.fn()
  }

  return {
    BACKEND_ERROR_CODE: 'BACKEND_ERROR',
    createFlatRequest,
    localStg,
    t: vi.fn((key: string) => key)
  }
})

vi.mock('@aetherlink/axios', () => ({
  BACKEND_ERROR_CODE: hoisted.BACKEND_ERROR_CODE,
  createFlatRequest: hoisted.createFlatRequest
}))

vi.mock('@/utils/storage', () => ({
  localStg: hoisted.localStg
}))

vi.mock('@/locales', () => ({
  $t: hoisted.t
}))

vi.mock('~/env.config', () => ({
  createProxyPattern: vi.fn(() => '/proxy-api'),
  createServiceConfig: vi.fn(() => ({
    otherBaseURL: {
      platform: 'https://platform.example/api'
    }
  }))
}))

type RequestOptions = Parameters<typeof hoisted.createFlatRequest>[1]

const getCapturedOptions = (callIndex: number) => hoisted.createFlatRequest.mock.calls[callIndex]?.[1] as RequestOptions
import { request } from '../request'

const requestOptions = getCapturedOptions(0)

describe('service/request/request.ts', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    hoisted.localStg.get.mockImplementation((key: string) => {
      if (key === 'token') return 'token-123'
      if (key === 'lang') return 'zh-CN'
      return null
    })
    hoisted.localStg.remove.mockReset()
    hoisted.t.mockClear()

    ;(window as any).$message = {
      destroyAll: vi.fn(),
      error: vi.fn()
    }
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('registers the platform request client', () => {
    expect(request).toBe(requestOptions)
    expect(hoisted.createFlatRequest).toHaveBeenCalledTimes(1)
  })

  it('request onRequest injects token and language headers and clears empty params', async () => {
    const config = {
      headers: {},
      params: {
        emptyValue: '',
        keepZero: 0,
        keepFalse: false,
        keepText: 'abc'
      }
    } as any

    const originalParams = config.params
    const result = await requestOptions.onRequest(config)

    expect(result.headers).toMatchObject({
      'x-token': 'token-123',
      'Accept-Language': 'zh-CN'
    })
    expect(result.params).not.toBe(originalParams)
    expect(result.params).toEqual({
      emptyValue: undefined,
      keepZero: 0,
      keepFalse: false,
      keepText: 'abc'
    })
    expect(originalParams).toEqual({
      emptyValue: '',
      keepZero: 0,
      keepFalse: false,
      keepText: 'abc'
    })
  })

  it('transformBackendResponse returns nested data by default and destroys messages for non-GET requests', () => {
    const response = {
      config: {
        method: 'post',
        needMessage: false
      },
      data: {
        code: 200,
        data: { id: 'device-1' },
        message: 'ok'
      }
    } as any

    const result = requestOptions.transformBackendResponse(response)

    expect((window as any).$message.destroyAll).toHaveBeenCalledTimes(1)
    expect(result).toEqual({ id: 'device-1' })
  })

  it('transformBackendResponse returns the full backend payload when needMessage is enabled', () => {
    const response = {
      config: {
        method: 'get',
        needMessage: true
      },
      data: {
        code: 200,
        data: { id: 'device-1' },
        message: 'ok'
      }
    } as any

    const result = requestOptions.transformBackendResponse(response)

    expect((window as any).$message.destroyAll).toHaveBeenCalledTimes(0)
    expect(result).toEqual({
      code: 200,
      data: { id: 'device-1' },
      message: 'ok'
    })
  })

  it('onError handles 401 by notifying, clearing auth state, and reloading', async () => {
    const reloadSpy = vi.spyOn(window.location, 'reload').mockImplementation(() => undefined)

    await requestOptions.onError({
      response: { status: 401 }
    } as any)

    expect((window as any).$message.destroyAll).toHaveBeenCalledTimes(1)
    expect((window as any).$message.error).toHaveBeenCalledWith('common.loginExpired')
    expect(hoisted.localStg.remove).toHaveBeenCalledTimes(0)

    vi.advanceTimersByTime(1000)

    expect(hoisted.localStg.remove).toHaveBeenNthCalledWith(1, 'token')
    expect(hoisted.localStg.remove).toHaveBeenNthCalledWith(2, 'userInfo')
    expect(hoisted.localStg.remove).toHaveBeenNthCalledWith(3, 'token_expires_in')
    expect(reloadSpy).toHaveBeenCalledTimes(1)
  })

  it('onError returns silently when silentError is enabled', async () => {
    const result = await requestOptions.onError({
      message: 'hidden failure',
      config: {
        silentError: true
      }
    } as any)

    expect(result).toBeUndefined()
    expect((window as any).$message.error).toHaveBeenCalledTimes(0)
  })

  it('onError maps 404 to the localized resource-not-found message', async () => {
    await requestOptions.onError({
      message: 'Not Found',
      response: {
        status: 404
      }
    } as any)

    expect((window as any).$message.destroyAll).toHaveBeenCalledTimes(1)
    expect((window as any).$message.error).toHaveBeenCalledWith('common.resourceNotFound')
  })

  it('onError maps Axios network failures without a response to the localized network message', async () => {
    await requestOptions.onError({
      message: 'Network Error',
      code: 'ERR_NETWORK'
    } as any)

    expect((window as any).$message.destroyAll).toHaveBeenCalledTimes(1)
    expect((window as any).$message.error).toHaveBeenCalledWith('common.networkError')
  })

  it('onError does not classify an HTTP response as an offline network failure', async () => {
    await requestOptions.onError({
      message: 'Gateway unavailable',
      code: 'ERR_NETWORK',
      response: {
        status: 503
      }
    } as any)

    expect((window as any).$message.destroyAll).toHaveBeenCalledTimes(1)
    expect((window as any).$message.error).toHaveBeenCalledWith('Gateway unavailable')
  })

  it('onError prefers backend message text for backend errors', async () => {
    await requestOptions.onError({
      message: 'fallback message',
      code: hoisted.BACKEND_ERROR_CODE,
      response: {
        data: {
          message: 'backend says no'
        }
      }
    } as any)

    expect((window as any).$message.destroyAll).toHaveBeenCalledTimes(1)
    expect((window as any).$message.error).toHaveBeenCalledWith('backend says no')
  })
})
