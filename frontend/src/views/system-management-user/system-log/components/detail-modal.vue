<!--
文件用途: 承载Detail Modal相关的系统管理用户侧页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { defineExpose, ref } from 'vue'
import dayjs from 'dayjs'
import { $t } from '@/locales'

const modalVisible = ref(false)
const detailInfo = ref({
  id: '',
  email: '',
  username: '',
  ip: '',
  request_message: '',
  response_message: '',
  latency: '',
  name: '',
  path: '',
  created_at: ''
})

const show = info => {
  modalVisible.value = true
  detailInfo.value = info
}
const closeModal = () => {
  modalVisible.value = false
}
defineExpose({
  show
})
</script>

<template>
  <NModal v-model:show="modalVisible" preset="card" :title="$t('custom.management.logDetail')" class="w-80%">
    <NForm v-model="detailInfo" label-placement="left" label-align="left" label-width="80px">
      <NFormItem :label="$t('custom.management.account')">
        <div class="result">{{ detailInfo.email }}</div>
      </NFormItem>
      <NFormItem :label="$t('custom.management.account')">
        <div class="result">{{ detailInfo.username }}</div>
      </NFormItem>
      <NFormItem :label="$t('custom.management.requestTime')">
        <div class="result">{{ detailInfo.latency }}ms</div>
      </NFormItem>
      <NFormItem :label="$t('custom.management.time')">
        <div class="result">
          {{ dayjs(detailInfo.created_at).format('YYYY-MM-DD hh:mm:ss') }}
        </div>
      </NFormItem>
      <NFormItem :label="$t('custom.management.requestPath')">
        <div class="result">{{ detailInfo.path }}</div>
      </NFormItem>
      <NFormItem :label="$t('custom.management.requestMethod')">
        <div class="result">{{ detailInfo.name }}</div>
      </NFormItem>
      <NFormItem :label="$t('custom.management.ipAddress')">
        <div class="result">{{ detailInfo.ip }}</div>
      </NFormItem>
      <NFormItem :label="$t('custom.management.requestContent')">
        <NInput v-model:value="detailInfo.request_message" type="textarea" readonly disabled></NInput>
      </NFormItem>
      <NFormItem :label="$t('custom.management.responseContent')">
        <NInput v-model:value="detailInfo.response_message" type="textarea" readonly disabled></NInput>
      </NFormItem>
    </NForm>
    <div class="text-right">
      <NButton @click="closeModal">{{ $t('custom.management.close') }}</NButton>
    </div>
  </NModal>
</template>

<style scoped>
.n-form-item .n-form-item-label {
  font-size: 14px;
  color: #101010;
}
.value {
  font-size: 14px;
  color: #101010;
}
</style>
