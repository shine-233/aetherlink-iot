<!--
文件用途：角色管理主页面，负责展示角色列表并承接角色新增、编辑、删除和权限分配入口。
核心逻辑：页面以列表查询为主线，组合表格列渲染、分页状态和多个弹窗，驱动角色资料维护与权限配置。
关键状态流：初始化触发 getTableData 拉取角色列表；表格操作会把当前行写入 editData，再按业务类型打开对应弹窗；新增/编辑成功后由子弹窗回调父页面重新拉取列表。
使用注意事项：当前页面把列表查询、弹窗编排和表格动作都集中在同一个 SFC 中，后续改动时要同步检查 editData 传递约定、分页回刷时机和权限分配弹窗的角色上下文。
静态审查建议：
1. editData 在编辑、分配权限之间复用，新增入口已在父层主动清空，后续新增动作也应保持该约定。
-->
<script setup lang="tsx">
import { computed, getCurrentInstance, ref } from 'vue'
import type { Ref } from 'vue'
import { NButton, NEmpty, NPopconfirm, NSpace } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useBoolean } from '@aetherlink/hooks'
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'
import { deleteRole, listRoles } from '@/service/api'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import TableActionModal from './modules/table-action-modal.vue'
import EditPermissionModal from './modules/edit-permission-modal.vue'
import type { ModalType } from './modules/table-action-modal.vue'

const { bool: visible, setTrue: openModal } = useBoolean()
const { bool: editPermissionVisible, setTrue: openEditPermissionModal } = useBoolean()

type QueryFormModel = Pick<UserManagement.User, 'email' | 'name' | 'status'>

// 查询、分页与过期请求丢弃统一由 useListPage 维护；新增/编辑/删除成功后调用 getTableData 回刷。
const {
  query: queryParams,
  rows: tableData,
  loading,
  pagination,
  load: getTableData
} = useListPage<UserManagement.User, QueryFormModel>({
  initialQuery: () => ({ email: null, name: null, status: null }),
  pageSizes: [10, 15, 20, 25, 30],
  fetcher: async (params) => fromFlatResponse<UserManagement.User>(await listRoles(params))
})

const columns: Ref<DataTableColumns<UserManagement.User>> = ref([
  {
    key: 'name',
    minWidth: '100px',
    title: $t('page.manage.role.roleName'),
    align: 'left'
  },
  {
    key: 'description',
    minWidth: '100px',
    title: $t('page.manage.role.roleDesc'),
    align: 'left'
  },
  {
    key: 'created_at',
    title: $t('page.product.update-ota.createTime'),
    minWidth: '100px',
    align: 'left',
    render: (row) => {
      return formatDateTime(row.created_at)
    }
  },
  {
    key: 'updated_at',
    title: $t('page.product.update-ota.updateDate'),
    minWidth: '130px',
    align: 'left',
    render: (row) => {
      return formatDateTime(row.updated_at)
    }
  },
  {
    key: 'actions',
    title: $t('common.actions'),
    align: 'left',
    width: '320px',
    render: (row) => {
      // 这里的三个按钮分别进入“资料编辑”“删除确认”“权限分配”三条链路，
      // 都依赖当前行数据仍然存在于 tableData 中，后续如改为服务端虚拟滚动需同步调整。
      return (
        <NSpace justify={'start'}>
          <NButton type="primary" size={'small'} onClick={() => handleEditTable(row.id)}>
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
          <NButton type="primary" size={'small'} onClick={() => handleEditPermission(row.id)}>
            {$t('page.manage.role.editPermission')}
          </NButton>
        </NSpace>
      )
    }
  }
]) as Ref<DataTableColumns<UserManagement.User>>

const modalType = ref<ModalType>('add')

function setModalType(type: ModalType) {
  modalType.value = type
}

const editData = ref<UserManagement.User | null>(null)

function setEditData(data: UserManagement.User | null) {
  editData.value = data
}

function handleAddTable() {
  // 新增场景只切换弹窗类型，不复用旧行数据。
  setEditData(null)
  openModal()
  setModalType('add')
}

function handleEditTable(rowId: string) {
  // 角色编辑沿用当前表格行作为弹窗初始值，属于“父页缓存一份行快照 -> 子弹窗回填”的模式。
  const findItem = tableData.value.find((item) => item.id === rowId)
  if (findItem) {
    setEditData(findItem)
  }
  setModalType('edit')
  openModal()
}

function handleEditPermission(rowId: string) {
  // 权限分配弹窗需要角色 id 和角色名：id 用于提交，name 用于弹窗标题提示当前上下文。
  const findItem = tableData.value.find((item) => item.id === rowId)
  if (findItem) {
    setEditData(findItem)
  }
  openEditPermissionModal()
}

async function handleDeleteTable(rowId: string) {
  // 删除成功后直接整表回刷，当前没有做“删除当前页最后一条后回退页码”的补偿逻辑。
  const data = await deleteRole(rowId)
  if (!data.error) {
    window.$message?.success($t('common.deleteSuccess'))
    getTableData()
  }
}

function init() {
  // 页面没有额外路由守卫缓存，进入即拉取一次角色列表。
  getTableData()
}
const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  // 宽度适配仍依赖全局实例方法，属于运行时注入契约，后续可收敛成组合式响应式能力。
  return proxy.getPlatform()
})
// 初始化
init()
</script>

<template>
  <div>
    <n-card>
      <div class="h-full flex-col gap-15px">
        <NSpace>
          <NButton type="primary" @click="handleAddTable">
            <icon-ic-round-plus class="mr-4px text-20px" />
            {{ $t('page.manage.role.addRole') }}
          </NButton>
        </NSpace>

        <NDataTable
          :columns="columns"
          :data="tableData"
          :loading="loading"
          :pagination="pagination"
          remote
          class="flex-1-hidden"
        >
          <template #empty>
            <NEmpty :description="$t('common.noData')" class="py-24px" />
          </template>
        </NDataTable>
        <TableActionModal
          v-model:visible="visible"
          :class="getPlatform ? 'w-90%' : 'w-500px'"
          :type="modalType"
          :edit-data="editData"
          @success="getTableData"
        />
        <EditPermissionModal
          v-model:visible="editPermissionVisible"
          :class="getPlatform ? 'w-90%' : 'w-600px'"
          :edit-data="editData"
        />
      </div>
    </n-card>
  </div>
</template>

<style scoped></style>
