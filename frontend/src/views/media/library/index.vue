<!--
文件用途：媒体库工作台（media_files，ROADMAP TB-41）——./files 上传资产的网格预览/上传/删除。
核心逻辑：
1. 分页检索本租户媒体登记（文件名模糊搜索 + MIME 过滤），网格卡片展示图片预览、大小、MIME 与引用数；
2. 通用上传（/file/up）：文件落盘成功即由后端登记进 media_files，前端上传成功后刷新网格；
3. 引用感知删除：引用计数>0 时后端拒绝（202004）并返回引用方列表，前端提示先解除引用。
关键注意事项：预览地址用 resolvePlatformAssetUrl 归一（./files/... → 平台绝对 URL）；
  引用统计为读时口径（详情/删除时后端实时扫描看板/SCADA 文档/OTA 升级包），列表页显示的是最近一次扫描值。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NCard,
  NDescriptions,
  NDescriptionsItem,
  NEmpty,
  NGi,
  NGrid,
  NImage,
  NInput,
  NModal,
  NPagination,
  NPopconfirm,
  NSpace,
  NSpin,
  NTag,
  NUpload,
  useMessage
} from 'naive-ui'
import type { UploadCustomRequestOptions, UploadFileInfo } from 'naive-ui'
import {
  deleteMediaFile,
  getMediaFileDetail,
  getMediaFilesList,
  uploadMediaFile,
  type MediaFileDetailResponse,
  type MediaFileItem
} from '@/service/api'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import { resolvePlatformAssetUrl } from '@/utils/auth-user-avatar'

const message = useMessage()
const mediaList = ref<MediaFileItem[]>([])
const searchValue = ref('')
const pagination = reactive({ page: 1, pageSize: 12, itemCount: 0 })
const listLoading = ref(false)
const uploading = ref(false)

const IMAGE_MIME_PREFIX = 'image/'
const SVG_MIME = 'image/svg+xml'

const isImageMime = (mime: string) => mime === SVG_MIME || mime.startsWith(IMAGE_MIME_PREFIX)

/** 预览地址：把后端相对路径归一为平台绝对 URL。 */
const previewUrlOf = (row: MediaFileItem) => resolvePlatformAssetUrl(row.file_path)

/** 字节数可读格式（页面局部展示用，未抽公共 helper）。 */
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

const gridCols = computed(() => '1 s:2 m:3 l:4 xl:5 2xl:6')

const fetchMediaList = async () => {
  listLoading.value = true
  try {
    const { data, error } = await getMediaFilesList({
      page: pagination.page,
      page_size: pagination.pageSize,
      search: searchValue.value || undefined
    })
    if (!error && data) {
      mediaList.value = data.list ?? []
      pagination.itemCount = data.total ?? 0
    }
  } finally {
    listLoading.value = false
  }
}

const handleSearch = () => {
  pagination.page = 1
  void fetchMediaList()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  void fetchMediaList()
}

/** 通用上传：后端 UpFile 落盘成功即登记进 media_files（TB-41 收编 ./files 存储）。 */
const handleUploadChange = async (options: { file: UploadFileInfo }) => {
  const file = options.file?.file
  if (!file || uploading.value) return
  const formData = new FormData()
  formData.append('file', file)
  formData.append('type', 'media')
  uploading.value = true
  try {
    const { error } = await uploadMediaFile(formData)
    if (!error) {
      message.success($t('page.media.uploadSuccess'))
      pagination.page = 1
      await fetchMediaList()
    }
  } finally {
    uploading.value = false
  }
}

// naive-ui 要求 custom-request 存在时才不发送原生请求；上传动作全部走 change 钩子。
const noopCustomRequest = (_options: UploadCustomRequestOptions) => {
  /* 上传统一在 handleUploadChange 处理 */
}

const detailVisible = ref(false)
const detailLoading = ref(false)
const detailData = ref<MediaFileDetailResponse | null>(null)

const openDetail = async (row: MediaFileItem) => {
  detailVisible.value = true
  detailLoading.value = true
  try {
    const { data, error } = await getMediaFileDetail(row.id)
    if (!error && data) {
      detailData.value = data
    }
  } finally {
    detailLoading.value = false
  }
}

const handleDelete = async (row: MediaFileItem) => {
  const { error } = await deleteMediaFile(row.id)
  if (!error) {
    message.success($t('page.media.deleteSuccess'))
    await fetchMediaList()
  }
}

onMounted(() => {
  void fetchMediaList()
})
</script>

<template>
  <div class="min-h-500px">
    <NCard :title="$t('route.media_library')" :bordered="false">
      <template #header-extra>
        <NSpace>
          <NInput
            v-model:value="searchValue"
            :placeholder="$t('page.media.searchPlaceholder')"
            clearable
            style="width: 220px"
            @keyup.enter="handleSearch"
          />
          <NButton type="primary" @click="handleSearch">{{ $t('common.search') }}</NButton>
          <NUpload
            :show-file-list="false"
            accept=".jpg,.jpeg,.png,.svg,.ico,.gif,.xlsx,.xls,.csv,.glb,.gltf,.obj,.fbx,.stl"
            :custom-request="noopCustomRequest"
            @change="handleUploadChange"
          >
            <NButton type="primary" :loading="uploading">{{ $t('page.media.upload') }}</NButton>
          </NUpload>
          <NButton @click="fetchMediaList">{{ $t('page.media.refresh') }}</NButton>
        </NSpace>
      </template>

      <NSpin :show="listLoading">
        <NEmpty v-if="mediaList.length === 0" :description="$t('page.media.empty')" class="py-60px" />
        <NGrid v-else :cols="gridCols" :x-gap="12" :y-gap="12" responsive="screen">
          <NGi v-for="row in mediaList" :key="row.id">
            <NCard size="small" hoverable class="h-full">
              <div class="flex flex-col gap-8px">
                <div
                  class="h-140px flex items-center justify-center overflow-hidden rounded bg-#f5f6f7 dark:bg-#1e1e1e"
                >
                  <NImage
                    v-if="isImageMime(row.mime)"
                    :src="previewUrlOf(row)"
                    object-fit="contain"
                    class="h-full w-full"
                    :img-props="{ style: { maxHeight: '140px' } }"
                  />
                  <div v-else class="flex flex-col items-center gap-4px text-gray-400">
                    <span class="text-32px">📄</span>
                    <span class="text-12px">{{ row.mime }}</span>
                  </div>
                </div>
                <div class="truncate text-13px font-500" :title="row.file_name">{{ row.file_name }}</div>
                <div class="flex flex-wrap items-center gap-4px">
                  <NTag size="small" type="info">{{ formatBytes(row.file_size) }}</NTag>
                  <NTag size="small" :type="row.referenced_count > 0 ? 'warning' : 'default'">
                    {{ $t('page.media.referenceCount') }}: {{ row.referenced_count }}
                  </NTag>
                </div>
                <div class="text-12px text-gray-400">
                  {{ $t('page.media.createdAt') }}: {{ formatDateTime(row.created_at) || '-' }}
                </div>
                <NSpace size="small">
                  <NButton size="tiny" @click="openDetail(row)">{{ $t('page.media.detail') }}</NButton>
                  <NPopconfirm @positive-click="handleDelete(row)">
                    <template #trigger>
                      <NButton size="tiny" type="error" ghost :disabled="row.referenced_count > 0">
                        {{ $t('common.delete') }}
                      </NButton>
                    </template>
                    {{ $t('page.media.deleteConfirm') }}
                  </NPopconfirm>
                </NSpace>
              </div>
            </NCard>
          </NGi>
        </NGrid>
      </NSpin>

      <div class="mt-16px flex justify-end">
        <NPagination
          :page="pagination.page"
          :page-size="pagination.pageSize"
          :item-count="pagination.itemCount"
          @change="handlePageChange"
        />
      </div>
    </NCard>

    <NModal v-model:show="detailVisible" preset="card" :title="$t('page.media.detailTitle')" class="w-640px">
      <NSpin :show="detailLoading">
        <template v-if="detailData">
          <NDescriptions bordered :column="1" size="small">
            <NDescriptionsItem :label="$t('page.media.fileName')">
              {{ detailData.file.file_name }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('page.media.filePath')">
              <span class="break-all font-mono text-12px">{{ detailData.file.file_path }}</span>
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('page.media.size')">
              {{ formatBytes(detailData.file.file_size) }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('page.media.mime')">
              {{ detailData.file.mime }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('page.media.createdAt')">
              {{ formatDateTime(detailData.file.created_at) || '-' }}
            </NDescriptionsItem>
          </NDescriptions>

          <div class="mt-16px mb-8px text-13px font-500">{{ $t('page.media.references') }}</div>
          <NEmpty
            v-if="detailData.referencers.length === 0"
            size="small"
            :description="$t('page.media.noReferences')"
          />
          <NSpace v-else vertical size="small">
            <div v-for="referencer in detailData.referencers" :key="`${referencer.kind}-${referencer.id}`">
              <NTag size="small" type="warning">{{ referencer.kind }}</NTag>
              <span class="ml-6px text-13px">{{ referencer.name || referencer.id }}</span>
            </div>
          </NSpace>
        </template>
      </NSpin>
    </NModal>
  </div>
</template>

<style scoped></style>
