/**
 * 文件用途：定义 系统设置状态模块 的 Pinia 状态模块。
 * 核心逻辑：维护模块状态、计算属性和动作，并把状态变化暴露给页面、组件和路由流程。
 * 关键注意事项：状态字段、持久化键和跨模块调用属于前端契约，调整时需要同步测试与调用方。
 * 重构建议：可将副作用、接口访问和纯状态推导拆分，降低 store 文件复杂度。
 */
import { defineStore } from 'pinia'
import { fetchThemeSetting } from '@/service/api/setting'
import { localStg } from '@/utils/storage'
import { createServiceConfig } from '~/env.config'

const { otherBaseURL } = createServiceConfig(import.meta.env)
const platformApiUrl = new URL(otherBaseURL.platform ? otherBaseURL.platform : `${window.location.origin}/api/v1`)
const defaultFavicon = '/rdi/logo.png'

type SysSetting = Omit<Api.GeneralSetting.ThemeSetting, 'id'>

const emptySysSetting: SysSetting = {
  system_name: '',
  logo_background: '',
  logo_loading: '',
  logo_cache: '',
  home_background: ''
}

function resolveAssetUrl(value?: string | null) {
  const path = String(value || '').trim()
  if (!path) return ''
  if (/^(https?:)?\/\//i.test(path) || path.startsWith('data:')) return path
  return `${platformApiUrl.origin}${path.startsWith('/') ? path : `/${path}`}`
}

function applyFavicon(url?: string | null) {
  if (typeof document === 'undefined') return

  const href = String(url || '').trim() || defaultFavicon
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  link.href = href
}

function normalizeSetting(setting?: Partial<Api.GeneralSetting.ThemeSetting> | null): SysSetting {
  return {
    system_name: String(setting?.system_name || '').trim(),
    logo_background: resolveAssetUrl(setting?.logo_background),
    logo_loading: resolveAssetUrl(setting?.logo_loading),
    logo_cache: resolveAssetUrl(setting?.logo_cache),
    home_background: resolveAssetUrl(setting?.home_background)
  }
}

function syncBrandingRuntime(setting: SysSetting) {
  applyFavicon(setting.logo_cache)
  localStg.set('logoLoading', setting.logo_loading || '')
  localStg.set('systemName', setting.system_name || '')
}

export const useSysSettingStore = defineStore('sys-setting', {
  state: (): SysSetting => ({ ...emptySysSetting }),
  actions: {
    async initSysSetting() {
      const { error, data } = await fetchThemeSetting()
      if (!error && data) {
        const list: Api.GeneralSetting.ThemeSetting[] = data.list
        const setting = normalizeSetting(list[0] || emptySysSetting)
        syncBrandingRuntime(setting)
        Object.assign(this.$state, setting)
      }
    }
  }
})
