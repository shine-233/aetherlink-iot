<!--
文件用途：通用 Secrets Storage（ROADMAP TB-18）管理控制台。
核心逻辑：
1. 密钥列表：按 Key/名称检索、按类型过滤、脱敏展示（分页与过期请求取消交给 useListPage）；
2. 密钥创建与更新：强校验 Key 规范，明文写入后端 AES-256-GCM 信封静态加密，AAD 绑定当前租户；
3. 受审解密（Reveal）：调用 /reveal 端点并在客户端安全弹窗展示明文，支持一键复制与 15 秒自动销毁倒计时；
4. 密钥轮换（Reseal）：检测 needs_reseal 并在前端提供一键在线重加密操作。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NAlert, NButton, NCard, NDataTable, NInput, NSelect, NSpace, useMessage } from 'naive-ui'
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'
import {
  createSecret,
  deleteSecret,
  getSecretsList,
  resealSecret,
  revealSecret,
  updateSecret,
  type CreateSecretParams,
  type SecretItem,
  type UpdateSecretParams
} from '@/service/api/secret'
import { createSecretColumns } from './secret-columns'
import SecretFormModal from './SecretFormModal.vue'
import SecretRevealModal from './SecretRevealModal.vue'

const message = useMessage()

const SECRET_TYPE_OPTIONS = [
  { label: '全部类型', value: '' },
  { label: '通用密钥 (GENERIC)', value: 'GENERIC' },
  { label: 'API 凭证 (API_KEY)', value: 'API_KEY' },
  { label: '授权令牌 (TOKEN)', value: 'TOKEN' },
  { label: '密码凭据 (PASSWORD)', value: 'PASSWORD' },
  { label: '客户端证书 (CERTIFICATE)', value: 'CERTIFICATE' },
  { label: 'OAuth2 凭证 (OAUTH2)', value: 'OAUTH2' }
]

// ---------- 列表 ----------
// 快速切换筛选条件时，旧条件的慢响应不会覆盖新条件的结果。
const {
  query,
  rows: secrets,
  loading,
  pagination,
  load: loadData,
  search: handleSearch
} = useListPage<SecretItem, { query: string; secret_type: string }>({
  initialQuery: () => ({ query: '', secret_type: '' }),
  pageSizes: [10, 20, 50],
  serialize: q => ({ query: q.query.trim() || undefined, secret_type: q.secret_type || undefined }),
  fetcher: async params => fromFlatResponse<SecretItem>(await getSecretsList(params))
})

// ---------- 新增/编辑 ----------
const modalVisible = ref(false)
const editingSecret = ref<SecretItem | null>(null)
const submitting = ref(false)

function handleOpenCreate() {
  editingSecret.value = null
  modalVisible.value = true
}

function handleOpenEdit(row: SecretItem) {
  editingSecret.value = row
  modalVisible.value = true
}

async function handleSubmit(payload: { id: string; create?: CreateSecretParams; update?: UpdateSecretParams }) {
  submitting.value = true
  try {
    if (payload.update) {
      await updateSecret(payload.id, payload.update)
      message.success('更新密钥成功')
    } else {
      await createSecret(payload.create as CreateSecretParams)
      message.success('创建密钥成功')
    }
    modalVisible.value = false
    loadData()
  } catch (err: any) {
    message.error(err?.message || '保存密钥失败')
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row: SecretItem) {
  try {
    await deleteSecret(row.id)
    message.success(`已删除密钥 ${row.key}`)
    loadData()
  } catch (err: any) {
    message.error(err?.message || '删除密钥失败')
  }
}

async function handleReseal(row: SecretItem) {
  try {
    await resealSecret(row.id)
    message.success(`密钥 ${row.key} 重新加密轮换完成`)
    loadData()
  } catch (err: any) {
    message.error(err?.message || '重加密失败')
  }
}

// ---------- 明文查看（Reveal） ----------
const revealModalVisible = ref(false)
const revealLoading = ref(false)
const revealedPlaintext = ref('')
const revealedKey = ref('')

async function handleReveal(row: SecretItem) {
  revealedKey.value = row.key
  revealedPlaintext.value = ''
  revealModalVisible.value = true
  revealLoading.value = true
  try {
    const res = await revealSecret(row.id)
    if (res?.data) {
      revealedPlaintext.value = res.data.value
    }
  } catch (err: any) {
    message.error(err?.message || '解密查看失败')
    revealModalVisible.value = false
  } finally {
    revealLoading.value = false
  }
}

async function handleCopyPlaintext() {
  if (!revealedPlaintext.value) return
  try {
    await navigator.clipboard.writeText(revealedPlaintext.value)
    message.success('已安全复制到剪贴板')
  } catch {
    message.warning('剪贴板写入受限，请手动选中复制')
  }
}

const columns = computed(() =>
  createSecretColumns({
    onReveal: handleReveal,
    onEdit: handleOpenEdit,
    onReseal: handleReseal,
    onDelete: handleDelete
  })
)

onMounted(() => {
  loadData()
})
</script>

<template>
  <div class="p-4 space-y-4">
    <NCard title="通用密钥保管库 (Secrets Storage)" :bordered="false" size="small">
      <template #header-extra>
        <NSpace align="center">
          <NInput
            v-model:value="query.query"
            placeholder="搜索 Key 或名称"
            clearable
            style="width: 220px"
            @keyup.enter="handleSearch"
            @clear="handleSearch"
          />
          <NSelect
            v-model:value="query.secret_type"
            :options="SECRET_TYPE_OPTIONS"
            placeholder="密钥类型"
            style="width: 170px"
            clearable
            @update:value="handleSearch"
          />
          <NButton type="primary" @click="handleOpenCreate">新增密钥</NButton>
          <NButton :loading="loading" @click="loadData">刷新</NButton>
        </NSpace>
      </template>

      <div class="mb-3">
        <NAlert type="info" size="small" :bordered="false">
          通用密钥用于集中保管第三方 API Key、访问令牌、密码与证书。所有敏感内容均由后端 AES-256-GCM
          信封高强静态加密存储（AAD 绑定当前租户），配置中可使用
          <code class="font-mono font-bold text-primary">${secret.KEY_NAME}</code>
          动态按需解析，杜绝硬编码泄漏。
        </NAlert>
      </div>

      <NDataTable remote :loading="loading" :columns="columns" :data="secrets" :pagination="pagination" />
    </NCard>

    <SecretFormModal
      v-model:show="modalVisible"
      :editing="editingSecret"
      :submitting="submitting"
      @submit="handleSubmit"
    />

    <SecretRevealModal
      v-model:show="revealModalVisible"
      :secret-key="revealedKey"
      :loading="revealLoading"
      :plaintext="revealedPlaintext"
      @copy="handleCopyPlaintext"
    />
  </div>
</template>
