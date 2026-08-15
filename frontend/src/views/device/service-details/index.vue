<!--
文件用途: 承载服务详情相关的设备页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<!-- eslint-disable require-atomic-updates -->
<script setup lang="tsx">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NPopconfirm, NSpace } from 'naive-ui'
import dayjs from 'dayjs'
import { delServiceAccess, getServiceAccess } from '@/service/api/plugin'
import { $t } from '@/locales'
import serviceModal from './components/serviceModal.vue'
import serviceConfigModal from './components/serviceConfigModal.vue'

const route: any = useRoute()
const router: any = useRouter()
const serviceModalRef = ref<any>(null)
const serviceConfigModalRef = ref<any>(null)
const service_plugin_id = ref<any>(route.query.id)
const pageData = ref<any>({
  loading: false,
  tableData: []
})

const queryInfo = ref<any>({
  service_plugin_id: service_plugin_id.value,
  page: 1,
  page_size: 10,
  total: 0,
  pageSizes: [10, 15, 20, 25, 30],
  onChange: (page: number) => {
    queryInfo.value.page = page
    getList()
  },
  onUpdatePageSize: (pageSize: number) => {
    queryInfo.value.page_size = pageSize
    queryInfo.value.page = 1
    getList()
  }
})

const getList: () => void = async () => {
  const { data }: { data: any } = await getServiceAccess(queryInfo.value)
  pageData.value.tableData = data.list
  queryInfo.value.itemCount = data.total
}

const see: (row: any) => void = row => {
  router.push(
    `/device/manage?service_identifier=${route.query.service_identifier}&device_name=${row.name}&service_access_id=${row.id}`
  )
}
const del: (row: any) => void = async row => {
  await delServiceAccess(row)
  getList()
}
const config: (row: any) => void = async row => {
  serviceModalRef.value.openModal(service_plugin_id.value, row)
}
const columns: any = ref([
  {
    title: $t('card.accessPointName'),
    key: 'name',
    minWidth: '200px'
  },
  {
    title: $t('common.creationTime'),
    key: 'create_at',
    minWidth: '200px',
    render: row => {
      if (row.create_at) {
        return <span>{dayjs(row.create_at).format('YYYY-MM-DD HH:mm:ss')}</span>
      }
      return <span></span>
    }
  },
  {
    key: 'actions',
    title: () => $t('common.actions'),
    align: 'left',
    width: '420px',
    ellipsis: {
      tooltip: {
        width: 420
      }
    },
    render: row => {
      return (
        <NSpace justify={'start'}>
          {
            <NButton size={'small'} type="primary" onClick={() => see(row)}>
              {$t('card.viewDevice')}
            </NButton>
          }
          {
            <NButton size={'small'} type="primary" onClick={() => config(row)}>
              {$t('card.modifyConfig')}
            </NButton>
          }
          <NPopconfirm
            negative-text={$t('common.cancel')}
            positive-text={$t('common.confirm')}
            onPositiveClick={() => del(row.id)}
          >
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
])

const addData: () => void = () => {
  serviceModalRef.value.openModal(service_plugin_id.value)
}

const isEdit: (val: any, row: any, edit: any) => void = (val, row, edit) => {
  if (edit) {
    if (row && row.auth_type === 'auto') {
      const adaptedRow = {
        ...row,
        mode: 'automatic'
      }
      serviceConfigModalRef.value.openModal(val, adaptedRow, edit)
    } else {
      serviceConfigModalRef.value.openModal(val, row, edit)
    }
    getList()
  } else {
    serviceConfigModalRef.value.openModal(val, row)
    getList()
  }
}
watch(
  () => queryInfo.value.service_type,
  () => {
    getList()
  },
  { deep: true }
)

getList()
</script>

<template>
  <div>
    <NCard :bordered="false" class="h-full rounded-8px shadow-sm" :title="route.query.service_name || '--'">
      <div class="header">
        <NButton type="primary" @click="addData">{{ $t('card.newAccess') }}</NButton>
      </div>
      <div class="h">
        <NDataTable
          :remote="true"
          :columns="columns"
          :data="pageData.tableData"
          :loading="pageData.loading"
          :pagination="queryInfo"
          class="flex-1-hidden"
        >
          <template #empty>
            <NEmpty :description="$t('custom.serviceAccess.emptyAccessPointTitle')" class="service-access-empty">
              <template #extra>
                <div class="service-access-empty__extra">
                  <div class="service-access-empty__hint">
                    {{ $t('custom.serviceAccess.emptyAccessPointHint') }}
                  </div>
                  <NButton type="primary" @click="addData">{{ $t('card.newAccess') }}</NButton>
                </div>
              </template>
            </NEmpty>
          </template>
        </NDataTable>
      </div>
    </NCard>
    <serviceConfigModal ref="serviceConfigModalRef" @get-list="getList"></serviceConfigModal>
    <serviceModal ref="serviceModalRef" @is-edit="isEdit"></serviceModal>
  </div>
</template>

<style lang="scss" scoped>
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
  .selectType {
    width: 100px;
  }
}
:deep(.n-data-table__pagination) {
  height: 80px;
}
.h {
  height: max-content;
}
.service-access-empty {
  padding: 32px 0;
}
.service-access-empty__extra {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}
.service-access-empty__hint {
  max-width: 360px;
  color: var(--n-text-color-3);
  font-size: 14px;
  line-height: 1.5;
  text-align: center;
}
</style>
