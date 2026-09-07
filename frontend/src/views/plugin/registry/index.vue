<!--
文件用途：插件框架 gRPC 网关管理页（PHASE-D-D9）。
核心逻辑：插件登记列表 + 创建（一次性凭证展示）+ 启停/凭证轮换/删除 + 下行命令测试发送。
关键注意事项：token 明文只在创建/轮换响应出现一次，页面以弹窗呈现并要求用户自行保存。
-->
<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, NSpace, NTag } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  createPluginRegistry,
  deletePluginRegistry,
  listPluginRegistries,
  rotatePluginToken,
  sendPluginDownlink,
  setPluginRegistryEnabled,
  type PluginRegistryRow
} from '@/service/api'
import { $t } from '@/locales'

defineOptions({ name: 'PluginRegistry' })

const rows = ref<PluginRegistryRow[]>([])
const loading = ref(false)

const columns: DataTableColumns<PluginRegistryRow> = [
  { title: () => $t('page.pluginRegistry.name'), key: 'name' },
  { title: () => $t('page.pluginRegistry.version'), key: 'version' },
  { title: () => $t('page.pluginRegistry.transport'), key: 'transport' },
  {
    title: () => $t('page.pluginRegistry.status'),
    key: 'status',
    render: row => h(NTag, { type: statusType(row.status), size: 'small' }, { default: () => row.status })
  },
  { title: () => $t('page.pluginRegistry.heartbeat'), key: 'last_heartbeat' },
  {
    title: () => $t('generate.operation'),
    key: 'actions',
    render: row =>
      h(NSpace, { size: 6 }, {
        default: () => [
          h(NButton, { size: 'tiny', onClick: () => handleToggle(row, row.status !== 'online') }, { default: () => (row.status === 'disabled' ? $t('page.pluginRegistry.enable') : $t('page.pluginRegistry.disable')) }),
          h(NButton, { size: 'tiny', onClick: () => handleRotate(row) }, { default: () => $t('page.pluginRegistry.rotate') }),
          h(NButton, { size: 'tiny', onClick: () => openDownlink(row) }, { default: () => $t('page.pluginRegistry.downlink') }),
          h(NButton, { size: 'tiny', type: 'error', onClick: () => handleDelete(row) }, { default: () => $t('generate.delete') })
        ]
      })
  }
]

const createVisible = ref(false)
const createForm = ref({ name: '', version: '', description: '' })
const createdToken = ref('')
const createdTokenVisible = ref(false)

const downlinkVisible = ref(false)
const downlinkTarget = ref('')
const downlinkForm = ref({ device_number: '', identify: '', params: '{}' })

async function loadRows() {
  loading.value = true
  try {
    const { data, error } = await listPluginRegistries()
    if (!error && Array.isArray(data)) rows.value = data
  } finally {
    loading.value = false
  }
}

async function handleCreate() {
  if (!createForm.value.name.trim()) {
    window.$message?.error($t('page.pluginRegistry.nameRequired'))
    return
  }
  const { data, error } = await createPluginRegistry(createForm.value)
  if (error) return
  createdToken.value = (data as unknown as { token: string })?.token || ''
  createdTokenVisible.value = true
  createVisible.value = false
  createForm.value = { name: '', version: '', description: '' }
  await loadRows()
}

async function handleToggle(row: PluginRegistryRow, enabled: boolean) {
  const { error } = await setPluginRegistryEnabled(row.id, enabled)
  if (!error) await loadRows()
}

async function handleRotate(row: PluginRegistryRow) {
  const { data, error } = await rotatePluginToken(row.id)
  if (error) return
  createdToken.value = (data as unknown as { token: string })?.token || ''
  createdTokenVisible.value = true
}

async function handleDelete(row: PluginRegistryRow) {
  const { error } = await deletePluginRegistry(row.id)
  if (!error) await loadRows()
}

function openDownlink(row: PluginRegistryRow) {
  downlinkTarget.value = row.id
  downlinkForm.value = { device_number: '', identify: '', params: '{}' }
  downlinkVisible.value = true
}

async function handleSendDownlink() {
  let params: Record<string, unknown> = {}
  try {
    params = JSON.parse(downlinkForm.value.params || '{}')
  } catch {
    window.$message?.error($t('custom.rule_chain.invalidJson'))
    return
  }
  const { error } = await sendPluginDownlink(downlinkTarget.value, {
    device_number: downlinkForm.value.device_number,
    identify: downlinkForm.value.identify,
    params
  })
  if (!error) {
    window.$message?.success($t('common.operationSuccess'))
    downlinkVisible.value = false
  }
}

function statusType(status: string) {
  if (status === 'online') return 'success'
  if (status === 'disabled') return 'error'
  return 'info'
}

onMounted(loadRows)
</script>

<template>
  <div class="min-h-full bg-gray-50 p-4 dark:bg-[#101014]">
    <n-card :bordered="false" :title="$t('page.pluginRegistry.title')" class="rounded-8px">
      <template #header-extra>
        <n-button type="primary" @click="createVisible = true">{{ $t('page.pluginRegistry.create') }}</n-button>
      </template>
      <n-data-table :columns="columns" :data="rows" :loading="loading" :bordered="false" />
    </n-card>

    <n-modal v-model:show="createVisible" preset="card" :title="$t('page.pluginRegistry.create')" style="width: 480px">
      <n-form label-placement="top">
        <n-form-item :label="$t('page.pluginRegistry.name')">
          <n-input v-model:value="createForm.name" placeholder="modbus-tcp" />
        </n-form-item>
        <n-form-item :label="$t('page.pluginRegistry.version')">
          <n-input v-model:value="createForm.version" placeholder="1.0.0" />
        </n-form-item>
        <n-form-item :label="$t('page.pluginRegistry.description')">
          <n-input v-model:value="createForm.description" type="textarea" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="createVisible = false">{{ $t('generate.cancel') }}</n-button>
          <n-button type="primary" @click="handleCreate">{{ $t('common.save') }}</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal v-model:show="createdTokenVisible" preset="card" :title="$t('page.pluginRegistry.tokenTitle')" style="width: 520px">
      <n-alert type="warning" :show-icon="true" class="mb-3">{{ $t('page.pluginRegistry.tokenOnce') }}</n-alert>
      <n-input :value="createdToken" readonly type="textarea" />
      <template #footer>
        <n-space justify="end">
          <n-button @click="createdTokenVisible = false">{{ $t('generate.close') }}</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal v-model:show="downlinkVisible" preset="card" :title="$t('page.pluginRegistry.downlinkTitle')" style="width: 480px">
      <n-form label-placement="top">
        <n-form-item :label="$t('page.pluginRegistry.deviceNumber')">
          <n-input v-model:value="downlinkForm.device_number" />
        </n-form-item>
        <n-form-item :label="$t('page.pluginRegistry.identify')">
          <n-input v-model:value="downlinkForm.identify" />
        </n-form-item>
        <n-form-item :label="$t('page.pluginRegistry.params')">
          <n-input v-model:value="downlinkForm.params" type="textarea" :autosize="{ minRows: 2, maxRows: 6 }" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="downlinkVisible = false">{{ $t('generate.cancel') }}</n-button>
          <n-button type="primary" @click="handleSendDownlink">{{ $t('page.pluginRegistry.send') }}</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>
