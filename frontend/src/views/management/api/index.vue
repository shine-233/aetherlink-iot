<!--
文件用途：承载后台管理中的 API 密钥管理页，负责展示租户级 API Key 列表并提供新增、编辑、删除、启停能力。
核心逻辑：页面通过查询参数、远端分页、表格列渲染与弹窗表单协同完成列表拉取、敏感字段脱敏、状态切换和数据回刷。
关键注意事项：
1. 后端只存 key 摘要：列表仅返回 `key_prefix` 展示前缀；明文只在创建响应中出现一次，必须引导用户当场保存。
2. 复制逻辑同时兼容 Clipboard API 与 `document.execCommand('copy')` 回退分支，修改时要保留安全上下文提示。
3. 当前状态切换为“先改本地值再请求后端”的乐观更新写法，请求失败时没有自动回滚，后续若重构需补一致性兜底。
静态审查建议：
1. `getTableData` 在接口异常分支没有统一 `endLoading`，后续宜改为 `try/finally` 收口加载态。
2. 启停、删除、复制等敏感操作仍分散在页面内，后续可抽成更聚焦的组合式逻辑以降低维护成本。
-->
<script setup lang="tsx">
import { computed, getCurrentInstance, reactive, ref } from 'vue'
import type { Ref } from 'vue'
import { NAlert, NButton, NPopconfirm, NSpace, NSwitch, NTag } from 'naive-ui'
import type { DataTableColumns, PaginationProps } from 'naive-ui'
import { useBoolean, useLoading } from '@aetherlink/hooks'
import { apiKeyDel, fetchKeyList, updateKey } from '@/service/api'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'
import { formatDateTime } from '@/utils/common/datetime'
import TableActionModal from './modules/table-action-modal.vue'
import type { ModalType } from './modules/table-action-modal.vue'

const { loading, startLoading, endLoading } = useLoading(false)
const { bool: visible, setTrue: openModal } = useBoolean()
type QueryFormModel = Pick<UserManagement.UserKey, 'name' | 'status'> & {
  page: number
  page_size: number
}

const queryParams = reactive<QueryFormModel>({
  name: null,
  status: null,
  page: 1,
  page_size: 10
})

const tableData = ref<UserManagement.UserKey[]>([])
const apiKeyHeaderName = 'X-API-Key'
const quickStartEndpoint = '/plugin/service/access/list'
const quickStartServiceIdentifier = '<SERVICE_IDENTIFIER>'
const maskedApiKey = '<YOUR_API_KEY>'

// 明文 key 只存在于创建响应中：保存到本地状态，用于一次性展示、复制和示例代码注入。
const justCreatedKey = ref('')
const showCreatedKeyModal = ref(false)

function handleCreatedKey(apiKey: string) {
  justCreatedKey.value = apiKey
  showCreatedKeyModal.value = true
}

function dismissCreatedKeyModal() {
  showCreatedKeyModal.value = false
  // 关闭即清空内存中的明文，之后页面不再有任何途径取回。
  justCreatedKey.value = ''
}

const apiBaseUrl = computed(() => {
  if (typeof window === 'undefined') return '/api/v1'
  return `${window.location.origin}/api/v1`
})

const swaggerDocsUrl = computed(() => {
  if (typeof window === 'undefined') return '/swagger/index.html'
  return `${window.location.origin}/swagger/index.html`
})

// 示例代码中的密钥：优先使用刚创建的一次性明文，否则显示占位符。
const visibleApiKey = computed(() => justCreatedKey.value || maskedApiKey)
const quickStartBody = computed(() => JSON.stringify({ service_identifier: quickStartServiceIdentifier }, null, 2))

const quickStartCurl = computed(
  () => `curl -X POST "${apiBaseUrl.value}${quickStartEndpoint}?page=1&page_size=10" \\
  -H "Content-Type: application/json" \\
  -H "${apiKeyHeaderName}: ${visibleApiKey.value}" \\
  -d '${quickStartBody.value}'`
)

const quickStartNode = computed(
  () => `const response = await fetch('${apiBaseUrl.value}${quickStartEndpoint}?page=1&page_size=10', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    '${apiKeyHeaderName}': '${visibleApiKey.value}'
  },
  body: JSON.stringify({ service_identifier: '${quickStartServiceIdentifier}' })
})
const data = await response.json()
if (!response.ok) {
  throw new Error(data?.message || \`Request failed with \${response.status}\`)
}
// Use data.list to render or synchronize services in your integration.`
)

const quickStartPython = computed(
  () => `import requests

response = requests.post(
    '${apiBaseUrl.value}${quickStartEndpoint}?page=1&page_size=10',
    headers={
        'Content-Type': 'application/json',
        '${apiKeyHeaderName}': '${visibleApiKey.value}',
    },
    json={'service_identifier': '${quickStartServiceIdentifier}'},
    timeout=10,
)
print(response.json())`
)

// 为每条记录保留接口返回的数据；列表不再包含明文 api_key。
function setTableData(data: UserManagement.UserKey[]) {
  tableData.value = data
}

// 列表查询主入口：统一消费分页参数并在成功后刷新表格数据与总数。
async function getTableData() {
  startLoading()
  try {
    const { data } = await fetchKeyList(queryParams)
    if (data) {
      const list: UserManagement.UserKey[] = data.list
      setTableData(list)
      pagination.itemCount = data.total || 0
    }
  } finally {
    endLoading()
  }
}

const columns: Ref<DataTableColumns<UserManagement.UserKey>> = ref([
  {
    key: 'name',
    minWidth: '100px',
    title: () => $t('page.manage.api.apiName'),
    align: 'left'
  },

  {
    key: 'api_key',
    minWidth: '100px',
    title: () => $t('page.manage.api.api_key'),
    align: 'left',
    // 后端只存摘要：列表仅展示不可还原的前缀，明文只能在创建时一次性保存。
    render: (row: any) => {
      if (row.key_prefix) {
        return <span>{`${row.key_prefix}••••••••`}</span>
      }
      return <span>********</span>
    }
  },
  {
    key: 'status',
    title: () => $t('page.manage.api.apiStatus'),
    minWidth: '140px',
    align: 'left',
    render: (row: any) => {
      if (row.status === 0) {
        return <NTag type="error">{$t('page.manage.api.apiStatus1.freeze')}</NTag>
      } else if (row.status === 1) {
        return <NTag type="success">{$t('page.manage.api.apiStatus1.normal')}</NTag>
      }
      return <span></span>
    }
  },
  {
    key: 'created_at',
    title: () => $t('page.manage.api.created_at'),
    minWidth: '130px',
    align: 'left',
    render: row => {
      return formatDateTime(row.updated_at)
    }
  },
  {
    key: 'actions',
    title: () => $t('common.actions'),
    align: 'left',
    width: '320px',
    render: (row: any) => {
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
          <NSwitch value={Boolean(row.status === 1)} onChange={() => handleSwitchChange(row.id)} />
        </NSpace>
      )
    }
  }
]) as Ref<DataTableColumns<UserManagement.UserKey>>

const modalType = ref<ModalType>('add')

function setModalType(type: ModalType) {
  modalType.value = type
}

const editData = ref<UserManagement.UserKey | null>(null)

function setEditData(data: UserManagement.UserKey | null) {
  editData.value = data
}

// 新增流程只负责切换弹窗模式，具体表单初始化交给子弹窗按 `type` 处理。
function handleAddTable() {
  openModal()
  setModalType('add')
}

// 复制属于敏感字段操作，需要兼顾安全上下文限制并给出可理解的失败提示。
async function handleCopyKey(key: string) {
  const success = await writeClipboardText(key)
  if (success) {
    window.$message?.success($t('theme.configOperation.copySuccess'))
    return
  }

  const secureContextMessage =
    window.isSecureContext === false
      ? $t('theme.configOperation.copyFailSecureFallback')
      : $t('theme.configOperation.copyFail')
  window.$message?.error(secureContextMessage)
}

async function handleCopyQuickStart(text: string) {
  const success = await writeClipboardText(text)
  window.$message?.[success ? 'success' : 'error'](
    success ? $t('theme.configOperation.copySuccess') : $t('theme.configOperation.copyFail')
  )
}

// 编辑时直接把当前行透传给子弹窗，子组件负责回填与提交模式切换。
function handleEditTable(rowId: string) {
  const findItem = tableData.value.find(item => item.id === rowId)
  if (findItem) {
    setEditData(findItem)
  }
  setModalType('edit')
  openModal()
}

// 启停开关直接复用当前行对象提交；后续若补失败回滚，需要保留切换前的旧状态快照。
async function handleSwitchChange(rowId: string) {
  const findItem = tableData.value.find(item => item.id === rowId)
  if (findItem) {
    const keyStatus = findItem.status === 1 ? 0 : 1
    findItem.status = keyStatus
    await updateKey(findItem)
  }
}

// 删除成功后重新查询整页，确保分页总数与列表内容同步刷新。
async function handleDeleteTable(rowId: string) {
  const data = await apiKeyDel(rowId)
  if (!data.error) {
    window.$message?.success($t('common.deleteSuccess'))
    getTableData()
  }
}

const pagination: PaginationProps = reactive({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
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

// 页面初始化目前只依赖首屏列表查询，后续若引入路由筛选可继续在这里汇总入口。
function init() {
  getTableData()
}
const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})
// 初始化
init()
</script>

<template>
  <div>
    <n-card>
      <div class="h-full flex-col gap-15px">
        <section class="api-quickstart">
          <div class="api-quickstart__header">
            <div>
              <div class="api-quickstart__title">{{ $t('page.manage.api.quickstart.title') }}</div>
              <div class="api-quickstart__subtitle">
                {{ $t('page.manage.api.quickstart.descPart1') }}
                <NTag size="small" type="info">{{ apiKeyHeaderName }}</NTag>
                {{ $t('page.manage.api.quickstart.descPart2') }}
                <NTag size="small">{{ quickStartServiceIdentifier }}</NTag>
                {{ $t('page.manage.api.quickstart.descPart3') }}
              </div>
            </div>
            <NButton secondary size="small" @click="handleCopyQuickStart(quickStartCurl)">
              <icon-ic-round-content-copy class="mr-4px text-16px" />
              {{ $t('page.manage.api.quickstart.copyCurl') }}
            </NButton>
            <NButton tag="a" secondary size="small" :href="swaggerDocsUrl" target="_blank" rel="noreferrer">
              <icon-ic-round-open-in-new class="mr-4px text-16px" />
              Swagger
            </NButton>
          </div>

          <div class="api-quickstart__meta">
            <div>
              <span class="api-quickstart__label">Base URL</span>
              <NTag>{{ apiBaseUrl }}</NTag>
            </div>
            <div>
              <span class="api-quickstart__label">Header</span>
              <NTag type="success">{{ apiKeyHeaderName }}: {{ visibleApiKey }}</NTag>
            </div>
            <div>
              <span class="api-quickstart__label">Probe</span>
              <NTag type="warning">{{ quickStartEndpoint }}</NTag>
            </div>
            <div>
              <span class="api-quickstart__label">Docs</span>
              <NTag>{{ swaggerDocsUrl }}</NTag>
            </div>
          </div>

          <NAlert v-if="!tableData.length" type="info" :show-icon="false" class="api-quickstart__alert">
            {{ $t('page.manage.api.quickstart.emptyHint') }}
          </NAlert>

          <div class="api-quickstart__requests">
            <div class="api-quickstart__request">
              <div class="api-quickstart__request-head">
                <span>curl</span>
                <NButton text size="small" @click="handleCopyQuickStart(quickStartCurl)">{{ $t('generate.copy') }}</NButton>
              </div>
              <pre>{{ quickStartCurl }}</pre>
            </div>
            <div class="api-quickstart__request">
              <div class="api-quickstart__request-head">
                <span>Node</span>
                <NButton text size="small" @click="handleCopyQuickStart(quickStartNode)">{{ $t('generate.copy') }}</NButton>
              </div>
              <pre>{{ quickStartNode }}</pre>
            </div>
            <div class="api-quickstart__request">
              <div class="api-quickstart__request-head">
                <span>Python</span>
                <NButton text size="small" @click="handleCopyQuickStart(quickStartPython)">{{ $t('generate.copy') }}</NButton>
              </div>
              <pre>{{ quickStartPython }}</pre>
            </div>
          </div>
        </section>

        <NSpace>
          <NButton type="primary" @click="handleAddTable">
            <icon-ic-round-plus class="mr-4px text-20px" />
            {{ $t('page.manage.api.addApiKey') }}
          </NButton>
        </NSpace>
        <NDataTable
          :columns="columns"
          :data="tableData"
          :loading="loading"
          :pagination="pagination"
          class="flex-1-hidden"
        />
        <TableActionModal
          v-model:visible="visible"
          :class="getPlatform ? 'w-90%' : 'w-500px'"
          :type="modalType"
          :edit-data="editData"
          @success="getTableData"
          @created="handleCreatedKey"
        />
        <n-modal v-model:show="showCreatedKeyModal" preset="card" :title="$t('page.manage.api.createdKeyTitle')" class="w-90%" :style="{ maxWidth: '560px' }">
          <n-alert type="warning" :show-icon="true" class="mb-12px">
            {{ $t('page.manage.api.createdKeyDesc') }}
          </n-alert>
          <n-input :value="justCreatedKey" readonly />
          <n-space justify="end" class="pt-16px" :size="16">
            <n-button @click="handleCopyKey(justCreatedKey)">{{ $t('generate.copy') }}</n-button>
            <n-button type="primary" @click="dismissCreatedKeyModal">{{ $t('common.confirm') }}</n-button>
          </n-space>
        </n-modal>
      </div>
    </n-card>
  </div>
</template>

<style scoped>
.api-quickstart {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  background: var(--n-color);
}

.api-quickstart__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.api-quickstart__title {
  font-size: 16px;
  font-weight: 600;
  line-height: 24px;
}

.api-quickstart__subtitle {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
  color: var(--n-text-color-2);
  line-height: 22px;
}

.api-quickstart__meta,
.api-quickstart__requests {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.api-quickstart__meta > div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 6px;
}

.api-quickstart__label,
.api-quickstart__request-head {
  color: var(--n-text-color-2);
  font-size: 12px;
}

.api-quickstart__alert {
  max-width: 760px;
}

.api-quickstart__request {
  min-width: 0;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  overflow: hidden;
}

.api-quickstart__request-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-bottom: 1px solid var(--n-border-color);
  background: var(--n-color-modal);
}

.api-quickstart__request pre {
  min-height: 126px;
  margin: 0;
  padding: 10px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 18px;
}

@media (max-width: 1080px) {
  .api-quickstart__meta,
  .api-quickstart__requests {
    grid-template-columns: 1fr;
  }

  .api-quickstart__header {
    flex-direction: column;
  }
}
</style>
