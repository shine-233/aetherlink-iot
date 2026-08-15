/**
 * 文件用途：验证 全局状态单元测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia } from 'pinia'
import { nextTick } from 'vue'

const hoisted = vi.hoisted(() => ({
  getToken: vi.fn(),
  localStgGet: vi.fn(),
  localStgSet: vi.fn(),
  savePreferredLanguage: vi.fn(),
  setLocale: vi.fn(),
  setDayjsLocale: vi.fn(),
  messageSuccess: vi.fn(),
  updateGlobalMenusByLocale: vi.fn(),
  updateTabsByLocale: vi.fn(),
  useTitle: vi.fn(),
  sysSettingStore: {
    system_name: 'AetherLink IoT'
  }
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    get: hoisted.localStgGet,
    set: hoisted.localStgSet,
    remove: vi.fn()
  }
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key,
  setLocale: hoisted.setLocale
}))

vi.mock('@/locales/dayjs', () => ({
  setDayjsLocale: hoisted.setDayjsLocale
}))

vi.mock('@/utils/common/discrete', () => ({
  message: {
    success: hoisted.messageSuccess
  }
}))

vi.mock('@/router', () => ({
  router: {
    currentRoute: {
      value: {
        meta: {}
      }
    }
  }
}))

vi.mock('@vueuse/core', async () => {
  const vue = await vi.importActual<typeof import('vue')>('vue')

  return {
    breakpointsTailwind: {},
    useBreakpoints: () => ({
      smaller: () => vue.ref(false)
    }),
    useTitle: hoisted.useTitle
  }
})

vi.mock('@aetherlink/hooks', async () => {
  const vue = await vi.importActual<typeof import('vue')>('vue')

  return {
    useBoolean: (initial = false) => {
      const bool = vue.ref(initial)

      return {
        bool,
        setTrue: () => {
          bool.value = true
        },
        setFalse: () => {
          bool.value = false
        },
        setBool: (value: boolean) => {
          bool.value = value
        },
        toggle: () => {
          bool.value = !bool.value
        }
      }
    }
  }
})

vi.mock('../modules/theme', () => ({
  useThemeStore: () => ({
    setThemeLayout: vi.fn()
  })
}))

vi.mock('../modules/sys-setting', () => ({
  useSysSettingStore: () => hoisted.sysSettingStore
}))

vi.mock('../modules/route', () => ({
  useRouteStore: () => ({
    updateGlobalMenusByLocale: hoisted.updateGlobalMenusByLocale
  })
}))

vi.mock('../modules/tab', () => ({
  useTabStore: () => ({
    updateTabsByLocale: hoisted.updateTabsByLocale
  })
}))

vi.mock('../modules/auth/shared', () => ({
  getToken: hoisted.getToken
}))

vi.mock('@/service/api/personal-center', () => ({
  savePreferredLanguage: hoisted.savePreferredLanguage
}))

import { useAppStore } from '../modules/app'

describe('app store locale persistence', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.sysSettingStore.system_name = 'AetherLink IoT'
    hoisted.localStgGet.mockImplementation((key: string) => {
      if (key === 'lang') return 'en-US'
      return ''
    })
    hoisted.savePreferredLanguage.mockResolvedValue({ error: null })
    hoisted.getToken.mockReturnValue('')
  })

  it('keeps anonymous language switching local without calling the authenticated preference API', async () => {
    const store = useAppStore(createPinia())

    store.changeLocale('zh-CN')
    await vi.dynamicImportSettled()

    expect(hoisted.setLocale).toHaveBeenCalledWith('zh-CN')
    expect(hoisted.localStgSet).toHaveBeenCalledWith('lang', 'zh-CN')
    expect(hoisted.savePreferredLanguage).toHaveBeenCalledTimes(0)
  })

  it('persists the preferred language when an auth token exists', async () => {
    hoisted.getToken.mockReturnValue('token-1')
    const store = useAppStore(createPinia())

    store.changeLocale('fr-FR')
    await vi.dynamicImportSettled()

    expect(hoisted.savePreferredLanguage).toHaveBeenCalledWith({
      prefer_lang: 'fr-FR',
      default_language: 'fr-FR'
    })
  })

  it('skips the authenticated preference API when the caller disables remote persistence', async () => {
    hoisted.getToken.mockReturnValue('token-2')
    const store = useAppStore(createPinia())

    store.changeLocale('zh-CN', { persistRemote: false })
    await vi.dynamicImportSettled()

    expect(hoisted.setLocale).toHaveBeenCalledWith('zh-CN')
    expect(hoisted.localStgSet).toHaveBeenCalledWith('lang', 'zh-CN')
    expect(hoisted.savePreferredLanguage).toHaveBeenCalledTimes(0)
  })

  it('keeps the system title suffix when locale updates the current document title', async () => {
    const routerModule = await import('@/router')
    routerModule.router.currentRoute.value = {
      path: '/management/setting',
      meta: {
        i18nKey: 'route.management_setting'
      }
    } as any

    const store = useAppStore(createPinia())
    store.changeLocale('fr-FR', { persistRemote: false })
    await nextTick()

    expect(hoisted.useTitle).toHaveBeenCalledWith('route.management_setting-AetherLink IoT')
  })
})
