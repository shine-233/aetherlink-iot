<!--
文件用途: OTA 升级任务页的升级包选择卡片(远程搜索下拉 + 已选包的配置/签名标签)。
核心逻辑: 纯展示组件,选中值与搜索词全部回抛父级,由 useOtaTaskPage 负责防抖和取数。
关键注意事项: 远程搜索透传的是搜索词而非事件对象,父级 fetchPackages(search) 首参是搜索词。
-->
<script setup lang="ts">
import { $t } from '@/locales'
import type { OtaPackageRecord } from './ota-task-types'

defineProps<{
  selectedPackageId: string | null
  packageOptions: Array<{ label: string; value: string }>
  loading: boolean
  selectedPackage: OtaPackageRecord | null
}>()

const emit = defineEmits<{
  'update:selectedPackageId': [value: string | null]
  search: [query: string]
}>()

function handleUpdateSelectedPackageId(value: string | null) {
  emit('update:selectedPackageId', value)
}

function handleSearch(query: string) {
  emit('search', query)
}
</script>

<template>
  <NCard :bordered="false">
    <NSpace align="center" :wrap="true">
      <NSelect
        class="package-select"
        filterable
        remote
        clearable
        :value="selectedPackageId"
        :loading="loading"
        :options="packageOptions"
        :placeholder="$t('page.product.update-package.packagePlaceholder')"
        @update:value="handleUpdateSelectedPackageId"
        @search="handleSearch"
      />
      <NTag v-if="selectedPackage?.device_config_name" type="info">{{ selectedPackage.device_config_name }}</NTag>
      <NTag v-if="selectedPackage?.signature" type="success">
        {{ $t('page.product.update-ota.packageSign') }}: {{ selectedPackage.signature }}
      </NTag>
    </NSpace>
  </NCard>
</template>

<style scoped>
.package-select {
  width: 320px;
}

@media (max-width: 720px) {
  .package-select {
    width: 100%;
  }
}
</style>
