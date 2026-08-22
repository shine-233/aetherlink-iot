<!--
  文件用途：设备配置详情页壳层。
  核心链路：
  1. 从路由 query 中读取配置 ID，调用配置详情接口拉取主配置对象；
  2. 如配置绑定了物模型，再补查物模型详情并回填名称；
  3. 通过 tab 懒加载把配置详情拆分给关联设备、物模型、连接信息、数据处理、自动化、告警、扩展信息和设备设置等子模块。
  使用注意：
  1. `configForm` 是多个子模块共享的上游配置快照，字段名变化会影响多个 tab；
  2. 页面当前只在进入时读取一次路由 ID，若未来支持同页切换不同配置，需要补 watch 或路由守卫；
  3. 物模型名称依赖二次查询，不应把 `device_template_name` 为空直接解读为未绑定物模型。
  静态审查建议：
  1. 配置读取、物模型补查、tab 状态管理都堆在壳层里，后续适合拆成 composable；
  2. `getConfig` 与 `getTemplateDetail` 缺少显式错误态和 loading 态，网络抖动时页面反馈较弱；
  3. `tabKeys` 与多个子模块事件名是跨组件契约，后续重命名要集中处理，避免静默失联。
-->
<script lang="ts" setup>
import { defineAsyncComponent, ref } from 'vue'
import { useRoute } from 'vue-router'
import { NButton } from 'naive-ui'
import { deviceConfigInfo, deviceTemplateDetail } from '@/service/api/device'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
const AssociatedDevices = defineAsyncComponent(() => import('./modules/associated-devices.vue'))
const ExtendInfo = defineAsyncComponent(() => import('./modules/extend-info.vue'))
const AttributeInfo = defineAsyncComponent(() => import('./modules/attribute-info.vue'))
const ConnectionInfo = defineAsyncComponent(() => import('./modules/connection-info.vue'))
const AlarmInfo = defineAsyncComponent(() => import('./modules/alarm-info.vue'))
const Automate = defineAsyncComponent(() => import('./modules/automate.vue'))

const SettingInfo = defineAsyncComponent(() => import('@/views/device/config-detail/modules/setting-info.vue'))
const DataHandle = defineAsyncComponent(() => import('@/views/device/config-detail/modules/data-handle.vue'))

const { routerPushByKey } = useRouterPush()
const route = useRoute()

// 当前页面把路由 query.id 视为唯一配置主键；空值时页面保持占位态而不主动跳转。
const configId = ref<string>(typeof route.query.id === 'string' ? route.query.id : '')

// `configForm` 既服务头部摘要，也会作为多个 tab 的共享输入对象。
// 这里先给一份稳定空壳，避免子模块在首屏渲染时直接访问 undefined。
type ConfigFormModel = Partial<{
  [K in keyof DeviceManagement.ConfigData]: DeviceManagement.ConfigData[K] | null
}> & {
  device_template_name?: string
}
const configForm = ref<ConfigFormModel>({
  id: typeof route.query.id === 'string' ? route.query.id : '',
  additional_info: null,
  description: null,
  device_conn_type: null,
  device_template_id: null,
  device_template_name: '',
  device_type: '',
  name: '',
  protocol_config: null,
  protocol_type: null,
  remark: null,
  voucher_type: null
})

// 编辑按钮跳回配置编辑页，继续沿用同一个配置 ID。
const editConfig = () => {
  routerPushByKey('device_config-edit', { query: { id: configId.value } })
}

// 物模型名通过详情接口补齐，而不是完全信任配置快照里的冗余字段。
const getTemplateDetail = async (templateId: string) => {
  const res = await deviceTemplateDetail({ id: templateId })
  if (res.data) {
    configForm.value.device_template_name = res.data.name
  }
}

// 配置详情是整个页面的数据源入口。
// 后续所有子模块刷新动作基本都要回到这里重新同步共享配置对象。
const getConfig = async () => {
  const res = await deviceConfigInfo({ id: configId.value })
  if (!res.data) return
  configForm.value = res.data
  if (configForm.value.device_template_id) {
    configForm.value.device_template_name = ''
    getTemplateDetail(String(configForm.value.device_template_id))
  }
}

// tab key 是壳层与物模型层的公共契约，保持集中声明便于后续统一调整。
const tabKeys = {
  associatedDevices: 'associatedDevices',
  thingModel: 'thingModel',
  protocolConfig: 'protocolConfig',
  dataProces: 'dataProces',
  automate: 'automate',
  alarm: 'alarm',
  extensionInfo: 'extensionInfo',
  devicesSetting: 'devicesSetting'
}

const activeName = ref(tabKeys.associatedDevices)
if (configId.value) {
  getConfig()
  activeName.value = tabKeys.associatedDevices
}

// 物模型名称支持从头部摘要直接跳转到详情，便于排查“配置 -> 物模型”的来源链路。
const clickConfig: () => void = () => {
  routerPushByKey('device_template', {
    query: {
      id: String(configForm.value.device_template_id || '')
    }
  })
}
</script>

<template>
  <div class="h-full overflow-auto">
    <NCard :title="configForm?.name || '--'">
      <template #header-extra>
        <NButton type="primary" @click="editConfig">{{ $t('common.edit') }}</NButton>
      </template>
      <div class="mb-4 flex">
        {{ $t('generate.deviceAccessType') }}
        <template v-if="configForm.device_type === '1'">{{ $t('generate.direct-connected-device') }}</template>
        <template v-if="configForm.device_type === '2'">{{ $t('generate.gateway') }}</template>
        <template v-if="configForm.device_type === '3'">{{ $t('generate.gateway-sub-device') }}</template>
        <div class="ml-20">
          {{ $t('route.device_template') }}：
          <span style="color: blue; cursor: pointer" @click="clickConfig">
            {{
              configForm.device_template_name || configForm.device_template_name === ''
                ? configForm.device_template_name
                : $t('generate.unbound')
            }}
          </span>
        </div>
      </div>

      <n-tabs v-model:value="activeName" animated type="line">
        <n-tab-pane :name="tabKeys.associatedDevices" :tab="$t('common.associatedDevices')">
          <AssociatedDevices v-if="activeName === tabKeys.associatedDevices" :device-config-id="configId" />
        </n-tab-pane>
        <n-tab-pane :name="tabKeys.thingModel" :tab="$t('common.thingModel')">
          <AttributeInfo
            v-if="activeName === tabKeys.thingModel"
            :config-info="configForm"
            @up-date-config="getConfig"
          />
        </n-tab-pane>
        <n-tab-pane :name="tabKeys.protocolConfig" :tab="$t('common.protocolConfig')">
          <ConnectionInfo
            v-if="activeName === tabKeys.protocolConfig"
            :config-info="configForm"
            @up-date-config="getConfig"
          />
        </n-tab-pane>
        <n-tab-pane :name="tabKeys.dataProces" :tab="$t('common.dataProces')">
          <DataHandle v-if="activeName === tabKeys.dataProces" :config-info="configForm" />
        </n-tab-pane>
        <n-tab-pane :name="tabKeys.automate" :tab="$t('custom.device_details.automate')">
          <Automate v-if="activeName === tabKeys.automate" :config-id="configId" />
        </n-tab-pane>
        <n-tab-pane :name="tabKeys.alarm" :tab="$t('route.alarm')">
          <AlarmInfo v-if="activeName === tabKeys.alarm" :config-id="configId" />
        </n-tab-pane>
        <n-tab-pane :name="tabKeys.extensionInfo" :tab="$t('generate.extension-info')">
          <ExtendInfo
            v-if="activeName === tabKeys.extensionInfo"
            :config-info="configForm"
            @up-date-config="getConfig"
          />
        </n-tab-pane>
        <n-tab-pane :name="tabKeys.devicesSetting" :tab="$t('common.devicesSetting')">
          <SettingInfo v-if="activeName === tabKeys.devicesSetting" :config-info="configForm" @change="getConfig" />
        </n-tab-pane>
      </n-tabs>
    </NCard>
  </div>
</template>

<style lang="scss" scoped>
:deep(.n-card-header__main) {
  width: auto;
  flex: none !important;
}

:deep(.n-card-header__extra) {
  flex: 1;
  margin-left: 20px;
}

:deep(.n-tabs) {
  height: calc(100% - 80px);
}
.flex {
  display: flex;
}
.ml-20 {
  margin-left: 20px;
}
</style>
