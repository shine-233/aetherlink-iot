<!--
  文件用途: 属性下发弹窗“属性配置”页签：属性集勾选 + 取值输入，支持全选 / 半选状态。
  核心逻辑: 直接修改传入行对象的 checked / inputValue（行数组由 useDistributionDialogState 持有）。
-->
<script setup lang="ts">
import { computed } from 'vue'
import type { DistributionAttributeRow } from './useDistributionTable'

const props = defineProps<{
  loading: boolean
  rows: DistributionAttributeRow[]
  hasSelection: boolean
}>()

const selectAll = computed({
  get: () => props.rows.length > 0 && props.rows.every((item) => item.checked),
  set: (value: boolean) => {
    props.rows.forEach((item) => {
      item.checked = value
    })
  }
})

const indeterminate = computed(() => {
  const checkedCount = props.rows.filter((item) => item.checked).length
  return checkedCount > 0 && checkedCount < props.rows.length
})
</script>

<template>
  <div v-if="loading" class="empty-params">
    <p>{{ $t('generate.loading') }}</p>
  </div>
  <div v-else-if="rows.length">
    <div class="attribute-toolbar">
      <NCheckbox v-model:checked="selectAll" :indeterminate="indeterminate">
        {{ $t('generate.select-all') }}
      </NCheckbox>
    </div>
    <div v-for="item in rows" :key="item.key || item.id" class="attribute-row">
      <div class="attribute-info">
        <NCheckbox v-model:checked="item.checked">
          <div class="attribute-label">
            <div v-if="item.data_name" class="attribute-name">
              {{ item.data_name }}
            </div>
            <div class="attribute-key">{{ item.key }}</div>
          </div>
        </NCheckbox>
      </div>
      <div class="attribute-input">
        <NInput
          v-model:value="item.inputValue"
          :placeholder="$t('generate.attribute-value-placeholder')"
          :disabled="!item.checked"
        />
      </div>
    </div>
    <div v-if="!hasSelection" class="attribute-helper">
      {{ $t('generate.attribute-helper-text') }}
    </div>
  </div>
  <div v-else class="empty-params">
    <p>{{ $t('generate.no-attributes-available') }}</p>
  </div>
</template>

<style lang="scss" scoped>
.empty-params {
  text-align: center;
  padding: 20px 16px;
  color: #999;

  p {
    margin: 0;
    font-size: 13px;
  }
}

.attribute-toolbar {
  margin-bottom: 12px;
}

.attribute-row {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid #f2f4f7;

  &:first-of-type {
    border-top: 1px solid #f2f4f7;
  }

  &:last-of-type {
    margin-bottom: 12px;
  }
}

.attribute-info {
  flex: 1;

  .attribute-label {
    display: flex;
    flex-direction: column;
    line-height: 1.3;
  }

  .attribute-name {
    font-weight: 500;
    color: #1f2937;
  }

  .attribute-key {
    font-size: 14px;
    color: #6b7280;
  }
}

.attribute-input {
  flex: 1.2;
}
</style>
