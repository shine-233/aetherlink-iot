<!--
  文件用途: RDI 干接点输出配置分区。
  核心逻辑: 告警/恢复延时相同时折叠成一个“触发生效时间”控件（写入时同步两个字段），不同则分别展示。
  关键注意事项: 折叠判断与写入逻辑来自 useRdiOperationsContext 的 dryContact 纯函数，已单测覆盖。
-->
<script setup lang="ts">
import { useRdiOperationsContext } from './composables/useRdiOperationsContext'
import RdiDurationControl from './RdiDurationControl.vue'

const { t, config: configState, telemetry: telemetryState, history, options, dryContact } = useRdiOperationsContext()
const { config } = configState
const { telemetry, formatSwitch } = telemetryState
const { formatDurationLabel, RDI_DURATION_MAX_SECONDS } = history
const { hasDistinctDelays, unifiedDelay } = dryContact
</script>

<template>
  <section class="rdi-section">
    <div class="rdi-section-title">{{ t('dryContact') }}</div>
    <div class="rdi-grid rdi-grid--four">
      <NFormItem :label="t('alarmLevel')">
        <NSelect v-model:value="config.dry_contact_alarm_level" :options="options.level.value" />
      </NFormItem>
      <NFormItem :label="t('normalLevel')">
        <NSelect v-model:value="config.dry_contact_normal_level" :options="options.level.value" />
      </NFormItem>
      <RdiDurationControl
        v-if="!hasDistinctDelays"
        v-model="unifiedDelay"
        :label="`${t('triggerEffectiveTime')} (s)`"
        :format-label="formatDurationLabel"
        :max="RDI_DURATION_MAX_SECONDS"
      />
      <template v-else>
        <RdiDurationControl
          v-model="config.dry_contact_alarm_delay"
          :label="`${t('alarmDelay')} (s)`"
          :format-label="formatDurationLabel"
          :max="RDI_DURATION_MAX_SECONDS"
        />
        <RdiDurationControl
          v-model="config.dry_contact_normal_delay"
          :label="`${t('normalDelay')} (s)`"
          :format-label="formatDurationLabel"
          :max="RDI_DURATION_MAX_SECONDS"
        />
      </template>
      <NFormItem :label="t('currentStatus')">
        <div class="rdi-status-value">{{ formatSwitch(telemetry.dry_contact_output) }}</div>
      </NFormItem>
    </div>
  </section>
</template>

<style scoped src="./rdi-section.css"></style>
