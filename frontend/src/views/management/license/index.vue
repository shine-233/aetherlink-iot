<!--
文件用途：P3 商业许可证状态页（SYS_ADMIN）——展示验证结论、有效期、特性与配额。
核心逻辑：只读视图；一切判定在 service/pkg/license 层，本页仅呈现与手动刷新。
关键注意事项：
1. 页面不展示材料本身，只展示结论与 SHA-256 摘要（后端已做裁剪）。
2. 未启用边界（enabled=false）是合法状态：明确提示"边界未启用"而不是渲染成错误。
-->
<script setup lang="tsx">
import { onMounted, ref } from 'vue'
import { NButton, NCard, NDescriptions, NDescriptionsItem, NTag } from 'naive-ui'
import { useLoading } from '@aetherlink/hooks'
import { fetchLicenseStatus, type LicenseStatus } from '@/service/api'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'

const { loading, startLoading, endLoading } = useLoading(false)
const status = ref<LicenseStatus | null>(null)
const loadError = ref('')

async function loadStatus() {
  startLoading()
  loadError.value = ''
  try {
    const { data, error } = await fetchLicenseStatus()
    if (error) {
      loadError.value = String((error as { message?: string }).message ?? error)
      return
    }
    status.value = data ?? null
  } finally {
    endLoading()
  }
}

function validityTag(s: LicenseStatus) {
  if (!s.enabled) return { type: 'default' as const, text: $t('page.license.notEnabled') }
  if (s.valid) return { type: 'success' as const, text: $t('page.license.valid') }
  return { type: 'error' as const, text: $t('page.license.invalid') }
}

function fmtMs(ms?: number) {
  if (!ms) return '-'
  return formatDateTime(new Date(ms).toISOString())
}

onMounted(() => {
  void loadStatus()
})
</script>

<template>
  <div class="min-h-500px">
    <NCard :title="$t('page.license.title')" class="h-full">
      <template #header-extra>
        <NButton size="small" :loading="loading" @click="loadStatus">
          {{ $t('page.license.refresh') }}
        </NButton>
      </template>

      <div v-if="loadError" class="py-4 text-red-500">{{ loadError }}</div>
      <template v-else-if="status">
        <NDescriptions :column="2" bordered size="small">
          <NDescriptionsItem :label="$t('page.license.state')">
            <NTag :type="validityTag(status).type" size="small">{{ validityTag(status).text }}</NTag>
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('page.license.required')">
            {{ status.required ? $t('page.license.yes') : $t('page.license.no') }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('page.license.edition')">
            {{ status.edition || '-' }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('page.license.issuedTo')">
            {{ status.issued_to || '-' }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('page.license.notBefore')">
            {{ fmtMs(status.not_before_ms) }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('page.license.notAfter')">
            {{ fmtMs(status.not_after_ms) }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('page.license.maxDevices')">
            {{ status.max_devices && status.max_devices > 0 ? status.max_devices : $t('page.license.unlimited') }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('page.license.maxTenants')">
            {{ status.max_tenants && status.max_tenants > 0 ? status.max_tenants : $t('page.license.unlimited') }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('page.license.features')" :span="2">
            <template v-if="status.features && status.features.length">
              <NTag v-for="f in status.features" :key="f" size="small" class="mr-2">{{ f }}</NTag>
            </template>
            <span v-else>-</span>
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('page.license.fingerprint')" :span="2">
            <span class="break-all font-mono text-xs">{{ status.fingerprint || '-' }}</span>
          </NDescriptionsItem>
        </NDescriptions>
        <div v-if="status.reason" class="mt-3 text-gray-500">{{ status.reason }}</div>
      </template>
    </NCard>
  </div>
</template>
