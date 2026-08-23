/**
 * 文件用途: RDI 设备基础信息展示 composable。
 * 核心逻辑: 归一化设备名称、编号、安装时间与心跳时间等元数据，推导在线状态并输出双列基础信息卡片配置。
 * 关键注意事项: 时间格式化需兼容 ISO 与 "YYYY-MM-DD HH:mm:ss" 两种输入；缺失值统一回退为 "--"。
 * 重构建议: 新增基础信息字段时优先扩展 basicInfoColumns，避免在模板中散落格式化判断。
 */
import { computed } from 'vue'
import dayjs from 'dayjs'
import type { ComputedRef, Ref } from 'vue'
import type { LabelKey } from '../constants/rdi-labels'

export type BasicInfoItem = {
  key: string
  label: string
  value: string
  kind?: 'status' | 'chip'
}

type UseRdiDeviceBasicInfoOptions = {
  deviceId: () => string
  online: () => number | undefined
  onlineUpdatedAt: () => string | undefined
  deviceData: () => Record<string, any> | undefined
  liveOnlineStatus: Ref<number | null>
  deviceOnlineText: ComputedRef<string>
  deviceDescriptionText: ComputedRef<string>
  t: (key: LabelKey) => string
}

export function useRdiDeviceBasicInfo(options: UseRdiDeviceBasicInfoOptions) {
  const { t, liveOnlineStatus, deviceOnlineText, deviceDescriptionText } = options

  function toDisplayText(value: unknown) {
    if (value === null || value === undefined) return '--'
    const text = String(value).trim()
    return text || '--'
  }

  function formatDeviceMetaTime(value: unknown) {
    const text = toDisplayText(value)
    if (text === '--') return text
    const isoLikeMatch = text.match(/^(\d{4}-\d{2}-\d{2})[T\s](\d{2}:\d{2}:\d{2})/)
    if (isoLikeMatch) return `${isoLikeMatch[1]} ${isoLikeMatch[2]}`
    const parsed = dayjs(text)
    return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : text
  }

  const deviceData = computed(options.deviceData)

  const deviceNameText = computed(() => toDisplayText(deviceData.value?.name || deviceData.value?.device_name))
  const deviceIdentifierText = computed(() => toDisplayText(deviceData.value?.device_number || options.deviceId()))
  const deviceAddedAtText = computed(() =>
    formatDeviceMetaTime(
      deviceData.value?.created_at || deviceData.value?.create_time || deviceData.value?.createdAt
    )
  )
  const deviceLastHeartbeatText = computed(() =>
    formatDeviceMetaTime(
      options.onlineUpdatedAt() ||
        deviceData.value?.last_heartbeat ||
        deviceData.value?.lastHeartbeat ||
        deviceData.value?.update_at ||
        deviceData.value?.updated_at
    )
  )
  const isDeviceOnline = computed(() => {
    const status = options.online() ?? deviceData.value?.is_online ?? liveOnlineStatus.value
    return status === 1 || status === true
  })

  const basicInfoColumns = computed<BasicInfoItem[][]>(() => [
    [
      { key: 'status', label: t('statusLabel'), value: deviceOnlineText.value, kind: 'status' },
      { key: 'deviceId', label: t('deviceId'), value: deviceIdentifierText.value, kind: 'chip' },
      { key: 'description', label: t('description'), value: deviceDescriptionText.value },
      { key: 'addedAt', label: t('addedAt'), value: deviceAddedAtText.value }
    ],
    [
      { key: 'deviceName', label: t('deviceName'), value: deviceNameText.value },
      { key: 'firmware', label: t('firmware'), value: toDisplayText(deviceData.value?.current_version) },
      { key: 'lastHeartbeat', label: t('lastHeartbeat'), value: deviceLastHeartbeatText.value }
    ]
  ])

  return {
    deviceNameText,
    deviceIdentifierText,
    deviceAddedAtText,
    deviceLastHeartbeatText,
    isDeviceOnline,
    basicInfoColumns
  }
}
