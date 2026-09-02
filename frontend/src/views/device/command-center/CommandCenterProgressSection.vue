<script setup lang="ts">
defineProps<{
  steps: Array<{
    key: string
    index: string
    state: string
    tagType: 'default' | 'primary' | 'info' | 'success' | 'warning' | 'error'
    statusKey: string
    titleKey: string
    descKey: string
  }>
  previewLoading: boolean
  canPreviewCommandJobNow: boolean
}>()

const emit = defineEmits<{
  preview: []
}>()
</script>

<template>
  <section class="command-job-progress" :aria-label="$t('custom.commandCenter.progressTitle')">
    <div class="command-job-progress__head">
      <div>
        <NTag type="primary" size="small">{{ $t('custom.commandCenter.progressTag') }}</NTag>
        <h2>{{ $t('custom.commandCenter.progressTitle') }}</h2>
        <p>{{ $t('custom.commandCenter.progressDesc') }}</p>
      </div>
      <NButton
        size="small"
        secondary
        :loading="previewLoading"
        :disabled="!canPreviewCommandJobNow"
        @click="emit('preview')"
      >
        {{ $t('custom.commandCenter.progressPreviewAction') }}
      </NButton>
    </div>
    <div class="command-job-progress__steps">
      <div v-for="step in steps" :key="step.key" class="command-job-progress__step" :class="`is-${step.state}`">
        <div class="command-job-progress__step-top">
          <span class="command-job-progress__index">{{ step.index }}</span>
          <NTag size="small" :type="step.tagType">{{ $t(step.statusKey) }}</NTag>
        </div>
        <strong>{{ $t(step.titleKey) }}</strong>
        <p>{{ $t(step.descKey) }}</p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.command-job-progress {
  position: relative;
  overflow: hidden;
  display: grid;
  gap: 14px;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background:
    radial-gradient(circle at 10% 0%, rgba(59, 130, 246, 0.18), transparent 28%),
    radial-gradient(circle at 92% 18%, rgba(245, 158, 11, 0.14), transparent 30%),
    linear-gradient(135deg, #f8fbff 0%, #eff6ff 54%, #ffffff 100%);
  padding: 16px;
  box-shadow: 0 18px 44px rgba(15, 23, 42, 0.08);
}

.command-job-progress::after {
  position: absolute;
  top: -72px;
  right: -62px;
  width: 190px;
  height: 190px;
  border-radius: 999px;
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.16), rgba(14, 165, 233, 0.12));
  content: '';
  pointer-events: none;
}

.command-job-progress__head {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.command-job-progress__head > div {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.command-job-progress__head h2 {
  margin: 0;
  color: #0f172a;
  font-size: 20px;
  line-height: 1.2;
}

.command-job-progress__head p {
  max-width: 780px;
  margin: 0;
  color: #475569;
  font-size: 13px;
  line-height: 1.6;
}

.command-job-progress__steps {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.command-job-progress__step {
  display: grid;
  gap: 8px;
  min-width: 0;
  border: 1px solid rgba(148, 163, 184, 0.28);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.76);
  padding: 12px;
  box-shadow: 0 10px 26px rgba(15, 23, 42, 0.05);
  backdrop-filter: blur(10px);
}

.command-job-progress__step.is-current {
  border-color: #f59e0b;
  background: #fffbeb;
  box-shadow:
    0 0 0 2px rgba(245, 158, 11, 0.14),
    0 14px 30px rgba(245, 158, 11, 0.1);
}

.command-job-progress__step.is-done {
  border-color: #86efac;
  background: #f0fdf4;
}

.command-job-progress__step-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.command-job-progress__index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 999px;
  background: #0f172a;
  color: #fff;
  font-size: 12px;
  font-weight: 800;
}

.command-job-progress__step.is-done .command-job-progress__index {
  background: #16a34a;
}

.command-job-progress__step.is-current .command-job-progress__index {
  background: #d97706;
}

.command-job-progress__step strong {
  overflow: hidden;
  color: #0f172a;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.command-job-progress__step p {
  margin: 0;
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 900px) {
  .command-job-progress__head {
    flex-direction: column;
  }

  .command-job-progress__steps {
    grid-template-columns: 1fr;
  }
}
</style>
