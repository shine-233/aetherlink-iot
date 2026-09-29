<!--
  文件用途：调度注册事件的创建/编辑弹窗。
  核心逻辑：表单字段按 event_type 条件显示（scene→ref_id+cron、report→cron、rpc→next_run_at）；
    校验与请求体由 calendar-model 构建；保存成功后 emit('saved')，由父组件刷新列表。
    同场景仅允许一条启用定时触发，后端拒绝时如实提示（请求层已弹错误），不做前端预判。
-->
<script setup lang="ts">
import { ref, watch } from 'vue'
import { NButton, NDatePicker, NForm, NFormItem, NInput, NModal, NSelect, NSpace, NSwitch, useMessage } from 'naive-ui'
import { createSchedulerEvent, updateSchedulerEvent, type SchedulerEventItem } from '@/service/api'
import { $t } from '@/locales'
import {
  buildEventPayload,
  emptyFormModel,
  formModelFromRow,
  validateFormModel,
  type SchedulerFormModel
} from '../calendar-model'

const props = defineProps<{
  /** null → create; otherwise edit this row. */
  row: SchedulerEventItem | null
  sourceTypeOptions: { label: string; value: SchedulerFormModel['event_type'] }[]
}>()

const show = defineModel<boolean>('show', { required: true })
const emit = defineEmits<{ (event: 'saved'): void }>()

const message = useMessage()
const saving = ref(false)
const model = ref<SchedulerFormModel>(emptyFormModel())

// 每次打开按当前 row 重置表单，避免上一次编辑残留。
watch(
  show,
  (open) => {
    if (open) model.value = props.row ? formModelFromRow(props.row) : emptyFormModel()
  },
  { immediate: true }
)

async function submit() {
  const errorKey = validateFormModel(model.value)
  if (errorKey) {
    message.error($t(errorKey))
    return
  }
  saving.value = true
  try {
    const payload = buildEventPayload(model.value)
    if (props.row) {
      await updateSchedulerEvent(props.row.id, payload)
      message.success($t('common.updateSuccess'))
    } else {
      await createSchedulerEvent({ ...payload, event_type: model.value.event_type })
      message.success($t('page.schedulerCalendar.createSuccess'))
    }
    show.value = false
    emit('saved')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <NModal
    v-model:show="show"
    preset="card"
    :title="row ? $t('page.schedulerCalendar.editEvent') : $t('page.schedulerCalendar.createEvent')"
    class="scheduler-form-modal"
  >
    <NForm label-placement="left" label-width="110">
      <NFormItem :label="$t('page.schedulerCalendar.formName')" required>
        <NInput v-model:value="model.name" :maxlength="128" />
      </NFormItem>
      <NFormItem :label="$t('page.schedulerCalendar.formType')" required>
        <NSelect v-model:value="model.event_type" :options="sourceTypeOptions" :disabled="!!row" />
      </NFormItem>
      <NFormItem
        v-if="model.event_type !== 'report'"
        :label="$t('page.schedulerCalendar.formRefId')"
        :required="model.event_type === 'scene'"
      >
        <NInput v-model:value="model.ref_id" :maxlength="64" :placeholder="$t('page.schedulerCalendar.formRefIdHint')" />
      </NFormItem>
      <NFormItem v-if="model.event_type !== 'rpc'" :label="$t('page.schedulerCalendar.formCron')" required>
        <NInput v-model:value="model.cron" :maxlength="64" :placeholder="$t('page.schedulerCalendar.formCronHint')" />
      </NFormItem>
      <NFormItem v-if="model.event_type === 'rpc'" :label="$t('page.schedulerCalendar.formRunAt')" required>
        <NDatePicker v-model:value="model.next_run_at" type="datetime" clearable />
      </NFormItem>
      <NFormItem :label="$t('page.schedulerCalendar.enabled')">
        <NSwitch v-model:value="model.enabled" />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="show = false">{{ $t('page.schedulerCalendar.cancel') }}</NButton>
        <NButton type="primary" :loading="saving" @click="submit">
          {{ $t('page.schedulerCalendar.save') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.scheduler-form-modal {
  width: 720px;
}
</style>
