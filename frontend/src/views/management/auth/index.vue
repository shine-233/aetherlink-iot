<!--
文件用途：承载权限元素管理页，负责维护按钮/菜单元素的基础信息、权限标识、图标与前端路由辅助字段。
核心逻辑：页面通过远端分页查询、数据表格渲染和新增编辑弹窗，完成权限元素的增删改查闭环。
关键注意事项：
1. 这里维护的是权限元素元数据，不是最终路由树本身；字段如 `authority`、`element_code`、`param1` 会被其他权限流程继续消费。
2. 删除、新增、编辑都会影响角色授权与页面权限显隐，改动接口契约时要同步检查菜单、角色、登录态刷新链路。
3. 当前列表查询只带分页参数，没有前端筛选条件；若后续增加查询表单，需同步处理分页归零和回刷节奏。
静态审查建议：
1. `getTableData` 同样缺少异常分支下的加载态收口，后续宜统一改成 `try/finally`。
2. 页面把权限标签、图标、类型映射都内联在列定义中，后续可拆出列工厂或展示组件，降低页面噪音。
3. 删除后依赖整页回查确保一致性是安全的，但新增/编辑/删除成功提示与错误处理仍偏分散，可继续统一。
-->
<script setup lang="tsx">
import { h, reactive, ref } from 'vue'
import type { Ref } from 'vue'
import { NButton, NEmpty, NPopconfirm, NSpace, NTag } from 'naive-ui'
import type { DataTableColumns, PaginationProps } from 'naive-ui'
import { useBoolean, useLoading } from '@aetherlink/hooks'
import { routerSysFlagLabels, routerTypeLabels } from '@/constants/business'
import { delElement, fetchElementList } from '@/service/api/route'
import { smartDeepClone as deepClone } from '@/utils/deep-clone'
import { $t } from '@/locales'
import TableActionModal from './components/table-action-modal.vue'
import type { ModalType } from './components/table-action-modal.vue'

const { loading, startLoading, endLoading } = useLoading(false)
const { bool: visible, setTrue: openModal } = useBoolean()

type QueryFormModel = {
  page: number
  page_size: number
}

const queryParams = reactive<QueryFormModel>({
  page: 1,
  page_size: 10
})

const tableData = ref<CustomRoute.Route[]>([])

function setTableData(data: CustomRoute.Route[]) {
  tableData.value = data
}

const pagination: PaginationProps = reactive({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  itemCount: 0,
  pageSizes: [10, 15, 20, 25, 30],
  onChange: (page: number) => {
    pagination.page = page
    queryParams.page = page
    getTableData()
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
    queryParams.page = 1
    queryParams.page_size = pageSize
    getTableData()
  }
})

// 权限元素列表主查询：只接受分页参数，结果既驱动表格也驱动总数显示。
async function getTableData() {
  startLoading()
  const { data } = await fetchElementList(queryParams)
  if (data) {
    const list: Api.Route.MenuRoute[] = data.list
    pagination.itemCount = data.total
    setTableData(list)
    endLoading()
  }
}

const rowKey = (row: CustomRoute.Route) => {
  return row.id
}

const columns: Ref<DataTableColumns<CustomRoute.Route>> = ref([
  {
    key: 'description',
    title: () => $t('page.manage.menu.title'),
    align: 'left',
    minWidth: '140px',
    render: row => {
      if (row.multilingual && row.multilingual !== 'default') {
        return <span>{$t(row.multilingual)}</span>
      }
      return <span>{row.description}</span>
    }
  },

  {
    key: 'param2',
    title: () => $t('page.manage.menu.icon'),
    align: 'left',
    minWidth: '140px',
    render: row => {
      if (row.param2) {
        return <svg-icon icon={row.param2} />
      }
      return <span></span>
    }
  },
  {
    key: 'element_code',
    minWidth: '140px',
    title: () => $t('page.manage.menu.menuName'),
    align: 'left'
  },
  {
    key: 'param1',
    minWidth: '140px',
    title: () => $t('page.manage.menu.routeName'),
    align: 'left'
  },
  // {
  //   key: 'param3',
  //   minWidth: '140px',
  //   title: () => $t('page.manage.menu.componentType'),
  //   align: 'left'
  // },
  {
    key: 'element_type',
    minWidth: '140px',
    title: () => $t('page.manage.menu.menuType'),
    align: 'left',
    // 元素类型直接决定后续权限消费方式，列表中用标签把原始枚举翻译成更易读的业务语义。
    render: row => {
      if (row.element_type) {
        const tagTypes: Record<CustomRoute.routerTypeKey, NaiveUI.ThemeColor> = {
          '1': 'success',
          // "2": "error",
          '3': 'warning'
          // "4": "default",
          // "5": "info",
        }
        return <NTag type={tagTypes[row.element_type]}>{routerTypeLabels[row.element_type]}</NTag>
      }
      return <span></span>
    }
  },
  {
    key: 'authority',
    minWidth: '140px',
    title: () => $t('page.manage.menu.authority'),
    align: 'left',
    // 权限标识是角色授权链路的核心字段，页面仅做可视化展示，不在列表态改写其值。
    render: row => {
      if (row.authority && row.authority.length) {
        const tags = row.authority.map((tagKey: string) => {
          return h(
            NTag,
            {
              type: 'success'
            },
            {
              default: () => routerSysFlagLabels[tagKey]
            }
          )
        })
        return tags
      }
      return <span></span>
    }
  },
  {
    key: 'remark',
    minWidth: '140px',
    title: () => $t('common.remark'),
    align: 'left'
  },
  {
    key: 'actions',
    title: () => $t('common.actions'),
    align: 'left',
    minWidth: '140px',
    render: row => {
      return (
        <NSpace>
          <NButton type="primary" size={'small'} onClick={() => handleEditTable(row)}>
            {$t('common.edit')}
          </NButton>
          <NPopconfirm
            negative-text={$t('common.cancel')}
            positive-text={$t('common.confirm')}
            onPositiveClick={() => handleDeleteTable(row.id)}
          >
            {{
              default: () => $t('common.confirm'),
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
]) as Ref<DataTableColumns<CustomRoute.Route>>

const modalType = ref<ModalType>('add')

function setModalType(type: ModalType) {
  modalType.value = type
}

const editData = ref<CustomRoute.Route | null>(null)

// 新增入口只切换模式与弹窗显示，实际默认值构造放在子弹窗内部完成。
function handleAddTable() {
  openModal()
  setModalType('add')
}

// 编辑前先做深拷贝，避免弹窗内的双向绑定直接污染表格当前行对象。
function handleEditTable(row: any) {
  editData.value = deepClone(row)
  setModalType('edit')
  openModal()
}

// 删除属于高影响权限变更，成功后必须回查列表，确保角色侧使用到的是最新元素集。
async function handleDeleteTable(rowId: string) {
  const data = await delElement(rowId)
  if (!data.error) {
    window.$message?.success($t('common.deleteSuccess'))
    await getTableData()
  }
}

// 页面首屏仅做一次列表拉取，作为权限元素维护页的统一初始化入口。
function init() {
  getTableData()
}

// 初始化
init()
</script>

<template>
  <div>
    <NCard :title="$t('page.manage.menu.title')" :bordered="false" class="h-full rounded-8px shadow-sm">
      <template #header-extra>
        <NButton type="primary" @click="handleAddTable">
          <IconIcRoundPlus class="mr-4px text-20px" />
          {{ $t('common.add') }}
        </NButton>
      </template>
      <div class="h-full flex-col">
        <NDataTable
          size="small"
          :row-key="rowKey"
          :remote="true"
          :columns="columns"
          :data="tableData"
          :loading="loading"
          :pagination="pagination"
          class="flex-1-hidden"
        >
          <template #empty>
            <NEmpty :description="$t('common.noData')" class="py-24px" />
          </template>
        </NDataTable>
        <TableActionModal
          v-model:visible="visible"
          :type="modalType"
          :edit-data="editData"
          @success="getTableData"
        />
      </div>
    </NCard>
  </div>
</template>

<style scoped></style>
