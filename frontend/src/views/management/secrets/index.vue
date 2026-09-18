<!--
文件用途：通用 Secrets Storage（ROADMAP TB-18）管理控制台。
核心逻辑：
1. 密钥列表：按 Key/名称检索、按类型过滤、脱敏展示；
2. 密钥创建与更新：强校验 Key 规范，明文写入后端 AES-256-GCM 信封静态加密，AAD 绑定当前租户；
3. 受审解密（Reveal）：调用 /reveal 端点并在客户端安全弹窗展示明文，支持一键复制与 15 秒自动销毁倒计时；
4. 密钥轮换（Reseal）：检测 needs_reseal 并在前端提供一键在线重加密操作。
-->
<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NTag,
  useMessage
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import {
  createSecret,
  deleteSecret,
  getSecretsList,
  resealSecret,
  revealSecret,
  updateSecret,
  type CreateSecretParams,
  type SecretItem,
  type SecretType,
  type UpdateSecretParams
} from '@/service/api/secret'
import { formatDateTime } from '@/utils/common/datetime'

const message = useMessage()

// 状态管理
const loading = ref(false)
const secrets = ref<SecretItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const searchQuery = ref('')
const selectedType = ref<string | null>(null)

// 模态窗控制
const modalVisible = ref(false)
const isEdit = ref(false)
const currentId = ref('')
const formRef = ref<FormInst | null>(null)
const submitting = ref(false)

// 表单数据
const formModel = reactive<{
  key: string
  name: string
  secret_type: SecretType
  description: string
  value: string
}>({
  key: '',
  name: '',
  secret_type: 'GENERIC',
  description: '',
  value: ''
})

const secretTypeOptions = [
  { label: '全部类型', value: '' },
  { label: '通用密钥 (GENERIC)', value: 'GENERIC' },
  { label: 'API 凭证 (API_KEY)', value: 'API_KEY' },
  { label: '授权令牌 (TOKEN)', value: 'TOKEN' },
  { label: '密码凭据 (PASSWORD)', value: 'PASSWORD' },
  { label: '客户端证书 (CERTIFICATE)', value: 'CERTIFICATE' },
  { label: 'OAuth2 凭证 (OAUTH2)', value: 'OAUTH2' }
]

const formTypeOptions = secretTypeOptions.filter(o => o.value !== '')

const formRules: FormRules = {
  key: [
    { required: true, message: '请输入密钥唯一标识 (Key)', trigger: 'blur' },
    {
      validator: (_rule, value: string) => {
        if (!value) return true
        const regex = /^[a-zA-Z0-9_\-\.]{1,64}$/
        if (!regex.test(value)) {
          return new Error('Key 仅允许 1-64 位的字母、数字、下划线、中划线和点号')
        }
        return true
      },
      trigger: 'blur'
    }
  ],
  name: [{ required: true, message: '请输入密钥展示名称', trigger: 'blur' }],
  value: [
    {
      validator: (_rule, value: string) => {
        if (!isEdit.value && (!value || value.trim() === '')) {
          return new Error('创建时密钥明文不能为空')
        }
        return true
      },
      trigger: 'blur'
    }
  ]
}

// 明文查看（Reveal）状态
const revealModalVisible = ref(false)
const revealLoading = ref(false)
const revealedPlaintext = ref('')
const revealedKey = ref('')
const countdownSeconds = ref(15)
let countdownTimer: number | null = null

// 类型标签样式映射
const typeTagMap: Record<SecretType, { type: 'default' | 'info' | 'success' | 'warning' | 'primary' | 'error'; label: string }> = {
  GENERIC: { type: 'default', label: '通用' },
  API_KEY: { type: 'info', label: 'API Key' },
  TOKEN: { type: 'success', label: 'Token' },
  PASSWORD: { type: 'warning', label: 'Password' },
  CERTIFICATE: { type: 'primary', label: '证书' },
  OAUTH2: { type: 'error', label: 'OAuth2' }
}

const loadData = async () => {
  loading.value = true
  try {
    const res = await getSecretsList({
      page: page.value,
      page_size: pageSize.value,
      query: searchQuery.value.trim() || undefined,
      secret_type: selectedType.value || undefined
    })
    if (res?.data) {
      secrets.value = res.data.list || []
      total.value = res.data.total || 0
    }
  } catch (err: any) {
    message.error(err?.message || '获取密钥列表失败')
  } finally {
    loading.value = false
  }
}

const handleOpenCreate = () => {
  isEdit.value = false
  currentId.value = ''
  formModel.key = ''
  formModel.name = ''
  formModel.secret_type = 'GENERIC'
  formModel.description = ''
  formModel.value = ''
  modalVisible.value = true
}

const handleOpenEdit = (row: SecretItem) => {
  isEdit.value = true
  currentId.value = row.id
  formModel.key = row.key
  formModel.name = row.name
  formModel.secret_type = row.secret_type
  formModel.description = row.description
  formModel.value = '' // 编辑时留空代表不修改明文
  modalVisible.value = true
}

const handleSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async errors => {
    if (errors) return
    submitting.value = true
    try {
      if (isEdit.value) {
        const updatePayload: UpdateSecretParams = {
          name: formModel.name,
          secret_type: formModel.secret_type,
          description: formModel.description,
          value: formModel.value.trim() ? formModel.value.trim() : undefined
        }
        await updateSecret(currentId.value, updatePayload)
        message.success('更新密钥成功')
      } else {
        const createPayload: CreateSecretParams = {
          key: formModel.key.trim(),
          name: formModel.name.trim(),
          secret_type: formModel.secret_type,
          description: formModel.description.trim(),
          value: formModel.value.trim()
        }
        await createSecret(createPayload)
        message.success('创建密钥成功')
      }
      modalVisible.value = false
      loadData()
    } catch (err: any) {
      message.error(err?.message || '保存密钥失败')
    } finally {
      submitting.value = false
    }
  })
}

const handleDelete = async (row: SecretItem) => {
  try {
    await deleteSecret(row.id)
    message.success(`已删除密钥 ${row.key}`)
    loadData()
  } catch (err: any) {
    message.error(err?.message || '删除密钥失败')
  }
}

const handleReseal = async (row: SecretItem) => {
  try {
    await resealSecret(row.id)
    message.success(`密钥 ${row.key} 重新加密轮换完成`)
    loadData()
  } catch (err: any) {
    message.error(err?.message || '重加密失败')
  }
}

const startCountdown = () => {
  if (countdownTimer) clearInterval(countdownTimer)
  countdownSeconds.value = 15
  countdownTimer = window.setInterval(() => {
    countdownSeconds.value -= 1
    if (countdownSeconds.value <= 0) {
      closeRevealModal()
    }
  }, 1000)
}

const closeRevealModal = () => {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
  revealedPlaintext.value = ''
  revealedKey.value = ''
  revealModalVisible.value = false
}

const handleReveal = async (row: SecretItem) => {
  revealedKey.value = row.key
  revealedPlaintext.value = ''
  revealModalVisible.value = true
  revealLoading.value = true
  try {
    const res = await revealSecret(row.id)
    if (res?.data) {
      revealedPlaintext.value = res.data.value
      startCountdown()
    }
  } catch (err: any) {
    message.error(err?.message || '解密查看失败')
    closeRevealModal()
  } finally {
    revealLoading.value = false
  }
}

const handleCopyPlaintext = async () => {
  if (!revealedPlaintext.value) return
  try {
    await navigator.clipboard.writeText(revealedPlaintext.value)
    message.success('已安全复制到剪贴板')
  } catch {
    message.warning('剪贴板写入受限，请手动选中复制')
  }
}

const columns: DataTableColumns<SecretItem> = [
  {
    title: '密钥标识 (Key)',
    key: 'key',
    width: 180,
    render(row) {
      return h('span', { class: 'font-mono text-sm font-semibold text-primary' }, row.key)
    }
  },
  {
    title: '名称',
    key: 'name',
    width: 160,
    ellipsis: { tooltip: true }
  },
  {
    title: '类型',
    key: 'secret_type',
    width: 120,
    render(row) {
      const meta = typeTagMap[row.secret_type] || { type: 'default', label: row.secret_type }
      return h(NTag, { size: 'small', type: meta.type as any }, { default: () => meta.label })
    }
  },
  {
    title: '脱敏掩码',
    key: 'mask_preview',
    width: 120,
    render(row) {
      return h('span', { class: 'font-mono text-xs text-gray-500 bg-gray-100 dark:bg-gray-800 px-2 py-0.5 rounded' }, row.mask_preview)
    }
  },
  {
    title: '轮换状态',
    key: 'needs_reseal',
    width: 110,
    render(row) {
      if (row.needs_reseal) {
        return h(NTag, { size: 'small', type: 'warning', bordered: false }, { default: () => '需重新加密' })
      }
      return h(NTag, { size: 'small', type: 'success', bordered: false }, { default: () => '最新' })
    }
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 170,
    render(row) {
      return formatDateTime(row.created_at)
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 240,
    fixed: 'right',
    render(row) {
      return h(
        NSpace,
        { size: 'small' },
        {
          default: () => [
            h(
              NButton,
              {
                size: 'tiny',
                type: 'info',
                quaternary: true,
                onClick: () => handleReveal(row)
              },
              { default: () => '查看明文' }
            ),
            h(
              NButton,
              {
                size: 'tiny',
                type: 'primary',
                quaternary: true,
                onClick: () => handleOpenEdit(row)
              },
              { default: () => '编辑' }
            ),
            row.needs_reseal &&
              h(
                NButton,
                {
                  size: 'tiny',
                  type: 'warning',
                  quaternary: true,
                  onClick: () => handleReseal(row)
                },
                { default: () => '轮换' }
              ),
            h(
              NPopconfirm,
              {
                onPositiveClick: () => handleDelete(row)
              },
              {
                trigger: () =>
                  h(
                    NButton,
                    {
                      size: 'tiny',
                      type: 'error',
                      quaternary: true
                    },
                    { default: () => '删除' }
                  ),
                default: () => `确认删除密钥 ${row.key} 吗？下游引用的任务可能因此失败。`
              }
            )
          ]
        }
      )
    }
  }
]

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
            v-model:value="searchQuery"
            placeholder="搜索 Key 或名称"
            clearable
            style="width: 220px"
            @keyup.enter="loadData"
            @clear="loadData"
          />
          <NSelect
            v-model:value="selectedType"
            :options="secretTypeOptions"
            placeholder="密钥类型"
            style="width: 170px"
            clearable
            @update:value="loadData"
          />
          <NButton type="primary" @click="handleOpenCreate">新增密钥</NButton>
          <NButton :loading="loading" @click="loadData">刷新</NButton>
        </NSpace>
      </template>

      <div class="mb-3">
        <NAlert type="info" size="small" :bordered="false">
          通用密钥用于集中保管第三方 API Key、访问令牌、密码与证书。所有敏感内容均由后端 AES-256-GCM 信封高强静态加密存储（AAD 绑定当前租户），配置中可使用
          <code class="font-mono font-bold text-primary">${secret.KEY_NAME}</code>
          动态按需解析，杜绝硬编码泄漏。
        </NAlert>
      </div>

      <NDataTable
        remote
        :loading="loading"
        :columns="columns"
        :data="secrets"
        :pagination="{
          page: page,
          pageSize: pageSize,
          itemCount: total,
          showSizePicker: true,
          pageSizes: [10, 20, 50],
          onChange: (p: number) => { page = p; loadData() },
          onUpdatePageSize: (ps: number) => { pageSize = ps; page = 1; loadData() }
        }"
      />
    </NCard>

    <!-- 新增 / 编辑模态框 -->
    <NModal
      v-model:show="modalVisible"
      preset="card"
      :title="isEdit ? '编辑密钥' : '新增通用密钥'"
      style="width: 560px"
      :segmented="{ content: 'soft', footer: 'soft' }"
    >
      <NForm ref="formRef" :model="formModel" :rules="formRules" label-placement="left" label-width="110px">
        <NFormItem label="密钥标识" path="key">
          <NInput
            v-model:value="formModel.key"
            placeholder="例如 AWS_IOT_ACCESS_KEY"
            :disabled="isEdit"
          />
        </NFormItem>
        <NFormItem label="展示名称" path="name">
          <NInput v-model:value="formModel.name" placeholder="请输入易于识别的名称" />
        </NFormItem>
        <NFormItem label="密钥类型" path="secret_type">
          <NSelect v-model:value="formModel.secret_type" :options="formTypeOptions" />
        </NFormItem>
        <NFormItem label="描述说明" path="description">
          <NInput
            v-model:value="formModel.description"
            type="textarea"
            placeholder="可选填写密钥用途或接入说明"
            :rows="2"
          />
        </NFormItem>
        <NFormItem label="密钥明文" path="value">
          <NInput
            v-model:value="formModel.value"
            type="password"
            show-password-on="click"
            :placeholder="isEdit ? '留空表示保持原有密钥明文不变' : '请输入凭据敏感内容'"
          />
        </NFormItem>
      </NForm>

      <template #footer>
        <NSpace justify="end">
          <NButton @click="modalVisible = false">取消</NButton>
          <NButton type="primary" :loading="submitting" @click="handleSubmit">保存</NButton>
        </NSpace>
      </template>
    </NModal>

    <!-- 解密查看（Reveal）模态框 -->
    <NModal
      v-model:show="revealModalVisible"
      preset="card"
      title="安全凭证解密查看"
      style="width: 520px"
      :segmented="{ content: 'soft', footer: 'soft' }"
      @after-leave="closeRevealModal"
    >
      <div class="space-y-3">
        <NAlert type="warning" title="安全警告" size="small">
          本次解密查看已记录至系统安全审计日志。为防泄密，请勿截屏或共享给无关人员。
          弹窗将在 <span class="font-bold text-error">{{ countdownSeconds }}</span> 秒后自动销毁关闭。
        </NAlert>

        <div>
          <div class="text-xs text-gray-500 mb-1">密钥标识 (Key):</div>
          <div class="font-mono text-sm font-semibold">{{ revealedKey }}</div>
        </div>

        <div>
          <div class="text-xs text-gray-500 mb-1">解密明文 (Plaintext):</div>
          <div v-if="revealLoading" class="text-xs text-gray-400 py-2">正在安全解密信封密文...</div>
          <div v-else class="p-2 bg-gray-100 dark:bg-gray-800 rounded font-mono text-sm break-all select-all">
            {{ revealedPlaintext || '（解密结果为空）' }}
          </div>
        </div>
      </div>

      <template #footer>
        <NSpace justify="space-between" align="center">
          <span class="text-xs text-gray-400">倒计时：{{ countdownSeconds }}s</span>
          <NSpace>
            <NButton type="primary" secondary @click="handleCopyPlaintext">复制明文</NButton>
            <NButton @click="closeRevealModal">立即关闭</NButton>
          </NSpace>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>
