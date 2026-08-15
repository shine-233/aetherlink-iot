<!--
文件用途: 承载设备分组详情相关的设备页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { type DataTableColumns, NButton, NDataTable, type PaginationProps, useMessage } from 'naive-ui'
import {
  deleteDeviceGroup,
  deleteDeviceGroupRelation,
  deviceGroupDetail,
  deviceListByGroup,
  getDeviceGroup
} from '@/service/api/device'
import { createNoSelectDeviceColumns, group_columns } from '@/views/device/modules/all-columns'
import useLoadingEmpty from '@/hooks/common/use-loading-empty'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import { useRouterPush } from '@/hooks/common/router'

type GroupDetailTab = 'subGroup' | 'device' | 'setting'

type DeviceGroupStatistics = {
  device_total?: number
  online_total?: number
  offline_total?: number
  alarm_total?: number
}

const AddOrEditDevices = defineAsyncComponent(() => import('@/views/device/grouping/components/add-or-edit-devices/index.vue'))
const DeviceSelectList = defineAsyncComponent(() => import('@/views/device/grouping-details/modules/device-select-list.vue'))

const group_data = ref([])
const device_data = ref<DeviceManagement.DeviceData[]>([])

const { loading, startLoading, endLoading } = useLoadingEmpty(false)
const route = useRoute()

const currentId = ref(route.query.id)
const activeGroupDetailTab = ref<GroupDetailTab>('subGroup')
const deviceListLoadedForGroup = ref<string | null>(null)
const isEdit = ref(true)
const the_modal1 = ref()
const the_modal2 = ref()
const renderAddChildGroupModal = ref(false)
const renderEditGroupModal = ref(false)
const pendingAddChildGroupModalOpen = ref(false)
const pendingEditGroupModalOpen = ref(false)

const editData = ref({ id: '', parent_id: '', name: '', description: '' })

const addChildData = reactive({
  id: '',
  parent_id: currentId.value as string,
  name: '',
  description: ''
})
const details_data = ref({
  detail: {
    created_at: '',
    description: '',
    id: '',
    name: '',
    parent_id: '',
    remark: '',
    tenant_id: '',
    tier: 0,
    updated_at: ''
  },
  tier: {
    group_path: ''
  },
  statistics: {
    device_total: 0,
    online_total: 0,
    offline_total: 0,
    alarm_total: 0
  }
})
const message = useMessage()

const groupStatisticValue = (key: keyof DeviceGroupStatistics) => {
  const value = details_data.value.statistics?.[key]
  return Number.isFinite(Number(value)) ? Number(value) : 0
}

const groupStatisticCards = computed(() => [
  {
    key: 'device_total',
    label: $t('custom.grouping_details.totalDevices'),
    value: groupStatisticValue('device_total'),
    tone: 'default'
  },
  {
    key: 'online_total',
    label: $t('custom.grouping_details.onlineDevices'),
    value: groupStatisticValue('online_total'),
    tone: 'success'
  },
  {
    key: 'offline_total',
    label: $t('custom.grouping_details.offlineDevices'),
    value: groupStatisticValue('offline_total'),
    tone: 'muted'
  },
  {
    key: 'alarm_total',
    label: $t('custom.grouping_details.alarmDevices'),
    value: groupStatisticValue('alarm_total'),
    tone: 'danger'
  }
])

const queryParams = reactive<{
  parent_id: string
  page: number
  page_size: number
}>({
  parent_id: '',
  page: 1,
  page_size: 10
})

const { routerPush } = useRouterPush()

const getChildGroups = async (tid: string) => {
  queryParams.parent_id = tid
  const res2 = await getDeviceGroup(queryParams)
  group_data.value = res2.data?.list ?? []
  group_pagination.itemCount = res2.data?.total ?? 0
}

const getDetails = async (tid: string) => {
  if (!currentId.value) {
    message.error('00')
  } else {
    startLoading()
    const { data, error } = await deviceGroupDetail({ id: tid })

    if (!error && data) {
      details_data.value = data as typeof details_data.value
      editData.value.id = data.detail.id
      editData.value.description = data.detail.description
      editData.value.name = data.detail.name
      editData.value.parent_id = data.detail.parent_id
    }

    await getChildGroups(tid)
    endLoading()
  }
}
const refreshChildGroups = async () => {
  startLoading()
  await getChildGroups(currentId.value as string)
  endLoading()
}
const group_pagination: PaginationProps = reactive({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  pageSizes: [10, 15, 20, 25, 30],
  onChange: (page: number) => {
    group_pagination.page = page
    queryParams.page = page
    refreshChildGroups()
  },
  onUpdatePageSize: (pageSize: number) => {
    group_pagination.pageSize = pageSize
    group_pagination.page = 1
    queryParams.page = 1
    queryParams.page_size = pageSize
    refreshChildGroups()
  }
})
const router = useRouter()
const viewDetails = (rid: string) => {
  router.push({ name: 'device_grouping-details', query: { id: rid } })
}
// Function to delete a device group
const deleteItem = async (rid: string) => {
  await deleteDeviceGroup({ id: rid })
  await getDetails(currentId.value as string)
}
const group_column = group_columns(viewDetails, deleteItem)
const openRenderedModal = (modalRef: typeof the_modal1, pendingOpen: typeof pendingAddChildGroupModalOpen) => {
  if (modalRef.value) {
    modalRef.value.showModal = true
    return
  }
  pendingOpen.value = true
}
const showGroupModal = () => {
  isEdit.value = true
  renderEditGroupModal.value = true
  openRenderedModal(the_modal2, pendingEditGroupModalOpen)
}

const showGroupDeviceModal = ref(false)
const handleChildChange = (newValue: boolean) => {
  showGroupDeviceModal.value = newValue
}
const showGroupModalChild = () => {
  addChildData.parent_id = currentId.value as string
  renderAddChildGroupModal.value = true
  openRenderedModal(the_modal1, pendingAddChildGroupModalOpen)
}

const queryParams2 = reactive<{
  group_id: string
  page: number
  page_size: number
}>({
  group_id: currentId.value as string,
  page: 1,
  page_size: 5
})
const getDeviceList = async (id: string) => {
  queryParams2.group_id = id
  const res = await deviceListByGroup({ ...queryParams2, group_id: id })
  if (res.data?.list) {
    device_data.value = res.data?.list
  } else {
    device_data.value = []
  }
  const total = res?.data?.total ?? 0
  devicePagination.pageCount = Math.ceil(total / queryParams2.page_size) || 1
  deviceListLoadedForGroup.value = id
}
const ensureDeviceListLoaded = async (id: string) => {
  if (deviceListLoadedForGroup.value === id) return
  await getDeviceList(id)
}
const refreshData = (newValue: boolean) => {
  if (newValue) {
    deviceListLoadedForGroup.value = null
    if (activeGroupDetailTab.value === 'device') {
      getDeviceList(currentId.value as string)
    }
  }
}
const devicePagination = reactive<PaginationProps>({
  page: 1,
  pageSize: 5,
  onChange: (page: number) => {
    devicePagination.page = page
    queryParams2.page = page
    getDeviceList(currentId.value as string)
  }
})
const viewDeviceDetails = (rid: string) => {
  router.push({ name: 'device_details', query: { d_id: rid } })
}
const deleteDeviceItem = async (rid: string) => {
  await deleteDeviceGroupRelation({
    device_id: rid,
    group_id: currentId.value
  })
  await getDeviceList(currentId.value as string)
}
const deviceColumns: DataTableColumns<DeviceManagement.DeviceData> = createNoSelectDeviceColumns(
  viewDeviceDetails,
  deleteDeviceItem
)
onMounted(async () => {
  await getDetails(currentId.value as string)
})
const reload = async (nid: string) => {
  deviceListLoadedForGroup.value = null
  await getDetails(nid)
  if (activeGroupDetailTab.value === 'device') {
    await ensureDeviceListLoaded(nid)
  }
}
const handleGroupDetailTabUpdate = async (tabName: GroupDetailTab) => {
  activeGroupDetailTab.value = tabName
  if (tabName === 'device') {
    await ensureDeviceListLoaded(currentId.value as string)
  }
}

/**
 * 导航到父级分组详情页
 */
const goToParentGroup = () => {
  if (details_data.value.detail.parent_id && details_data.value.detail.parent_id !== '0') {
    routerPush({ name: 'device_grouping-details', query: { id: details_data.value.detail.parent_id } })
  } else {
    console.error('无法导航到父级分组，parent_id 无效或为顶级:', details_data.value.detail.parent_id)
  }
}

/**
 * 导航到顶层分组列表页
 */
const goToGroupListRoot = () => {
  routerPush({ name: 'device_grouping' })
}

watch(
  () => route.query.id,
  newId => {
    if (newId) {
      currentId.value = newId
      reload(newId as string)
    }
  }
)
watch(the_modal1, modal => {
  if (!pendingAddChildGroupModalOpen.value || !modal) return
  modal.showModal = true
  pendingAddChildGroupModalOpen.value = false
})
watch(the_modal2, modal => {
  if (!pendingEditGroupModalOpen.value || !modal) return
  modal.showModal = true
  pendingEditGroupModalOpen.value = false
})
</script>

<template>
  <div>
    <NSpace vertical :size="16">
      <NCard :title="details_data.detail.name">
        <template #header-extra>
          <NSpace>
            <NButton v-if="details_data.detail.parent_id !== '0'" type="primary" @click="goToParentGroup">
              <template #icon>
                <svg-icon icon="material-symbols:arrow-upward" />
              </template>
              {{ $t('custom.grouping_details.parentLevel') }}
            </NButton>
            <NButton @click="goToGroupListRoot">
              {{ $t('custom.grouping_details.allGroups') }}
            </NButton>
          </NSpace>
        </template>
        <div class="group-statistics-overview" data-testid="group-statistics-overview">
          <div
            v-for="item in groupStatisticCards"
            :key="item.key"
            class="group-statistics-card"
            :class="`group-statistics-card--${item.tone}`"
          >
            <span class="group-statistics-label">{{ item.label }}</span>
            <strong class="group-statistics-value">{{ item.value }}</strong>
          </div>
        </div>
        <NTabs v-model:value="activeGroupDetailTab" type="line" animated @update:value="handleGroupDetailTabUpdate">
          <NTabPane name="subGroup" :tab="$t('custom.grouping_details.subGroup')">
            <NSpace>
              <NButton type="primary" @click="showGroupModalChild">
                {{ $t('custom.grouping_details.addSubGroup') }}
              </NButton>
            </NSpace>
            <NSpace class="mt4">
              <NDataTable
                :columns="group_column"
                :data="group_data"
                :loading="loading"
                remote
                :pagination="group_pagination"
                class="h-auto"
              ></NDataTable>
            </NSpace>
            <AddOrEditDevices
              v-if="renderAddChildGroupModal"
              ref="the_modal1"
              :is-edit="false"
              :edit-data="addChildData"
              is-pid-no-edit
              :refresh-data="
                () => {
                  getDetails(currentId as string)
                }
              "
            />
          </NTabPane>

          <NTabPane name="device" :tab="$t('custom.grouping_details.device')">
            <NSpace class="mb6">
              <NButton type="primary" @click="showGroupDeviceModal = true">
                {{ $t('custom.grouping_details.addDeviceToGroup') }}
              </NButton>
            </NSpace>

            <NDataTable :columns="deviceColumns" :data="device_data" :loading="loading" class="h-auto"></NDataTable>
            <NFlex justify="end" class="mt-4">
              <NPagination
                v-model:page="devicePagination.page"
                v-model:page-size="devicePagination.pageSize"
                :page-count="devicePagination.pageCount"
                @update:page="devicePagination.onChange"
              />
            </NFlex>
          </NTabPane>

          <NTabPane name="setting" :tab="$t('custom.grouping_details.setting')">
            <NButton type="primary" @click="showGroupModal">{{ $t('custom.grouping_details.edit') }}</NButton>
            <NDescriptions label-class="min-w-100px" label-placement="top" bordered :column="3">
              <NDescriptionsItem :label="$t('custom.grouping_details.groupLevel')">
                {{ details_data.tier.group_path }}
              </NDescriptionsItem>
              <NDescriptionsItem :label="$t('custom.grouping_details.description')">
                {{ details_data.detail.description }}
              </NDescriptionsItem>
              <NDescriptionsItem :label="$t('custom.grouping_details.createTime')">
                {{ formatDateTime(details_data.detail.created_at) }}
              </NDescriptionsItem>
            </NDescriptions>
            <AddOrEditDevices
              v-if="renderEditGroupModal"
              ref="the_modal2"
              :is-edit="true"
              :edit-data="editData"
              :refresh-data="
                () => {
                  getDetails(currentId as string)
                }
              "
            />
          </NTabPane>
        </NTabs>
      </NCard>
    </NSpace>

    <NModal v-model:show="showGroupDeviceModal">
      <NCard
        style="width: 800px"
        :title="$t('custom.grouping_details.addDeviceToGroup')"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        <DeviceSelectList
          v-if="showGroupDeviceModal"
          :group-id="currentId as string"
          @closed-modal="handleChildChange"
          @refresh-data="refreshData"
        />
      </NCard>
    </NModal>
  </div>
</template>

<style scoped>
.group-statistics-overview {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(136px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.group-statistics-card {
  display: flex;
  min-height: 72px;
  flex-direction: column;
  justify-content: center;
  border: 1px solid #e5e7eb;
  border-left: 4px solid #94a3b8;
  border-radius: 8px;
  background: #fff;
  padding: 12px 14px;
}

.group-statistics-card--success {
  border-left-color: #16a34a;
}

.group-statistics-card--danger {
  border-left-color: #dc2626;
}

.group-statistics-card--muted {
  border-left-color: #64748b;
}

.group-statistics-label {
  color: #64748b;
  font-size: 13px;
  line-height: 1.4;
}

.group-statistics-value {
  margin-top: 4px;
  color: #0f172a;
  font-size: 24px;
  font-weight: 600;
  line-height: 1.2;
}
</style>
