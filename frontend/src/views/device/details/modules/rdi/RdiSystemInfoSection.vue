<!--
  文件用途: RDI 系统信息分区（安装/维护/客户信息 + 扩展字段）。
  核心逻辑: 固定字段用声明式列表渲染，label 来源分为 rdi 标签（t）与扩展字段标签（systemExtraFieldLabel）。
  关键注意事项: 字段顺序按客户手册固定，新增字段追加到 SYSTEM_FIELDS 即可。
-->
<script setup lang="ts">
import type { RDISystemInfo } from '@/service/api/rdi'
import type { LabelKey } from './constants/rdi-labels'
import { useRdiOperationsContext } from './composables/useRdiOperationsContext'

const { t, config: configState } = useRdiOperationsContext()
const { systemInfo, systemExtraFieldLabel, systemExtraFieldDefinitions, getSystemExtraField, setSystemExtraField } =
  configState

type SystemField = { key: keyof RDISystemInfo & string; label?: LabelKey }

const SYSTEM_FIELDS: SystemField[] = [
  { key: 'installation_location', label: 'location' },
  { key: 'address' },
  { key: 'installation_date' },
  { key: 'installer_company' },
  { key: 'installer_contact' },
  { key: 'installer_name' },
  { key: 'installer_phone' },
  { key: 'installer_email' },
  { key: 'controller_serial_number' },
  { key: 'maintenance_technician', label: 'technician' },
  { key: 'customer_name', label: 'customer' },
  { key: 'contact_email', label: 'email' },
  { key: 'contact_phone', label: 'phone' },
  { key: 'warranty_status', label: 'warranty' }
]

function fieldLabel(field: SystemField) {
  return field.label ? t(field.label) : systemExtraFieldLabel(field.key)
}
</script>

<template>
  <section class="rdi-section">
    <div class="rdi-section-title">{{ t('system') }}</div>
    <div class="rdi-grid rdi-grid--three">
      <NFormItem v-for="field in SYSTEM_FIELDS" :key="field.key" :label="fieldLabel(field)">
        <NInput v-model:value="systemInfo[field.key] as string | undefined" />
      </NFormItem>
    </div>
    <div class="rdi-fieldset rdi-system-extra">
      <div class="rdi-fieldset-title">{{ t('extendedFields') }}</div>
      <div class="rdi-grid rdi-grid--three">
        <NFormItem
          v-for="field in systemExtraFieldDefinitions"
          :key="field.key"
          :label="systemExtraFieldLabel(field.key)"
        >
          <NInput
            :value="getSystemExtraField(field.key)"
            @update:value="(value) => setSystemExtraField(field.key, value)"
          />
        </NFormItem>
      </div>
    </div>
  </section>
</template>

<style scoped src="./rdi-section.css"></style>

<style scoped>
.rdi-system-extra {
  margin-top: 12px;
}
</style>
