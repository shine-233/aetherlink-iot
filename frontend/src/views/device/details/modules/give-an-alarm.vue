<!--
  文件用途：设备详情页中的告警历史与告警规则双面板。
  核心链路：
  1. 默认展示当前设备全部告警历史，支持可选时间范围和等级筛选；
  2. 提供告警详情查看、维护备注、确认告警和复位告警等处置动作；
  3. 在第二个 tab 中复用自动化规则列表，承接“设备级告警规则”入口。
  使用注意：
  1. 告警确认和复位都作用于真实告警记录或状态，属于有业务副作用的运维动作；
     告警历史作为审计记录永久保留，不提供删除入口；
  2. `alarm_status` 为 `N` 时表示正常/已恢复，不能简单等同于“无需审查”；
  3. 当前历史列表采用无限滚动累加加载，筛选条件变化后必须重置本地列表再重新查询。
  静态审查建议：
  1. 历史列表、详情弹窗、备注编辑、规则列表四类职责集中在同一文件，后续适合拆出局部子组件；
  2. 查询参数、滚动分页和动作回调目前都混在页面层，适合抽成 composable 提升可维护性；
  3. 多处 `any`、字典下标访问和字符串状态码让字段语义偏弱，后续可做轻量类型收敛。
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton, NCard, NFlex, NInput } from 'naive-ui'
import { EyeOutline, Refresh, Heart, HeartDislike, CreateOutline } from '@vicons/ionicons5'
import dayjs from 'dayjs'
import { $t } from '@/locales'
import { deviceAlarmHistory, deviceAlarmHistoryPut } from '@/service/api'
import { acknowledgeAlarmHistory, resetAlarmHistory } from '@/service/api/alarm'
import { useRouterPush } from '@/hooks/common/router'
import alarmDataList from '@/views/automation/scene-linkage/modules/dataList.vue'
import RdiAlarmSummary from './RdiAlarmSummary.vue'

const { routerPushByKey } = useRouterPush()

const props = defineProps<{
  id: string
}>()

// tab1 管告警历史，tab2 管设备级告警规则。
const tabValue = ref(1)
const choseTab = (data) => {
  tabValue.value = data
  if (data === 1) {
    refresh()
  }
}

type AlarmHistoryTimeRange = [number, number] | null

const createDefaultQueryParams = () => ({
  selected_time: null as AlarmHistoryTimeRange,
  alarm_status: '',
  page: 1,
  page_size: 10
})

// 页面状态保留时间选择器使用的 selected_time；请求前再转换成后端字段，避免透传 UI-only 参数。
// 每次重置或切换设备后都应从第一页重新开始累加。
const queryParams = ref(createDefaultQueryParams())
const alarmStatusOptions = ref([
  {
    label: $t('common.allStatus'),
    value: ''
  },
  {
    label: $t('common.highAlarm'),
    value: 'H'
  },
  {
    label: $t('common.intermediateAlarm'),
    value: 'M'
  },
  {
    label: $t('common.lowAlarm'),
    value: 'L'
  },
  {
    label: $t('common.normal'),
    value: 'N'
  }
])

const alarmSummaryRef = ref<InstanceType<typeof RdiAlarmSummary> | null>(null)
const refreshAlarmSummary = () => {
  void alarmSummaryRef.value?.refresh?.()
}

// 刷新会回到“全部历史”，并清空历史列表缓存后重查。
const refresh = () => {
  queryParams.value = createDefaultQueryParams()
  alarmHistory.value = []
  noMore.value = false
  getAlarmHistory()
  refreshAlarmSummary()
}
const alarmHistory = ref([] as any)
const alarmHistoryTotal = ref(0)

const buildAlarmHistoryQuery = () => {
  const { selected_time: selectedTime, ...paginationAndStatus } = queryParams.value
  const request = {
    ...paginationAndStatus,
    device_id: props.id
  }

  if (!selectedTime || selectedTime.length !== 2) return request

  return {
    ...request,
    start_time: dayjs(selectedTime[0]).format('YYYY-MM-DDTHH:mm:ssZ'),
    end_time: dayjs(selectedTime[1]).format('YYYY-MM-DDTHH:mm:ssZ')
  }
}

// 历史列表默认查询全部记录；用户显式选择时间范围后才向后端附加 start/end。
// 当前为“追加式”加载，而不是替换式刷新，因此调用前必须确认是否已清空旧列表。
const getAlarmHistory = async () => {
  const res = await deviceAlarmHistory(buildAlarmHistoryQuery())
  alarmHistory.value.push(...(res.data.list || []))
  alarmHistoryTotal.value = res.data.total
  loading.value = false
  if (alarmHistory.value.length === alarmHistoryTotal.value) {
    noMore.value = true
  }
}
const resetQuery = () => {
  queryParams.value.page = 1
  alarmHistory.value = []
  noMore.value = false
  getAlarmHistory()
}
const showDialog = ref(false)
const closeModal = () => {
  showDialog.value = false
}
const infoData = ref({} as any)

// 告警备注里可能嵌着确认/复位操作元信息，这里先兼容对象与 JSON 字符串两种形态。
function parseAlarmRemark(raw: unknown) {
  if (!raw) return {} as Record<string, unknown>
  if (typeof raw === 'object') return raw as Record<string, unknown>
  if (typeof raw !== 'string') return {} as Record<string, unknown>
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, unknown>) : {}
  } catch {
    return {} as Record<string, unknown>
  }
}
function alarmActionField(row: { remark?: unknown }, key: string) {
  const value = parseAlarmRemark(row.remark)[key]
  return value === undefined || value === null || value === '' ? '-' : String(value)
}
function isAcknowledged(row: { remark?: unknown }) {
  return parseAlarmRemark(row.remark).acknowledged === true
}

// 详情弹窗只负责展示当前选中的告警记录，不做二次查询。
const getInfo = (data: any) => {
  infoData.value = data
  showDialog.value = true
}
const showModal = ref(false)
const description = ref('')
const cancelCallback = () => {
  description.value = ''
  showModal.value = false
}

// 维护备注沿用选中告警对象的当前描述，便于运维在原内容上追加说明。
const showDescModal = (item: any) => {
  showModal.value = true
  infoData.value = item
  description.value = infoData.value.description
}
const submitCallback = async () => {
  if (description.value === '') {
    window.$message?.error($t('common.enterAlarmDesc'))
    return
  }
  const putData = {
    id: infoData.value.id,
    description: description.value
  }
  await deviceAlarmHistoryPut(putData)
  alarmHistory.value.map((item) => {
    if (item.id === infoData.value.id) {
      item.description = description.value
    }
  })
  cancelCallback()
  // await getAlarmHistory();
}

// “确认告警”与“复位告警”是两种不同的运维动作：
// 1. 确认表示已知悉并记录处理状态；
// 2. 复位表示尝试把告警从异常态切回正常态。
const acknowledgeAlarm = async (item: any) => {
  await acknowledgeAlarmHistory(item.id)
  window.$message?.success($t('rdi.overview.alarmAcknowledged'))
  refresh()
}
const resetAlarm = async (item: any) => {
  window.$dialog?.warning({
    title: $t('rdi.overview.confirmResetAlarm'),
    content: item.name || item.content || '-',
    positiveText: $t('common._confirm'),
    negativeText: $t('common._cancel'),
    onPositiveClick: async () => {
      await resetAlarmHistory(item.id)
      window.$message?.success($t('rdi.overview.alarmReset'))
      refresh()
    }
  })
}
const alarmAdd = () => {
  routerPushByKey('automation_linkage-edit', {
    query: { device_id: props.id, backType: 'device' }
  })
}
const loading = ref(false)
const noMore = ref(false)

// 无限滚动只负责推进页码与追加查询，不做去重；若后端分页不稳定，前端可能出现重复记录。
const handleLoad = () => {
  if (loading.value || noMore.value) return
  loading.value = true
  queryParams.value.page += 1
  getAlarmHistory()
}

onMounted(() => {
  getAlarmHistory()
})
</script>

<template>
  <div class="w-full">
    <NFlex justify="space-between" class="mb-4">
      <n-button-group>
        <NButton :type="tabValue === 1 ? 'primary' : 'default'" @click="choseTab(1)">
          {{ $t('common.alarmHistory') }}
        </NButton>
        <NButton :type="tabValue === 2 ? 'primary' : 'default'" @click="choseTab(2)">
          {{ $t('common.alarmRules') }}
        </NButton>
      </n-button-group>
      <NFlex v-if="tabValue === 1" class="w-70%" justify="flex-end">
        <NDatePicker
          v-model:value="queryParams.selected_time"
          type="datetimerange"
          :clearable="true"
          separator="-"
          class="w-400px"
        />
        <n-select
          v-model:value="queryParams.alarm_status"
          :options="alarmStatusOptions"
          class="w-150px"
          :clearable="false"
        />
        <NButton type="primary" @click="resetQuery">{{ $t('common.search') }}</NButton>
        <NButton @click="refresh">{{ $t('common.reset') }}</NButton>
        <NButton :bordered="false" class="justify-end" @click="refresh">
          <NIcon size="18">
            <Refresh />
          </NIcon>
          {{ $t('generate.refresh') }}
        </NButton>
      </NFlex>
      <NFlex v-if="tabValue === 2" justify="flex-end">
        <NButton type="primary" @click="alarmAdd()">{{ $t('generate.addAlarmRule') }}</NButton>
      </NFlex>
    </NFlex>
    <div v-if="tabValue === 1" class="history-list">
      <RdiAlarmSummary ref="alarmSummaryRef" :device-id="props.id" @open="getInfo" />
      <n-infinite-scroll v-if="alarmHistory.length > 0" style="height: 100%" :distance="10" @load="handleLoad">
        <div v-for="(item, index) in alarmHistory" :key="index" class="alarm-item">
          <div class="alarm-time">
            <div class="line-style"></div>
            <span class="alarm-icon" :class="[item['alarm_status'] !== 'N' ? 'color-ye-bg' : 'color-gre-bg']"></span>
            <span>{{ dayjs(item['create_at']).format('YYYY-MM-DD HH:mm:ss') }}</span>
          </div>
          <div
            class="alarm-item-content"
            :class="[item['alarm_status'] !== 'N' ? 'color-ye-bg-low' : 'color-gre-bg-low']"
          >
            <NFlex class="mb-30px" justify="space-between">
              <NFlex class="alarm-type" :class="[item['alarm_status'] !== 'N' ? 'color-ye' : 'color-gre']">
                <NIcon v-if="item['alarm_status'] !== 'N'" size="22" class="ml-1">
                  <HeartDislike />
                </NIcon>
                <NIcon v-if="item['alarm_status'] === 'N'" size="22" class="ml-1">
                  <Heart />
                </NIcon>
                <span>
                  {{ alarmStatusOptions.find((data) => data.value === item['alarm_status'])?.label || '' }}
                </span>
              </NFlex>
              <NFlex>
                <div>{{ item['name'] }}</div>
                <!--              <div style="color: #646cff">设备名称</div>-->
              </NFlex>
            </NFlex>
            <div>
              <NButton text @click="getInfo(item)">
                <NIcon size="18">
                  <EyeOutline />
                </NIcon>
                {{ $t('custom.devicePage.details') }}
              </NButton>
              <NButton text class="ml-8" @click="showDescModal(item)">
                <NIcon size="18">
                  <CreateOutline />
                </NIcon>
                {{ $t('custom.devicePage.maintenance') }}
              </NButton>
              <NButton text class="ml-8" :disabled="isAcknowledged(item)" @click="acknowledgeAlarm(item)">
                {{ $t('rdi.overview.acknowledgeAlarm') }}
              </NButton>
              <NButton text class="ml-8" :disabled="item['alarm_status'] === 'N'" @click="resetAlarm(item)">
                {{ $t('rdi.overview.resetAlarm') }}
              </NButton>
            </div>
          </div>
        </div>
        <div v-if="loading" class="text">{{ $t('card.loading') }}</div>
        <div v-if="noMore" class="text">{{ $t('card.noMore') }}</div>
      </n-infinite-scroll>
      <n-empty
        v-if="alarmHistory.length === 0"
        size="huge"
        :description="$t('common.noData')"
        class="min-h-60 justify-center"
      ></n-empty>
      <n-modal v-model:show="showDialog" :title="$t('generate.alarm-info')" class="max-w-[800px]">
        <NCard>
          <div>
            <NH3>{{ $t('generate.alarm-info') }}</NH3>
          </div>
          <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('generate.alarmConfugName')}:`">
            {{ infoData.name }}
          </n-form-item>
          <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('generate.sceneLinkageName')}:`">
            {{ infoData['alarm_config_name'] }}
          </n-form-item>
          <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('common.alarm_time')}:`">
            {{ dayjs(infoData['create_at']).format('YYYY-MM-DD HH:mm:ss') }}
          </n-form-item>
          <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('generate.alarmStatus')}:`">
            {{ alarmStatusOptions.find((data) => data.value === infoData['alarm_status'])?.label || '' }}
          </n-form-item>
          <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('generate.alarmReason')}:`">
            {{ infoData.content }}
          </n-form-item>
          <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('generate.alarm-description')}:`">
            {{ infoData.description }}
          </n-form-item>
          <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('rdi.overview.acknowledgedBy')}:`">
            {{ alarmActionField(infoData, 'acknowledged_by') }}
          </n-form-item>
          <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('rdi.overview.acknowledgedAt')}:`">
            {{ alarmActionField(infoData, 'acknowledged_at') }}
          </n-form-item>
          <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('rdi.overview.resetBy')}:`">
            {{ alarmActionField(infoData, 'reset_by') }}
          </n-form-item>
          <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('rdi.overview.resetAt')}:`">
            {{ alarmActionField(infoData, 'reset_at') }}
          </n-form-item>
          <n-form-item label-placement="top" :show-feedback="false" :label="`${$t('generate.alarmDevices')}:`">
            <NTable size="small" :bordered="false" :single-line="false" class="mb-6">
              <thead>
                <tr>
                  <th>{{ $t('generate.order-number') }}</th>
                  <th class="min-w-180px">{{ $t('generate.device-code') }}</th>
                  <th>{{ $t('generate.device-name') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(device, index) in infoData.alarm_device_list" :key="index">
                  <td class="min-w-100px">{{ Number(index) + 1 }}</td>
                  <td>{{ device.id }}</td>
                  <td>{{ device['name'] }}</td>
                </tr>
              </tbody>
            </NTable>
          </n-form-item>
          <NFlex justify="flex-end">
            <NButton @click="closeModal">{{ $t('custom.devicePage.close') }}</NButton>
          </NFlex>
        </NCard>
      </n-modal>
      <n-modal v-model:show="showModal" class="max-w-[600px]">
        <NCard>
          <n-form-item :show-feedback="false" :label="$t('generate.alarm-description')">
            <NInput v-model:value="description" type="textarea" />
          </n-form-item>
          <NFlex justify="flex-end" class="mt-4">
            <NButton @click="cancelCallback">{{ $t('generate.cancel') }}</NButton>
            <NButton @click="submitCallback">{{ $t('common.save') }}</NButton>
          </NFlex>
        </NCard>
      </n-modal>
    </div>
  </div>
  <div v-if="tabValue === 2" class="alarm-list">
    <alarmDataList :is-alarm="true" :device_id="props.id" back-type="device"></alarmDataList>
  </div>
</template>

<style scoped lang="scss">
.history-list {
  max-height: 700px;
  overflow: auto;
  height: 100%;
  .alarm-item {
    //padding: 20px;
    .alarm-time {
      display: flex;
      flex-flow: row;
      align-items: center;

      .alarm-icon {
        width: 20px;
        height: 20px;
        border-radius: 50%;
        //background: #dca550;
        display: block;
        margin-right: 20px;
        z-index: 1;
      }

      .line-style {
        position: relative;
        height: 20px;
        /* 线的高度 */
        width: 1px;
      }

      .line-style::after {
        content: '';
        position: absolute;
        left: 10px;
        right: 0;
        top: 18px;
        background: #e5e7ec;
        width: 1px;
        height: 150px;
      }
    }

    .alarm-item-content {
      //border-left: solid 2px #fdfaf6;
      //background: #fdfaf6;
      margin: 10px 40px;
      padding: 15px 10px;

      .alarm-type {
        //color: #dca550;
        margin-bottom: 30px;
      }
    }
  }
}

.color-ye {
  color: #dca550;
}

.color-ye-bg {
  background: #dca550;
}

.color-ye-bg-low {
  background: #fdfaf6;
}

.color-gre {
  color: #7ec050;
}

.color-gre-bg {
  background: #7ec050;
}

.color-gre-bg-low {
  background: #f8fcf6;
}
.text {
  text-align: center;
}
</style>
