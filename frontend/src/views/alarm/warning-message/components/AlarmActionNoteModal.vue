<!--
文件用途：告警单条/批量处置共用的「带备注确认」弹窗。
核心逻辑：展示处置提示与可选备注，确认/取消通过 emit 交还给父级组合函数执行。
关键注意事项：loading 期间禁止遮罩关闭与取消，避免请求在途时丢失处置结果。
-->
<script setup lang="ts">
import { NButton, NCard, NFlex, NFormItem, NInput, NModal } from 'naive-ui'
import { $t } from '@/locales'

defineProps<{
  title: string
  hint: string
  loading: boolean
  maxLength: number
  placeholder: string
}>()

const show = defineModel<boolean>('show', { required: true })
const note = defineModel<string>('note', { required: true })

defineEmits<{
  cancel: []
  confirm: []
}>()
</script>

<template>
  <NModal v-model:show="show" aria-label="dialog" class="max-w-[600px]" :mask-closable="!loading">
    <NCard :title="title" class="alarm-action-modal-card">
      <div class="alarm-action-note-hint">
        <div class="whitespace-pre-line">{{ hint }}</div>
      </div>
      <NFormItem :label="$t('custom.alarmPage.batchActionNoteLabel')">
        <NInput
          v-model:value="note"
          type="textarea"
          :maxlength="maxLength"
          show-count
          :autosize="{ minRows: 3, maxRows: 5 }"
          :placeholder="placeholder"
        />
      </NFormItem>
      <NFlex justify="flex-end" class="mt-4">
        <NButton :disabled="loading" @click="$emit('cancel')">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="loading" @click="$emit('confirm')">
          {{ $t('common.confirm') }}
        </NButton>
      </NFlex>
    </NCard>
  </NModal>
</template>

<style scoped>
.alarm-action-modal-card {
  width: min(96vw, 600px);
}

.alarm-action-note-hint {
  margin-bottom: 16px;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
  color: #475569;
  font-size: 13px;
  line-height: 1.5;
}
</style>
