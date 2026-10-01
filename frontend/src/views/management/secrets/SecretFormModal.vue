<!--
文件用途：密钥新增/编辑弹窗。
核心逻辑：强校验 Key 规范；编辑时明文留空代表不修改。
-->
<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { NButton, NForm, NFormItem, NInput, NModal, NSelect, NSpace } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import type { CreateSecretParams, SecretItem, SecretType, UpdateSecretParams } from '@/service/api/secret'

const props = defineProps<{
  show: boolean
  editing: SecretItem | null
  submitting: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (
    e: 'submit',
    payload: { id: string; create: CreateSecretParams } | { id: string; update: UpdateSecretParams }
  ): void
}>()

const SECRET_TYPE_OPTIONS = [
  { label: '通用密钥 (GENERIC)', value: 'GENERIC' },
  { label: 'API 凭证 (API_KEY)', value: 'API_KEY' },
  { label: '授权令牌 (TOKEN)', value: 'TOKEN' },
  { label: '密码凭据 (PASSWORD)', value: 'PASSWORD' },
  { label: '客户端证书 (CERTIFICATE)', value: 'CERTIFICATE' },
  { label: 'OAuth2 凭证 (OAUTH2)', value: 'OAUTH2' }
]

const formRef = ref<FormInst | null>(null)

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

const isEdit = () => Boolean(props.editing)

/** 每次打开弹窗按「新增 / 编辑」重置表单，避免上一次输入残留。 */
watch(
  () => [props.show, props.editing] as const,
  ([show, editing]) => {
    if (!show) return
    formModel.key = editing?.key ?? ''
    formModel.name = editing?.name ?? ''
    formModel.secret_type = (editing?.secret_type ?? 'GENERIC') as SecretType
    formModel.description = editing?.description ?? ''
    // 编辑时留空代表不修改明文
    formModel.value = ''
  },
  { immediate: true }
)

const rules: FormRules = {
  key: [
    { required: true, message: '请输入密钥唯一标识 (Key)', trigger: 'blur' },
    {
      validator: (_rule, value: string) => {
        if (!value) return true
        const regex = /^[a-zA-Z0-9_.-]{1,64}$/
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
        if (!isEdit() && (!value || value.trim() === '')) {
          return new Error('创建时密钥明文不能为空')
        }
        return true
      },
      trigger: 'blur'
    }
  ]
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate(async errors => {
    if (errors) return
    const id = props.editing?.id ?? ''
    if (props.editing) {
      emit('submit', {
        id,
        update: {
          name: formModel.name,
          secret_type: formModel.secret_type,
          description: formModel.description,
          value: formModel.value.trim() ? formModel.value.trim() : undefined
        }
      })
    } else {
      emit('submit', {
        id,
        create: {
          key: formModel.key.trim(),
          name: formModel.name.trim(),
          secret_type: formModel.secret_type,
          description: formModel.description.trim(),
          value: formModel.value.trim()
        }
      })
    }
  })
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="editing ? '编辑密钥' : '新增通用密钥'"
    style="width: 560px"
    :segmented="{ content: 'soft', footer: 'soft' }"
    @update:show="emit('update:show', $event)"
  >
    <NForm ref="formRef" :model="formModel" :rules="rules" label-placement="left" label-width="110px">
      <NFormItem label="密钥标识" path="key">
        <NInput v-model:value="formModel.key" placeholder="例如 AWS_IOT_ACCESS_KEY" :disabled="Boolean(editing)" />
      </NFormItem>
      <NFormItem label="展示名称" path="name">
        <NInput v-model:value="formModel.name" placeholder="请输入易于识别的名称" />
      </NFormItem>
      <NFormItem label="密钥类型" path="secret_type">
        <NSelect v-model:value="formModel.secret_type" :options="SECRET_TYPE_OPTIONS" />
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
          :placeholder="editing ? '留空表示保持原有密钥明文不变' : '请输入凭据敏感内容'"
        />
      </NFormItem>
    </NForm>

    <template #footer>
      <NSpace justify="end">
        <NButton @click="emit('update:show', false)">取消</NButton>
        <NButton type="primary" :loading="submitting" @click="handleSubmit">保存</NButton>
      </NSpace>
    </template>
  </NModal>
</template>
