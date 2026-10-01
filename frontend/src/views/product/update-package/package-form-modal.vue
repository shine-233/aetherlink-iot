<!--
文件用途: 升级包新增/编辑弹窗（纯展示组件）。
核心逻辑: 接收页面注入的响应式表单模型与选项数据，负责表单布局与固件文件拖拽选择交互；
  保存/上传/选文件/远程搜索设备配置通过事件上抛，由页面统一编排。
关键注意事项: form 为响应式模型 prop，直接双向绑定其字段（项目 eslint shallowOnly 允许）；
  每次打开时清空原生 input 与拖拽态，保证连续选择同一文件仍能触发 change（迁移前 resetForm 行为）。
重构建议: 表单校验加重后可引入 naive-ui Form rules 并在此组件内托管校验状态。
-->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { $t } from '@/locales'
import type { DeviceConfigOption } from './ota-package-types'
import type { OtaPackageFormModel } from './use-ota-package-form'

const props = defineProps<{
  show: boolean
  isEditing: boolean
  saving: boolean
  uploading: boolean
  form: OtaPackageFormModel
  selectedFile: File | null
  packageTypeOptions: Array<{ label: string; value: number }>
  signatureOptions: Array<{ label: string; value: string }>
  deviceConfigLoading: boolean
  deviceConfigOptions: DeviceConfigOption[]
  isReturnToOtaTaskFlow: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  save: []
  upload: []
  'select-file': [file: File | null]
  'search-device-configs': [keyword: string]
}>()

const visible = computed({
  get: () => props.show,
  set: (value) => emit('update:show', value)
})

const fileDragActive = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)

watch(
  () => props.show,
  async (show) => {
    if (!show) return
    fileDragActive.value = false
    await nextTick()
    if (fileInputRef.value) fileInputRef.value.value = ''
  }
)

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  emit('select-file', input.files?.[0] || null)
}

function onFileDrop(event: DragEvent) {
  fileDragActive.value = false
  emit('select-file', event.dataTransfer?.files?.[0] || null)
}

function onFileDragLeave(event: DragEvent) {
  const target = event.currentTarget as HTMLElement
  const related = event.relatedTarget as Node | null
  if (related && target.contains(related)) return
  fileDragActive.value = false
}

function onDeviceConfigSearch(keyword: string) {
  emit('search-device-configs', keyword)
}
</script>

<template>
  <NModal
    v-model:show="visible"
    preset="card"
    class="package-modal"
    :title="isEditing ? $t('page.product.update-package.packageEdit') : $t('page.product.update-package.packageAdd')"
  >
    <NForm label-placement="top">
      <NGrid cols="1 s:2" responsive="screen" :x-gap="16">
        <NFormItemGi :label="$t('page.product.update-package.packageName')" required>
          <NInput v-model:value="form.name" :placeholder="$t('page.product.update-package.packageNamePlaceholder')" />
        </NFormItemGi>
        <NFormItemGi :label="$t('page.product.update-package.versionCode')" required>
          <NInput
            v-model:value="form.version"
            :placeholder="$t('page.product.update-package.versionCodePlaceholder')"
          />
        </NFormItemGi>
        <NFormItemGi :label="$t('page.product.update-package.version')">
          <NInput
            v-model:value="form.target_version"
            :placeholder="$t('page.product.update-package.versionPlaceholder')"
          />
        </NFormItemGi>
        <NFormItemGi :label="$t('page.product.update-package.deviceConfig')" required>
          <NSelect
            v-model:value="form.device_config_id"
            filterable
            remote
            :loading="deviceConfigLoading"
            :options="deviceConfigOptions"
            :placeholder="$t('page.product.update-package.productPlaceholder')"
            @search="onDeviceConfigSearch"
          />
        </NFormItemGi>
        <NFormItemGi :label="$t('page.product.update-package.type')" required>
          <NSelect v-model:value="form.package_type" :options="packageTypeOptions" />
        </NFormItemGi>
        <NFormItemGi :label="$t('page.product.update-package.signMode')" required>
          <NSelect v-model:value="form.signature_type" :options="signatureOptions" />
        </NFormItemGi>
        <NFormItemGi :label="$t('page.product.update-package.moduleName')">
          <NInput v-model:value="form.module" />
        </NFormItemGi>
        <NFormItemGi :label="$t('page.product.update-package.package')" required>
          <NSpace vertical class="file-box">
            <div
              class="file-drop-zone"
              :class="{ 'is-dragging': fileDragActive }"
              @dragenter.prevent="fileDragActive = true"
              @dragover.prevent="fileDragActive = true"
              @dragleave.prevent="onFileDragLeave"
              @drop.prevent="onFileDrop"
            >
              <input
                ref="fileInputRef"
                type="file"
                accept=".bin,.hex,.elf,.tar,.gz,.gzip,.zip,.apk,.dav,.pack"
                @change="onFileChange"
              />
              <div class="file-drop-hint">{{ $t('page.product.update-package.dragDropHint') }}</div>
              <div class="file-drop-types">{{ $t('page.product.update-package.fileTypeHint') }}</div>
            </div>
            <NSpace align="center">
              <NButton :loading="uploading" :disabled="!selectedFile" @click="emit('upload')">
                {{ $t('page.product.list.file') }}
              </NButton>
              <span class="file-name">
                {{
                  selectedFile
                    ? `${$t('page.product.update-package.selectedFile')}: ${selectedFile.name}`
                    : form.package_url || '-'
                }}
              </span>
            </NSpace>
          </NSpace>
        </NFormItemGi>
        <NFormItemGi :label="$t('page.product.update-package.package')">
          <NInput v-model:value="form.package_url" />
        </NFormItemGi>
      </NGrid>
      <NFormItem :label="$t('page.product.update-package.customInfo')">
        <NInput v-model:value="form.additional_info" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" />
      </NFormItem>
      <NFormItem :label="$t('page.product.update-package.desc')">
        <NInput v-model:value="form.description" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="visible = false">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="saving" @click="emit('save')">
          {{ isReturnToOtaTaskFlow ? $t('page.product.update-package.returnToOtaSaveAction') : $t('common.save') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.package-modal {
  width: min(760px, calc(100vw - 32px));
}

.file-box {
  width: 100%;
}

.file-drop-zone {
  display: grid;
  min-height: 88px;
  padding: 14px;
  border: 1px dashed var(--border-color);
  border-radius: 6px;
  background: var(--card-color);
  gap: 8px;
  place-items: center;
  transition:
    border-color 0.2s ease,
    background-color 0.2s ease;
}

.file-drop-zone.is-dragging {
  border-color: var(--primary-color);
  background: var(--primary-color-hover);
}

.file-drop-hint {
  color: var(--text-color-3);
  font-size: 13px;
}

.file-drop-types {
  color: var(--text-color-3);
  font-size: 12px;
}

.file-name {
  max-width: 260px;
  overflow: hidden;
  color: var(--text-color-2);
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
