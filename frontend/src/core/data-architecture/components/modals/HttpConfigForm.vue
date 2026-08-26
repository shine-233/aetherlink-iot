<!--
  文件用途: HTTP 接口配置表单弹窗。
  核心逻辑: 按步骤组织基础配置、请求头、参数和脚本配置，并向父级提交完整 HTTP 配置。
  关键注意事项: 表单输出是持久化配置的一部分，Tab 锁定和默认值变化需要谨慎。
  重构建议: 将步骤编排、完整性校验和提交归一拆成组合函数。
-->
<script setup lang="ts">
/**
 * HttpConfigForm - HTTP接口配置表单（UI优化版）
 *
 * 🎯 优化：表单验证渐进式引导（Tab锁定图标、Hover提示、参数计数）
 */

import { ref, reactive, computed, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import type {
  HttpHeader,
  HttpParam,
  HttpPathParam,
  HttpConfig,
  HttpParameter,
  PathParameter
} from '@/core/data-architecture/types/http-config'
import { extractPathParamsFromUrl } from '@/core/data-architecture/types/http-config'
// 导入分步配置组件
import HttpConfigStep1 from '@/core/data-architecture/components/common/HttpConfigStep1.vue'
import HttpConfigStep2 from '@/core/data-architecture/components/common/HttpConfigStep2.vue'
import HttpConfigStep3 from '@/core/data-architecture/components/common/HttpConfigStep3.vue'
import HttpConfigStep4 from '@/core/data-architecture/components/common/HttpConfigStep4.vue'
// 导入图标
import { LockClosedOutline as LockIcon } from '@vicons/ionicons5'

// Props接口 - 支持v-model模式
interface Props {
  /** v-model绑定的HTTP配置 */
  modelValue?: Partial<HttpConfig>
  /** 🔥 新增：当前组件ID，用于属性绑定 */
  componentId?: string
}

// Emits接口
interface Emits {
  (e: 'update:modelValue', value: Props['modelValue']): void
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: () => ({
    url: '',
    method: 'GET',
    timeout: 10000,
    addressType: 'external', // 默认为外部地址
    selectedInternalAddress: '',
    headers: [],
    params: [],
    pathParams: [],
    body: '',
    preRequestScript: '',
    postResponseScript: ''
  })
})

const emit = defineEmits<Emits>()
const { t } = useI18n()
const message = useMessage()

/**
 * 当前Tab - 改用Tab切换替代步骤条
 * 'basic': 基础配置, 'headers': 请求头, 'params': 参数配置, 'scripts': 请求脚本
 */
const currentTab = ref<'basic' | 'headers' | 'params' | 'scripts'>('basic')

/**
 * 当前选择的内部接口信息 - 用于接口模板功能
 */
const currentApiInfo = ref(null)

/**
 * 数据转换帮助函数
 */
const convertHttpToEnhanced = (param: any) => ({
  key: param.key || '',
  value: param.value || '',
  enabled: param.enabled !== false,
  valueMode: param.valueMode || (param.isDynamic ? 'property' : 'manual'),
  selectedTemplate: param.selectedTemplate || (param.isDynamic ? 'property-binding' : 'manual'),
  variableName: param.variableName || '',
  description: param.description || '',
  dataType: param.dataType || 'string'
})

/**
 * 本地配置状态 - 包含地址类型状态
 */
const localConfig = reactive<HttpConfig>({
  url: '',
  method: 'GET',
  timeout: 10000,
  addressType: 'external',
  selectedInternalAddress: '',
  pathParameter: undefined,
  headers: [],
  params: [],
  pathParams: [],
  body: '',
  preRequestScript: ''
})

function initializeParameters(config?: HttpConfig): HttpParameter[] {
  const parameters: HttpParameter[] = []

  if (config?.parameters && Array.isArray(config.parameters)) {
    return [...config.parameters]
  }

  if (config?.headers) {
    config.headers.forEach((header) => {
      parameters.push({
        ...header,
        paramType: 'header'
      })
    })
  }

  if (config?.params) {
    config.params.forEach((param) => {
      parameters.push({
        ...param,
        paramType: 'query'
      })
    })
  }

  if (config?.pathParams) {
    config.pathParams.forEach((pathParam) => {
      parameters.push({
        key: pathParam.key,
        value: pathParam.value,
        enabled: pathParam.enabled,
        isDynamic: pathParam.isDynamic,
        dataType: pathParam.dataType,
        variableName: pathParam.variableName,
        description: pathParam.description,
        paramType: 'path'
      })
    })
  }

  return parameters
}

/**
 * URL变化时自动检测路径参数
 */
const onUrlChange = () => {
  const detectedParams = extractPathParamsFromUrl(localConfig.url)

  if (detectedParams.length > 0) {
    const existingKeys = (localConfig.pathParams || []).map((p) => p.key)
    const newParams = detectedParams.filter((p) => !existingKeys.includes(p.key))

    if (newParams.length > 0) {
      localConfig.pathParams = localConfig.pathParams || []
      localConfig.pathParams.push(...newParams)
    }
  }

  updateConfig()
}

/**
 * 处理接口信息更新（从Step1传递过来）
 */
const onApiInfoUpdate = (apiInfo: any) => {
  currentApiInfo.value = apiInfo
}

/**
 * Tab切换函数
 */
const switchToTab = (tab: 'basic' | 'headers' | 'params' | 'scripts') => {
  currentTab.value = tab
}

/**
 * 🎯 优化：Tab验证 - 基础配置是否完成
 */
const isBasicConfigValid = computed(() => {
  return localConfig.url && localConfig.method
})

/**
 * 🎯 优化：计算各类参数的数量（用于Tab计数显示）
 */
const headersCount = computed(() => {
  return localConfig.headers?.filter((h) => h.enabled !== false).length || 0
})

const paramsCount = computed(() => {
  return localConfig.params?.filter((p) => p.enabled !== false).length || 0
})

const pathParamsCount = computed(() => {
  return localConfig.pathParams?.filter((p) => p.enabled !== false).length || 0
})

/**
 * 简化的配置更新函数 - 立即发射事件，不进行复杂转换
 */
const updateConfig = () => {
  const config = { ...localConfig }

  if (config.headers) {
    config.headers = config.headers.map((header) => ({
      ...header,
      isDynamic: header.valueMode === 'property',
      paramType: 'header' as const
    }))
  }

  if (config.params) {
    config.params = config.params.map((param) => ({
      ...param,
      isDynamic: param.valueMode === 'property',
      paramType: 'query' as const
    }))
  }

  if (config.pathParams && config.pathParams.length > 0) {
    config.pathParams = config.pathParams.map((param) => ({
      ...param,
      isDynamic: param.valueMode === 'property',
      paramType: 'path' as const
    }))

    const firstParam = config.pathParams[0]
    if (process.env.NODE_ENV === 'development') {
      /* intentionally empty */
    }

    config.pathParameter = {
      value: firstParam.value,
      isDynamic: firstParam.valueMode === 'component' || firstParam.selectedTemplate === 'component-property-binding',
      dataType: firstParam.dataType,
      variableName: firstParam.variableName || '',
      description: firstParam.description || '',
      selectedTemplate: firstParam.selectedTemplate,
      defaultValue: firstParam.defaultValue,
      key: firstParam.key,
      enabled: firstParam.enabled
    }

    if (process.env.NODE_ENV === 'development') {
      /* intentionally empty */
    }
  } else {
    config.pathParameter = undefined
    config.pathParams = []
  }

  emit('update:modelValue', config)
}

/**
 * 防止循环更新的同步标识
 */
let isUpdatingFromProps = false
let isUpdatingToParent = false

/**
 * 安全的配置更新 - 防止循环更新
 */
const safeUpdateConfig = () => {
  if (isUpdatingFromProps || isUpdatingToParent) {
    return
  }

  isUpdatingToParent = true

  try {
    updateConfig()
  } finally {
    nextTick(() => {
      isUpdatingToParent = false
    })
  }
}

/**
 * 监听本地配置变化 - 使用防护机制
 */
watch(
  () => localConfig,
  () => {
    if (isUpdatingFromProps) {
      nextTick(() => {
        isUpdatingFromProps = false
        safeUpdateConfig()
      })
    } else {
      safeUpdateConfig()
    }
  },
  {
    deep: true,
    flush: 'post'
  }
)

/**
 * 监听props变化同步到本地状态 - 改进防护机制
 */
const syncPropsToLocal = (newValue: any) => {
  if (!newValue) return

  if (isUpdatingToParent && !isUpdatingFromProps) {
    return
  }

  isUpdatingFromProps = true

  try {
    if (newValue.url !== undefined) localConfig.url = newValue.url
    if (newValue.method !== undefined) localConfig.method = newValue.method
    if (newValue.timeout !== undefined) localConfig.timeout = newValue.timeout

    if (newValue.addressType !== undefined) localConfig.addressType = newValue.addressType
    if (newValue.selectedInternalAddress !== undefined) {
      localConfig.selectedInternalAddress = newValue.selectedInternalAddress
    }
    if (newValue.enableParams !== undefined) localConfig.enableParams = newValue.enableParams
    if (newValue.pathParameter !== undefined) localConfig.pathParameter = newValue.pathParameter
    if (newValue.body !== undefined) localConfig.body = newValue.body
    if (newValue.preRequestScript !== undefined) {
      localConfig.preRequestScript = newValue.preRequestScript
    }

    localConfig.headers = newValue.headers ? newValue.headers.map(convertHttpToEnhanced) : []
    localConfig.params = newValue.params ? newValue.params.map(convertHttpToEnhanced) : []

    if (newValue.pathParams) {
      localConfig.pathParams = newValue.pathParams.map(convertHttpToEnhanced)
    } else if (newValue.pathParameter) {
      localConfig.pathParams = [
        convertHttpToEnhanced({
          key: 'pathParam',
          value: newValue.pathParameter.value,
          enabled: true,
          isDynamic: newValue.pathParameter.isDynamic,
          variableName: newValue.pathParameter.variableName,
          description: newValue.pathParameter.description,
          dataType: newValue.pathParameter.dataType
        })
      ]
    } else {
      localConfig.pathParams = []
    }
  } finally {
    nextTick(() => {
      isUpdatingFromProps = false
    })
  }
}

watch(() => props.modelValue, syncPropsToLocal, { deep: true, immediate: true })
</script>

<template>
  <div class="http-config-form">
    <!-- 🎯 优化：Tab导航 - 带锁定提示和参数计数 -->
    <div class="tabs-section">
      <n-tabs v-model:value="currentTab" type="line" size="medium" :animated="true" @update:value="switchToTab">
        <!-- 基础配置Tab -->
        <n-tab-pane name="basic">
          <template #tab>
            <n-space :size="4" align="center">
              <span>{{ isBasicConfigValid ? '●' : '○' }}</span>
              <span>基础配置</span>
              <n-tag v-if="isBasicConfigValid" type="success" size="small" :bordered="false">✓</n-tag>
            </n-space>
          </template>
          <HttpConfigStep1
            :model-value="localConfig"
            :component-id="componentId"
            @update:model-value="
              (value) => {
                Object.assign(localConfig, value)
              }
            "
            @url-change="onUrlChange"
            @api-info-update="onApiInfoUpdate"
          />
        </n-tab-pane>

        <!-- 请求头Tab -->
        <n-tab-pane name="headers" :disabled="!isBasicConfigValid">
          <template #tab>
            <n-tooltip :disabled="isBasicConfigValid">
              <template #trigger>
                <n-space :size="4" align="center">
                  <n-icon v-if="!isBasicConfigValid" size="14"><lock-icon /></n-icon>
                  <span>请求头</span>
                  <n-tag v-if="headersCount > 0" type="info" size="small" :bordered="false">{{ headersCount }}</n-tag>
                </n-space>
              </template>
              请先完成基础配置（URL和请求方法）
            </n-tooltip>
          </template>
          <HttpConfigStep2
            :model-value="localConfig"
            :component-id="componentId"
            :current-api-info="currentApiInfo"
            @update:model-value="
              (value) => {
                Object.assign(localConfig, value)
              }
            "
          />
        </n-tab-pane>

        <!-- 参数配置Tab -->
        <n-tab-pane name="params" :disabled="!isBasicConfigValid">
          <template #tab>
            <n-tooltip :disabled="isBasicConfigValid">
              <template #trigger>
                <n-space :size="4" align="center">
                  <n-icon v-if="!isBasicConfigValid" size="14"><lock-icon /></n-icon>
                  <span>参数配置</span>
                  <n-tag v-if="paramsCount > 0" type="info" size="small" :bordered="false">{{ paramsCount }}</n-tag>
                </n-space>
              </template>
              请先完成基础配置（URL和请求方法）
            </n-tooltip>
          </template>
          <HttpConfigStep3
            :model-value="localConfig"
            :component-id="componentId"
            :current-api-info="currentApiInfo"
            @update:model-value="
              (value) => {
                if (isUpdatingFromProps) {
                  isUpdatingFromProps = false
                }

                localConfig.params = value.params || []

                nextTick(() => {})
              }
            "
          />
        </n-tab-pane>

        <!-- 请求脚本Tab -->
        <n-tab-pane name="scripts" :disabled="!isBasicConfigValid">
          <template #tab>
            <n-tooltip :disabled="isBasicConfigValid">
              <template #trigger>
                <n-space :size="4" align="center">
                  <n-icon v-if="!isBasicConfigValid" size="14"><lock-icon /></n-icon>
                  <span>请求脚本</span>
                  <n-tag v-if="localConfig.preRequestScript" type="warning" size="small" :bordered="false">
                    已配置
                  </n-tag>
                </n-space>
              </template>
              请先完成基础配置（URL和请求方法）
            </n-tooltip>
          </template>
          <HttpConfigStep4
            :model-value="localConfig"
            :component-id="componentId"
            @update:model-value="
              (value) => {
                Object.assign(localConfig, value)
              }
            "
          />
        </n-tab-pane>
      </n-tabs>
    </div>

    <!-- 🎯 优化：配置状态提示 -->
    <div v-if="!isBasicConfigValid" class="config-tip">
      <n-alert type="info" style="margin-top: 16px">
        <template #header>
          <n-space align="center">
            <span>📝 配置进度</span>
          </n-space>
        </template>
        <n-space vertical size="small">
          <n-text depth="3">请先完成基础配置，然后可以配置其他选项</n-text>
          <n-progress
            type="line"
            :percentage="localConfig.url && localConfig.method ? 100 : localConfig.url || localConfig.method ? 50 : 0"
            :show-indicator="true"
            status="info"
          />
          <n-space size="small">
            <n-tag :type="localConfig.url ? 'success' : 'default'" size="small">
              {{ localConfig.url ? '✓' : '○' }} URL
            </n-tag>
            <n-tag :type="localConfig.method ? 'success' : 'default'" size="small">
              {{ localConfig.method ? '✓' : '○' }} 请求方法
            </n-tag>
          </n-space>
        </n-space>
      </n-alert>
    </div>
  </div>
</template>

<style scoped>
.http-config-form {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.tabs-section {
  flex: 1;
  min-height: 500px;
  overflow: visible; /* 🔥 修复：确保下拉菜单不被外层容器裁剪 */
  position: relative;
}

/* Tab内容样式调整 */
.tabs-section :deep(.n-tab-pane) {
  min-height: 450px;
  max-height: 600px;
  overflow-y: visible; /* 🔥 修复：改为visible避免下拉菜单被裁剪 */
  padding: 16px 0;
  position: relative;
  z-index: 1;
}

/* Tab标签样式 */
.tabs-section :deep(.n-tabs-nav) {
  margin-bottom: 16px;
}

.config-tip {
  padding: 12px;
}

/* Tab标签增强样式 */
.tabs-section :deep(.n-tabs-tab) {
  padding: 8px 16px;
}

.tabs-section :deep(.n-tabs-tab--disabled) {
  opacity: 0.5;
  cursor: not-allowed;
}

/* 锁定图标样式 */
.tabs-section :deep(.n-tabs-tab--disabled .n-icon) {
  color: var(--warning-color);
}
</style>
