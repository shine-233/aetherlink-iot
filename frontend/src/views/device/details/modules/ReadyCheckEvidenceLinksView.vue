<script setup lang="ts">
import { formatReadyCheckDeepLink, type ReadyCheckDeepLink } from './ready-check-deep-links'

defineProps<{
  links: ReadyCheckDeepLink[]
}>()

const emit = defineEmits<{
  open: [link: ReadyCheckDeepLink]
  copy: [link: ReadyCheckDeepLink]
  copyAll: []
}>()
</script>

<template>
  <section class="ready-check-deep-links" data-testid="device-ready-check-deep-links">
    <div class="ready-check-deep-links__head">
      <div>
        <h3>{{ $t('custom.device_details.readyCheckDeepLinksTitle') }}</h3>
        <p>{{ $t('custom.device_details.readyCheckDeepLinksDesc') }}</p>
      </div>
      <NButton class="ready-check-deep-links__copy-all" size="small" secondary @click="emit('copyAll')">
        <span>{{ $t('custom.device_details.readyCheckDeepLinkCopyAll') }}</span>
        <span class="ready-check-deep-links__button-context">
          {{ $t('custom.device_details.readyCheckEvidenceBoundaryValue') }}
        </span>
      </NButton>
    </div>
    <div class="ready-check-deep-links__grid">
      <article v-for="(link, index) in links" :key="link.key" class="ready-check-deep-links__item">
        <div class="ready-check-deep-links__item-head">
          <span class="ready-check-deep-links__step" aria-hidden="true">{{ index + 1 }}</span>
          <strong>{{ $t(link.labelKey) }}</strong>
        </div>
        <span>{{ $t(link.descriptionKey) }}</span>
        <code class="ready-check-deep-links__target">{{ formatReadyCheckDeepLink(link) }}</code>
        <small class="ready-check-deep-links__boundary">
          <span>{{ $t('custom.device_details.readyCheckEvidenceBoundary') }}</span>
          <span>{{ $t(link.boundaryKey) }}</span>
        </small>
        <div class="ready-check-deep-links__actions">
          <NButton size="small" secondary type="primary" @click="emit('open', link)">
            {{ $t('custom.device_details.readyCheckDeepLinkOpen') }}
            <span class="ready-check-deep-links__button-context">{{ $t(link.labelKey) }}</span>
          </NButton>
          <NButton size="small" secondary @click="emit('copy', link)">
            {{ $t('custom.device_details.readyCheckDeepLinkCopy') }}
            <span class="ready-check-deep-links__button-context">{{ $t(link.labelKey) }}</span>
          </NButton>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.ready-check-deep-links {
  display: grid;
  gap: 12px;
  min-width: 0;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
  padding: 14px;
}

.ready-check-deep-links__head {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 12px;
}

.ready-check-deep-links__head > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.ready-check-deep-links__head h3 {
  margin: 0;
  color: #0f172a;
  font-size: 15px;
}

.ready-check-deep-links__head p {
  margin: 0;
  color: #64748b;
  font-size: 13px;
  line-height: 1.5;
}

.ready-check-deep-links__copy-all {
  flex-shrink: 0;
}

.ready-check-deep-links__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.ready-check-deep-links__item {
  display: grid;
  gap: 6px;
  min-width: 0;
  border-radius: 6px;
  background: #f8fafc;
  padding: 10px;
}

.ready-check-deep-links__item-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.ready-check-deep-links__step {
  display: inline-grid;
  flex: 0 0 auto;
  width: 22px;
  height: 22px;
  place-items: center;
  border-radius: 999px;
  background: #dbeafe;
  color: #1d4ed8;
  font-size: 12px;
  font-weight: 700;
}

.ready-check-deep-links__item strong {
  min-width: 0;
  color: #0f172a;
  font-size: 13px;
}

.ready-check-deep-links__item span,
.ready-check-deep-links__item small {
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.ready-check-deep-links__boundary {
  display: grid;
  gap: 3px;
  border-left: 3px solid #bfdbfe;
  padding-left: 8px;
}

.ready-check-deep-links__boundary span:first-child {
  color: #334155;
  font-weight: 600;
}

.ready-check-deep-links__target {
  overflow-wrap: anywhere;
  border-radius: 6px;
  background: #eef6ff;
  padding: 6px 8px;
  color: #1d4ed8;
  font-size: 12px;
  line-height: 1.4;
}

.ready-check-deep-links__actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.ready-check-deep-links__actions :deep(.n-button) {
  width: 100%;
}

.ready-check-deep-links__button-context::before {
  content: "·";
  padding: 0 4px;
}

@media (max-width: 768px) {
  .ready-check-deep-links__head {
    display: grid;
  }

  .ready-check-deep-links__copy-all {
    flex-shrink: 0;
  }

  .ready-check-deep-links__grid {
    grid-template-columns: 1fr;
  }
}
</style>
