<!--
设备配置列表页，负责本地设备配置管理与物模型市场浏览/安装的双入口编排。
核心链路：加载本地设备配置列表 -> 支持搜索、分页、详情跳转和编辑 -> 在“市场”tab 内浏览物模型 -> 安装物模型或把本地配置发布到市场。
静态维护重点：
1. 页面同时承载“本地配置管理”和“市场物模型流转”两条业务线，后续若继续扩展市场能力，建议抽独立 composable 管理发布/登录状态。
2. 发布到市场的真实主键是 `device_config_id`，且要求当前配置已绑定物模型；这个约束要与后端发布接口保持一致。
3. 本地列表、市场登录弹窗和发布确认弹窗之间通过 pending 状态串联，后续若增加更多动作，建议收敛成更显式的状态机。
-->
<script lang="tsx" setup>
import { onMounted, ref, computed, h, onActivated, watch, defineAsyncComponent, nextTick } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton,
  NInput,
  NIcon,
  NPagination,
  NDataTable,
  NTag,
  NSpace,
  NEmpty,
  NDropdown,
  NTabs,
  NTabPane,
  NTooltip
} from 'naive-ui'
import { SearchOutline } from '@vicons/ionicons5'
import { deviceConfig } from '@/service/api/device'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
import AdvancedListLayout from '@/components/list-page/index.vue'
import ItemCard from '@/components/dev-card-item/index.vue'

const MarketLoginModal = defineAsyncComponent(() => import('./modules/market-login-modal.vue'))
const MarketTemplateList = defineAsyncComponent(() => import('./modules/market-template-list.vue'))
const PublishConfirmModal = defineAsyncComponent(() => import('./modules/publish-confirm-modal.vue'))

const router = useRouter()
const { routerPushByKey } = useRouterPush()

// 市场相关弹窗和挂起发布状态都集中在页面壳层统一编排。
const marketLoginRef = ref<ComponentPublicInstance<{ open: () => void }> | null>(null)
const publishConfirmRef = ref<ComponentPublicInstance<{ open: (deviceConfigId: string, defaultName?: string) => void }> | null>(
  null
)
const pendingPublishId = ref('')
const pendingPublishName = ref('')
const activeTab = ref('local')
const marketTabVisited = ref(false)
const marketLoginVisited = ref(false)
const publishConfirmVisited = ref(false)

watch(activeTab, value => {
  if (value === 'market') {
    marketTabVisited.value = true
  }
})

// 市场物模型安装成功后切回本地配置页，并刷新本地列表，方便用户继续查看新装入的配置。
const handleInstalled = () => {
  activeTab.value = 'local'
  getData()
}

const openMarketLoginModal = async () => {
  marketLoginVisited.value = true
  await nextTick()
  marketLoginRef.value?.open()
}

const openPublishConfirmModal = async (deviceConfigId: string, defaultName?: string) => {
  publishConfirmVisited.value = true
  await nextTick()
  publishConfirmRef.value?.open(deviceConfigId, defaultName)
}

// 本地设备配置列表查询参数；市场物模型列表由子组件内部单独维护。
const queryData = ref({
  page: 1,
  page_size: 10,
  name: ''
})

// 本地配置列表是真相源，发布、编辑或返回列表页后都依赖它刷新。
const deviceConfigList = ref([] as any[])
const dataTotal = ref(0)
const loading = ref(false)
let deviceConfigRequestSeq = 0
let localConfigMounted = false

// 获取本地设备配置列表，供“local” tab 的表格和卡片展示复用。
const getData = async () => {
  const requestSeq = ++deviceConfigRequestSeq
  loading.value = true
  try {
    const res = await deviceConfig(queryData.value)
    if (requestSeq !== deviceConfigRequestSeq) return
    if (!res.error) {
      deviceConfigList.value = res.data.list
      dataTotal.value = res.data.total
    }
  } finally {
    if (requestSeq === deviceConfigRequestSeq) {
      loading.value = false
    }
  }
}

// 搜索时回到第一页，避免带着旧分页造成结果缺失错觉。
const handleQuery = async () => {
  queryData.value.page = 1
  await getData()
}

// 重置只影响本地配置列表，不动市场 tab 的搜索状态。
const handleReset = async () => {
  queryData.value.page = 1
  queryData.value.name = ''
  await getData()
}

// 新建设备配置走独立编辑页，当前列表页不内嵌复杂创建表单。
const handleAddNew = () => {
  routerPushByKey('device_config-edit')
}

// 详情页承载更完整的物模型绑定、协议表单和物模型信息。
const goToDetail = (id: string) => {
  router.push({ path: '/device/config-detail', query: { id } })
}

// 发布到市场前统一检查是否已有 market token；无 token 时先挂起动作并拉起登录弹窗。
const handlePublishToMarket = (deviceConfigId: string, defaultName?: string) => {
  if (!deviceConfigId) {
    window.$message?.warning($t('device_template.requireThingModelBeforePublish'))
    return
  }
  const token = sessionStorage.getItem('market_token')
  if (!token) {
    pendingPublishId.value = deviceConfigId
    pendingPublishName.value = defaultName || ''
    void openMarketLoginModal()
  } else {
    void openPublishConfirmModal(deviceConfigId, defaultName)
  }
}

// 登录成功后继续刚才被挂起的发布动作，避免用户重复点击发布按钮。
const onMarketLoginSuccess = () => {
  if (pendingPublishId.value) {
    void openPublishConfirmModal(pendingPublishId.value, pendingPublishName.value)
    pendingPublishId.value = ''
    pendingPublishName.value = ''
  }
}

// 列表里仍用轻量文本区分直连设备、网关和子设备，便于用户快速浏览配置类型。
const deviceTypeMap = {
  '1': $t('generate.direct-connected-device'),
  '2': $t('generate.gateway'),
  '3': $t('generate.gateway-sub-device')
}

// 本地配置表格列定义同时承载详情跳转、编辑和发布入口。
const columns = computed(() => [
  {
    title: $t('device_template.templateName'),
    key: 'name',
    ellipsis: {
      tooltip: true
    },
    render: (row: any) => {
      return h(
        NButton,
        {
          text: true,
          type: 'primary',
          onClick: () => goToDetail(row.id)
        },
        { default: () => row.name }
      )
    }
  },
  {
    title: $t('generate.device-type'),
    key: 'device_type',
    render: (row: any) => {
      const typeText = deviceTypeMap[row.device_type as keyof typeof deviceTypeMap] || row.device_type
      const type = row.device_type === '1' ? 'info' : row.device_type === '2' ? 'success' : 'warning'
      return h(NTag, { type }, { default: () => typeText })
    }
  },
  {
    title: $t('generate.device-count'),
    key: 'device_count',
    render: (row: any) => `${row.device_count} ${$t('generate.individual')}`
  },
  {
    title: $t('common.actions'),
    key: 'actions',
    width: 200,
    render: (row: any) => {
      return h(
        NSpace,
        {},
        {
          default: () => [
            h(
              NButton,
              {
                size: 'small',
                onClick: () => handleEdit(row.id)
              },
              { default: () => $t('common.edit') }
            ),
            h(
              NTooltip,
              { trigger: 'hover', disabled: !!row.device_template_id },
              {
                trigger: () =>
                  h(
                    NButton,
                    {
                      size: 'small',
                      type: 'info',
                      disabled: !row.device_template_id,
                      onClick: () => handlePublishToMarket(row.id, row.name)
                    },
                    { default: () => $t('device_template.publishToMarket') }
                  ),
                default: () => $t('device_template.requireThingModelBeforePublish')
              }
            )
          ]
        }
      )
    }
  }
])

// 编辑复用设备配置编辑页，当前页面只负责跳转。
const handleEdit = (id: string) => {
  routerPushByKey('device_config-edit', { query: { id } })
}

// 本地配置列表分页切换。
const handlePageChange = (page: number) => {
  queryData.value.page = page
  getData()
}

// 分页大小变化时重置页码，避免越界请求。
const handlePageSizeChange = (pageSize: number) => {
  queryData.value.page_size = pageSize
  queryData.value.page = 1
  getData()
}

// 排序处理
const handleSorterChange = () => {
  // 根据需要实现排序逻辑
}

// 刷新数据
const handleRefresh = () => {
  getData()
}

// 组件挂载时获取数据
onMounted(() => {
  localConfigMounted = true
  getData()
})

// 打开时自动刷新页面
onActivated(() => {
  if (!localConfigMounted || loading.value) return
  getData()
})
import { ListOutline, GridOutline as CardIcon, EllipsisHorizontal } from '@vicons/ionicons5'
import SvgIcon from '@/components/custom/svg-icon.vue'
import { getPlatformApiBaseUrl } from '@/utils/common/tool'

const platformApiBaseUrl = getPlatformApiBaseUrl()

// 设备类型图标映射 - 使用本地SVG图标名称
const deviceTypeIcons = {
  1: 'direct', // 直连设备
  2: 'gateway', // 网关设备
  3: 'subdevice', // 子设备
  default: 'defaultdevice' // 默认设备图标
}

// 获取设备图标名称的函数
const getDeviceIconName = (deviceType: string): string => {
  return deviceTypeIcons[deviceType] || deviceTypeIcons.default
}

const getConfigImageUrl = (imageUrl?: string) => {
  if (!imageUrl) return ''
  if (/^https?:\/\//i.test(imageUrl)) return imageUrl
  return `${platformApiBaseUrl.replace('api/v1', '')}${imageUrl}`
}

const availableViews = [
  { key: 'card', icon: CardIcon, label: 'common.viewCard' },
  { key: 'list', icon: ListOutline, label: 'common.viewList' }
]
</script>

<template>
  <div class="p-4">
    <NTabs v-model:value="activeTab" type="line" animated>
      <NTabPane name="local" :tab="$t('device_template.localTemplates')">
        <AdvancedListLayout
          :loading="loading"
          :show-query-button="false"
          :show-reset-button="false"
          :available-views="availableViews"
          @add-new="handleAddNew"
          @query="handleQuery"
          @reset="handleReset"
          @refresh="handleRefresh"
        >
          <template #header-left>
            <div class="flex gap-2">
              <n-button type="primary" @click="handleAddNew">{{ $t('generate.createDeviceConfig') }}</n-button>
            </div>
          </template>
          <!-- 搜索表单内容 -->
          <template #search-form-content>
            <div class="flex gap-4">
              <NInput
                v-model:value="queryData.name"
                :placeholder="$t('generate.enter-config-name')"
                type="text"
                clearable
                style="width: 210px"
                @clear="handleReset"
                @keydown.enter="handleQuery"
              >
                <template #prefix>
                  <NIcon>
                    <SearchOutline />
                  </NIcon>
                </template>
              </NInput>
              <NButton class="w-72px" type="primary" @click="handleQuery">{{ $t('common.search') }}</NButton>
            </div>
          </template>

          <!-- 卡片视图 -->
          <template #card-view>
            <n-spin :show="loading">
              <div v-if="deviceConfigList.length === 0 && !loading" class="empty-state">
                <NEmpty size="huge" :description="$t('common.noData')" class="min-h-60" />
              </div>
              <n-grid cols="1 s:2 m:3 l:4 xl:5 2xl:8" x-gap="18" y-gap="18" responsive="screen">
                <n-gi v-for="item in deviceConfigList" :key="item.id">
                  <ItemCard
                    :title="item.name"
                    :footer-text="`${item.device_count} ${$t('generate.individual')} ${$t('generate.device')}`"
                    :subtitle="deviceTypeMap[item.device_type as keyof typeof deviceTypeMap]"
                    :device-config-id="item.id"
                    :isStatus="false"
                    @click-card="goToDetail(item.id)"
                  >
                    <template #subtitle-icon>
                      <SvgIcon :local-icon="getDeviceIconName(item.device_type)" class="image-icon" />
                    </template>

                    <!-- 右上角操作按钮 -->
                    <template #top-right-icon>
                      <NDropdown
                        placement="bottom-end"
                        trigger="hover"
                        :options="[
                          { label: $t('common.edit'), key: 'edit' },
                          {
                            label: $t('device_template.publishToMarket'),
                            key: 'publish',
                            disabled: !item.device_template_id
                          }
                        ]"
                        @select="
                          key => {
                            if (key === 'edit') handleEdit(item.id)
                            if (key === 'publish') handlePublishToMarket(item.id, item.name)
                          }
                        "
                      >
                        <NTooltip :disabled="!!item.device_template_id" trigger="hover">
                          <template #trigger>
                            <NButton size="tiny" quaternary circle>
                              <template #icon>
                                <NIcon><EllipsisHorizontal /></NIcon>
                              </template>
                            </NButton>
                          </template>
                          {{ $t('device_template.requireThingModelBeforePublish') }}
                        </NTooltip>
                      </NDropdown>
                    </template>

                    <!-- 底部图标 - 左下角显示配置图片 -->
                    <template #footer-icon>
                      <div class="footer-icon-container">
                        <img
                          v-if="item.image_url"
                          :src="getConfigImageUrl(item.image_url)"
                          alt="config image"
                          loading="lazy"
                          decoding="async"
                          class="config-image"
                        />
                        <SvgIcon v-else local-icon="default-config" class="config-image" />
                      </div>
                    </template>

                    <!-- 卡片内容区域可以显示更多信息 -->
                  </ItemCard>
                </n-gi>
              </n-grid>
            </n-spin>
          </template>

          <!-- 表格视图 -->
          <template #list-view>
            <NDataTable
              :columns="columns"
              :data="deviceConfigList"
              :loading="loading"
              size="small"
              :pagination="false"
              :bordered="false"
              :single-line="false"
              striped
              @update:sorter="handleSorterChange"
            />
          </template>

          <!-- 底部分页 -->
          <template #footer>
            <NPagination
              v-model:page="queryData.page"
              :page-size="queryData.page_size"
              :item-count="dataTotal"
              show-size-picker
              :page-sizes="[10, 20, 30, 50]"
              @update:page="handlePageChange"
              @update:page-size="handlePageSizeChange"
            />
          </template>
        </AdvancedListLayout>
      </NTabPane>

      <NTabPane name="market" :tab="$t('device_template.marketTemplates')">
        <MarketTemplateList v-if="marketTabVisited" @installed="handleInstalled" />
      </NTabPane>
    </NTabs>

    <!-- 市场登录弹窗 -->
    <MarketLoginModal v-if="marketLoginVisited" ref="marketLoginRef" @login-success="onMarketLoginSuccess" />
    <!-- 发布确认弹窗 -->
    <PublishConfirmModal v-if="publishConfirmVisited" ref="publishConfirmRef" @publish-success="getData" />
  </div>
</template>

<style scoped lang="scss">
.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 300px;
}

.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 20px;
  padding: 0 4px;
}

.card-item {
  min-height: 200px;
}

.card-extra-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}

.info-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 14px;
}

.info-label {
  color: #666;
  font-weight: 500;
}

.info-value {
  color: #333;
}

// 设备类型图标样式
.image-icon {
  width: 24px;
  height: 24px;
  object-fit: contain;
  vertical-align: middle;
}

// 底部图标容器 - 固定40x40正方形
.footer-icon-container {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 6px;
  background-color: #f8f9fa;
  border: 1px solid #e9ecef;
}

.config-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center;
}

// 响应式设计
@media (max-width: 768px) {
  .card-grid {
    grid-template-columns: 1fr;
    gap: 16px;
  }
}

@media (min-width: 769px) and (max-width: 1200px) {
  .card-grid {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  }
}

@media (min-width: 1201px) {
  .card-grid {
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  }
}
</style>
