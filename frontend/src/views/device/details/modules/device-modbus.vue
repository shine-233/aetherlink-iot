<!--
设备 Modbus 点表配置面板（ROADMAP B1 前端寄存器映射界面）。
编辑 target（Modbus TCP 从站）与 registers 点表，保存到平台后由 modbus-plugin 以
OpenAPI Key 拉取生效。凭证（username/password）不在本页管理，仍归插件本地安全存储。
数据边界：
1. 仅设备写权限用户可保存；后端拒绝包含凭证字段的点表。
2. data_type 与 register type 的合法组合以后端/插件 Normalize 校验为准。
-->
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { getModbusProfile, saveModbusProfile } from '@/service/api'
import { $t } from '@/locales'

const props = defineProps<{
  id: string
}>()

interface RegisterRow {
  key: string
  type: string
  address: number
  dataType: string
  multiplier: number
  offset: number
  writable: boolean
}

const loading = ref(false)
const saving = ref(false)
const updatedInfo = ref('')

const target = reactive({
  host: '',
  port: 502,
  unit_id: 1,
  timeout_ms: 3000
})

const registers = reactive<RegisterRow[]>([])

const typeOptions = [
  { label: 'holding', value: 'holding' },
  { label: 'input', value: 'input' },
  { label: 'coil', value: 'coil' },
  { label: 'discrete', value: 'discrete' }
]

function dataTypeOptions(row: RegisterRow) {
  if (row.type === 'coil' || row.type === 'discrete') {
    return [{ label: 'bool', value: 'bool' }]
  }
  return ['u16', 'i16', 'u32', 'i32', 'f32'].map(value => ({ label: value, value }))
}

function addRegister() {
  registers.push({
    key: '',
    type: 'holding',
    address: 0,
    dataType: 'u16',
    multiplier: 1,
    offset: 0,
    writable: false
  })
}

function removeRegister(index: number) {
  registers.splice(index, 1)
}

async function loadProfile() {
  loading.value = true
  try {
    const { data, error } = await getModbusProfile(props.id)
    if (!error && data) {
      const profile = data.profile || {}
      if (profile.target) {
        Object.assign(target, profile.target)
      }
      registers.length = 0
      for (const row of profile.registers || []) {
        registers.push({
          key: String(row.key ?? ''),
          type: String(row.type ?? 'holding'),
          address: Number(row.address ?? 0),
          dataType: String(row.data_type ?? 'u16'),
          multiplier: Number(row.multiplier ?? 1),
          offset: Number(row.offset ?? 0),
          writable: Boolean(row.writable ?? false)
        })
      }
    }
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  const payload = {
    target: {
      host: target.host,
      port: Number(target.port) || 502,
      unit_id: Number(target.unit_id) || 1,
      timeout_ms: Number(target.timeout_ms) || 3000
    },
    registers: registers.map(row => ({
      key: row.key,
      type: row.type,
      address: Number(row.address) || 0,
      data_type: row.dataType,
      multiplier: Number(row.multiplier) || 1,
      offset: Number(row.offset) || 0,
      writable: row.writable
    }))
  }
  saving.value = true
  try {
    const { error } = await saveModbusProfile(props.id, payload)
    if (!error) {
      window.$message?.success($t('common.operationSuccess'))
      await loadProfile()
    }
  } finally {
    saving.value = false
  }
}

onMounted(loadProfile)
</script>

<template>
  <div class="modbus-profile" v-loading="loading">
    <n-alert type="info" :show-icon="false" class="mb-3">
      {{ $t('custom.device_details.modbusIntro') }}
    </n-alert>

    <n-form label-placement="left" label-width="120" inline>
      <n-form-item :label="$t('custom.device_details.modbusTargetHost')">
        <n-input v-model:value="target.host" :placeholder="'192.168.1.50'" style="width: 180px" />
      </n-form-item>
      <n-form-item :label="$t('custom.device_details.modbusTargetPort')">
        <n-input-number v-model:value="target.port" :min="1" :max="65535" style="width: 130px" />
      </n-form-item>
      <n-form-item :label="$t('custom.device_details.modbusUnitId')">
        <n-input-number v-model:value="target.unit_id" :min="0" :max="255" style="width: 110px" />
      </n-form-item>
      <n-form-item :label="$t('custom.device_details.modbusTimeout')">
        <n-input-number v-model:value="target.timeout_ms" :min="100" :step="500" style="width: 140px" />
      </n-form-item>
    </n-form>

    <div class="registers-head">
      <span>{{ $t('custom.device_details.modbusRegisters') }}</span>
      <n-button size="small" type="primary" secondary @click="addRegister">
        {{ $t('custom.device_details.modbusAddRegister') }}
      </n-button>
    </div>

    <n-table v-if="registers.length" size="small" :bordered="false" :single-line="false">
      <thead>
        <tr>
          <th>{{ $t('custom.device_details.modbusKey') }}</th>
          <th>{{ $t('custom.device_details.modbusType') }}</th>
          <th>{{ $t('custom.device_details.modbusAddress') }}</th>
          <th>{{ $t('custom.device_details.modbusDataType') }}</th>
          <th>{{ $t('custom.device_details.modbusMultiplier') }}</th>
          <th>{{ $t('custom.device_details.modbusOffset') }}</th>
          <th>{{ $t('custom.device_details.modbusWritable') }}</th>
          <th>{{ $t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(row, index) in registers" :key="index">
          <td><n-input v-model:value="row.key" size="small" /></td>
          <td>
            <n-select v-model:value="row.type" size="small" :options="typeOptions" style="width: 110px" @update:value="row.dataType = row.type === 'coil' || row.type === 'discrete' ? 'bool' : 'u16'" />
          </td>
          <td><n-input-number v-model:value="row.address" size="small" :min="0" :max="65535" style="width: 110px" /></td>
          <td>
            <n-select v-model:value="row.dataType" size="small" :options="dataTypeOptions(row)" style="width: 90px" />
          </td>
          <td><n-input-number v-model:value="row.multiplier" size="small" :step="0.1" style="width: 110px" /></td>
          <td><n-input-number v-model:value="row.offset" size="small" :step="1" style="width: 100px" /></td>
          <td><n-checkbox v-model:checked="row.writable" /></td>
          <td>
            <n-button size="small" quaternary type="error" @click="removeRegister(index)">
              {{ $t('common.delete') }}
            </n-button>
          </td>
        </tr>
      </tbody>
    </n-table>
    <n-empty v-else :description="$t('custom.device_details.modbusNoRegisters')" />

    <div class="save-bar">
      <n-button type="primary" :loading="saving" @click="handleSave">
        {{ $t('common.save') }}
      </n-button>
      <span v-if="updatedInfo" class="updated-info">{{ updatedInfo }}</span>
    </div>
  </div>
</template>

<style scoped>
.registers-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.save-bar {
  margin-top: 12px;
  display: flex;
  gap: 12px;
  align-items: center;
}
.updated-info {
  color: var(--n-text-color-disabled);
  font-size: 12px;
}
</style>
