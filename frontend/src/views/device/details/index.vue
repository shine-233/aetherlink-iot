<!--
  设备详情页壳层，负责承接路由设备 ID、详情加载、tab 可见性裁剪、在线状态订阅与子模块挂载。
  关键链路：路由参数 d_id -> 详情请求 -> 依据设备能力裁剪 tab -> 订阅在线状态 -> 将公共状态传给子模块。
  静态维护重点：
  1. 路由与刷新入口统一走 reloadDeviceDetailFromRoute / reloadDeviceDetail，避免出现多套详情同步流程。
  2. tab 列表由接口数据和模板能力共同决定，调整显隐规则时要同时检查默认激活 tab 与 refreshKey 重挂载逻辑。
  3. 在线状态 WebSocket 兼容了多种历史报文结构，修改字段兼容逻辑时不要只按单一 payload 形态处理。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, getCurrentInstance, onBeforeMount, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useLoading } from '@aetherlink/hooks'
import { useWebSocket } from '@vueuse/core'
import { $t } from '@/locales'
import { useAppStore } from '@/store/modules/app'
import { deviceDetail, deviceUpdate } from '@/service/api/device'
import { localStg } from '@/utils/storage'
import { useRouterPush } from '@/hooks/common/router'
import { getWebsocketServerUrl } from '@/utils/common/tool'
import { formatDateTime } from '@/utils/common/datetime'
import { createLogger } from '@/utils/logger'
import { message } from '@/utils/common/discrete'
import {
  createDeviceUpdatePayload,
  normalizeDeviceDetailState,
  syncDeviceUpdateQueryTarget,
  validateDeviceUpdate,
  type DeviceDetailData
} from './device-edit-state'
import { normalizeOnlineStatus, normalizeOnlineStatusUpdatedAt } from './device-online-status-frame'
import { useDeviceDetailTabsController } from './useDeviceDetailTabsController'

const logger = createLogger('DeviceDetail')
const DeviceStatusHistory = defineAsyncComponent(() => import('@/views/device/details/modules/device-status.vue'))
const DeviceOperationsWorkbench = defineAsyncComponent(
  () => import('@/views/device/details/modules/DeviceOperationsWorkbench.vue')
)
const route = useRoute()
const router = useRouter()
const { query } = route
const appStore = useAppStore()

/**
 * 路由查询参数可能是数组、空值或非字符串，这里统一规整成页面内部唯一使用的设备 ID。
 * 后续所有详情请求、WebSocket 订阅和子模块刷新都依赖这个值。
 */
function normalizeRouteQueryParam(value: unknown): string {
  if (Array.isArray(value)) return normalizeRouteQueryParam(value[0])
  if (value === null || value === undefined) return ''
  return String(value)
}

const routeDeviceId = ref(normalizeRouteQueryParam(query.d_id))
const routeTabKey = ref(normalizeRouteQueryParam(query.tab))

function syncRouteDeviceId(value: unknown) {
  routeDeviceId.value = normalizeRouteQueryParam(value)
}

function syncRouteTabKey(value: unknown) {
  routeTabKey.value = normalizeRouteQueryParam(value)
}

const getDeviceId = () => {
  return routeDeviceId.value
}

const { loading, startLoading, endLoading } = useLoading()

type DeviceDetailReloadOptions = {
  refreshActiveTab?: boolean
}

/**
 * 在这里声明所有候选 tab，再根据设备详情、模板能力和 RDI 能力裁剪出最终可见集合。
 * 这样可以把“候选能力列表”和“显隐规则”分开维护，避免模板里散落条件分支。
 */
const device_type = ref('')

const {
  activateTabIfVisible,
  changeTabs,
  components,
  refreshActiveTabIfNeeded,
  remountActiveTabLabel,
  syncDeviceDetailTabs,
  tabValue,
  tabsRenderKey
} = useDeviceDetailTabsController({
  startLoading,
  endLoading,
  onDeviceTypeChange: (nextDeviceType) => {
    device_type.value = nextDeviceType
  },
  onTabChange: (nextTabKey) => {
    router.replace({
      path: route.path,
      query: {
        ...route.query,
        d_id: getDeviceId(),
        tab: nextTabKey
      }
    })
  }
})

/**
 * 首次进入时先保持空 tab，等待详情接口确认可见 tab 后再回填，避免短暂渲染错误模块。
 */
const showDialog = ref(false)
const showStatusHistoryDialog = ref(false)
const labels = ref<string[]>([])

const deviceData: any = ref({})
const loadedDeviceId = ref('')
const sharedRouteRequested = computed(() => {
  const shared = normalizeRouteQueryParam(route.query.shared).toLowerCase()
  const access = normalizeRouteQueryParam(route.query.access).toLowerCase()
  return shared === '1' || shared === 'true' || access === 'shared'
})
const isSharedReadOnly = computed(
  () => sharedRouteRequested.value || deviceData.value?.shared_read_only === true
)
const isCurrentDeviceDetailLoaded = computed(() => {
  const currentDeviceId = getDeviceId()
  const routeQueryDeviceId = normalizeRouteQueryParam(route.query.d_id)
  return Boolean(
    currentDeviceId &&
      currentDeviceId === routeQueryDeviceId &&
      loadedDeviceId.value === currentDeviceId &&
      normalizeRouteQueryParam(deviceData.value?.id) === currentDeviceId
  )
})
const canUseOwnerDetailActions = computed(
  () => isCurrentDeviceDetailLoaded.value && !isSharedReadOnly.value
)
const visibleDetailComponents = computed(() => {
  if (!isCurrentDeviceDetailLoaded.value) return []
  return isSharedReadOnly.value
    ? components.value.filter((component) => component.sharedReadOnlySafe === true)
    : components.value
})
const name = ref('')
const device_number = ref('')
const device_is_online = ref(0)
const deviceOnlineStatusUpdatedAt = ref('')
const device_loop = ref(false)
let wsUrl = getWebsocketServerUrl()
const DEVICE_STATUS_INACTIVE_COLOR = '#ccc'
const DEVICE_ALARM_ACTIVE_COLOR = '#ee0808'

wsUrl += `/device/online/status/ws`

const deviceOnlineStatusUpdatedAtDisplay = computed(() => formatDateTime(deviceOnlineStatusUpdatedAt.value) || '--')

/**
 * WebSocket 每次推送只尝试更新当前页面设备的在线态，不主动覆盖其它详情字段。
 */
function applyOnlineStatusFrame(frame: string) {
  if (!frame || frame === 'pong') return

  try {
    const payload = JSON.parse(frame)
    const status = normalizeOnlineStatus(payload, getDeviceId())
    if (status !== null) {
      device_is_online.value = status
      deviceOnlineStatusUpdatedAt.value = normalizeOnlineStatusUpdatedAt(payload, getDeviceId()) || new Date().toISOString()
    }
  } catch {
    /**
     * 忽略心跳外的非 JSON 文本帧，避免无意义日志污染。
     */
  }
}

const { send } = useWebSocket(wsUrl, {
  heartbeat: {
    message: 'ping',
    interval: 8000,
    pongTimeout: 3000
  },
  onMessage(_ws: WebSocket, event: MessageEvent) {
    applyOnlineStatusFrame(event.data)
  }
})

const normalizeAlarmActive = (raw: unknown): boolean => {
  if (typeof raw === 'boolean') return raw
  if (typeof raw === 'number') return raw > 0
  if (typeof raw === 'string') {
    const normalized = raw.trim().toLowerCase()
    if (
      !normalized ||
      normalized === '0' ||
      normalized === 'false' ||
      normalized === 'off' ||
      normalized === 'no' ||
      normalized === 'n'
    ) {
      return false
    }
    if (
      normalized === '1' ||
      normalized === 'true' ||
      normalized === 'on' ||
      normalized === 'yes' ||
      normalized === 'y'
    ) {
      return true
    }
  }
  return Boolean(raw)
}

const deviceAlarmActive = computed(() =>
  normalizeAlarmActive(
    deviceData.value?.warn_status ??
      deviceData.value?.warnStatus ??
      deviceData.value?.alarm_status ??
      deviceData.value?.alarmStatus
  )
)

const deviceAlarmColor = computed(() =>
  deviceAlarmActive.value ? DEVICE_ALARM_ACTIVE_COLOR : DEVICE_STATUS_INACTIVE_COLOR
)
const visibleDeviceOperationTabs = computed(() => visibleDetailComponents.value.map((component) => component.key))

const queryParams = reactive({
  label: '',
  id: '',
  name: '',
  device_number: '',
  description: ''
})

const editConfig = () => {
  if (!canUseOwnerDetailActions.value) return
  showDialog.value = true
}

const rules = {
  name: {
    required: true,
    message: $t('custom.devicePage.enterDeviceName'),
    trigger: 'blur'
  },
  device_number: {
    required: true,
    message: $t('custom.devicePage.enterDeviceNumber'),
    trigger: 'blur'
  }
}

function syncDeviceDetailState(data: DeviceDetailData) {
  const normalized = normalizeDeviceDetailState(data)

  deviceData.value = data
  if (data?.shared_read_only === true) showDialog.value = false
  labels.value = normalized.labels
  device_number.value = normalized.deviceNumber as string
  device_is_online.value = normalized.isOnline as number
  deviceOnlineStatusUpdatedAt.value = normalizeOnlineStatusUpdatedAt(data, getDeviceId())
  name.value = normalized.name as string
}

function subscribeDeviceOnlineStatus(deviceId: string) {
  send(
    JSON.stringify({
      device_id: deviceId,
      token: localStg.get('token')
    })
  )
}

let deviceDetailRequestSeq = 0

function isCurrentDeviceDetailRequest(requestSeq: number, requestDeviceId: string) {
  return (
    requestSeq === deviceDetailRequestSeq &&
    normalizeRouteQueryParam(route.query.d_id) === requestDeviceId
  )
}

function clearDeviceDetailForRouteTransition(requestDeviceId: string) {
  if (!loadedDeviceId.value || loadedDeviceId.value === requestDeviceId) return

  loadedDeviceId.value = ''
  deviceData.value = {}
  labels.value = []
  name.value = ''
  device_number.value = ''
  device_is_online.value = 0
  deviceOnlineStatusUpdatedAt.value = ''
  device_type.value = ''
  showDialog.value = false
  showStatusHistoryDialog.value = false
}

/**
 * 详情请求失败时保留现有页面态，避免把已展示的数据与 tab 直接清空。
 * 跨设备切换是例外：旧设备数据会立即失效，且只有最新路由请求可以提交新状态。
 */
async function requestDeviceDetail(currentDeviceId: string) {
  const requestSeq = ++deviceDetailRequestSeq
  clearDeviceDetailForRouteTransition(currentDeviceId)
  device_loop.value = false
  const { error, data } = await deviceDetail(currentDeviceId)

  if (!isCurrentDeviceDetailRequest(requestSeq, currentDeviceId)) {
    logger.info('[DeviceDetail] Discard stale detail response.', {
      deviceId: currentDeviceId,
      requestSeq
    })
    return null
  }

  device_loop.value = true

  if (error || !data) {
    logger.warn('[DeviceDetail] Skip detail state reset because request failed.', {
      deviceId: currentDeviceId,
      hasData: Boolean(data),
      error: error instanceof Error ? error.message : error
    })
    return null
  }

  return { data, requestSeq }
}

async function syncDeviceDetailView(data: DeviceDetailData) {
  syncDeviceDetailState(data)
  await syncDeviceDetailTabs(data)
}

/**
 * 详情加载成功后的副作用入口：
 * 1. 重新订阅当前设备在线状态；
 * 2. 让依赖设备 store 的其它局部区域同步刷新。
 */
function runDeviceDetailSideEffects(currentDeviceId: string) {
  subscribeDeviceOnlineStatus(currentDeviceId)
}

async function loadAndSyncDeviceDetail(currentDeviceId: string) {
  const result = await requestDeviceDetail(currentDeviceId)
  if (!result || !isCurrentDeviceDetailRequest(result.requestSeq, currentDeviceId)) return false

  await syncDeviceDetailView(result.data)
  if (!isCurrentDeviceDetailRequest(result.requestSeq, currentDeviceId)) return false

  loadedDeviceId.value = currentDeviceId
  runDeviceDetailSideEffects(currentDeviceId)
  return true
}

async function reloadDeviceDetail(currentDeviceId: string, options: DeviceDetailReloadOptions = {}) {
  if (!currentDeviceId) return

  const hasSynced = await loadAndSyncDeviceDetail(currentDeviceId)
  if (!hasSynced) return

  activateRequestedDetailTab(routeTabKey.value)
  refreshActiveTabIfNeeded(options.refreshActiveTab)
}

function getRouteDeviceId() {
  syncRouteDeviceId(route.query.d_id)
  syncRouteTabKey(route.query.tab)
  return getDeviceId()
}

async function reloadDeviceDetailFromRoute(options: DeviceDetailReloadOptions = {}) {
  await reloadDeviceDetail(getRouteDeviceId(), options)
}

// 路由刷新、弹窗关闭、保存成功和子模块变更都统一走这个详情重载入口。
const getDeviceDetail = async (options: DeviceDetailReloadOptions = {}) => {
  await reloadDeviceDetail(getDeviceId(), options)
}

const closeModal = async () => {
  await getDeviceDetail()
  showDialog.value = false
}

const { routerPushByKey } = useRouterPush()

function navigateByKey(routeKey: string, query: Record<string, unknown>) {
  routerPushByKey(routeKey as Parameters<typeof routerPushByKey>[0], {
    query: query as Record<string, string>
  })
}

const clickConfig: () => void = () => {
  if (!canUseOwnerDetailActions.value) return
  navigateByKey('device_config-detail', {
    id: deviceData.value?.device_config_id
  })
}

const clickGateway = () => {
  if (!canUseOwnerDetailActions.value) return
  navigateByKey('device_details', {
    d_id: deviceData.value?.parent_id
  })
}

const clickAlarmHistory = () => {
  if (!canUseOwnerDetailActions.value) return
  navigateByKey('alarm_warning-message', {
    device_id: getDeviceId()
  })
}

function openOperationWorkbenchTab(tabKey: string) {
  if (!canUseOwnerDetailActions.value) return
  changeTabs(tabKey)
}

function openStatusHistory() {
  if (!canUseOwnerDetailActions.value) return
  showStatusHistoryDialog.value = true
}

function activateRequestedDetailTab(requestedTabKey: string) {
  if (!isSharedReadOnly.value) {
    activateTabIfVisible(requestedTabKey)
    return
  }

  const visibleComponents = visibleDetailComponents.value
  if (!visibleComponents.length) return
  const requested = visibleComponents.find((component) => component.key === requestedTabKey)
  const fallback = visibleComponents.find((component) => component.key === 'message') || visibleComponents[0]
  changeTabs((requested || fallback).key)
}

const goBack = () => {
  router.back()
}

const refreshCurrentTab = async () => {
  await getDeviceDetail({ refreshActiveTab: true })
}

onBeforeMount(() => {
  getDeviceDetail()
})

// 当 d_id 改变时复用当前页面实例，并在原地刷新详情与当前激活 tab。
watch(
  () => [route.query.d_id, route.query.tab, route.query.shared, route.query.access],
  async ([nextDeviceId, nextTabKey], [previousDeviceId]) => {
    if (nextDeviceId === previousDeviceId) {
      syncRouteTabKey(nextTabKey)
      activateRequestedDetailTab(routeTabKey.value)
      return
    }

    await reloadDeviceDetailFromRoute({ refreshActiveTab: true })
  }
)

function validateDeviceBeforeSave() {
  const result = validateDeviceUpdate(deviceData.value, {
    nameRequired: $t('custom.devicePage.enterDeviceName'),
    numberRequired: $t('custom.devicePage.enterDeviceNumber'),
    numberMax: $t('custom.devicePage.deviceNumberMax')
  })
  if (!result.valid) {
    message.error(result.message || '')
    return false
  }
  return true
}

function handleDeviceUpdateSuccess() {
  showDialog.value = false
  getDeviceDetail()
}

const save = async () => {
  if (!canUseOwnerDetailActions.value) return
  if (!validateDeviceBeforeSave()) return

  const payload = createDeviceUpdatePayload(deviceData.value, labels.value)
  device_number.value = payload.device_number as string
  syncDeviceUpdateQueryTarget(queryParams, payload)

  const { error } = await deviceUpdate(queryParams)
  if (!error) {
    handleDeviceUpdateSuccess()
  }
}

// tab 标题依赖国际化文案，语言切换后需要重挂载当前标签文本。
watch(() => appStore.locale, remountActiveTabLabel)

const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})

const isEmbeddedHost = computed(() => {
  try {
    return window.self !== window.top
  } catch {
    return true
  }
})
</script>

<template>
  <div class="device-details-page" :class="{ 'device-details-page--embedded': isEmbeddedHost }">
    <section class="device-details-shell">
      <div class="device-details-header">
        <div class="device-details-title-row">
          <span class="device-details-title">{{ name || '--' }}</span>
          <NTag v-if="isSharedReadOnly" type="info">{{ $t('script.readonly') }}</NTag>
          <NButton @click="goBack">{{ $t('common.back') }}</NButton>
          <NButton :loading="loading" @click="refreshCurrentTab">{{ $t('common.refresh') }}</NButton>
          <NButton v-if="canUseOwnerDetailActions" type="primary" @click="editConfig">
            {{ $t('common.edit') }}
          </NButton>
        </div>

        <n-modal aria-label="dialog"
          v-if="canUseOwnerDetailActions"
          v-model:show="showDialog"
          :title="$t('generate.issue-attribute')"
          :class="getPlatform ? 'w-90%' : 'w-400px'"
        >
          <n-card>
            <n-form :model="deviceData" :rules="rules">
              <div>
                <NH3>{{ $t('generate.modify-device-info') }}</NH3>
              </div>
              <n-form-item :label="$t('custom.devicePage.deviceName')" path="name">
                <n-input v-model:value="deviceData.name" aria-required="true" />
              </n-form-item>
              <n-form-item :label="$t('generate.device-code')" path="device_number">
                <n-input v-model:value="deviceData.device_number" />
              </n-form-item>
              <n-form-item :label="$t('custom.devicePage.label')" path="label">
                <n-dynamic-tags v-model:value="labels" />
              </n-form-item>
              <n-form-item :label="$t('generate.device-description')">
                <NInput v-model:value="deviceData.description" type="textarea" />
              </n-form-item>
              <n-space>
                <n-button @click="closeModal">{{ $t('generate.cancel') }}</n-button>
                <n-button @click="save">{{ $t('common.save') }}</n-button>
              </n-space>
            </n-form>
          </n-card>
        </n-modal>

        <DeviceStatusHistory
          v-if="canUseOwnerDetailActions"
          v-model:visible="showStatusHistoryDialog"
          :device-id="getDeviceId()"
        />

        <NFlex class="device-details-meta">
          <div class="device-details-meta-item">
            <span class="device-details-meta-label">ID:</span>
            <span>{{ getDeviceId() || '--' }}</span>
          </div>
          <div class="device-details-meta-item">
            <span class="device-details-meta-label">{{ $t('custom.devicePage.configTemplate') }} :</span>
            <span
              v-if="deviceData?.device_config_name && canUseOwnerDetailActions"
              class="device-details-link"
              @click="clickConfig"
            >
              {{ deviceData?.device_config_name }}
            </span>
            <span v-else-if="deviceData?.device_config_name">{{ deviceData?.device_config_name }}</span>
            <span v-else>--</span>
          </div>
          <div v-if="device_type === '3'" class="device-details-meta-item">
            <span class="device-details-meta-label">{{ $t('generate.gateway') }}:</span>
            <span v-if="canUseOwnerDetailActions" class="device-details-link" @click="clickGateway">
              {{ deviceData?.gateway_device_name || '--' }}
            </span>
            <span v-else>{{ deviceData?.gateway_device_name || '--' }}</span>
          </div>
          <!-- Click the online-status badge to open the status history dialog. -->
          <div
            class="device-details-status"
            :class="{ 'device-details-status--read-only': !canUseOwnerDetailActions }"
            @click="openStatusHistory"
          >
            <SvgIcon
              local-icon="CellTowerRound"
              :style="{ color: DEVICE_STATUS_INACTIVE_COLOR, marginRight: '5px' }"
              class="text-20px text-primary"
              :stroke="device_is_online === 1 ? 'rgb(2,153,52)' : DEVICE_STATUS_INACTIVE_COLOR"
            />
            <span
              :style="{
                color: device_is_online === 1 ? 'rgb(2,153,52)' : DEVICE_STATUS_INACTIVE_COLOR
              }"
            >
              {{ device_is_online === 1 ? $t('custom.device_details.online') : $t('custom.device_details.offline') }}
            </span>
            <span class="device-details-status-time">
              {{ $t('custom.device_details.lastUpdate') }}: {{ deviceOnlineStatusUpdatedAtDisplay }}
            </span>

            <!-- Reuse the icon as the history affordance to keep the header compact. -->
            <SvgIcon
              v-if="canUseOwnerDetailActions"
              local-icon="history"
              style="margin-left: 5px"
              class="text-18px text-primary"
            />
          </div>
          <div
            v-if="canUseOwnerDetailActions"
            class="device-details-status"
            :class="{ 'device-details-status--alarm': deviceAlarmActive }"
            @click="clickAlarmHistory"
          >
            <SvgIcon
              local-icon="AlertFilled"
              :style="{ color: deviceAlarmColor, marginRight: '5px' }"
              class="text-20px text-primary"
            />
            <span :style="{ color: deviceAlarmColor }">{{ $t('generate.alarmHistory') }}</span>
          </div>
        </NFlex>
      </div>
      <div class="device-details-content">
        <DeviceOperationsWorkbench
          v-if="canUseOwnerDetailActions"
          :device-id="getDeviceId()"
          :device-data="deviceData"
          :online="device_is_online"
          :online-updated-at="deviceOnlineStatusUpdatedAtDisplay"
          :alarm-active="deviceAlarmActive"
          :visible-tabs="visibleDeviceOperationTabs"
          @open-tab="openOperationWorkbenchTab"
        />
        <n-tabs
          :key="tabsRenderKey"
          v-model:value="tabValue"
          class="device-details-tabs"
          :class="{ 'device-details-tabs--chart-active': tabValue === 'chart' }"
          animated
          type="line"
          @update:value="changeTabs"
        >
          <n-tab-pane
            v-for="component in visibleDetailComponents"
            :key="component.key"
            :tab="component.name()"
            :name="component.key"
            display-directive="show:lazy"
          >
            <n-spin class="device-details-tab-body" size="small" :show="loading">
              <component
                :is="component.component"
                :id="getDeviceId()"
                :key="`${getDeviceId()}:${component.refreshKey}`"
                :online="device_is_online"
                :online-updated-at="deviceOnlineStatusUpdatedAtDisplay"
                :device-data="deviceData"
                :device-config-id="deviceData?.device_config_id || ''"
                :device-template-id="deviceData?.device_config?.device_template_id || ''"
                @change="getDeviceDetail"
              />
            </n-spin>
          </n-tab-pane>
        </n-tabs>
      </div>
    </section>
  </div>
</template>

<style scoped lang="scss">
.device-details-page {
  padding: 12px;
}

.device-details-page--embedded {
  padding: 8px;
}

.device-details-shell {
  overflow: hidden;
  border-radius: 12px;
  border: 1px solid #e5e7eb;
  background: #ffffff;
}

.device-details-page--embedded .device-details-shell {
  border-radius: 10px;
}

.device-details-header {
  padding: 18px 20px 8px;
}

.device-details-page--embedded .device-details-header {
  padding: 16px 18px 6px;
}

.device-details-title-row {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.device-details-title {
  font-size: 18px;
  font-weight: 500;
  line-height: 1.2;
  color: inherit;
}

.device-details-meta {
  margin-top: 10px;
  gap: 10px 16px;
  color: inherit;
}

.device-details-meta-item {
  display: flex;
  align-items: center;
  min-height: 28px;
}

.device-details-meta-label {
  margin-right: 8px;
  color: #666;
}

.device-details-link {
  color: blue;
  cursor: pointer;
}

.device-details-status {
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
}

.device-details-status--read-only {
  cursor: default;
}

.device-details-status-time {
  max-width: 180px;
  overflow: hidden;
  color: #888;
  font-size: 12px;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-details-status--alarm {
  cursor: pointer;
}

.device-details-content {
  padding-bottom: 8px;
}

.device-details-page--embedded .device-details-content {
  padding-bottom: 8px;
}

.device-details-tab-body {
  padding: 10px 14px 14px;
}

.device-details-page--embedded .device-details-tab-body {
  padding: 10px 12px 12px;
}

:deep(.device-details-tabs .n-tabs-nav) {
  padding: 0 20px;
}

:deep(.device-details-page--embedded .device-details-tabs .n-tabs-nav) {
  padding: 0 18px;
}

:deep(.device-details-tabs .n-tabs-nav::before) {
  border-bottom-color: #e5e7eb;
}

/* 图表 Tab 激活时去掉导航底部分隔线，让图表区域与内容区视觉连成一体。 */
:deep(.device-details-tabs.device-details-tabs--chart-active .n-tabs-nav::before) {
  border-bottom: none;
}

:deep(.device-details-tabs .n-tabs-tab) {
  padding-bottom: 12px;
  font-weight: 500;
}

:deep(.device-details-tabs .n-tab-pane) {
  padding-top: 0;
}

/* ≤640px 现场手机查设备详情最小保障；断点统一取 _mixins.scss 的 mobile mixin（=--breakpoint-sm=640px）。 */
@include mobile {
  .device-details-page {
    padding: 8px;
  }

  .device-details-header {
    padding: 12px 14px 6px;
  }

  /* 头部信息栅格降级单列：meta 纵向堆叠，状态时间戳允许换行不再截断 */
  .device-details-meta {
    flex-direction: column;
    align-items: stretch;
    gap: 6px;
  }

  .device-details-status-time {
    max-width: none;
    white-space: normal;
  }

  /* tab 导航横向滑动（隐藏滚动条），避免多 tab 在窄屏溢出 */
  :deep(.device-details-tabs .n-tabs-nav) {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    scrollbar-width: none;

    &::-webkit-scrollbar {
      display: none;
    }
  }
}
</style>
