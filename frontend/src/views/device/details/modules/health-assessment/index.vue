<!--
文件用途：Device Health Assessment（对标 ThingsBoard PE / ThingsPanel 算法中心）设备健康度评估卡片组件。
核心逻辑：
1. 展示单设备的综合健康得分（0~100）与状态评级（健康/亚健康/告警/危险）；
2. 维度扣分分析（告警扣分、离线时长扣分、异常遥测扣分）；
3. TP-21 MSET 多元状态估计特征维度（后端开关开启时返回 mset 字段）：偏差分/马氏距离/特征键展示，
   降级态（冷启动/样本不足/奇异矩阵/取数失败）如实展示原因，不伪装成"正常"；
4. 智能运维建议与即时重新评估。
-->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NGrid,
  NGridItem,
  NProgress,
  NSpace,
  NSpin,
  NStatistic,
  NTag,
  useMessage
} from 'naive-ui'
import {
  evaluateDeviceHealth,
  getDeviceHealthDetail,
  type DeviceHealthDetailResponse,
  type DeviceHealthMSETFeature,
  type HealthStatus
} from '@/service/api/device-health'
import { formatDateTime } from '@/utils/common/datetime'

const props = defineProps<{
  deviceId: string
}>()

const message = useMessage()
const loading = ref(false)
const evaluating = ref(false)
const healthData = ref<DeviceHealthDetailResponse | null>(null)

const getStatusColor = (status?: HealthStatus) => {
  switch (status) {
    case 'HEALTHY':
      return { type: 'success' as const, label: '健康 (Healthy)', color: '#18a058' }
    case 'SUB_HEALTHY':
      return { type: 'info' as const, label: '亚健康 (Sub-Healthy)', color: '#2080f0' }
    case 'WARNING':
      return { type: 'warning' as const, label: '异常告警 (Warning)', color: '#f0a020' }
    case 'CRITICAL':
      return { type: 'error' as const, label: '严重危险 (Critical)', color: '#d03050' }
    default:
      return { type: 'default' as const, label: '未知', color: '#909399' }
  }
}

// MSET 偏差分配色：贴合基线绿、中度黄、显著偏离红（与后端 ≥70 记建议的阈值对齐）。
const getMSETDeviationColor = (score: number) => {
  if (score >= 70) return '#d03050'
  if (score >= 30) return '#f0a020'
  return '#18a058'
}

const getMSETDegradeReasonText = (reason?: string) => {
  switch (reason) {
    case 'cold_start':
      return '冷启动：训练窗口内没有完整的历史样本'
    case 'insufficient_samples':
      return '历史样本不足：完整样本数低于训练下限'
    case 'singular_matrix':
      return '特征协方差奇异：特征共线或零方差，无法求逆'
    case 'invalid_sample':
      return '推理样本非法（缺失或非数值）'
    case 'invalid_feature_config':
      return '特征键配置不合法'
    case 'no_feature_keys':
      return '设备无可用的数值遥测特征键'
    case 'history_fetch_failed':
      return '训练窗口历史数据取数失败'
    default:
      return reason || '未知原因'
  }
}

const msetFeature = computed<DeviceHealthMSETFeature | null>(() => healthData.value?.mset ?? null)

const loadHealth = async () => {
  if (!props.deviceId) return
  loading.value = true
  try {
    const res = await getDeviceHealthDetail(props.deviceId)
    if (res.data) {
      healthData.value = res.data
    }
  } catch (err: any) {
    message.error(err.message || '加载设备健康诊断失败')
  } finally {
    loading.value = false
  }
}

const handleReevaluate = async () => {
  if (!props.deviceId) return
  evaluating.value = true
  try {
    const res = await evaluateDeviceHealth(props.deviceId)
    if (res.data) {
      message.success(`健康度评估完成，最新得分: ${res.data.score}`)
      loadHealth()
    }
  } catch (err: any) {
    message.error(err.message || '健康度评估触发失败')
  } finally {
    evaluating.value = false
  }
}

watch(
  () => props.deviceId,
  () => {
    loadHealth()
  }
)

onMounted(() => {
  loadHealth()
})
</script>

<template>
  <NCard title="设备健康度诊断 (Device Health Assessment)" size="small">
    <template #header-extra>
      <NButton size="small" type="primary" secondary :loading="evaluating" @click="handleReevaluate">
        即时重新评估
      </NButton>
    </template>

    <NSpin :show="loading">
      <div v-if="healthData" class="space-y-4">
        <!-- 综合得分大指标 -->
        <div class="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
          <div class="flex items-center gap-6">
            <NProgress
              type="circle"
              :percentage="Math.round(healthData.score)"
              :color="getStatusColor(healthData.health_status).color"
              :stroke-width="10"
              style="width: 100px"
            >
              <div class="text-center">
                <span class="text-2xl font-bold">{{ Math.round(healthData.score) }}</span>
                <div class="text-xs text-gray-500">综合得分</div>
              </div>
            </NProgress>

            <div>
              <div class="flex items-center gap-2 mb-1">
                <span class="text-lg font-semibold">{{ healthData.device_name }}</span>
                <NTag :type="getStatusColor(healthData.health_status).type" size="small" round>
                  {{ getStatusColor(healthData.health_status).label }}
                </NTag>
              </div>
              <div class="text-xs text-gray-500">设备编号: {{ healthData.device_number || '-' }}</div>
              <div class="text-xs text-gray-400 mt-1">
                评估时间: {{ healthData.evaluated_at ? formatDateTime(healthData.evaluated_at) : '-' }}
              </div>
            </div>
          </div>

          <div class="flex items-center gap-6 text-right">
            <NStatistic label="在线状态">
              <span :class="healthData.is_online ? 'text-green-600 font-bold' : 'text-red-500 font-bold'">
                {{ healthData.is_online ? '在线 (Online)' : '离线 (Offline)' }}
              </span>
            </NStatistic>
            <NStatistic label="当前未消告警" :value="healthData.active_alarm_count" />
          </div>
        </div>

        <!-- 维度扣分分析 -->
        <NGrid cols="3" x-gap="12">
          <NGridItem>
            <NCard size="small" embedded title="告警扣分" class="text-center">
              <div class="text-xl font-bold text-orange-500">-{{ healthData.alarm_penalty.toFixed(1) }} 分</div>
              <div class="text-xs text-gray-500 mt-1">未恢复或严重告警累加扣分</div>
            </NCard>
          </NGridItem>
          <NGridItem>
            <NCard size="small" embedded title="通信离线扣分" class="text-center">
              <div class="text-xl font-bold text-red-500">-{{ healthData.offline_penalty.toFixed(1) }} 分</div>
              <div class="text-xs text-gray-500 mt-1">离线时长与心跳超时加权</div>
            </NCard>
          </NGridItem>
          <NGridItem>
            <NCard size="small" embedded title="异常遥测扣分" class="text-center">
              <div class="text-xl font-bold text-yellow-600">-{{ healthData.anomaly_penalty.toFixed(1) }} 分</div>
              <div class="text-xs text-gray-500 mt-1">偏离基线与影子同步延迟</div>
            </NCard>
          </NGridItem>
        </NGrid>

        <!-- TP-21 MSET 多元状态估计特征维度：后端开关（health.mset.enabled，默认关）开启时才返回 mset 字段 -->
        <NCard v-if="msetFeature" size="small" embedded>
          <template #header>
            <div class="flex items-center gap-2">
              <span class="text-sm font-semibold">MSET 多元状态估计 (Multivariate State Estimation)</span>
              <NTag v-if="msetFeature.applied" type="primary" size="small" round>有效推理</NTag>
              <NTag v-else type="warning" size="small" round>已降级（未参与评分）</NTag>
            </div>
          </template>

          <!-- 降级态：如实展示原因，偏差分为中性 0、不扣分，避免"未生效"被误读成"正常" -->
          <NAlert v-if="msetFeature.degraded" type="warning" size="small" :show-icon="true">
            {{ getMSETDegradeReasonText(msetFeature.degrade_reason) }}，本次评分未包含多元偏差扣分（中性处理）。
          </NAlert>

          <!-- 有效推理态：偏差分 + 马氏距离 + 训练样本 + 特征键 -->
          <template v-else>
            <NGrid cols="3" x-gap="12">
              <NGridItem>
                <div class="text-center">
                  <NProgress
                    type="line"
                    :percentage="Math.min(100, Math.round(msetFeature.deviation_score))"
                    :color="getMSETDeviationColor(msetFeature.deviation_score)"
                    :show-indicator="false"
                    style="width: 90%"
                  />
                  <div
                    class="text-lg font-bold mt-1"
                    :style="{ color: getMSETDeviationColor(msetFeature.deviation_score) }"
                  >
                    偏差分 {{ msetFeature.deviation_score.toFixed(1) }}
                  </div>
                  <div class="text-xs text-gray-500 mt-1">相对训练基线的多元偏离程度（0~100）</div>
                </div>
              </NGridItem>
              <NGridItem>
                <NStatistic label="马氏距离" :value="msetFeature.mahalanobis ?? 0" />
                <div class="text-xs text-gray-500 mt-1">距历史基线中心的统计距离</div>
              </NGridItem>
              <NGridItem>
                <NStatistic label="训练样本数" :value="msetFeature.train_samples ?? 0" />
                <div class="text-xs text-gray-500 mt-1">参与建模的完整历史样本行</div>
              </NGridItem>
            </NGrid>
            <div
              v-if="msetFeature.feature_keys && msetFeature.feature_keys.length"
              class="mt-2 flex items-center gap-2"
            >
              <span class="text-xs text-gray-500">特征键:</span>
              <NSpace :size="4" wrap>
                <NTag v-for="key in msetFeature.feature_keys" :key="key" size="small" :bordered="false">
                  {{ key }}
                </NTag>
              </NSpace>
            </div>
            <div class="text-xs text-gray-400 mt-2">
              偏差分 × 权重已折入「异常遥测扣分」（本次 -{{ msetFeature.penalty.toFixed(1) }} 分）
            </div>
          </template>
        </NCard>

        <!-- 运维建议 -->
        <div v-if="healthData.suggestions && healthData.suggestions.length">
          <div class="text-sm font-semibold mb-2">智能运维建议</div>
          <NSpace vertical :size="8">
            <NAlert
              v-for="(suggestion, index) in healthData.suggestions"
              :key="index"
              type="warning"
              size="small"
              :show-icon="true"
            >
              {{ suggestion }}
            </NAlert>
          </NSpace>
        </div>
      </div>
      <div v-else class="text-center py-8 text-gray-400">
        暂无该设备的健康度评估记录，点击右上角「即时重新评估」进行首次体检。
      </div>
    </NSpin>
  </NCard>
</template>
