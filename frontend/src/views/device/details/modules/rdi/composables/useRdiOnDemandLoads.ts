/**
 * 文件用途: RDI 详情视图按需加载与设备切换编排 composable。
 * 核心逻辑: 能耗统计与 OTA 固件包首次按需加载；挂载与设备切换时统一触发"配置 -> 实时状态 -> 能耗"刷新链路。
 * 关键注意事项: 设备切换后需先复位分享状态与在线缓存，再重新启动加载与轮询链路，避免旧设备状态残留。
 * 重构建议: 新增懒加载数据块时复用 hasXxxLoaded 标记模式，保持首屏不请求重数据。
 */
import { onMounted, ref, watch } from 'vue'
import type { Ref } from 'vue'

type UseRdiOnDemandLoadsOptions = {
  deviceId: () => string
  loadConfig: () => Promise<unknown>
  loadRealtimeState: () => Promise<unknown>
  loadEnergyStatistics: () => Promise<unknown>
  loadOtaPackages: () => Promise<unknown>
  otaPackageLoading: Ref<boolean>
  resetShareState: () => unknown
  liveOnlineStatus: Ref<number | null>
  startTelemetryRefresh: () => void
}

export function useRdiOnDemandLoads(options: UseRdiOnDemandLoadsOptions) {
  const hasLoadedEnergyStatistics = ref(false)
  const hasRequestedOtaPackages = ref(false)

  async function loadEnergyStatisticsOnDemand() {
    hasLoadedEnergyStatistics.value = true
    await options.loadEnergyStatistics()
  }

  async function ensureOtaPackagesLoaded() {
    if (hasRequestedOtaPackages.value || options.otaPackageLoading.value) return

    hasRequestedOtaPackages.value = true
    await options.loadOtaPackages()
  }

  async function reloadOtaPackages() {
    hasRequestedOtaPackages.value = true
    await options.loadOtaPackages()
  }

  async function loadConfigAndRefresh() {
    await options.loadConfig()
    await options.loadRealtimeState()
    hasLoadedEnergyStatistics.value = true
    await options.loadEnergyStatistics()
  }

  onMounted(() => {
    loadConfigAndRefresh()
    options.startTelemetryRefresh()
  })

  watch(
    options.deviceId,
    () => {
      options.resetShareState()
      options.liveOnlineStatus.value = null
      hasLoadedEnergyStatistics.value = false
      loadConfigAndRefresh()
      options.startTelemetryRefresh()
    }
  )

  return {
    hasLoadedEnergyStatistics,
    loadEnergyStatisticsOnDemand,
    ensureOtaPackagesLoaded,
    reloadOtaPackages,
    loadConfigAndRefresh
  }
}
