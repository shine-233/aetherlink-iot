<!--
短信通知配置组件，负责短信服务参数回显、编辑与启停控制。
核心链路：拉取短信通知服务 -> 将 config 反序列化成 `sme_config` -> 编辑阿里云短信参数 -> 保存后重新回读最新配置。
静态维护重点：
1. 当前实现默认只暴露 `ALIYUN` 供应商，后续扩展多厂商时要优先抽象 provider 映射和不同配置结构，而不是继续在单文件内堆条件分支。
2. `sme_config` 是页面真实编辑对象，`config` 只是后端字符串镜像，修改字段命名时要同步确认前后端契约。
3. access key、secret、模板编码都属于敏感配置，后续应补更明确的失败提示和权限边界说明。
-->
<script setup lang="ts">
import { reactive, ref } from 'vue'
import type { FormInst } from 'naive-ui'
import { useLoading } from '@aetherlink/hooks'
import { editNotificationServices, fetchNotificationServicesSms } from '@/service/api'
import { smartDeepClone as deepClone } from '@/utils/deep-clone'
import { createRequiredFormRule } from '@/utils/form/rule'
import { $t } from '~/src/locales'

const { loading, startLoading, endLoading } = useLoading(false)
const formRef = ref<HTMLElement & FormInst>()
const providerOptions = [{ label: 'Aliyun SMS', value: 'ALIYUN' }]

const formModel = reactive<NotificationServices.Sms>(createDefaultFormModel())

// 默认结构既提供 provider，也补齐阿里云短信所需的嵌套字段，避免回显和保存阶段缺键。
function createDefaultSmsConfig(): Api.NotificationServices.SmsConfig {
  return {
    provider: 'ALIYUN',
    aliyun_sms_config: {
      access_key_id: '',
      access_key_secret: '',
      endpoint: '',
      sign_name: '',
      template_code: ''
    }
  }
}

function createDefaultFormModel(): NotificationServices.Sms {
  return {
    id: '',
    config: '',
    sme_config: createDefaultSmsConfig(),
    notice_type: 'SME_CODE',
    status: 'OPEN',
    remark: ''
  }
}

// 把后端字符串化 config 合并回默认对象，兼容老数据缺字段的场景。
function setTableData(data: Api.NotificationServices.Sms) {
  Object.assign(formModel, data)
  if (data.config && data.config !== 'null') {
    formModel.sme_config = {
      ...createDefaultSmsConfig(),
      ...JSON.parse(data.config)
    }
  }
}

// 初始化与保存成功后的统一刷新入口，保证页面总是以服务端最新配置为准。
async function getNotificationServices() {
  startLoading()
  const { data } = await fetchNotificationServicesSms()
  if (data) {
    setTableData(data)
  }
  endLoading()
}

const rules = {
  'sme_config.provider': createRequiredFormRule($t('common.pleaseCheckValue')),
  'sme_config.aliyun_sms_config.access_key_id': createRequiredFormRule($t('common.pleaseCheckValue')),
  'sme_config.aliyun_sms_config.access_key_secret': createRequiredFormRule($t('common.pleaseCheckValue')),
  'sme_config.aliyun_sms_config.endpoint': createRequiredFormRule($t('common.pleaseCheckValue')),
  'sme_config.aliyun_sms_config.sign_name': createRequiredFormRule($t('common.pleaseCheckValue')),
  'sme_config.aliyun_sms_config.template_code': createRequiredFormRule($t('common.pleaseCheckValue'))
}

// 保存前剔除后端镜像字段，只提交当前编辑态的短信配置对象。
async function handleSubmit() {
  await formRef.value?.validate()
  startLoading()
  const formData = deepClone(formModel)
  delete (formData as Record<string, unknown>).config
  const data: any = await editNotificationServices(formData)
  if (!data.error) {
    window.$message?.success('success')
    await getNotificationServices()
  }
  endLoading()
}

getNotificationServices()
</script>

<template>
  <NSpin :show="loading">
    <NForm ref="formRef" label-placement="left" :label-width="150" :model="formModel" :rules="rules">
      <NGrid :cols="24">
        <NFormItemGridItem :span="6" :label="$t('page.manage.notification.shortMessage.form.provider')" path="sme_config.provider">
          <NSelect v-model:value="formModel.sme_config.provider" :options="providerOptions" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem
          :span="6"
          :label="$t('page.manage.notification.shortMessage.form.accessKeyId')"
          path="sme_config.aliyun_sms_config.access_key_id"
        >
          <NInput v-model:value="formModel.sme_config.aliyun_sms_config.access_key_id" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem
          :span="6"
          :label="$t('page.manage.notification.shortMessage.form.accessKeySecret')"
          path="sme_config.aliyun_sms_config.access_key_secret"
        >
          <NInput v-model:value="formModel.sme_config.aliyun_sms_config.access_key_secret" type="password" show-password-on="click" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem
          :span="6"
          :label="$t('page.manage.notification.shortMessage.form.endpoint')"
          path="sme_config.aliyun_sms_config.endpoint"
        >
          <NInput v-model:value="formModel.sme_config.aliyun_sms_config.endpoint" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem
          :span="6"
          :label="$t('page.manage.notification.shortMessage.form.signName')"
          path="sme_config.aliyun_sms_config.sign_name"
        >
          <NInput v-model:value="formModel.sme_config.aliyun_sms_config.sign_name" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem
          :span="6"
          :label="$t('page.manage.notification.shortMessage.form.templateCode')"
          path="sme_config.aliyun_sms_config.template_code"
        >
          <NInput v-model:value="formModel.sme_config.aliyun_sms_config.template_code" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem :span="6" :label="$t('page.manage.notification.enableDisableService')" path="status">
          <NSwitch v-model:value="formModel.status" checked-value="OPEN" unchecked-value="CLOSE" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem :span="24" class="mt-20px">
          <div class="w-150px"></div>
          <NButton class="w-72px" type="primary" @click="handleSubmit">
            {{ $t('common.save') }}
          </NButton>
        </NFormItemGridItem>
      </NGrid>
    </NForm>
  </NSpin>
</template>
