<!--
文件用途：边缘节点证书工作台模态框（从 index.vue 拆出）。
核心逻辑：
1. 打开时拉取节点当前生效的 X.509 客户端证书（mTLS 凭证）并脱敏展示；
2. 支持按有效期限（天）签发新证书，私钥仅在签发当次一次性展示；
3. 支持吊销证书，吊销后清空证书详情与一次性私钥。
关键注意事项：打开弹窗时重置一次性私钥展示，避免上一个节点的私钥残留。
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NDescriptions,
  NDescriptionsItem,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSpace,
  NTag
} from 'naive-ui'
import {
  fetchEdgeNodeCertificate,
  issueEdgeNodeCertificate,
  revokeEdgeNodeCertificate,
  type EdgeNodeCertificateInfo,
  type EdgeNodeEntry
} from '@/service/api'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'

const props = defineProps<{
  show: boolean
  node: EdgeNodeEntry | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const visible = computed({
  get: () => props.show,
  set: (value) => emit('update:show', value)
})

const certLoading = ref(false)
const certInfo = ref<EdgeNodeCertificateInfo | null>(null)
const certValidityDays = ref(365)
const newlyIssuedKey = ref('')
const newlyIssuedCert = ref('')

watch(
  () => props.show,
  (show) => {
    if (!show || !props.node) return
    certInfo.value = null
    newlyIssuedKey.value = ''
    newlyIssuedCert.value = ''
    void loadCert(props.node.id)
  }
)

async function loadCert(nodeId: string) {
  certLoading.value = true
  try {
    const { data } = await fetchEdgeNodeCertificate(nodeId)
    if (data) {
      certInfo.value = data
    }
  } finally {
    certLoading.value = false
  }
}

async function handleIssueCert() {
  if (!props.node) return
  certLoading.value = true
  try {
    const { data, error } = await issueEdgeNodeCertificate(props.node.id, certValidityDays.value)
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
  if (!props.node) return
  certLoading.value = true
  try {
    const { error } = await revokeEdgeNodeCertificate(props.node.id)
    if (!error) {
      certInfo.value = null
      newlyIssuedKey.value = ''
      newlyIssuedCert.value = ''
    }
  } finally {
    certLoading.value = false
  }
}

defineExpose({ handleIssueCert, handleRevokeCert, certInfo, certValidityDays, newlyIssuedKey })
</script>

<template>
  <NModal
    v-model:show="visible"
    preset="card"
    :title="`${$t('page.edgeNodes.certificateTitle')} - ${node?.id}`"
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
</template>
