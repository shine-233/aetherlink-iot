/**
 * 文件用途：验证 请求封装单元测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest'

type TestWindow = Omit<Window, '$message'> & {
  $message: {
    destroyAll: Mock
    error: Mock
  }
}

const hoisted = vi.hoisted(() => {
  const flatRequestFn = vi.fn(() => Promise.resolve({ data: null, error: null }))
  const localStg = {
    get: vi.fn(),
    set: vi.fn(),
    remove: vi.fn()
  }

  return {
    BACKEND_ERROR_CODE: 'BACKEND_ERROR',
    createFlatRequest: vi.fn((_config, options) => Object.assign(flatRequestFn, options) as never),
    flatRequestFn,
    localStg,
    t: vi.fn((key: string) => key),
    refreshAuthToken: vi.fn(),
    scheduleProactiveTokenRefresh: vi.fn(),
    clearAuthStorage: vi.fn()
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

vi.mock('../auth-refresh', () => ({
  AUTH_TOKEN_REFRESHED_EVENT: 'aetherlink:token-refreshed',
  refreshAuthToken: hoisted.refreshAuthToken,
  scheduleProactiveTokenRefresh: hoisted.scheduleProactiveTokenRefresh
}))

vi.mock('@/store/modules/auth/shared', () => ({
  clearAuthStorage: hoisted.clearAuthStorage
}))

type RequestOptions = Parameters<typeof hoisted.createFlatRequest>[1]

const getCapturedOptions = (callIndex: number) => hoisted.createFlatRequest.mock.calls[callIndex]?.[1] as RequestOptions
import { request } from '../request'

const requestOptions = getCapturedOptions(0)

type FakeAxiosInstance = {
  request: Mock
}

/** 替换 window.location，避免测试环境真实导航；返回桩对象供断言 */
function installLocationStub() {
  const locationStub = {
    pathname: '/device/manage',
    search: '?page=1',
    href: 'http://localhost/device/manage?page=1',
    reload: vi.fn()
  }
  Object.defineProperty(window, 'location', { configurable: true, value: locationStub })
  return locationStub
}

describe('service/request/request.ts', () => {
  const originalLocation = window.location

  beforeEach(() => {
    vi.useFakeTimers()
    hoisted.localStg.get.mockImplementation((key: string) => {
      if (key === 'token') return 'token-123'
      if (key === 'lang') return 'zh-CN'
      return null
    })
    hoisted.localStg.remove.mockReset()
    hoisted.localStg.set.mockReset()
    hoisted.t.mockClear()
    hoisted.refreshAuthToken.mockReset()
    hoisted.scheduleProactiveTokenRefresh.mockReset()
    hoisted.clearAuthStorage.mockReset()
    hoisted.flatRequestFn.mockClear()

    ;(window as TestWindow).$message = {
      destroyAll: vi.fn(),
      error: vi.fn()
    }

    installLocationStub()
  })

  afterEach(() => {
    vi.useRealTimers()
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
  })

  it('registers the platform request client with flat-request options attached', () => {
    expect(request).toBe(hoisted.flatRequestFn)
    expect(hoisted.createFlatRequest).toHaveBeenCalledTimes(1)
    expect((request as unknown as { onRequest: unknown }).onRequest).toBe(requestOptions.onRequest)
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
    }

    const originalParams = config.params
    const result = await requestOptions.onRequest(config)

    expect(hoisted.scheduleProactiveTokenRefresh).toHaveBeenCalledTimes(1)
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
    }

    const result = requestOptions.transformBackendResponse(response)

    expect((window as TestWindow).$message.destroyAll).toHaveBeenCalledTimes(1)
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
    }

    const result = requestOptions.transformBackendResponse(response)

    expect((window as TestWindow).$message.destroyAll).toHaveBeenCalledTimes(0)
    expect(result).toEqual({
      code: 200,
      data: { id: 'device-1' },
      message: 'ok'
    })
  })

  it('onError replays the original request once when 40102 refresh succeeds', async () => {
    const locationStub = installLocationStub()
    const replayedResponse = {
      status: 200,
      data: { code: 200, data: { id: 'device-1' }, message: 'ok' }
    }
    const fakeInstance: FakeAxiosInstance = {
      request: vi.fn(async () => replayedResponse)
    }
    hoisted.refreshAuthToken.mockResolvedValueOnce(true)

    const errorConfig = { url: '/device/list', method: 'get', headers: {} }

    await requestOptions.onError(
      { config: errorConfig, response: { status: 401, data: { code: 40102 } } } as never,
      fakeInstance as never
    )

    expect(hoisted.refreshAuthToken).toHaveBeenCalledTimes(1)
    expect(fakeInstance.request).toHaveBeenCalledTimes(1)
    expect(fakeInstance.request.mock.calls[0][0]).toMatchObject({ url: '/device/list', _retry: true })
    // 静默续签成功：不登出、不提示、不跳转
    expect(hoisted.clearAuthStorage).not.toHaveBeenCalled()
    expect((window as TestWindow).$message.error).not.toHaveBeenCalled()
    expect(locationStub.href).toBe('http://localhost/device/manage?page=1')
  })

  it('onError falls back to logout redirect when the silent refresh fails (40102)', async () => {
    installLocationStub()
    const fakeInstance: FakeAxiosInstance = { request: vi.fn() }
    hoisted.refreshAuthToken.mockResolvedValueOnce(false)

    await requestOptions.onError(
      {
        config: { url: '/device/list', method: 'get' },
        response: { status: 401, data: { code: 40102 } }
      } as never,
      fakeInstance as never
    )

    expect(hoisted.refreshAuthToken).toHaveBeenCalledTimes(1)
    // 失败不重放原请求
    expect(fakeInstance.request).not.toHaveBeenCalled()
    expect(hoisted.clearAuthStorage).toHaveBeenCalledTimes(1)
    expect((window as TestWindow).$message.error).toHaveBeenCalledWith('common.loginExpired')
    expect(window.location.href).toBe(`/login?redirect=${encodeURIComponent('/device/manage?page=1')}`)
  })

  it('onError logs out immediately on invalid-token 40101 without attempting a refresh', async () => {
    installLocationStub()
    const fakeInstance: FakeAxiosInstance = { request: vi.fn() }

    await requestOptions.onError(
      {
        config: { url: '/device/list' },
        response: { status: 401, data: { code: 40101 } }
      } as never,
      fakeInstance as never
    )

    expect(hoisted.refreshAuthToken).not.toHaveBeenCalled()
    expect(fakeInstance.request).not.toHaveBeenCalled()
    expect(hoisted.clearAuthStorage).toHaveBeenCalledTimes(1)
    expect((window as TestWindow).$message.error).toHaveBeenCalledWith('common.loginExpired')
    expect(window.location.href).toBe(`/login?redirect=${encodeURIComponent('/device/manage?page=1')}`)
  })

  it('onError does not retry a request that has already been retried once (_retry)', async () => {
    installLocationStub()
    const fakeInstance: FakeAxiosInstance = { request: vi.fn() }

    await requestOptions.onError(
      {
        config: { url: '/device/list', _retry: true },
        response: { status: 401, data: { code: 40102 } }
      } as never,
      fakeInstance as never
    )

    expect(hoisted.refreshAuthToken).not.toHaveBeenCalled()
    expect(fakeInstance.request).not.toHaveBeenCalled()
    expect(hoisted.clearAuthStorage).toHaveBeenCalledTimes(1)
    expect(window.location.href).toBe(`/login?redirect=${encodeURIComponent('/device/manage?page=1')}`)
  })

  it('onError returns silently when silentError is enabled', async () => {
    const result = await requestOptions.onError({
      message: 'hidden failure',
      config: {
        silentError: true
      }
    })

    expect(result).toBeUndefined()
    expect((window as TestWindow).$message.error).toHaveBeenCalledTimes(0)
  })

  it('onError maps 404 to the localized resource-not-found message', async () => {
    await requestOptions.onError({
      message: 'Not Found',
      response: {
        status: 404
      }
    })

    expect((window as TestWindow).$message.destroyAll).toHaveBeenCalledTimes(1)
    expect((window as TestWindow).$message.error).toHaveBeenCalledWith('common.resourceNotFound')
  })

  it('onError maps Axios network failures without a response to the localized network message', async () => {
    await requestOptions.onError({
      message: 'Network Error',
      code: 'ERR_NETWORK'
    })

    expect((window as TestWindow).$message.destroyAll).toHaveBeenCalledTimes(1)
    expect((window as TestWindow).$message.error).toHaveBeenCalledWith('common.networkError')
  })

  it('onError does not classify an HTTP response as an offline network failure', async () => {
    await requestOptions.onError({
      message: 'Gateway unavailable',
      code: 'ERR_NETWORK',
      response: {
        status: 503
      }
    })

    expect((window as TestWindow).$message.destroyAll).toHaveBeenCalledTimes(1)
    expect((window as TestWindow).$message.error).toHaveBeenCalledWith('Gateway unavailable')
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
    })

    expect((window as TestWindow).$message.destroyAll).toHaveBeenCalledTimes(1)
    expect((window as TestWindow).$message.error).toHaveBeenCalledWith('backend says no')
  })
})
