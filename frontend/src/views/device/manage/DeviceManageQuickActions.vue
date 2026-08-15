<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { deviceUpdate } from '@/service/api/device'
import { createRdiShareToken } from '@/service/api/rdi'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'

interface DeviceManageQuickActionRow {
  id?: string | number
  name?: string
  description?: string
  device_number?: string
  pid_number?: string
}

const emit = defineEmits<{
  (event: 'updated'): void
}>()

const DEFAULT_SHARE_EXPIRY_SECONDS = 7 * 24 * 60 * 60

const editDeviceVisible = ref(false)
const editDeviceSaving = ref(false)
const editDeviceForm = reactive({
  id: '',
  name: '',
  description: ''
})

const shareDeviceVisible = ref(false)
const shareDeviceLoading = ref(false)
const shareDeviceForm = reactive({
  id: '',
  name: ''
})
const shareExpiresIn = ref(DEFAULT_SHARE_EXPIRY_SECONDS)
const shareLink = ref('')

const shareExpiryOptions = computed(() => [
  { label: '24h', value: 24 * 60 * 60 },
  { label: '7d', value: DEFAULT_SHARE_EXPIRY_SECONDS },
  { label: '30d', value: 30 * 24 * 60 * 60 }
])

const shareExpiresAt = computed(() => {
  if (!shareLink.value) return ''
  return new Date((Math.floor(Date.now() / 1000) + shareExpiresIn.value) * 1000).toLocaleString()
})

function openEditDevice(row: DeviceManageQuickActionRow) {
  editDeviceForm.id = String(row?.id || '')
  editDeviceForm.name = String(row?.name || '')
  editDeviceForm.description = String(row?.description || '')
  editDeviceVisible.value = true
}

async function saveDeviceEdit() {
  const name = editDeviceForm.name.trim()
  if (!name) {
    window.$message?.error($t('custom.devicePage.enterDeviceName'))
    return
  }

  editDeviceSaving.value = true
  try {
    const { error } = await deviceUpdate({
      id: editDeviceForm.id,
      name,
      description: editDeviceForm.description.trim()
    })
    if (!error) {
      window.$message?.success($t('common.saveSuccess'))
      editDeviceVisible.value = false
      emit('updated')
    }
  } finally {
    editDeviceSaving.value = false
  }
}

function openShareDevice(row: DeviceManageQuickActionRow) {
  shareDeviceForm.id = String(row?.id || '')
  shareDeviceForm.name = String(row?.name || row?.device_number || row?.pid_number || '')
  shareExpiresIn.value = shareExpiryOptions.value[1]?.value ?? DEFAULT_SHARE_EXPIRY_SECONDS
  shareLink.value = ''
  shareDeviceVisible.value = true
}

async function copyShareLink() {
  if (!shareLink.value) return
  const copied = await writeClipboardText(shareLink.value)
  if (copied) {
    window.$message?.success($t('rdi.device.shareLinkCopied'))
  } else {
    window.$message?.error($t('common.copyFailed'))
  }
}

async function generateShareLink() {
  if (!shareDeviceForm.id) return

  shareDeviceLoading.value = true
  try {
    const { error, data } = await createRdiShareToken(shareDeviceForm.id, { expires_in: shareExpiresIn.value })
    if (error || !data) return

    const path = data.share_path || (data.token ? `/device/share?share_token=${encodeURIComponent(data.token)}` : '')
    if (!path) return

    shareLink.value = `${window.location.origin}${path.startsWith('/') ? path : `/${path}`}`
    await copyShareLink()
  } finally {
    shareDeviceLoading.value = false
  }
}

defineExpose({
  openEditDevice,
  openShareDevice
})
</script>

<template>
  <NModal v-model:show="editDeviceVisible" preset="card" class="max-w-520px">
    <template #header>{{ $t('common.editNameAndDesc') }}</template>
    <NFlex vertical :size="12">
      <div class="device-edit-field">
        <div class="device-edit-field__label">{{ $t('custom.devicePage.deviceName') }}</div>
        <NInput
          v-model:value="editDeviceForm.name"
          maxlength="255"
          show-count
          :placeholder="$t('custom.devicePage.enterDeviceName')"
        />
      </div>
      <div class="device-edit-field">
        <div class="device-edit-field__label">{{ $t('custom.devicePage.description') }}</div>
        <NInput
          v-model:value="editDeviceForm.description"
          type="textarea"
          maxlength="500"
          show-count
          :autosize="{ minRows: 3, maxRows: 6 }"
        />
      </div>
      <NFlex justify="end">
        <NButton @click="editDeviceVisible = false">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="editDeviceSaving" @click="saveDeviceEdit">
          {{ $t('common.save') }}
        </NButton>
      </NFlex>
    </NFlex>
  </NModal>

  <NModal v-model:show="shareDeviceVisible" preset="card" class="max-w-560px">
    <template #header>{{ $t('rdi.device.shareTitle') }}</template>
    <NFlex vertical :size="12">
      <NAlert type="info" :show-icon="false">
        {{ shareDeviceForm.name || shareDeviceForm.id }}
      </NAlert>
      <NFlex align="center" :size="8" wrap>
        <NSelect v-model:value="shareExpiresIn" :options="shareExpiryOptions" class="share-device-select" />
        <NButton type="primary" :loading="shareDeviceLoading" @click="generateShareLink">
          {{ $t('rdi.device.generateShareLink') }}
        </NButton>
      </NFlex>
      <NInput :value="shareLink" readonly :placeholder="$t('rdi.device.generateShareLink')" />
      <NText v-if="shareExpiresAt" depth="3">{{ shareExpiresAt }}</NText>
      <NFlex justify="end">
        <NButton @click="shareDeviceVisible = false">{{ $t('common.cancel') }}</NButton>
      </NFlex>
    </NFlex>
  </NModal>
</template>

<style scoped lang="scss">
.device-edit-field {
  display: grid;
  gap: 6px;
}

.device-edit-field__label {
  color: #333;
  font-size: 14px;
  font-weight: 500;
  line-height: 1.4;
}

.share-device-select {
  width: 180px;
}
</style>
