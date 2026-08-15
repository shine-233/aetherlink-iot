<!--
文件用途: 承载服务接入相关的设备页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { PaginationProps } from 'naive-ui'
import { getServiceList } from '@/service/api/device'
import DevCardItem from '@/components/dev-card-item/index.vue'
import AdvancedListLayout from '@/components/list-page/index.vue'
import { $t } from '@/locales'
import { GridOutline as CardIcon } from '@vicons/ionicons5'
const SERVICE_ACCESS_PAGE_SIZE = 15
const loading = ref(false)
const router = useRouter()
const pagination: PaginationProps = reactive({
  page: 1,
  pageSize: SERVICE_ACCESS_PAGE_SIZE,
  pageCount: 1
})
const queryParams = reactive({
  page_size: SERVICE_ACCESS_PAGE_SIZE,
  service_type: 2
})
const deviceTemplateList = ref([] as any[])
let serviceAccessRequestSeq = 0

const getData = async () => {
  const requestSeq = ++serviceAccessRequestSeq
  loading.value = true
  try {
    const res = await getServiceList({
      page: pagination.page as number,
      ...queryParams
    })
    if (requestSeq !== serviceAccessRequestSeq) return
    if (!res.error) {
      const total = Number(res.data?.total || 0)
      deviceTemplateList.value = res.data?.list || []
      pagination.pageCount = Math.max(1, Math.ceil(total / SERVICE_ACCESS_PAGE_SIZE))
    }
  } catch {
    if (requestSeq === serviceAccessRequestSeq) deviceTemplateList.value = []
  } finally {
    if (requestSeq === serviceAccessRequestSeq) {
      loading.value = false
    }
  }
}

onMounted(() => {
  getData()
})

const clickDevice = async row => {
  router.push(
    `/device/service-details?id=${row.id}&service_type=${row.service_type}&service_name=${row.name}&service_identifier=${row.service_identifier}`
  )
}

const handleRefresh = () => {
  getData()
}

const openServiceCatalog = () => {
  router.push('/apply/service')
}
</script>

<template>
  <div>
    <AdvancedListLayout
      :available-views="[{ key: 'card', icon: CardIcon, label: 'common.viewCard' }]"
      :showQueryButton="false"
      :showResetButton="false"
      :showAddButton="false"
      @refresh="handleRefresh"
    >
      <!-- Card view -->
      <template #card-view>
        <n-spin :show="loading">
          <n-empty
            v-if="!deviceTemplateList.length"
            class="service-access-empty"
            description="No service access templates are available yet. Enable a service first, then create an access point for devices."
          >
            <template #extra>
              <n-space justify="center">
                <n-button type="primary" @click="openServiceCatalog">Open service catalog</n-button>
                <n-button @click="handleRefresh">{{ $t('common.refresh') }}</n-button>
              </n-space>
            </template>
          </n-empty>
          <n-grid v-else cols="1 s:2 m:3 l:4 xl:5 2xl:8" x-gap="18" y-gap="18" responsive="screen">
            <n-gi v-for="item in deviceTemplateList" :key="item.id">
              <DevCardItem
                :isStatus="false"
                :title="item.name"
                :subtitle="item.description || $t('common.noDescription')"
                :footer-text="item.version || '--'"
                @click-card="clickDevice(item)"
              >
                <!-- Default footer icon -->
                <template #footer-icon>
                  <div class="service-icon-container">
                    <svg width="32" height="32" viewBox="0 0 100 100" fill="none">
                      <rect
                        x="15"
                        y="20"
                        width="70"
                        height="50"
                        rx="3"
                        fill="none"
                        stroke="#333"
                        stroke-width="3"
                      ></rect>
                      <line
                        x1="25"
                        y1="80"
                        x2="75"
                        y2="80"
                        stroke="#333"
                        stroke-width="3"
                        stroke-linecap="round"
                      ></line>
                    </svg>
                  </div>
                </template>
              </DevCardItem>
            </n-gi>
          </n-grid>
        </n-spin>
      </template>

      <!-- Footer pagination -->
      <template #footer>
        <NPagination
          v-if="deviceTemplateList.length || (pagination.pageCount ?? 0) > 1"
          v-model:page="pagination.page"
          :page-count="pagination.pageCount"
          @update:page="
            page => {
              pagination.page = page
              getData()
            }
          "
        />
      </template>
    </AdvancedListLayout>
  </div>
</template>

<style lang="scss" scoped>
.service-icon-container {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
}

.service-access-empty {
  min-height: 260px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}
</style>
