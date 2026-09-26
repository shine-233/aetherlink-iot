<!--
文件用途：Billing/API 配额页（TB-17 + TB-17R，ThingsBoard PE per-tenant API quotas 对标）——
展示当前租户今日 REST API 调用数、套餐日限额与剩余量，以及传输维度（MQTT 连接认证）日配额。
核心逻辑：
1. 挂载时拉取 GET /api/v1/billing/api-quota（SYS_ADMIN 可经 tenant_id 查询其他租户）；
2. API 维度：三级用量卡（今日调用/限额/剩余）+ NProgress 用量进度 + 状态 Tag（与后端执法口径一致）；
3. TB-17R 传输维度：同构展示 transport_* 字段（今日连接事件/限额/剩余），
   限额与状态为 broker 执法实际使用的 Redis 缓存口径，展示与执法不脱节；
4. 限额 <=0（remaining=-1 / max_transport_per_day=0）按"不限量"展示，进度条隐藏。
关键注意事项：展示口径复用后端执法判定纯函数的结果（usage_pct/quota_status），
前端不再自行换算，避免展示与 429 执法两套口径；传输维度文案在四语言 custom.json 的
page.billing_api-quota 命名空间（transport* 键，TB-17R 补齐；剩余/不限量/状态复用既有键）。
重构建议：若后续套餐页并入本目录，把用量卡抽成组件复用；传输区块可抽为独立卡片组件。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NButton, NCard, NDescriptions, NDescriptionsItem, NEmpty, NProgress, NTag } from 'naive-ui'
import { getApiQuota, type ApiQuotaReport } from '@/service/api'
import { $t } from '@/locales'

const report = ref<ApiQuotaReport | null>(null)
const loading = ref(false)
const loadError = ref(false)

const unlimited = computed(() => !report.value || report.value.max_api_calls_per_day <= 0)

/** 传输维度"不限量"判定：后端未发布限额缓存（0）或字段缺失（旧版后端兼容）均按不限量展示。 */
const transportUnlimited = computed(() => {
  const limit = Number(report.value?.max_transport_per_day ?? 0)
  return limit <= 0
})

/** 状态 Tag 语义色：与 quota_status 四态一一对应（API 维度）。 */
const statusType = computed(() => quotaStatusType(report.value?.quota_status))

/** 状态 Tag 语义色：与 quota_status 四态一一对应（传输维度）。 */
const transportStatusType = computed(() => quotaStatusType(report.value?.transport_quota_status))

type QuotaSemanticColor = 'default' | 'success' | 'error' | 'warning'

function quotaStatusType(status?: string): QuotaSemanticColor {
  switch (status) {
    case 'exceeded':
      return 'error'
    case 'warning':
      return 'warning'
    case 'unlimited':
      return 'default'
    default:
      return 'success'
  }
}

function quotaStatusText(status?: string) {
  switch (status) {
    case 'exceeded':
      return $t('page.billing_api-quota.status.exceeded')
    case 'warning':
      return $t('page.billing_api-quota.status.warning')
    case 'unlimited':
      return $t('page.billing_api-quota.status.unlimited')
    default:
      return $t('page.billing_api-quota.status.normal')
  }
}

const statusText = computed(() => quotaStatusText(report.value?.quota_status))
const transportStatusText = computed(() => quotaStatusText(report.value?.transport_quota_status))

/** 进度条语义色：exceeded 红 / warning 橙 / 正常主题色（API 维度）。 */
const progressStatus = computed(() => progressColor(report.value?.quota_status))

/** 进度条语义色：传输维度。 */
const transportProgressStatus = computed(() => progressColor(report.value?.transport_quota_status))

function progressColor(status?: string): QuotaSemanticColor {
  switch (status) {
    case 'exceeded':
      return 'error'
    case 'warning':
      return 'warning'
    default:
      return 'success'
  }
}

const fetchQuota = async () => {
  loading.value = true
  loadError.value = false
  try {
    const { data, error } = await getApiQuota()
    if (!error && data) {
      report.value = data
    } else {
      loadError.value = true
    }
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void fetchQuota()
})
</script>

<template>
  <div class="min-h-500px">
    <NCard :title="$t('route.billing_api-quota')" :bordered="false">
      <template #header-extra>
        <NButton type="primary" :loading="loading" @click="fetchQuota">
          {{ $t('page.billing_api-quota.refresh') }}
        </NButton>
      </template>

      <p class="mb-16px text-gray-400">{{ $t('page.billing_api-quota.description') }}</p>

      <NEmpty v-if="loadError" :description="$t('page.billing_api-quota.loadFailed')" />

      <template v-else>
        <div class="grid grid-cols-1 gap-16px md:grid-cols-3">
          <NCard size="small" :bordered="true">
            <div class="text-gray-400">{{ $t('page.billing_api-quota.apiCallsToday') }}</div>
            <div class="mt-8px text-28px font-semibold">{{ report?.api_calls_today ?? 0 }}</div>
          </NCard>
          <NCard size="small" :bordered="true">
            <div class="text-gray-400">{{ $t('page.billing_api-quota.quotaLimit') }}</div>
            <div class="mt-8px text-28px font-semibold">
              <template v-if="unlimited">{{ $t('page.billing_api-quota.unlimited') }}</template>
              <template v-else>{{ report?.max_api_calls_per_day ?? 0 }}</template>
            </div>
          </NCard>
          <NCard size="small" :bordered="true">
            <div class="text-gray-400">{{ $t('page.billing_api-quota.remaining') }}</div>
            <div class="mt-8px text-28px font-semibold">
              <template v-if="unlimited">{{ $t('page.billing_api-quota.unlimited') }}</template>
              <template v-else>{{ report?.remaining ?? 0 }}</template>
            </div>
          </NCard>
        </div>

        <NCard v-if="!unlimited" size="small" :bordered="true" class="mt-16px">
          <div class="mb-8px flex justify-between">
            <span>{{ $t('page.billing_api-quota.usageProgress') }}</span>
            <span>{{ report?.usage_pct ?? 0 }}%</span>
          </div>
          <NProgress
            type="line"
            :percentage="Math.min(report?.usage_pct ?? 0, 100)"
            :status="progressStatus"
            :show-indicator="false"
          />
        </NCard>

        <NDescriptions v-if="report" :column="2" bordered class="mt-16px" label-placement="left">
          <NDescriptionsItem :label="$t('page.billing_api-quota.plan')">
            <NTag size="small">{{ report.plan_code }}</NTag>
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('page.billing_api-quota.meterDate')">
            {{ report.date }}
          </NDescriptionsItem>
          <NDescriptionsItem :label="$t('page.billing_api-quota.usageProgress')">
            <NTag :type="statusType" size="small">{{ statusText }}</NTag>
          </NDescriptionsItem>
        </NDescriptions>

        <!-- TB-17R 传输维度：MQTT 连接认证日配额（展示与 broker 执法同口径）。 -->
        <NCard
          size="small"
          :bordered="true"
          class="mt-16px"
          :title="$t('page.billing_api-quota.transportSectionTitle')"
        >
          <p class="mb-12px text-gray-400">{{ $t('page.billing_api-quota.transportSectionDesc') }}</p>

          <div class="grid grid-cols-1 gap-16px md:grid-cols-3">
            <NCard size="small" :bordered="true">
              <div class="text-gray-400">{{ $t('page.billing_api-quota.transportEventsToday') }}</div>
              <div class="mt-8px text-28px font-semibold">{{ report?.transport_events_today ?? 0 }}</div>
            </NCard>
            <NCard size="small" :bordered="true">
              <div class="text-gray-400">{{ $t('page.billing_api-quota.transportLimit') }}</div>
              <div class="mt-8px text-28px font-semibold">
                <template v-if="transportUnlimited">{{ $t('page.billing_api-quota.unlimited') }}</template>
                <template v-else>{{ report?.max_transport_per_day ?? 0 }}</template>
              </div>
            </NCard>
            <NCard size="small" :bordered="true">
              <div class="text-gray-400">{{ $t('page.billing_api-quota.remaining') }}</div>
              <div class="mt-8px text-28px font-semibold">
                <template v-if="transportUnlimited">{{ $t('page.billing_api-quota.unlimited') }}</template>
                <template v-else>{{ report?.transport_remaining ?? 0 }}</template>
              </div>
            </NCard>
          </div>

          <NCard v-if="!transportUnlimited" size="small" :bordered="true" class="mt-16px">
            <div class="mb-8px flex justify-between">
              <span>{{ $t('page.billing_api-quota.transportUsageProgress') }}</span>
              <span>{{ report?.transport_usage_pct ?? 0 }}%</span>
            </div>
            <NProgress
              type="line"
              :percentage="Math.min(report?.transport_usage_pct ?? 0, 100)"
              :status="transportProgressStatus"
              :show-indicator="false"
            />
          </NCard>

          <div class="mt-12px">
            <NTag :type="transportStatusType" size="small">{{ transportStatusText }}</NTag>
          </div>
        </NCard>
      </template>
    </NCard>
  </div>
</template>

<style scoped></style>
