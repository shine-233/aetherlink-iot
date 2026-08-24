<!--
  文件用途: 设备配置关联设备面板。
  核心逻辑: 查询当前配置下已关联的设备列表，并支持为当前配置批量新增关联设备。
  查询链路: 页面挂载或分页切换时读取 props.deviceConfigId，调用 deviceList 拉取列表数据。
  保存链路: 弹窗内选择设备后调用 deviceConfigBatch 建立关联，成功后关闭弹窗并回刷列表。
  关键注意事项: 设备选择下拉采用独立分页加载，弹窗状态、列表分页和配置 ID 需要分别维护。
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, getCurrentInstance, h, onMounted, ref } from 'vue'
import type { DataTableColumns, FormInst } from 'naive-ui'
import { NButton, NDataTable, NFlex, NForm, NFormItem, NModal, NPagination, NPopconfirm, useMessage } from 'naive-ui'
import dayjs from 'dayjs'
import { detachDeviceFromConfig, deviceConfigBatch, deviceList, getDeviceListForSelect } from '@/service/api'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
import DeviceSelectWithScroll from './DeviceSelectWithScroll.vue'

const message = useMessage()

interface Props {
  deviceConfigId?: string
}

const props = withDefaults(defineProps<Props>(), {
  deviceConfigId: ''
})
// 控制“新增关联设备”弹窗的显示状态。
const visible = ref(false)
const associatedFormRef = ref<HTMLElement & FormInst>()

interface AssociatedFormType {
  device_ids: string[] | null
  device_config_id: string
}
// 保存弹窗内待提交的关联设备集合，提交前会回填当前配置 ID。
const associatedForm = ref<AssociatedFormType>(defaultAssociatedForm())
// 设备选择器使用滚动分页加载，这里缓存当前已拉取的可选设备。
const deviceOptions = ref<Api.Device.DeviceSelectItem[]>([])
// 标记选择器后续是否还有更多设备可继续加载。
const hasMoreDevices = ref(true)
// 避免滚动触底时重复触发并发加载请求。
const loadingMore = ref(false)

// 下拉设备选择器的分页查询参数。
const queryDevice = ref({
  page: 1,
  page_size: 30
})

function initQueryDevice() {
  queryDevice.value = {
    page: 1,
    page_size: 30
  }
  deviceOptions.value = []
  hasMoreDevices.value = true
}

function defaultAssociatedForm() {
  return {
    device_ids: null,
    device_config_id: ''
  }
}

const queryData = ref({
  device_config_id: props.deviceConfigId,
  page: 1,
  page_size: 10
})

const associatedFormRules = ref({
  // device_ids: {
  //   required: true,
  //   message: '请选择设备',
  //   trigger: 'change'
  // },
})

const addDevice = () => {
  visible.value = true
}
const modalClose = () => {
  initQueryDevice()
  associatedForm.value = defaultAssociatedForm()
}
// 保存链路: 表单校验 -> 补齐当前配置 ID -> 调用批量关联接口 -> 成功后关闭并刷新列表。
const handleSubmit = async () => {
  await associatedFormRef?.value?.validate()

  if (!associatedForm.value.device_ids || associatedForm.value.device_ids.length === 0) {
    message.warning($t('custom.associatedDevices.selectDeviceFirst'))
    return
  }

  associatedForm.value.device_config_id = props.deviceConfigId
  const { error } = await deviceConfigBatch(associatedForm.value)
  if (!error) {
    message.success($t('common.addSuccess') || 'Added successfully')
    handleClose()
  }
}
const handleClose = () => {
  associatedFormRef.value?.restoreValidation?.()
  associatedForm.value = defaultAssociatedForm()
  visible.value = false
  queryData.value.page = 1
  getDeviceList()
}

// 查询链路: 按页加载“可关联设备”候选项，供弹窗内滚动选择器持续追加数据。
const getDeviceOptions = async (isInitialLoad = false) => {
  if (loadingMore.value) {
    console.error('Load request ignored, already loading.')
    return
  }
  if (!isInitialLoad && !hasMoreDevices.value) return

  if (isInitialLoad) {
    queryDevice.value.page = 1
    deviceOptions.value = []
    hasMoreDevices.value = true
  }

  loadingMore.value = true

  const params: Api.Device.DeviceSelectorParams = {
    page: String(queryDevice.value.page),
    page_size: String(queryDevice.value.page_size),
    has_device_config: false
  }

  try {
    const { data, error } = await getDeviceListForSelect(params)

    if (!error && data?.list) {
      deviceOptions.value.push(...data.list)

      if (data.list.length < queryDevice.value.page_size) {
        // eslint-disable-next-line require-atomic-updates
        hasMoreDevices.value = false
      } else {
        // eslint-disable-next-line require-atomic-updates
        hasMoreDevices.value = true
      }
    } else {
      // eslint-disable-next-line require-atomic-updates
      hasMoreDevices.value = false
      if (error) {
        message.error($t('common.fetchDataFailed'))
      }
    }
  } catch (apiError) {
    message.error($t('common.networkError'))
    // eslint-disable-next-line require-atomic-updates
    hasMoreDevices.value = false
  } finally {
    // eslint-disable-next-line require-atomic-updates
    loadingMore.value = false
  }
}

const handleLoadMoreDevices = () => {
  queryDevice.value.page += 1
  getDeviceOptions()
}

const handleInitialLoadDevices = () => {
  getDeviceOptions(true)
}

const configDevice = ref<DeviceManagement.DeviceData[]>([])
const configDeviceTotal = ref(0)
// 查询链路: 根据当前配置 ID 与分页参数拉取关联设备列表，并补充在线状态展示字段。
const getDeviceList = async () => {
  queryData.value.device_config_id = props.deviceConfigId
  const { data, error } = await deviceList(queryData.value)
  if (!error && data?.list) {
    data.list.forEach(sitem => {
      sitem.activate_flag = sitem.is_online === 0 ? $t('custom.devicePage.offline') : $t('custom.devicePage.online')
    })
    configDevice.value = data.list || []
    configDeviceTotal.value = data.total || 0
  } else {
    configDevice.value = []
    configDeviceTotal.value = 0
  }
}

// 解绑链路: 对单行设备发起解绑请求，成功后仅回刷当前列表。
const handleDelete = async row => {
  const { error } = await detachDeviceFromConfig({
    device_id: row.id,
    device_config_id: ''
  })
  if (!error) {
    message.success($t('card.removeSuccess') || 'Removed successfully')
    getDeviceList()
  }
}

const columnsData: Ref<DataTableColumns<any>> = ref([
  {
    key: 'name',
    minWidth: '140px',
    title: $t('custom.devicePage.deviceName')
  },
  {
    key: 'device_number',
    minWidth: '140px',
    title: $t('generate.device-code')
  },
  {
    key: 'activate_flag',
    minWidth: '140px',
    title: $t('custom.devicePage.onlineStatus')
  },
  {
    key: 'ts',
    minWidth: '140px',
    title: $t('custom.devicePage.pushTime'),
    render: row => {
      if (row.ts) {
        return dayjs(row.ts).format('YYYY-MM-DD HH:mm:ss')
      }
      return ''
    }
  },
  {
    key: 'actions',
    title: () => $t('common.actions'),
    align: 'center',
    width: '250px',
    render: row => {
      return h(
        NPopconfirm,
        {
          onPositiveClick: () => handleDelete(row)
        },
        {
          default: () => $t('common.confirmDelete'),
          trigger: () => {
            return h(
              NButton,
              {
                type: 'error',
                size: 'small',
                onClick: e => {
                  e.stopPropagation()
                }
              },
              { default: () => $t('common.remove') }
            )
          }
        }
      )
    }
  }
])

const { routerPushByKey } = useRouterPush()
const rowProps = (row: any) => {
  return {
    style: 'cursor: pointer;',
    onClick: () => {
      routerPushByKey('device_details', {
        query: {
          d_id: row.id
        }
      })
    }
  }
}
const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})
onMounted(async () => {
  await getDeviceList()
})
</script>

<template>
  <div class="associated-box">
    <NButton type="primary" @click="addDevice()">{{ $t('generate.+add-device') }}</NButton>
    <n-data-table
      :columns="columnsData"
      :data="configDevice"
      size="small"
      :row-key="item => item.id"
      class="table-class"
      :row-props="rowProps"
    >
      <template #empty>
        <n-empty :description="$t('common.noData')" class="py-8" />
      </template>
    </n-data-table>

    <div class="pagination-box">
      <NPagination
        v-model:page="queryData.page"
        :page-size="queryData.page_size"
        :item-count="configDeviceTotal"
        @update:page="getDeviceList"
      />
    </div>
    <NModal
      v-model:show="visible"
      :mask-closable="false"
      :title="$t('generate.add-device')"
      :class="getPlatform ? 'w-90%' : 'w-600px'"
      preset="card"
      @after-leave="modalClose"
    >
      <NForm
        ref="associatedFormRef"
        :model="associatedForm"
        :rules="associatedFormRules"
        label-placement="left"
        label-width="auto"
      >
        <NFormItem :label="$t('generate.select-device')" path="device_ids">
          <DeviceSelectWithScroll
            v-model:modelValue="associatedForm.device_ids"
            :options="deviceOptions"
            :loading="loadingMore"
            :has-more="hasMoreDevices"
            :placeholder="$t('generate.select-device')"
            @load-more="handleLoadMoreDevices"
            @initial-load="handleInitialLoadDevices"
          />
        </NFormItem>
        <NFlex justify="flex-end">
          <NButton @click="handleClose">{{ $t('generate.cancel') }}</NButton>
          <NButton
            type="primary"
            :disabled="!associatedForm.device_ids || associatedForm.device_ids.length === 0"
            @click="handleSubmit"
          >
            {{ $t('generate.add') }}
          </NButton>
        </NFlex>
      </NForm>
    </NModal>
  </div>
</template>

<style scoped lang="scss">
.associated-box {
  height: 100%;
}

.pagination-box {
  display: flex;
  justify-content: flex-end;
}

.table-class {
  margin: 10px;
  height: 50%;
}
</style>
