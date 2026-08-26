import { defineAsyncComponent, markRaw } from 'vue'
import { $t } from '@/locales'

export type DeviceDetailTabComponent = {
  key: string
  name: () => string
  component: any
  refreshKey: number
  /** Explicit capability gate for content that is safe in an accepted-share view. */
  sharedReadOnlySafe?: boolean
}

const createAsyncDeviceTab = (loader: () => Promise<any>) => markRaw(defineAsyncComponent(loader))
const rdiHistoryTabComponent = createAsyncDeviceTab(
  () => import('@/views/device/details/modules/RdiDeviceHistoryView.vue')
)
const rdiDetailsTabComponent = createAsyncDeviceTab(
  () => import('@/views/device/details/modules/RdiDeviceDetailsView.vue')
)
// REQ-07（客户原话）：详情页顶部只允许四个切换按钮 ——
// 「设备详细信息 / 历史数据 / 报警信息 / 当前参数设定」。客户在需求原文里同时强调
// 「只需要跟踪 4-5 个数据点」「不需要在这里太复杂，只需要简单明了」，
// 因此 REQ-48（用电量统计）与 REQ-53（Field Setting）不再各占一个顶层 Tab，
// 改为嵌入 message（详细信息）页内的两个只读区块，见 RdiDeviceDetailsView.vue。
const rdiCustomerTabKeys = ['message', 'chart', 'give-an-alarm', 'rdi'] as const

const rdiCustomerTabNameKeys: Record<(typeof rdiCustomerTabKeys)[number], string> = {
  message: 'custom.device_details.rdiDetailedInfo',
  chart: 'custom.device_details.history',
  'give-an-alarm': 'custom.device_details.rdiAlarmInfo',
  rdi: 'custom.device_details.rdiCurrentParameterSettings'
}

export function applyRdiCustomerTabs(tabs: DeviceDetailTabComponent[]): DeviceDetailTabComponent[] {
  const tabsByKey = new Map(tabs.map((tab) => [tab.key, tab]))

  return rdiCustomerTabKeys.flatMap((key) => {
    const tab = tabsByKey.get(key)
    if (!tab) return []

    const component =
      key === 'message' ? rdiDetailsTabComponent : key === 'chart' ? rdiHistoryTabComponent : tab.component

    return [
      {
        ...tab,
        name: () => $t(rdiCustomerTabNameKeys[key]),
        component,
        sharedReadOnlySafe: key === 'message' || key === 'chart'
      }
    ]
  })
}

export function createBaseDeviceTabs(): DeviceDetailTabComponent[] {
  return [
    {
      key: 'ready-check',
      name: () => $t('custom.device_details.readyCheck'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/onboarding-ready-check.vue')),
      refreshKey: 0
    },
    {
      key: 'chart',
      name: () => $t('custom.device_details.chart'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/telemetry-chart.vue')),
      refreshKey: 0,
      sharedReadOnlySafe: true
    },
    {
      key: 'telemetry',
      name: () => $t('custom.device_details.telemetry'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/telemetry/telemetry.vue')),
      refreshKey: 0
    },
    {
      key: 'device-twin',
      name: () => $t('custom.device_details.twinLite'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/device-twin.vue')),
      refreshKey: 0
    },
    {
      key: 'device-3d',
      name: () => $t('custom.device_details.preview3d'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/device-3d.vue')),
      refreshKey: 0,
      sharedReadOnlySafe: true
    },
    {
      key: 'rdi',
      name: () => 'RDI',
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/RdiDeviceOperationsView.vue')),
      refreshKey: 0
    },
    {
      key: 'join',
      name: () => $t('custom.device_details.join'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/join.vue')),
      refreshKey: 0
    },
    {
      key: 'device-analysis',
      name: () => $t('custom.device_details.subdevice'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/device-analysis.vue')),
      refreshKey: 0
    },
    {
      key: 'message',
      name: () => $t('custom.device_details.AdditionalDetails'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/message.vue')),
      refreshKey: 0
    },
    {
      key: 'stats',
      name: () => $t('custom.device_details.attributes'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/stats.vue')),
      refreshKey: 0
    },
    {
      key: 'event-report',
      name: () => $t('custom.device_details.eventReport'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/event-report.vue')),
      refreshKey: 0
    },
    {
      key: 'command-delivery',
      name: () => $t('custom.device_details.commandDelivery'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/command-delivery.vue')),
      refreshKey: 0
    },
    {
      key: 'expect-message',
      name: () => $t('custom.device_details.expectMessage'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/expect-message.vue')),
      refreshKey: 0
    },
    {
      key: 'device-shadow',
      name: () => $t('custom.device_details.shadowQueue'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/device-shadow.vue')),
      refreshKey: 0
    },
    {
      key: 'automate',
      name: () => $t('custom.device_details.automate'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/automate.vue')),
      refreshKey: 0
    },
    {
      key: 'give-an-alarm',
      name: () => $t('custom.device_details.giveAnAlarm'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/give-an-alarm.vue')),
      refreshKey: 0
    },
    {
      key: 'device-diagnosis',
      name: () => $t('custom.device_details.deviceDiagnosis'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/device-diagnosis.vue')),
      refreshKey: 0
    },
    {
      key: 'settings',
      name: () => $t('custom.device_details.settings'),
      component: createAsyncDeviceTab(() => import('@/views/device/details/modules/settings.vue')),
      refreshKey: 0
    }
  ]
}
