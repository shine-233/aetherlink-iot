<!--
文件用途：移动应用中心工作台（mobile_app_bundles，ROADMAP TB-23）——安装包版本列表/上传/发布/归档。
核心逻辑：
1. 分页检索本租户应用包（platform/status 精确过滤），表格展示版本、文件名、大小、SHA-256 校验和与发布履历；
2. 上传登记（multipart：file+platform+version+release_notes）：后端按平台校验扩展名（apk/ipa/zip）与 ZIP 签名，
   同租户同平台同版本重复登记被拒（202006），新包固定落 draft；
3. 发布状态机：publish 仅 draft→published、archive 仅 published→archived（非法流转返回 202005）；
   按钮禁用与后端 CanTransitionAppBundle 同源口径，前端禁用只是引导，后端仍 fail-closed 复核。
关键注意事项：删除仅 draft/archived（published 须先归档）；下载走 /files 静态面（file_path 经
  resolvePlatformAssetUrl 归一为平台绝对 URL，与媒体库预览同链路），带鉴权头的下载端点供 uniapp/Range 场景使用。
-->
<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NSpin,
  NTag,
  NTooltip,
  NUpload,
  useMessage
} from 'naive-ui'
import type { DataTableColumns, SelectOption, UploadCustomRequestOptions, UploadFileInfo } from 'naive-ui'
import {
  archiveMobileAppBundle,
  deleteMobileAppBundle,
  getMobileAppBundles,
  publishMobileAppBundle,
  updateMobileAppBundle,
  uploadMobileAppBundle,
  type MobileAppBundleItem,
  type MobileAppBundlePlatform,
  type MobileAppBundleStatus
} from '@/service/api'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import { resolvePlatformAssetUrl } from '@/utils/auth-user-avatar'

const message = useMessage()

const bundleList = ref<MobileAppBundleItem[]>([])
const listLoading = ref(false)
const pagination = reactive({ page: 1, pageSize: 10, itemCount: 0 })
const filterPlatform = ref<MobileAppBundlePlatform | null>(null)
const filterStatus = ref<MobileAppBundleStatus | null>(null)

const PLATFORM_OPTIONS: SelectOption[] = [
  { label: $t('page.app_bundle.platformAndroid'), value: 'android' },
  { label: $t('page.app_bundle.platformIos'), value: 'ios' },
  { label: $t('page.app_bundle.platformH5'), value: 'h5' }
]
const STATUS_OPTIONS: SelectOption[] = [
  { label: $t('page.app_bundle.statusDraft'), value: 'draft' },
  { label: $t('page.app_bundle.statusPublished'), value: 'published' },
  { label: $t('page.app_bundle.statusArchived'), value: 'archived' }
]

/** 字节数可读格式（与媒体库口径一致的局部展示 helper）。 */
const formatBytes = (size: number) => {
  if (!Number.isFinite(size) || size <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  return `${value.toFixed(value >= 100 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`
}

const statusTypeOf = (status: MobileAppBundleStatus) => {
  switch (status) {
    case 'published':
      return 'success' as const
    case 'archived':
      return 'default' as const
    default:
      return 'warning' as const
  }
}

const statusTextOf = (status: MobileAppBundleStatus) => {
  switch (status) {
    case 'published':
      return $t('page.app_bundle.statusPublished')
    case 'archived':
      return $t('page.app_bundle.statusArchived')
    default:
      return $t('page.app_bundle.statusDraft')
  }
}

const platformTextOf = (platform: MobileAppBundlePlatform) => {
  switch (platform) {
    case 'android':
      return $t('page.app_bundle.platformAndroid')
    case 'ios':
      return $t('page.app_bundle.platformIos')
    default:
      return $t('page.app_bundle.platformH5')
  }
}

const columns = computed<DataTableColumns<MobileAppBundleItem>>(() => [
  {
    title: $t('page.app_bundle.platform'),
    key: 'platform',
    width: 100,
    render: (row) => h(NTag, { size: 'small', type: 'info' }, { default: () => platformTextOf(row.platform) })
  },
  { title: $t('page.app_bundle.version'), key: 'version', width: 120, ellipsis: { tooltip: true } },
  { title: $t('page.app_bundle.fileName'), key: 'file_name', minWidth: 160, ellipsis: { tooltip: true } },
  { title: $t('page.app_bundle.size'), key: 'file_size', width: 90, render: (row) => formatBytes(row.file_size) },
  {
    title: $t('common.status'),
    key: 'status',
    width: 100,
    render: (row) =>
      h(NTag, { size: 'small', type: statusTypeOf(row.status) }, { default: () => statusTextOf(row.status) })
  },
  {
    title: $t('page.app_bundle.checksum'),
    key: 'checksum',
    width: 130,
    render: (row) =>
      h(
        NTooltip,
        {},
        {
          trigger: () => h('span', { class: 'font-mono text-12px' }, `${row.checksum.slice(0, 10)}…`),
          default: () => h('span', { class: 'break-all font-mono' }, `sha256:${row.checksum}`)
        }
      )
  },
  {
    title: $t('page.app_bundle.publishedAt'),
    key: 'published_at',
    width: 170,
    render: (row) => formatDateTime(row.published_at) || '-'
  },
  {
    title: $t('common.actions'),
    key: 'actions',
    width: 260,
    render: (row) =>
      h(
        NSpace,
        { size: 'small', wrap: false },
        {
          default: () => [
            h(
              NButton,
              {
                size: 'tiny',
                type: 'primary',
                ghost: true,
                disabled: row.status !== 'draft',
                onClick: () => handlePublish(row)
              },
              { default: () => $t('page.app_bundle.publish') }
            ),
            h(
              NButton,
              {
                size: 'tiny',
                type: 'warning',
                ghost: true,
                disabled: row.status !== 'published',
                onClick: () => handleArchive(row)
              },
              { default: () => $t('page.app_bundle.archive') }
            ),
            h(
              NButton,
              { size: 'tiny', disabled: !row.file_path, onClick: () => handleDownload(row) },
              { default: () => $t('page.app_bundle.download') }
            ),
            h(
              NButton,
              {
                size: 'tiny',
                disabled: row.status !== 'draft',
                onClick: () => openEdit(row)
              },
              { default: () => $t('common.edit') }
            ),
            h(
              NPopconfirm,
              { onPositiveClick: () => handleDelete(row) },
              {
                trigger: () =>
                  h(
                    NButton,
                    { size: 'tiny', type: 'error', ghost: true, disabled: row.status === 'published' },
                    { default: () => $t('common.delete') }
                  ),
                default: () => $t('page.app_bundle.deleteConfirm')
              }
            )
          ]
        }
      )
  }
])

const fetchBundleList = async () => {
  listLoading.value = true
  try {
    const { data, error } = await getMobileAppBundles({
      page: pagination.page,
      page_size: pagination.pageSize,
      platform: filterPlatform.value || undefined,
      status: filterStatus.value || undefined
    })
    if (!error && data) {
      bundleList.value = data.list ?? []
      pagination.itemCount = data.total ?? 0
    }
  } finally {
    listLoading.value = false
  }
}

const handleFilterChange = () => {
  pagination.page = 1
  void fetchBundleList()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  void fetchBundleList()
}

// ---- 上传登记 ----
const uploadVisible = ref(false)
const uploadSubmitting = ref(false)
const uploadFile = ref<File | null>(null)
const uploadForm = reactive({
  platform: 'android' as MobileAppBundlePlatform,
  version: '',
  releaseNotes: ''
})

const ACCEPT_BY_PLATFORM: Record<MobileAppBundlePlatform, string> = {
  android: '.apk',
  ios: '.ipa',
  h5: '.zip'
}

const openUpload = () => {
  uploadFile.value = null
  uploadForm.platform = 'android'
  uploadForm.version = ''
  uploadForm.releaseNotes = ''
  uploadVisible.value = true
}

const handleUploadFileChange = (options: { file: UploadFileInfo }) => {
  uploadFile.value = options.file?.file ?? null
}

// naive-ui 要求 custom-request 存在时才不发送原生请求；上传动作统一在提交时处理。
const noopCustomRequest = (_options: UploadCustomRequestOptions) => {
  /* 上传统一在 handleUploadSubmit 处理 */
}

const handleUploadSubmit = async () => {
  const version = uploadForm.version.trim()
  if (!version) {
    message.error($t('page.app_bundle.versionPlaceholder'))
    return
  }
  if (!uploadFile.value) {
    message.error($t('page.app_bundle.filePlaceholder'))
    return
  }
  const formData = new FormData()
  formData.append('file', uploadFile.value)
  formData.append('platform', uploadForm.platform)
  formData.append('version', version)
  if (uploadForm.releaseNotes.trim()) {
    formData.append('release_notes', uploadForm.releaseNotes.trim())
  }
  uploadSubmitting.value = true
  try {
    const { error } = await uploadMobileAppBundle(formData)
    if (!error) {
      message.success($t('page.app_bundle.uploadSuccess'))
      uploadVisible.value = false
      pagination.page = 1
      await fetchBundleList()
    }
  } finally {
    uploadSubmitting.value = false
  }
}

// ---- 发布说明更新（仅 draft）----
const editVisible = ref(false)
const editSubmitting = ref(false)
const editTarget = ref<MobileAppBundleItem | null>(null)
const editNotes = ref('')

const openEdit = (row: MobileAppBundleItem) => {
  editTarget.value = row
  editNotes.value = row.release_notes ?? ''
  editVisible.value = true
}

const handleEditSubmit = async () => {
  if (!editTarget.value) return
  editSubmitting.value = true
  try {
    const { error } = await updateMobileAppBundle(editTarget.value.id, {
      release_notes: editNotes.value.trim()
    })
    if (!error) {
      message.success($t('page.app_bundle.updateSuccess'))
      editVisible.value = false
      await fetchBundleList()
    }
  } finally {
    editSubmitting.value = false
  }
}

// ---- 状态机流转与删除 ----
const handlePublish = async (row: MobileAppBundleItem) => {
  const { error } = await publishMobileAppBundle(row.id)
  if (!error) {
    message.success($t('page.app_bundle.publishSuccess'))
    await fetchBundleList()
  }
}

const handleArchive = async (row: MobileAppBundleItem) => {
  const { error } = await archiveMobileAppBundle(row.id)
  if (!error) {
    message.success($t('page.app_bundle.archiveSuccess'))
    await fetchBundleList()
  }
}

const handleDelete = async (row: MobileAppBundleItem) => {
  const { error } = await deleteMobileAppBundle(row.id)
  if (!error) {
    message.success($t('page.app_bundle.deleteSuccess'))
    await fetchBundleList()
  }
}

/** 下载：file_path 是 /files 静态面对外路径，归一为平台绝对 URL 后触发浏览器下载。 */
const handleDownload = (row: MobileAppBundleItem) => {
  const url = resolvePlatformAssetUrl(row.file_path)
  if (!url) return
  const link = document.createElement('a')
  link.href = url
  link.download = row.file_name || `${row.platform}-${row.version}`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

onMounted(() => {
  void fetchBundleList()
})
</script>

<template>
  <div class="min-h-500px">
    <NCard :title="$t('route.mobile-app_app-center')" :bordered="false">
      <template #header-extra>
        <NSpace>
          <NSelect
            v-model:value="filterPlatform"
            :options="PLATFORM_OPTIONS"
            :placeholder="$t('page.app_bundle.platform')"
            clearable
            style="width: 140px"
            @update:value="handleFilterChange"
          />
          <NSelect
            v-model:value="filterStatus"
            :options="STATUS_OPTIONS"
            :placeholder="$t('common.status')"
            clearable
            style="width: 140px"
            @update:value="handleFilterChange"
          />
          <NButton type="primary" @click="openUpload">{{ $t('page.app_bundle.upload') }}</NButton>
          <NButton @click="fetchBundleList">{{ $t('common.refresh') }}</NButton>
        </NSpace>
      </template>

      <NSpin :show="listLoading">
        <NEmpty v-if="bundleList.length === 0" :description="$t('page.app_bundle.empty')" class="py-60px" />
        <NDataTable
          v-else
          :columns="columns"
          :data="bundleList"
          :pagination="{
            page: pagination.page,
            pageSize: pagination.pageSize,
            itemCount: pagination.itemCount,
            onChange: handlePageChange
          }"
          :bordered="false"
          size="small"
          :scroll-x="1200"
        />
      </NSpin>
    </NCard>

    <NModal v-model:show="uploadVisible" preset="card" :title="$t('page.app_bundle.uploadTitle')" class="w-560px">
      <NForm label-placement="left" label-width="100">
        <NFormItem :label="$t('page.app_bundle.platform')">
          <NSelect v-model:value="uploadForm.platform" :options="PLATFORM_OPTIONS" />
        </NFormItem>
        <NFormItem :label="$t('page.app_bundle.version')">
          <NInput
            v-model:value="uploadForm.version"
            :placeholder="$t('page.app_bundle.versionPlaceholder')"
            :maxlength="50"
          />
        </NFormItem>
        <NFormItem :label="$t('page.app_bundle.file')">
          <NUpload
            :show-file-list="false"
            :accept="ACCEPT_BY_PLATFORM[uploadForm.platform]"
            :custom-request="noopCustomRequest"
            :max="1"
            @change="handleUploadFileChange"
          >
            <NButton>{{ uploadFile ? uploadFile.name : $t('page.app_bundle.filePlaceholder') }}</NButton>
          </NUpload>
        </NFormItem>
        <NFormItem :label="$t('page.app_bundle.releaseNotes')">
          <NInput
            v-model:value="uploadForm.releaseNotes"
            type="textarea"
            :rows="4"
            :placeholder="$t('page.app_bundle.releaseNotesPlaceholder')"
            :maxlength="2000"
            show-count
          />
        </NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="uploadVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="uploadSubmitting" @click="handleUploadSubmit">
            {{ $t('common.confirm') }}
          </NButton>
        </NSpace>
      </template>
    </NModal>

    <NModal v-model:show="editVisible" preset="card" :title="$t('page.app_bundle.editTitle')" class="w-520px">
      <NInput
        v-model:value="editNotes"
        type="textarea"
        :rows="5"
        :maxlength="2000"
        show-count
        :placeholder="$t('page.app_bundle.releaseNotesPlaceholder')"
      />
      <template #footer>
        <NSpace justify="end">
          <NButton @click="editVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="editSubmitting" @click="handleEditSubmit">
            {{ $t('common.confirm') }}
          </NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>

<style scoped></style>
