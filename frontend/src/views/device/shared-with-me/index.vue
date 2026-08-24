<!--
  文件用途: 当前用户收到的共享 RDI 设备列表页。
  核心逻辑: 查询 shared-with-me 设备列表，处理加载/空状态，并提供跳转到共享设备详情的入口。
  关键注意事项: 权限、空列表、接口失败和详情跳转参数会影响分享闭环体验。
  重构建议: 抽出列表数据 normalize 和跳转 helper，并与 share 接受页一起维护端到端测试。
-->
<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { DataTableColumns, PaginationProps } from 'naive-ui'
import { NButton, NTag } from 'naive-ui'
import { rdiSharedWithMeDevices } from '@/service/api/rdi'
import type { RDISharedDeviceRecord } from '@/service/api/rdi'
import { $t } from '@/locales'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const tableData = ref<RDISharedDeviceRecord[]>([])
const detailVisible = ref(false)
const selectedRecord = ref<RDISharedDeviceRecord | null>(null)

const queryParams = reactive({
  page: 1,
  page_size: 10,
  device_id: '',
  device_name: ''
})

function formatTime(value?: number) {
  if (!value) return '-'
  return new Date(value * 1000).toLocaleString()
}

function showDeviceDetails(record: RDISharedDeviceRecord) {
  selectedRecord.value = record
  detailVisible.value = true
}

function openDeviceDetails(record: RDISharedDeviceRecord) {
  const deviceId = record.device?.device_id
  if (!deviceId) return
  router.push({ name: 'device_details', query: { d_id: deviceId, access: 'shared' } })
}

function goBack() {
  router.back()
}

const columns: DataTableColumns<RDISharedDeviceRecord> = [
  {
    key: 'device_name',
    title: () => $t('rdi.sharedWithMe.deviceName'),
    minWidth: 180,
    render: row =>
      h(
        NButton,
        {
          text: true,
          type: 'primary',
          'data-testid': 'shared-with-me-device-name',
          onClick: () => showDeviceDetails(row)
        },
        { default: () => row.device?.device_name || '-' }
      )
  },
  {
    key: 'device_id',
    title: () => $t('rdi.sharedWithMe.deviceId'),
    minWidth: 220,
    render: row => row.device?.device_id || '-'
  },
  {
    key: 'online',
    title: () => $t('rdi.sharedWithMe.status'),
    width: 120,
    render: row =>
      h(
        NTag,
        { type: row.device?.online ? 'success' : 'default' },
        { default: () => (row.device?.online ? $t('rdi.sharedWithMe.online') : $t('rdi.sharedWithMe.offline')) }
      )
  },
  {
    key: 'accepted_at',
    title: () => $t('rdi.sharedWithMe.acceptedAt'),
    minWidth: 180,
    render: row => formatTime(row.accepted_at)
  },
  {
    key: 'actions',
    title: () => $t('rdi.sharedWithMe.actions'),
    width: 220,
    render: row =>
      h('div', { class: 'action-row' }, [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            'data-testid': 'shared-with-me-view',
            onClick: () => showDeviceDetails(row)
          },
          { default: () => $t('rdi.sharedWithMe.view') }
        ),
        h(
          NButton,
          {
            size: 'small',
            'data-testid': 'shared-with-me-open-device',
            onClick: () => openDeviceDetails(row)
          },
          { default: () => $t('rdi.sharedWithMe.openDevice') }
        )
      ])
  }
]

const pagination: PaginationProps = reactive({
  page: queryParams.page,
  pageSize: queryParams.page_size,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  itemCount: 0,
  onChange: page => {
    queryParams.page = page
    pagination.page = page
    fetchSharedDevices()
  },
  onUpdatePageSize: pageSize => {
    queryParams.page = 1
    queryParams.page_size = pageSize
    pagination.page = 1
    pagination.pageSize = pageSize
    fetchSharedDevices()
  }
})

async function fetchSharedDevices() {
  loading.value = true
  try {
    const { data } = await rdiSharedWithMeDevices({
      page: queryParams.page,
      page_size: queryParams.page_size,
      device_id: queryParams.device_id || undefined,
      device_name: queryParams.device_name || undefined
    })
    tableData.value = data?.list || []
    pagination.itemCount = data?.total || 0
    if (queryParams.device_id && tableData.value.length === 1) {
      showDeviceDetails(tableData.value[0])
    }
  } catch (error: any) {
    tableData.value = []
    pagination.itemCount = 0
    window.$message?.error(error?.error?.message || error?.message || $t('rdi.share.failedDescription'))
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  queryParams.page = 1
  pagination.page = 1
  fetchSharedDevices()
}

function handleReset() {
  queryParams.device_id = ''
  queryParams.device_name = ''
  handleSearch()
}

onMounted(() => {
  const deviceId = route.query.device_id
  queryParams.device_id = Array.isArray(deviceId) ? deviceId[0] || '' : deviceId || ''
  fetchSharedDevices()
})
</script>

<template>
  <div class="shared-page" data-testid="shared-with-me-page">
    <NCard :bordered="false" data-testid="shared-with-me-card">
      <NSpace vertical size="large">
        <div class="shared-header">
          <div>
            <div class="shared-title" data-testid="shared-with-me-title">{{ $t('rdi.sharedWithMe.title') }}</div>
            <div class="shared-subtitle">{{ $t('rdi.sharedWithMe.subtitle') }}</div>
          </div>
          <NSpace>
            <NButton data-testid="shared-with-me-back" @click="goBack">{{ $t('rdi.sharedWithMe.back') }}</NButton>
            <NButton data-testid="shared-with-me-refresh" :loading="loading" @click="fetchSharedDevices">
              {{ $t('rdi.sharedWithMe.refresh') }}
            </NButton>
          </NSpace>
        </div>

        <NSpace align="center" :wrap="true" data-testid="shared-with-me-filters">
          <NInput
            v-model:value="queryParams.device_id"
            clearable
            :placeholder="$t('rdi.sharedWithMe.deviceId')"
            class="search-input"
            data-testid="shared-with-me-device-id"
          />
          <NInput
            v-model:value="queryParams.device_name"
            clearable
            :placeholder="$t('rdi.sharedWithMe.deviceName')"
            class="search-input"
            data-testid="shared-with-me-device-name-filter"
          />
          <NButton data-testid="shared-with-me-search" type="primary" @click="handleSearch">
            {{ $t('rdi.sharedWithMe.search') }}
          </NButton>
          <NButton data-testid="shared-with-me-reset" @click="handleReset">{{ $t('rdi.sharedWithMe.reset') }}</NButton>
        </NSpace>

        <NDataTable
          data-testid="shared-with-me-table"
          :columns="columns"
          :data="tableData"
          :loading="loading"
          :pagination="pagination"
          :scroll-x="900"
        >
          <template #empty>
            <n-empty :description="$t('common.noData')" class="py-8" />
          </template>
        </NDataTable>
      </NSpace>
    </NCard>

    <NDrawer v-model:show="detailVisible" width="520" data-testid="shared-with-me-drawer">
      <NDrawerContent data-testid="shared-with-me-drawer-content" :title="$t('rdi.sharedWithMe.drawerTitle')" closable>
        <NDescriptions v-if="selectedRecord" bordered :column="1" size="small">
          <NDescriptionsItem :label="$t('rdi.sharedWithMe.deviceName')">
            {{ selectedRecord.device.device_name || '-' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="PID">{{ selectedRecord.device.pid_number || '-' }}</NDescriptionsItem>
          <NDescriptionsItem :label="$t('rdi.sharedWithMe.firmware')">
            {{ selectedRecord.device.firmware_version || '-' }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('rdi.sharedWithMe.connection')">
            {{ selectedRecord.device.connection_type || '-' }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('rdi.sharedWithMe.onlineLabel')">
            {{ selectedRecord.device.online ? $t('rdi.sharedWithMe.online') : $t('rdi.sharedWithMe.offline') }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('rdi.sharedWithMe.sensor1Range')">
            {{ selectedRecord.device.config.sensor_1_lower }} - {{ selectedRecord.device.config.sensor_1_upper }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('rdi.sharedWithMe.sensor2Range')">
            {{ selectedRecord.device.config.sensor_2_lower }} - {{ selectedRecord.device.config.sensor_2_upper }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('rdi.sharedWithMe.switchAlarm')">
            {{ selectedRecord.device.config.switch_1_alarm_mode }} /
            {{ selectedRecord.device.config.switch_2_alarm_mode }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('rdi.sharedWithMe.notification')">
            {{
              selectedRecord.device.config.notification_enabled
                ? $t('rdi.sharedWithMe.enabled')
                : $t('rdi.sharedWithMe.disabled')
            }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('rdi.sharedWithMe.acceptedAt')">
            {{ formatTime(selectedRecord.accepted_at) }}
          </NDescriptionsItem>
        </NDescriptions>
        <template #footer>
          <NButton
            v-if="selectedRecord"
            data-testid="shared-with-me-drawer-open-device"
            type="primary"
            @click="openDeviceDetails(selectedRecord)"
          >
            {{ $t('rdi.sharedWithMe.openDevice') }}
          </NButton>
        </template>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>

<style scoped>
.shared-page {
  padding: 16px;
}

.shared-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.shared-title {
  font-size: 22px;
  font-weight: 700;
}

.shared-subtitle {
  margin-top: 4px;
  color: var(--text-color-3);
}

.search-input {
  width: 220px;
}

.action-row {
  display: flex;
  gap: 8px;
}

@media (max-width: 560px) {
  .shared-header {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
