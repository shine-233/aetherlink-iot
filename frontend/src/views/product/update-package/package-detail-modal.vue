<!--
文件用途: 升级包详情弹窗（纯展示组件）。
核心逻辑: 只读展示传入的升级包记录；时间/类型/文件名等格式化复用 ota-package-format。
关键注意事项: record 可能为 null（弹窗未打开时），描述区需判空渲染；v-model:show 与页面双向同步。
重构建议: 字段继续增多时可改为配置数组驱动 NDescriptions 渲染。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'
import type { OtaPackageRecord } from './ota-package-types'
import { formatOptional, formatTime, packageFileName, packageTypeLabel } from './ota-package-format'

const props = defineProps<{
  show: boolean
  record: OtaPackageRecord | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const visible = computed({
  get: () => props.show,
  set: (value) => emit('update:show', value)
})
</script>

<template>
  <NModal
    v-model:show="visible"
    preset="card"
    class="package-modal"
    :title="$t('page.product.update-package.packageDetail')"
  >
    <NDescriptions v-if="record" bordered :column="1" label-placement="left" size="small">
      <NDescriptionsItem :label="$t('page.product.update-package.packageName')">
        {{ formatOptional(record.name) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.versionCode')">
        {{ formatOptional(record.version) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.version')">
        {{ formatOptional(record.target_version) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.deviceConfig')">
        {{ formatOptional(record.device_config_name || record.device_config_id) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.type')">
        {{ packageTypeLabel(record.package_type) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.moduleName')">
        {{ formatOptional(record.module) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.signMode')">
        {{ formatOptional(record.signature_type) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-ota.packageSign')">
        {{ formatOptional(record.signature) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.fileName')">
        {{ packageFileName(record.package_url) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.packageUrl')">
        {{ formatOptional(record.package_url) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.desc')">
        {{ formatOptional(record.description) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.customInfo')">
        {{ formatOptional(record.additional_info) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.createTime')">
        {{ formatTime(record.created_at) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.updatedAt')">
        {{ formatTime(record.updated_at) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.product.update-package.remark')">
        {{ formatOptional(record.remark) }}
      </NDescriptionsItem>
    </NDescriptions>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="visible = false">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.package-modal {
  width: min(760px, calc(100vw - 32px));
}
</style>
