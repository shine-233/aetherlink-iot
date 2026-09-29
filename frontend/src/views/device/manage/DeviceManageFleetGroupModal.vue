<!--
  Bulk "add selected devices to group" dialog for the device-management fleet toolbar.
-->
<script setup lang="ts">
import { $t } from '@/locales'
import './device-fleet-modal.css'

defineProps<{
  selectedCount: number
  groupOptions: any[]
  assigning: boolean
}>()

const visible = defineModel<boolean>('show', { required: true })
const groupId = defineModel<string | number | null>('groupId', { required: true })

const emit = defineEmits<{ confirm: [] }>()
</script>

<template>
  <NModal v-model:show="visible" preset="card" class="max-w-520px">
    <template #header>{{ $t('custom.devicePage.addSelectedToGroupTitle') }}</template>
    <NFlex vertical :size="12">
      <NAlert type="info" :show-icon="false">
        {{ $t('custom.devicePage.addSelectedToGroupHint').replace('{count}', String(selectedCount)) }}
      </NAlert>
      <NTreeSelect
        v-model:value="groupId"
        :options="groupOptions"
        :placeholder="$t('custom.devicePage.selectTargetGroup')"
        filterable
        clearable
      />
      <NFlex justify="end" :size="8">
        <NButton @click="visible = false">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="assigning" @click="emit('confirm')">
          {{ $t('common.confirm') }}
        </NButton>
      </NFlex>
    </NFlex>
  </NModal>
</template>
