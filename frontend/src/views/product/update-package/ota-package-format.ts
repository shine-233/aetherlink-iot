/**
 * 文件用途: 升级包页面的纯展示格式化工具（时间、类型标签、可选值、文件名、URL 归一化）。
 * 核心逻辑: 列工厂、详情弹窗与页面动作共享同一份展示格式，避免多处拷贝导致口径漂移。
 * 关键注意事项: 只做展示层转换，不改接口载荷；i18n key 与迁移前页面保持一致。
 * 重构建议: 若其它 OTA 页面也需要同类格式化，可上移到 product 共享工具层。
 */
import dayjs from 'dayjs'
import { $t } from '@/locales'

export function formatTime(value?: string) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'
}

export function packageTypeLabel(value?: number) {
  return value === 1 ? $t('page.product.update-package.diff') : $t('page.product.update-package.full')
}

export function normalizePackageUrl(url?: string) {
  if (!url) return ''
  return url.startsWith('./') ? url.slice(1) : url
}

export function formatOptional(value?: string | number | null) {
  if (value === undefined || value === null || value === '') return '-'
  return String(value)
}

export function packageFileName(url?: string | null) {
  if (!url) return '-'
  const normalized = url.replace(/\\/g, '/')
  return normalized.split('/').filter(Boolean).pop() || normalized
}
