<!--
  文件用途：部件库创建/编辑弹窗（从 index.vue 拆出）。
  核心逻辑：名称/版本/行业分类/描述/部件定义 JSON 表单；打开时按 editing 行回填或重置默认值；
  提交前先走表单校验，再本地校验部件定义 JSON（validateWidgetsJson），成功后 emit saved 并关闭。
  关键注意事项：部件定义 JSON 必须是对象数组且逐项含 type/version，校验失败只提示不发请求。
-->
<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { NButton, NForm, NFormItem, NInput, NModal, NSpace, useMessage } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { createWidgetBundle, updateWidgetBundle, type WidgetBundleItem } from '@/service/api'
import { $t } from '@/locales'
import { validateWidgetsJson } from './widget-bundle'

interface Props {
  /** 弹窗显示状态（v-model:show）。 */
  show: boolean
  /** 编辑目标行；null 表示创建。 */
  editing: WidgetBundleItem | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'saved'): void
}>()

const message = useMessage()
const submitting = ref(false)
const formRef = ref<FormInst | null>(null)
const formModel = reactive({
  name: '',
  version: '1.0.0',
  type_key: '',
  description: '',
  widgets: '[]'
})

const formRules: FormRules = {
  name: { required: true, message: $t('page.widgetBundle.nameRequired'), trigger: 'blur' }
}

// 打开弹窗时回填：编辑用行数据兜底默认值，创建恢复初始表单。
watch(
  () => props.show,
  (show) => {
    if (!show) return
    if (props.editing) {
      Object.assign(formModel, {
        name: props.editing.name,
        version: props.editing.version || '1.0.0',
        type_key: props.editing.type_key || '',
        description: props.editing.description || '',
        widgets: props.editing.widgets || '[]'
      })
    } else {
      Object.assign(formModel, {
        name: '',
        version: '1.0.0',
        type_key: '',
        description: '',
        widgets: '[]'
      })
    }
  },
  { immediate: true }
)

const closeModal = () => {
  emit('update:show', false)
}

const handleSubmit = async () => {
  await formRef.value?.validate()
  const widgetsError = validateWidgetsJson(formModel.widgets)
  if (widgetsError) {
    message.error(widgetsError)
    return
  }
  submitting.value = true
  try {
    const payload = {
      name: formModel.name,
      version: formModel.version || undefined,
      type_key: formModel.type_key || undefined,
      description: formModel.description || undefined,
      widgets: formModel.widgets
    }
    const { error } = props.editing
      ? await updateWidgetBundle({ id: props.editing.id, ...payload })
      : await createWidgetBundle(payload)
    if (!error) {
      message.success($t('page.widgetBundle.saveSuccess'))
      emit('saved')
      closeModal()
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="editing ? $t('page.widgetBundle.editTitle') : $t('page.widgetBundle.createTitle')"
    class="w-720px"
    @update:show="closeModal"
  >
    <NForm ref="formRef" :model="formModel" :rules="formRules" label-placement="left" label-width="110">
      <NFormItem :label="$t('page.widgetBundle.name')" path="name">
        <NInput v-model:value="formModel.name" :placeholder="$t('page.widgetBundle.namePlaceholder')" />
      </NFormItem>
      <NFormItem :label="$t('page.widgetBundle.version')" path="version">
        <NInput v-model:value="formModel.version" placeholder="1.0.0" />
      </NFormItem>
      <NFormItem :label="$t('page.widgetBundle.typeKey')" path="type_key">
        <NInput v-model:value="formModel.type_key" :placeholder="$t('page.widgetBundle.typeKeyPlaceholder')" />
      </NFormItem>
      <NFormItem :label="$t('page.widgetBundle.description')" path="description">
        <NInput v-model:value="formModel.description" type="textarea" :rows="2" />
      </NFormItem>
      <NFormItem :label="$t('page.widgetBundle.widgets')" path="widgets">
        <NInput
          v-model:value="formModel.widgets"
          type="textarea"
          :rows="12"
          class="font-mono"
          :placeholder="$t('page.widgetBundle.widgetsPlaceholder')"
        />
      </NFormItem>
      <div class="mb-8px text-12px text-gray-400">{{ $t('page.widgetBundle.widgetsHint') }}</div>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="closeModal">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="submitting" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
