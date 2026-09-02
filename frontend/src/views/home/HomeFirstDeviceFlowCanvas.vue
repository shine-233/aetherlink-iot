<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'

const props = defineProps<{
  ready: boolean
  flowNodes: any[]
  getFlowNodeAction: (node: any) => { label: string; disabled: boolean; loading: boolean; run: () => void }
}>()

const emit = defineEmits<{
  focusNode: [key: string]
}>()

const flowNodeEntries = computed(() =>
  props.flowNodes.map((node) => ({
    node,
    action: props.getFlowNodeAction(node)
  }))
)
</script>

<template>
  <div class="first-device-flow-canvas">
    <div class="flex flex-col gap-6px sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <div class="font-600">{{ $t('custom.home.firstDevice.canvas.title') }}</div>
        <div class="mt-3px text-12px line-height-18px text-gray-500">
          {{ $t('custom.home.firstDevice.canvas.desc') }}
        </div>
      </div>
      <n-tag size="small" round :bordered="false" :type="ready ? 'success' : 'warning'">
        {{
          ready ? $t('custom.home.firstDevice.canvas.loopDone') : $t('custom.home.firstDevice.canvas.loopInProgress')
        }}
      </n-tag>
    </div>
    <div class="first-device-flow-grid">
      <div
        v-for="({ node, action }, index) in flowNodeEntries"
        :key="node.key"
        class="first-device-flow-node"
        :class="`first-device-flow-node--${node.state}`"
        @click="emit('focusNode', node.key)"
      >
        <div class="first-device-flow-node-head">
          <span>{{ index + 1 }}</span>
          <n-tag size="tiny" round :bordered="false" :type="node.stateType">{{ node.stateLabel }}</n-tag>
        </div>
        <strong>{{ node.title }}</strong>
        <small>{{ node.short }}</small>
        <p>{{ node.detail }}</p>
        <n-button
          size="tiny"
          class="mt-8px"
          :type="node.state === 'active' ? 'primary' : 'default'"
          :ghost="node.state !== 'active'"
          :disabled="action.disabled"
          :loading="action.loading"
          @click.stop="action.run()"
        >
          {{ action.label }}
        </n-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.first-device-flow-canvas {
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
}

.first-device-flow-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(118px, 1fr));
  gap: 8px;
  margin-top: 12px;
  overflow-x: auto;
  padding-bottom: 2px;
}

.first-device-flow-node {
  position: relative;
  min-width: 0;
  min-height: 142px;
  padding: 10px;
  cursor: pointer;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  background: #f8fafc;
}

.first-device-flow-node:not(:last-child)::after {
  position: absolute;
  top: 50%;
  right: -8px;
  z-index: 1;
  width: 8px;
  height: 2px;
  background: #cbd5e1;
  content: '';
}

.first-device-flow-node--done {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.first-device-flow-node--active {
  border-color: #fed7aa;
  background: #fff7ed;
}

.first-device-flow-node-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}

.first-device-flow-node-head span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 999px;
  background: #94a3b8;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
}

.first-device-flow-node--done .first-device-flow-node-head span {
  background: #16a34a;
}

.first-device-flow-node--active .first-device-flow-node-head span {
  background: #d97706;
}

.first-device-flow-node strong {
  display: block;
  margin-top: 8px;
  color: #0f172a;
  font-size: 13px;
}

.first-device-flow-node small {
  display: block;
  margin-top: 3px;
  color: #64748b;
  font-size: 11px;
}

.first-device-flow-node p {
  margin: 7px 0 0;
  color: #475569;
  font-size: 11px;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

@media (max-width: 640px) {
  .first-device-flow-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    overflow-x: visible;
  }

  .first-device-flow-node:not(:last-child)::after {
    display: none;
  }
}
</style>
