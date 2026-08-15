<!--
  文件用途：RDI Field Setting 只读展示面板（REQ-53），列出 n00-n07 与 sw1-sw4 的原始值与解析值。
  核心逻辑：复用 useRdiConfig 加载已保存配置，raw 取 config.field_setting 原始内容，解析值取 getFieldValue（与写入侧同一套解析规则）。
  关键注意事项：
  1. 该面板只读：没有输入框、没有保存、没有命令下发入口。
  2. 12 个字段全部为空时展示统一空状态文案，不渲染只有 '--' 的空表格。
  3. 字段键顺序固定，便于客户按固件文档逐项比对。
-->
<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRdiConfig } from './composables/useRdiConfig'

defineOptions({
  name: 'RdiFieldSettingsPanel'
})

const props = defineProps<{
  id: string
}>()

const READONLY_FIELD_SETTING_KEYS = [
  'n00',
  'n01',
  'n02',
  'n03',
  'n04',
  'n05',
  'n06',
  'n07',
  'sw1',
  'sw2',
  'sw3',
  'sw4'
] as const

const EMPTY_CELL = '--'

const { loading, config, t, getFieldValue, loadConfig } = useRdiConfig(
  () => props.id,
  () => {}
)

function stringifyRawFieldValue(value: unknown) {
  if (value === undefined || value === null) return EMPTY_CELL
  if (typeof value === 'string') return value.trim() || EMPTY_CELL
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

const fieldSettingRows = computed(() =>
  READONLY_FIELD_SETTING_KEYS.map((key) => {
    const raw = (config.field_setting as Record<string, unknown> | undefined)?.[key]
    const interpreted = getFieldValue(key)
    return {
      key,
      raw: stringifyRawFieldValue(raw),
      interpreted: interpreted && interpreted.trim() ? interpreted : EMPTY_CELL
    }
  })
)

const hasFieldSettingValues = computed(() =>
  fieldSettingRows.value.some((row) => row.raw !== EMPTY_CELL || row.interpreted !== EMPTY_CELL)
)

onMounted(() => {
  void loadConfig()
})

watch(
  () => props.id,
  (nextId, previousId) => {
    if (nextId === previousId) return
    if (nextId) void loadConfig()
  }
)
</script>

<template>
  <section class="rdi-field-settings-panel" data-testid="rdi-field-settings-panel">
    <h3 class="rdi-field-settings-title">{{ t('fieldSettingsReadonlyTitle') }}</h3>

    <NSpin :show="loading">
      <table v-if="hasFieldSettingValues" class="rdi-field-settings-table">
        <caption class="rdi-field-settings-caption">
          {{ t('fieldSettingsReadonlyTitle') }}
        </caption>
        <thead>
          <tr>
            <th scope="col">{{ t('fieldSettingsFieldColumn') }}</th>
            <th scope="col">{{ t('fieldSettingsRawValueColumn') }}</th>
            <th scope="col">{{ t('fieldSettingsInterpretedValueColumn') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in fieldSettingRows" :key="row.key" :data-field-row="row.key">
            <th scope="row">{{ row.key }}</th>
            <td>{{ row.raw }}</td>
            <td>{{ row.interpreted }}</td>
          </tr>
        </tbody>
      </table>
      <NEmpty v-else :description="t('empty')" class="rdi-field-settings-empty" />
    </NSpin>
  </section>
</template>

<style scoped>
.rdi-field-settings-panel {
  padding: 8px 0 16px;
}

.rdi-field-settings-title {
  margin: 0 0 12px;
  font-size: 15px;
  font-weight: 600;
}

.rdi-field-settings-caption {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}

.rdi-field-settings-table {
  width: 100%;
  border-collapse: collapse;
}

.rdi-field-settings-table th,
.rdi-field-settings-table td {
  border: 1px solid #e5e7eb;
  padding: 8px 10px;
  text-align: left;
  font-size: 13px;
}

.rdi-field-settings-table thead th {
  color: #667085;
  font-weight: 600;
}

.rdi-field-settings-table tbody th {
  width: 20%;
  font-weight: 600;
}

.rdi-field-settings-empty {
  padding: 24px 0;
}
</style>
