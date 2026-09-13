<!--
文件用途：P1.5 边缘节点管理页——注册表列表、健康状态、注册与心跳操作。
核心逻辑：列表来自 /edge/nodes（服务端已逐节点给出健康分类）；注册走幂等语义
（同租户重注册即更新）；心跳触发 last_seen 刷新。
关键注意事项：
1. 健康为 unknown/offline 时不提供 Reconcile 入口的自动放行语义——编排请求仍可发起
  （服务端闸门会 fail closed），但 UI 上 offline 节点不展示"同步编排"按钮，避免误导。
2. 注册是幂等操作：重复提交同 node_id 属正常运维路径，不是错误。
-->
<script setup lang="tsx">
import { onMounted, reactive, ref } from 'vue'
import { NButton, NCard, NForm, NFormItem, NInput, NModal, NPopconfirm, NTag } from 'naive-ui'
import type { DataTableColumns, FormRules } from 'naive-ui'
import { useLoading } from '@aetherlink/hooks'
import { fetchEdgeNodes, heartbeatEdgeNode, registerEdgeNode, type EdgeNodeEntry } from '@/service/api'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'

const { loading, startLoading, endLoading } = useLoading(false)
const nodes = ref<EdgeNodeEntry[]>([])
const registerVisible = ref(false)
const submitting = ref(false)

const registerForm = reactive({
  node_id: '',
  version: '',
  capabilities: ''
})

const registerRules: FormRules = {
  node_id: { required: true, message: $t('page.edgeNodes.nodeIdRequired'), trigger: 'blur' },
  version: { required: true, message: $t('page.edgeNodes.versionRequired'), trigger: 'blur' }
}

const healthType: Record<EdgeNodeEntry['health'], 'success' | 'warning' | 'error' | 'default'> = {
  online: 'success',
  degraded: 'warning',
  offline: 'error',
  unknown: 'default'
}

const columns: DataTableColumns<EdgeNodeEntry> = [
  { title: $t('page.edgeNodes.nodeId'), key: 'id', width: 220, ellipsis: { tooltip: true } },
  { title: $t('page.edgeNodes.version'), key: 'version', width: 100 },
  {
    title: $t('page.edgeNodes.health'),
    key: 'health',
    width: 110,
    render: row => (
      <NTag type={healthType[row.health] ?? 'default'} size="small">
        {row.health}
      </NTag>
    )
  },
  {
    title: $t('page.edgeNodes.capabilities'),
    key: 'capabilities',
    render: row => (row.capabilities?.length ? row.capabilities.join(', ') : '-')
  },
  {
    title: $t('page.edgeNodes.lastSeen'),
    key: 'last_seen_at',
    width: 180,
    render: row => (row.last_seen_at ? formatDateTime(row.last_seen_at) : '-')
  },
  {
    title: $t('page.edgeNodes.actions'),
    key: 'actions',
    width: 150,
    render: row =>
      row.status === 'active' ? (
        <NPopconfirm onPositiveClick={() => handleHeartbeat(row.id)}>
          {{
            trigger: () => (
              <NButton size="tiny" quaternary type="primary">
                {$t('page.edgeNodes.heartbeat')}
              </NButton>
            ),
            default: () => $t('page.edgeNodes.heartbeatConfirm')
          }}
        </NPopconfirm>
      ) : (
        <NTag type="error" size="small">
          {row.status}
        </NTag>
      )
  }
]

async function loadNodes() {
  startLoading()
  try {
    const { data, error } = await fetchEdgeNodes(200)
    if (error) return
    nodes.value = data ?? []
  } finally {
    endLoading()
  }
}

async function handleRegister() {
  submitting.value = true
  try {
    const capabilities = registerForm.capabilities
      .split(',')
      .map(item => item.trim())
      .filter(Boolean)
    const { error } = await registerEdgeNode({
      node_id: registerForm.node_id.trim(),
      version: registerForm.version.trim(),
      ...(capabilities.length ? { capabilities } : {})
    })
    if (!error) {
      registerVisible.value = false
      registerForm.node_id = ''
      registerForm.version = ''
      registerForm.capabilities = ''
      await loadNodes()
    }
  } finally {
    submitting.value = false
  }
}

async function handleHeartbeat(nodeId: string) {
  await heartbeatEdgeNode(nodeId)
  await loadNodes()
}

onMounted(() => {
  void loadNodes()
})
</script>

<template>
  <div class="min-h-500px">
    <NCard :title="$t('page.edgeNodes.title')" class="h-full">
      <template #header-extra>
        <NButton size="small" type="primary" @click="registerVisible = true">
          {{ $t('page.edgeNodes.register') }}
        </NButton>
      </template>

      <NDataTable
        remote
        :columns="columns"
        :data="nodes"
        :loading="loading"
        :pagination="false"
        :row-key="(row: EdgeNodeEntry) => row.id"
        :scroll-x="900"
      />
    </NCard>

    <NModal v-model:show="registerVisible" preset="card" :title="$t('page.edgeNodes.register')" class="w-520px">
      <NForm :model="registerForm" :rules="registerRules" label-placement="top">
        <NFormItem :label="$t('page.edgeNodes.nodeId')" path="node_id">
          <NInput v-model:value="registerForm.node_id" :placeholder="$t('page.edgeNodes.nodeIdPlaceholder')" />
        </NFormItem>
        <NFormItem :label="$t('page.edgeNodes.version')" path="version">
          <NInput v-model:value="registerForm.version" :placeholder="'1.2.0'" />
        </NFormItem>
        <NFormItem :label="$t('page.edgeNodes.capabilities')" path="capabilities">
          <NInput v-model:value="registerForm.capabilities" :placeholder="$t('page.edgeNodes.capabilitiesPlaceholder')" />
        </NFormItem>
        <div class="flex justify-end gap-2">
          <NButton @click="registerVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="submitting" @click="handleRegister">
            {{ $t('common.confirm') }}
          </NButton>
        </div>
      </NForm>
    </NModal>
  </div>
</template>
