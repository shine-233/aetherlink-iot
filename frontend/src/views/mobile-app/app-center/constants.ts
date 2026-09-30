/*
 * 文件用途：应用包中心共享常量与展示 helper（列表、筛选、上传弹窗共用）。
 * 从 index.vue 拆出，保持 i18n key 与取值口径不变。
 */
import { $t } from '@/locales'
import type { SelectOption } from 'naive-ui'
import type { MobileAppBundlePlatform, MobileAppBundleStatus } from '@/service/api'

export const PLATFORM_OPTIONS: SelectOption[] = [
  { label: $t('page.app_bundle.platformAndroid'), value: 'android' },
  { label: $t('page.app_bundle.platformIos'), value: 'ios' },
  { label: $t('page.app_bundle.platformH5'), value: 'h5' }
]

export const STATUS_OPTIONS: SelectOption[] = [
  { label: $t('page.app_bundle.statusDraft'), value: 'draft' },
  { label: $t('page.app_bundle.statusPublished'), value: 'published' },
  { label: $t('page.app_bundle.statusArchived'), value: 'archived' }
]

/** naive-ui 上传组件的 accept 过滤：与后端按平台校验的扩展名（apk/ipa/zip）保持一致。 */
export const ACCEPT_BY_PLATFORM: Record<MobileAppBundlePlatform, string> = {
  android: '.apk',
  ios: '.ipa',
  h5: '.zip'
}

export const statusTypeOf = (status: MobileAppBundleStatus) => {
  switch (status) {
    case 'published':
      return 'success' as const
    case 'archived':
      return 'default' as const
    default:
      return 'warning' as const
  }
}

export const statusTextOf = (status: MobileAppBundleStatus) => {
  switch (status) {
    case 'published':
      return $t('page.app_bundle.statusPublished')
    case 'archived':
      return $t('page.app_bundle.statusArchived')
    default:
      return $t('page.app_bundle.statusDraft')
  }
}

export const platformTextOf = (platform: MobileAppBundlePlatform) => {
  switch (platform) {
    case 'android':
      return $t('page.app_bundle.platformAndroid')
    case 'ios':
      return $t('page.app_bundle.platformIos')
    default:
      return $t('page.app_bundle.platformH5')
  }
}

/** 字节数可读格式（与媒体库口径一致的局部展示 helper）。 */
export const formatBytes = (size: number) => {
  if (!Number.isFinite(size) || size <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  return `${value.toFixed(value >= 100 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`
}
