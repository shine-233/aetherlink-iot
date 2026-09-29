<!--
文件用途：告警列表为空时的引导卡片（规则、筛选、闭环三条提示 + 重置筛选）。
-->
<script setup lang="ts">
import { NButton, NCard, NTag } from 'naive-ui'
import { $t } from '@/locales'

defineEmits<{ reset: [] }>()

const items = [
  {
    key: 'rule',
    title: 'custom.alarmPage.emptyGuideRuleTitle',
    desc: 'custom.alarmPage.emptyGuideRuleDesc',
    type: 'info'
  },
  {
    key: 'filter',
    title: 'custom.alarmPage.emptyGuideFilterTitle',
    desc: 'custom.alarmPage.emptyGuideFilterDesc',
    type: 'warning'
  },
  {
    key: 'closure',
    title: 'custom.alarmPage.emptyGuideClosureTitle',
    desc: 'custom.alarmPage.emptyGuideClosureDesc',
    type: 'success'
  }
] as const
</script>

<template>
  <NCard embedded size="small" class="alarm-empty-guide">
    <div class="alarm-empty-guide__head">
      <div>
        <div class="alarm-empty-guide__title">{{ $t('custom.alarmPage.emptyGuideTitle') }}</div>
        <div class="alarm-empty-guide__desc">{{ $t('custom.alarmPage.emptyGuideDesc') }}</div>
      </div>
      <NButton size="small" secondary @click="$emit('reset')">
        {{ $t('custom.alarmPage.emptyGuideResetAction') }}
      </NButton>
    </div>
    <div class="alarm-empty-guide__items">
      <div v-for="item in items" :key="item.key" class="alarm-empty-guide__item">
        <NTag :type="item.type" size="small">{{ $t(item.title) }}</NTag>
        <span>{{ $t(item.desc) }}</span>
      </div>
    </div>
  </NCard>
</template>

<style scoped lang="scss">
.alarm-empty-guide {
  margin-bottom: 12px;
  border: 1px solid #dbeafe;
  background: linear-gradient(135deg, #f8fbff 0%, #eff6ff 100%);
}

.alarm-empty-guide__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.alarm-empty-guide__title {
  color: #0f172a;
  font-weight: 700;
}

.alarm-empty-guide__desc {
  margin-top: 4px;
  color: #475569;
  font-size: 12px;
  line-height: 1.6;
}

.alarm-empty-guide__items {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-top: 12px;
}

.alarm-empty-guide__item {
  display: grid;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.72);
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 900px) {
  .alarm-empty-guide__items {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@include mobile {
  .alarm-empty-guide__items {
    grid-template-columns: 1fr;
  }
}
</style>
