<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NCard,
  NEmpty,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NInput,
  NModal,
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

const PAGE_SIZE = 12
const NATIVE_BOARD_CONFIG = { version: 1, columns: 24, rowHeight: 60, widgets: [] }
const ADMIN_ROLES = new Set(['SYS_ADMIN', 'TENANT_ADMIN'])

interface QuerySnapshot {
  page: number
  pageSize: number
  name: string
  tenantId: string
}

interface TenantOption {
  label: string
  value: string
}

const authStore = useAuthStore()
const rememberedTenantId = readNativeBoardTenantContext(authStore.userInfo)
const { routerPushByKey } = useRouterPush()
const message = useMessage()
const providerFacade = getDefaultVisualizationProviderFacade()
const boards = ref<VisualizationDashboardSummary[]>([])
const total = ref(0)
const loading = ref(false)
const failed = ref(false)
const page = ref(1)
const searchInput = ref('')
const nameFilter = ref('')
const selectedTenantId = ref<string | null>(rememberedTenantId || null)
const createTenantId = ref<string | null>(rememberedTenantId || null)
const tenantOptions = ref<TenantOption[]>([])
const loadingTenants = ref(false)
const showCreateModal = ref(false)
const creating = ref(false)
const deletingBoardId = ref<string | null>(null)
const publishingBoardId = ref<string | null>(null)
const createForm = reactive({ name: '', description: '' })
let requestSequence = 0

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

function currentQuery(): QuerySnapshot {
  return {
    page: page.value,
    pageSize: PAGE_SIZE,
    name: nameFilter.value,
    tenantId: selectedTenantId.value?.trim() || ''
  }
}

function isCurrentRequest(sequence: number, snapshot: QuerySnapshot) {
  const current = currentQuery()
  return (
    sequence === requestSequence &&
    snapshot.page === current.page &&
    snapshot.pageSize === current.pageSize &&
    snapshot.name === current.name &&
    snapshot.tenantId === current.tenantId
  )
}

async function loadTenantOptions() {
  if (!isSysAdmin.value) return
  loadingTenants.value = true
  try {
    const response = await fetchUserList({ page: 1, page_size: 1000 })
    const rows = response?.data?.list ?? []
    const seen = new Set<string>()
    tenantOptions.value = rows.flatMap(row => {
      const tenantId = String(row.tenant_id ?? '').trim()
      if (!tenantId || seen.has(tenantId) || (row.authority && row.authority !== 'TENANT_ADMIN')) return []
      seen.add(tenantId)
      const name = String(row.name ?? row.email ?? tenantId).trim()
      return [{ label: `${name} (${tenantId})`, value: tenantId }]
    })
    const remembered = readNativeBoardTenantContext(authStore.userInfo)
    if (remembered && seen.has(remembered)) {
      selectedTenantId.value = remembered
      createTenantId.value = remembered
    } else if (remembered) {
      writeNativeBoardTenantContext(authStore.userInfo, null)
      selectedTenantId.value = null
      createTenantId.value = null
    }
    if (createTenantId.value && !seen.has(createTenantId.value)) createTenantId.value = null
    if (!createTenantId.value && tenantOptions.value.length === 1) createTenantId.value = tenantOptions.value[0].value
  } catch {
    tenantOptions.value = []
    message.error($t('custom.nativeBoards.loadFailed'))
  } finally {
    loadingTenants.value = false
  }
}

async function loadBoards() {
  const snapshot = currentQuery()
  const sequence = ++requestSequence
  boards.value = []
  total.value = 0
  failed.value = false
  loading.value = true

  try {
    const result = await providerFacade.execute(provider =>
      provider.listDashboards({
        projectId: NATIVE_BOARD_PROJECT_ID,
        page: snapshot.page,
        limit: snapshot.pageSize,
        ...(snapshot.name ? { name: snapshot.name } : {}),
        ...(snapshot.tenantId ? { tenantId: snapshot.tenantId } : {})
      })
    )
    if (!isCurrentRequest(sequence, snapshot)) return
    if (!result.ok) {
      failed.value = true
      message.error($t('custom.nativeBoards.loadFailed'))
      return
    }
    boards.value = result.data.items
    total.value = result.data.total
  } catch {
    if (!isCurrentRequest(sequence, snapshot)) return
    failed.value = true
    message.error($t('custom.nativeBoards.loadFailed'))
  } finally {
    if (isCurrentRequest(sequence, snapshot)) loading.value = false
  }
}

function handleSearch() {
  nameFilter.value = searchInput.value.trim()
  page.value = 1
  void loadBoards()
}

function handlePageChange(nextPage: number) {
  page.value = nextPage
  void loadBoards()
}

function handleTenantChange() {
  if (isSysAdmin.value) {
    // The list filter is the active tenant context for SYS_ADMIN. Reuse it
    // when opening the create flow so the POST cannot lose tenant_id between
    // the list page and the modal.
    createTenantId.value = selectedTenantId.value?.trim() || null
    writeNativeBoardTenantContext(authStore.userInfo, selectedTenantId.value)
  }
  page.value = 1
  void loadBoards()
}

function openBoard(id: string) {
  routerPushByKey('visualization_native-board', { query: { id } })
}

function editBoard(id: string) {
  if (!canCreate.value) return
  routerPushByKey('visualization_native-board-editor', { query: { id } })
}

function openCreateModal() {
  if (!canCreate.value) return
  createForm.name = ''
  createForm.description = ''
  if (isSysAdmin.value) {
    if (selectedTenantId.value?.trim()) {
      createTenantId.value = selectedTenantId.value.trim()
    } else if (!createTenantId.value && tenantOptions.value.length === 1) {
      createTenantId.value = tenantOptions.value[0].value
    }
  }
  showCreateModal.value = true
}

async function handleCreate() {
  if (!canCreate.value || creating.value) return
  const name = createForm.name.trim()
  if (!name || name.length > 255) {
    message.error($t('custom.nativeBoards.nameInvalid'))
    return
  }
  if (createForm.description.length > 500) {
    message.error($t('custom.nativeBoards.descriptionInvalid'))
    return
  }
  const tenantId = createTenantId.value?.trim() || ''
  if (isSysAdmin.value && !tenantId) {
    message.error('Select a tenant before creating a native board')
    return
  }

  creating.value = true
  try {
    const result = await providerFacade.execute(provider =>
      provider.createDashboard({
        name,
        description: createForm.description,
        projectId: NATIVE_BOARD_PROJECT_ID,
        rendererData: NATIVE_BOARD_CONFIG,
        ...(tenantId ? { tenantId } : {})
      })
    )
    if (!result.ok || !result.data.id.trim()) {
      message.error($t('custom.nativeBoards.createFailed'))
      return
    }
    message.success($t('custom.nativeBoards.createSuccess'))
    showCreateModal.value = false
    routerPushByKey('visualization_native-board', { query: { id: result.data.id } })
  } catch {
    message.error($t('custom.nativeBoards.createFailed'))
  } finally {
    creating.value = false
  }
}

async function handleDelete(id: string) {
  if (!canCreate.value || deletingBoardId.value) return

  deletingBoardId.value = id
  try {
    const result = await providerFacade.execute(provider => provider.deleteDashboard(id))
    if (!result.ok) {
      message.error($t('common.deleteFailed'))
      return
    }

    message.success($t('common.deleteSuccess'))
    if (boards.value.length === 1 && page.value > 1) page.value -= 1
    await loadBoards()
  } catch {
    message.error($t('common.deleteFailed'))
  } finally {
    deletingBoardId.value = null
  }
}

async function handlePublish(id: string) {
  if (!canCreate.value || publishingBoardId.value) return
  const board = boards.value.find(item => item.id === id)
  if (!board || board.published) return

  publishingBoardId.value = id
  try {
    const result = await providerFacade.execute(provider => provider.publishDashboard(id))
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

onMounted(() => {
  void loadTenantOptions()
  void loadBoards()
})
onBeforeUnmount(() => {
  requestSequence += 1
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
                <NButton
                  size="small"
                  data-testid="native-board-edit-button"
                  @click.stop="editBoard(board.id)"
                >
                  {{ $t('custom.nativeBoards.edit') }}
                </NButton>
                <NPopconfirm
                  :disabled="Boolean(deletingBoardId)"
                  @positive-click="handleDelete(board.id)"
                >
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

    <NModal
      v-model:show="showCreateModal"
      preset="card"
      :title="$t('custom.nativeBoards.createTitle')"
      class="w-500px"
      data-testid="native-board-create-modal"
    >
      <NForm :model="createForm">
        <NFormItem v-if="isSysAdmin" label="Tenant" path="tenantId">
          <NSelect
            v-model:value="createTenantId"
            :options="tenantOptions"
            :loading="loadingTenants"
            filterable
            placeholder="Select tenant"
            data-testid="native-board-tenant-select"
          />
        </NFormItem>
        <NFormItem :label="$t('custom.nativeBoards.name')" path="name">
          <NInput
            v-model:value="createForm.name"
            :maxlength="255"
            show-count
            :placeholder="$t('custom.nativeBoards.namePlaceholder')"
            data-testid="native-board-name"
          />
        </NFormItem>
        <NFormItem :label="$t('custom.nativeBoards.description')" path="description">
          <NInput
            v-model:value="createForm.description"
            type="textarea"
            :maxlength="500"
            show-count
            :placeholder="$t('custom.nativeBoards.descriptionPlaceholder')"
            data-testid="native-board-description"
          />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="flex justify-end gap-2">
          <NButton :disabled="creating" @click="showCreateModal = false">
            {{ $t('custom.nativeBoards.cancel') }}
          </NButton>
          <NButton type="primary" :loading="creating" data-testid="native-board-submit" @click="handleCreate">
            {{ $t('custom.nativeBoards.submit') }}
          </NButton>
        </div>
      </template>
    </NModal>
  </div>
</template>
