<!--
  文件用途：应用包发布说明编辑弹窗（仅 draft 状态可编辑）。
  核心逻辑：沿用选中包的当前说明便于追加，提交 updateMobileAppBundle 后回刷列表。
-->
<script setup lang="ts">
import { ref, watch } from 'vue'
import { NButton, NInput, NModal, NSpace, useMessage } from 'naive-ui'
import { updateMobileAppBundle, type MobileAppBundleItem } from '@/service/api'
import { $t } from '@/locales'

defineOptions({ name: 'AppBundleEditNotesModal' })

const props = defineProps<{
  show: boolean
  bundle: MobileAppBundleItem | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  success: []
}>()

const message = useMessage()

const submitting = ref(false)
const notes = ref('')

// 打开时回填目标包当前的发布说明。
watch(
  () => [props.show, props.bundle] as const,
  ([show, bundle]) => {
    if (show && bundle) {
      notes.value = bundle.release_notes ?? ''
    }
  },
  { immediate: true }
)

const close = () => {
  emit('update:show', false)
}

const handleSubmit = async () => {
  if (!props.bundle) return
  submitting.value = true
  try {
    const { error } = await updateMobileAppBundle(props.bundle.id, {
      release_notes: notes.value.trim()
    })
    if (!error) {
      message.success($t('page.app_bundle.updateSuccess'))
      close()
      emit('success')
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <NModal
    :show="props.show"
    preset="card"
    :title="$t('page.app_bundle.editTitle')"
    class="w-520px"
    @update:show="emit('update:show', $event)"
  >
    <NInput
      v-model:value="notes"
      type="textarea"
      :rows="5"
      :maxlength="2000"
      show-count
      :placeholder="$t('page.app_bundle.releaseNotesPlaceholder')"
    />
    <template #footer>
      <NSpace justify="end">
        <NButton @click="close">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="submitting" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
