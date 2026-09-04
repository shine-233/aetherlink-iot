<!--
规则链可视化编辑器（ROADMAP B2）：基于 @vue-flow/core 的拖拽式 DAG 编排。
左侧节点面板拖入画布，连线表达执行顺序，右侧属性面板编辑节点配置；
保存时序列化为 {nodes:[{id,type,name,config}], edges:[{from,to}]} 提交后端。
校验边界：DAG 无环与类型合法性由后端 ParseRuleChainGraph 兜底。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { VueFlow, useVueFlow, type Connection } from '@vue-flow/core'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import { ruleChainCreate, ruleChainGet, ruleChainUpdate } from '@/service/api'
import { $t } from '@/locales'

const route = useRoute()
const router = useRouter()
const chainId = String(route.query.id || '')

interface PaletteItem {
  type: string
  label: string
}

const palette: PaletteItem[] = [
  { type: 'trigger.telemetry', label: $t('custom.rule_chain.nodeTriggerTelemetry') },
  { type: 'trigger.device_online', label: $t('custom.rule_chain.nodeTriggerOnline') },
  { type: 'filter.threshold', label: $t('custom.rule_chain.nodeFilterThreshold') },
  { type: 'transform.mapping', label: $t('custom.rule_chain.nodeTransformMapping') },
  { type: 'action.webhook', label: $t('custom.rule_chain.nodeActionWebhook') },
  { type: 'action.command', label: $t('custom.rule_chain.nodeActionCommand') },
  { type: 'action.alarm', label: $t('custom.rule_chain.nodeActionAlarm') }
]

const flowNodes = ref<any[]>([])
const flowEdges = ref<any[]>([])
const selectedNodeId = ref<string>('')

const form = reactive({
  name: '',
  description: '',
  enabled: true
})
const saving = ref(false)
const loading = ref(false)

const { addEdges, project, onConnect, onNodeClick, onPaneClick } = useVueFlow()

onConnect((connection: Connection) => {
  // 禁止自环；重复边由序列化侧去重即可
  if (connection.source === connection.target) return
  addEdges([{ ...connection, id: `e-${connection.source}-${connection.target}-${flowEdges.value.length}` }])
})

onNodeClick(({ node }) => {
  selectedNodeId.value = node.id
  syncMappingFromSelection()
})

onPaneClick(() => {
  selectedNodeId.value = ''
})

let nodeSeq = 0
function nextNodeId() {
  nodeSeq += 1
  return `n${Date.now().toString(36)}${nodeSeq}`
}

function onDrop(event: DragEvent) {
  const type = event.dataTransfer?.getData('application/rule-chain-node')
  if (!type) return
  const position = project({ x: event.clientX - 280, y: event.clientY - 120 })
  const id = nextNodeId()
  flowNodes.value.push({
    id,
    type: 'input',
    position,
    data: { label: `${labelOf(type)}\n#${id}` }
  })
  if (type.startsWith('action.')) {
    ensureOutputHandle(id)
  }
}

// input 类型节点默认只有 source 句柄；动作/转换节点需要 target+source，
// 统一改用 default 类型（双向句柄），仅触发器用 input。
function ensureOutputHandle(_id: string) {
  /* default 节点自带双句柄，无需处理 */
}

function labelOf(type: string) {
  const found = palette.find(item => item.type === type)
  return found ? found.label : type
}

const selectedNode = computed<any>(() => flowNodes.value.find(node => node.id === selectedNodeId.value) || null)

const selectedConfig = computed(() => {
  return (selectedNode.value?.data?.config as Record<string, any> | undefined) ?? {}
})

function updateSelectedConfig(key: string, value: unknown) {
  if (!selectedNode.value) return
  const config = { ...(selectedNode.value.data.config as Record<string, any> | undefined) || {} }
  config[key] = value
  selectedNode.value.data = { ...selectedNode.value.data, config }
}

const isThresholdNode = computed(
  () => !!selectedNode.value && graphNodeType(selectedNode.value) === 'filter.threshold'
)
function graphNodeType(node: any): string {
  // 创建时把原始类型存进 data.nodeType，避免 Vue Flow 自身 node type 冲突
  return String(node?.data?.nodeType ?? '')
}

const isMappingNode = computed(
  () => !!selectedNode.value && graphNodeType(selectedNode.value) === 'transform.mapping'
)
const isWebhookNode = computed(
  () => !!selectedNode.value && graphNodeType(selectedNode.value) === 'action.webhook'
)
const isCommandNode = computed(
  () => !!selectedNode.value && graphNodeType(selectedNode.value) === 'action.command'
)

const isAlarmNode = computed(
  () => !!selectedNode.value && graphNodeType(selectedNode.value) === 'action.alarm'
)
// mapping fields 以 "from=to" 行文本编辑，简单直观
const mappingLines = ref('')

function syncMappingFromSelection() {
  if (!isMappingNode.value) return
  const fields = (selectedConfig.value.fields as Record<string, any> | undefined) || {}
  mappingLines.value = Object.entries(fields)
    .map(([from, to]) => `${from}=${String(to ?? from)}`)
    .join('\n')
}

function applyMappingLines() {
  const fields: Record<string, string> = {}
  for (const line of mappingLines.value.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const [from, to] = trimmed.split('=')
    if (!from) continue
    fields[from.trim()] = (to || from).trim()
  }
  updateSelectedConfig('fields', fields)
}

function handleParamsBlur(event: FocusEvent) {
  const input = event.target as HTMLInputElement
  try {
    updateSelectedConfig('params', JSON.parse(input.value || '{}'))
  } catch {
    window.$message?.error($t('custom.rule_chain.invalidJson'))
  }
}

function thresholdOpOptions() {
  return ['>', '>=', '<', '<=', '==', '!='].map(op => ({ label: op, value: op }))
}

async function loadChain() {
  if (!chainId) return
  loading.value = true
  try {
    const { data, error } = await ruleChainGet(chainId)
    if (!error && data) {
      form.name = data.name
      form.description = data.description || ''
      form.enabled = Boolean(data.enabled)
      let graph = data.graph
      if (typeof graph === 'string') {
        try {
          graph = JSON.parse(graph)
        } catch {
          graph = { nodes: [], edges: [] }
        }
      }
      const nodes: any[] = []
      for (const gn of graph.nodes || []) {
        nodes.push({
          id: gn.id,
          type: gn.type?.startsWith('trigger.') ? 'input' : 'default',
          position: gn.position || { x: 80 + nodes.length * 60, y: 60 + nodes.length * 50 },
          data: {
            label: `${labelOf(gn.type)}\n#${gn.id}`,
            nodeType: gn.type,
            config: gn.config || {}
          }
        })
      }
      flowNodes.value = nodes
      flowEdges.value = (graph.edges || []).map((edge: any, index: number) => ({
        id: `e-${index}`,
        source: edge.from,
        target: edge.to
      }))
      nodeSeq = nodes.length
    }
  } finally {
    loading.value = false
  }
}

function serializeGraph() {
  const nodes = flowNodes.value.map(node => ({
    id: node.id,
    type: String(node.data?.nodeType || ''),
    name: String(node.data?.label || '').split('\n')[0],
    config: (node.data?.config as Record<string, any>) || {},
    position: node.position
  }))
  const seen = new Set<string>()
  const edges = flowEdges.value
    .filter(edge => {
      const key = `${edge.source}->${edge.target}`
      if (seen.has(key)) return false
      seen.add(key)
      return edge.source !== edge.target
    })
    .map(edge => ({ from: edge.source, to: edge.target }))
  return { nodes, edges }
}

async function handleSave() {
  if (!form.name.trim()) {
    window.$message?.error($t('custom.rule_chain.nameRequired'))
    return
  }
  if (!flowNodes.value.length) {
    window.$message?.error($t('custom.rule_chain.emptyCanvas'))
    return
  }
  saving.value = true
  try {
    const payload = {
      name: form.name,
      description: form.description || null,
      enabled: form.enabled,
      graph: JSON.stringify(serializeGraph())
    }
    if (chainId) {
      const { error } = await ruleChainUpdate({ id: chainId, ...payload })
      if (error) return
    } else {
      const { data, error } = await ruleChainCreate(payload)
      if (error) return
      if (data?.id) {
        router.replace({ path: '/automation/rule-chain/edit', query: { id: data.id } })
      }
    }
    window.$message?.success($t('common.operationSuccess'))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadChain().then(syncMappingFromSelection)
})

defineExpose({ serializeGraph })
</script>

<template>
  <div class="rule-chain-editor min-h-full bg-gray-50 p-4 dark:bg-[#101014]">
    <n-card :bordered="false" class="rounded-8px">
      <template #header>
        <div class="flex items-center gap-3">
          <span>{{ chainId ? $t('custom.rule_chain.editTitle') : $t('custom.rule_chain.create') }}</span>
          <n-input v-model:value="form.name" :placeholder="$t('custom.rule_chain.name')" style="width: 220px" />
          <n-switch v-model:value="form.enabled">
            <template #checked>{{ $t('custom.rule_chain.enabled') }}</template>
            <template #unchecked>{{ $t('custom.rule_chain.disabled') }}</template>
          </n-switch>
        </div>
      </template>
      <template #header-extra>
        <div class="flex gap-2">
          <n-button @click="router.push('/automation/rule-chain')">
            {{ $t('common.back') }}
          </n-button>
          <n-button type="primary" :loading="saving" @click="handleSave">
            {{ $t('common.save') }}
          </n-button>
        </div>
      </template>

      <div class="editor-layout">
        <!-- 左：节点面板 -->
        <aside class="palette">
          <div class="palette-title">{{ $t('custom.rule_chain.palette') }}</div>
          <div
            v-for="item in palette"
            :key="item.type"
            class="palette-item"
            draggable="true"
            @dragstart="event => event.dataTransfer?.setData('application/rule-chain-node', item.type)"
          >
            {{ item.label }}
          </div>
          <n-alert type="default" :show-icon="false" class="mt-3 text-12px">
            {{ $t('custom.rule_chain.canvasHint') }}
          </n-alert>
        </aside>

        <!-- 中：画布 -->
        <div class="canvas-wrap" @drop.prevent="onDrop" @dragover.prevent>
          <VueFlow v-model:nodes="flowNodes" v-model:edges="flowEdges" fit-view-on-init />
        </div>

        <!-- 右：属性面板 -->
        <aside class="props">
          <div class="palette-title">{{ $t('custom.rule_chain.properties') }}</div>
          <template v-if="selectedNode">
            <n-descriptions :column="1" size="small" bordered class="mb-3">
              <n-descriptions-item :label="$t('custom.rule_chain.nodeId')">
                {{ selectedNode.id }}
              </n-descriptions-item>
              <n-descriptions-item :label="$t('custom.device_details.modbusType')">
                {{ graphNodeType(selectedNode) }}
              </n-descriptions-item>
            </n-descriptions>

            <template v-if="isThresholdNode">
              <n-form-item :label="$t('custom.rule_chain.thresholdKey')" label-placement="top">
                <n-input :value="String(selectedConfig.key || '')" @update:value="(v: string) => updateSelectedConfig('key', v)" />
              </n-form-item>
              <n-form-item :label="$t('custom.rule_chain.thresholdOp')" label-placement="top">
                <n-select :value="String(selectedConfig.op || '>')" :options="thresholdOpOptions()" @update:value="(v: string) => updateSelectedConfig('op', v)" />
              </n-form-item>
              <n-form-item :label="$t('custom.rule_chain.thresholdValue')" label-placement="top">
                <n-input-number :value="Number(selectedConfig.value ?? 0)" style="width: 100%" @update:value="(v: number | null) => updateSelectedConfig('value', Number(v ?? 0))" />
              </n-form-item>
            </template>

            <template v-if="isMappingNode">
              <n-form-item :label="$t('custom.rule_chain.mappingHint')" label-placement="top">
                <n-input
                  v-model:value="mappingLines"
                  type="textarea"
                  :autosize="{ minRows: 4, maxRows: 10 }"
                  placeholder="temperature=temp_c"
                  @blur="applyMappingLines"
                />
              </n-form-item>
            </template>

            <template v-if="isWebhookNode">
              <n-form-item :label="$t('custom.rule_chain.webhookUrl')" label-placement="top">
                <n-input :value="String(selectedConfig.url || '')" placeholder="https://example.com/hook" @update:value="(v: string) => updateSelectedConfig('url', v)" />
              </n-form-item>
            </template>

            <template v-if="isCommandNode">
              <n-form-item :label="$t('custom.rule_chain.commandIdentify')" label-placement="top">
                <n-input :value="String(selectedConfig.identify || '')" @update:value="(v: string) => updateSelectedConfig('identify', v)" />
              </n-form-item>
              <n-form-item :label="$t('custom.rule_chain.commandParams')" label-placement="top">
                <n-input
                  :value="JSON.stringify(selectedConfig.params || {})"
                  type="textarea"
                  :autosize="{ minRows: 3, maxRows: 6 }"
                  @blur="handleParamsBlur"
                />
              </n-form-item>
            </template>

            <template v-if="isAlarmNode">
              <n-form-item :label="$t('custom.rule_chain.alarmName')" label-placement="top">
                <n-input :value="String(selectedConfig.name || '')" @update:value="(v: string) => updateSelectedConfig('name', v)" :placeholder="$t('custom.rule_chain.alarmNamePh')" />
              </n-form-item>
              <n-form-item :label="$t('custom.rule_chain.alarmSeverity')" label-placement="top">
                <n-input :value="String(selectedConfig.severity || 'H')" @update:value="(v: string) => updateSelectedConfig('severity', v)" placeholder="L/M/H" style="width:120px" />
              </n-form-item>
            </template>
          </template>
          <n-empty v-else :description="$t('custom.rule_chain.selectNodeHint')" />
        </aside>
      </div>
    </n-card>
  </div>
</template>

<style scoped>
.editor-layout {
  display: grid;
  grid-template-columns: 200px 1fr 300px;
  gap: 12px;
  height: calc(100vh - 260px);
  min-height: 480px;
}
.palette-title,
.props > .palette-title {
  font-weight: 600;
  margin-bottom: 8px;
}
.palette-item {
  padding: 8px 10px;
  margin-bottom: 8px;
  border: 1px dashed var(--n-border-color);
  border-radius: 6px;
  cursor: grab;
  background: var(--n-color);
  user-select: none;
}
.canvas-wrap {
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background:
    radial-gradient(circle, var(--n-border-color) 1px, transparent 1px) 0 0 / 16px 16px;
}
.props {
  overflow-y: auto;
}
</style>
