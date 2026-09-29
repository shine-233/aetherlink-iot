<!--
  文件用途: RDI 告警分区：两个温度传感器 + 两个开关量输入的告警配置。
  核心逻辑: 传感器复用 RdiSensorAlarmFieldset；开关量按序号映射 switch_N_alarm_mode / switch_N_alarm_duration。
  关键注意事项: “当前状态”取实时 telemetry.switch_N，经 formatSwitch 本地化。
-->
<script setup lang="ts">
import { useRdiOperationsContext } from './composables/useRdiOperationsContext'
import RdiDurationControl from './RdiDurationControl.vue'
import RdiSensorAlarmFieldset from './RdiSensorAlarmFieldset.vue'

const { t, config: configState, telemetry: telemetryState, history, options } = useRdiOperationsContext()
const { config } = configState
const { telemetry, formatSwitch } = telemetryState
const { formatDurationLabel, RDI_DURATION_MAX_SECONDS } = history

const SWITCHES = [
  {
    title: 'switch1',
    mode: 'switch_1_alarm_mode',
    duration: 'switch_1_alarm_duration',
    telemetry: 'switch_1'
  },
  {
    title: 'switch2',
    mode: 'switch_2_alarm_mode',
    duration: 'switch_2_alarm_duration',
    telemetry: 'switch_2'
  }
] as const
</script>

<template>
  <section class="rdi-section">
    <div class="rdi-section-title">{{ t('alarm') }}</div>
    <div class="rdi-grid rdi-grid--two">
      <RdiSensorAlarmFieldset :sensor="1" />
      <RdiSensorAlarmFieldset :sensor="2" />

      <div v-for="item in SWITCHES" :key="item.title" class="rdi-fieldset">
        <div class="rdi-fieldset-title">{{ t(item.title) }}</div>
        <NFormItem :label="t('switchAlarmLevel')">
          <NSelect v-model:value="config[item.mode]" :options="options.switchMode.value" />
        </NFormItem>
        <RdiDurationControl
          v-model="config[item.duration]"
          :label="`${t('triggerEffectiveTime')} (s)`"
          :format-label="formatDurationLabel"
          :max="RDI_DURATION_MAX_SECONDS"
        />
        <NFormItem :label="t('currentStatus')">
          <div class="rdi-status-value">{{ formatSwitch(telemetry[item.telemetry]) }}</div>
        </NFormItem>
      </div>
    </div>
  </section>
</template>

<style scoped src="./rdi-section.css"></style>
