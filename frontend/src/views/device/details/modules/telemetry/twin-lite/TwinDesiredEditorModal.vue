<!--
  文件用途: Twin Lite 期望值（desired）新建 / 编辑弹窗。
  核心逻辑: 直接绑定 useTwinLite 暴露的 desiredForm（reactive 对象，按引用传入）；保存、取消、使用上报值通过 emit 交回。
  关键注意事项: 命令类行不可编辑（由调用方拦截）；“使用上报值”仅在编辑已有且有上报值的行时可用。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'
import type { TwinLiteRow } from './twin-lite-normalizer'
import type { EditableTwinSource } from './useTwinLite'

const props = defineProps<{
  form: { source: EditableTwinSource; key: string; desiredText: string }
  editingRow: TwinLiteRow | null
  error: string
  saving: boolean
  sourceLabels: Record<string, string>
}>()

defineEmits<{
  cancel: []
  submit: []
  useReported: []
}>()

const show = defineModel<boolean>('show', { required: true })

const title = computed(() =>
  props.editingRow ? $t('custom.device_details.twinEditDesired') : $t('custom.device_details.twinSetDesired')
)
const sourceOptions = computed(() => [
  { label: props.sourceLabels.telemetry, value: 'telemetry' },
  { label: props.sourceLabels.attribute, value: 'attribute' }
])
</script>

<template>
  <n-modal v-model:show="show" preset="card" class="w-640px max-w-[calc(100vw-24px)]" :title="title">
    <n-space vertical size="large">
      <n-alert v-if="error" type="warning" :show-icon="false">
        {{ error }}
      </n-alert>

      <n-form label-placement="top" :show-feedback="false">
        <n-form-item :label="$t('custom.device_details.twinDesiredSource')">
          <n-select v-model:value="form.source" :options="sourceOptions" />
        </n-form-item>
        <n-form-item :label="$t('custom.device_details.twinDesiredKey')">
          <n-input v-model:value="form.key" />
        </n-form-item>
        <n-form-item :label="$t('custom.device_details.twinDesiredValue')">
          <n-input
            v-model:value="form.desiredText"
            type="textarea"
            :autosize="{ minRows: 6, maxRows: 12 }"
            :placeholder="$t('custom.device_details.twinDesiredValueHint')"
          />
        </n-form-item>
      </n-form>

      <div class="flex items-center justify-between gap-12px">
        <n-button quaternary :disabled="!editingRow || editingRow.reported === null" @click="$emit('useReported')">
          {{ $t('custom.device_details.twinUseReported') }}
        </n-button>
        <div class="flex items-center gap-12px">
          <n-button @click="$emit('cancel')">
            {{ $t('generate.cancel') }}
          </n-button>
          <n-button type="primary" :loading="saving" @click="$emit('submit')">
            {{ $t('custom.device_details.twinSaveDesired') }}
          </n-button>
        </div>
      </div>
    </n-space>
  </n-modal>
</template>
