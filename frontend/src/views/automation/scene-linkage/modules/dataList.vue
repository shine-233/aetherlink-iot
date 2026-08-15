<!--
文件用途: 承载DataList相关的自动化页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script lang="tsx" setup>
import { computed, getCurrentInstance, ref } from 'vue'
import { NButton, NCard, NFlex, NGrid, NGridItem, NPagination, useDialog } from 'naive-ui'
import { PencilOutline as editIcon, TrashOutline as trashIcon, DocumentTextOutline } from '@vicons/ionicons5'
import dayjs from 'dayjs'
import { useRouterPush } from '@/hooks/common/router'
import ItemCard from '@/components/dev-card-item/index.vue'
import {
  sceneAutomationsDel,
  sceneAutomationsGet,
  sceneAutomationsLog,
  sceneAutomationsSwitch
} from '@/service/api/automation'
import { $t } from '@/locales'
import { deviceAlarmList } from '@/service/api'
const dialog = useDialog()
const { routerPushByKey } = useRouterPush()

interface Props {
  deviceId?: string
  deviceConfigId?: string
  isAlarm?: boolean
  backType?: string
  onboarding?: string
  starter?: string
  firstDeviceName?: string
  firstDeviceNumber?: string
  telemetryKey?: string
  telemetryValue?: string
  telemetryAt?: string
}

const props = withDefaults(defineProps<Props>(), {
  deviceId: '',
  deviceConfigId: '',
  isAlarm: false,
  backType: 'automation',
  onboarding: '',
  starter: '',
  firstDeviceName: '',
  firstDeviceNumber: '',
  telemetryKey: '',
  telemetryValue: '',
  telemetryAt: ''
})

const sceneLinkageList = ref([] as any)

// 新建场景
const isDeviceAutomationStarter = computed(
  () =>
    !props.isAlarm &&
    (Boolean(props.deviceId) || props.onboarding === 'first-device' || props.starter === 'first-telemetry-rule')
)

const buildLinkAddQuery = () => {
  const query: Record<string, string> = {
    device_id: props.deviceId,
    device_config_id: props.deviceConfigId,
    backType: props.backType
  }

  if (isDeviceAutomationStarter.value) {
    query.onboarding = props.onboarding || 'first-device'
    query.starter = 'first-telemetry-rule'
    if (props.firstDeviceName) query.first_device_name = props.firstDeviceName
    if (props.firstDeviceNumber) query.first_device_number = props.firstDeviceNumber
    if (props.telemetryKey) query.telemetry_key = props.telemetryKey
    if (props.telemetryValue) query.telemetry_value = props.telemetryValue
    if (props.telemetryAt) query.telemetry_at = props.telemetryAt
  }

  return query
}

const linkAdd = () => {
  routerPushByKey('automation_linkage-edit', {
    query: buildLinkAddQuery()
  })
}

// 编辑场景
const linkEdit = (item: any) => {
  routerPushByKey('automation_linkage-edit', {
    query: {
      id: item.id,
      backType: props.backType,
      device_id: props.deviceId,
      device_config_id: props.deviceConfigId
    }
  })
}

// 开启/关闭场景
const linkActivation = async (item: any) => {
  const res = await sceneAutomationsSwitch(item.id)
  if (!res.error) {
    await getData()
  }
}

const queryData = ref({
  name: '',
  page: 1,
  page_size: 12,
  device_id: '',
  device_config_id: ''
})
const dataTotal = ref(0)

const getData = async () => {
  queryData.value.device_id = props.deviceId
  queryData.value.device_config_id = props.deviceConfigId
  let res: any = null
  if (props.isAlarm) {
    res = await deviceAlarmList(queryData.value)
  } else {
    res = await sceneAutomationsGet(queryData.value)
  }
  if (!res || res.error || !res.data) {
    sceneLinkageList.value = []
    dataTotal.value = 0
    return
  }
  if (res && !res.error) {
    sceneLinkageList.value = res.data.list || []
    dataTotal.value = res.data.total || 0
  }
}
const handleQuery = async () => {
  queryData.value.page = 1
  await getData()
}
const bodyStyle = ref({
  width: '1000px'
})
const execution_result_options = ref([
  {
    label: $t('custom.device_details.whole'),
    value: ''
  },
  {
    label: $t('generate.execution-successful'),
    value: 'S'
  },
  {
    label: $t('generate.execution-failed'),
    value: 'F'
  }
])
const showLog = ref(false)
const logQuery = ref({
  page: 1,
  page_size: 10,
  scene_automation_id: '',
  execution_result: '',
  execution_start_time: '',
  execution_end_time: '',
  queryTime: ref<[number, number]>([dayjs().subtract(7, 'day').valueOf(), dayjs().valueOf()])
})
const logDataTotal = ref(0)
const logData = ref([])
const queryLog = () => {
  logQuery.value.page = 1
  getLogList()
}
const getLogList = async () => {
  if (logQuery.value.queryTime) {
    logQuery.value.execution_start_time = dayjs(logQuery.value.queryTime[0]).format()
    logQuery.value.execution_end_time = dayjs(logQuery.value.queryTime[1]).format()
  }
  const res = await sceneAutomationsLog(logQuery.value)
  if (res.error || !res.data) {
    logData.value = []
    logDataTotal.value = 0
    return
  }
  logData.value = res.data.list || []
  logDataTotal.value = res.data.total || 0
}

// 查看日志
const openLog = (item: any) => {
  logQuery.value.scene_automation_id = item.id
  getLogList()
  showLog.value = true
}
// 删除场景
const deleteLink = async (item: any) => {
  dialog.warning({
    title: $t('common.deletePrompt'),
    content: $t('common.sceneLinkageInfo'),
    positiveText: $t('device_template.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: async () => {
      const res = await sceneAutomationsDel(item.id)
      if (!res.error) {
        await getData()
      }
    }
  })
}
const closeLog = () => {
  logQuery.value = {
    page: 1,
    page_size: 10,
    scene_automation_id: '',
    execution_result: '',
    execution_start_time: '',
    execution_end_time: '',
    queryTime: [dayjs().subtract(7, 'day').valueOf(), dayjs().valueOf()]
  }
  showLog.value = false
}

const getPlatform = computed(() => {
  const proxy = getCurrentInstance()?.proxy as any
  return proxy?.getPlatform?.() || false
})
getData()
</script>

<template>
  <NCard class="w-full">
    <NFlex v-if="!isAlarm" justify="space-between" class="mb-4">
      <NButton type="primary" @click="linkAdd()">{{ $t('generate.+add-scene-linkage') }}</NButton>
      <NFlex align="center" justify="flex-end" :wrap="false">
        <NInput
          v-model:value="queryData.name"
          :placeholder="$t('generate.enter-scene-linkage-name')"
          class="search-input"
          type="text"
          clearable
        ></NInput>
        <NButton class="w-72px" type="primary" @click="handleQuery">{{ $t('common.search') }}</NButton>
      </NFlex>
    </NFlex>
    <n-empty v-if="sceneLinkageList.length === 0" size="huge" class="min-h-60 justify-center">
      <template #default>
        <div class="automation-empty">
          <div class="automation-empty__title">
            {{
              isDeviceAutomationStarter
                ? $t('custom.automation.firstTelemetryRuleEmptyTitle')
                : $t('common.noData')
            }}
          </div>
          <div v-if="isDeviceAutomationStarter" class="automation-empty__desc">
            {{ $t('custom.automation.firstTelemetryRuleEmptyDesc') }}
          </div>
          <NButton v-if="isDeviceAutomationStarter" type="primary" @click="linkAdd">
            {{ $t('custom.automation.createFirstTelemetryRule') }}
          </NButton>
        </div>
      </template>
    </n-empty>
    <NGrid v-else x-gap="20px" y-gap="20px" cols="1 s:2 m:3 l:4" responsive="screen">
      <NGridItem v-for="(item, index) in sceneLinkageList" :key="index">
        <ItemCard
          :title="item.name"
          :status-active="true"
          :status-type="'success'"
          :isStatus="false"
          :hideFooterLeft="true"
          hoverable
        >
          <template #default>{{ item.description }}</template>
          <!-- 右上角开关 -->
          <template #top-right-icon>
            <n-switch
              v-model:value="item.enabled"
              checked-value="Y"
              unchecked-value="N"
              @update-value="() => linkActivation(item)"
            />
          </template>

          <!-- 底部操作按钮 -->
          <template #footer>
            <div class="flex items-center gap-2 w-full justify-between">
              <NTooltip trigger="hover">
                <template #trigger>
                  <NButton size="small" quaternary circle @click="linkEdit(item)">
                    <template #icon>
                      <n-icon color="#888">
                        <editIcon />
                      </n-icon>
                    </template>
                  </NButton>
                </template>
                {{ $t('common.edit') }}
              </NTooltip>
              <NTooltip trigger="hover">
                <template #trigger>
                  <NButton size="small" quaternary circle @click="openLog(item)">
                    <template #icon>
                      <n-icon color="#888">
                        <DocumentTextOutline />
                      </n-icon>
                    </template>
                  </NButton>
                </template>
                {{ $t('generate.log') }}
              </NTooltip>
              <NTooltip trigger="hover">
                <template #trigger>
                  <NButton size="small" quaternary circle @click="deleteLink(item)">
                    <template #icon>
                      <n-icon color="#888">
                        <trashIcon />
                      </n-icon>
                    </template>
                  </NButton>
                </template>
                {{ $t('common.delete') }}
              </NTooltip>
            </div>
          </template>
        </ItemCard>
        <!-- <NCard hoverable style="height: 180px" content-style="padding: 0px;margin: 0px;">
          <NFlex justify="space-between" align="center" class="mb-4" :wrap="false">
            <div class="mr-2 flex-1 overflow-hidden text-16px font-600">
              <n-ellipsis>
                {{ item.name }}
              </n-ellipsis>
            </div>
            <n-switch
              v-model:value="item.enabled"
              checked-value="Y"
              unchecked-value="N"
              @update-value="() => linkActivation(item)"
            />
          </NFlex>
          <n-ellipsis :line-clamp="2" class="h-40px">
            {{ item.description }}
          </n-ellipsis>
          <NFlex justify="flex-end" class="mt-4" style="display: flex; position: absolute; bottom: 15px; right: 20px">
            <NTooltip trigger="hover">
              <template #trigger>
                <NButton tertiary circle type="warning" @click="linkEdit(item)">
                  <template #icon>
                    <n-icon>
                      <editIcon />
                    </n-icon>
                  </template>
                </NButton>
              </template>
              {{ $t('common.edit') }}
            </NTooltip>
            <NTooltip trigger="hover">
              <template #trigger>
                <NButton circle tertiary type="info" @click="openLog(item)">
                  <template #icon>
                    <n-icon>
                      <DocumentTextOutline />
                    </n-icon>
                  </template>
                </NButton>
              </template>
              {{ $t('generate.log') }}
            </NTooltip>
            <NTooltip trigger="hover">
              <template #trigger>
                <NButton circle tertiary type="error" @click="deleteLink(item)">
                  <template #icon>
                    <n-icon>
                      <trashIcon />
                    </n-icon>
                  </template>
                </NButton>
              </template>
              {{ $t('common.delete') }}
            </NTooltip>
          </NFlex>
        </NCard> -->
      </NGridItem>
    </NGrid>
    <NFlex justify="flex-end" class="mt-4">
      <NPagination
        v-model:page="queryData.page"
        :page-size="queryData.page_size"
        :item-count="dataTotal"
        @update:page="getData"
      />
    </NFlex>
  </NCard>
  <n-modal
    v-model:show="showLog"
    :style="bodyStyle"
    preset="card"
    :title="$t('generate.log')"
    size="huge"
    :bordered="false"
    :class="getPlatform ? 'max-w-90%' : 'w-600px'"
    @close="closeLog()"
  >
    <NFlex class="mb-6">
      <n-date-picker v-model:value="logQuery.queryTime" type="datetimerange" @update:value="queryLog" />
      <n-select
        v-model:value="logQuery.execution_result"
        :options="execution_result_options"
        class="max-w-40"
        :placeholder="$t('generate.select-execution-status')"
        @update:value="queryLog"
      ></n-select>
      <NButton type="primary" @click="queryLog()">{{ $t('common.search') }}</NButton>
    </NFlex>
    <n-empty
      v-if="logDataTotal === 0"
      size="huge"
      :description="$t('common.noData')"
      class="min-h-60 justify-center"
    ></n-empty>
    <template v-else>
      <NTable size="small" :bordered="false" :single-line="false" class="mb-6">
        <thead>
          <tr>
            <th>{{ $t('generate.order-number') }}</th>
            <th class="min-w-180px">{{ $t('generate.execution-time') }}</th>
            <th>{{ $t('generate.execution-description') }}</th>
            <th class="min-w-120px">{{ $t('generate.execution-status') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(sceneItem, index) in logData" :key="index">
            <td class="min-w-100px">{{ index + 1 }}</td>
            <td>{{ dayjs(sceneItem['executed_at']).format('YYYY-MM-DD HH:mm:ss') }}</td>
            <td>{{ sceneItem['detail'] }}</td>
            <td>
              <span v-if="sceneItem['execution_result'] === 'S'">{{ $t('generate.execution-successful') }}</span>
              <span v-if="sceneItem['execution_result'] === 'F'">{{ $t('generate.execution-failed') }}</span>
            </td>
          </tr>
        </tbody>
      </NTable>
      <NFlex justify="end">
        <NPagination
          v-model:page="logQuery.page"
          :page-size="logQuery.page_size"
          :item-count="logDataTotal"
          @update:page="getLogList"
        />
      </NFlex>
    </template>
  </n-modal>
</template>

<style scoped lang="scss">
.config-content {
  display: flex;
  flex-flow: row;
  justify-content: flex-start;
  align-items: center;
  flex-wrap: wrap;
  padding: 10px 0;

  .scene-item {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    padding: 18px;
    flex: 0 0 26%;
    margin-right: calc(20% / 2);
    margin-bottom: 30px;

    .item-name {
      display: flex;
      flex-flow: row;
      align-items: center;
      justify-content: space-between;
    }

    .item-desc {
      margin: 15px 0;
    }

    .item-operate {
      display: flex;
      flex-flow: row;
      justify-content: space-between;
      align-items: center;
    }
  }

  .scene-item:hover {
    cursor: pointer;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
  }

  /* 去除每行尾多余的边距 */
  .scene-item:nth-child(3n) {
    margin-right: 0;
  }
}

.pagination-box {
  display: flex;
  justify-content: flex-end;
}

.search-input {
  width: 200px;
}

.log-card {
  width: 600px;
}

.automation-empty {
  display: grid;
  justify-items: center;
  gap: 10px;
  max-width: 520px;
  text-align: center;
}

.automation-empty__title {
  font-size: 16px;
  font-weight: 700;
  color: #1f2937;
}

.automation-empty__desc {
  color: #4b5563;
  line-height: 1.55;
}
</style>
