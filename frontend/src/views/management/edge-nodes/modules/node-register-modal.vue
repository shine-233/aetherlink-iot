<!--
文件用途：边缘节点注册模态框（从 index.vue 拆出）。
核心逻辑：注册走幂等更新——node_id 相同即覆盖 version/capabilities；capabilities 为
  逗号分隔的自由文本，提交前按逗号切分、去空白、过滤空项。
关键注意事项：注册成功后重置表单、关闭弹窗并通知父页面回刷列表。
-->
<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { NButton, NForm, NFormItem, NInput, NModal, NSpace } from 'naive-ui'
import type { FormRules } from 'naive-ui'
import { registerEdgeNode } from '@/service/api'
import { $t } from '@/locales'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  registered: []
}>()

const visible = computed({
  get: () => props.show,
  set: (value) => emit('update:show', value)
})

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

function close() {
  visible.value = false
}

async function handleRegister() {
  submitting.value = true
  try {
    const capabilities = registerForm.capabilities
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean)
    const { error } = await registerEdgeNode({
      node_id: registerForm.node_id.trim(),
      version: registerForm.version.trim(),
      ...(capabilities.length ? { capabilities } : {})
    })
    if (!error) {
      registerForm.node_id = ''
      registerForm.version = ''
      registerForm.capabilities = ''
      close()
      emit('registered')
    }
  } finally {
    submitting.value = false
  }
}

defineExpose({ handleRegister, registerForm })
</script>

<template>
  <NModal v-model:show="visible" preset="card" :title="$t('page.edgeNodes.register')" class="w-520px">
    <NForm :model="registerForm" :rules="registerRules" label-placement="top">
      <NFormItem :label="$t('page.edgeNodes.nodeId')" path="node_id">
        <NInput v-model:value="registerForm.node_id" :placeholder="$t('page.edgeNodes.nodeIdPlaceholder')" />
      </NFormItem>
      <NFormItem :label="$t('page.edgeNodes.version')" path="version">
        <NInput v-model:value="registerForm.version" placeholder="1.0.0" />
      </NFormItem>
      <NFormItem :label="$t('page.edgeNodes.capabilities')" path="capabilities">
        <NInput v-model:value="registerForm.capabilities" :placeholder="$t('page.edgeNodes.capabilitiesPlaceholder')" />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="visible = false">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="submitting" @click="handleRegister">
          {{ $t('common.confirm') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>
