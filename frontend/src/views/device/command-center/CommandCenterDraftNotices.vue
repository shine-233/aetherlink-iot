<script setup lang="ts">
type ReusedDraftNotice = {
  jobId: string
  identify?: string | null
}

type RouteDraftNotice = {
  identify?: string | null
  source?: string | null
}

defineProps<{
  reusedDraft: ReusedDraftNotice | null
  routeDraftNotice: RouteDraftNotice | null
  previewLoading: boolean
  canPreviewNow: boolean
}>()

const emit = defineEmits<{
  preview: []
  dismissReusedDraft: []
  dismissRouteDraft: []
}>()
</script>

<template>
  <NAlert v-if="reusedDraft" type="warning" :show-icon="false" class="command-reused-draft">
    <div>
      <strong>{{ $t('custom.commandCenter.reusedDraftTitle') }}</strong>
      <span>
        {{
          $t('custom.commandCenter.reusedDraftDesc')
            .replace('{jobId}', reusedDraft.jobId)
            .replace('{identify}', reusedDraft.identify || '--')
        }}
      </span>
    </div>
    <NSpace :size="[8, 8]">
      <NButton size="small" type="primary" :loading="previewLoading" :disabled="!canPreviewNow" @click="emit('preview')">
        {{ $t('custom.commandCenter.reusedDraftPreviewNow') }}
      </NButton>
      <NButton size="small" secondary @click="emit('dismissReusedDraft')">
        {{ $t('custom.commandCenter.reusedDraftDismiss') }}
      </NButton>
    </NSpace>
  </NAlert>

  <NAlert v-if="routeDraftNotice" type="info" :show-icon="false" class="command-route-draft">
    <div>
      <strong>{{ $t('custom.commandCenter.routeDraftTitle') }}</strong>
      <span>
        {{
          $t('custom.commandCenter.routeDraftDesc')
            .replace('{source}', routeDraftNotice.source || '--')
            .replace('{identify}', routeDraftNotice.identify || '--')
        }}
      </span>
    </div>
    <NSpace :size="[8, 8]">
      <NButton size="small" type="primary" :loading="previewLoading" :disabled="!canPreviewNow" @click="emit('preview')">
        {{ $t('custom.commandCenter.routeDraftPreviewNow') }}
      </NButton>
      <NButton size="small" secondary @click="emit('dismissRouteDraft')">
        {{ $t('custom.commandCenter.reusedDraftDismiss') }}
      </NButton>
    </NSpace>
  </NAlert>
</template>

<style scoped>
.command-reused-draft {
  border-color: #facc15;
  background: #fefce8;
}

.command-route-draft {
  border-color: #93c5fd;
  background: #eff6ff;
}

.command-reused-draft :deep(.n-alert-body__content),
.command-route-draft :deep(.n-alert-body__content) {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-reused-draft strong {
  display: block;
  margin-bottom: 4px;
  color: #854d0e;
}

.command-reused-draft span {
  color: #713f12;
  font-size: 12px;
}

.command-route-draft strong {
  display: block;
  margin-bottom: 4px;
  color: #1d4ed8;
}

.command-route-draft span {
  color: #1e3a8a;
  font-size: 12px;
}

@media (max-width: 900px) {
  .command-reused-draft :deep(.n-alert-body__content),
  .command-route-draft :deep(.n-alert-body__content) {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
