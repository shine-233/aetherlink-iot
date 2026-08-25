<!--
文件用途: 操作审计日志展开行里的请求/响应载荷展示块。
核心逻辑: 尝试把 message 解析为 JSON 后美化缩进展示；超长内容默认折叠，通过按钮在折叠/展开间切换。
关键注意事项: 载荷可能包含敏感信息，只做本地渲染，禁止外发；解析失败时按原始文本兜底展示。
重构建议: 若后续需要语法高亮或复制按钮，优先抽公共代码块组件，避免每个审计页面各写一份。
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { $t } from '@/locales'

const props = defineProps<{
  /** 块标题，如“请求内容” */
  label: string
  /** 原始载荷文本，可能为空或非 JSON */
  message: string | null
}>()

/** 超过该长度默认折叠，避免超长响应撑开表格 */
const FOLD_THRESHOLD = 400

const folded = ref(true)

/** JSON 美化失败时回退为原始文本 */
const prettyText = computed(() => {
  const raw = props.message ?? ''
  if (!raw) return ''
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
})

const hasContent = computed(() => prettyText.value.length > 0)
/** 长文本才允许折叠；短内容直接完整展示 */
const foldable = computed(() => prettyText.value.length > FOLD_THRESHOLD)

function toggleFold() {
  folded.value = !folded.value
}
</script>

<template>
  <section class="operation-log-message-block">
    <div class="operation-log-message-block__header">
      <span class="operation-log-message-block__label">{{ label }}</span>
      <button
        v-if="foldable"
        type="button"
        class="operation-log-message-block__toggle"
        @click="toggleFold"
      >
        {{ folded ? $t('custom.management.operationLog.detail.expand') : $t('custom.management.operationLog.detail.collapse') }}
      </button>
    </div>
    <pre v-if="hasContent" class="operation-log-message-block__body" :class="{ 'is-folded': folded && foldable }">{{ prettyText }}</pre>
    <span v-else class="operation-log-message-block__empty">{{ $t('custom.management.operationLog.detail.empty') }}</span>
  </section>
</template>

<style lang="scss" scoped>
.operation-log-message-block {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;

  &__header {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  &__label {
    font-weight: 600;
    font-size: 13px;
  }

  &__toggle {
    cursor: pointer;
    border: none;
    background: transparent;
    color: var(--primary-color, #18a058);
    font-size: 12px;
    padding: 0;
  }

  &__body {
    margin: 0;
    padding: 8px 12px;
    border-radius: 6px;
    background-color: rgba(128, 128, 128, 0.08);
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    word-break: break-all;
    max-height: 480px;
    overflow: auto;

    &.is-folded {
      max-height: 200px;
      overflow: hidden;
    }
  }

  &__empty {
    color: rgba(128, 128, 128, 0.6);
    font-size: 12px;
  }
}
</style>
