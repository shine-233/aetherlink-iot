/**
 * 文件用途：RDI 概览「系统快照」卡片的展示规则（纯函数）。
 * 核心逻辑：状态优先级 告警 > 离线 > 正常(明确无告警) > 在线；安装信息只展示非占位（'--'）字段。
 */
import { $t } from '@/locales'
import type { DeviceSnapshot } from './rdiOverviewState'

export type SnapshotTagType = 'error' | 'default' | 'success' | 'info'

export function snapshotStatusTagType(device: Pick<DeviceSnapshot, 'alarm' | 'online'>): SnapshotTagType {
  if (device.alarm === true) return 'error'
  if (!device.online) return 'default'
  if (device.alarm === false) return 'success'
  return 'info'
}

const STATUS_LABEL_KEY: Record<SnapshotTagType, string> = {
  error: 'custom.devicePage.alarmed',
  default: 'rdi.overview.offline',
  success: 'rdi.overview.normal',
  info: 'rdi.overview.online'
}

export function snapshotStatusLabel(device: Pick<DeviceSnapshot, 'alarm' | 'online'>) {
  return $t(STATUS_LABEL_KEY[snapshotStatusTagType(device)] as any)
}

/** 安装信息字段与其标签键，顺序即卡片展示顺序。 */
export const SNAPSHOT_INSTALLATION_FIELDS = [
  ['serialNumber', 'rdi.overview.serialNumber'],
  ['installDate', 'rdi.overview.installedAt'],
  ['installLocation', 'rdi.overview.installLocation'],
  ['installAddress', 'rdi.overview.installAddress'],
  ['installerName', 'rdi.overview.installer'],
  ['installerContact', 'rdi.overview.installerContact'],
  ['adminName', 'rdi.overview.administrator']
] as const satisfies ReadonlyArray<readonly [keyof DeviceSnapshot, string]>

const isFilled = (value: unknown) => Boolean(value) && value !== '--'

export function snapshotInstallationEntries(device: DeviceSnapshot) {
  return SNAPSHOT_INSTALLATION_FIELDS.filter(([field]) => isFilled(device[field])).map(([field, labelKey]) => ({
    field,
    labelKey,
    value: String(device[field])
  }))
}

export function hasInstallationInfo(device: DeviceSnapshot) {
  return SNAPSHOT_INSTALLATION_FIELDS.some(([field]) => isFilled(device[field]))
}
