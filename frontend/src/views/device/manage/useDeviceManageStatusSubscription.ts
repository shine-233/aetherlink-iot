import { onUnmounted, type Ref } from 'vue'
import { useDeviceStatusWebSocket } from '@/utils/deviceStatusWebSocket'

type DeviceStatusRow = {
  id?: string
  is_online?: 0 | 1
}

type TablePageRef = Ref<
  | {
      dataList?: DeviceStatusRow[]
    }
  | undefined
>

type Logger = {
  error: (message: string, context?: Record<string, unknown>) => void
}

type UseDeviceManageStatusSubscriptionOptions = {
  tablePageRef: TablePageRef
  logger: Logger
  subscribeDelayMs?: number
}

const DEFAULT_SUBSCRIBE_DELAY_MS = 100

export function useDeviceManageStatusSubscription(options: UseDeviceManageStatusSubscriptionOptions) {
  const deviceStatusWS = useDeviceStatusWebSocket()
  let deviceStatusSubscribeTimer: ReturnType<typeof setTimeout> | undefined
  const subscribeDelayMs = options.subscribeDelayMs ?? DEFAULT_SUBSCRIBE_DELAY_MS

  const collectVisibleDeviceIds = () => {
    const deviceIds = options.tablePageRef.value?.dataList?.map((device) => device.id).filter(Boolean) || []
    return Array.from(new Set(deviceIds)) as string[]
  }

  const updateDeviceStatusInTable = (deviceId: string, isOnline: boolean) => {
    try {
      const rows = options.tablePageRef.value?.dataList
      if (!Array.isArray(rows)) return

      const device = rows.find((item) => item.id === deviceId)
      if (device) {
        device.is_online = isOnline ? 1 : 0
      }
    } catch (error) {
      options.logger.error('[DeviceManage] 更新设备状态失败:', {
        deviceId,
        isOnline,
        error: error instanceof Error ? error.message : error
      })
    }
  }

  const subscribeDeviceStatus = () => {
    const deviceIds = collectVisibleDeviceIds()

    if (deviceIds.length > 0) {
      deviceStatusWS.updateSubscription(deviceIds, updateDeviceStatusInTable)
    } else {
      deviceStatusWS.disconnect()
    }
  }

  const clearDeviceStatusSubscriptionTimer = () => {
    if (!deviceStatusSubscribeTimer) return

    clearTimeout(deviceStatusSubscribeTimer)
    deviceStatusSubscribeTimer = undefined
  }

  const scheduleDeviceStatusSubscription = () => {
    clearDeviceStatusSubscriptionTimer()

    deviceStatusSubscribeTimer = setTimeout(() => {
      deviceStatusSubscribeTimer = undefined
      subscribeDeviceStatus()
    }, subscribeDelayMs)
  }

  const stopDeviceStatusSubscription = () => {
    clearDeviceStatusSubscriptionTimer()
    deviceStatusWS.disconnect()
  }

  onUnmounted(stopDeviceStatusSubscription)

  return {
    scheduleDeviceStatusSubscription,
    stopDeviceStatusSubscription
  }
}
