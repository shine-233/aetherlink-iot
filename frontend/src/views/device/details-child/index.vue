<!--
设备子详情页，负责承载设备详情、tab 编排、在线状态展示和编辑入口。
静态维护重点：
1. 路由参数 `d_id` 是页面数据加载的唯一入口，所有详情与刷新逻辑都应围绕它展开。
2. 详情接口决定当前设备可见的 tab，在线状态会同步映射到颜色和图标样式。
3. 语言切换会重新挂载当前 tab，用于刷新 Naive UI 的标签文本。
-->
<script setup lang="ts">
import { defineAsyncComponent, markRaw, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useLoading } from '@aetherlink/hooks'
import { useDeviceDataStore } from '@/store/modules/device'
import { $t } from '@/locales'
import { useAppStore } from '@/store/modules/app'
import { deviceDetail, deviceUpdate } from '@/service/api/device'
import { useRouterPush } from '@/hooks/common/router'
import { message } from '@/utils/common/discrete'

const ONLINE_COLOR = 'rgb(2,153,52)'
const DEFAULT_STATUS_COLOR = '#ccc'
const TAB_LOADING_DELAY = 500
const LOCALE_REFRESH_DELAY = 50
const MAX_DEVICE_NUMBER_LENGTH = 100

const { query } = useRoute()
const appStore = useAppStore()
const d_id = Array.isArray(query.d_id) ? query.d_id[0] || '' : typeof query.d_id === 'string' ? query.d_id : ''
const { loading, startLoading, endLoading } = useLoading()
const deviceDataStore = useDeviceDataStore()

/**
 * 这里维护的是子详情页固定 tab 壳层。
 * 与主详情页不同，这个页面不做复杂能力裁剪，只在子设备和 join 页面上按设备类型做轻量过滤。
 */
const createAsyncChildTab = (loader: () => Promise<any>) => markRaw(defineAsyncComponent(loader))

const createTabComponents = () => [
  {
    key: 'telemetry',
    name: () => $t('custom.device_details.telemetry'),
    component: createAsyncChildTab(() => import('@/views/device/details/modules/telemetry/telemetry.vue'))
  },
  {
    key: 'join',
    name: () => $t('custom.device_details.join'),
    component: createAsyncChildTab(() => import('@/views/device/details/modules/join.vue'))
  },
  {
    key: 'device-analysis',
    name: () => $t('custom.device_details.subdevice'),
    component: createAsyncChildTab(() => import('@/views/device/details/modules/device-analysis.vue'))
  },
  {
    key: 'message',
    name: () => $t('custom.device_details.AdditionalDetails'),
    component: createAsyncChildTab(() => import('@/views/device/details/modules/message.vue'))
  },
  {
    key: 'stats',
    name: () => $t('custom.device_details.attributes'),
    component: createAsyncChildTab(() => import('@/views/device/details/modules/stats.vue'))
  },
  {
    key: 'event-report',
    name: () => $t('custom.device_details.eventReport'),
    component: createAsyncChildTab(() => import('@/views/device/details/modules/event-report.vue'))
  },
  {
    key: 'command-delivery',
    name: () => $t('custom.device_details.commandDelivery'),
    component: createAsyncChildTab(() => import('@/views/device/details/modules/command-delivery.vue'))
  },
  {
    key: 'automate',
    name: () => $t('custom.device_details.automate'),
    component: createAsyncChildTab(() => import('@/views/device/details/modules/automate.vue'))
  },
  {
    key: 'give-an-alarm',
    name: () => $t('custom.device_details.giveAnAlarm'),
    component: createAsyncChildTab(() => import('@/views/device/details/modules/give-an-alarm.vue'))
  },
  {
    key: 'settings',
    name: () => $t('custom.device_details.settings'),
    component: createAsyncChildTab(() => import('@/views/device/details/modules/settings.vue'))
  },
]

const components = ref(createTabComponents())
const tabValue = ref('telemetry')
const showDialog = ref(false)
const labels = ref<string[]>([])
const device_color = ref(DEFAULT_STATUS_COLOR)
const icon_type = ref('')
const device_number = ref('')

// 弹窗关闭或保存完成后统一刷新 store 中的详情，避免页面头部信息与 tab 内容脱节。
const refreshDeviceData = () => {
  if (!d_id) return

  deviceDataStore.fetchData(d_id as string)
}

// 根据设备类型裁剪子设备页和 join 页，避免无效 tab 对最终用户暴露。
const resetTabComponentsByDeviceType = (type?: string) => {
  let nextComponents = createTabComponents()

  if (type !== '2') {
    nextComponents = nextComponents.filter(item => item.key !== 'device-analysis')
  }

  if (type === '3') {
    nextComponents = nextComponents.filter(item => item.key !== 'join')
  }

  components.value = nextComponents
}

// 子详情页只负责把在线状态映射为 UI 颜色，不在这里处理更复杂的状态来源兼容。
const syncDeviceStatus = (isOnline?: number) => {
  const statusColor = isOnline !== 0 ? ONLINE_COLOR : DEFAULT_STATUS_COLOR
  device_color.value = statusColor
  icon_type.value = statusColor
}

// 保留轻量 loading 过渡，避免切换 tab 时内容瞬时跳变。
const changeTabs = (value: string) => {
  startLoading()
  tabValue.value = value
  setTimeout(() => {
    endLoading()
  }, TAB_LOADING_DELAY)
}

const editConfig = () => {
  showDialog.value = true
}

const close = () => {
  showDialog.value = false
  refreshDeviceData()
}

// 保存仅提交页内可编辑字段，详情主体仍以 store 拉取结果为准。
const save = async () => {
  const device = deviceDataStore?.deviceData

  if (!device?.name) {
    message.error($t('custom.devicePage.enterDeviceName'))
    return
  }

  if (!device?.device_number) {
    message.error($t('custom.devicePage.enterDeviceNumber'))
    return
  }

  if (device.device_number.length > MAX_DEVICE_NUMBER_LENGTH) {
    message.error($t('custom.devicePage.deviceNumberMax'))
    return
  }

  device_number.value = device.device_number

  const { error } = await deviceUpdate({
    id: device.id,
    name: device.name,
    device_number: device.device_number,
    label: labels.value.join(','),
    description: device.description
  })

  if (!error) {
    showDialog.value = false
    refreshDeviceData()
  }
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

// 这里单独拉一次详情，用于同步页头状态和按设备类型裁剪 tab。
const getDeviceDetail = async () => {
  if (!d_id) return

  const { data, error } = await deviceDetail(d_id)

  if (error) {
    return
  }

  device_number.value = data.device_number
  syncDeviceStatus(data.is_online)

  if (data.device_config !== undefined) {
    resetTabComponentsByDeviceType(data.device_config.device_type)
    return
  }

  resetTabComponentsByDeviceType()
}

const { routerPushByKey } = useRouterPush()

// 设备配置详情仍复用既有配置详情路由，不在这里额外拼装业务上下文。
const clickConfig = () => {
  routerPushByKey('device_config-detail', {
    query: {
      id: deviceDataStore?.deviceData?.device_config_id
    }
  })
}

onMounted(() => {
  getDeviceDetail()
  refreshDeviceData()
})

watch(
  () => appStore.locale,
  () => {
    // 语言切换后通过短暂卸载再挂载当前 tab，强制刷新 Naive UI 的标签文本。
    const currentTab = tabValue.value
    tabValue.value = ''
    setTimeout(() => {
      tabValue.value = currentTab
    }, LOCALE_REFRESH_DELAY)
  }
)
</script>

<template>
  <div>
    <n-card>
      <div>
        <div style="display: flex; margin-top: -5px">
          <span style="margin-right: 20px">{{ deviceDataStore?.deviceData?.name || '--' }}</span>
          <NButton type="primary" style="margin-top: -5px" @click="editConfig">
            {{ $t('common.edit') }}
          </NButton>
        </div>

        <n-modal v-model:show="showDialog" :title="$t('generate.issue-attribute')" class="w-[400px]">
          <n-card>
            <n-form :model="deviceDataStore.deviceData" :rules="rules">
              <div>
                <NH3>{{ $t('generate.modify-device-info') }}</NH3>
              </div>
              <n-form-item :label="$t('custom.devicePage.deviceName')" path="name">
                <n-input v-model:value="deviceDataStore.deviceData.name" aria-required="true" />
              </n-form-item>
              <n-form-item :label="$t('generate.device-number')" path="device_number">
                <n-input v-model:value="deviceDataStore.deviceData.device_number" />
              </n-form-item>
              <n-form-item :label="$t('custom.devicePage.label')" path="label">
                <n-dynamic-tags v-model:value="labels" />
              </n-form-item>
              <n-form-item :label="$t('generate.device-description')">
                <NInput v-model:value="deviceDataStore.deviceData.description" type="textarea" />
              </n-form-item>
              <n-space>
                <n-button @click="close">{{ $t('generate.cancel') }}</n-button>
                <n-button @click="save">{{ $t('common.save') }}</n-button>
              </n-space>
            </n-form>
          </n-card>
        </n-modal>

        <NFlex style="margin-top: 8px">
          <div class="mr-4">
            <span class="mr-2" style="color: #ccc">ID:</span>
            <span style="color: #ccc">{{ device_number || '--' }}</span>
          </div>
          <div class="mr-4" style="color: #ccc">
            <span class="mr-2">{{ $t('custom.device_details.deviceConfig') }}:</span>
            <span style="color: blue; cursor: pointer" @click="clickConfig">
              {{ deviceDataStore?.deviceData?.device_config_name || '--' }}
            </span>
          </div>
          <div class="mr-4" style="display: flex">
            <SvgIcon
              local-icon="CellTowerRound"
              style="color: #ccc; margin-right: 5px"
              class="text-20px text-primary"
              :stroke="icon_type"
            />
            <span :style="{ color: device_color }">
              {{
                deviceDataStore?.deviceData?.is_online === 1
                  ? $t('custom.device_details.online')
                  : $t('custom.device_details.offline')
              }}
            </span>
          </div>
          <div class="mr-4" style="display: flex">
            <SvgIcon
              local-icon="AlertFilled"
              style="color: #ccc; margin-right: 5px"
              class="text-20px text-primary"
              :stroke="icon_type"
            />
            <span style="color: #ccc">
              {{ $t('custom.device_details.noAlarm') }}
            </span>
          </div>
        </NFlex>
      </div>
      <n-divider title-placement="left"></n-divider>
      <div>
        <n-tabs v-model:value="tabValue" animated type="line" @update:value="changeTabs">
          <n-tab-pane
            v-for="component in components"
            :key="component.key"
            :tab="component.name"
            :name="component.key"
            display-directive="show:lazy"
          >
            <n-spin size="small" :show="loading">
              <component
                :is="component.component"
                :id="d_id as string"
                :device-config-id="deviceDataStore?.deviceData?.device_config_id || ''"
              />
            </n-spin>
          </n-tab-pane>
        </n-tabs>
      </div>
    </n-card>
  </div>
</template>
