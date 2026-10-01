<!--
  文件用途: RDI 通知配置分区（开关 + 采集间隔 + 各类告警收件邮箱）。
  核心逻辑: 开关与邮箱字段用声明式列表渲染，替代原视图中 12 段手写 NFormItem。
  关键注意事项: 采集间隔限定 45..60 秒；邮箱为空时后端回落到系统设置中的全局告警收件人。
-->
<script setup lang="ts">
import { useRdiOperationsContext } from './composables/useRdiOperationsContext'

const { t, config: configState } = useRdiOperationsContext()
const { config } = configState

const SWITCH_FIELDS = [
  { label: 'enabled', key: 'notification_enabled' },
  { label: 'temperatureAlarmNotice', key: 'notification_temperature_alarm' },
  { label: 'switchAlarmNotice', key: 'notification_switch_alarm' },
  { label: 'warrantyAlarmNotice', key: 'notification_warranty_alarm' }
] as const

const EMAIL_FIELDS = [
  { label: 'temperatureMail', key: 'sensor_alarm_emails' },
  { label: 'switchMail', key: 'switch_alarm_emails' },
  { label: 'warrantyMail', key: 'warranty_alarm_emails' },
  { label: 'sensor1Mail', key: 'sensor_1_alarm_emails' },
  { label: 'sensor2Mail', key: 'sensor_2_alarm_emails' },
  { label: 'switch1Mail', key: 'switch_1_alarm_emails' },
  { label: 'switch2Mail', key: 'switch_2_alarm_emails' }
] as const
</script>

<template>
  <section class="rdi-section">
    <div class="rdi-section-title">{{ t('notification') }}</div>
    <NAlert type="info" class="rdi-notification-hint" :show-icon="false">
      {{ t('notificationFallbackHint') }}
    </NAlert>
    <div class="rdi-grid rdi-grid--two">
      <NFormItem v-for="field in SWITCH_FIELDS" :key="field.key" :label="t(field.label)">
        <NSwitch v-model:value="config[field.key]" />
      </NFormItem>
      <NFormItem :label="t('interval')">
        <NInputNumber v-model:value="config.data_collection_interval" :min="45" :max="60" />
      </NFormItem>
      <NFormItem v-for="field in EMAIL_FIELDS" :key="field.key" :label="t(field.label)">
        <NInput v-model:value="config[field.key]" />
      </NFormItem>
    </div>
  </section>
</template>

<style scoped src="./rdi-section.css"></style>
