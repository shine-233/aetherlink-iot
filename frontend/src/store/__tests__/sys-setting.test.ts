import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const hoisted = vi.hoisted(() => ({
  fetchThemeSetting: vi.fn(),
  fetchWhitelabelOverrides: vi.fn(),
  localStgSet: vi.fn(),
  i18nLocale: { value: 'zh-CN' },
  i18nMessages: { value: {} as Record<string, Record<string, unknown>> },
  setLocaleMessage: vi.fn((locale: string, message: Record<string, unknown>) => {
    hoisted.i18nMessages.value[locale] = message
  })
}))

vi.mock('@/service/api/setting', () => ({
  fetchThemeSetting: hoisted.fetchThemeSetting
}))

vi.mock('@/service/api/whitelabel', () => ({
  fetchWhitelabelOverrides: hoisted.fetchWhitelabelOverrides
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    set: hoisted.localStgSet
  }
}))

// 白标覆盖（TB-47）需要操作 i18n 实例；这里用最小形状替身隔离真实目录加载。
vi.mock('@/locales', () => ({
  i18n: {
    global: {
      locale: hoisted.i18nLocale,
      messages: hoisted.i18nMessages,
      setLocaleMessage: hoisted.setLocaleMessage
    }
  }
}))

vi.mock('~/env.config', () => ({
  createServiceConfig: vi.fn(() => ({
    otherBaseURL: {
      platform: 'http://localhost/api/v1'
    }
  }))
}))

import { useSysSettingStore } from '../modules/sys-setting'

describe('sys-setting store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    document.head.innerHTML = '<link rel="icon" href="/rdi/logo.png">'
    hoisted.i18nLocale.value = 'zh-CN'
    hoisted.i18nMessages.value = {
      'zh-CN': { 'page.customer.title': '客户', 'custom.management.branding': '品牌设置' }
    }
    hoisted.fetchThemeSetting.mockResolvedValue({
      error: null,
      data: {
        list: [
          {
            id: 'branding-1',
            system_name: 'AetherLink IoT',
            logo_cache: '/uploads/favicon.ico',
            logo_background: '/uploads/logo.png',
            logo_loading: '/uploads/loading.png',
            home_background: '/uploads/home.png'
          }
        ]
      }
    })
  })

  it('initSysSetting resolves branding asset urls and updates favicon', async () => {
    const store = useSysSettingStore()

    await store.initSysSetting()

    expect(store.logo_cache).toBe('http://localhost/uploads/favicon.ico')
    expect(store.logo_background).toBe('http://localhost/uploads/logo.png')
    expect(store.logo_loading).toBe('http://localhost/uploads/loading.png')
    expect(store.home_background).toBe('http://localhost/uploads/home.png')
    expect(document.querySelector('link[rel="icon"]')?.getAttribute('href')).toBe(
      'http://localhost/uploads/favicon.ico'
    )
    expect(hoisted.localStgSet).toHaveBeenCalledWith('logoLoading', 'http://localhost/uploads/loading.png')
    expect(hoisted.localStgSet).toHaveBeenCalledWith('systemName', 'AetherLink IoT')
  })

  it('falls back to the default favicon when logo_cache is blank', async () => {
    hoisted.fetchThemeSetting.mockResolvedValue({
      error: null,
      data: {
        list: [
          {
            id: 'branding-1',
            system_name: 'AetherLink IoT',
            logo_cache: '',
            logo_background: '',
            logo_loading: '',
            home_background: ''
          }
        ]
      }
    })
    const store = useSysSettingStore()

    await store.initSysSetting()

    expect(document.querySelector('link[rel="icon"]')?.getAttribute('href')).toBe('/rdi/logo.png')
  })

  it('clears cached systemName when branding title is blank', async () => {
    hoisted.fetchThemeSetting.mockResolvedValue({
      error: null,
      data: {
        list: [
          {
            id: 'branding-1',
            system_name: '   ',
            logo_cache: '',
            logo_background: '',
            logo_loading: '',
            home_background: ''
          }
        ]
      }
    })
    const store = useSysSettingStore()

    await store.initSysSetting()

    expect(store.system_name).toBe('')
    expect(hoisted.localStgSet).toHaveBeenCalledWith('systemName', '')
    expect(document.querySelector('link[rel="icon"]')?.getAttribute('href')).toBe('/rdi/logo.png')
  })

  it('resets branding state when the backend returns no active record', async () => {
    hoisted.fetchThemeSetting.mockResolvedValue({
      error: null,
      data: {
        list: []
      }
    })
    const store = useSysSettingStore()
    store.$patch({
      system_name: 'Old Title',
      logo_cache: 'http://localhost/uploads/old.ico',
      logo_background: 'http://localhost/uploads/old-logo.png',
      logo_loading: 'http://localhost/uploads/old-loading.png',
      home_background: 'http://localhost/uploads/old-home.png'
    })

    await store.initSysSetting()

    expect(store.$state).toEqual({
      system_name: '',
      logo_cache: '',
      logo_background: '',
      logo_loading: '',
      home_background: '',
      theme_color: '',
      favicon: '',
      customCSS: ''
    })
    expect(hoisted.localStgSet).toHaveBeenCalledWith('logoLoading', '')
    expect(hoisted.localStgSet).toHaveBeenCalledWith('systemName', '')
    expect(document.querySelector('link[rel="icon"]')?.getAttribute('href')).toBe('/rdi/logo.png')
  })
})

describe('sys-setting store whitelabel overrides (TB-47)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    document.head.innerHTML = '<link rel="icon" href="/rdi/logo.png">'
    hoisted.i18nLocale.value = 'zh-CN'
    hoisted.i18nMessages.value = {
      'zh-CN': { 'page.customer.title': '客户', 'custom.management.branding': '品牌设置' }
    }
  })

  it('initWhitelabelOverrides 注入自定义 CSS（textContent）并按当前 locale 平铺键合并覆盖', async () => {
    hoisted.fetchWhitelabelOverrides.mockResolvedValue({
      error: null,
      data: {
        tenant_id: 'tenant-a',
        translations: { 'zh-cn': { 'page.customer.title': '客户中心' } },
        css: '.app { color: red; }'
      }
    })
    const store = useSysSettingStore()

    await store.initWhitelabelOverrides()

    expect(store.customCSS).toBe('.app { color: red; }')
    const style = document.getElementById('aetherlink-tenant-custom-css')
    expect(style).not.toBeNull()
    // 主防线断言：CSS 必须以 textContent 落入 style 标签（textContent 属性承载原文）
    expect(style?.textContent).toBe('.app { color: red; }')
    expect(style?.innerHTML).toBe('.app { color: red; }')
    // 翻译覆盖按平铺键合并进当前 locale 目录，且静态键保留
    const merged = hoisted.i18nMessages.value['zh-CN']
    expect(merged['page.customer.title']).toBe('客户中心')
    expect(merged['custom.management.branding']).toBe('品牌设置')
  })

  it('initWhitelabelOverrides 空 CSS 时移除既有 style 标签（清除语义）', async () => {
    hoisted.fetchWhitelabelOverrides.mockResolvedValue({
      error: null,
      data: { tenant_id: 'tenant-a', translations: {}, css: '' }
    })
    const store = useSysSettingStore()
    store.customCSS = '.stale { color: blue; }'
    const stale = document.createElement('style')
    stale.id = 'aetherlink-tenant-custom-css'
    document.head.appendChild(stale)

    await store.initWhitelabelOverrides()

    expect(store.customCSS).toBe('')
    expect(document.getElementById('aetherlink-tenant-custom-css')).toBeNull()
  })

  it('initWhitelabelOverrides 接口失败时保持现状（fail-open，不抛错不注入）', async () => {
    hoisted.fetchWhitelabelOverrides.mockResolvedValue({ error: new Error('boom'), data: null })
    const store = useSysSettingStore()

    await expect(store.initWhitelabelOverrides()).resolves.toBeUndefined()

    expect(store.customCSS).toBe('')
    expect(document.getElementById('aetherlink-tenant-custom-css')).toBeNull()
    expect(hoisted.setLocaleMessage).not.toHaveBeenCalled()
  })

  it('initWhitelabelOverrides 未知 locale（无目录映射）时不合并消息', async () => {
    hoisted.fetchWhitelabelOverrides.mockResolvedValue({
      error: null,
      data: { tenant_id: 'tenant-a', translations: { 'zh-cn': { 'page.x': 'X' } }, css: '' }
    })
    hoisted.i18nLocale.value = 'de-DE'
    const store = useSysSettingStore()

    await store.initWhitelabelOverrides()

    expect(hoisted.setLocaleMessage).not.toHaveBeenCalled()
  })

  it('resetWhitelabelRuntime 清除 CSS 状态并移除 style 标签', async () => {
    const store = useSysSettingStore()
    store.customCSS = '.app { color: red; }'
    const style = document.createElement('style')
    style.id = 'aetherlink-tenant-custom-css'
    style.textContent = '.app { color: red; }'
    document.head.appendChild(style)

    store.resetWhitelabelRuntime()

    expect(store.customCSS).toBe('')
    expect(document.getElementById('aetherlink-tenant-custom-css')).toBeNull()
  })
})
