<!--
文件用途: 承载移动端设备详情相关的移动端设备详情页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { extractPlatformFields, mergePlatformFieldsById } from '@/utils/thingsvis/platform-fields'
import { $t, setLocale } from '@/locales'
import { deviceDetail, deviceTemplateDetail, telemetryDataCurrent, getAttributeDataSet } from '@/service/api/device'
import { telemetryApi, attributesApi, eventsApi, commandsApi } from '@/service/api'
import { formatDateTime } from '@/utils/common/datetime'
import { localStg } from '@/utils/storage'
import type { PlatformField } from '@/utils/thingsvis/types'
import TelemetryDataCards from './telemetryDataCards.vue'
import { useRealtimePush } from '@/hooks/thingsvis/useRealtimePush'
import { useAlarmPush } from '@/hooks/thingsvis/useAlarmPush'

const ThingsVisWidget = defineAsyncComponent({
  loader: () => import('@/components/thingsvis/ThingsVisWidget.vue'),
  suspensible: false
})

const TEMPLATE_PLATFORM_FIELD_PAGE_SIZE = 200

type ThingsVisWidgetExposed = {
  pushPlatformData: (fields: Record<string, unknown>, deviceId?: string) => void
}

const route = useRoute()
const router = useRouter()
const resolveQueryString = (value: unknown): string => {
  if (Array.isArray(value)) return typeof value[0] === 'string' ? value[0] : ''
  return typeof value === 'string' ? value : ''
}
const d_id = resolveQueryString(route.query.d_id)
const token = resolveQueryString(route.query.token)
const lang = resolveQueryString(route.query.lang)
const deviceData: any = ref({})

if (token) {
  localStg.set('token', token)

  const hash = window.location.hash
  if (hash.includes('token=')) {
    const [path, queryStr] = hash.split('?')
    if (queryStr) {
      const params = new URLSearchParams(queryStr)
      params.delete('token')
      const newQuery = params.toString()
      const newHash = path + (newQuery ? `?${newQuery}` : '')
      const newUrl = window.location.href.replace(hash, newHash)
      window.history.replaceState({}, '', newUrl)
    }
  }
}

if (!localStg.get('token')) {
  router.push({ name: 'login' })
}

if (lang) {
  setLocale(lang as App.I18n.LangType)
}

const icon_type = ref('')
const device_number = ref('')
const showDefaultCards = ref(false)
const showAppChart = ref(false)
const cardHeight = ref(160)
const cardMargin = ref(15)

const initialConfig = ref<any>(null)
const platformFields = ref<PlatformField[]>([])
const currentData = ref<Record<string, any>>({})
const viewerHeight = computed(() => {
  const config = initialConfig.value
  if (!config) return '400px'

  const canvas = config.canvas || config.canvasConfig || {}
  const nodes = Array.isArray(config.nodes) ? config.nodes : []
  const nodeBottom = nodes.reduce((max: number, node: any) => {
    const y = typeof node?.y === 'number' ? node.y : node?.position?.y
    const height = typeof node?.height === 'number' ? node.height : node?.size?.height
    if (typeof y !== 'number' || typeof height !== 'number') return max
    return Math.max(max, y + height)
  }, 0)

  const canvasHeight = typeof canvas.height === 'number' ? canvas.height : 0
  const expandedHeight = Math.max(canvasHeight, nodeBottom + 96, 400)
  return `${Math.ceil(expandedHeight)}px`
})
const viewerPlatformDevices = computed(() => {
  if (!d_id || platformFields.value.length === 0) return []
  return [
    {
      deviceId: d_id,
      deviceName: deviceData.value?.name || device_number.value || 'Device',
      fields: platformFields.value
    }
  ]
})

const visWidgetRef = ref<ThingsVisWidgetExposed | null>(null)
const deviceIdRef = computed(() => d_id)

const realtimePush = ref<ReturnType<typeof useRealtimePush> | null>(null)
const alarmPush = ref<ReturnType<typeof useAlarmPush> | null>(null)

const pushDataToVis = (fields: Record<string, unknown>) => {
  if (Object.keys(fields).length === 0) return
  currentData.value = {
    ...currentData.value,
    ...fields
  }
  visWidgetRef.value?.pushPlatformData(fields, d_id)
}

const extractResponseList = (response: { data?: unknown } | null | undefined): any[] => {
  const payload: unknown = response?.data

  if (Array.isArray(payload)) {
    return payload
  }

  if (payload && typeof payload === 'object' && 'list' in payload) {
    const { list } = payload as { list?: unknown }
    if (Array.isArray(list)) {
      return list as any[]
    }
  }

  return []
}

const loadTemplatePlatformFields = async (templateId: string, templateData: any) => {
  const [telemetryRes, attributesRes, eventsRes, commandsRes] = await Promise.all([
    telemetryApi({ page: 1, page_size: TEMPLATE_PLATFORM_FIELD_PAGE_SIZE, device_template_id: templateId }),
    attributesApi({ page: 1, page_size: TEMPLATE_PLATFORM_FIELD_PAGE_SIZE, device_template_id: templateId }),
    eventsApi({ page: 1, page_size: TEMPLATE_PLATFORM_FIELD_PAGE_SIZE, device_template_id: templateId }),
    commandsApi({ page: 1, page_size: TEMPLATE_PLATFORM_FIELD_PAGE_SIZE, device_template_id: templateId })
  ])

  const platformSource = {
    telemetry: extractResponseList(telemetryRes),
    attributes: extractResponseList(attributesRes),
    events: extractResponseList(eventsRes),
    commands: extractResponseList(commandsRes)
  }

  const extractedFields = extractPlatformFields(platformSource)
  return mergePlatformFieldsById(extractedFields, extractPlatformFields(templateData))
}

const fetchDeviceData = async () => {
  if (!showAppChart.value) return

  try {
    const hasAttributes = platformFields.value.some(f => f.dataType === 'attribute')

    const [telemetryRes, attributeRes] = await Promise.all([
      telemetryDataCurrent(d_id),
      hasAttributes ? getAttributeDataSet({ device_id: d_id }) : Promise.resolve({ data: [] })
    ])

    const telemetryList = telemetryRes?.data || []
    const attributeList = attributeRes?.data || []

    const kvMap: Record<string, any> = {}
    const processItem = (item: any) => {
      if (item?.key !== undefined) {
        kvMap[item.key] = item.value
      } else if (item?.label !== undefined) {
        if (!kvMap[item.label]) kvMap[item.label] = item.value
      }
    }

    if (Array.isArray(telemetryList)) telemetryList.forEach(processItem)
    if (Array.isArray(attributeList)) attributeList.forEach(processItem)

    const dataMap: Record<string, any> = {}
    platformFields.value.forEach(field => {
      const val = kvMap[field.id] ?? kvMap[field.name]
      if (val !== undefined) {
        dataMap[field.id] = val
      }
    })

    if (Object.keys(dataMap).length > 0) {
      currentData.value = {
        ...currentData.value,
        ...dataMap
      }
      pushDataToVis(dataMap)
    }
  } catch (error) {
    console.error('[DeviceDetailsApp] 获取设备数据失败:', error)
  }
}

const getDeviceDetail = async () => {
  if (!d_id) return

  const { data, error } = await deviceDetail(d_id)
  if (error) return

  deviceData.value = data
  device_number.value = data.device_number
  icon_type.value = data.is_online !== 0 ? 'rgb(2,153,52)' : '#ccc'
  currentData.value = {
    ...currentData.value,
    is_online: data.is_online,
    online_text: data.is_online === 1 ? '在线' : '离线',
    online_status_updated_at: typeof data.ts === 'number' ? data.ts : Date.now()
  }
  showDefaultCards.value = false
  showAppChart.value = false
  initialConfig.value = null
  platformFields.value = []

  if (!data.device_config?.device_template_id) {
    showDefaultCards.value = true
    return
  }

  const templateId = data.device_config.device_template_id
  const res = await deviceTemplateDetail({ id: templateId })
  if (!res.data) {
    showDefaultCards.value = true
    return
  }

  if (!res.data.app_chart_config) {
    showDefaultCards.value = true
    return
  }

  try {
    const configJson = JSON.parse(res.data.app_chart_config)
    platformFields.value = await loadTemplatePlatformFields(templateId, res.data)

    if (configJson.dataSources && Array.isArray(configJson.dataSources)) {
      configJson.dataSources.forEach((ds: any) => {
        if (ds.type === 'PLATFORM_FIELD') {
          ds.config = ds.config || {}
          ds.config.deviceId = d_id as string
        }
      })
    }

    initialConfig.value = configJson
    showAppChart.value = true

    realtimePush.value?.stop()
    alarmPush.value?.stop()

    realtimePush.value = useRealtimePush(deviceIdRef, platformFields, pushDataToVis, fetchDeviceData)
    alarmPush.value = useAlarmPush(deviceIdRef, platformFields, pushDataToVis)

    realtimePush.value?.start()
    alarmPush.value?.start()
  } catch (e) {
    console.warn('解析 app_chart_config 失败', e)
    showDefaultCards.value = true
  }
}

const onVisReady = async () => {
  setTimeout(async () => {
    await fetchDeviceData()
  }, 500)
}

onMounted(() => {
  getDeviceDetail()
})

onBeforeUnmount(() => {
  realtimePush.value?.stop()
  alarmPush.value?.stop()
})
</script>

<template>
  <div class="device-details-app bg-gray-50" data-testid="device-details-app">
    <div class="mb-6 flex items-center justify-between">
      <h1 class="text-2xl text-gray-900 font-semibold">{{ deviceData?.name || '--' }}</h1>
      <div class="flex items-center">
        <SvgIcon
          local-icon="CellTowerRound"
          style="margin-right: 5px"
          class="color-ccc text-20px text-primary"
          :stroke="icon_type"
        />
        <span class="text-sm text-blue-500 font-medium">
          {{ deviceData?.is_online === 1 ? $t('custom.device_details.online') : $t('custom.device_details.offline') }}
        </span>
        <template v-if="deviceData?.alarmStatus === true">
          <SvgIcon
            local-icon="AlertFilled"
            style="color: #ee0808; margin-right: 5px"
            class="text-20px text-primary"
            :stroke="icon_type"
          />
          <span style="color: #ee0808">{{ $t('custom.device_details.alarm') }}</span>
        </template>
      </div>
    </div>

    <div class="mb-6 text-sm text-gray-500">
      {{ $t('custom.device_details.lastUpdate') }}: {{ formatDateTime(deviceData?.ts) || '--' }}
    </div>

    <n-divider title-placement="left"></n-divider>

    <TelemetryDataCards
      v-if="showDefaultCards"
      data-testid="device-details-default-cards"
      :id="d_id as string"
      :card-height="cardHeight"
      :card-margin="cardMargin"
    />
    <div v-if="showAppChart" class="device-details-app__viewer" data-testid="device-details-app-chart-viewer">
      <ThingsVisWidget
        ref="visWidgetRef"
        mode="viewer"
        :config="initialConfig"
        :data="currentData"
        :platform-fields="platformFields"
        :platform-devices="viewerPlatformDevices"
        :height="viewerHeight"
        :buffer-size="100"
        :device-id="d_id as string"
        @ready="onVisReady"
      />
    </div>
  </div>
</template>

<style scoped>
.color-ccc {
  color: #ccc;
}

.device-details-app {
  width: 100%;
  min-height: 100vh;
  padding: 24px;
  box-sizing: border-box;
}

.device-details-app__viewer {
  width: 100%;
  overflow: visible;
}

.device-details-app__viewer :deep(.thingsvis-widget-container) {
  min-height: 400px;
  overflow: visible;
}

:deep(.device-details-app__viewer iframe) {
  overflow: hidden;
}

:root {
  --n-padding-left: 0px;
  --n-padding-right: 0px;
}

:deep(.n-card__content) {
  padding-left: 5px !important;
  padding-right: 5px !important;
}
</style>
