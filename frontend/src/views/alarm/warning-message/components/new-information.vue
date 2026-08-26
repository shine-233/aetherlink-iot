<!--
文件用途：提供 告警消息管理 页面内的 new-information 子组件。
核心逻辑：封装局部表单、弹窗、列表或展示模块，通过 props、emit 与父页面协作。
关键注意事项：保持组件边界清晰，避免在子组件中绕过父页面的数据刷新与权限控制。
重构建议：后续可把重复表单规则、选项转换和弹窗状态管理抽成可复用组合函数。
-->
<script setup lang="tsx">
import { computed, getCurrentInstance, h, reactive, ref } from 'vue'
import type { Ref } from 'vue'
import type { DataTableColumns, PaginationProps } from 'naive-ui'
import { NButton, NEmpty, NPopconfirm, useMessage } from 'naive-ui'
import { delInfo, editInfo, warningMessageList } from '@/service/api/alarm'
import { $t } from '@/locales'
import type { ModalType } from './pop-up.vue'
import popUp from './pop-up.vue'
import { useBoolean } from '~/packages/hooks'
const rowKey = (row: DeviceManagement.DeviceData) => row.id
const { bool: visible, setTrue: openModal } = useBoolean()
const modalType = ref<ModalType>('add')
const params = {
  ID: '',
  enabled: 'Y'
}
const deleteId = ref('')

function setModalType(type: ModalType) {
  modalType.value = type
}

function addWarningMessageBut() {
  openModal()
  setModalType('add')
}

function newEdit() {
  list()
}

/** 表格案例处理事件 */
const editData = ref<Api.Alarm.NotificationGroupList | null>(null)

function handleEditPwd(row, type) {
  // type:edit编辑，enable停用启用
  if (type === 'edit') {
    editData.value = row
    setModalType('edit')
    openModal()
  } else if (type === 'enable') {
    const enableds = row.enabled === 'Y' ? 'N' : 'Y'
    params.ID = row.id
    params.enabled = enableds
    editInfos()
  }
}

const loading = ref(false)
const message = useMessage()
const pagination: PaginationProps = reactive({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 15, 20, 25, 30],
  onChange: (page: number) => {
    pagination.page = page
    list()
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
    list()
  }
})

interface ColumnsData {
  name: string
  description: string
  alarm_level: string
  notification_group_id: string
  enabled: string

  [key: string]: any
}

const tableData = ref<ColumnsData[]>([])

/** 告警信息列表 */
async function list() {
  loading.value = true
  try {
    const innerparams = { page: pagination.page, page_size: pagination.pageSize }
    const { data } = await warningMessageList(innerparams)

    if (data) {
      tableData.value = data.list
      const operatorBtn: { btnName: string; type: string; color: string }[] = [
        {
          btnName: $t('common.edit'),
          type: 'edit',
          color: 'info'
        },
        {
          btnName: $t('page.manage.common.status.disable'),
          type: 'enable',
          color: 'warning'
        },
        {
          btnName: $t('common.delete'),
          type: 'delete',
          color: 'error'
        }
      ]
      const operatorBtns: { btnName: string; type: string; color: string }[] = [
        { btnName: $t('common.edit'), type: 'edit', color: 'info' },
        { btnName: $t('page.manage.common.status.enable'), type: 'enable', color: 'success' },
        { btnName: $t('common.delete'), type: 'delete', color: 'error' }
      ]
      tableData.value.forEach(item => {
        if (item.enabled === 'Y') {
          item.operatorBtn = operatorBtn
        } else {
          item.operatorBtn = operatorBtns
        }
      })
      pagination.itemCount = data.total
    }
  } finally {
    loading.value = false
  }
}

const columns: Ref<DataTableColumns<ColumnsData>> = ref([
  {
    key: 'name',
    title: $t('generate.alarm-name'),
    align: 'left',
    minWidth: '140px',
    ellipsis: {
      tooltip: true
    }
  },
  {
    key: 'description',
    title: $t('generate.alarm-description'),
    align: 'left',
    minWidth: '180px',
    ellipsis: {
      tooltip: true
    }
  },
  {
    key: 'alarm_level',
    title: $t('common.alarm_level'),
    align: 'left',
    minWidth: '100px',
    render(row) {
      if (row.alarm_level === 'H') {
        return $t('common.high')
      } else if (row.alarm_level === 'M') {
        return $t('common.middle')
      }
      return $t('common.low')
    }
  },

  {
    key: 'notification_group_name',
    title: $t('generate.notification-group'),
    align: 'left',
    minWidth: '140px',
    ellipsis: {
      tooltip: true
    }
  },
  {
    key: 'enabled',
    title: $t('generate.runstate'),
    align: 'left',
    minWidth: '100px',
    render(row) {
      if (row.enabled === 'Y') {
        return $t('page.manage.common.status.enable')
      }
      return $t('page.manage.common.status.disable')
    }
  },

  {
    key: 'actions',
    width: '200px',
    title: $t('common.actions'),
    align: 'left',
    render: (row: any) => {
      const operatorBtn = row.operatorBtn.map(item => {
        if (item.type === 'delete') {
          return h(
            <NPopconfirm onPositiveClick={() => handleDeleteTable(row)}>
              {{
                default: () => $t('common.confirmDelete'),
                trigger: () => (
                  <NButton type={item.color} size={'small'}>
                    {item.btnName}
                  </NButton>
                )
              }}
            </NPopconfirm>
          )
        }
        return h(
          <NButton type={item.color} size={'small'}>
            {item.btnName}
          </NButton>,
          { onClick: () => handleEditPwd(row, item.type) }
        )
      })
      return <div class="flex">{operatorBtn}</div>
    }
  }
]) as Ref<DataTableColumns<ColumnsData>>

list()

/** 删除 */
function handleDeleteTable(rowId) {
  deleteId.value = rowId.id
  deleteInfo()
}

/** 编辑:启动停止 */

async function editInfos() {
  const { data } = await editInfo(params)
  if (data) {
    params.enabled === 'Y' ? message.success($t('common.startSuccess')) : message.success($t('common.stopSuccess'))

    list()
  } else {
    params.enabled === 'Y' ? message.error($t('common.startFail')) : message.error($t('common.stopFail'))
  }
}

/** 删除告警 */

async function deleteInfo() {
  const { data } = await delInfo(deleteId.value)
  if (!data) {
    message.success($t('common.deleteSuccess'))
  } else {
    message.error($t('common.deleteFail'))
  }
  list()
}

const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})
</script>

<template>
  <div class="p-y-12px">
    <NButton type="primary" @click="addWarningMessageBut">
      <IconIcRoundPlus class="mr-4px text-20px" />
      {{ $t('generate.addAlarm') }}
    </NButton>
  </div>
  <div class="h-full flex-col">
    <NDataTable
      remote
      :loading="loading"
      :row-key="rowKey"
      :columns="columns"
      :data="tableData"
      :pagination="pagination"
      class="w-full"
    >
      <template #empty>
        <NEmpty :description="$t('common.noData')" class="py-24px" />
      </template>
    </NDataTable>
  </div>

  <popUp
    v-model:visible="visible"
    :class="getPlatform ? 'w-90%' : 'w-600px'"
    :type="modalType"
    :edit-data="editData"
    @new-edit="newEdit"
  />
</template>

<style scoped>
:deep(.n-button) {
  cursor: pointer;
  margin-left: 10px;
}
</style>
