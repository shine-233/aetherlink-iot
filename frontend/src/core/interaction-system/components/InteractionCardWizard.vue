<!--
  文件用途：提供交互卡片的轻量向导界面，用于新增、编辑、删除和概览组件交互规则。
  核心功能：
  1. 维护交互列表，并通过 v-model 与父组件同步。
  2. 支持点击、悬停、数据变化三类触发条件，以及跳转、属性修改两类动作。
  3. 兼容旧版响应载荷，保证历史交互配置仍可回填与保存。
  主要逻辑：弹窗表单负责编辑单条交互，摘要区负责展示规则概览，保存时统一组装 responses 协议字段。
  使用注意事项：
  1. 跳转和属性修改的持久化结构需要与交互引擎及导入导出协议保持一致。
  2. 目标属性选择已经委托给 ComponentPropertySelector，本文件仅维护绑定结果，不再重复预加载候选项。
  静态审查备注：当前文件体量偏大，后续可继续拆分为“摘要格式化”“表单状态机”“协议适配”三个子模块。
-->
<template>
  <div class="interaction-simple">
    <!-- 简洁列表 + 添加按钮 -->
    <div class="interaction-header">
      <h4 class="section-title">{{ t('interaction.wizard.title') }}</h4>
      <n-button size="small" type="primary" @click="showAddModal = true">
        <template #icon>
          <n-icon><FlashOutline /></n-icon>
        </template>
        {{ t('interaction.wizard.addInteraction') }}
      </n-button>
    </div>

    <!-- 交互列表 -->
    <div class="interactions-list">
      <div v-if="interactions.length === 0" class="empty-state">
        <div class="empty-icon">🎯</div>
        <div class="empty-text">{{ t('interaction.wizard.noInteractions') }}</div>
        <div class="empty-desc">{{ t('interaction.wizard.noInteractionsDesc') }}</div>
      </div>

      <div v-else>
        <div v-for="(interaction, index) in interactions" :key="index" class="interaction-item">
          <div class="interaction-summary">
            <div class="summary-badge" :class="getEventType(interaction.event)">
              {{ getEventLabel(interaction.event) }}
            </div>
            <div class="summary-text">
              <div class="summary-title">{{ getSummaryTitle(interaction) }}</div>
              <div class="summary-desc">{{ getSummaryDesc(interaction) }}</div>
            </div>
            <div class="summary-actions">
              <n-switch
                :value="interaction.enabled"
                size="small"
                @update:value="toggleInteractionEnabled(index, $event)"
              />
              <n-button size="tiny" quaternary @click="editInteraction(index)">{{ t('interaction.edit') }}</n-button>
              <n-button size="tiny" quaternary @click="deleteInteraction(index)">
                <template #icon>
                  <n-icon><TrashOutline /></n-icon>
                </template>
              </n-button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 添加/编辑弹窗 -->
    <n-modal
      v-model:show="showAddModal"
      :title="editingIndex >= 0 ? t('interaction.wizard.editInteraction') : t('interaction.wizard.addInteraction')"
    >
      <n-card style="width: 600px" :bordered="false">
        <n-form :model="currentInteraction" label-placement="left" label-width="auto">
          <!-- 触发条件 -->
          <n-form-item :label="t('interaction.events.title')">
            <n-select
              v-model:value="currentInteraction.event"
              :options="eventOptions"
              :placeholder="t('interaction.placeholders.selectTriggerCondition')"
            />
          </n-form-item>

          <!-- 动作类型 -->
          <n-form-item :label="t('interaction.actions.title')">
            <n-select
              v-model:value="currentActionType"
              :options="actionTypeOptions"
              :placeholder="t('interaction.placeholders.selectAction')"
              @update:value="handleActionTypeChange"
            />
          </n-form-item>

          <!-- URL跳转配置 -->
          <template v-if="currentActionType === 'jump'">
            <n-form-item :label="t('interaction.properties.linkType')">
              <n-radio-group v-model:value="urlType" @update:value="handleUrlTypeChange">
                <n-space>
                  <n-radio value="external">{{ t('interaction.linkTypes.external') }}</n-radio>
                  <n-radio value="internal">{{ t('interaction.linkTypes.internal') }}</n-radio>
                </n-space>
              </n-radio-group>
            </n-form-item>

            <n-form-item v-if="urlType === 'external'" :label="t('interaction.properties.jumpAddress')">
              <n-input v-model:value="currentInteraction.url" :placeholder="t('interaction.placeholders.enterUrl')" />
            </n-form-item>

            <n-form-item v-if="urlType === 'internal'" :label="t('interaction.properties.selectMenu')">
              <n-select
                v-model:value="selectedMenuPath"
                :options="menuOptions"
                :placeholder="t('interaction.placeholders.selectMenuToJump')"
                :loading="menuLoading"
                filterable
                @update:value="handleMenuPathChange"
              />
            </n-form-item>

            <n-form-item :label="t('interaction.properties.openMethod')">
              <n-radio-group v-model:value="currentInteraction.target">
                <n-radio value="_self">{{ t('interaction.openMethods.currentWindow') }}</n-radio>
                <n-radio value="_blank">{{ t('interaction.openMethods.newWindow') }}</n-radio>
              </n-radio-group>
            </n-form-item>
          </template>

          <!-- 数据变化时的属性选择和条件配置 -->
          <template v-if="currentInteraction.event === 'dataChange'">
            <n-form-item :label="t('interaction.properties.watchedProperty')">
              <n-select
                v-model:value="currentWatchedProperty"
                :options="availablePropertyOptions"
                :placeholder="t('interaction.placeholders.selectWatchedProperty')"
                @update:value="handleWatchedPropertyChange"
              />
            </n-form-item>

            <n-form-item :label="t('interaction.properties.executionCondition')">
              <n-space>
                <n-select
                  v-model:value="currentConditionType"
                  :options="conditionTypeOptions"
                  :placeholder="t('interaction.placeholders.conditionType')"
                  style="width: 120px"
                  @update:value="handleConditionTypeChange"
                />
                <template v-if="currentConditionType === 'comparison'">
                  <n-select
                    v-model:value="currentConditionOperator"
                    :options="comparisonOperatorOptions"
                    :placeholder="t('interaction.placeholders.comparison')"
                    style="width: 100px"
                  />
                  <n-input
                    v-model:value="currentConditionValue"
                    :placeholder="t('interaction.placeholders.value')"
                    style="width: 120px"
                  />
                </template>
                <template v-else-if="currentConditionType === 'range'">
                  <n-input
                    v-model:value="currentConditionValue"
                    :placeholder="t('interaction.placeholders.rangeValue')"
                    style="width: 120px"
                  />
                </template>
                <template v-else-if="currentConditionType === 'expression'">
                  <n-input
                    v-model:value="currentConditionValue"
                    :placeholder="t('interaction.placeholders.expressionValue')"
                    style="width: 200px"
                  />
                </template>
              </n-space>
            </n-form-item>
          </template>

          <!-- 属性修改配置 -->
          <template v-if="currentActionType === 'modify'">
            <!-- 属性选择器 -->
            <n-form-item :label="t('interaction.properties.modifyProperty')">
              <ComponentPropertySelector
                v-model:value="currentTargetPropertyBinding"
                :placeholder="t('interaction.placeholders.selectPropertyToModify')"
                :current-component-id="props.componentId"
                @change="handleTargetPropertyChange"
              />
            </n-form-item>
            <n-form-item :label="t('interaction.properties.newValue')">
              <n-input
                v-model:value="currentInteraction.updateValue"
                :placeholder="t('interaction.placeholders.enterNewPropertyValue')"
              />
            </n-form-item>
          </template>
        </n-form>

        <template #footer>
          <n-space justify="end">
            <n-button @click="showAddModal = false">{{ t('interaction.cancel') }}</n-button>
            <n-button type="primary" @click="saveInteraction">{{ t('interaction.confirm') }}</n-button>
          </n-space>
        </template>
      </n-card>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
/**
 * 交互卡片配置向导。
 * 采用“列表概览 + 弹窗编辑”的轻量交互模式，兼顾新协议与历史协议回填。
 */

import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NSpace,
  NButton,
  NIcon,
  NInput,
  NSelect,
  NSwitch,
  NRadioGroup,
  NRadio,
  NModal,
  NCard,
  NForm,
  NFormItem,
  useMessage
} from 'naive-ui'
import { FlashOutline, TrashOutline } from '@vicons/ionicons5'
import { fetchGetUserRoutes } from '@/service/api/route'
import ComponentPropertySelector from '@/core/data-architecture/components/common/ComponentPropertySelector.vue'
import { configurationIntegrationBridge } from '@/components/visual-editor/configuration/ConfigurationIntegrationBridge'
import type { InteractionActionType, InteractionUrlType } from './interactionResponseProtocol'
import type { TargetPropertyInfo } from './interactionFormState'
import { interactionSummaryDesc, interactionSummaryTitle } from './interactionSummary'
import { useInteractionCardWizardDraft } from './useInteractionCardWizardDraft'

interface Props {
  modelValue?: any[]
  componentId?: string
  componentType?: string
}

interface Emits {
  (e: 'update:modelValue', value: any[]): void
}

type PropertyDefinition = {
  path: string
  displayPath: string
  type: string
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const message = useMessage()
const { t } = useI18n()

const interactions = ref(props.modelValue || [])
const showAddModal = ref(false)

const {
  editingIndex,
  currentInteraction,
  currentActionType,
  urlType,
  selectedMenuPath,
  currentWatchedProperty,
  currentConditionType,
  currentConditionOperator,
  currentConditionValue,
  currentTargetPropertyBinding,
  currentTargetPropertyInfo,
  beginInteractionEdit,
  saveCurrentInteraction
} = useInteractionCardWizardDraft({
  interactions,
  emitUpdate: (value) => emit('update:modelValue', value),
  loadMenuOptions: () => loadMenuOptions()
})

watch(
  () => props.modelValue,
  (newValue) => {
    if (newValue) {
      interactions.value = [...newValue]
    }
  },
  { immediate: true, deep: true }
)

const menuOptions = ref<{ label: string; value: string }[]>([])
const menuLoading = ref(false)
const reportedUnavailablePropertyConfigs = new Set<string>()

// 触发事件选择项。
const eventOptions = computed(() => [
  { label: t('interaction.events.click'), value: 'click' },
  { label: t('interaction.events.hover'), value: 'hover' },
  { label: t('interaction.events.dataChange'), value: 'dataChange' }
])

const conditionTypeOptions = computed(() => [
  { label: t('interaction.conditions.comparison'), value: 'comparison' },
  { label: t('interaction.conditions.range'), value: 'range' },
  { label: t('interaction.conditions.expression'), value: 'expression' }
])

const comparisonOperatorOptions = computed(() => [
  { label: t('interaction.operators.equals'), value: 'equals' },
  { label: t('interaction.operators.notEquals'), value: 'notEquals' },
  { label: t('interaction.operators.greaterThan'), value: 'greaterThan' },
  { label: t('interaction.operators.greaterThanOrEqual'), value: 'greaterThanOrEqual' },
  { label: t('interaction.operators.lessThan'), value: 'lessThan' },
  { label: t('interaction.operators.lessThanOrEqual'), value: 'lessThanOrEqual' },
  { label: t('interaction.operators.contains'), value: 'contains' },
  { label: t('interaction.operators.startsWith'), value: 'startsWith' },
  { label: t('interaction.operators.endsWith'), value: 'endsWith' }
])

// 动作类型选择项。
const actionTypeOptions = computed(() => [
  { label: t('interaction.summary.pageJump'), value: 'jump' },
  { label: t('interaction.summary.modifyProperty'), value: 'modify' }
])

const baseWatchedPropertyDefinitions: PropertyDefinition[] = [
  // 显示配置
  { path: 'showTitle', displayPath: '显示标题', type: 'boolean' },
  { path: 'title', displayPath: '标题', type: 'string' },
  { path: 'visible', displayPath: '可见性', type: 'boolean' },
  { path: 'opacity', displayPath: '透明度', type: 'number' },

  // 样式配置
  { path: 'backgroundColor', displayPath: '背景颜色', type: 'string' },
  { path: 'borderWidth', displayPath: '边框宽度', type: 'number' },
  { path: 'borderColor', displayPath: '边框颜色', type: 'string' },
  { path: 'borderStyle', displayPath: '边框样式', type: 'string' },
  { path: 'borderRadius', displayPath: '圆角大小', type: 'number' },
  { path: 'boxShadow', displayPath: '阴影效果', type: 'string' },

  // 布局配置
  { path: 'padding', displayPath: '内边距', type: 'object' },
  { path: 'margin', displayPath: '外边距', type: 'object' },

  // 设备关联配置（核心必需）
  { path: 'deviceId', displayPath: '设备ID', type: 'string' },
  { path: 'metricsList', displayPath: '指标列表', type: 'array' }
]

const componentWatchedPropertyDefinitions: PropertyDefinition[] = [
  { path: 'properties', displayPath: '组件属性', type: 'object' },
  { path: 'styles', displayPath: '组件样式', type: 'object' },
  { path: 'behavior', displayPath: '组件行为', type: 'object' }
]

function createWatchedPropertyOption(layer: 'base' | 'component', prop: PropertyDefinition, currentValue: any) {
  const layerLabel = layer === 'base' ? '基础' : '组件'
  return {
    label: `[${layerLabel}] ${prop.displayPath} (${prop.type})`,
    value: `${layer}.${prop.path}`,
    property: {
      name: prop.path,
      label: prop.displayPath,
      type: prop.type,
      currentValue
    }
  }
}

// 数据变化事件只允许监听当前组件配置，因此这里直接从配置桥接层读取候选项。
const availablePropertyOptions = computed(() => {
  if (!props.componentId) {
    reportUnavailablePropertyConfig('__missing_component_id__')
    return []
  }

  const config = configurationIntegrationBridge.getConfiguration(props.componentId)
  if (!config) {
    reportUnavailablePropertyConfig(props.componentId)
    return []
  }

  return [
    ...baseWatchedPropertyDefinitions.map((prop) =>
      createWatchedPropertyOption('base', prop, config?.base?.[prop.path])
    ),
    ...componentWatchedPropertyDefinitions.map((prop) =>
      createWatchedPropertyOption('component', prop, config?.component?.[prop.path])
    )
  ]
})

function reportUnavailablePropertyConfig(key: string) {
  if (reportedUnavailablePropertyConfigs.has(key)) {
    return
  }
  reportedUnavailablePropertyConfigs.add(key)
  console.error('InteractionCardWizard property configuration unavailable', key)
}

const getEventType = (event: string) => {
  const typeMap = {
    click: 'click',
    hover: 'hover',
    dataChange: 'condition'
  }
  return typeMap[event] || 'default'
}

const getEventLabel = (event: string) => {
  const labelMap = {
    click: t('interaction.events.click'),
    hover: t('interaction.events.hover'),
    dataChange: t('interaction.events.dataChange')
  }
  return labelMap[event] || event
}

const getSummaryTitle = (interaction: any) => {
  return interactionSummaryTitle(interaction, t)
}

const getSummaryDesc = (interaction: any) => {
  return interactionSummaryDesc(interaction, t)
}

const editInteraction = (index: number) => {
  beginInteractionEdit(index)
  showAddModal.value = true
}

const deleteInteraction = (index: number) => {
  interactions.value.splice(index, 1)
  emit('update:modelValue', interactions.value)
}

const toggleInteractionEnabled = (index: number, enabled: boolean) => {
  interactions.value = interactions.value.map((interaction, currentIndex) =>
    currentIndex === index
      ? {
          ...interaction,
          enabled
        }
      : interaction
  )
  emit('update:modelValue', interactions.value)
}

// 监听属性选择器当前只消费绑定路径，第二参数由选择器保留给未来扩展。
const handleWatchedPropertyChange = (bindingPath: string) => {
  currentWatchedProperty.value = bindingPath
}

// 将属性选择器返回的结构化信息同步到持久化协议字段。
const handleTargetPropertyChange = (bindingPath: string, propertyInfo?: TargetPropertyInfo) => {
  currentTargetPropertyBinding.value = bindingPath
  currentTargetPropertyInfo.value = propertyInfo ?? null

  if (bindingPath && propertyInfo) {
    currentInteraction.value.targetComponentId = propertyInfo.componentId
    currentInteraction.value.targetProperty = `${propertyInfo.layer}.${propertyInfo.propertyName}`
  } else {
    currentInteraction.value.targetComponentId = ''
    currentInteraction.value.targetProperty = ''
  }
}

const handleConditionTypeChange = (value: string) => {
  currentConditionType.value = value
  currentConditionOperator.value = ''
  currentConditionValue.value = ''
}

// 切换链接类型时清理互斥字段，避免内部路径与外部 URL 串用。
const handleUrlTypeChange = (value?: InteractionUrlType) => {
  if (value) {
    urlType.value = value
  }

  if (urlType.value === 'internal') {
    menuOptions.value = []
    loadMenuOptions()
    currentInteraction.value.url = ''
  } else {
    selectedMenuPath.value = ''
  }
}

const handleMenuPathChange = () => {
  currentInteraction.value.url = selectedMenuPath.value
}

async function loadMenuOptions() {
  menuLoading.value = true
  try {
    const result = await fetchGetUserRoutes()
    if (result && result.data && result.data.list) {
      const flattened = flattenRoutes(result.data.list)
      menuOptions.value = flattened

      if (flattened.length === 0) {
        message.error(t('interaction.messages.menuDataProcessFailed'))
      }
    } else {
      message.error(t('interaction.messages.menuDataAbnormal'))
    }
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error)
    message.error(`${t('interaction.messages.menuLoadFailed')}: ${errorMessage}`)
  } finally {
    menuLoading.value = false
  }
}

// 扁平化菜单路由，保留层级标题，过滤隐藏项。
const flattenRoutes = (routes: any[]): { label: string; value: string }[] => {
  const options: { label: string; value: string }[] = []

  const processRoute = (route: any, parentTitle = '') => {
    const path = route.path
    const title = route.meta?.title || route.meta?.i18nKey || route.name
    const displayLabel = parentTitle ? `${parentTitle} / ${title}` : title

    if (path && title && !route.meta?.hideInMenu) {
      const option = { label: displayLabel, value: path }
      options.push(option)
    }

    if (route.children && Array.isArray(route.children) && route.children.length > 0) {
      route.children.forEach((child) => processRoute(child, displayLabel))
    }
  }

  routes.forEach((route) => processRoute(route))
  return options
}

const handleActionTypeChange = (value: InteractionActionType) => {
  currentActionType.value = value
  if (value === 'jump') {
    urlType.value = 'external'
    currentInteraction.value.url = ''
    currentInteraction.value.target = '_blank'
    selectedMenuPath.value = ''
  } else if (value === 'modify') {
    currentInteraction.value.targetComponentId = ''
    currentInteraction.value.targetProperty = 'backgroundColor'
    currentInteraction.value.updateValue = '#ff0000'
  }
}

const saveInteraction = () => {
  saveCurrentInteraction()
  showAddModal.value = false
}
</script>

<style scoped>
.interaction-simple {
  padding: 16px;
  height: 100%;
}

.interaction-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.section-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-color);
}

/* 空状态 */
.empty-state {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-color-3);
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 12px;
}

.empty-text {
  font-size: 16px;
  font-weight: 500;
  margin-bottom: 8px;
  color: var(--text-color-2);
}

.empty-desc {
  font-size: 12px;
}

/* 交互列表 */
.interactions-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.interaction-item {
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--card-color);
}

.interaction-summary {
  padding: 12px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.summary-badge {
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
}

.summary-badge.click {
  background: var(--success-color-suppl);
  color: var(--success-color);
}

.summary-badge.hover {
  background: var(--info-color-suppl);
  color: var(--info-color);
}

.summary-badge.condition {
  background: var(--warning-color-suppl);
  color: var(--warning-color);
}

.summary-text {
  flex: 1;
}

.summary-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-color);
  margin-bottom: 2px;
}

.summary-desc {
  font-size: 12px;
  color: var(--text-color-3);
}

.summary-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
