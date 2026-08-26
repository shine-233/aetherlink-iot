<!--
文件用途: 产品预注册页面——按产品+批次批量建档（自动生成/CSV 导入）、清单查询与脱敏导出。
核心逻辑: 筛选条 + 远程分页表格 + 导入向导弹窗（模式选择 → 填参/传文件 → 结果回显）。
关键注意事项: 创建响应中的 voucher 为一次性明文，仅结果面板展示，不做任何持久化。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { DataTableColumns, UploadFileInfo } from 'naive-ui'
import { exportDevice } from '@/service/product/list'
import { $t } from '@/locales'
import PageHeader from '@/components/common/page-header/index.vue'
import { createPreRegisterColumns, formatPreRegisterTime } from './pre-register-table-columns'
import type { PreRegisterRecord } from './types'
import { usePreRegisterList } from './use-pre-register-list'
import { usePreRegisterImport } from './use-pre-register-import'

const { loading, tableData, queryParams, pagination, hasActiveFilters, fetchList, resetQuery } = usePreRegisterList()

const {
  modalVisible,
  submitting,
  uploading,
  mode,
  form,
  formRef,
  rules,
  productOptions,
  productLoading,
  fetchProductOptions,
  importResult,
  submitError,
  canSubmit,
  openModal,
  selectFile,
  submitImport
} = usePreRegisterImport({ onImported: () => fetchList(pagination.page, pagination.pageSize) })

const exporting = ref(false)

const modeOptions = computed(() => [
  { label: $t('page.product.pre-register.modeAuto'), value: 'auto' },
  { label: $t('page.product.pre-register.modeFile'), value: 'file' }
])

const activateFlagOptions = [
  { label: $t('page.product.pre-register.waitingActivate'), value: 'inactive' },
  { label: $t('page.product.pre-register.activated'), value: 'active' }
]

function handleProductSearch(keyword: string) {
  fetchProductOptions(keyword)
}

function fileListToFile(list: UploadFileInfo[]) {
  selectFile(list[0]?.file ?? null)
}

function onFileListChange(list: UploadFileInfo[]) {
  fileListToFile(list)
}

async function handleExport() {
  if (!queryParams.product_id) {
    window.$message?.warning($t('page.product.pre-register.exportNeedProduct'))
    return
  }
  exporting.value = true
  try {
    const params: Record<string, any> = { product_id: queryParams.product_id }
    if (queryParams.batch_number.trim()) params.batch_number = queryParams.batch_number.trim()
    if (queryParams.activate_flag) params.activate_flag = queryParams.activate_flag
    const { data, error } = await exportDevice(params)
    if (error || !data) {
      window.$message?.error($t('page.product.pre-register.exportFailed'))
      return
    }
    const url = String(data).startsWith('./') ? String(data).slice(1) : String(data)
    window.open(url, '_blank', 'noopener,noreferrer')
  } finally {
    exporting.value = false
  }
}

const columns: DataTableColumns<PreRegisterRecord> = createPreRegisterColumns({
  formatTime: formatPreRegisterTime
})

onMounted(() => {
  fetchList()
})
</script>

<template>
  <div class="pre-register-page">
    <NSpace vertical size="medium">
      <PageHeader
        :title="$t('route.product_pre-register')"
        :subtitle="$t('page.product.pre-register.subtitle')"
      >
        <NButton :loading="exporting" @click="handleExport">{{ $t('page.product.pre-register.export') }}</NButton>
        <NButton type="primary" @click="openModal">{{ $t('page.product.pre-register.import') }}</NButton>
      </PageHeader>

      <NCard :bordered="false">
        <NSpace align="center" :wrap="true">
          <NSelect
            v-model:value="queryParams.product_id"
            class="filter-control"
            clearable
            filterable
            remote
            :loading="productLoading"
            :options="productOptions"
            :placeholder="$t('page.product.pre-register.productPlaceholder')"
            @search="handleProductSearch"
          />
          <NInput
            v-model:value="queryParams.batch_number"
            class="filter-control"
            clearable
            :placeholder="$t('page.product.pre-register.batchPlaceholder')"
          />
          <NSelect
            v-model:value="queryParams.activate_flag"
            class="filter-control filter-narrow"
            clearable
            :options="activateFlagOptions"
            :placeholder="$t('page.product.pre-register.activateFlag')"
          />
          <NButton type="primary" @click="fetchList()">{{ $t('common.search') }}</NButton>
          <NButton @click="resetQuery">{{ $t('common.reset') }}</NButton>
        </NSpace>
      </NCard>

      <NDataTable
        remote
        :columns="columns"
        :data="tableData"
        :loading="loading"
        :pagination="pagination"
        :scroll-x="1100"
      >
        <template #empty>
          <div class="pr-empty">
            <NEmpty
              :description="
                $t(hasActiveFilters ? 'page.product.pre-register.emptyFiltered' : 'page.product.pre-register.empty')
              "
            />
          </div>
        </template>
      </NDataTable>
    </NSpace>

    <NModal
      v-model:show="modalVisible"
      preset="card"
      class="import-modal"
      :title="$t('page.product.pre-register.import')"
    >
      <NSpace vertical size="medium">
        <NRadioGroup v-model:value="mode" name="pre-register-mode">
          <NSpace>
            <NRadio v-for="item in modeOptions" :key="item.value" :value="item.value">{{ item.label }}</NRadio>
          </NSpace>
        </NRadioGroup>

        <NForm ref="formRef" :model="form" :rules="rules" label-placement="top">
          <NGrid cols="1 s:2" responsive="screen" :x-gap="16">
            <NFormItemGi :label="$t('page.product.pre-register.product')" required>
              <NSelect
                v-model:value="form.product_id"
                filterable
                remote
                :loading="productLoading"
                :options="productOptions"
                :placeholder="$t('page.product.pre-register.productPlaceholder')"
                @search="handleProductSearch"
              />
            </NFormItemGi>
            <NFormItemGi :label="$t('page.product.pre-register.batchNumber')" required>
              <NInput
                v-model:value="form.batch_number"
                maxlength="36"
                :placeholder="$t('page.product.pre-register.batchPlaceholder')"
              />
            </NFormItemGi>
            <NFormItemGi :label="$t('page.product.pre-register.currentVersion')">
              <NInput
                v-model:value="form.current_version"
                maxlength="36"
                :placeholder="$t('page.product.pre-register.versionPlaceholder')"
              />
            </NFormItemGi>
            <template v-if="mode === 'auto'">
              <NFormItemGi :label="$t('page.product.pre-register.deviceCount')" required>
                <NInputNumber v-model:value="form.device_count" :min="1" :max="10000" class="w-full" />
              </NFormItemGi>
            </template>
          </NGrid>

          <NFormItem v-if="mode === 'file'" :label="$t('page.product.pre-register.csvFile')" required>
            <NSpace vertical class="w-full">
              <NUpload accept=".csv" :max="1" :default-upload="false" @update:file-list="onFileListChange">
                <NButton>{{ $t('page.product.pre-register.chooseCsv') }}</NButton>
              </NUpload>
              <div class="csv-hint">{{ $t('page.product.pre-register.csvHint') }}</div>
            </NSpace>
          </NFormItem>
        </NForm>

        <NAlert v-if="submitError" type="error" :show-icon="true">{{ submitError }}</NAlert>

        <NResult
          v-if="importResult"
          status="success"
          :title="$t('page.product.pre-register.resultTitle', { count: importResult.created_count })"
        >
          <template #footer>
            <NSpace vertical size="small" class="result-panel">
              <div v-if="importResult.skipped_existing.length">
                {{ $t('page.product.pre-register.skippedExisting') }}:
                <NTag v-for="item in importResult.skipped_existing" :key="item" size="small" class="result-tag">
                  {{ item }}
                </NTag>
              </div>
              <div v-if="importResult.skipped_duplicate_rows.length">
                {{ $t('page.product.pre-register.skippedDuplicate') }}:
                <NTag v-for="item in importResult.skipped_duplicate_rows" :key="item" size="small" class="result-tag">
                  {{ item }}
                </NTag>
              </div>
              <NAlert type="warning" :show-icon="false">
                {{ $t('page.product.pre-register.voucherOnceHint') }}
              </NAlert>
            </NSpace>
          </template>
        </NResult>
      </NSpace>

      <template #footer>
        <NSpace justify="end">
          <NButton @click="modalVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="submitting || uploading" :disabled="!canSubmit" @click="submitImport()">
            {{ $t('page.product.pre-register.submitImport') }}
          </NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.pre-register-page {
  padding: 16px;
}

.filter-control {
  width: 220px;
}

.filter-narrow {
  width: 150px;
}

.pr-empty {
  display: flex;
  min-height: 220px;
  align-items: center;
  justify-content: center;
}

.import-modal {
  width: min(720px, calc(100vw - 32px));
}

.w-full {
  width: 100%;
}

.csv-hint {
  color: var(--text-color-3);
  font-size: 12px;
}

.result-tag {
  margin-right: 6px;
}
</style>
