<!--
  文件用途: HTTP 配置第四步请求前脚本组件。
  核心逻辑: 编辑请求发送前用于修改 URL、请求头和参数的脚本。
  关键注意事项: 脚本能力依赖 script-engine，脚本片段和上下文字段需要保持安全且可执行。
  重构建议: 将脚本上下文说明、默认模板和校验提示拆分。
-->
<script setup lang="ts">
/**
 * HttpConfigStep4 - HTTP请求前脚本配置步骤
 * 用于在发送请求前动态修改URL、请求头和参数
 */

import { defineAsyncComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import type { HttpConfig } from '@/core/data-architecture/types/http-config'

const SimpleScriptEditor = defineAsyncComponent(
  () => import('@/core/script-engine/components/SimpleScriptEditor.vue')
)

interface Props {
  /** HTTP配置数据 */
  modelValue: Partial<HttpConfig>
  /** 当前组件ID，用于属性绑定（预留） */
  componentId?: string
}

interface Emits {
  (e: 'update:modelValue', value: Props['modelValue']): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const { t } = useI18n()

/**
 * 更新请求前脚本
 */
const updatePreRequestScript = (script: string) => {
  emit('update:modelValue', {
    ...props.modelValue,
    preRequestScript: script
  })
}
</script>

<template>
  <div class="http-config-step4">
    <div class="script-editor-section">
      <SimpleScriptEditor
        :model-value="modelValue.preRequestScript || ''"
        template-category="http-pre-request"
        placeholder="请求前处理脚本"
        height="300px"
        @update:model-value="updatePreRequestScript"
      />
    </div>
  </div>
</template>

<style scoped>
.http-config-step4 {
  width: 100%;
  padding: 12px;
}

.script-editor-section {
  min-height: 280px;
}
</style>
