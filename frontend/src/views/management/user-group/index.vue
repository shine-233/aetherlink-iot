<!--
文件用途：用户组与组权限管理页（TB-46 GPE v1，/management/user-group）——
租户内用户组的增删改查、成员管理与组权限元素（board:/asset: 资源元素）绑定。
核心逻辑：
1. 列表走 GET /user_groups（分页 + 名称模糊过滤）；组 CRUD 走 /user_group 三端点；
2. 成员面板：GET/POST /user_group/:id/users，全量替换语义（保存时提交完整勾选集）；
   候选人来自既有 GET /user/selector（同租户账号，customer 客户不会出现）；
3. 组权限面板：GET/POST /user_group/:id/permissions，元素码 v1 仅支持 board:<id>/asset:<id>，
   由 kind 选择 + 资源 ID 输入拼装，后端对资源存在性与同租户归属 fail-closed 校验；
4. 组共享语义：绑定到组的看板/资产对组外成员默认不可见（fail-closed），管理员不受限。
关键注意事项：成员与权限保存都是全量替换——必须以当前草稿全集提交，禁止增量提交；
SYS_ADMIN 建组必须显式提供目标租户（tenant_id），TENANT_ADMIN 由后端强制本租户。
重构建议：若组权限后续扩展功能权限点命名空间，把元素码编辑抽成独立组件并支持资源选择器。
-->
<script setup lang="tsx">
import { computed, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NCard,
  NDrawer,
  NDrawerContent,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NTag,
  useMessage
} from 'naive-ui'
import type { DataTableColumns, PaginationProps, SelectOption } from 'naive-ui'
import { useAuthStore } from '@/store/modules/auth'
import { useLoading } from '@aetherlink/hooks'
import {
  assignUserGroupMembers,
  assignUserGroupPermissions,
  createUserGroup,
  deleteUserGroup,
  getUserGroupList,
  getUserGroupMembers,
  getUserGroupPermissions,
  updateUserGroup
} from '@/service/api'
import type { CreateUserGroupParams, UpdateUserGroupParams, UserGroupElementInfo, UserGroupItem, UserGroupMemberSummary } from '@/service/api'
import { getUserList } from '@/service/api/notification'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'

defineOptions({ name: 'ManagementUserGroup' })

const message = useMessage()
const authStore = useAuthStore()

const isSysAdmin = computed(() => authStore.userInfo?.authority === 'SYS_ADMIN')

const { loading, startLoading, endLoading } = useLoading(false)
const { loading: saving, startLoading: startSaving, endLoading: endSaving } = useLoading(false)

const searchName = ref('')
const tableData = ref<UserGroupItem[]>([])

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

const queryParams = reactive({ page: 1, page_size: 10 })

async function getTableData() {
  startLoading()
  try {
    const params: Record<string, unknown> = { ...queryParams }
    if (searchName.value.trim()) {
      params.name = searchName.value.trim()
    }
    const { data, error } = await getUserGroupList(params)
    if (!error && data) {
      tableData.value = data.list ?? []
      pagination.itemCount = data.total || 0
    } else {
      message.error($t('page.user_group.loadFailed'))
    }
  } finally {
    endLoading()
  }
}

function handleSearch() {
  pagination.page = 1
  queryParams.page = 1
  getTableData()
}

// ---- 组创建/编辑弹窗 ----
type ModalType = 'add' | 'edit'
const modalVisible = ref(false)
const modalType = ref<ModalType>('add')
const editTarget = ref<UserGroupItem | null>(null)
const groupForm = reactive({ name: '', description: '', tenant_id: '' })

function openAddModal() {
  modalType.value = 'add'
  editTarget.value = null
  groupForm.name = ''
  groupForm.description = ''
  groupForm.tenant_id = ''
  modalVisible.value = true
}

function openEditModal(row: UserGroupItem) {
  modalType.value = 'edit'
  editTarget.value = row
  groupForm.name = row.name
  groupForm.description = row.description ?? ''
  groupForm.tenant_id = row.tenant_id
  modalVisible.value = true
}

async function handleSaveGroup() {
  if (!groupForm.name.trim()) {
    message.error($t('page.user_group.namePlaceholder'))
    return
  }
  startSaving()
  try {
    if (modalType.value === 'add') {
      const payload: CreateUserGroupParams = { name: groupForm.name.trim() }
      if (groupForm.description.trim()) payload.description = groupForm.description.trim()
      if (isSysAdmin.value && groupForm.tenant_id.trim()) {
        payload.tenant_id = groupForm.tenant_id.trim()
      }
      const { error } = await createUserGroup(payload)
      if (error) return
      message.success($t('page.user_group.groupSaved'))
    } else if (editTarget.value) {
      const payload: UpdateUserGroupParams = { id: editTarget.value.id, name: groupForm.name.trim() }
      if (groupForm.description.trim()) payload.description = groupForm.description.trim()
      const { error } = await updateUserGroup(payload)
      if (error) return
      message.success($t('page.user_group.groupSaved'))
    }
    modalVisible.value = false
    getTableData()
  } finally {
    endSaving()
  }
}

async function handleDeleteGroup(rowId: string) {
  const { error } = await deleteUserGroup(rowId)
  if (!error) {
    message.success($t('common.deleteSuccess'))
    getTableData()
  }
}

// ---- 成员面板（全量替换） ----
const membersVisible = ref(false)
const membersTarget = ref<UserGroupItem | null>(null)
const currentMembers = ref<UserGroupMemberSummary[]>([])
const draftMemberIds = ref<string[]>([])
const memberOptions = ref<SelectOption[]>([])
const memberSearchLoading = ref(false)

function memberLabel(id: string, name?: string | null, email?: string | null): string {
  return name || email || id
}

function syncMemberOptions() {
  // 已入选成员固定进 options，避免远程搜索分页丢掉已选人。
  const optionMap = new Map<string, SelectOption>()
  for (const opt of memberOptions.value) {
    optionMap.set(String(opt.value), opt)
  }
  for (const member of currentMembers.value) {
    optionMap.set(member.id, {
      label: memberLabel(member.id, member.name, member.email),
      value: member.id
    })
  }
  memberOptions.value = Array.from(optionMap.values())
}

async function openMembersDrawer(row: UserGroupItem) {
  membersTarget.value = row
  draftMemberIds.value = []
  currentMembers.value = []
  memberOptions.value = []
  membersVisible.value = true
  const { data, error } = await getUserGroupMembers(row.id)
  if (error || !data) {
    message.error($t('page.user_group.loadFailed'))
    return
  }
  currentMembers.value = data.users ?? []
  draftMemberIds.value = currentMembers.value.map((item) => item.id)
  syncMemberOptions()
}

async function searchMemberOptions(query = '') {
  memberSearchLoading.value = true
  try {
    const { data } = await getUserList({ page: 1, page_size: 20, name: query || undefined })
    const list =
      (data as { list?: { user_id?: string; name?: string; email?: string }[] } | undefined)?.list ??
      []
    memberOptions.value = list
      .filter((item): item is { user_id: string; name?: string; email?: string } =>
        Boolean(item?.user_id)
      )
      .map((item) => ({ label: item.name || item.email || item.user_id, value: item.user_id }))
    syncMemberOptions()
  } finally {
    memberSearchLoading.value = false
  }
}

async function handleSaveMembers() {
  if (!membersTarget.value) return
  startSaving()
  try {
    const { error } = await assignUserGroupMembers(membersTarget.value.id, draftMemberIds.value)
    if (error) return
    message.success($t('page.user_group.membersSaved'))
    membersVisible.value = false
  } finally {
    endSaving()
  }
}

// ---- 组权限面板（全量替换） ----
const permissionsVisible = ref(false)
const permissionsTarget = ref<UserGroupItem | null>(null)
const currentElements = ref<UserGroupElementInfo[]>([])
const draftElementCodes = ref<string[]>([])
const draftElementKind = ref<'board' | 'asset'>('board')
const draftElementResourceId = ref('')

const elementKindOptions: SelectOption[] = [
  { label: 'board', value: 'board' },
  { label: 'asset', value: 'asset' }
]

function composeElementCode(): string {
  return `${draftElementKind.value}:${draftElementResourceId.value.trim()}`
}

function parseElementKind(code: string): 'board' | 'asset' | null {
  const idx = code.indexOf(':')
  if (idx <= 0 || idx === code.length - 1) return null
  const kind = code.slice(0, idx)
  if (kind !== 'board' && kind !== 'asset') return null
  return kind
}

function openPermissionsDrawer(row: UserGroupItem) {
  permissionsTarget.value = row
  currentElements.value = []
  draftElementCodes.value = []
  draftElementKind.value = 'board'
  draftElementResourceId.value = ''
  permissionsVisible.value = true
  void loadGroupPermissions(row.id)
}

async function loadGroupPermissions(rowId: string) {
  const { data, error } = await getUserGroupPermissions(rowId)
  if (error || !data) {
    message.error($t('page.user_group.loadFailed'))
    return
  }
  currentElements.value = data.elements ?? []
  draftElementCodes.value = currentElements.value.map((item) => item.code)
}

function addDraftElement() {
  const code = composeElementCode()
  if (parseElementKind(code) === null) {
    message.error($t('page.user_group.invalidElementCode'))
    return
  }
  if (draftElementCodes.value.includes(code)) {
    message.warning($t('page.user_group.duplicateElementCode'))
    return
  }
  draftElementCodes.value = [...draftElementCodes.value, code]
  draftElementResourceId.value = ''
}

function removeDraftElement(code: string) {
  draftElementCodes.value = draftElementCodes.value.filter((item) => item !== code)
}

function elementDisplayName(code: string): string {
  const found = currentElements.value.find((item) => item.code === code)
  if (found && found.name) return `${found.name} (${code})`
  return code
}

async function handleSavePermissions() {
  if (!permissionsTarget.value) return
  startSaving()
  try {
    const { error } = await assignUserGroupPermissions(
      permissionsTarget.value.id,
      draftElementCodes.value
    )
    if (error) return
    message.success($t('page.user_group.permissionsSaved'))
    permissionsVisible.value = false
  } finally {
    endSaving()
  }
}

// ---- 表格列 ----
const columns = ref<DataTableColumns<UserGroupItem>>([
  {
    key: 'name',
    title: $t('page.user_group.name'),
    minWidth: '140px',
    align: 'left'
  },
  {
    key: 'description',
    title: $t('page.user_group.description'),
    minWidth: '180px',
    align: 'left',
    render: (row) => row.description || '-'
  },
  {
    key: 'created_at',
    title: $t('page.user_group.createdAt'),
    minWidth: '150px',
    align: 'left',
    render: (row) => formatDateTime(row.created_at)
  },
  {
    key: 'actions',
    title: $t('common.actions'),
    align: 'left',
    width: '360px',
    render: (row) => (
      <NSpace justify={'start'}>
        <NButton type="primary" size={'small'} onClick={() => openEditModal(row)}>
          {$t('common.edit')}
        </NButton>
        <NButton type="primary" size={'small'} onClick={() => openMembersDrawer(row)}>
          {$t('page.user_group.manageMembers')}
        </NButton>
        <NButton type="primary" size={'small'} onClick={() => openPermissionsDrawer(row)}>
          {$t('page.user_group.managePermissions')}
        </NButton>
        <NPopconfirm onPositiveClick={() => handleDeleteGroup(row.id)}>
          {{
            default: () => $t('page.user_group.deleteConfirm'),
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
])

onMounted(() => {
  void getTableData()
})
</script>

<template>
  <div class="min-h-500px">
    <NCard :title="$t('route.management_user-group')" :bordered="false">
      <p class="mb-16px text-gray-400">{{ $t('page.user_group.sharedScopeHint') }}</p>

      <NSpace class="mb-16px" justify="space-between">
        <NSpace>
          <NInput
            v-model:value="searchName"
            :placeholder="$t('page.user_group.namePlaceholder')"
            clearable
            class="w-240px"
            @keyup.enter="handleSearch"
          />
          <NButton type="primary" @click="handleSearch">{{ $t('common.search') }}</NButton>
        </NSpace>
        <NButton type="primary" @click="openAddModal">
          {{ $t('page.user_group.addGroup') }}
        </NButton>
      </NSpace>

      <NDataTable
        :columns="columns"
        :data="tableData"
        :loading="loading"
        :pagination="pagination"
        remote
      >
        <template #empty>
          <NEmpty :description="$t('common.noData')" class="py-24px" />
        </template>
      </NDataTable>
    </NCard>

    <!-- 组创建/编辑弹窗 -->
    <NModal
      v-model:show="modalVisible"
      preset="card"
      :title="modalType === 'add' ? $t('page.user_group.addGroup') : $t('page.user_group.editGroup')"
      class="w-520px"
    >
      <NForm label-placement="left" :label-width="100">
        <NFormItem :label="$t('page.user_group.name')" required>
          <NInput v-model:value="groupForm.name" :placeholder="$t('page.user_group.namePlaceholder')" />
        </NFormItem>
        <NFormItem :label="$t('page.user_group.description')">
          <NInput
            v-model:value="groupForm.description"
            type="textarea"
            :rows="3"
            :placeholder="$t('page.user_group.descriptionPlaceholder')"
          />
        </NFormItem>
        <NFormItem v-if="modalType === 'add' && isSysAdmin" :label="$t('page.user_group.sysTenantId')">
          <NInput v-model:value="groupForm.tenant_id" placeholder="tenant id" />
        </NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="modalVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="saving" @click="handleSaveGroup">
            {{ $t('common.confirm') }}
          </NButton>
        </NSpace>
      </template>
    </NModal>

    <!-- 成员管理抽屉 -->
    <NDrawer v-model:show="membersVisible" :width="520">
      <NDrawerContent :title="`${$t('page.user_group.members')} - ${membersTarget?.name ?? ''}`" closable>
        <p class="mb-12px text-gray-400">{{ $t('page.user_group.selectMembersHint') }}</p>
        <NSelect
          v-model:value="draftMemberIds"
          multiple
          filterable
          remote
          clearable
          :options="memberOptions"
          :loading="memberSearchLoading"
          :max-tag-count="5"
          :placeholder="$t('page.user_group.selectMembersHint')"
          @search="(query: string) => searchMemberOptions(query)"
        />
        <div v-if="currentMembers.length" class="mt-16px">
          <NTag
            v-for="member in currentMembers"
            :key="member.id"
            size="small"
            class="mr-6px mb-6px"
          >
            {{ memberLabel(member.id, member.name, member.email) }}
          </NTag>
        </div>
        <NEmpty v-else :description="$t('page.user_group.noMembers')" class="mt-16px" />
        <template #footer>
          <NSpace justify="end">
            <NButton @click="membersVisible = false">{{ $t('common.cancel') }}</NButton>
            <NButton type="primary" :loading="saving" @click="handleSaveMembers">
              {{ $t('common.save') }}
            </NButton>
          </NSpace>
        </template>
      </NDrawerContent>
    </NDrawer>

    <!-- 组权限元素绑定抽屉 -->
    <NDrawer v-model:show="permissionsVisible" :width="560">
      <NDrawerContent
        :title="`${$t('page.user_group.permissions')} - ${permissionsTarget?.name ?? ''}`"
        closable
      >
        <NSpace class="mb-12px" align="center">
          <NSelect
            v-model:value="draftElementKind"
            :options="elementKindOptions"
            class="w-120px"
            size="small"
          />
          <span class="text-gray-500">:</span>
          <NInput
            v-model:value="draftElementResourceId"
            size="small"
            class="w-260px"
            :placeholder="$t('page.user_group.elementResourceIdPlaceholder')"
            @keyup.enter="addDraftElement"
          />
          <NButton size="small" type="primary" @click="addDraftElement">
            {{ $t('page.user_group.addElement') }}
          </NButton>
        </NSpace>
        <p class="mb-8px text-gray-400">{{ $t('page.user_group.elementCodeLabel') }}</p>
        <div v-if="draftElementCodes.length">
          <NTag
            v-for="code in draftElementCodes"
            :key="code"
            size="small"
            closable
            class="mr-6px mb-6px"
            @close="removeDraftElement(code)"
          >
            {{ elementDisplayName(code) }}
          </NTag>
        </div>
        <NEmpty v-else :description="$t('page.user_group.noPermissions')" class="mt-8px" />
        <template #footer>
          <NSpace justify="end">
            <NButton @click="permissionsVisible = false">{{ $t('common.cancel') }}</NButton>
            <NButton type="primary" :loading="saving" @click="handleSavePermissions">
              {{ $t('common.save') }}
            </NButton>
          </NSpace>
        </template>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>

<style scoped></style>
