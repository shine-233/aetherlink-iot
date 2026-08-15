<!--
  文件用途：作为共享业务组件管理系统或当前租户的告警邮件模板，并在保存前预览白名单变量渲染结果。
  核心逻辑：分页读取模板 -> 创建/编辑 -> 预览 -> 启用并设为默认；实际收件人仍由告警通知链路决定。
  关键注意事项：模板只能使用页面列出的变量；本组件不读取 SMTP 密码、收件人或任意设备私有字段。
-->
<script setup lang="ts">
import { reactive, ref } from 'vue'
import type { FormInst } from 'naive-ui'
import {
  createAlarmEmailTemplate,
  deleteAlarmEmailTemplate,
  fetchAlarmEmailTemplates,
  previewAlarmEmailTemplate,
  setDefaultAlarmEmailTemplate,
  updateAlarmEmailTemplate,
  type AlarmEmailTemplate,
  type AlarmEmailTemplatePayload
} from '@/service/api/notification-services'
import { createRequiredFormRule } from '@/utils/form/rule'
import { $t } from '@/locales'

const loading = ref(false)
const saving = ref(false)
const modalVisible = ref(false)
const previewVisible = ref(false)
const editingID = ref('')
const templates = ref<AlarmEmailTemplate[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const formRef = ref<FormInst>()

const form = reactive<AlarmEmailTemplatePayload>({
  name: '',
  subject_template: '[AetherLink] {{.Subject}}',
  body_template: '{{.Message}}\n\nDevices: {{.DeviceIDs}}\nSent at: {{.SentAt}}',
  enabled: true,
  is_default: false
})

const preview = reactive({ subject: '', body: '' })
const rules = {
  name: createRequiredFormRule($t('common.pleaseCheckValue')),
  subject_template: createRequiredFormRule($t('common.pleaseCheckValue')),
  body_template: createRequiredFormRule($t('common.pleaseCheckValue'))
}

async function loadTemplates() {
  loading.value = true
  try {
    const { data } = await fetchAlarmEmailTemplates({ page: page.value, page_size: pageSize })
    templates.value = data?.list || []
    total.value = Number(data?.total || 0)
  } finally {
    loading.value = false
  }
}

function resetForm() {
  editingID.value = ''
  Object.assign(form, {
    name: '',
    subject_template: '[AetherLink] {{.Subject}}',
    body_template: '{{.Message}}\n\nDevices: {{.DeviceIDs}}\nSent at: {{.SentAt}}',
    enabled: true,
    is_default: false
  })
}

function openCreate() {
  resetForm()
  modalVisible.value = true
}

function openEdit(template: AlarmEmailTemplate) {
  editingID.value = template.id
  Object.assign(form, {
    name: template.name,
    subject_template: template.subject_template,
    body_template: template.body_template,
    enabled: template.enabled,
    is_default: template.is_default
  })
  modalVisible.value = true
}

async function showPreview() {
  await formRef.value?.validate()
  const { data, error } = await previewAlarmEmailTemplate({
    subject_template: form.subject_template,
    body_template: form.body_template,
    subject: 'High temperature alarm',
    message: 'Temperature Sensor T1 exceeded the configured limit.',
    device_ids: ['RDI-DEMO-001']
  })
  if (!error && data) {
    preview.subject = data.subject
    preview.body = data.body
    previewVisible.value = true
  }
}

async function saveTemplate() {
  await formRef.value?.validate()
  saving.value = true
  try {
    const result = editingID.value
      ? await updateAlarmEmailTemplate(editingID.value, { ...form })
      : await createAlarmEmailTemplate({ ...form })
    if (!result.error) {
      modalVisible.value = false
      window.$message?.success($t('common.saveSuccess'))
      await loadTemplates()
    }
  } finally {
    saving.value = false
  }
}

async function makeDefault(template: AlarmEmailTemplate) {
  const result = await setDefaultAlarmEmailTemplate(template.id)
  if (!result.error) {
    window.$message?.success($t('common.modifySuccess'))
    await loadTemplates()
  }
}

function removeTemplate(template: AlarmEmailTemplate) {
  window.$dialog?.warning({
    title: $t('common.tip'),
    content: $t('page.manage.notification.email.template.deleteConfirm'),
    positiveText: $t('common.confirm'),
    negativeText: $t('common.cancel'),
    async onPositiveClick() {
      const result = await deleteAlarmEmailTemplate(template.id)
      if (!result.error) {
        window.$message?.success($t('common.deleteSuccess'))
        if (templates.value.length === 1 && page.value > 1) page.value -= 1
        await loadTemplates()
      }
    }
  })
}

function changePage(value: number) {
  page.value = value
  void loadTemplates()
}

function changeEnabled(value: boolean) {
  form.enabled = value
  if (!value) form.is_default = false
}

void loadTemplates()
</script>

<template>
  <NCard class="mt-24px" size="small" :bordered="true">
    <template #header>
      <div class="flex items-center justify-between gap-16px">
        <div>
          <div class="font-600">{{ $t('page.manage.notification.email.template.title') }}</div>
          <div class="mt-4px text-12px text-gray-500">
            {{ $t('page.manage.notification.email.template.description') }}
          </div>
        </div>
        <NButton type="primary" @click="openCreate">
          {{ $t('page.manage.notification.email.template.create') }}
        </NButton>
      </div>
    </template>

    <NSpin :show="loading">
      <NEmpty v-if="templates.length === 0" :description="$t('page.manage.notification.email.template.empty')" />
      <NList v-else bordered>
        <NListItem v-for="template in templates" :key="template.id">
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-8px">
              <span class="font-600">{{ template.name }}</span>
              <NTag v-if="template.is_default" size="small" type="success">
                {{ $t('page.manage.notification.email.template.default') }}
              </NTag>
              <NTag size="small" :type="template.enabled ? 'info' : 'default'">
                {{
                  template.enabled
                    ? $t('page.manage.notification.email.template.enabled')
                    : $t('page.manage.notification.email.template.disabled')
                }}
              </NTag>
            </div>
            <div class="mt-6px truncate text-12px text-gray-500">{{ template.subject_template }}</div>
          </div>
          <template #suffix>
            <NSpace>
              <NButton v-if="!template.is_default" text type="primary" @click="makeDefault(template)">
                {{ $t('page.manage.notification.email.template.setDefault') }}
              </NButton>
              <NButton text type="primary" @click="openEdit(template)">{{ $t('common.edit') }}</NButton>
              <NButton text type="error" @click="removeTemplate(template)">{{ $t('common.delete') }}</NButton>
            </NSpace>
          </template>
        </NListItem>
      </NList>
      <div v-if="total > pageSize" class="mt-16px flex justify-end">
        <NPagination :page="page" :page-size="pageSize" :item-count="total" @update:page="changePage" />
      </div>
    </NSpin>
  </NCard>

  <NModal
    v-model:show="modalVisible"
    preset="card"
    class="w-720px max-w-[calc(100vw-32px)]"
    :title="editingID ? $t('page.manage.notification.email.template.edit') : $t('page.manage.notification.email.template.create')"
  >
    <NAlert type="info" class="mb-16px">
      {{ $t('page.manage.notification.email.template.variablesHint') }}
    </NAlert>
    <NForm ref="formRef" :model="form" :rules="rules" label-placement="top">
      <NFormItem :label="$t('page.manage.notification.email.template.name')" path="name">
        <NInput v-model:value="form.name" maxlength="120" show-count />
      </NFormItem>
      <NFormItem :label="$t('page.manage.notification.email.template.subject')" path="subject_template">
        <NInput v-model:value="form.subject_template" maxlength="500" show-count />
      </NFormItem>
      <NFormItem :label="$t('page.manage.notification.email.template.body')" path="body_template">
        <NInput v-model:value="form.body_template" type="textarea" :autosize="{ minRows: 8, maxRows: 18 }" maxlength="20000" show-count />
      </NFormItem>
      <NSpace>
        <NCheckbox :checked="form.enabled" @update:checked="changeEnabled">
          {{ $t('page.manage.notification.email.template.enabled') }}
        </NCheckbox>
        <NCheckbox v-model:checked="form.is_default" :disabled="!form.enabled">
          {{ $t('page.manage.notification.email.template.default') }}
        </NCheckbox>
      </NSpace>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="showPreview">{{ $t('page.manage.notification.email.template.preview') }}</NButton>
        <NButton type="primary" :loading="saving" @click="saveTemplate">{{ $t('common.save') }}</NButton>
      </NSpace>
    </template>
  </NModal>

  <NModal v-model:show="previewVisible" preset="card" class="w-640px max-w-[calc(100vw-32px)]" :title="$t('page.manage.notification.email.template.preview')">
    <NDescriptions label-placement="top" :column="1" bordered>
      <NDescriptionsItem :label="$t('page.manage.notification.email.template.renderedSubject')">
        {{ preview.subject }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.manage.notification.email.template.renderedBody')">
        <pre class="m-0 whitespace-pre-wrap break-words font-inherit">{{ preview.body }}</pre>
      </NDescriptionsItem>
    </NDescriptions>
  </NModal>
</template>
