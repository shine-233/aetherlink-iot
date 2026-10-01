<!--
文件用途：移动应用中心工作台（mobile_app_bundles，ROADMAP TB-23）——安装包版本列表/上传/发布/归档。
核心逻辑：
1. 分页检索本租户应用包（platform/status 精确过滤），表格展示版本、文件名、大小、SHA-256 校验和与发布履历；
  分页/加载态/过期请求收口在 useListPage，页面只负责筛选与动作编排；
2. 上传登记（multipart：file+platform+version+release_notes）：后端按平台校验扩展名（apk/ipa/zip）与 ZIP 签名，
   同租户同平台同版本重复登记被拒（202006），新包固定落 draft；
3. 发布状态机：publish 仅 draft→published、archive 仅 published→archived（非法流转返回 202005）；
   按钮禁用与后端 CanTransitionAppBundle 同源口径，前端禁用只是引导，后端仍 fail-closed 复核。
关键注意事项：删除仅 draft/archived（published 须先归档）；下载走 /files 静态面（file_path 经
  resolvePlatformAssetUrl 归一为平台绝对 URL，与媒体库预览同链路），带鉴权头的下载端点供 uniapp/Range 场景使用。
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton, NCard, NDataTable, NEmpty, NSelect, NSpace, NSpin } from 'naive-ui'
import {
  archiveMobileAppBundle,
  deleteMobileAppBundle,
  getMobileAppBundles,
  publishMobileAppBundle,
  type MobileAppBundleItem,
  type MobileAppBundlePlatform,
  type MobileAppBundleStatus
} from '@/service/api'
import { $t } from '@/locales'
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'
import { resolvePlatformAssetUrl } from '@/utils/auth-user-avatar'
import { PLATFORM_OPTIONS, STATUS_OPTIONS } from './constants'
import UploadModal from './modules/upload-modal.vue'
import EditNotesModal from './modules/edit-notes-modal.vue'
import { useBundleColumns } from './modules/use-bundle-columns'

type QueryFormModel = {
  platform: MobileAppBundlePlatform | null
  status: MobileAppBundleStatus | null
}

// 列表查询主入口：分页、加载态与过期请求丢弃交给 useListPage；动作成功后按需回刷。
const { query: filter, rows: bundleList, loading: listLoading, pagination, load: fetchBundleList, setPage } =
  useListPage<MobileAppBundleItem, QueryFormModel>({
    initialQuery: () => ({ platform: null, status: null }),
    fetcher: async (params) => {
      const response = await getMobileAppBundles({
        page: params.page,
        page_size: params.page_size,
        platform: params.platform || undefined,
        status: params.status || undefined
      })
      return fromFlatResponse<MobileAppBundleItem>(response)
    }
  })

// ---- 筛选与动作 ----
const handleFilterChange = () => {
  void setPage(1)
}

const handlePublish = async (row: MobileAppBundleItem) => {
  const { error } = await publishMobileAppBundle(row.id)
  if (!error) {
    window.$message?.success($t('page.app_bundle.publishSuccess'))
    await fetchBundleList()
  }
}

const handleArchive = async (row: MobileAppBundleItem) => {
  const { error } = await archiveMobileAppBundle(row.id)
  if (!error) {
    window.$message?.success($t('page.app_bundle.archiveSuccess'))
    await fetchBundleList()
  }
}

const handleDelete = async (row: MobileAppBundleItem) => {
  const { error } = await deleteMobileAppBundle(row.id)
  if (!error) {
    window.$message?.success($t('page.app_bundle.deleteSuccess'))
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

// ---- 弹窗编排（表单细节在子组件内） ----
const uploadVisible = ref(false)
const editVisible = ref(false)
const editTarget = ref<MobileAppBundleItem | null>(null)

const openUpload = () => {
  uploadVisible.value = true
}

const openEdit = (row: MobileAppBundleItem) => {
  editTarget.value = row
  editVisible.value = true
}

const { columns } = useBundleColumns({
  onPublish: (row) => void handlePublish(row),
  onArchive: (row) => void handleArchive(row),
  onDownload: handleDownload,
  onEdit: openEdit,
  onDelete: (row) => void handleDelete(row)
})

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
            v-model:value="filter.platform"
            :options="PLATFORM_OPTIONS"
            :placeholder="$t('page.app_bundle.platform')"
            clearable
            style="width: 140px"
            @update:value="handleFilterChange"
          />
          <NSelect
            v-model:value="filter.status"
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
        <NEmpty v-if="bundleList.length === 0 && !listLoading" :description="$t('page.app_bundle.empty')" class="py-60px" />
        <NDataTable
          v-else
          remote
          :columns="columns"
          :data="bundleList"
          :loading="listLoading"
          :pagination="pagination"
          :bordered="false"
          size="small"
          :scroll-x="1200"
        />
      </NSpin>
    </NCard>

    <UploadModal v-model:show="uploadVisible" @success="setPage(1)" />
    <EditNotesModal v-model:show="editVisible" :bundle="editTarget" @success="fetchBundleList" />
  </div>
</template>

<style scoped></style>
