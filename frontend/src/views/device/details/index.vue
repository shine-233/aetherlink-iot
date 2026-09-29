<!--
  设备详情页壳层，负责编排路由设备 ID、详情加载、tab 可见性裁剪、在线状态订阅与子模块挂载。
  关键链路：路由参数 d_id -> useDeviceDetailLoader 请求详情 -> useDeviceDetailTabsController 裁剪 tab
  -> useDeviceOnlineStatusSocket 订阅在线状态 -> 将公共状态传给子模块。
  静态维护重点：
  1. 路由与刷新入口统一走 loader.reload / loader.reloadFromRoute，避免出现多套详情同步流程。
  2. tab 列表由接口数据和模板能力共同决定，调整显隐规则时要同时检查默认激活 tab 与 refreshKey 重挂载逻辑。
  3. 在线状态 WebSocket 帧兼容逻辑在 device-online-status-frame.ts，告警/路由归一化在 device-detail-route.ts。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, getCurrentInstance, onBeforeMount, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useLoading } from '@aetherlink/hooks'
import { $t } from '@/locales'
import { useAppStore } from '@/store/modules/app'
import { deviceUpdate } from '@/service/api/device'
import { useRouterPush } from '@/hooks/common/router'
import { message } from '@/utils/common/discrete'
import { createDeviceUpdatePayload, syncDeviceUpdateQueryTarget, validateDeviceUpdate } from './device-edit-state'
import { isDomEventPayload, resolveDeviceAlarmActive } from './device-detail-route'
import { useDeviceDetailTabsController } from './useDeviceDetailTabsController'
import { useDeviceOnlineStatusSocket } from './useDeviceOnlineStatusSocket'
import { useDeviceDetailLoader } from './useDeviceDetailLoader'
import DeviceDetailsMeta from './components/DeviceDetailsMeta.vue'
import DeviceEditModal from './components/DeviceEditModal.vue'

const DeviceStatusHistory = defineAsyncComponent(() => import('@/views/device/details/modules/device-status.vue'))
const DeviceOperationsWorkbench = defineAsyncComponent(
  () => import('@/views/device/details/modules/DeviceOperationsWorkbench.vue')
)
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const { loading, startLoading, endLoading } = useLoading()

const showDialog = ref(false)
const showStatusHistoryDialog = ref(false)
const deviceType = ref('')

// loader 依赖 tabs 与 onlineStatus，而二者的回调又需要读取当前设备 ID：用延迟绑定打破循环。
let resolveDeviceId = () => ''
const getDeviceId = () => resolveDeviceId()

const tabs = useDeviceDetailTabsController({
  startLoading,
  endLoading,
  onDeviceTypeChange: (nextDeviceType) => {
    deviceType.value = nextDeviceType
  },
  onTabChange: (nextTabKey) => {
    router.replace({ path: route.path, query: { ...route.query, d_id: getDeviceId(), tab: nextTabKey } })
  }
})
const { changeTabs, components, remountActiveTabLabel, tabValue, tabsRenderKey } = tabs

const onlineStatus = useDeviceOnlineStatusSocket(getDeviceId)
const { isOnline: deviceIsOnline, updatedAtDisplay: deviceOnlineStatusUpdatedAtDisplay } = onlineStatus

const loader = useDeviceDetailLoader({
  route,
  tabs,
  onlineStatus,
  deviceType,
  onResetDialogs: (reason) => {
    showDialog.value = false
    if (reason === 'route-transition') showStatusHistoryDialog.value = false
  }
})
resolveDeviceId = loader.getDeviceId

const {
  deviceData,
  loadedDeviceId,
  labels,
  name,
  deviceNumber,
  isSharedReadOnly,
  canUseOwnerDetailActions,
  visibleDetailComponents
} = loader

const deviceAlarmActive = computed(() => resolveDeviceAlarmActive(deviceData.value))
const visibleDeviceOperationTabs = computed(() => visibleDetailComponents.value.map((component) => component.key))

const queryParams = reactive({ label: '', id: '', name: '', device_number: '', description: '' })

const getDeviceDetail = loader.reload

const editConfig = () => {
  if (!canUseOwnerDetailActions.value) return
  showDialog.value = true
}

const closeModal = async () => {
  await getDeviceDetail()
  showDialog.value = false
}

const { routerPushByKey } = useRouterPush()

/** 头部跳转统一经过只读权限拦截。 */
function navigateByKey(routeKey: string, query: Record<string, unknown>) {
  if (!canUseOwnerDetailActions.value) return
  routerPushByKey(routeKey as Parameters<typeof routerPushByKey>[0], { query: query as Record<string, string> })
}

const clickConfig = () => navigateByKey('device_config-detail', { id: deviceData.value?.device_config_id })
const clickGateway = () => navigateByKey('device_details', { d_id: deviceData.value?.parent_id })
const clickAlarmHistory = () => navigateByKey('alarm_warning-message', { device_id: getDeviceId() })

function openOperationWorkbenchTab(tabKey: string) {
  if (!canUseOwnerDetailActions.value) return
  changeTabs(tabKey)
}

function openStatusHistory() {
  if (!canUseOwnerDetailActions.value) return
  showStatusHistoryDialog.value = true
}

const goBack = () => router.back()

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
      loader.syncRouteTabKey(nextTabKey)
      loader.activateRequestedDetailTab(loader.routeTabKey.value)
      return
    }
    await loader.reloadFromRoute({ refreshActiveTab: true })
  }
)

function handleChildComponentChange(payload: unknown) {
  // 阻止原生 DOM input/change 事件（例如 radio/select/checkbox 切换冒泡）误触发全页详情重载
  if (isDomEventPayload(payload)) return
  getDeviceDetail()
}

const save = async () => {
  if (!canUseOwnerDetailActions.value) return
  const result = validateDeviceUpdate(deviceData.value, {
    nameRequired: $t('custom.devicePage.enterDeviceName'),
    numberRequired: $t('custom.devicePage.enterDeviceNumber'),
    numberMax: $t('custom.devicePage.deviceNumberMax')
  })
  if (!result.valid) {
    message.error(result.message || '')
    return
  }

  const payload = createDeviceUpdatePayload(deviceData.value, labels.value)
  deviceNumber.value = payload.device_number as string
  syncDeviceUpdateQueryTarget(queryParams, payload)

  const { error } = await deviceUpdate(queryParams)
  if (!error) {
    showDialog.value = false
    getDeviceDetail()
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

        <DeviceEditModal
          v-if="canUseOwnerDetailActions"
          v-model:show="showDialog"
          v-model:labels="labels"
          :device-data="deviceData"
          :compact="Boolean(getPlatform)"
          @cancel="closeModal"
          @save="save"
        />

        <DeviceStatusHistory
          v-if="canUseOwnerDetailActions"
          v-model:visible="showStatusHistoryDialog"
          :device-id="getDeviceId()"
        />

        <DeviceDetailsMeta
          :device-id="getDeviceId()"
          :device-data="deviceData"
          :device-type="deviceType"
          :online="deviceIsOnline"
          :online-updated-at-display="deviceOnlineStatusUpdatedAtDisplay"
          :alarm-active="deviceAlarmActive"
          :can-use-owner-actions="canUseOwnerDetailActions"
          @open-config="clickConfig"
          @open-gateway="clickGateway"
          @open-status-history="openStatusHistory"
          @open-alarm-history="clickAlarmHistory"
        />
      </div>
      <div class="device-details-content">
        <DeviceOperationsWorkbench
          v-if="canUseOwnerDetailActions"
          :device-id="getDeviceId()"
          :device-data="deviceData"
          :online="deviceIsOnline"
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
                :online="deviceIsOnline"
                :online-updated-at="deviceOnlineStatusUpdatedAtDisplay"
                :device-data="deviceData"
                :device-config-id="deviceData?.device_config_id || ''"
                :device-template-id="deviceData?.device_config?.device_template_id || ''"
                @change="handleChildComponentChange"
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
