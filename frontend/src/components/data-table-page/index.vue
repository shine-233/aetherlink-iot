<!--
  文件用途：通用“搜索表单 + 卡片/列表/地图三视图 + 分页”列表组件（全局注册为 DataTablePage）。
  分层：
    - 查询/分页/过期请求丢弃/勾选状态 -> useListPage（本目录 useListPage.ts）；
    - 列生成、重置取值、下拉选项懒加载 -> useListPage.ts 中的纯函数（buildTableColumns /
      emptySearchValue / createLazyOptionsLoader），可单测；
    - 地图视图 -> DataTableMapView.vue（异步加载腾讯地图），由 showMap 开关或 #map-view 插槽覆盖。
  关键注意事项：defineExpose 契约（handleSearch/handleReset/forceChangeParamsByKey/dataList/
             selectedRows/clearSelection）被外部页面直接调用，保持不变。
-->
<script lang="tsx" setup>
import type { Component, VNodeChild } from 'vue'
import { computed, onMounted, onUnmounted, watchEffect } from 'vue'
import { debounce } from 'lodash-es'
import { useRouter } from 'vue-router'
import { GridOutline as CardIcon, ListOutline, MapOutline } from '@vicons/ionicons5'
import { NButton, NDataTable, NDatePicker, NInput, NPagination, NSelect, NSpace, NSpin } from 'naive-ui'
import type { DataTableRowKey, SelectOption } from 'naive-ui'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import { createLogger } from '@/utils/logger'
import { getPlatformApiBaseUrl } from '@/utils/common/tool'
import AdvancedListLayout from '@/components/list-page/index.vue'
import DevCardItem from '@/components/dev-card-item/index.vue'
import SvgIcon from '@/components/custom/svg-icon.vue'
import type { SearchConfig, theLabel } from './types'
import DataTableMapView from './DataTableMapView.vue'
import {
  buildTableColumns,
  createLazyOptionsLoader,
  emptySearchValue,
  fromFlatResponse,
  rowKeySignature,
  serializeDates,
  useListPage
} from './useListPage'
import type { LazyOptionsConfig, ListColumnSpec } from './useListPage'

interface DeviceItem {
  id: string
  device_number: string
  name: string
  device_config_id: string
  device_config_name: string
  ts: string | null
  activate_flag: string
  activate_at: string | null
  batch_number: string
  current_version: string
  created_at: string
  is_online: 0 | 1
  location: string
  access_way: string
  protocol_type: string
  device_status: number
  warn_status: string // 'N' 正常, 'Y' 告警
  device_type: string
  image_url?: string
  // DevCardItem 可能用到的备用字段
  title?: string
  description?: string
  status?: string | number
  value?: string
  indicator?: string
  timestamp?: string
  updatedAt?: string
  key?: string
}

// 查询值类型随控件而异（字符串/数组/时间戳/null），并直接绑定 v-model，故此处保留 any。
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type QueryState = Record<string, any>
type FlatResponse = { data?: unknown; error?: unknown } | null | undefined

interface ColumnSpec {
  key: string
  label: theLabel
  /** 自定义单元格渲染；历史调用方声明为无参函数，同样兼容。 */
  render?: (row: DeviceItem) => VNodeChild
  [prop: string]: unknown
}

const logger = createLogger('TablePage')

const props = withDefaults(
  defineProps<{
    /** 数据获取函数：接收扁平参数（筛选条件 + page + page_size），返回 { data, error }。 */
    fetchData: (params: QueryState & { page: number; page_size: number }) => Promise<FlatResponse> | FlatResponse
    /** 表格列配置；'all' 表示按首行字段自动生成列。 */
    columnsToShow: ColumnSpec[] | 'all'
    searchConfigs: SearchConfig[]
    tableActions: Array<{ theKey?: string; label: theLabel; callback: (...args: never[]) => unknown }>
    topActions: { element: Component | (() => VNodeChild) }[]
    rowClick?: (row: DeviceItem) => void
    initPage?: number
    initPageSize?: number
    selectableRows?: boolean
    /** 是否提供地图视图（默认 true，与历史行为一致）；传入 #map-view 插槽可替换为其他地图实现。 */
    showMap?: boolean
  }>(),
  { initPage: 1, initPageSize: 10, selectableRows: false, rowClick: undefined, showMap: true }
)

const emit = defineEmits<{
  paramsUpdate: [params: Record<string, unknown>]
  selectionUpdate: [rows: DeviceItem[]]
}>()

const getRowKey = (row: DeviceItem) => row.id || row.key || row.device_number

// ---- 列表状态：查询、分页、过期请求丢弃、勾选统一交给 useListPage ----
let hadSelection = false
const list = useListPage<DeviceItem, QueryState>({
  // searchConfigs 数组对象由父组件原地更新 options，这里只在初始化时读取 initValue。
  initialQuery: () => Object.fromEntries(props.searchConfigs.map((item) => [item.key, item.initValue])),
  initialPage: props.initPage || 1,
  initialPageSize: props.initPageSize || 10,
  pageSizes: [10, 20, 30, 40, 50],
  rowKey: getRowKey,
  // 父页面（设备状态推送）会原地修改行字段，需要深响应。
  deepRows: true,
  serialize: serializeDates,
  fetcher: async (params) => {
    const { page: _page, page_size: _pageSize, ...criteria } = params
    emit('paramsUpdate', criteria)
    const response = await props.fetchData(params)
    if (response?.error) logger.error({ 'Error fetching data:': response.error })
    return fromFlatResponse<DeviceItem>(response)
  },
  onLoaded: () => {
    if (props.selectableRows && hadSelection) emit('selectionUpdate', list.selectedRows.value)
  }
})

const { loading, total, rows: dataList, selectedKeys: selectedRowKeys, selectedRows } = list
const searchCriteria = list.query
const currentPage = list.page
const pageSize = list.pageSize
const getData = () => {
  hadSelection = selectedRowKeys.value.length > 0
  return list.load()
}

const isEmpty = computed(() => !loading.value && dataList.value.length === 0)

// ---- 列生成：只随列配置变化重建，行级格式化放进 render 回调 ----
const renderCell = (row: DeviceItem, key: string): VNodeChild => {
  const value = (row as unknown as Record<string, unknown>)[key]
  if (key === 'ts' && value) return formatDateTime(String(value))
  return <>{value}</>
}

// 'all' 模式下列跟随首行字段：用字段签名做中间 computed，数据刷新但字段不变时不会触发列重建。
const allColumnsSignature = computed(() => (props.columnsToShow === 'all' ? rowKeySignature(dataList.value[0]) : ''))

const columnSpecs = computed<ListColumnSpec<DeviceItem>[]>(() => {
  if (props.columnsToShow !== 'all') return props.columnsToShow
  const signature = allColumnsSignature.value
  return signature ? signature.split('\u0000').map((key) => ({ key, label: key })) : []
})

const generatedColumns = computed(() =>
  buildTableColumns(columnSpecs.value, { selectable: props.selectableRows, renderCell })
)
// ---- 查询交互 ----
type ExtendParamsConfig = {
  key: string
  extendParams?: { label: string; value: string }[]
  options?: Array<{ dict_value: string; device_type: string; [field: string]: unknown }>
}

// 观察搜索条件变化以回填扩展参数（如选中设备类型后写入其关联字段），不自动获取数据。
watchEffect(() => {
  for (const config of props.searchConfigs as ExtendParamsConfig[]) {
    const selected = searchCriteria[config.key]
    if (!config.extendParams || !selected) continue
    for (const option of config.options ?? []) {
      if (option.dict_value + option.device_type !== selected) continue
      for (const param of config.extendParams) searchCriteria[param.label] = option[param.value]
    }
  }
})

const handleSearch = () => {
  currentPage.value = 1
  return getData()
}

const handleReset = () => {
  for (const key of Object.keys(searchCriteria)) {
    const config = props.searchConfigs.find((item) => item.key === key)
    if (config) searchCriteria[key] = emptySearchValue(config)
  }
  return handleSearch()
}

// 强制更新指定参数并刷新数据（保持当前页码，与历史行为一致）
const forceChangeParamsByKey = (params: Record<string, unknown>) => {
  for (const [key, value] of Object.entries(params)) {
    if (key in searchCriteria) searchCriteria[key] = value
  }
  return getData()
}

const clearSelection = () => {
  list.clearSelection()
  emit('selectionUpdate', [])
}

// 暴露给父组件的契约，保持不变。
defineExpose({
  handleSearch,
  handleReset,
  forceChangeParamsByKey,
  dataList, // 父组件可直接原地更新行数据
  selectedRows,
  clearSelection
})

const onUpdatePage = (newPage: number) => {
  currentPage.value = newPage
  getData()
}
const onUpdatePageSize = (newPageSize: number) => {
  pageSize.value = newPageSize
  currentPage.value = 1
  getData()
}

const handleCheckedRowKeysUpdate = (keys: DataTableRowKey[]) => {
  list.setSelectedKeys(keys)
  emit('selectionUpdate', selectedRows.value)
}

const handleTreeSelectUpdate = (value: unknown, key: string) => {
  searchCriteria[key] = value
  handleSearch()
}

const debouncedInputSearch = debounce(() => {
  handleSearch()
}, 400)
const handleInputChange = () => debouncedInputSearch()
const handleSelectChange = () => handleSearch()

// select / tree-select 的动态选项：首次展开时加载一次，并发展开共享同一请求。
const lazyOptions = createLazyOptionsLoader()
const ensureSearchOptionsLoadedWhenShown = (show: boolean, config: SearchConfig) => {
  if (!show) return
  lazyOptions.ensure(config as LazyOptionsConfig).catch((error) => {
    logger.error({ 'Error loading search options:': error, key: config.key })
  })
}

const filterSelectOption = (pattern: string, option: SelectOption) => {
  const label = typeof option.label === 'string' ? option.label : ''
  return label.includes(pattern)
}

// ---- 行点击 ----
const INTERACTIVE_ROW_TARGETS = [
  'button',
  'a',
  'input',
  'textarea',
  'select',
  '[role="button"]',
  '[role="checkbox"]',
  '.n-checkbox',
  '.n-button',
  '.n-dropdown',
  '.n-select',
  '.n-tree-select'
].join(',')

const isInteractiveRowClickTarget = (event: MouseEvent) => {
  const target = event.target
  return target instanceof HTMLElement && Boolean(target.closest(INTERACTIVE_ROW_TARGETS))
}

const rowProps = (row: DeviceItem) => {
  const onRowClick = props.rowClick
  if (!onRowClick) return {}
  return {
    style: 'cursor: pointer;',
    onClick: (event: MouseEvent) => {
      if (!isInteractiveRowClickTarget(event)) onRowClick(row)
    }
  }
}

onMounted(() => {
  getData()
})

onUnmounted(() => {
  debouncedInputSearch.cancel()
})

// ---- 卡片视图辅助 ----
const deviceTypeIcons: Record<string, string> = {
  '1': 'direct', // 直连设备
  '2': 'gateway', // 网关
  '3': 'subdevice', // 网关子设备
  default: 'defaultdevice'
}

// “默认配置”或无配置名时统一使用直连设备图标
const getDeviceIconName = (deviceType: string, deviceConfigName?: string): string => {
  if (!deviceConfigName || deviceConfigName === '默认配置') return deviceTypeIcons['1']
  return deviceTypeIcons[deviceType] || deviceTypeIcons.default
}

const platformAssetBaseUrl = getPlatformApiBaseUrl().replace('api/v1', '')
const getConfigImageUrl = (imagePath: string | undefined): string => {
  if (!imagePath) return '' // 空字符串交给模板走默认图标
  return platformAssetBaseUrl + imagePath.replace(/^\.?\//, '')
}

const router = useRouter()
// 告警铃铛：有告警跳到该设备的告警详情，否则跳到告警列表
const handleWarningClick = (item: DeviceItem) => {
  router.push(item.warn_status === 'Y' ? `/alarm/warning-message?device_id=${item.id}` : '/alarm/warning-message')
}

// map 视图仅在 showMap 时出现（AdvancedListLayout 按插槽是否存在过滤可选视图）
const availableViews = [
  { key: 'card', icon: CardIcon, label: 'common.viewCard' },
  { key: 'list', icon: ListOutline, label: 'common.viewList' },
  { key: 'map', icon: MapOutline, label: 'common.viewMap' }
]
const formSize = undefined
</script>

<template>
  <AdvancedListLayout
    :initial-view="'card'"
    :available-views="availableViews"
    @query="handleSearch()"
    @reset="handleReset()"
    @refresh="getData()"
  >
    <!-- 搜索表单内容 -->
    <template #search-form-content>
      <n-grid cols="1 s:2 m:3 l:4 xl:6 2xl:8" x-gap="18" y-gap="18" responsive="screen">
        <n-gi v-for="config in searchConfigs" :key="config.key">
          <template v-if="config.type === 'input'">
            <NInput
              v-model:value="searchCriteria[config.key]"
              :size="formSize"
              :placeholder="$t(config.label)"
              class="input-style"
              @update:value="handleInputChange"
            />
          </template>
          <template v-else-if="config.type === 'date-range'">
            <NDatePicker
              v-model:value="searchCriteria[config.key]"
              :size="formSize"
              type="daterange"
              :placeholder="$t(config.label)"
              class="input-style"
            />
          </template>
          <template v-else-if="config.type === 'select'">
            <NSelect
              v-model:value="searchCriteria[config.key]"
              :value-field="config.valueField"
              :label-field="config.labelField"
              :size="formSize"
              filterable
              :filter="filterSelectOption"
              :options="config.options"
              :render-label="config.renderLabel"
              :render-tag="config.renderTag"
              :placeholder="$t(config.label)"
              class="input-style"
              @update:show="(show) => ensureSearchOptionsLoadedWhenShown(show, config)"
              @update:value="handleSelectChange"
            />
          </template>
          <template v-else-if="config.type === 'date'">
            <NDatePicker
              v-model:value="searchCriteria[config.key]"
              :size="formSize"
              type="date"
              :placeholder="$t(config.label)"
              class="input-style"
            />
          </template>
          <template v-else-if="config.type === 'tree-select'">
            <n-tree-select
              v-model:value="searchCriteria[config.key]"
              :size="formSize"
              filterable
              :options="config.options"
              :multiple="config.multiple"
              class="input-style"
              @update:show="(show) => ensureSearchOptionsLoadedWhenShown(show, config)"
              @update:value="(value) => handleTreeSelectUpdate(value, config.key)"
            />
          </template>
        </n-gi>
        <n-gi>
          <n-space>
            <n-button type="primary" :size="formSize" @click="handleSearch">
              {{ $t('generate.query') }}
            </n-button>
            <n-button type="default" :size="formSize" @click="handleReset">
              {{ $t('generate.reset') }}
            </n-button>
          </n-space>
        </n-gi>
      </n-grid>
    </template>

    <!-- 头部左侧操作区域 -->
    <template #header-left>
      <div class="flex gap-2">
        <component :is="action.element" v-for="(action, index) in topActions" :key="index"></component>
      </div>
    </template>

    <!-- 卡片视图 - 使用铃铛图标插槽 -->
    <template #card-view>
      <n-scrollbar style="height: calc(100vh - 442px)" :size="1">
        <n-spin :show="loading">
          <slot
            v-if="isEmpty && $slots.empty"
            name="empty"
            :reset="handleReset"
            :search-criteria="searchCriteria"
            :total="total"
          />
          <NGrid x-gap="20px" y-gap="20px" cols="1 s:2 m:3 l:4" responsive="screen">
            <NGridItem v-for="item in dataList" :key="item.id">
              <DevCardItem
                :title="item.name || 'N/A'"
                :status-active="item.is_online === 1"
                :subtitle="item.device_config_name || '--'"
                :footer-text="(item.ts ? formatDateTime(item.ts) : null) ?? '--'"
                :warn-status="item.warn_status"
                :device-id="item.id"
                @click-card="() => props.rowClick && props.rowClick(item)"
                @click-top-right-icon="handleWarningClick(item)"
              >
                <template #subtitle-icon>
                  <SvgIcon
                    :local-icon="getDeviceIconName(item.device_type, item.device_config_name)"
                    class="image-icon"
                  />
                </template>

                <!-- 右上角铃铛图标插槽 -->
                <template #top-right-icon>
                  <svg
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    :fill="item.warn_status === 'Y' ? '#ff4d4f' : '#d9d9d9'"
                    class="bell-icon"
                  >
                    <!-- 铃铛图标 SVG 路径 -->
                    <path
                      d="M12 22c1.1 0 2-.9 2-2h-4c0 1.1.89 2 2 2zm6-6v-5c0-3.07-1.64-5.64-4.5-6.32V4c0-.83-.67-1.5-1.5-1.5s-1.5.67-1.5 1.5v.68C7.63 5.36 6 7.92 6 11v5l-2 2v1h16v-1l-2-2z"
                    />
                  </svg>
                </template>
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
                    <SvgIcon v-else local-icon="defaultdevice" class="config-image" />
                  </div>
                </template>
              </DevCardItem>
            </NGridItem>
          </NGrid>
        </n-spin>
      </n-scrollbar>
    </template>

    <!-- 列表视图 -->
    <template #list-view>
      <n-scrollbar style="height: calc(100vh - 442px)" :size="1">
        <slot
          v-if="isEmpty && $slots.empty"
          name="empty"
          :reset="handleReset"
          :search-criteria="searchCriteria"
          :total="total"
        />
        <NDataTable
          v-else
          aria-label="data table"
          size="small"
          :row-props="rowProps"
          :row-key="getRowKey"
          :loading="loading"
          :columns="generatedColumns"
          :data="dataList"
          :checked-row-keys="selectedRowKeys"
          class="w-full"
          virtual-scroll
          :max-height="'calc(100vh - 442px)'"
          @update:checked-row-keys="handleCheckedRowKeysUpdate"
        />
      </n-scrollbar>
    </template>

    <!-- 地图视图 -->
    <!-- 不提供该插槽时 AdvancedListLayout 会自动隐藏地图视图入口 -->
    <template v-if="showMap" #map-view>
      <slot name="map-view" :rows="dataList" :loading="loading">
        <DataTableMapView :devices="dataList" :loading="loading" :show-empty="isEmpty && !!$slots.empty">
          <template #empty>
            <slot name="empty" :reset="handleReset" :search-criteria="searchCriteria" :total="total" />
          </template>
        </DataTableMapView>
      </slot>
    </template>

    <!-- 底部分页 -->
    <template #footer>
      <NPagination
        v-model:page="currentPage"
        v-model:page-size="pageSize"
        class="justify-end"
        :item-count="total"
        :page-sizes="[10, 20, 30, 40, 50]"
        show-size-picker
        @update:page="onUpdatePage"
        @update:page-size="onUpdatePageSize"
      />
    </template>
  </AdvancedListLayout>
</template>

<style scoped lang="scss">
.btn-style {
  @apply hover:bg-[var(--color-primary-hover)] rounded-md shadow;
}

.card-wrapper {
  @apply rounded-lg shadow overflow-hidden;
  margin: 0 auto;
  padding: 16px;
}

.image-icon {
  max-width: 100%;
  max-height: 100%;
  width: 24px;
  height: 24px;
  object-fit: contain;
}

.bell-icon {
  transition: fill 0.3s ease;
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
</style>
