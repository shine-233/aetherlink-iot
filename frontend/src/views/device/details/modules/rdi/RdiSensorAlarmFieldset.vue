<!--
  文件用途: 单个温度传感器（sensor 1 / sensor 2）告警配置 fieldset。
  核心逻辑: 按 sensor 序号映射 config 字段（alarm_sensor_N_enabled / sensor_N_lower / upper / duration），
  替代原视图中两份逐字重复的模板。
  关键注意事项: 温度范围固定 -40..125，与 backend RDI 配置校验保持一致。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { useRdiOperationsContext } from './composables/useRdiOperationsContext'
import RdiDurationControl from './RdiDurationControl.vue'

const RdiTemperatureAlarmAxis = defineAsyncComponent(() => import('../RdiTemperatureAlarmAxis.vue'))

const props = defineProps<{
  sensor: 1 | 2
}>()

const TEMPERATURE_MIN = -40
const TEMPERATURE_MAX = 125

const { t, config: configState, telemetry: telemetryState, history } = useRdiOperationsContext()
const { config } = configState
const { telemetry, temperatureUnit, toAxisValue } = telemetryState
const { formatDurationLabel, RDI_DURATION_MAX_SECONDS } = history

const keys = computed(() =>
  props.sensor === 1
    ? ({
        enabled: 'alarm_sensor_1_enabled',
        lower: 'sensor_1_lower',
        upper: 'sensor_1_upper',
        duration: 'sensor_1_duration',
        telemetry: 'temperature_1'
      } as const)
    : ({
        enabled: 'alarm_sensor_2_enabled',
        lower: 'sensor_2_lower',
        upper: 'sensor_2_upper',
        duration: 'sensor_2_duration',
        telemetry: 'temperature_2'
      } as const)
)

const range = props.sensor === 1 ? configState.sensor1Range : configState.sensor2Range
</script>

<template>
  <div class="rdi-fieldset">
    <div class="rdi-fieldset-title">{{ t(sensor === 1 ? 'sensor1' : 'sensor2') }}</div>
    <NFormItem :label="t('enabled')">
      <NSwitch v-model:value="config[keys.enabled]" />
    </NFormItem>
    <NSlider v-model:value="range" range :min="TEMPERATURE_MIN" :max="TEMPERATURE_MAX" />
    <RdiTemperatureAlarmAxis
      v-if="config[keys.enabled]"
      v-model:lower="config[keys.lower]"
      v-model:upper="config[keys.upper]"
      :current="toAxisValue(telemetry[keys.telemetry])"
      :unit="temperatureUnit"
      :lower-label="t('lower')"
      :upper-label="t('upper')"
      :current-label="t('currentValue')"
    />
    <div class="rdi-inline">
      <NFormItem :label="t('lower')">
        <NInputNumber v-model:value="config[keys.lower]" :min="TEMPERATURE_MIN" :max="TEMPERATURE_MAX" />
      </NFormItem>
      <NFormItem :label="t('upper')">
        <NInputNumber v-model:value="config[keys.upper]" :min="TEMPERATURE_MIN" :max="TEMPERATURE_MAX" />
      </NFormItem>
      <RdiDurationControl
        v-model="config[keys.duration]"
        :label="`${t('duration')} (s)`"
        :format-label="formatDurationLabel"
        :max="RDI_DURATION_MAX_SECONDS"
      />
    </div>
  </div>
</template>

<style scoped src="./rdi-section.css"></style>

<style scoped>
.rdi-inline {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(100px, 1fr));
  gap: 10px;
  margin-top: 10px;
}
</style>
