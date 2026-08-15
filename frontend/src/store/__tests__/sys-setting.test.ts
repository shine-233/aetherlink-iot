import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const hoisted = vi.hoisted(() => ({
  fetchThemeSetting: vi.fn(),
  localStgSet: vi.fn()
}))

vi.mock('@/service/api/setting', () => ({
  fetchThemeSetting: hoisted.fetchThemeSetting
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    set: hoisted.localStgSet
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
    expect(document.querySelector('link[rel="icon"]')?.getAttribute('href')).toBe('http://localhost/uploads/favicon.ico')
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
      home_background: ''
    })
    expect(hoisted.localStgSet).toHaveBeenCalledWith('logoLoading', '')
    expect(hoisted.localStgSet).toHaveBeenCalledWith('systemName', '')
    expect(document.querySelector('link[rel="icon"]')?.getAttribute('href')).toBe('/rdi/logo.png')
  })
})
