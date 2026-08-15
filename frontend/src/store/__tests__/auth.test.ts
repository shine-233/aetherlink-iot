/**
 * 文件用途：验证 全局状态单元测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia } from 'pinia'

const hoisted = vi.hoisted(() => ({
  fetchLogin: vi.fn(),
  fetchGetUserInfo: vi.fn(),
  logout: vi.fn(),
  transformUser: vi.fn(),
  getToken: vi.fn(),
  getUserInfo: vi.fn(),
  clearAuthStorage: vi.fn(),
  clearThingsVisToken: vi.fn(),
  encryptDataByRsa: vi.fn((value: string) => value),
  generateRandomHexString: vi.fn(() => 'salt-value'),
  validPassword: vi.fn(),
  localStgSet: vi.fn(),
  localStgGet: vi.fn(),
  toLogin: vi.fn(),
  redirectFromLogin: vi.fn(),
  initAuthRoute: vi.fn(),
  resetRouteStore: vi.fn(),
  clearTabs: vi.fn()
}))

vi.mock('@/service/api', () => ({
  fetchLogin: hoisted.fetchLogin,
  fetchGetUserInfo: hoisted.fetchGetUserInfo,
  logout: hoisted.logout
}))

vi.mock('@/service/api/auth', () => ({
  transformUser: hoisted.transformUser
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    set: hoisted.localStgSet,
    get: hoisted.localStgGet,
    remove: vi.fn()
  }
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({
    error: vi.fn(),
    info: vi.fn(),
    warn: vi.fn()
  })
}))

vi.mock('@/utils/common/tool', () => ({
  generateRandomHexString: hoisted.generateRandomHexString,
  validPassword: hoisted.validPassword
}))

vi.mock('@/utils/security/rsa-encrypt', () => ({
  encryptDataByRsa: hoisted.encryptDataByRsa
}))

vi.mock('@/utils/thingsvis', () => ({
  clearThingsVisToken: hoisted.clearThingsVisToken
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({
    route: { value: { meta: { constant: false } } },
    toLogin: hoisted.toLogin,
    redirectFromLogin: hoisted.redirectFromLogin
  })
}))

vi.mock('../modules/route', () => ({
  useRouteStore: () => ({
    initAuthRoute: hoisted.initAuthRoute,
    resetStore: hoisted.resetRouteStore,
    isInitAuthRoute: true
  })
}))

vi.mock('../modules/tab', () => ({
  useTabStore: () => ({
    clearTabs: hoisted.clearTabs,
    initHomeTab: vi.fn()
  })
}))

vi.mock('../modules/auth/shared', () => ({
  getToken: hoisted.getToken,
  getUserInfo: hoisted.getUserInfo,
  clearAuthStorage: hoisted.clearAuthStorage
}))

vi.mock('@aetherlink/hooks', () => ({
  useLoading: () => ({
    loading: { value: false },
    startLoading: vi.fn(),
    endLoading: vi.fn()
  }),
  useContext: vi.fn(() => ({
    setupStore: vi.fn(),
    useStore: vi.fn()
  }))
}))

import { useAuthStore } from '../modules/auth'

describe('auth store', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    vi.clearAllMocks()
    pinia = createPinia()
    hoisted.getToken.mockReturnValue('')
    hoisted.getUserInfo.mockReturnValue({
      authority: '',
      id: '',
      userId: '',
      userName: '',
      roles: []
    })
    hoisted.validPassword.mockReturnValue(true)
    hoisted.toLogin.mockResolvedValue(undefined)
    hoisted.redirectFromLogin.mockResolvedValue(undefined)
    hoisted.initAuthRoute.mockResolvedValue(undefined)
    hoisted.resetRouteStore.mockResolvedValue(undefined)
    hoisted.clearTabs.mockReset()
    localStorage.clear()
  })

  it('derives token and login state from shared auth storage', () => {
    hoisted.getToken.mockReturnValue('test-token')

    const store = useAuthStore(pinia)

    expect(store.token).toBe('test-token')
    expect(store.isLogin).toBe(true)
  })

  it('stores token and user info on successful loginByToken', async () => {
    hoisted.fetchGetUserInfo.mockResolvedValue({
      data: {
        authority: 'ADMIN',
        id: 'user-1',
        userId: 'user-1',
        userName: 'Test User',
        name: 'Test User'
      },
      error: null
    })

    const store = useAuthStore(pinia)
    const result = await store.loginByToken({
      token: 'new-token',
      expires_in: 3600
    })

    expect(hoisted.localStgSet).toHaveBeenCalledWith('token', 'new-token')
    expect(hoisted.localStgSet).toHaveBeenCalledWith('token_expires_in', expect.any(String))
    expect(hoisted.localStgSet).toHaveBeenCalledWith(
      'userInfo',
      expect.objectContaining({ authority: 'ADMIN', roles: ['ADMIN'] })
    )
    expect(hoisted.clearThingsVisToken).toHaveBeenCalledTimes(1)
    expect(result.loop).toBe(true)
    expect(store.token).toBe('new-token')
  })

  it('returns loop false when loginByToken cannot fetch user info', async () => {
    hoisted.fetchGetUserInfo.mockResolvedValue({
      data: null,
      error: { message: 'Unauthorized' }
    })

    const store = useAuthStore(pinia)
    const result = await store.loginByToken({
      token: 'bad-token',
      expires_in: 3600
    })

    expect(result.loop).toBe(false)
  })

  it('logs in with plain password when frontend encryption is disabled', async () => {
    hoisted.fetchLogin.mockResolvedValue({
      data: { token: 'login-token', expires_in: 3600 }
    })
    hoisted.fetchGetUserInfo.mockResolvedValue({
      data: {
        authority: 'ADMIN',
        id: 'user-1',
        userId: 'user-1',
        userName: 'Test User',
        name: 'Test User',
        password_last_updated: new Date().toISOString()
      },
      error: null
    })

    const store = useAuthStore(pinia)
    await store.login('testuser', 'password123')
    await vi.dynamicImportSettled()

    expect(hoisted.fetchLogin).toHaveBeenCalledWith('testuser', 'password123', null)
    expect(hoisted.initAuthRoute).toHaveBeenCalledTimes(1)
    expect(hoisted.redirectFromLogin).toHaveBeenCalledTimes(1)
  })

  it('encrypts password when frontend encryption is enabled', async () => {
    localStorage.setItem('enableZcAndYzm', JSON.stringify([{ name: 'frontend_res', enable_flag: 'enable' }]))
    hoisted.encryptDataByRsa.mockReturnValueOnce('encrypted-password')
    hoisted.fetchLogin.mockResolvedValue({
      data: { token: 'login-token', expires_in: 3600 }
    })
    hoisted.fetchGetUserInfo.mockResolvedValue({
      data: {
        authority: 'ADMIN',
        id: 'user-1',
        userId: 'user-1',
        userName: 'Test User',
        name: 'Test User',
        password_last_updated: new Date().toISOString()
      },
      error: null
    })

    const store = useAuthStore(pinia)
    await store.login('testuser', 'password123')

    expect(hoisted.generateRandomHexString).toHaveBeenCalledWith(16)
    expect(hoisted.encryptDataByRsa).toHaveBeenCalledWith('password123salt-value')
    expect(hoisted.fetchLogin).toHaveBeenCalledWith('testuser', 'encrypted-password', 'salt-value')
  })

  it.each(['{broken-json', 'null', '{}', '[null]', '[{"name":123,"enable_flag":"enable"}]'])(
    'fails safe for invalid frontend encryption config %s',
    async rawConfig => {
      localStorage.setItem('enableZcAndYzm', rawConfig)
      hoisted.fetchLogin.mockResolvedValue({ data: null })

      const store = useAuthStore(pinia)
      await store.login('testuser', 'password123')

      expect(hoisted.generateRandomHexString).not.toHaveBeenCalled()
      expect(hoisted.encryptDataByRsa).not.toHaveBeenCalled()
      expect(hoisted.fetchLogin).toHaveBeenCalledWith('testuser', 'password123', null)
    }
  )

  it('resets store when login returns no token', async () => {
    hoisted.fetchLogin.mockResolvedValue({ data: null })

    const store = useAuthStore(pinia)
    await store.login('testuser', 'wrong-password')
    await vi.dynamicImportSettled()

    expect(hoisted.clearAuthStorage).toHaveBeenCalledTimes(1)
    expect(hoisted.clearThingsVisToken).toHaveBeenCalledTimes(1)
  })

  it('resets store when login throws', async () => {
    hoisted.fetchLogin.mockRejectedValue(new Error('Network error'))

    const store = useAuthStore(pinia)
    await store.login('testuser', 'password123')
    await vi.dynamicImportSettled()

    expect(hoisted.clearAuthStorage).toHaveBeenCalledTimes(1)
  })

  it('transforms user and clears tabs on enter success', async () => {
    hoisted.transformUser.mockResolvedValue({
      data: { token: 'enter-token', expires_in: 3600 },
      error: null
    })
    hoisted.fetchGetUserInfo.mockResolvedValue({
      data: {
        authority: 'ADMIN',
        id: 'user-2',
        userId: 'user-2',
        userName: 'Entered User',
        name: 'Entered User'
      },
      error: null
    })

    const store = useAuthStore(pinia)
    await store.enter('user-2')
    await vi.dynamicImportSettled()

    expect(hoisted.transformUser).toHaveBeenCalledWith({ become_user_id: 'user-2' })
    expect(hoisted.clearTabs).toHaveBeenCalledTimes(1)
    expect(hoisted.initAuthRoute).toHaveBeenCalledTimes(1)
    expect(hoisted.redirectFromLogin).toHaveBeenCalledTimes(1)
  })

  it('resets store when transformUser returns error', async () => {
    hoisted.transformUser.mockResolvedValue({
      data: null,
      error: { message: 'Forbidden' }
    })

    const store = useAuthStore(pinia)
    await store.enter('user-2')
    await vi.dynamicImportSettled()

    expect(hoisted.clearAuthStorage).toHaveBeenCalledTimes(1)
  })

  it('waits for auth state reset before requestLogout resolves', async () => {
    hoisted.logout.mockResolvedValue({})
    let finishRouteReset!: () => void
    hoisted.resetRouteStore.mockImplementationOnce(
      () =>
        new Promise<void>(resolve => {
          finishRouteReset = resolve
        })
    )

    const store = useAuthStore(pinia)
    let logoutResolved = false
    const logoutPromise = store.requestLogout().then(() => {
      logoutResolved = true
    })

    await vi.waitFor(() => expect(hoisted.resetRouteStore).toHaveBeenCalledTimes(1))
    expect(logoutResolved).toBe(false)

    finishRouteReset()
    await logoutPromise

    expect(hoisted.logout).toHaveBeenCalledTimes(1)
    expect(hoisted.clearAuthStorage).toHaveBeenCalledTimes(1)
    expect(logoutResolved).toBe(true)
  })

  it('clears auth storage and thingsvis token on resetStore', async () => {
    const store = useAuthStore(pinia)
    await store.resetStore()
    await vi.dynamicImportSettled()

    expect(hoisted.clearAuthStorage).toHaveBeenCalledTimes(1)
    expect(hoisted.clearThingsVisToken).toHaveBeenCalledTimes(1)
    expect(hoisted.toLogin).toHaveBeenCalledTimes(1)
    expect(hoisted.resetRouteStore).toHaveBeenCalledTimes(1)
  })
})
