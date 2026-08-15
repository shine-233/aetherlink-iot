<!--
  文件用途：设备详情页中的遥测图表展示面板。
  核心链路：读取物模型里的 `web_chart_config`，归一化旧版字段绑定表达式与运行时数据源，再结合首屏遥测/属性快照和实时推送，把设备当前数据喂给 ThingsVis viewer 渲染。
  使用注意：
  1. 这里展示的是“物模型驱动的图表视图”，没有物模型或物模型里没有图表配置时不会自动兜底成普通折线图。
  2. 图表可用性同时依赖物模型配置、平台字段提取、实时推送和当前快照，任一环节失配都可能导致局部控件空白。
  3. 当前模块同时承担物模型归一化、首屏补数、实时推送和高度自适应，链路完整但文件体积较大。
  静态审查建议：
  1. 物模型归一化逻辑较多，适合继续下沉到独立 helper，减少组件文件体积和理解门槛。
  2. `fetchAndUpdateData`、`initTemplateData` 与实时推送初始化之间时序耦合较强，后续可引入更显式的状态机或分阶段 composable。
  3. 图表空态、解析失败、运行时字段缺失目前偏向静默降级，适合补更细的观测日志和运维提示。
-->
<script setup lang="tsx">
/**
 * 设备详情图表 Tab 使用 ThingsVis 预览模式渲染物模型图表。
 *
 * 数据推送策略：
 *  - tp-03: WebSocket 实时推送（首选），断线自动回退轮询
 *  - tp-02: ThingsVis ready 后补抓一次当前快照，避免首屏空白
 *  - tp-04: 告警/事件字段 30s 轮询推送
 */

import { onMounted, onBeforeUnmount, ref, watch, computed, defineAsyncComponent } from 'vue'
import { NEmpty, NCard, NSkeleton } from 'naive-ui'
import { $t } from '@/locales'
import { extractPlatformFields, mergePlatformFieldsById } from '@/utils/thingsvis/platform-fields'
import { telemetryDataCurrent, getAttributeDataSet } from '@/service/api/device'
import { telemetryApi, attributesApi, eventsApi, commandsApi } from '@/service/api'
import type { PlatformField } from '@/utils/thingsvis/types'
import { useRealtimePush } from '@/hooks/thingsvis/useRealtimePush'
import { useAlarmPush } from '@/hooks/thingsvis/useAlarmPush'
import { getCachedDeviceTemplateDetail } from '@/utils/thingsvis/template-detail-cache'
import { normalizeTemplateChartConfig } from './telemetryChartTemplateNormalizer'

const ThingsVisWidget = defineAsyncComponent({
  loader: () => import('@/components/thingsvis/ThingsVisWidget.vue'),
  suspensible: false
})

const TEMPLATE_PLATFORM_FIELD_PAGE_SIZE = 200

type ThingsVisWidgetExposed = {
  pushPlatformData: (fields: Record<string, unknown>, deviceId?: string) => void
}

function extractResponseList(response: { data?: unknown } | null | undefined): any[] {
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

const props = defineProps<{
  /** 设备ID */
  id: string
  /** 物模型ID */
  deviceTemplateId?: string
  /** 设备详情数据 (可选) */
  deviceData?: Record<string, any>
}>()

// 动态计算图表容器高度：基于元素在视口中的位置自适应
const chartCardRef = ref<InstanceType<typeof NCard> | null>(null)
const availableHeight = ref(0)
let resizeObserver: ResizeObserver | null = null

function updateAvailableHeight() {
  const el = chartCardRef.value?.$el as HTMLElement | undefined
  if (!el) return
  const rect = el.getBoundingClientRect()
  const cardPaddingTop = 24
  availableHeight.value = Math.max(window.innerHeight - rect.top - cardPaddingTop - 24, 200)
}

const chartHeight = computed(() => {
  if (availableHeight.value > 0) {
    return `${availableHeight.value}px`
  }
  return 'calc(100vh - 200px)'
})

// 图表展示状态：
// - chartLoading 控制首屏骨架与切物模型过程；
// - hasTemplate 表示当前物模型是否真的提供了可渲染图表；
// - initialConfig / platformFields 共同决定 ThingsVis 如何解释后续数据。
const chartLoading = ref(true)
const hasTemplate = ref(false)
const initialConfig = ref<any>(null)
const platformFields = ref<PlatformField[]>([])
const visWidgetRef = ref<ThingsVisWidgetExposed | null>(null)

// 当前数据快照既用于首屏回填，也作为实时推送汇总后的单一事实源。
// 如果 viewer 已 ready 但实时流还没到，组件也能先吃到接口抓回来的当前值。
const currentData = ref<Record<string, any>>({})
const currentDataDeviceId = ref('')
const viewerPlatformDevices = computed(() => {
  if (!props.id || platformFields.value.length === 0) return []
  return [
    {
      deviceId: props.id,
      deviceName: props.deviceData?.name || 'Device',
      fields: platformFields.value
    }
  ]
})

const deviceIdRef = computed(() => props.id)
// 物模型上下文要么来自已解析的 deviceData，要么来自单独传入的物模型 ID。
// 只有上下文解析完成后，才值得继续拉物模型配置和初始化推送。
const templateContextResolved = computed(() => {
  return Boolean(props.deviceData) || Boolean(props.deviceTemplateId)
})
const hasLoadedInitialSnapshot = computed(() => {
  return currentDataDeviceId.value === props.id && Object.keys(currentData.value).length > 0
})

// 首屏补抓设备当前遥测/属性快照：
// 1. 根据平台字段判断是否还要额外请求 attribute；
// 2. 把 telemetry / attribute 统一归一成字段键值；
// 3. 再推送给 ThingsVis，保证 viewer 初始化后立刻有数据可画。
const fetchAndUpdateData = async () => {
  if (!props.id || platformFields.value.length === 0) return

  try {
    const hasAttributes = platformFields.value.some((f) => f.dataType === 'attribute')

    const [telemetryRes, attributeRes] = await Promise.all([
      telemetryDataCurrent(props.id),
      hasAttributes ? getAttributeDataSet({ device_id: props.id }) : Promise.resolve({ data: [] })
    ])

    const telemetryList = telemetryRes?.data || []
    const attributeList = attributeRes?.data || []
    const kvMap: Record<string, any> = {}

    const processItem = (item: any) => {
      if (item?.key !== undefined) kvMap[item.key] = item.value
      if (item?.label) kvMap[item.label] = item.value
    }

    if (Array.isArray(telemetryList)) telemetryList.forEach(processItem)
    if (Array.isArray(attributeList)) attributeList.forEach(processItem)

    const dataMap: Record<string, any> = {}
    platformFields.value.forEach((field) => {
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
      currentDataDeviceId.value = props.id
      pushDataToVis(dataMap)
    }
  } catch (err) {
    console.error('[TelemetryChart] 获取设备实时数据失败:', err)
  }
}

// 首屏快照、实时推送和告警补数最终都汇总到同一入口，
// 避免多个数据源各自直接改 viewer 数据，导致状态不一致。
const pushDataToVis = (fields: Record<string, unknown>) => {
  if (Object.keys(fields).length === 0) return
  currentData.value = {
    ...currentData.value,
    ...fields
  }
  visWidgetRef.value?.pushPlatformData(fields, props.id)
}

// 历史推送方法
const realtimePush = ref<ReturnType<typeof useRealtimePush> | null>(null)
const alarmPush = ref<ReturnType<typeof useAlarmPush> | null>(null)
let initSequence = 0

// 加载物模型与图表配置：
// 1. 先拿缓存物模型详情；
// 2. 再补齐平台字段元信息；
// 3. 把物模型配置里的旧字段表达式归一化成当前运行时可识别的结构；
// 4. 成功后立即回填一次当前设备快照，避免 viewer 空转。
const initTemplateData = async (deviceTemplateId: string) => {
  if (!deviceTemplateId) {
    hasTemplate.value = false
    chartLoading.value = false
    return
  }

  try {
    const res = await getCachedDeviceTemplateDetail(deviceTemplateId)

    if (res.data) {
      const [telemetryRes, attributesRes, eventsRes, commandsRes] = await Promise.all([
        telemetryApi({ page: 1, page_size: TEMPLATE_PLATFORM_FIELD_PAGE_SIZE, device_template_id: deviceTemplateId }),
        attributesApi({ page: 1, page_size: TEMPLATE_PLATFORM_FIELD_PAGE_SIZE, device_template_id: deviceTemplateId }),
        eventsApi({ page: 1, page_size: TEMPLATE_PLATFORM_FIELD_PAGE_SIZE, device_template_id: deviceTemplateId }),
        commandsApi({ page: 1, page_size: TEMPLATE_PLATFORM_FIELD_PAGE_SIZE, device_template_id: deviceTemplateId })
      ])

      const telemetryList = extractResponseList(telemetryRes)
      const attributesList = extractResponseList(attributesRes)
      const eventsList = extractResponseList(eventsRes)
      const commandsList = extractResponseList(commandsRes)

      const platformSource = {
        telemetry: telemetryList,
        attributes: attributesList,
        events: eventsList,
        commands: commandsList
      }

      const extractedFields = extractPlatformFields(platformSource)
      platformFields.value = mergePlatformFieldsById(extractedFields, extractPlatformFields(res.data))
      const availableFieldIds = new Set(
        platformFields.value
          .map((field) => field?.id)
          .filter((fieldId): fieldId is string => typeof fieldId === 'string' && fieldId.length > 0)
      )

      if (res.data.web_chart_config) {
        try {
          const configJson = normalizeTemplateChartConfig(
            JSON.parse(res.data.web_chart_config),
            props.id,
            availableFieldIds
          )
          initialConfig.value = configJson
          hasTemplate.value = true
          await fetchAndUpdateData()
        } catch (e) {
          console.warn('[TelemetryChart] 解析 web_chart_config 失败', e)
          hasTemplate.value = false
        }
      } else {
        hasTemplate.value = false
      }
    }
  } catch (error) {
    console.error('[TelemetryChart] 加载物模型数据失败:', error)
    hasTemplate.value = false
  } finally {
    chartLoading.value = false
  }
}

const onVisReady = async () => {
  // ThingsVis 真正 ready 后再补一次当前快照，
  // 避免组件先 ready、数据后到，导致图表或指标卡首屏短暂空白。
  if (!hasLoadedInitialSnapshot.value) {
    await fetchAndUpdateData()
  }
}

// 监听设备 ID / 物模型 ID / 物模型上下文变化，完整重建图表运行时。
// currentSequence 用来丢弃过期初始化结果，避免用户快速切设备时旧请求覆盖新页面。
watch(
  [() => props.deviceTemplateId, templateContextResolved, () => props.id],
  async ([newVal]) => {
    const currentSequence = ++initSequence

    // 先停止旧的推送
    realtimePush.value?.stop()
    alarmPush.value?.stop()
    realtimePush.value = null
    alarmPush.value = null
    currentData.value = {}
    currentDataDeviceId.value = ''
    platformFields.value = []
    initialConfig.value = null
    hasTemplate.value = false

    if (!templateContextResolved.value) {
      chartLoading.value = true
      return
    }

    if (!newVal) {
      chartLoading.value = false
      return
    }

    chartLoading.value = true

    await initTemplateData(newVal)

    if (currentSequence !== initSequence || !hasTemplate.value) {
      return
    }

    // 物模型和首屏快照准备好之后，再初始化实时推送 composable，
    // 保证推送进来的字段至少有一套可解释的平台字段与图表配置。
    realtimePush.value = useRealtimePush(deviceIdRef, platformFields, pushDataToVis, async () => {
      if (!hasLoadedInitialSnapshot.value) {
        await fetchAndUpdateData()
      }
    })

    alarmPush.value = useAlarmPush(deviceIdRef, platformFields, pushDataToVis)

    // 先启动普通实时推送，再启动告警/事件补充推送，两者最终都回流到 pushDataToVis。
    realtimePush.value?.start()
    // 启动告警轮询
    alarmPush.value?.start()
  },
  { immediate: true }
)

onMounted(() => {
  const el = chartCardRef.value?.$el as HTMLElement | undefined
  if (el) {
    resizeObserver = new ResizeObserver(() => updateAvailableHeight())
    resizeObserver.observe(el.parentElement || el)
  }
  window.addEventListener('resize', updateAvailableHeight)
  updateAvailableHeight()
})

onBeforeUnmount(() => {
  realtimePush.value?.stop()
  alarmPush.value?.stop()
  resizeObserver?.disconnect()
  window.removeEventListener('resize', updateAvailableHeight)
})
</script>

<template>
  <NCard ref="chartCardRef" class="device-chart-card w-full h-full" :bordered="false" content-style="padding: 0;">
    <template v-if="chartLoading">
      <div class="device-chart-card__state">
        <NSkeleton text :repeat="3" />
        <NSkeleton height="180px" class="mt-12px" />
      </div>
    </template>

    <template v-else-if="!hasTemplate">
      <div class="device-chart-card__state">
        <NEmpty :description="$t('custom.device_details.noChartTemplate')" />
      </div>
    </template>

    <template v-else>
      <ThingsVisWidget
        ref="visWidgetRef"
        mode="viewer"
        :config="initialConfig"
        :data="currentData"
        :platform-fields="platformFields"
        :platform-devices="viewerPlatformDevices"
        :height="chartHeight"
        :buffer-size="100"
        :device-id="props.id"
        @ready="onVisReady"
      />
    </template>
  </NCard>
</template>

<style scoped>
.device-chart-card {
  overflow: hidden;
  border-radius: 12px;
  background: #fff;
}

.device-chart-card__state {
  padding: 18px 20px;
}
</style>
