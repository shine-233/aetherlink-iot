<!--
  文件用途：原生看板（native boards）列表页。
  核心逻辑：分页卡片列表（名称搜索 + SYS_ADMIN 租户过滤），支持发布/复制分享链接/编辑/删除；
  列表状态（分页、加载、过期请求丢弃、失败态）收口在 useListPage，
  创建弹窗拆到 modules/native-board-create-modal.vue（创建契约与租户必填校验在弹窗内）。
  关键注意事项：
  1. SYS_ADMIN 的列表租户过滤同时是创建租户上下文（写入 native-tenant-context）；
  2. 列表失败态 fail-closed：请求异常时保留空列表并展示加载失败文案；
  3. useListPage 的请求序号取代旧的 requestSequence 快照门：仅当出现更新的请求时旧响应被丢弃。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NCard,
  NEmpty,
  NGrid,
  NGridItem,
  NInput,
  NPagination,
  NPopconfirm,
  NSelect,
  NTag,
  NSpin,
  useMessage
} from 'naive-ui'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'
import { fetchUserList } from '@/service/api/auth'
import { getDefaultVisualizationProviderFacade } from '@/service/visualization-provider/composition'
import type { VisualizationDashboardSummary } from '@/service/visualization-provider/contracts'
import {
  readNativeBoardTenantContext,
  writeNativeBoardTenantContext
} from '@/service/visualization-provider/native-tenant-context'
import { NATIVE_BOARD_PROJECT_ID } from '@/service/visualization-provider/provider-ids'
import { useAuthStore } from '@/store/modules/auth'
import { buildThingsVisDashboardClipboardLink } from '../thingsvis-dashboards/thingsVisDashboardSharing'
import { useListPage } from '@/components/data-table-page/useListPage'
import NativeBoardCreateModal from './modules/native-board-create-modal.vue'

const PAGE_SIZE = 12
const ADMIN_ROLES = new Set(['SYS_ADMIN', 'TENANT_ADMIN'])

interface TenantOption {
  label: string
  value: string
}

const authStore = useAuthStore()
const rememberedTenantId = readNativeBoardTenantContext(authStore.userInfo)
const { routerPushByKey } = useRouterPush()
const message = useMessage()
const providerFacade = getDefaultVisualizationProviderFacade()

const searchInput = ref('')
const failed = ref(false)
const selectedTenantId = ref<string | null>(rememberedTenantId || null)
const tenantOptions = ref<TenantOption[]>([])
const loadingTenants = ref(false)
const showCreateModal = ref(false)
const deletingBoardId = ref<string | null>(null)
const publishingBoardId = ref<string | null>(null)

function hasRole(role: string) {
  if (authStore.userInfo.authority === role) return true
  return Array.isArray(authStore.userInfo.roles) && authStore.userInfo.roles.includes(role)
}

const isSysAdmin = computed(() => hasRole('SYS_ADMIN'))
const tenantFilterOptions = computed(() => [{ label: 'All tenants', value: '' }, ...tenantOptions.value])

const canCreate = computed(() => {
  const roles = new Set<string>()
  if (typeof authStore.userInfo.authority === 'string') roles.add(authStore.userInfo.authority)
  if (Array.isArray(authStore.userInfo.roles)) {
    authStore.userInfo.roles.forEach((role) => {
      if (typeof role === 'string') roles.add(role)
    })
  }
  return [...roles].some((role) => ADMIN_ROLES.has(role))
})

type QueryFormModel = {
  name: string
  tenantId: string
}

// 列表查询主入口：分页、加载态与过期请求丢弃交给 useListPage。
const {
  query: listFilter,
  rows: boards,
  total,
  loading,
  page,
  load: loadBoards,
  search: runSearch,
  setPage,
  patchQuery
} = useListPage<VisualizationDashboardSummary, QueryFormModel>({
  initialQuery: () => ({ name: '', tenantId: selectedTenantId.value?.trim() || '' }),
  initialPageSize: PAGE_SIZE,
  fetcher: async (params) => {
    failed.value = false
    try {
      const result = await providerFacade.execute((provider) =>
        provider.listDashboards({
          projectId: NATIVE_BOARD_PROJECT_ID,
          page: params.page,
          limit: params.page_size,
          ...(params.name ? { name: params.name } : {}),
          ...(params.tenantId ? { tenantId: params.tenantId } : {})
        })
      )
      if (!result.ok) {
        failed.value = true
        message.error($t('custom.nativeBoards.loadFailed'))
        return null
      }
      return { list: result.data.items, total: result.data.total }
    } catch {
      failed.value = true
      message.error($t('custom.nativeBoards.loadFailed'))
      return null
    }
  }
})

function handleSearch() {
  listFilter.name = searchInput.value.trim()
  void runSearch()
}

function handlePageChange(nextPage: number) {
  void setPage(nextPage)
}

function handleTenantChange() {
  if (isSysAdmin.value) {
    // The list filter is the active tenant context for SYS_ADMIN. Reuse it
    // when opening the create flow so the POST cannot lose tenant_id between
    // the list page and the modal.
    writeNativeBoardTenantContext(authStore.userInfo, selectedTenantId.value)
    patchQuery({ tenantId: selectedTenantId.value?.trim() || '' })
    return
  }
  void runSearch()
}

function openBoard(id: string) {
  routerPushByKey('visualization_native-board', { query: { id } })
}

function editBoard(id: string) {
  if (!canCreate.value) return
  routerPushByKey('visualization_native-board-editor', { query: { id } })
}

// 打开创建弹窗：把当前列表租户过滤作为创建租户 prefill 注入。
const createModalPrefill = ref<string | null>(null)

function openCreateModal() {
  if (!canCreate.value) return
  if (isSysAdmin.value) {
    if (selectedTenantId.value?.trim()) {
      createModalPrefill.value = selectedTenantId.value.trim()
    } else if (tenantOptions.value.length === 1) {
      createModalPrefill.value = tenantOptions.value[0].value
    } else {
      createModalPrefill.value = null
    }
  }
  showCreateModal.value = true
}

function handleCreated(boardId: string) {
  routerPushByKey('visualization_native-board', { query: { id: boardId } })
}

async function handleDelete(id: string) {
  if (!canCreate.value || deletingBoardId.value) return

  deletingBoardId.value = id
  try {
    const result = await providerFacade.execute((provider) => provider.deleteDashboard(id))
    if (!result.ok) {
      message.error($t('common.deleteFailed'))
      return
    }

    message.success($t('common.deleteSuccess'))
    // 删掉当前页最后一条时回退一页（与旧实现一致）；useListPage 的空页回退仅兜底非预判场景。
    if (boards.value.length === 1 && page.value > 1) {
      void setPage(page.value - 1)
    } else {
      await loadBoards()
    }
  } catch {
    message.error($t('common.deleteFailed'))
  } finally {
    deletingBoardId.value = null
  }
}

async function handlePublish(id: string) {
  if (!canCreate.value || publishingBoardId.value) return
  const board = boards.value.find((item) => item.id === id)
  if (!board || board.published) return

  publishingBoardId.value = id
  try {
    const result = await providerFacade.execute((provider) => provider.publishDashboard(id))
    if (!result.ok) {
      message.error($t('rdi.thingsvis.publishFailed'))
      return
    }

    message.success($t('rdi.thingsvis.publishSuccess', { name: board.name }))
    await loadBoards()
  } catch {
    message.error($t('rdi.thingsvis.publishFailed'))
  } finally {
    publishingBoardId.value = null
  }
}

async function handleCopyLink(board: VisualizationDashboardSummary) {
  if (!board.shareToken) return
  const copied = await writeClipboardText(buildThingsVisDashboardClipboardLink(board))
  if (copied) {
    message.success($t('rdi.thingsvis.copyLinkSuccess'))
  } else {
    message.error($t('rdi.thingsvis.copyLinkFailed'))
  }
}

async function loadTenantOptions() {
  if (!isSysAdmin.value) return
  loadingTenants.value = true
  try {
    const response = await fetchUserList({ page: 1, page_size: 1000 })
    const rows = response?.data?.list ?? []
    const seen = new Set<string>()
    tenantOptions.value = rows.flatMap((row) => {
      const tenantId = String(row.tenant_id ?? '').trim()
      if (!tenantId || seen.has(tenantId) || (row.authority && row.authority !== 'TENANT_ADMIN')) return []
      seen.add(tenantId)
      const name = String(row.name ?? row.email ?? tenantId).trim()
      return [{ label: `${name} (${tenantId})`, value: tenantId }]
    })
    const remembered = readNativeBoardTenantContext(authStore.userInfo)
    if (remembered && seen.has(remembered)) {
      selectedTenantId.value = remembered
    } else if (remembered) {
      writeNativeBoardTenantContext(authStore.userInfo, null)
      selectedTenantId.value = null
      listFilter.tenantId = ''
    }
    if (!selectedTenantId.value && tenantOptions.value.length === 1) {
      selectedTenantId.value = tenantOptions.value[0].value
      listFilter.tenantId = tenantOptions.value[0].value
    }
  } catch {
    tenantOptions.value = []
    message.error($t('custom.nativeBoards.loadFailed'))
  } finally {
    loadingTenants.value = false
  }
}

onMounted(() => {
  void loadTenantOptions()
  void loadBoards()
})
</script>

<template>
  <div class="h-full">
    <NCard>
      <div class="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-xl font-bold">{{ $t('custom.nativeBoards.title') }}</h2>
          <div class="mt-1 text-sm text-gray-400">{{ $t('custom.nativeBoards.subtitle') }}</div>
        </div>
        <div class="flex items-center gap-3">
          <NSelect
            v-if="isSysAdmin"
            v-model:value="selectedTenantId"
            :options="tenantFilterOptions"
            :loading="loadingTenants"
            clearable
            filterable
            placeholder="All tenants"
            style="width: 220px"
            data-testid="native-board-tenant-filter"
            @update:value="handleTenantChange"
          />
          <NInput
            v-model:value="searchInput"
            clearable
            :placeholder="$t('custom.nativeBoards.searchPlaceholder')"
            data-testid="native-board-search"
            @keyup.enter="handleSearch"
          />
          <NButton data-testid="native-board-search-button" @click="handleSearch">
            {{ $t('custom.nativeBoards.search') }}
          </NButton>
          <NButton v-if="canCreate" type="primary" data-testid="native-board-create-button" @click="openCreateModal">
            {{ $t('custom.nativeBoards.create') }}
          </NButton>
        </div>
      </div>

      <NSpin :show="loading">
        <NEmpty
          v-if="!loading && boards.length === 0"
          :description="$t(failed ? 'custom.nativeBoards.loadFailed' : 'custom.nativeBoards.empty')"
          class="py-20"
        />
        <NGrid v-else cols="1 s:2 m:3 l:4" responsive="screen" x-gap="16" y-gap="16" data-testid="native-board-list">
          <NGridItem v-for="board in boards" :key="board.id">
            <NCard hoverable class="cursor-pointer" data-testid="native-board-item" @click="openBoard(board.id)">
              <div class="text-base font-semibold">{{ board.name }}</div>
              <div class="mt-2 min-h-10 text-sm text-gray-500">
                {{ board.description || $t('custom.nativeBoards.noDescription') }}
              </div>
              <div class="mt-4 text-xs text-gray-400">
                {{ $t('custom.nativeBoards.updatedAt') }}: {{ board.updatedAt }}
              </div>
              <div class="mt-1 text-xs text-gray-400">
                {{ $t('custom.nativeBoards.home') }}:
                {{ board.home ? $t('custom.nativeBoards.yes') : $t('custom.nativeBoards.no') }}
              </div>
              <div v-if="isSysAdmin && board.tenantId" class="mt-1 text-xs text-gray-400">
                Tenant: {{ board.tenantId }}
              </div>
              <div class="mt-4 flex flex-wrap items-center justify-end gap-2" @click.stop>
                <NTag v-if="board.published" size="small" type="success">
                  {{ $t('rdi.thingsvis.published') }}
                </NTag>
                <NButton
                  v-if="canCreate"
                  size="small"
                  type="primary"
                  :loading="publishingBoardId === board.id"
                  :disabled="board.published || Boolean(publishingBoardId)"
                  data-testid="native-board-publish-button"
                  @click.stop="handlePublish(board.id)"
                >
                  {{ $t('rdi.thingsvis.publish') }}
                </NButton>
                <NButton
                  v-if="board.shareToken"
                  size="small"
                  :disabled="Boolean(publishingBoardId)"
                  data-testid="native-board-copy-link-button"
                  @click.stop="handleCopyLink(board)"
                >
                  {{ $t('rdi.thingsvis.copyLink') }}
                </NButton>
              </div>
              <div v-if="canCreate" class="mt-2 flex justify-end gap-2" @click.stop>
                <NButton size="small" data-testid="native-board-edit-button" @click.stop="editBoard(board.id)">
                  {{ $t('custom.nativeBoards.edit') }}
                </NButton>
                <NPopconfirm :disabled="Boolean(deletingBoardId)" @positive-click="handleDelete(board.id)">
                  <template #trigger>
                    <NButton
                      size="small"
                      type="error"
                      :loading="deletingBoardId === board.id"
                      :disabled="Boolean(deletingBoardId)"
                      data-testid="native-board-delete-button"
                      @click.stop
                    >
                      Delete
                    </NButton>
                  </template>
                  {{ $t('common.confirmDelete') }}
                </NPopconfirm>
              </div>
            </NCard>
          </NGridItem>
        </NGrid>
      </NSpin>

      <div v-if="total > PAGE_SIZE" class="mt-5 flex justify-end">
        <NPagination
          :page="page"
          :page-size="PAGE_SIZE"
          :item-count="total"
          data-testid="native-board-pagination"
          @update:page="handlePageChange"
        />
      </div>
    </NCard>

    <NativeBoardCreateModal
      v-model:show="showCreateModal"
      :is-sys-admin="isSysAdmin"
      :tenant-options="tenantOptions"
      :loading-tenants="loadingTenants"
      :prefill-tenant-id="createModalPrefill"
      @created="handleCreated"
    />
  </div>
</template>
