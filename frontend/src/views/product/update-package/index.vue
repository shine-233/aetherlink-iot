<!--
文件用途: 承载升级包管理相关的产品升级页面或业务组件。
核心逻辑: 列表分页/筛选/加载态收口在共享 useListPage（use-ota-package-list 配置），设备配置远程
  选项与防抖在 use-device-config-options，表单状态与提交在 use-ota-package-form；新增/编辑与详情
  弹窗拆为 package-form-modal / package-detail-modal，本组件专注布局与事件编排。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 若页面继续增长，可把行动作（删除/下载）也抽成组合函数。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { deleteOtaPackage } from '@/service/product/update-package'
import { $t } from '@/locales'
import PageHeader from '@/components/common/page-header/index.vue'
import PackageFormModal from './package-form-modal.vue'
import PackageDetailModal from './package-detail-modal.vue'
import { createOtaPackageColumns } from './ota-package-table-columns'
import { normalizePackageUrl } from './ota-package-format'
import type { OtaPackageRecord } from './ota-package-types'
import { useOtaPackageList } from './use-ota-package-list'
import { useDeviceConfigOptions } from './use-device-config-options'
import { useOtaPackageForm } from './use-ota-package-form'

const route = useRoute()
const router = useRouter()

const isReturnToOtaTaskFlow = computed(() => route.query.return_to === 'ota_task')

// 列表查询主入口：分页、加载态与过期请求丢弃交给 useListPage。
const {
  query: filter,
  rows: tableData,
  loading,
  pagination,
  load: fetchPackages,
  search: searchPackages,
  reset: resetQuery
} = useOtaPackageList()

// 设备配置远程搜索选项：列表筛选与表单弹窗共用，防抖与过期响应丢弃在组合函数内收口。
const {
  deviceConfigLoading,
  deviceConfigOptions,
  fetchDeviceConfigs,
  ensureDeviceConfigOption,
  handleDeviceConfigSearch
} = useDeviceConfigOptions({ getSelectedId: () => filter.device_config_id })

const {
  saving,
  uploading,
  modalVisible,
  isEditing,
  selectedFile,
  form,
  packageTypeOptions,
  signatureOptions,
  openCreateModal,
  openEditModal,
  selectPackageFile,
  uploadSelectedFile,
  savePackage
} = useOtaPackageForm({ refresh: fetchPackages })

const detailVisible = ref(false)
const detailRecord = ref<OtaPackageRecord | null>(null)

const hasActivePackageFilters = computed(() =>
  Boolean(filter.name.trim() || filter.version.trim() || filter.device_config_id)
)

function openDetailModal(row: OtaPackageRecord) {
  detailRecord.value = row
  detailVisible.value = true
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

async function savePackageAndContinue() {
  const saved = await savePackage()
  if (saved && isReturnToOtaTaskFlow.value) {
    router.push({ name: 'product_update-ota' })
  }
}

const columns = createOtaPackageColumns({
  openDetailModal,
  downloadPackage,
  openEditModal: openPackageEditModal,
  deletePackage
})

onMounted(() => {
  fetchDeviceConfigs()
  void fetchPackages()
})
</script>

<template>
  <div class="product-page">
    <NSpace vertical size="medium">
      <PageHeader :title="$t('page.product.update-package.packageList')" :subtitle="$t('route.product_update-package')">
        <NButton @click="fetchPackages">{{ $t('common.refresh') }}</NButton>
        <NButton type="primary" @click="openCreateModal">{{ $t('page.product.update-package.packageAdd') }}</NButton>
      </PageHeader>

      <NAlert v-if="isReturnToOtaTaskFlow" type="info" :show-icon="true">
        <strong>{{ $t('page.product.update-package.returnToOtaTitle') }}</strong>
        <span class="return-flow-desc">{{ $t('page.product.update-package.returnToOtaDesc') }}</span>
      </NAlert>

      <NCard :bordered="false">
        <NSpace align="center" :wrap="true">
          <NInput
            v-model:value="filter.name"
            class="filter-control"
            clearable
            :placeholder="$t('page.product.update-package.packageNamePlaceholder')"
          />
          <NInput
            v-model:value="filter.version"
            class="filter-control"
            clearable
            :placeholder="$t('page.product.update-package.versionCodePlaceholder')"
          />
          <NSelect
            v-model:value="filter.device_config_id"
            class="filter-control"
            clearable
            filterable
            remote
            :loading="deviceConfigLoading"
            :options="deviceConfigOptions"
            :placeholder="$t('page.product.update-package.productPlaceholder')"
            @search="handleDeviceConfigSearch"
          />
          <NButton type="primary" @click="searchPackages">{{ $t('common.search') }}</NButton>
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

    <PackageFormModal
      v-model:show="modalVisible"
      :is-editing="isEditing"
      :saving="saving"
      :uploading="uploading"
      :form="form"
      :selected-file="selectedFile"
      :package-type-options="packageTypeOptions"
      :signature-options="signatureOptions"
      :device-config-loading="deviceConfigLoading"
      :device-config-options="deviceConfigOptions"
      :is-return-to-ota-task-flow="isReturnToOtaTaskFlow"
      @save="savePackageAndContinue"
      @upload="uploadSelectedFile"
      @select-file="selectPackageFile"
      @search-device-configs="handleDeviceConfigSearch"
    />

    <PackageDetailModal v-model:show="detailVisible" :record="detailRecord" />
  </div>
</template>

<style scoped>
.product-page {
  padding: 16px;
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

@media (max-width: 720px) {
  .filter-control {
    width: 100%;
  }
}
</style>
