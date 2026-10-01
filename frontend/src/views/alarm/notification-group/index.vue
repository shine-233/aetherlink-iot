<!--
文件用途：承载 告警通知组管理 的页面级视图。
核心逻辑：组合表格、表单、弹窗、接口请求和国际化文案，完成页面初始化、查询与交互反馈。
关键注意事项：页面通常依赖权限、分页、远端接口和路由状态，改动时需同步检查测试与接口契约。
重构建议：后续可继续拆分数据编排、列配置和弹窗流程，降低页面级组件复杂度。
-->
<script setup lang="tsx">
import { computed, getCurrentInstance, ref } from 'vue'
import type { Ref } from 'vue'
import { NButton, NEmpty, NPopconfirm, NSpace, NSwitch } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  deleteNotificationGroup,
  getNotificationGroupDetail,
  getNotificationGroupList,
  putNotificationGroup
} from '@/service/api/notification'
import { notificationOptions } from '@/constants/business'
import { $t } from '@/locales'
import EmailTemplateManager from '@/components/business/email-template-manager.vue'
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'
import type { ModalType } from './components/table-action-modal.vue'
import TableActionModal from './components/table-action-modal.vue'
import { useBoolean } from '~/packages/hooks'

const { bool: visible, setTrue: openModal } = useBoolean()

// 分页/加载态/过期请求丢弃由 useListPage 统一处理。deepRows：状态开关先乐观改行内字段，需要深响应才能即时重绘。
// 删除当前页最后一行后 useListPage 会自动回退到上一页。
const {
  rows: tableData,
  total,
  loading,
  pagination,
  load: getTableData
} = useListPage<Api.Alarm.NotificationGroupList>({
  deepRows: true,
  pageSizes: [10, 15, 20, 25, 30],
  fetcher: async (params) =>
    fromFlatResponse<Api.Alarm.NotificationGroupList>(
      await getNotificationGroupList(params as Api.Alarm.NotificationGroupParams)
    )
})

function setTableData(data: Api.Alarm.NotificationGroupList[]) {
  tableData.value = data
}

/**
 * 启停开关：乐观更新行状态，提交时发送不含 id 的副本（id 走路径参数），
 * 不再 `delete row.id` 改坏表格行（否则刷新前该行的编辑/删除按钮会拿到 undefined id）。
 * 提交失败回滚状态；无论成败都重新拉取当前页以对齐服务端。
 */
const handleSwitchChange = async (row, value) => {
  const previous = row.status
  row.status = value ? 'OPEN' : 'CLOSE'
  const { id = '', ...payload } = row ?? {}
  const res = await putNotificationGroup(payload, id || '')
  if (res?.error) row.status = previous
  getTableData()
}
const handleDeleteTable = async (rowId: string) => {
  const res = await deleteNotificationGroup({ id: rowId })
  if (res?.error) return
  window.$message?.info($t('generate.notificationGroup'))
  getTableData()
}
const editData = ref<Api.Alarm.NotificationGroupList | null>(null)
const handleEditTable = async (rowId: string) => {
  const res = await getNotificationGroupDetail({ id: rowId })
  if (res?.data) {
    editData.value = res.data
    setModalType('edit')
    openModal()
  }
}
const columns = ref([
  {
    key: 'name',
    title: $t('generate.notification-group-name'),
    minWidth: '140px',
    align: 'left'
  },
  {
    key: 'notification_type',
    title: $t('generate.notification-type'),
    align: 'left',
    minWidth: '140px',
    render: (row: any) => {
      const notificationType = notificationOptions.find((option) => option.value === row.notification_type)?.label || ''
      return notificationType
    }
  },
  {
    key: 'status',
    title: $t('generate.status'),
    align: 'left',
    minWidth: '140px',
    render: (row: any) => {
      return <NSwitch value={row.status === 'OPEN'} onChange={(value) => handleSwitchChange(row, value)} />
    }
  },
  {
    key: 'actions',
    title: $t('common.actions'),
    align: 'left',
    width: '200px',
    render: (row: any) => {
      return (
        <NSpace justify={'start'}>
          <NButton size={'small'} type="primary" onClick={() => handleEditTable(row.id)}>
            {$t('common.edit')}
          </NButton>
          <NPopconfirm onPositiveClick={() => handleDeleteTable(row.id)}>
            {{
              default: () => $t('common.confirmDelete'),
              trigger: () => (
                <NButton type="error" size={'small'}>
                  {$t('common.delete')}
                </NButton>
              )
            }}
          </NPopconfirm>
        </NSpace>
      )
    }
  }
]) as Ref<DataTableColumns<DataService.Data>>

const modalType = ref<ModalType>('add')

function setModalType(type: ModalType) {
  modalType.value = type
}

function handleAddTable() {
  openModal()
  setModalType('add')
}

const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})
getTableData()
</script>

<template>
  <div>
    <NCard :title="$t('generate.notification-group')">
      <template #header-extra>
        <NButton type="primary" @click="handleAddTable">+{{ $t('device_template.add') }}</NButton>
      </template>
      <div class="h-full flex-col">
        <NDataTable :columns="columns" :data="tableData" :loading="loading">
          <template #empty>
            <NEmpty :description="$t('common.noData')" class="py-24px" />
          </template>
        </NDataTable>
        <div class="pagination-box">
          <NPagination
            :page="pagination.page"
            :page-size="pagination.pageSize"
            :item-count="total"
            :show-size-picker="true"
            :page-sizes="pagination.pageSizes"
            @update:page="pagination.onUpdatePage"
            @update:page-size="pagination.onUpdatePageSize"
          />
        </div>
        <TableActionModal
          v-model:visible="visible"
          :class="getPlatform ? 'w-90%' : 'w-600px'"
          :type="modalType"
          :edit-data="editData"
          @get-table-data="getTableData"
        />
      </div>
    </NCard>
    <EmailTemplateManager />
  </div>
</template>

<style scoped>
.pagination-box {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
}
</style>
