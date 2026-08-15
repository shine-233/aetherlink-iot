<!--
推送通知配置组件，负责推送服务地址回显与保存。
核心链路：拉取当前推送配置 -> 归一化 url 字段 -> 编辑推送服务地址 -> 保存后重新回读配置。
静态维护重点：
1. 当前表单很轻，但它直接影响全局推送通知可用性，后续新增鉴权头、token 或多推送通道时要先抽象配置模型。
2. `url === 'null'` 的兼容处理说明后端存在历史字符串占位数据，修改接口契约前要同步清理回显逻辑。
3. 这里暂时没有调试发送能力，若后续补充，应与邮件测试弹窗复用相似的“测试不落库”模式。
-->
<script lang="ts" setup>
import { reactive, ref } from 'vue'
import type { FormInst } from 'naive-ui'
import { useLoading } from '@aetherlink/hooks'
import { createRequiredFormRule } from '@/utils/form/rule'
import { smartDeepClone as deepClone } from '@/utils/deep-clone'
import { editPushNotificationServices, fetchPushNotificationServices } from '@/service/api'
import { $t } from '~/src/locales'

const { loading, startLoading, endLoading } = useLoading(false)

const formModel = reactive<NotificationServices.PushNotification>(createDefaultFormModel())

// 后端历史上可能把空值写成字符串 `null`，这里统一归一化为空串，避免输入框显示脏值。
function setTableData(data: Api.NotificationServices.PushNotification) {
  Object.assign(formModel, data, {
    url: data.url === 'null' ? '' : data.url
  })
}

// 初始化与保存完成后统一重拉推送配置，避免页面保留旧 URL。
async function getNotificationServices() {
  startLoading()
  const { data } = await fetchPushNotificationServices()
  if (data) {
    setTableData(data)
  }
  endLoading()
}

function createDefaultFormModel(): NotificationServices.PushNotification {
  return {
    url: ''
  }
}

const rules = {
  pushServer: createRequiredFormRule($t('common.pleaseCheckValue'))
}
const formRef = ref<HTMLElement & FormInst>()

// 保存时沿用当前表单模型，剔除遗留镜像字段，保持请求体简洁稳定。
async function handleSubmit() {
  await formRef.value?.validate()
  startLoading()
  const formData = deepClone(formModel)
  delete (formData as Record<string, unknown>).config
  const data: any = await editPushNotificationServices(formData)
  if (!data.error) {
    window.$message?.success($t('common.saveSuccess'))
    endLoading()
    await getNotificationServices()
  }
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
          :span="14"
          :label="$t('page.manage.notification.pushNotification.pushServer')"
          path="pushNotification.pushServer"
        >
          <NInput v-model:value="formModel.url" />
        </NFormItemGridItem>
      </NGrid>
      <NGrid :cols="24">
        <NFormItemGridItem :span="24" class="mt-20px">
          <div class="w-120px"></div>
          <NButton class="ml-20px w-72px" type="primary" @click="handleSubmit">
            {{ $t('common.save') }}
          </NButton>
        </NFormItemGridItem>
      </NGrid>
    </NForm>
  </NSpin>
</template>

<style lang="scss"></style>
