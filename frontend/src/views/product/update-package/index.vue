<!--
文件用途: 承载升级包管理相关的产品升级页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import dayjs from 'dayjs'
import { useRoute, useRouter } from 'vue-router'
import { deleteOtaPackage } from '@/service/product/update-package'
import { $t } from '@/locales'
import { createOtaPackageColumns } from './ota-package-table-columns'
import type { OtaPackageRecord } from './ota-package-types'
import { useOtaPackageForm } from './use-ota-package-form'
import { useOtaPackageList } from './use-ota-package-list'

const route = useRoute()
const router = useRouter()
const detailVisible = ref(false)
const detailRecord = ref<OtaPackageRecord | null>(null)
const isReturnToOtaTaskFlow = computed(() => route.query.return_to === 'ota_task')
const {
  loading,
  deviceConfigLoading,
  tableData,
  deviceConfigOptions,
  queryParams,
  pagination,
  fetchDeviceConfigs,
  ensureDeviceConfigOption,
  fetchPackages,
  resetQuery
} = useOtaPackageList()

const {
  saving,
  uploading,
  modalVisible,
  isEditing,
  selectedFile,
  fileDragActive,
  fileInputRef,
  form,
  packageTypeOptions,
  signatureOptions,
  resetForm,
  openCreateModal,
  openEditModal,
  selectPackageFile,
  onFileChange,
  onFileDrop,
  onFileDragLeave,
  uploadSelectedFile,
  buildPayload,
  savePackage
} = useOtaPackageForm({ fetchPackages })

const hasActivePackageFilters = computed(
  () => Boolean(queryParams.name.trim() || queryParams.version.trim() || queryParams.device_config_id)
)
let deviceConfigSearchTimer: ReturnType<typeof setTimeout> | undefined

function formatTime(value?: string) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'
}

function packageTypeLabel(value?: number) {
  return value === 1 ? $t('page.product.update-package.diff') : $t('page.product.update-package.full')
}

function normalizePackageUrl(url?: string) {
  if (!url) return ''
  return url.startsWith('./') ? url.slice(1) : url
}

function openDetailModal(row: OtaPackageRecord) {
  detailRecord.value = row
  detailVisible.value = true
}

function formatOptional(value?: string | number | null) {
  if (value === undefined || value === null || value === '') return '-'
  return String(value)
}

function packageFileName(url?: string | null) {
  if (!url) return '-'
  const normalized = url.replace(/\\/g, '/')
  return normalized.split('/').filter(Boolean).pop() || normalized
}

function deletePackage(row: OtaPackageRecord) {
  window.$dialog?.warning({
    title: $t('common.deletePrompt'),
    content: `${$t('common.confirmDelete')} ${row.name || row.id}`,
    positiveText: $t('common.delete'),
    negativeText: $t('common.cancel'),
    onPositiveClick: async () => {
      const { error } = await deleteOtaPackage(row.id)
      if (!error) {
        window.$message?.success($t('common.deleteSuccess'))
        await fetchPackages()
      }
    }
  })
}

function downloadPackage(row: OtaPackageRecord) {
  const url = normalizePackageUrl(row.package_url)
  if (!url) return
  window.open(url, '_blank', 'noopener,noreferrer')
}

function handleDeviceConfigSearch(search: string) {
  if (deviceConfigSearchTimer) clearTimeout(deviceConfigSearchTimer)
  deviceConfigSearchTimer = setTimeout(() => {
    fetchDeviceConfigs(search)
  }, 250)
}

function openPackageEditModal(row: OtaPackageRecord) {
  if (row.device_config_id) {
    ensureDeviceConfigOption({
      label: row.device_config_name || row.device_config_id,
      value: row.device_config_id
    })
  }
  openEditModal(row)
}

async function savePackageAndContinue() {
  const saved = await savePackage()
  if (saved && isReturnToOtaTaskFlow.value) {
    router.push({ name: 'product_update-ota' })
  }
}

const columns: DataTableColumns<OtaPackageRecord> = createOtaPackageColumns({
  formatTime,
  packageTypeLabel,
  openDetailModal,
  downloadPackage,
  openEditModal: openPackageEditModal,
  deletePackage
})

onMounted(() => {
  fetchDeviceConfigs()
  fetchPackages()
})

onBeforeUnmount(() => {
  if (deviceConfigSearchTimer) clearTimeout(deviceConfigSearchTimer)
})
</script>

<template>
  <div class="product-page">
    <NSpace vertical size="medium">
      <div class="page-header">
        <div>
          <div class="page-title">{{ $t('page.product.update-package.packageList') }}</div>
          <div class="page-subtitle">{{ $t('route.product_update-package') }}</div>
        </div>
        <NSpace>
          <NButton @click="fetchPackages">{{ $t('common.refresh') }}</NButton>
          <NButton type="primary" @click="openCreateModal">{{ $t('page.product.update-package.packageAdd') }}</NButton>
        </NSpace>
      </div>

      <NAlert v-if="isReturnToOtaTaskFlow" type="info" :show-icon="true">
        <strong>{{ $t('page.product.update-package.returnToOtaTitle') }}</strong>
        <span class="return-flow-desc">{{ $t('page.product.update-package.returnToOtaDesc') }}</span>
      </NAlert>

      <NCard :bordered="false">
        <NSpace align="center" :wrap="true">
          <NInput
            v-model:value="queryParams.name"
            class="filter-control"
            clearable
            :placeholder="$t('page.product.update-package.packageNamePlaceholder')"
          />
          <NInput
            v-model:value="queryParams.version"
            class="filter-control"
            clearable
            :placeholder="$t('page.product.update-package.versionCodePlaceholder')"
          />
          <NSelect
            v-model:value="queryParams.device_config_id"
            class="filter-control"
            clearable
            filterable
            remote
            :loading="deviceConfigLoading"
            :options="deviceConfigOptions"
            :placeholder="$t('page.product.update-package.productPlaceholder')"
            @search="handleDeviceConfigSearch"
          />
          <NButton type="primary" @click="fetchPackages">{{ $t('common.search') }}</NButton>
          <NButton @click="resetQuery">{{ $t('common.reset') }}</NButton>
        </NSpace>
      </NCard>

      <NDataTable
        remote
        :columns="columns"
        :data="tableData"
        :loading="loading"
        :pagination="pagination"
        :scroll-x="1280"
      >
        <template #empty>
          <div class="package-empty-state">
            <NEmpty
              :description="
                hasActivePackageFilters
                  ? $t('page.product.update-package.emptyFilteredTitle')
                  : $t('page.product.update-package.emptyTitle')
              "
            >
              <template #extra>
                <div class="package-empty-extra">
                  <p>
                    {{
                      hasActivePackageFilters
                        ? $t('page.product.update-package.emptyFilteredDesc')
                        : $t('page.product.update-package.emptyDesc')
                    }}
                  </p>
                  <NButton v-if="hasActivePackageFilters" secondary @click="resetQuery">
                    {{ $t('page.product.update-package.emptyClearFilters') }}
                  </NButton>
                  <NButton v-else type="primary" @click="openCreateModal">
                    {{ $t('page.product.update-package.packageAdd') }}
                  </NButton>
                </div>
              </template>
            </NEmpty>
          </div>
        </template>
      </NDataTable>
    </NSpace>

    <NModal
      v-model:show="modalVisible"
      preset="card"
      class="package-modal"
      :title="isEditing ? $t('page.product.update-package.packageEdit') : $t('page.product.update-package.packageAdd')"
    >
      <NForm label-placement="top">
        <NGrid cols="1 s:2" responsive="screen" :x-gap="16">
          <NFormItemGi :label="$t('page.product.update-package.packageName')" required>
            <NInput v-model:value="form.name" :placeholder="$t('page.product.update-package.packageNamePlaceholder')" />
          </NFormItemGi>
          <NFormItemGi :label="$t('page.product.update-package.versionCode')" required>
            <NInput
              v-model:value="form.version"
              :placeholder="$t('page.product.update-package.versionCodePlaceholder')"
            />
          </NFormItemGi>
          <NFormItemGi :label="$t('page.product.update-package.version')">
            <NInput
              v-model:value="form.target_version"
              :placeholder="$t('page.product.update-package.versionPlaceholder')"
            />
          </NFormItemGi>
          <NFormItemGi :label="$t('page.product.update-package.deviceConfig')" required>
            <NSelect
              v-model:value="form.device_config_id"
              filterable
              remote
              :loading="deviceConfigLoading"
              :options="deviceConfigOptions"
              :placeholder="$t('page.product.update-package.productPlaceholder')"
              @search="handleDeviceConfigSearch"
            />
          </NFormItemGi>
          <NFormItemGi :label="$t('page.product.update-package.type')" required>
            <NSelect v-model:value="form.package_type" :options="packageTypeOptions" />
          </NFormItemGi>
          <NFormItemGi :label="$t('page.product.update-package.signMode')" required>
            <NSelect v-model:value="form.signature_type" :options="signatureOptions" />
          </NFormItemGi>
          <NFormItemGi :label="$t('page.product.update-package.moduleName')">
            <NInput v-model:value="form.module" />
          </NFormItemGi>
          <NFormItemGi :label="$t('page.product.update-package.package')" required>
            <NSpace vertical class="file-box">
              <div
                class="file-drop-zone"
                :class="{ 'is-dragging': fileDragActive }"
                @dragenter.prevent="fileDragActive = true"
                @dragover.prevent="fileDragActive = true"
                @dragleave.prevent="onFileDragLeave"
                @drop.prevent="onFileDrop"
              >
                <input
                  ref="fileInputRef"
                  type="file"
                  accept=".bin,.hex,.elf,.tar,.gz,.gzip,.zip,.apk,.dav,.pack"
                  @change="onFileChange"
                />
                <div class="file-drop-hint">{{ $t('page.product.update-package.dragDropHint') }}</div>
                <div class="file-drop-types">{{ $t('page.product.update-package.fileTypeHint') }}</div>
              </div>
              <NSpace align="center">
                <NButton :loading="uploading" :disabled="!selectedFile" @click="uploadSelectedFile">
                  {{ $t('page.product.list.file') }}
                </NButton>
                <span class="file-name">
                  {{
                    selectedFile
                      ? `${$t('page.product.update-package.selectedFile')}: ${selectedFile.name}`
                      : form.package_url || '-'
                  }}
                </span>
              </NSpace>
            </NSpace>
          </NFormItemGi>
          <NFormItemGi :label="$t('page.product.update-package.package')">
            <NInput v-model:value="form.package_url" />
          </NFormItemGi>
        </NGrid>
        <NFormItem :label="$t('page.product.update-package.customInfo')">
          <NInput v-model:value="form.additional_info" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" />
        </NFormItem>
        <NFormItem :label="$t('page.product.update-package.desc')">
          <NInput v-model:value="form.description" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" />
        </NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="modalVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="saving" @click="savePackageAndContinue">
            {{ isReturnToOtaTaskFlow ? $t('page.product.update-package.returnToOtaSaveAction') : $t('common.save') }}
          </NButton>
        </NSpace>
      </template>
    </NModal>

    <NModal
      v-model:show="detailVisible"
      preset="card"
      class="package-modal"
      :title="$t('page.product.update-package.packageDetail')"
    >
      <NDescriptions v-if="detailRecord" bordered :column="1" label-placement="left" size="small">
        <NDescriptionsItem :label="$t('page.product.update-package.packageName')">
          {{ formatOptional(detailRecord.name) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.versionCode')">
          {{ formatOptional(detailRecord.version) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.version')">
          {{ formatOptional(detailRecord.target_version) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.deviceConfig')">
          {{ formatOptional(detailRecord.device_config_name || detailRecord.device_config_id) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.type')">
          {{ packageTypeLabel(detailRecord.package_type) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.moduleName')">
          {{ formatOptional(detailRecord.module) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.signMode')">
          {{ formatOptional(detailRecord.signature_type) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-ota.packageSign')">
          {{ formatOptional(detailRecord.signature) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.fileName')">
          {{ packageFileName(detailRecord.package_url) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.packageUrl')">
          {{ formatOptional(detailRecord.package_url) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.desc')">
          {{ formatOptional(detailRecord.description) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.customInfo')">
          {{ formatOptional(detailRecord.additional_info) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.createTime')">
          {{ formatTime(detailRecord.created_at) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.updatedAt')">
          {{ formatTime(detailRecord.updated_at) }}
        </NDescriptionsItem>
        <NDescriptionsItem :label="$t('page.product.update-package.remark')">
          {{ formatOptional(detailRecord.remark) }}
        </NDescriptionsItem>
      </NDescriptions>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="detailVisible = false">{{ $t('common.confirm') }}</NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.product-page {
  padding: 16px;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.page-title {
  font-size: 22px;
  font-weight: 700;
}

.page-subtitle {
  margin-top: 4px;
  color: var(--text-color-3);
}

.filter-control {
  width: 220px;
}

.action-row {
  display: flex;
  gap: 8px;
}

.return-flow-desc {
  display: block;
  margin-top: 4px;
}

.package-empty-state {
  display: flex;
  min-height: 220px;
  align-items: center;
  justify-content: center;
  padding: 28px 16px;
}

.package-empty-extra {
  display: grid;
  justify-items: center;
  gap: 12px;
}

.package-empty-extra p {
  max-width: 440px;
  margin: 0;
  color: var(--text-color-3);
  font-size: 13px;
  line-height: 1.6;
  text-align: center;
}

.package-modal {
  width: min(760px, calc(100vw - 32px));
}

.file-box {
  width: 100%;
}

.file-drop-zone {
  display: grid;
  min-height: 88px;
  padding: 14px;
  border: 1px dashed var(--border-color);
  border-radius: 6px;
  background: var(--card-color);
  gap: 8px;
  place-items: center;
  transition:
    border-color 0.2s ease,
    background-color 0.2s ease;
}

.file-drop-zone.is-dragging {
  border-color: var(--primary-color);
  background: var(--primary-color-hover);
}

.file-drop-hint {
  color: var(--text-color-3);
  font-size: 13px;
}

.file-drop-types {
  color: var(--text-color-3);
  font-size: 12px;
}

.file-name {
  max-width: 260px;
  overflow: hidden;
  color: var(--text-color-2);
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 720px) {
  .page-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .filter-control {
    width: 100%;
  }
}
</style>
