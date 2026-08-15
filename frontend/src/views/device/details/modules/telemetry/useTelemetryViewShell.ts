/**
 * 文件用途：承接遥测页面中纯展示壳层的常量和平台判断。
 * 核心逻辑：集中卡片尺寸、筛选/排序选项文案和平台宽度判断，避免这些展示状态堆回 `telemetry.vue`。
 * 关键注意事项：这里故意不接入日志、控制项、实时流或弹窗副作用，只服务页面展示层。
 * 重构建议：后续若卡片头部图标、时间展示或布局断点继续增长，可继续沿该壳层 helper 下沉。
 */
import { computed, getCurrentInstance, ref } from 'vue'
import { TELEMETRY_CARD_FRESHNESS_FILTER, TELEMETRY_CARD_SORT_MODE } from './telemetryCardViewState'

type TelemetrySortOption = {
  label: string
  value: string
}

type UseTelemetryViewShellOptions = {
  translate: (key: string) => string
}

export const useTelemetryViewShell = ({ translate }: UseTelemetryViewShellOptions) => {
  const nowTime = ref<unknown>()
  const cardHeight = ref(160)
  const cardMargin = ref(15)

  const telemetrySortOptions = computed<TelemetrySortOption[]>(() => [
    { label: translate('custom.device_details.telemetrySortDefault'), value: TELEMETRY_CARD_SORT_MODE.default },
    { label: translate('custom.device_details.telemetrySortName'), value: TELEMETRY_CARD_SORT_MODE.name },
    { label: translate('custom.device_details.telemetrySortLastUpdate'), value: TELEMETRY_CARD_SORT_MODE.lastUpdate }
  ])

  const telemetryFreshnessOptions = computed<TelemetrySortOption[]>(() => [
    {
      label: translate('custom.device_details.telemetryFreshnessAll'),
      value: TELEMETRY_CARD_FRESHNESS_FILTER.all
    },
    {
      label: translate('custom.device_details.telemetryFreshnessAttention'),
      value: TELEMETRY_CARD_FRESHNESS_FILTER.attention
    },
    {
      label: translate('custom.device_details.telemetryFreshnessStaleOnly'),
      value: TELEMETRY_CARD_FRESHNESS_FILTER.stale
    },
    {
      label: translate('custom.device_details.telemetryFreshnessMissingOnly'),
      value: TELEMETRY_CARD_FRESHNESS_FILTER.missingTimestamp
    }
  ])

  const instance = getCurrentInstance()
  const getPlatform = computed(() => {
    const { proxy }: any = instance
    return proxy.getPlatform()
  })

  return {
    cardHeight,
    cardMargin,
    getPlatform,
    nowTime,
    telemetryFreshnessOptions,
    telemetrySortOptions
  }
}
