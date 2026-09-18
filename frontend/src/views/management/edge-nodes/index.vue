<!--
文件用途：P1.5 边缘节点管理页——注册表列表、健康状态、证书签发与版本升级/回滚工作台。
核心逻辑：
1. 节点注册与心跳：注册走幂等更新；心跳刷新 last_seen；
2. 证书生命周期：为边缘节点签发 X.509 客户端证书（mTLS 凭证），支持证书脱敏查看、私钥一次性展示、证书吊销；
3. 远程升级与回滚：版本严格递增校验的在线升级、升级历史流水展示、一键回滚到历史版本。
-->
<script setup lang="tsx">
import { onMounted, reactive, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NDescriptions,
  NDescriptionsItem,
  NDrawer,
  NDrawerContent,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSpace,
  NTag
} from 'naive-ui'
import type { DataTableColumns, FormRules } from 'naive-ui'
import { useLoading } from '@aetherlink/hooks'
import {
  fetchEdgeNodes,
  heartbeatEdgeNode,
  registerEdgeNode,
  fetchEdgeNodeCertificate,
  issueEdgeNodeCertificate,
  revokeEdgeNodeCertificate,
  upgradeEdgeNode,
  rollbackEdgeNode,
  fetchEdgeNodeUpgradeHistory,
  type EdgeNodeEntry,
  type EdgeNodeCertificateInfo,
  type EdgeNodeUpgradeHistoryEntry
} from '@/service/api'
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

// 证书相关状态
const certModalVisible = ref(false)
const certLoading = ref(false)
const currentNode = ref<EdgeNodeEntry | null>(null)
const certInfo = ref<EdgeNodeCertificateInfo | null>(null)
const certValidityDays = ref(365)
const newlyIssuedKey = ref('')
const newlyIssuedCert = ref('')

// 升级与历史相关状态
const upgradeDrawerVisible = ref(false)
const upgradeLoading = ref(false)
const upgradeSubmitting = ref(false)
const upgradeHistory = ref<EdgeNodeUpgradeHistoryEntry[]>([])
const upgradeForm = reactive({
  target_version: '',
  package_url: '',
  checksum: '',
  description: ''
})

const upgradeRules: FormRules = {
  target_version: { required: true, message: $t('page.edgeNodes.targetVersionPlaceholder'), trigger: 'blur' }
}

const columns: DataTableColumns<EdgeNodeEntry> = [
  { title: $t('page.edgeNodes.nodeId'), key: 'id', width: 200, ellipsis: { tooltip: true } },
  { title: $t('page.edgeNodes.version'), key: 'version', width: 100 },
  {
    title: $t('page.edgeNodes.health'),
    key: 'health',
    width: 100,
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
    width: 170,
    render: row => (row.last_seen_at ? formatDateTime(row.last_seen_at) : '-')
  },
  {
    title: $t('page.edgeNodes.actions'),
    key: 'actions',
    width: 240,
    render: row =>
      row.status === 'active' ? (
        <NSpace size="small">
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
          <NButton size="tiny" quaternary type="info" onClick={() => openCertModal(row)}>
            {$t('page.edgeNodes.certificate')}
          </NButton>
          <NButton size="tiny" quaternary type="warning" onClick={() => openUpgradeDrawer(row)}>
            {$t('page.edgeNodes.upgrade')}
          </NButton>
        </NSpace>
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

// 证书逻辑
async function openCertModal(node: EdgeNodeEntry) {
  currentNode.value = node
  certInfo.value = null
  newlyIssuedKey.value = ''
  newlyIssuedCert.value = ''
  certModalVisible.value = true
  certLoading.value = true
  try {
    const { data } = await fetchEdgeNodeCertificate(node.id)
    if (data) {
      certInfo.value = data
    }
  } finally {
    certLoading.value = false
  }
}

async function handleIssueCert() {
  if (!currentNode.value) return
  certLoading.value = true
  try {
    const { data, error } = await issueEdgeNodeCertificate(currentNode.value.id, certValidityDays.value)
    if (!error && data) {
      newlyIssuedKey.value = data.private_key
      newlyIssuedCert.value = data.certificate
      certInfo.value = data
    }
  } finally {
    certLoading.value = false
  }
}

async function handleRevokeCert() {
  if (!currentNode.value) return
  certLoading.value = true
  try {
    const { error } = await revokeEdgeNodeCertificate(currentNode.value.id)
    if (!error) {
      certInfo.value = null
      newlyIssuedKey.value = ''
      newlyIssuedCert.value = ''
    }
  } finally {
    certLoading.value = false
  }
}

// 升级与历史逻辑
async function openUpgradeDrawer(node: EdgeNodeEntry) {
  currentNode.value = node
  upgradeForm.target_version = ''
  upgradeForm.package_url = ''
  upgradeForm.checksum = ''
  upgradeForm.description = ''
  upgradeDrawerVisible.value = true
  await loadUpgradeHistory(node.id)
}

async function loadUpgradeHistory(nodeId: string) {
  upgradeLoading.value = true
  try {
    const { data } = await fetchEdgeNodeUpgradeHistory(nodeId, 50)
    upgradeHistory.value = data ?? []
  } finally {
    upgradeLoading.value = false
  }
}

async function handleUpgrade() {
  if (!currentNode.value || !upgradeForm.target_version.trim()) return
  upgradeSubmitting.value = true
  try {
    const { error } = await upgradeEdgeNode(currentNode.value.id, {
      target_version: upgradeForm.target_version.trim(),
      package_url: upgradeForm.package_url.trim() || undefined,
      checksum: upgradeForm.checksum.trim() || undefined,
      description: upgradeForm.description.trim() || undefined
    })
    if (!error) {
      upgradeForm.target_version = ''
      upgradeForm.package_url = ''
      upgradeForm.checksum = ''
      upgradeForm.description = ''
      await loadUpgradeHistory(currentNode.value.id)
      await loadNodes()
    }
  } finally {
    upgradeSubmitting.value = false
  }
}

async function handleRollback(historyId: string) {
  if (!currentNode.value) return
  upgradeLoading.value = true
  try {
    const { error } = await rollbackEdgeNode(currentNode.value.id, historyId)
    if (!error) {
      await loadUpgradeHistory(currentNode.value.id)
      await loadNodes()
    }
  } finally {
    upgradeLoading.value = false
  }
}

const historyColumns: DataTableColumns<EdgeNodeUpgradeHistoryEntry> = [
  { title: $t('page.edgeNodes.fromVersion'), key: 'from_version', width: 90 },
  { title: $t('page.edgeNodes.targetVersion'), key: 'target_version', width: 90 },
  {
    title: $t('page.edgeNodes.health'),
    key: 'status',
    width: 100,
    render: row => (
      <NTag type={row.status === 'rolled_back' ? 'warning' : row.status === 'dispatched' ? 'info' : 'default'} size="small">
        {row.status}
      </NTag>
    )
  },
  {
    title: $t('page.edgeNodes.lastSeen'),
    key: 'created_at',
    width: 160,
    render: row => (row.created_at ? formatDateTime(row.created_at) : '-')
  },
  {
    title: $t('page.edgeNodes.actions'),
    key: 'actions',
    width: 80,
    render: row => (
      <NPopconfirm onPositiveClick={() => handleRollback(row.id)}>
        {{
          trigger: () => (
            <NButton size="tiny" quaternary type="warning">
              {$t('page.edgeNodes.rollback')}
            </NButton>
          ),
          default: () => $t('page.edgeNodes.rollbackConfirm')
        }}
      </NPopconfirm>
    )
  }
]

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

    <!-- 注册节点模态框 -->
    <NModal v-model:show="registerVisible" preset="card" :title="$t('page.edgeNodes.register')" class="w-520px">
      <NForm :model="registerForm" :rules="registerRules" label-placement="top">
        <NFormItem :label="$t('page.edgeNodes.nodeId')" path="node_id">
          <NInput v-model:value="registerForm.node_id" :placeholder="$t('page.edgeNodes.nodeIdPlaceholder')" />
        </NFormItem>
        <NFormItem :label="$t('page.edgeNodes.version')" path="version">
          <NInput v-model:value="registerForm.version" placeholder="1.0.0" />
        </NFormItem>
        <NFormItem :label="$t('page.edgeNodes.capabilities')" path="capabilities">
          <NInput
            v-model:value="registerForm.capabilities"
            :placeholder="$t('page.edgeNodes.capabilitiesPlaceholder')"
          />
        </NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="registerVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="submitting" @click="handleRegister">
            {{ $t('common.confirm') }}
          </NButton>
        </NSpace>
      </template>
    </NModal>

    <!-- 证书管理模态框 -->
    <NModal
      v-model:show="certModalVisible"
      preset="card"
      :title="`${$t('page.edgeNodes.certificateTitle')} - ${currentNode?.id}`"
      class="w-680px"
    >
      <NSpace vertical :size="16">
        <div v-if="certInfo">
          <NDescriptions bordered :column="2" label-placement="left">
            <NDescriptionsItem :label="$t('page.edgeNodes.certSerial')">
              {{ certInfo.serial_number }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('page.edgeNodes.certStatus')">
              <NTag type="success" size="small">{{ certInfo.status }}</NTag>
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('page.edgeNodes.certNotBefore')">
              {{ formatDateTime(certInfo.not_before) }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('page.edgeNodes.certNotAfter')">
              {{ formatDateTime(certInfo.not_after) }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('page.edgeNodes.certFingerprint')" :span="2">
              <code class="text-xs break-all">{{ certInfo.fingerprint }}</code>
            </NDescriptionsItem>
          </NDescriptions>

          <NSpace class="mt-4" justify="end">
            <NPopconfirm @positive-click="handleRevokeCert">
              <template #trigger>
                <NButton size="small" type="error" :loading="certLoading">
                  {{ $t('page.edgeNodes.revokeCert') }}
                </NButton>
              </template>
              {{ $t('page.edgeNodes.revokeCertConfirm') }}
            </NPopconfirm>
          </NSpace>
        </div>
        <div v-else>
          <NAlert type="info" :show-icon="true">当前节点尚未签发生效的 X.509 客户端证书。</NAlert>
        </div>

        <div v-if="newlyIssuedKey" class="p-3 bg-amber-50 rounded border border-amber-200">
          <NAlert type="warning" :title="$t('page.edgeNodes.certPrivateKeyNotice')" class="mb-3" />
          <div class="mb-1 text-xs font-semibold">私钥 Private Key (仅本次可见):</div>
          <NInput :value="newlyIssuedKey" type="textarea" :rows="5" readonly class="font-mono text-xs" />
          <div class="mt-2 mb-1 text-xs font-semibold">证书 Certificate:</div>
          <NInput :value="newlyIssuedCert" type="textarea" :rows="5" readonly class="font-mono text-xs" />
        </div>

        <div class="p-4 border rounded">
          <div class="text-sm font-semibold mb-2">{{ $t('page.edgeNodes.issueCert') }}</div>
          <NSpace align="center">
            <span>有效期限 (天):</span>
            <NInputNumber v-model:value="certValidityDays" :min="1" :max="3650" class="w-120px" />
            <NPopconfirm @positive-click="handleIssueCert">
              <template #trigger>
                <NButton type="primary" size="small" :loading="certLoading">
                  {{ $t('page.edgeNodes.issueCert') }}
                </NButton>
              </template>
              {{ $t('page.edgeNodes.issueCertConfirm') }}
            </NPopconfirm>
          </NSpace>
        </div>
      </NSpace>
    </NModal>

    <!-- 版本升级与历史抽屉 -->
    <NDrawer v-model:show="upgradeDrawerVisible" :width="600" placement="right">
      <NDrawerContent :title="`${$t('page.edgeNodes.upgradeTitle')} - ${currentNode?.id}`">
        <NSpace vertical :size="20">
          <NCard size="small" :title="$t('page.edgeNodes.executeUpgrade')">
            <NForm :model="upgradeForm" :rules="upgradeRules" label-placement="top">
              <NFormItem label="当前版本">
                <NTag type="info" size="medium">{{ currentNode?.version }}</NTag>
              </NFormItem>
              <NFormItem :label="$t('page.edgeNodes.targetVersion')" path="target_version">
                <NInput
                  v-model:value="upgradeForm.target_version"
                  :placeholder="$t('page.edgeNodes.targetVersionPlaceholder')"
                />
              </NFormItem>
              <NFormItem label="升级包 URL (可选)">
                <NInput v-model:value="upgradeForm.package_url" placeholder="https://..." />
              </NFormItem>
              <NFormItem label="校验和 Checksum (可选)">
                <NInput v-model:value="upgradeForm.checksum" placeholder="sha256:..." />
              </NFormItem>
              <NFormItem label="升级说明 (可选)">
                <NInput v-model:value="upgradeForm.description" placeholder="升级说明..." />
              </NFormItem>
              <NButton type="primary" block :loading="upgradeSubmitting" @click="handleUpgrade">
                {{ $t('page.edgeNodes.executeUpgrade') }}
              </NButton>
            </NForm>
          </NCard>

          <div>
            <div class="text-base font-semibold mb-3">{{ $t('page.edgeNodes.upgradeHistory') }}</div>
            <NDataTable
              remote
              :columns="historyColumns"
              :data="upgradeHistory"
              :loading="upgradeLoading"
              :pagination="false"
              :row-key="(row: EdgeNodeUpgradeHistoryEntry) => row.id"
              size="small"
            />
          </div>
        </NSpace>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>
