<!--
文件用途：个人中心「修改账号邮箱」验证弹窗（新邮箱 + 验证码 + 倒计时按钮）。
核心逻辑：状态由父级 usePersonalCenterEmailChange 持有，本组件只双向绑定与转发动作。
-->
<script setup lang="ts">
import { NAlert, NButton, NForm, NFormItem, NInput, NModal, NSpace } from 'naive-ui'
import { $t } from '@/locales'

defineProps<{
  form: { new_email: string; verify_code: string }
  codeLoading: boolean
  codeCounting: boolean
  codeLabel: string
  submitting: boolean
}>()

const show = defineModel<boolean>('show', { required: true })
defineEmits<{ sendCode: []; submit: [] }>()
</script>

<template>
  <NModal v-model:show="show" preset="card" :title="$t('custom.personalCenter.changeAccountEmail')" class="max-w-520px">
    <NSpace vertical size="large">
      <NAlert type="info" :show-icon="false">
        {{ $t('custom.personalCenter.emailChangeNotice') }}
      </NAlert>
      <NForm label-placement="top">
        <NFormItem :label="$t('custom.personalCenter.newEmail')">
          <NInput v-model:value="form.new_email" placeholder="name@example.com" />
        </NFormItem>
        <NFormItem :label="$t('custom.personalCenter.verificationCode')">
          <div class="flex gap-8px w-full">
            <NInput v-model:value="form.verify_code" :placeholder="$t('custom.personalCenter.codePlaceholder')" />
            <NButton :loading="codeLoading" :disabled="codeCounting" @click="$emit('sendCode')">
              {{ codeLabel }}
            </NButton>
          </div>
        </NFormItem>
      </NForm>
      <div class="flex justify-end gap-8px">
        <NButton @click="show = false">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="submitting" @click="$emit('submit')">{{ $t('common.confirm') }}</NButton>
      </div>
    </NSpace>
  </NModal>
</template>
