<!--
  文件用途：应用包上传登记弹窗（multipart：file+platform+version+release_notes）。
  核心逻辑：表单校验（版本必填、平台决定 accept 扩展名）→ FormData 提交 uploadMobileAppBundle；
  新包固定落 draft，重复登记由后端 202006 拒绝，这里只负责请求与结果提示。
-->
<script setup lang="ts">
import { reactive, ref } from 'vue'
import { NButton, NForm, NFormItem, NInput, NModal, NSelect, NSpace, NUpload, useMessage } from 'naive-ui'
import type { UploadCustomRequestOptions, UploadFileInfo } from 'naive-ui'
import { uploadMobileAppBundle, type MobileAppBundlePlatform } from '@/service/api'
import { $t } from '@/locales'
import { ACCEPT_BY_PLATFORM, PLATFORM_OPTIONS } from '../constants'

defineOptions({ name: 'AppBundleUploadModal' })

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  success: []
}>()

const message = useMessage()

const uploading = ref(false)
const uploadFile = ref<File | null>(null)
const uploadForm = reactive({
  platform: 'android' as MobileAppBundlePlatform,
  version: '',
  releaseNotes: ''
})

// naive-ui 要求 custom-request 存在时才不发送原生请求；上传动作统一在提交时处理。
const noopCustomRequest = (_options: UploadCustomRequestOptions) => {
  /* 上传统一在 handleUploadSubmit 处理 */
}

const handleUploadFileChange = (options: { file: UploadFileInfo }) => {
  uploadFile.value = options.file?.file ?? null
}

// 每次打开弹窗都从干净表单开始，避免上一次的残留文件被误提交。
const resetForm = () => {
  uploadFile.value = null
  uploadForm.platform = 'android'
  uploadForm.version = ''
  uploadForm.releaseNotes = ''
}

const close = () => {
  emit('update:show', false)
}

const handleUploadSubmit = async () => {
  const version = uploadForm.version.trim()
  if (!version) {
    message.error($t('page.app_bundle.versionPlaceholder'))
    return
  }
  if (!uploadFile.value) {
    message.error($t('page.app_bundle.filePlaceholder'))
    return
  }
  const formData = new FormData()
  formData.append('file', uploadFile.value)
  formData.append('platform', uploadForm.platform)
  formData.append('version', version)
  if (uploadForm.releaseNotes.trim()) {
    formData.append('release_notes', uploadForm.releaseNotes.trim())
  }
  uploading.value = true
  try {
    const { error } = await uploadMobileAppBundle(formData)
    if (!error) {
      message.success($t('page.app_bundle.uploadSuccess'))
      close()
      emit('success')
    }
  } finally {
    uploading.value = false
  }
}

defineExpose({ resetForm })
</script>

<template>
  <NModal
    :show="props.show"
    preset="card"
    :title="$t('page.app_bundle.uploadTitle')"
    class="w-560px"
    @update:show="emit('update:show', $event)"
    @after-leave="resetForm"
  >
    <NForm label-placement="left" label-width="100">
      <NFormItem :label="$t('page.app_bundle.platform')">
        <NSelect v-model:value="uploadForm.platform" :options="PLATFORM_OPTIONS" />
      </NFormItem>
      <NFormItem :label="$t('page.app_bundle.version')">
        <NInput
          v-model:value="uploadForm.version"
          :placeholder="$t('page.app_bundle.versionPlaceholder')"
          :maxlength="50"
        />
      </NFormItem>
      <NFormItem :label="$t('page.app_bundle.file')">
        <NUpload
          :show-file-list="false"
          :accept="ACCEPT_BY_PLATFORM[uploadForm.platform]"
          :custom-request="noopCustomRequest"
          :max="1"
          @change="handleUploadFileChange"
        >
          <NButton>{{ uploadFile ? uploadFile.name : $t('page.app_bundle.filePlaceholder') }}</NButton>
        </NUpload>
      </NFormItem>
      <NFormItem :label="$t('page.app_bundle.releaseNotes')">
        <NInput
          v-model:value="uploadForm.releaseNotes"
          type="textarea"
          :rows="4"
          :placeholder="$t('page.app_bundle.releaseNotesPlaceholder')"
          :maxlength="2000"
          show-count
        />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="close">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="uploading" @click="handleUploadSubmit">
          {{ $t('common.confirm') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
