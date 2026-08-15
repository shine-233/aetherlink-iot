<!--
邮件通知配置组件，负责邮件服务参数回显、保存和测试邮件发送。
核心链路：拉取当前邮件配置 -> 反序列化 config 到 email_config -> 编辑后提交保存 -> 可选发送调试邮件验证通道连通性。
静态维护重点：
1. `config` 是后端字符串字段，页面内部真正编辑的是 `email_config` 对象，修改字段结构时要同步检查序列化契约。
2. 调试弹窗与正式配置共用部分校验规则，后续若继续扩展测试参数，建议拆分独立 rules，避免保存表单与调试表单互相牵连。
3. 邮件账号、授权码属于敏感信息，后续若增强 UX，应优先补脱敏展示、失败反馈和提交中禁用态，而不是继续堆字段。
-->
<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useMessage } from 'naive-ui'
import type { FormInst, MessageReactive } from 'naive-ui'
import { useBoolean, useLoading } from '@aetherlink/hooks'
import { editNotificationServices, fetchNotificationServicesEmail, sendTestEmail } from '@/service/api'
import { smartDeepClone as deepClone } from '@/utils/deep-clone'
import { createRequiredFormRule } from '@/utils/form/rule'
import { $t } from '~/src/locales'
import EmailTemplateManager from '@/components/business/email-template-manager.vue'

const { loading, startLoading, endLoading } = useLoading(false)
const { bool: visible, setTrue: openModal, setFalse: closeModal } = useBoolean()

const formModel = reactive<NotificationServices.Email>(createDefaultFormModel())

// 将后端返回的字符串化配置映射回页面可编辑对象，避免表单直接操作原始 JSON 字符串。
function setTableData(data: Api.NotificationServices.Email) {
  Object.assign(formModel, data)
  if (data.config !== 'null') {
    formModel.email_config = JSON.parse(data.config)
  }
}

// 页面初始化和保存成功后都复用这一入口重新拉取最新邮件通道配置。
async function getNotificationServices() {
  startLoading()
  const { data } = await fetchNotificationServicesEmail()
  if (data) {
    setTableData(data)
  }
  endLoading()
}

function createDefaultFormModel(): NotificationServices.Email {
  return {
    id: '',
    email_config: {},
    config: '',
    notice_type: 'EMAIL',
    status: 'OPEN',
    remark: ''
  }
}

const rules = {
  'email_config.host': createRequiredFormRule($t('common.pleaseCheckValue')),
  'email_config.port': createRequiredFormRule($t('common.pleaseCheckValue')),
  'email_config.from_email': createRequiredFormRule($t('common.pleaseCheckValue')),
  'email_config.from_password': createRequiredFormRule($t('common.pleaseCheckValue')),
  email: createRequiredFormRule($t('common.pleaseCheckValue')),
  body: createRequiredFormRule($t('common.pleaseCheckValue'))
}
const formRef = ref<HTMLElement & FormInst>()

// 保存时删除只读的 config 镜像字段，避免把旧字符串和新对象一起提交给后端。
async function handleSubmit() {
  await formRef.value?.validate()
  startLoading()
  const formData = deepClone(formModel)
  delete (formData as Record<string, unknown>).config
  const data: any = await editNotificationServices(formData)
  if (!data.error) {
    window.$message?.success('success')
    endLoading()
    await getNotificationServices()
  }
}

type FormModel = {
  body: string
  email: string
  header: string
}

const debugData = reactive<FormModel>({
  body: '',
  email: '',
  header: ''
})

// 每次打开调试弹窗都重置输入，避免复用上次测试地址和正文。
function handleOpenModal() {
  Object.assign(debugData, {
    body: '',
    email: '',
    header: ''
  })
  openModal()
}

const message = useMessage()
const debugFormRef = ref<HTMLElement & FormInst>()

// 测试发送只验证通知通道是否可用，不会持久化正式配置。
async function handleSend() {
  await debugFormRef.value?.validate()
  let messageReactive: MessageReactive | null = message.loading($t('common.modifySuccess'), {
    duration: 100000
  })
  const data: any = await sendTestEmail(debugData)
  if (!data.error) {
    window.$message?.success('success')
  }
  if (messageReactive) {
    messageReactive.destroy()
    messageReactive = null
  }
  closeModal()
}

function init() {
  getNotificationServices()
}

init()
</script>

<template>
  <NSpin :show="loading">
    <NForm ref="formRef" label-placement="left" :label-width="130" :model="formModel" :rules="rules">
      <NGrid :cols="24">
        <NFormItemGridItem
          :span="6"
          :label="$t('page.manage.notification.email.form.sendMailServer')"
          path="email_config.host"
        >
          <NInput v-model:value="formModel.email_config.host" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem
          :span="6"
          :label="$t('page.manage.notification.email.form.sendPort')"
          path="email_config.port"
        >
          <NInputNumber v-model:value="formModel.email_config.port" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem
          :span="6"
          :label="$t('page.manage.notification.email.form.senderMail')"
          path="email_config.from_email"
        >
          <NInput v-model:value="formModel.email_config.from_email" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem
          :span="6"
          :label="$t('page.manage.notification.email.form.authorizationCodeOrPassword')"
          path="email_config.from_password"
        >
          <NInput v-model:value="formModel.email_config.from_password" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem :span="6" :label="$t('page.manage.notification.email.form.ssl')" path="email_config.ssl">
          <n-checkbox v-model:checked="formModel.email_config.ssl"></n-checkbox>
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem :span="6" :label="$t('page.manage.notification.enableDisableService')" path="status">
          <n-switch v-model:value="formModel.status" checked-value="OPEN" unchecked-value="CLOSE" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem :span="24" class="mt-20px">
          <div class="w-120px"></div>
          <NButton class="w-72px" @click="handleOpenModal">
            {{ $t('common.debug') }}
          </NButton>
          <NButton class="ml-20px w-72px" type="primary" @click="handleSubmit">
            {{ $t('common.save') }}
          </NButton>
        </NFormItemGridItem>
      </NGrid>
      <NSpace class="w-full pt-16px" :size="24" justify="start"></NSpace>
    </NForm>
  </NSpin>

  <NModal v-model:show="visible" preset="card" :title="$t('common.debug')" class="w-500px">
    <NForm ref="debugFormRef" label-placement="left" :label-width="120" :model="debugData" :rules="rules">
      <NGrid :cols="24" :x-gap="18">
        <NFormItemGridItem :span="24" :label="$t('page.manage.notification.email.form.inbox')" path="email">
          <NInput v-model:value="debugData.email" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('page.manage.notification.email.form.message')" path="body">
          <NInput v-model:value="debugData.body" />
        </NFormItemGridItem>
      </NGrid>
      <NSpace class="w-full pt-16px" :size="24" justify="center">
        <NButton class="w-72px" type="primary" @click="handleSend">{{ $t('common.send') }}</NButton>
      </NSpace>
    </NForm>
  </NModal>

  <EmailTemplateManager />
</template>

<style lang="scss"></style>
