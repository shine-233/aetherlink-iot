<!--
  文件用途: HTTP 配置第二步请求头组件。
  核心逻辑: 使用动态参数编辑器维护请求头，并展示接口模板推荐。
  关键注意事项: 请求头变量和值会进入执行器请求构造，需保持动态参数结构兼容。
  重构建议: 将模板推荐、参数编辑和请求头校验拆分。
-->
<script setup lang="ts">
/**
 * HttpConfigStep2 - HTTP请求头配置步骤（UI优化版）
 * 使用通用的动态参数编辑器配置请求头
 *
 * 🎯 优化3：接口模板智能推荐
 * - 检测currentApiInfo是否有预制参数
 * - 显示智能推荐卡片
 * - 应用模板后高亮提示
 */

import { ref, computed, watch } from 'vue'
import type { HttpConfig, HttpHeader } from '@/core/data-architecture/types/http-config'
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import DynamicParameterEditor from '@/core/data-architecture/components/common/DynamicParameterEditor.vue'

// 历史持久化形态 HttpHeader 与编辑器形态 EnhancedParameter 是同构异名边界，
// 在组件入参/出参两处集中桥接，避免散落断言。
const toEditorHeaders = (list: HttpHeader[] | undefined): EnhancedParameter[] =>
  (list || []) as unknown as EnhancedParameter[]
const fromEditorHeaders = (list: EnhancedParameter[]): HttpHeader[] => list as unknown as HttpHeader[]
// 导入图标
import { Sparkles as SparkleIcon } from '@vicons/ionicons5'

interface Props {
  /** HTTP配置数据 */
  modelValue: Partial<HttpConfig>
  /** 当前选择的内部接口信息 */
  currentApiInfo?: any
  /** 🔥 新增：当前组件ID，用于属性绑定 */
  componentId?: string
}

interface Emits {
  (e: 'update:modelValue', value: Props['modelValue']): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

/**
 * 🎯 优化3：智能推荐卡片显示状态
 */
const showTemplateRecommend = ref(false)
const hasAppliedTemplate = ref(false)

/**
 * 🎯 优化3：检测是否有可用的请求头模板
 */
const hasHeaderTemplate = computed(() => {
  if (!props.currentApiInfo || !props.currentApiInfo.commonParams) return false

  // 检查是否有请求头类型的参数
  const headerParams = props.currentApiInfo.commonParams.filter(
    (param: any) => param.type === 'header' || param.paramType === 'header'
  )

  return headerParams.length > 0
})

/**
 * 🎯 优化3：获取请求头模板参数
 */
const headerTemplateParams = computed(() => {
  if (!props.currentApiInfo || !props.currentApiInfo.commonParams) return []

  return props.currentApiInfo.commonParams.filter(
    (param: any) => param.type === 'header' || param.paramType === 'header'
  )
})

/**
 * 🎯 优化3：监听currentApiInfo变化，自动显示推荐卡片
 */
watch(
  () => props.currentApiInfo,
  newValue => {
    if (newValue && hasHeaderTemplate.value && !hasAppliedTemplate.value) {
      showTemplateRecommend.value = true
    }
  },
  { immediate: true }
)

/**
 * 🎯 优化3：应用接口模板
 */
const applyTemplate = () => {
  if (!headerTemplateParams.value || headerTemplateParams.value.length === 0) return

  // 生成模板参数
  const templateHeaders: EnhancedParameter[] = headerTemplateParams.value.map((param: any) => ({
    key: param.name,
    value: param.example || param.defaultValue || '',
    enabled: true,
    isDynamic: false,
    valueMode: 'manual',
    selectedTemplate: 'manual',
    variableName: '',
    description: param.description || `${param.name}请求头`,
    dataType: 'string',
    _id: `header_template_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`
  }))

  // 合并到现有请求头（避免重复）
  const existingKeys = new Set((props.modelValue.headers || []).map(h => h.key))
  const newHeaders = templateHeaders.filter(h => !existingKeys.has(h.key))

  if (newHeaders.length > 0) {
    const updatedHeaders = [...(props.modelValue.headers || []), ...newHeaders]
    updateHeaders(updatedHeaders)
    // 标记已应用模板
    hasAppliedTemplate.value = true
    showTemplateRecommend.value = false
  }
}

/**
 * 🎯 优化3：关闭推荐卡片
 */
const dismissRecommend = () => {
  showTemplateRecommend.value = false
}

/**
 * 更新请求头配置
 */
const updateHeaders = (headers: Array<EnhancedParameter | HttpHeader>) => {
  const updatedValue = {
    ...props.modelValue,
    headers: fromEditorHeaders(headers as EnhancedParameter[])
  }

  emit('update:modelValue', updatedValue)
}
</script>

<template>
  <div class="http-config-step2">
    <!-- 🎯 优化3：接口模板智能推荐卡片 -->
    <n-alert v-if="showTemplateRecommend" type="success" closable style="margin-bottom: 16px" @close="dismissRecommend">
      <template #header>
        <n-space align="center">
          <n-icon size="18"><sparkle-icon /></n-icon>
          <span>检测到内部接口模板可用</span>
        </n-space>
      </template>

      <n-space vertical size="small">
        <n-text depth="3">
          接口 "
          <n-text type="success" strong>{{ currentApiInfo?.label }}</n-text>
          " 包含
          <n-text type="success" strong>{{ headerTemplateParams.length }}</n-text>
          个预制请求头参数
        </n-text>

        <n-space size="small">
          <n-tag
            v-for="param in headerTemplateParams.slice(0, 3)"
            :key="param.name"
            type="success"
            size="small"
            :bordered="false"
          >
            {{ param.name }}
          </n-tag>
          <n-text v-if="headerTemplateParams.length > 3" depth="3" style="font-size: 12px">
            +{{ headerTemplateParams.length - 3 }} 个
          </n-text>
        </n-space>

        <n-space style="margin-top: 8px">
          <n-button type="success" size="small" @click="applyTemplate">
            <template #icon>
              <n-icon><sparkle-icon /></n-icon>
            </template>
            应用模板
          </n-button>
          <n-button size="small" @click="dismissRecommend">稍后手动配置</n-button>
        </n-space>
      </n-space>
    </n-alert>

    <!-- 请求头配置 -->
    <DynamicParameterEditor
      :model-value="toEditorHeaders(modelValue.headers)"
      parameter-type="header"
      title="请求头配置"
      add-button-text="添加请求头"
      key-placeholder="头部名称（如：Content-Type）"
      value-placeholder="头部值（如：application/json）"
      :current-api-info="currentApiInfo"
      :current-component-id="componentId"
      @update:model-value="updateHeaders"
    />
  </div>
</template>

<style scoped>
.http-config-step2 {
  width: 100%;
  padding: 12px;
}
</style>
