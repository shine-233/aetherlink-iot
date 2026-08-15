<!--
文件用途: 承载AutomaticModeStep相关的设备页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<template>
  <div class="automatic-mode-step-content">
    <n-data-table
      :remote="true"
      :columns="columns"
      :data="pageData.tableData"
      :loading="pageData.loading"
      :pagination="pagination"
      :bordered="true"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { $t } from '@/locales'
import { NDataTable, useMessage } from 'naive-ui'
import { deviceConfig } from '@/service/api/device'

const message = useMessage()
const route = useRoute()

const pageData = ref({
  loading: false,
  tableData: []
})

const pagination = ref({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  pageSizes: [10, 20, 30, 40],
  showSizePicker: true,
  prefix({ itemCount }) {
    return `${$t('common.total')}: ${itemCount}`
  },
  onChange: page => {
    pagination.value.page = page
    loadDeviceConfigs()
  },
  onUpdatePageSize: pageSize => {
    pagination.value.pageSize = pageSize
    pagination.value.page = 1
    loadDeviceConfigs()
  }
})

const columns = ref([
  {
    title: $t('card.deviceConfigName'),
    key: 'name',
    align: 'center' as const
  },
  {
    title: $t('card.deviceConfigKey'),
    key: 'id',
    align: 'center' as const
  },
  {
    title: $t('card.serviceSecret'),
    key: 'template_secret',
    align: 'center' as const,
    render(row) {
      return row.template_secret ? '******' : $t('card.serviceSecretNotConfigured')
    }
  }
])

const loadDeviceConfigs = async () => {
  pageData.value.loading = true
  try {
    const { data } = await deviceConfig({
      page: pagination.value.page,
      page_size: pagination.value.pageSize,
      protocol_type: route.query.service_identifier
    })

    if (data && data.list) {
      pageData.value.tableData = data.list
      pagination.value.itemCount = data.total
    } else {
      pageData.value.tableData = []
      pagination.value.itemCount = 0
    }
  } catch (error) {
    console.error($t('card.loadDeviceConfigFailed'), error)
    message.error($t('common.loadFailed'))
  } finally {
    pageData.value.loading = false
  }
}

onMounted(() => {
  loadDeviceConfigs()
})
</script>

<style scoped lang="scss">
.automatic-mode-step-content {
  padding: 20px;
  height: 100%;
}
</style>
