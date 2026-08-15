<!--
市场物模型列表组件，负责市场物模型的查询、筛选、详情查看、登录拦截与安装触发。
核心链路：加载物模型列表 -> 支持关键字/分类/排序筛选 -> 查看物模型详情抽屉 -> 安装前检查市场登录态 -> 调用安装接口并处理缺失插件提示。
静态维护重点：
1. 登录态与安装副作用都集中在本组件，后续若市场能力继续扩展，建议把“鉴权”“安装反馈”“缺失插件弹窗”拆成独立 composable。
2. 安装错误分支同时处理 token 失效、重复安装和缺失插件提示，逻辑已经偏重，后续应优先抽离错误分类 helper。
3. 分类选项当前部分写死中文值，若市场分类体系扩展，前后端必须先统一 value 契约，避免筛选值漂移。
-->
<script setup lang="ts">
import { ref, reactive, onMounted, watch, defineAsyncComponent, nextTick } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { NAlert, NButton, NInput, NSelect, NSpace, NSpin, NGrid, NGi, NEmpty, NPagination, NIcon } from 'naive-ui'
import { SearchOutline } from '@vicons/ionicons5'
import { $t } from '@/locales'
import { getMarketTemplates, installFromMarket } from '@/service/api/market'
import { useMarketAuth } from '../composables/use-market-auth'
import { useMarketTemplateInstall } from '../composables/useMarketTemplateInstall'
import MarketTemplateCard from './market-template-card.vue'
import MarketTemplateDrawer from './market-template-drawer.vue'

const MarketLoginModal = defineAsyncComponent(() => import('./market-login-modal.vue'))

const emit = defineEmits(['installed'])

const { isLoggedIn, getToken, clearToken } = useMarketAuth()

const loading = ref(false)
const templateList = ref<any[]>([])
const total = ref(0)
const listError = ref('')

// 搜索条件既驱动筛选，也驱动分页；修改任一非分页条件时都要回到第一页。
const searchParams = reactive({
  keyword: '',
  category: null as string | null,
  sort_by: 'latest',
  page: 1,
  page_size: 12
})

const categoryOptions = [
  { label: 'IoT', value: 'IoT' },
  { label: $t('device_template.marketCatIndustrial'), value: '工业' },
  { label: $t('device_template.marketCatAgriculture'), value: '农业' },
  { label: $t('device_template.marketCatSmartCity'), value: '智慧城市' },
  { label: $t('device_template.marketCatOther'), value: '其他' }
]

const sortOptions = [
  { label: $t('market.sortLatest'), value: 'latest' },
  { label: $t('market.sortHottest'), value: 'hottest' }
]

const getMarketErrorMessage = (error: any) => error?.msg || error?.message || $t('market.loadFailed')

// 详情抽屉只关心当前选中的物模型 ID。
const drawerVisible = ref(false)
const selectedTemplateId = ref('')

// 登录弹窗用于补全安装前的市场登录，不直接管理物模型列表数据。
const marketLoginRef = ref<ComponentPublicInstance<{ open: () => void }> | null>(null)
const marketLoginVisited = ref(false)

const openMarketLoginModal = async () => {
  marketLoginVisited.value = true
  await nextTick()
  const open = marketLoginRef.value?.open
  if (typeof open === 'function') {
    open()
  }
}

const findTemplateName = (id: string) => templateList.value.find(item => String(item.id) === String(id))?.name || ''

const {
  pendingInstallId,
  isInstalling,
  handleInstall,
  doInstall,
  onMarketLoginSuccess
} = useMarketTemplateInstall({
  isLoggedIn,
  getToken,
  clearToken,
  openLoginModal: () => {
    void openMarketLoginModal()
  },
  resolveTemplateName: findTemplateName,
  installTemplate: (payload: { market_template_id: string; market_token: string }) =>
    installFromMarket(payload) as any,
  onInstalled: () => emit('installed'),
  t: $t,
  message: {
    success: (message) => window.$message?.success(message),
    warning: (message) => window.$message?.warning(message),
    error: (message) => window.$message?.error(message)
  },
  dialog: {
    success: (options) => window.$dialog?.success?.(options),
    warning: (options) => window.$dialog?.warning?.(options)
  }
})

// 物模型列表是真相源，首次进入、筛选变化和分页切换都复用这一入口。
const fetchMarketTemplates = async () => {
  loading.value = true
  listError.value = ''
  try {
    const params: any = {
      page: searchParams.page,
      page_size: searchParams.page_size,
      sort_by: searchParams.sort_by
    }
    if (searchParams.keyword) params.keyword = searchParams.keyword
    if (searchParams.category) params.category = searchParams.category

    const res: any = await getMarketTemplates(params)
    if (res && !res.error) {
      templateList.value = res.data?.list || (Array.isArray(res.data) ? res.data : [])
      total.value = res.data?.total ?? 0
    } else {
      templateList.value = []
      total.value = 0
      listError.value = getMarketErrorMessage(res?.error)
    }
  } catch (e) {
    console.error(e)
    templateList.value = []
    total.value = 0
    listError.value = getMarketErrorMessage(e)
  } finally {
    loading.value = false
  }
}

// 显式搜索时重置回第一页，避免沿用旧分页造成“搜不到”的假象。
const handleSearch = () => {
  searchParams.page = 1
  fetchMarketTemplates()
}

const debouncedSearch = useDebounceFn(() => {
  handleSearch()
}, 500)

// 关键字搜索做防抖，避免市场接口被频繁触发。
watch(
  () => searchParams.keyword,
  () => {
    debouncedSearch()
  }
)

// 查看详情只更新当前物模型 ID，由抽屉组件自己决定何时拉详情。
const handleViewDetail = (id: string) => {
  selectedTemplateId.value = id
  drawerVisible.value = true
}

onMounted(() => {
  fetchMarketTemplates()
})
</script>

<template>
  <div class="market-template-list">
    <!-- Filter area -->
    <NSpace class="mb-4" align="center" :size="12">
      <NInput
        v-model:value="searchParams.keyword"
        :placeholder="$t('market.searchPlaceholder')"
        clearable
        style="width: 260px"
        @keyup.enter="handleSearch"
      >
        <template #prefix>
          <NIcon><SearchOutline /></NIcon>
        </template>
      </NInput>
      <NSelect
        v-model:value="searchParams.category"
        :options="categoryOptions"
        :placeholder="$t('market.allCategories')"
        clearable
        style="width: 140px"
        @update:value="handleSearch"
      />
      <NSelect
        v-model:value="searchParams.sort_by"
        :options="sortOptions"
        style="width: 120px"
        @update:value="handleSearch"
      />
    </NSpace>

    <!-- Template card grid -->
    <NSpin :show="loading">
      <div v-if="!loading && listError" class="market-list-error">
        <NAlert type="error" :show-icon="false">
          <template #header>{{ $t('market.loadFailed') }}</template>
          <div class="market-list-error-content">
            <span>{{ listError }}</span>
            <NButton size="small" secondary @click="fetchMarketTemplates">{{ $t('market.retry') }}</NButton>
          </div>
        </NAlert>
      </div>
      <NEmpty
        v-else-if="!loading && !templateList.length"
        :description="$t('market.noTemplates')"
        style="padding: 80px 0"
      />
      <NGrid v-else cols="1 s:2 m:3 l:4" x-gap="16" y-gap="16" responsive="screen">
        <NGi v-for="item in templateList" :key="item.id">
          <MarketTemplateCard
            :template="item"
            :installing="isInstalling(item.id)"
            @install="handleInstall"
            @view-detail="handleViewDetail"
          />
        </NGi>
      </NGrid>
    </NSpin>

    <!-- Pagination -->
    <div v-if="total > searchParams.page_size" class="mt-4" style="display: flex; justify-content: flex-end">
      <NPagination
        v-model:page="searchParams.page"
        :page-size="searchParams.page_size"
        :item-count="total"
        @update:page="fetchMarketTemplates"
      />
    </div>

    <!-- Template detail drawer -->
    <MarketTemplateDrawer
      v-model:visible="drawerVisible"
      :template-id="selectedTemplateId"
      :installing="isInstalling(selectedTemplateId)"
      @install="handleInstall"
    />

    <!-- Market login modal -->
    <MarketLoginModal v-if="marketLoginVisited" ref="marketLoginRef" @login-success="onMarketLoginSuccess" />
  </div>
</template>

<style scoped>
.market-list-error {
  padding: 32px 0;
}

.market-list-error-content {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
</style>
