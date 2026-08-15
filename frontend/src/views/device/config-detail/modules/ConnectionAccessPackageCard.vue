<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  summaryRows: Array<{
    label: string
    value: string
  }>
  checklist: string[]
}>()

const emit = defineEmits<{
  copyPackage: []
  copyConfig: []
}>()

const { t } = useI18n()
</script>

<template>
  <NCard class="connection-workbench mt-4" size="small" :bordered="false">
    <div class="connection-workbench__header">
      <div class="connection-workbench__intro">
        <div class="connection-workbench__eyebrow">{{ t('generate.connectionWorkbench.eyebrow') }}</div>
        <div class="connection-workbench__title">{{ t('generate.connectionWorkbench.title') }}</div>
        <div class="connection-workbench__desc">{{ t('generate.connectionWorkbench.desc') }}</div>
      </div>
      <NSpace class="connection-workbench__actions">
        <NButton secondary type="primary" @click="emit('copyPackage')">
          {{ t('generate.connectionWorkbench.copyPackage') }}
        </NButton>
        <NButton secondary @click="emit('copyConfig')">
          {{ t('generate.connectionWorkbench.copyConfig') }}
        </NButton>
      </NSpace>
    </div>

    <div class="connection-workbench__summary">
      <div v-for="row in summaryRows" :key="row.label" class="connection-workbench__summary-item">
        <span>{{ row.label }}</span>
        <strong>{{ row.value }}</strong>
      </div>
    </div>

    <NAlert type="info" :show-icon="false" class="connection-workbench__alert">
      {{ t('generate.connectionWorkbench.safeCopyHint') }}
    </NAlert>

    <div class="connection-workbench__checklist">
      <div class="connection-workbench__checklist-title">{{ t('generate.connectionWorkbench.checklistTitle') }}</div>
      <div v-for="item in checklist" :key="item" class="connection-workbench__checklist-item">
        <span class="connection-workbench__check-icon">✓</span>
        <span>{{ item }}</span>
      </div>
    </div>
  </NCard>
</template>

<style scoped lang="scss">
.connection-workbench {
  border: 1px solid rgba(39, 119, 255, 0.14);
  background:
    radial-gradient(circle at top left, rgba(39, 119, 255, 0.12), transparent 34%),
    linear-gradient(135deg, #f7fbff 0%, #ffffff 58%, #f9fbf5 100%);
  border-radius: 18px;

  :deep(.n-card__content) {
    padding: 18px;
  }
}

.connection-workbench__header {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: flex-start;
}

.connection-workbench__intro {
  min-width: 0;
}

.connection-workbench__actions {
  flex: 0 0 auto;
}

.connection-workbench__eyebrow {
  color: #2777ff;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.connection-workbench__title {
  margin-top: 4px;
  color: #17233d;
  font-size: 18px;
  font-weight: 750;
}

.connection-workbench__desc {
  max-width: 720px;
  margin-top: 6px;
  color: #526173;
  line-height: 1.6;
}

.connection-workbench__summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
  margin-top: 18px;
}

.connection-workbench__summary-item {
  min-width: 0;
  padding: 12px;
  border: 1px solid rgba(23, 35, 61, 0.08);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.74);

  span {
    display: block;
    color: #69778a;
    font-size: 12px;
  }

  strong {
    display: block;
    margin-top: 4px;
    overflow: hidden;
    color: #17233d;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.connection-workbench__alert {
  margin-top: 14px;
  border-radius: 12px;
}

.connection-workbench__checklist {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 10px;
  margin-top: 14px;
}

.connection-workbench__checklist-title {
  grid-column: 1 / -1;
  color: #17233d;
  font-weight: 700;
}

.connection-workbench__checklist-item {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  padding: 10px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.72);
  color: #526173;
  line-height: 1.45;
}

.connection-workbench__check-icon {
  display: inline-flex;
  flex: 0 0 18px;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 999px;
  background: #e7f8ef;
  color: #13a058;
  font-size: 12px;
  font-weight: 700;
}

@media (max-width: 960px) {
  .connection-workbench__header {
    flex-direction: column;
  }

  .connection-workbench__actions {
    width: 100%;
  }
}
</style>
