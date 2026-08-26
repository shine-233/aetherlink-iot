<!--
文件用途: 承载Device Select List相关的设备页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import type { DataTableColumns, DataTableRowKey, PaginationProps } from 'naive-ui'
import { NDataTable, NEmpty } from 'naive-ui'
import { deviceGroupRelation, deviceList } from '@/service/api/device'
import { createDeviceColumns } from '@/views/device/modules/all-columns'
import { $t } from '@/locales'

const emit = defineEmits<{
  (event: 'closedModal', visible: boolean): void
  (event: 'refreshData', shouldRefresh: boolean): void
}>()

interface TProps {
  groupId: string
}

const props = defineProps<TProps>()
const data = ref<DeviceManagement.DeviceData[]>([])
const checkedRowKeysRef = ref<DataTableRowKey[]>([])
const searchKeyword = ref('')

const queryParams = reactive<{ page: number; page_size: number; search?: string }>({
  page: 1,
  page_size: 5
})
const getDeviceList = async () => {
  queryParams.search = searchKeyword.value.trim() || undefined
  const res = await deviceList(queryParams)
  const rows = Array.isArray(res.data?.list) ? res.data.list : []
  data.value = rows.filter(item => item.group_id !== props.groupId) as DeviceManagement.DeviceData[]
  if (res?.data?.total) {
    pagination.pageCount = Math.ceil(res?.data?.total / 5)
  } else {
    pagination.pageCount = 1
  }
}
const pagination = reactive<PaginationProps>({
  page: 1,
  pageSize: 5,
  onChange: (page: number) => {
    pagination.page = page
    queryParams.page = page
    getDeviceList()
  }
})
const rowKey = (row: DeviceManagement.DeviceData) => row.id

const deviceColumns: DataTableColumns<DeviceManagement.DeviceData> = createDeviceColumns()

function handleCheck(rowKeys: DataTableRowKey[]) {
  checkedRowKeysRef.value = rowKeys
}

const handleSearch = () => {
  pagination.page = 1
  queryParams.page = 1
  checkedRowKeysRef.value = []
  getDeviceList()
}

const handleReset = () => {
  searchKeyword.value = ''
  handleSearch()
}

// 定义 emit 函数，指定可能发出的事件类型

const closeModal = () => {
  emit('closedModal', false)
}

const reload = () => {
  emit('refreshData', true)
}

interface ruquestParams {
  device_id_list: string[]
  group_id: string
}

const submit_data = async () => {
  const params: ruquestParams = {
    device_id_list: checkedRowKeysRef.value as string[],
    group_id: props.groupId
  }
  await deviceGroupRelation(params)
  reload()
  closeModal()
}
onMounted(getDeviceList)
</script>

<template>
  <NFlex justify="space-between" align="center" class="mb-4">
    <NInput
      v-model:value.trim="searchKeyword"
      clearable
      :placeholder="$t('custom.devicePage.deviceNameOrNumber')"
      style="max-width: 320px"
      @keyup.enter="handleSearch"
    >
      <template #prefix>
        <SvgIcon icon="material-symbols:search" />
      </template>
    </NInput>
    <NFlex align="center">
      <NButton type="primary" @click="handleSearch">{{ $t('common.search') }}</NButton>
      <NButton quaternary @click="handleReset">{{ $t('common.reset') }}</NButton>
    </NFlex>
  </NFlex>
  <NDataTable
    size="small"
    :columns="deviceColumns"
    :data="data"
    :row-key="rowKey"
    :checked-row-keys="checkedRowKeysRef"
    class="h-auto"
    @update:checked-row-keys="handleCheck"
  >
    <template #empty>
      <NEmpty :description="$t('common.noData')" class="py-24px" />
    </template>
  </NDataTable>
  <NFlex justify="end" class="mt-4">
    <NPagination
      v-model:page="pagination.page"
      v-model:page-size="pagination.pageSize"
      :page-count="pagination.pageCount"
      @update:page="pagination.onChange"
    />
  </NFlex>
  <NFlex justify="end" class="mt-5" align="center">
    <NFlex align="center">
      <div class="text-16px">
        {{ $t('generate.selected') }}
        <span class="text-blue-4">{{ checkedRowKeysRef.length }}</span>
        {{ $t('generate.number-of-devices') }}
      </div>
      <NButton type="info" @click="closeModal">{{ $t('custom.grouping_details.cancel') }}</NButton>
      <NButton type="primary" @click="submit_data">{{ $t('custom.grouping_details.confirm') }}</NButton>
    </NFlex>
  </NFlex>
</template>

<style scoped></style>
