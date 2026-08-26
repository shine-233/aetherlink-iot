<!--
  文件用途: HTTP 配置第一步基础配置组件。
  核心逻辑: 编辑 URL、方法、超时、请求体和基础请求设置。
  关键注意事项: 基础字段是后续请求头、参数和脚本步骤的前置条件。
  重构建议: 把 URL 校验、请求体编辑和模板应用拆成独立小组件。
-->
<script setup lang="ts">
/**
 * HttpConfigStep1 - HTTP基础配置步骤
 * 包含URL、请求方法、超时时间、请求体配置
 */

import { computed, ref, watch, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import type { HttpConfig } from '@/core/data-architecture/types/http-config'
import DynamicParameterEditor from '@/core/data-architecture/components/common/DynamicParameterEditor.vue'
import { internalAddressOptions, getApiByValue } from '@/core/data-architecture/data/internal-address-data'
import type { InternalApiItem } from '@/core/data-architecture/types/internal-api'
import type { SelectMixedOption } from 'naive-ui/es/select/src/interface'
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'

interface Props {
  /** HTTP配置数据 */
  modelValue: Partial<HttpConfig>
  /** 🔥 新增：当前组件ID，用于属性绑定 */
  componentId?: string
}

interface Emits {
  (e: 'update:modelValue', value: Props['modelValue']): void
  (e: 'urlChange'): void
  (e: 'apiInfoUpdate', apiInfo: any): void // 新增：接口信息更新事件
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const { t } = useI18n()

// 模板里直接写 navigator 会被 vue-tsc 当作组件实例属性解析；显式暴露浏览器对象。
const navigatorRef = navigator

// 🔥 新增：防止循环更新的标记
const isUpdatingFromChild = ref(false)

/**
 * 地址类型选择：直接从modelValue获取和设置
 */
const addressType = computed({
  get: () => (props.modelValue.addressType !== undefined ? props.modelValue.addressType : 'external'),
  set: (value: 'internal' | 'external') => {
    updateConfig('addressType', value)
  }
})

/**
 * 获取选中的API信息
 */
const selectedApiInfo = computed(() => {
  if (!selectedInternalAddress.value) return null
  return getApiByValue(selectedInternalAddress.value)
})

/**
 * 选中的内部地址：直接从modelValue获取和设置
 */
const selectedInternalAddress = computed({
  get: () => (props.modelValue.selectedInternalAddress !== undefined ? props.modelValue.selectedInternalAddress : ''),
  set: (value: string) => {
    updateConfig('selectedInternalAddress', value)
  }
})

/**
 * 是否启用传参
 */
const enableParams = ref(false)

/**
 * 传参配置
 */
const urlParams = ref<EnhancedParameter[]>([])

const isDynamicBindingParameter = (param: EnhancedParameter) =>
  param.valueMode === 'component' || param.selectedTemplate === 'component-property-binding'

const createEditorParamId = () => `param_${Date.now()}_${Math.random()}`

// 兼容三种历史路径参数形态（EnhancedParameter / HttpPathParam / PathParameter）
// 的最小结构输入；toEditorPathParam 只消费以下字段（valueMode 用宽 string，
// 因为历史形态未收窄到联合类型）。
type EditorPathParamInput = {
  key?: string
  value?: string | number | boolean
  enabled?: boolean
  valueMode?: string
  selectedTemplate?: string
  isDynamic?: boolean
  variableName?: string
  description?: string
  dataType?: 'string' | 'number' | 'boolean' | 'json'
  defaultValue?: string | number | boolean
}

const toEditorPathParam = (
  param: EditorPathParamInput,
  fallbackKey = 'pathParam'
): EnhancedParameter => ({
  key: param.key || fallbackKey,
  value: param.value || '',
  enabled: param.enabled !== false,
  valueMode: (param.valueMode || (param.isDynamic ? 'property' : 'manual')) as EnhancedParameter['valueMode'],
  selectedTemplate: param.selectedTemplate || (param.isDynamic ? 'property-binding' : 'manual'),
  variableName: param.variableName || '',
  description: param.description || '',
  dataType: param.dataType || 'string',
  defaultValue: param.defaultValue,
  _id: createEditorParamId()
})

const syncUrlParamFormState = (params: EnhancedParameter[]) => {
  urlParams.value = params
  enableParams.value = params.length > 0
}

const restoreSavedPathParams = () => {
  if (!props.modelValue.pathParams || props.modelValue.pathParams.length === 0) {
    return null
  }

  return props.modelValue.pathParams.map((param) => toEditorPathParam(param))
}

const restoreCompatibilityPathParameter = () => {
  if (!props.modelValue.pathParameter) {
    return null
  }

  return [toEditorPathParam(props.modelValue.pathParameter)]
}

const getRestoredUrlParams = () => restoreSavedPathParams() || restoreCompatibilityPathParameter()

const shouldRestoreInternalPathParams = () => {
  if (addressType.value !== 'internal' || !selectedInternalAddress.value) {
    return false
  }

  const apiInfo = getApiByValue(selectedInternalAddress.value)
  return Boolean(apiInfo?.hasPathParams)
}

const emitApiInfoForAddress = (addressValue?: string) => {
  if (!addressValue) {
    return
  }

  const apiInfo = getApiByValue(addressValue)
  if (apiInfo) {
    emit('apiInfoUpdate', apiInfo)
  }
}

const scheduleUrlParamsInitialization = (options?: { emitApiInfo?: boolean; modelValue?: Partial<HttpConfig> }) => {
  const shouldEmitApiInfo = options?.emitApiInfo ?? false
  const modelValue = options?.modelValue ?? props.modelValue

  nextTick(() => {
    if (shouldEmitApiInfo && modelValue.addressType === 'internal' && modelValue.selectedInternalAddress) {
      emitApiInfoForAddress(modelValue.selectedInternalAddress)
    }

    initializeUrlParamsState()
  })
}

const buildCompatibilityPathParameterMirror = (params: EnhancedParameter[]) => {
  if (params.length === 0) {
    return undefined
  }

  const firstParam = params[0]
  return {
    value: firstParam.value,
    isDynamic: isDynamicBindingParameter(firstParam),
    variableName: firstParam.variableName || '',
    description: firstParam.description || '',
    dataType: firstParam.dataType || 'string',
    defaultValue: firstParam.defaultValue,
    selectedTemplate: firstParam.selectedTemplate,
    key: firstParam.key,
    enabled: firstParam.enabled
  }
}

const buildUrlParamConfigUpdates = (params: EnhancedParameter[]): Partial<HttpConfig> => {
  const batchUpdates: Partial<HttpConfig> = {
    pathParams: params
  }

  const compatibilityPathParameter = buildCompatibilityPathParameterMirror(params)
  if (compatibilityPathParameter) {
    batchUpdates.pathParameter = compatibilityPathParameter
  }

  const apiInfo = selectedApiInfo.value
  if (apiInfo) {
    batchUpdates.url = apiInfo.url
  }

  return batchUpdates
}

const finishChildSyncUpdate = () => {
  nextTick(() => {
    isUpdatingFromChild.value = false
  })
}

/**
 * HTTP方法选项
 */
const httpMethods = [
  { label: 'GET', value: 'GET' },
  { label: 'POST', value: 'POST' },
  { label: 'PUT', value: 'PUT' },
  { label: 'DELETE', value: 'DELETE' },
  { label: 'PATCH', value: 'PATCH' }
]

/**
 * 是否显示请求体配置
 */
const showBody = computed(() => {
  return ['POST', 'PUT', 'PATCH'].includes(props.modelValue.method || '')
})

/**
 * 更新配置数据
 */
const updateConfig = (field: keyof HttpConfig, value: any) => {
  const newConfig = {
    ...props.modelValue,
    [field]: value
  }

  // 🔥 调试：监听所有配置更新
  if (process.env.NODE_ENV === 'development') {
    /* intentionally empty */
  }
  if (field === 'pathParameter') {
    /* intentionally empty */
  }

  emit('update:modelValue', newConfig)
}

/**
 * 地址类型变化处理
 */
const onAddressTypeChange = (type: 'internal' | 'external') => {
  addressType.value = type

  if (type === 'external') {
    // 切换到外部地址时，清空内部地址相关配置
    selectedInternalAddress.value = ''
    enableParams.value = false
    urlParams.value = []
  } else {
    // 切换到内部地址时，清空URL和所有相关状态
    selectedInternalAddress.value = ''
    enableParams.value = false
    urlParams.value = []
    updateConfig('url', '')
  }
}

/**
 * 内部地址选择处理
 */
const onInternalAddressSelect = (value: string, option: any) => {
  selectedInternalAddress.value = value

  // 获取API详情信息
  const apiInfo = getApiByValue(value)
  if (apiInfo) {
    // 同时设置请求方法
    updateConfig('method', apiInfo.method)

    // 🔥 关键修复：保存选择的内部地址到父组件
    updateConfig('selectedInternalAddress', value)

    // 立即设置初始URL（无参数替换的版本）
    updateConfig('url', apiInfo.url)

    // 🔥 发射接口信息更新事件，让父组件知道当前选择的接口
    emit('apiInfoUpdate', apiInfo)

    // 🔥 修复：选择内部地址时不自动填充参数，只记录是否有参数
    if (apiInfo.hasPathParams && apiInfo.pathParamNames) {
      // 只清空现有参数，不自动生成新的
      urlParams.value = []
      enableParams.value = false
      updateConfig('pathParams', [])
      updateConfig('enableParams', false)
    } else {
      // 没有路径参数时，清空参数配置
      urlParams.value = []
      enableParams.value = false
      updateConfig('pathParams', [])
      updateConfig('enableParams', false)
    }
  } else {
    // 如果没有找到API信息，直接使用选择的值
    updateConfig('url', value)
  }
}

/**
 * 传参启用状态变化
 */
const onEnableParamsChange = (enabled: boolean) => {
  enableParams.value = enabled

  // 🔥 关键修复：同步启用状态到父组件
  updateConfig('enableParams', enabled)

  if (!enabled) {
    // 禁用传参时，清空参数配置
    urlParams.value = []
    updateConfig('pathParams', [])
    updateConfig('pathParameter', undefined)
  }
  if (!enabled) {
    urlParams.value = []
    // 禁用参数时，恢复到原始URL（不进行参数替换）
    const apiInfo = selectedApiInfo.value
    if (apiInfo) {
      updateConfig('url', apiInfo.url)
    }
  }
}

/**
 * 🔥 修复：传参配置更新 - 批量更新避免频繁重渲染
 */
const onUrlParamsUpdate = (params: EnhancedParameter[]) => {
  // 🔥 设置标记，避免watch监听器再次触发初始化
  isUpdatingFromChild.value = true

  urlParams.value = params
  const batchUpdates = buildUrlParamConfigUpdates(params)

  // 🔥 一次性批量更新，避免多次emit
  const newConfig = {
    ...props.modelValue,
    ...batchUpdates
  }

  if (process.env.NODE_ENV === 'development') {
    /* intentionally empty */
  }
  emit('update:modelValue', newConfig)

  // 🔥 重置标记，延迟执行避免立即触发watch
  finishChildSyncUpdate()
}

/**
 * URL变化时触发事件
 */
const onUrlChange = (value: string) => {
  updateConfig('url', value)
  emit('urlChange')
}

/**
 * 格式化地址显示文本
 */
const formatAddressDisplayText = (apiInfo: InternalApiItem) => {
  return `${apiInfo.label} (${apiInfo.method} ${apiInfo.url})`
}

/**
 * 当前地址的显示文本（包含路径参数替换）
 */
const currentAddressDisplay = computed(() => {
  if (addressType.value === 'external') {
    return props.modelValue.url || ''
  }

  const apiInfo = selectedApiInfo.value
  if (apiInfo) {
    let url = apiInfo.url

    // 如果启用了参数配置，用实际参数值替换URL中的占位符 - 正确解析属性绑定和默认值
    if (enableParams.value && urlParams.value.length > 0) {
      urlParams.value.forEach((param) => {
        if (param.enabled && param.key) {
          let resolvedValue = param.value

          // 如果是属性绑定，显示默认值用于预览（实际请求时会解析属性值）
          if (param.selectedTemplate === 'component-property-binding' && typeof param.value === 'string') {
            // URL预览时：如果是属性绑定，优先显示默认值，否则显示绑定路径
            resolvedValue = param.defaultValue || `[${param.value}]`
          }

          // 检查值是否为"空"
          const isEmpty =
            resolvedValue === null ||
            resolvedValue === undefined ||
            resolvedValue === '' ||
            (typeof resolvedValue === 'string' && resolvedValue.trim() === '')

          if (!isEmpty) {
            url = url.replace(`{${param.key}}`, resolvedValue)
          } else if (param.defaultValue) {
            // 使用默认值
            url = url.replace(`{${param.key}}`, param.defaultValue)
          }
        }
      })
    }

    return `${apiInfo.label} (${apiInfo.method} ${url})`
  }

  return props.modelValue.url || ''
})

/**
 * 获取处理过路径参数的最终URL
 */
const getFinalUrl = computed(() => {
  if (addressType.value === 'external') {
    return props.modelValue.url || ''
  }

  const apiInfo = selectedApiInfo.value
  if (apiInfo) {
    let url = apiInfo.url

    // 替换路径参数 - 正确解析属性绑定和默认值
    if (enableParams.value && urlParams.value.length > 0) {
      urlParams.value.forEach((param) => {
        if (param.enabled && param.key) {
          let resolvedValue = param.value

          // 如果是属性绑定，显示默认值用于预览（实际请求时会解析属性值）
          if (param.selectedTemplate === 'component-property-binding' && typeof param.value === 'string') {
            // URL预览时：如果是属性绑定，优先显示默认值，否则显示绑定路径
            resolvedValue = param.defaultValue || `[${param.value}]`
          }

          // 检查值是否为"空"
          const isEmpty =
            resolvedValue === null ||
            resolvedValue === undefined ||
            resolvedValue === '' ||
            (typeof resolvedValue === 'string' && resolvedValue.trim() === '')

          if (!isEmpty) {
            url = url.replace(`{${param.key}}`, resolvedValue)
          } else if (param.defaultValue) {
            // 使用默认值
            url = url.replace(`{${param.key}}`, param.defaultValue)
          }
        }
      })
    }

    return url
  }

  return props.modelValue.url || ''
})

/**
 * 初始化URL参数状态 - 从props中恢复配置
 */
const initializeUrlParamsState = () => {
  if (!shouldRestoreInternalPathParams()) {
    return
  }

  const restoredParams = getRestoredUrlParams()
  if (restoredParams) {
    syncUrlParamFormState(restoredParams)
  }
}

/**
 * 🔥 修复：监听 props 变化，同步URL参数状态 - 避免循环更新
 */
watch(
  () => [
    props.modelValue.addressType || 'external',
    props.modelValue.selectedInternalAddress || '',
    props.modelValue.pathParams || [],
    props.modelValue.pathParameter || null,
    props.modelValue.enableParams || false
  ],
  () => {
    // 🔥 如果正在从子组件更新，跳过此次同步，避免循环
    if (isUpdatingFromChild.value) {
      if (process.env.NODE_ENV === 'development') {
        /* intentionally empty */
      }
      return
    }

    scheduleUrlParamsInitialization()
  },
  { deep: true, immediate: true }
)

/**
 * 🔥 修复：监听关键字段变化，强制重新初始化 - 避免循环更新
 */
watch(
  () => props.modelValue,
  (newValue) => {
    // 🔥 如果正在从子组件更新，跳过此次同步
    if (isUpdatingFromChild.value) {
      return
    }

    // 当modelValue完全变化时（比如从编辑数据加载），重新初始化
    if (newValue && (newValue.addressType === 'internal' || newValue.selectedInternalAddress)) {
      scheduleUrlParamsInitialization({
        emitApiInfo: true,
        modelValue: newValue
      })
    }
  },
  { deep: true }
)

/**
 * 组件挂载时初始化状态
 */
onMounted(() => {
  initializeUrlParamsState()
})
</script>

<template>
  <div class="http-config-step1">
    <n-form size="small" :show-feedback="false">
      <!-- 地址类型选择 -->
      <n-form-item label="地址类型" required>
        <n-radio-group :value="addressType" @update:value="onAddressTypeChange">
          <n-radio value="external">外部地址</n-radio>
          <n-radio value="internal">内部地址</n-radio>
        </n-radio-group>
      </n-form-item>

      <!-- 外部地址输入 -->
      <n-form-item v-if="addressType === 'external'" label="请求URL" required>
        <n-input :value="modelValue.url" placeholder="/api/v1/telemetry/datas/statistic" @update:value="onUrlChange" />
      </n-form-item>

      <!-- 内部地址选择 -->
      <n-form-item v-if="addressType === 'internal'" label="选择内部接口" required>
        <n-select
          :value="selectedInternalAddress"
          :options="internalAddressOptions as unknown as SelectMixedOption[]"
          placeholder="请选择内部接口"
          @update:value="onInternalAddressSelect"
        />
      </n-form-item>

      <!-- 地址显示 -->
      <n-form-item v-if="modelValue.url" label="当前地址">
        <n-input :value="currentAddressDisplay" readonly placeholder="将显示选中的地址">
          <template #prefix>
            <span class="address-type-indicator">
              {{ addressType === 'internal' ? '💻' : '🌐' }}
            </span>
          </template>
          <template #suffix>
            <n-button text size="small" @click="() => navigatorRef.clipboard?.writeText(getFinalUrl)">复制</n-button>
          </template>
        </n-input>
      </n-form-item>

      <!-- 是否启用传参 -->
      <n-form-item v-if="addressType === 'internal' && selectedApiInfo?.hasPathParams" label="URL传参">
        <n-space align="center">
          <n-switch :value="enableParams" @update:value="onEnableParamsChange" />
          <n-text depth="3" style="font-size: 12px">
            配置URL路径参数值
            <n-text v-if="selectedApiInfo?.pathParamNames" type="info" style="margin-left: 8px">
              (需要配置: {{ selectedApiInfo.pathParamNames.join(', ') }})
            </n-text>
          </n-text>
        </n-space>
      </n-form-item>

      <!-- 传参配置 -->
      <n-form-item v-if="addressType === 'internal' && enableParams && selectedApiInfo?.hasPathParams" label="参数配置">
        <DynamicParameterEditor
          :model-value="urlParams"
          parameter-type="path"
          title=""
          add-button-text="添加URL参数"
          key-placeholder="参数名（如：id）"
          value-placeholder="参数值"
          :max-parameters="1"
          :current-api-info="selectedApiInfo"
          :current-component-id="componentId"
          @update:model-value="onUrlParamsUpdate"
        />
        <n-text v-if="urlParams.length === 0" depth="3" style="font-size: 12px; margin-top: 8px">
          💡 提示：配置参数值后将自动替换到上方地址的占位符中
        </n-text>
        <n-text v-else-if="urlParams.length > 0" depth="3" style="font-size: 12px; margin-top: 8px">
          ✅ 参数已配置，地址中的 {{ '{' + urlParams[0].key + '}' }} 将被替换为 "{{ urlParams[0].value }}"
        </n-text>
      </n-form-item>

      <n-form-item label="请求方法" required>
        <n-select
          :value="modelValue.method"
          :options="httpMethods"
          @update:value="(value) => updateConfig('method', value)"
        />
      </n-form-item>

      <n-form-item label="超时时间 (ms)">
        <n-input-number
          :value="modelValue.timeout"
          :min="1000"
          :max="60000"
          :step="1000"
          @update:value="(value) => updateConfig('timeout', value)"
        />
      </n-form-item>

      <n-form-item v-if="showBody" label="请求体">
        <n-input
          :value="modelValue.body"
          type="textarea"
          :rows="4"
          placeholder='{"key": "value"}'
          :input-props="{ style: 'font-family: monospace; font-size: 12px;' }"
          @update:value="(value) => updateConfig('body', value)"
        />
      </n-form-item>
    </n-form>
  </div>
</template>

<style scoped>
.http-config-step1 {
  width: 100%;
  padding: 12px;
}

/* 地址显示区域样式 */
.http-config-step1 :deep(.n-input--readonly) {
  background-color: var(--code-color);
  border: 1px solid var(--border-color);
}

.http-config-step1 :deep(.n-input--readonly .n-input__input) {
  color: var(--text-color);
  font-family: monospace;
  font-size: 13px;
  font-weight: 500;
}

.address-type-indicator {
  display: inline-flex;
  align-items: center;
  font-size: 14px;
  margin-right: 4px;
}
</style>
