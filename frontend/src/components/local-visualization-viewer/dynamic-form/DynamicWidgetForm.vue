<script setup lang="ts">
import { ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NColorPicker,
  NDrawer,
  NDrawerContent,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NRadio,
  NRadioGroup,
  NSelect,
  NSpace,
  NSwitch,
  NTabPane,
  NTabs,
  useMessage
} from 'naive-ui'
import type { LocalWidgetConfig, LocalWidgetType } from '../types'
import { TimewindowSelector } from '../timewindow'
import type { DynamicWidgetFormData } from './types'
import {
  convertFormToWidgetConfig,
  convertWidgetConfigToForm,
  validateWidgetForm
} from './form-schema'

const UNIT_SYSTEM_OPTIONS = [
  { label: '公制 (Metric: °C, m, kg, L, kPa, m/s...)', value: 'metric' },
  { label: '英制 (Imperial: °F, ft, lb, gal, psi, mph...)', value: 'imperial' }
]

const unitMode = ref<'system' | 'custom'>('system')

const props = withDefaults(
  defineProps<{
    show: boolean
    type: LocalWidgetType
    config?: LocalWidgetConfig | null
  }>(),
  {
    show: false,
    config: null
  }
)

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'save', config: LocalWidgetConfig): void
}>()

const message = useMessage()
const formData = ref<DynamicWidgetFormData>({})
const activeTab = ref('general')

const CHART_STYLE_OPTIONS = [
  { label: '标准折线图 (Line)', value: 'line' },
  { label: '平滑曲线图 (Smooth)', value: 'smooth' },
  { label: '面积填充图 (Area)', value: 'area' },
  { label: '直方柱状图 (Bar)', value: 'bar' }
]

const ENTITY_TYPE_OPTIONS = [
  { label: '设备 (Device)', value: 'device' },
  { label: '资产 (Asset)', value: 'asset' },
  { label: '网关 (Gateway)', value: 'gateway' },
  { label: '客户 (Customer)', value: 'customer' }
]

const TARGET_ENTITY_TYPE_OPTIONS = [
  { label: '设备 (Device)', value: 'device' },
  { label: '资产 (Asset)', value: 'asset' },
  { label: '网关 (Gateway)', value: 'gateway' }
]

const AGGREGATION_OPTIONS = [
  { label: '最新值 (Latest)', value: 'latest' },
  { label: '平均值 (Average)', value: 'avg' },
  { label: '累加求和 (Sum)', value: 'sum' },
  { label: '最大值 (Max)', value: 'max' },
  { label: '最小值 (Min)', value: 'min' },
  { label: '目标计数 (Count)', value: 'count' }
]

const COMMON_RELATION_TYPES = [
  { label: 'Contains (包含)', value: 'Contains' },
  { label: 'Monitors (监控)', value: 'Monitors' },
  { label: 'Manages (管理)', value: 'Manages' },
  { label: 'BelongsTo (归属于)', value: 'BelongsTo' },
  { label: 'DependsOn (依赖于)', value: 'DependsOn' }
]

function syncFromProps() {
  formData.value = convertWidgetConfigToForm(props.type, props.config)
  if (!formData.value.threshold) {
    formData.value.threshold = { enabled: false, value: 0, label: '报警阈值', color: '#ff4d4f' }
  }
  if (!formData.value.entityRelation) {
    formData.value.entityRelation = {
      enabled: false,
      rootType: 'device',
      rootId: '',
      direction: 'from',
      relationType: 'Contains',
      targetType: 'device',
      targetKey: '',
      aggregation: 'latest'
    }
  }
  if (!formData.value.unitConversion) {
    formData.value.unitConversion = {
      enabled: false,
      unitSystem: 'imperial',
      targetUnit: ''
    }
  }
  if (formData.value.unitConversion.targetUnit) {
    unitMode.value = 'custom'
  } else {
    unitMode.value = 'system'
  }
  if (props.type === 'html') {
    activeTab.value = 'html_editor'
    if (!formData.value.html) {
      formData.value.html = '<div class="html-card">\n  <h4>HTML 容器</h4>\n  <p>状态: <span class="badge">正常</span></p>\n</div>'
      formData.value.css = '.html-card { padding: 8px; }\n.badge { background: #10b981; color: #fff; padding: 2px 6px; border-radius: 4px; font-size: 12px; }'
    }
  } else {
    activeTab.value = 'general'
  }
}

watch(() => [props.show, props.type, props.config], () => {
  if (props.show) syncFromProps()
}, { immediate: true, deep: true })

function handleClose() {
  emit('update:show', false)
}

function handleSave() {
  if (formData.value.unitConversion?.enabled) {
    if (unitMode.value === 'system') {
      formData.value.unitConversion.targetUnit = undefined
    } else {
      formData.value.unitConversion.unitSystem = undefined
    }
  }
  const result = validateWidgetForm(props.type, formData.value)
  if (!result.valid) {
    message.error(result.errors.join('；'))
    return
  }
  const config = convertFormToWidgetConfig(props.type, formData.value)
  emit('save', config)
  emit('update:show', false)
  message.success('小部件配置已更新')
}
</script>

<template>
  <NDrawer
    :show="show"
    width="500"
    placement="right"
    @update:show="handleClose"
  >
    <NDrawerContent :title="`配置 ${type.toUpperCase()} 小部件`" closable>
      <NTabs v-model:value="activeTab" type="line" animated>
        <!-- 基础配置 -->
        <NTabPane name="general" tab="基础设置">
          <NForm :model="formData" label-placement="top" size="small" class="pt-2">
            <NFormItem v-if="type === 'text'" label="展示文本 (支持静态文本或模板)">
              <NInput v-model:value="formData.title" placeholder="例如：车间主控设备运行中" />
            </NFormItem>
            <NFormItem v-else label="小部件标题 (Title)">
              <NInput v-model:value="formData.title" placeholder="例如：实时环境温度监测" />
            </NFormItem>

            <NFormItem label="绑定遥测/属性字段 (Field Key)">
              <NInput v-model:value="formData.field" placeholder="例如：temperature, humidity, voltage" />
            </NFormItem>

            <NFormItem label="无数据时的回退兜底显示 (Fallback)">
              <NInput v-model:value="formData.fallback" placeholder="例如：-- 或 离线" />
            </NFormItem>
          </NForm>
        </NTabPane>

        <!-- HTML 容器专属代码配置 (ROADMAP TB-11) -->
        <NTabPane v-if="type === 'html'" name="html_editor" tab="HTML / CSS 代码">
          <NForm :model="formData" label-placement="top" size="small" class="pt-2">
            <NFormItem label="HTML 模板 (支持 {{field}} 或 ${field} 占位符)">
              <NInput
                v-model:value="formData.html"
                type="textarea"
                :rows="8"
                placeholder="<div class=&quot;custom-card&quot;>\n  <h4>设备状态</h4>\n  <p>实时遥测: {{temp}} °C</p>\n</div>"
              />
            </NFormItem>

            <NFormItem label="自定义 CSS (自动 Scoped 隔离至当前小部件)">
              <NInput
                v-model:value="formData.css"
                type="textarea"
                :rows="5"
                placeholder=".custom-card { padding: 8px; }\nh4 { color: #1f2937; }"
              />
            </NFormItem>

            <NAlert type="info" :show-icon="false" class="text-xs">
              安全防御说明 (Fail-Closed)：系统内嵌 XSS 递归白名单净化引擎，自动剥离 script、iframe、on* 事件及危险伪协议，并在渲染时自动注入 Scoped 作用域防样式污染。
            </NAlert>
          </NForm>
        </NTabPane>

        <!-- 指标小部件特定 -->
        <NTabPane v-if="type === 'metric'" name="metric_options" tab="指标参数">
          <NForm :model="formData" label-placement="top" size="small" class="pt-2">
            <NFormItem label="显示标签 (Label)">
              <NInput v-model:value="formData.label" placeholder="例如：温度" />
            </NFormItem>

            <NFormItem label="单位符号 (Unit)">
              <NInput v-model:value="formData.unit" placeholder="例如：℃, %, kW·h" />
            </NFormItem>

            <NFormItem label="小数保留位数 (Decimals)">
              <NInputNumber v-model:value="formData.decimals" :min="0" :max="10" placeholder="例如：2" class="w-full" />
            </NFormItem>
          </NForm>
        </NTabPane>

        <!-- 图表样式与阈值配置 -->
        <NTabPane v-if="type === 'line-chart' || type === 'bar-chart'" name="chart_visual" tab="视觉与阈值">
          <NForm :model="formData" label-placement="top" size="small" class="pt-2">
            <NFormItem label="图表表现风格">
              <NSelect v-model:value="formData.chartStyle" :options="CHART_STYLE_OPTIONS" />
            </NFormItem>

            <NFormItem label="系列名称 (Series Name)">
              <NInput v-model:value="formData.seriesName" placeholder="例如：实测值" />
            </NFormItem>

            <div class="grid grid-cols-2 gap-2">
              <NFormItem label="Y 轴下限 (Min)">
                <NInputNumber v-model:value="formData.yMin" placeholder="默认自动" class="w-full" />
              </NFormItem>
              <NFormItem label="Y 轴上限 (Max)">
                <NInputNumber v-model:value="formData.yMax" placeholder="默认自动" class="w-full" />
              </NFormItem>
            </div>

            <!-- 报警阈值参考线 -->
            <div class="mt-4 rounded border border-gray-100 p-3 bg-gray-50/50">
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-semibold text-gray-700">告警参考阈值线 (Threshold Line)</span>
                <NSwitch v-model:value="formData.threshold!.enabled" size="small" />
              </div>

              <div v-if="formData.threshold?.enabled" class="space-y-2 pt-2">
                <div class="grid grid-cols-2 gap-2">
                  <NFormItem label="阈值数值" :show-feedback="false">
                    <NInputNumber v-model:value="formData.threshold!.value" class="w-full" />
                  </NFormItem>
                  <NFormItem label="标识文字" :show-feedback="false">
                    <NInput v-model:value="formData.threshold!.label" placeholder="警戒线" />
                  </NFormItem>
                </div>
                <NFormItem label="线条颜色" :show-feedback="false">
                  <NColorPicker v-model:value="formData.threshold!.color" />
                </NFormItem>
              </div>
            </div>

            <!-- 静态模拟数据（无动态时 fallback） -->
            <div class="mt-3">
              <NFormItem label="静态分类标签 (逗号分隔)">
                <NInput v-model:value="formData.categoriesText" placeholder="00:00, 04:00, 08:00, 12:00" />
              </NFormItem>
              <NFormItem label="静态数值列表 (逗号分隔)">
                <NInput v-model:value="formData.valuesText" placeholder="10, 25, 40, 30" />
              </NFormItem>
            </div>
          </NForm>
        </NTabPane>

        <!-- 独立时间窗口覆盖 -->
        <NTabPane v-if="type === 'line-chart' || type === 'bar-chart'" name="timewindow" tab="时间窗口">
          <div class="space-y-4 pt-2">
            <div class="flex items-center justify-between rounded border border-gray-100 p-3">
              <div>
                <div class="text-sm font-medium">小部件独立时间窗口</div>
                <div class="text-xs text-gray-400">开启后将不再继承看板全局时间窗口，使用当前小部件私有窗口</div>
              </div>
              <NSwitch v-model:value="formData.overrideTimewindow" />
            </div>

            <div v-if="formData.overrideTimewindow" class="pt-2">
              <TimewindowSelector v-model="formData.timewindow" />
            </div>
          </div>
        </NTabPane>

        <!-- 实体关系数据源 (ROADMAP P1.1 对标 ThingsBoard) -->
        <NTabPane name="entity_relation" tab="实体关系数据源">
          <div class="space-y-4 pt-2">
            <div class="flex items-center justify-between rounded border border-gray-100 p-3 bg-gray-50/50">
              <div>
                <div class="text-sm font-medium">按实体关系动态关联数据源</div>
                <div class="text-xs text-gray-400">对标 ThingsBoard 关系图谱：从起点实体沿拓扑关系动态查找目标实体并提取遥测</div>
              </div>
              <NSwitch v-model:value="formData.entityRelation!.enabled" />
            </div>

            <div v-if="formData.entityRelation?.enabled" class="space-y-3 pt-2">
              <div class="grid grid-cols-2 gap-2">
                <NFormItem label="起点实体类型">
                  <NSelect v-model:value="formData.entityRelation!.rootType" :options="ENTITY_TYPE_OPTIONS" />
                </NFormItem>
                <NFormItem label="起点实体 ID">
                  <NInput v-model:value="formData.entityRelation!.rootId" placeholder="例如：gw-001 或资产 ID" />
                </NFormItem>
              </div>

              <NFormItem label="关系拓扑方向">
                <NRadioGroup v-model:value="formData.entityRelation!.direction">
                  <NSpace>
                    <NRadio value="from">从起点发出 (起点 → 目标)</NRadio>
                    <NRadio value="to">指向起点 (目标 → 起点)</NRadio>
                  </NSpace>
                </NRadioGroup>
              </NFormItem>

              <NFormItem label="关系类型 (Relation Type)">
                <NSelect
                  v-model:value="formData.entityRelation!.relationType"
                  filterable
                  tag
                  :options="COMMON_RELATION_TYPES"
                  placeholder="选择或直接输入关系类型，如 Contains"
                />
              </NFormItem>

              <div class="grid grid-cols-2 gap-2">
                <NFormItem label="目标实体类型">
                  <NSelect v-model:value="formData.entityRelation!.targetType" :options="TARGET_ENTITY_TYPE_OPTIONS" />
                </NFormItem>
                <NFormItem label="目标遥测 Key">
                  <NInput v-model:value="formData.entityRelation!.targetKey" placeholder="例如：temperature, humidity" />
                </NFormItem>
              </div>

              <NFormItem label="多实体聚合策略">
                <NSelect v-model:value="formData.entityRelation!.aggregation" :options="AGGREGATION_OPTIONS" />
              </NFormItem>
            </div>
          </div>
        </NTabPane>

        <!-- 单位换算 (ROADMAP TB-9 对标 ThingsBoard Units Conversion) -->
        <NTabPane v-if="type === 'metric' || type === 'line-chart' || type === 'bar-chart'" name="unit_conversion" tab="单位换算">
          <div class="space-y-4 pt-2">
            <div class="flex items-center justify-between rounded border border-gray-100 p-3 bg-gray-50/50">
              <div>
                <div class="text-sm font-medium">启用单位自动换算 (Units Conversion)</div>
                <div class="text-xs text-gray-400">对标 ThingsBoard 4.1：将原始物理量统一转换至目标公制/英制单位或指定单位</div>
              </div>
              <NSwitch v-model:value="formData.unitConversion!.enabled" />
            </div>

            <div v-if="formData.unitConversion?.enabled" class="space-y-3 pt-2">
              <NFormItem v-if="type === 'line-chart' || type === 'bar-chart'" label="图表源物理单位 (Source Unit)">
                <NInput v-model:value="formData.unit" placeholder="例如：°C, kPa, km/h, m/s (为空时将使用原始数值)" />
              </NFormItem>

              <NFormItem label="换算目标模式">
                <NRadioGroup v-model:value="unitMode">
                  <NSpace>
                    <NRadio value="system">按目标制式换算 (Unit System)</NRadio>
                    <NRadio value="custom">指定目标单位 (Target Unit)</NRadio>
                  </NSpace>
                </NRadioGroup>
              </NFormItem>

              <NFormItem v-if="unitMode === 'system'" label="目标制式 (Target System)">
                <NSelect
                  v-model:value="formData.unitConversion!.unitSystem"
                  :options="UNIT_SYSTEM_OPTIONS"
                  placeholder="选择公制或英制"
                />
              </NFormItem>

              <NFormItem v-else label="目标单位符号 (Target Unit Symbol)">
                <NInput
                  v-model:value="formData.unitConversion!.targetUnit"
                  placeholder="例如：°F, psi, mph, gal, BTU, hp"
                />
              </NFormItem>

              <NAlert type="info" :show-icon="false" class="text-xs">
                物理安全保证 (Fail-Closed)：量纲不匹配或不支持的聚合将安全保留原值，绝不在物理层面上伪造错误数据。
              </NAlert>
            </div>
          </div>
        </NTabPane>
      </NTabs>

      <template #footer>
        <div class="flex justify-end gap-2">
          <NButton @click="handleClose">取消</NButton>
          <NButton type="primary" @click="handleSave">保存配置</NButton>
        </div>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>
