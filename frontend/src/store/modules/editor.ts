/**
 * 文件用途: Pinia 可视化编辑器基础画布 store，保存节点、视口和编辑/预览模式。
 * 核心逻辑: 提供节点增删改查、视口更新、模式切换和重置能力，并在删除节点时同步 widget 选中状态。
 * 关键注意事项: 该 store 与 widget 选择状态耦合，同时需要和 visual-editor 统一架构保持职责边界清晰。
 * 重构建议: 明确 classic 画布状态与 unified-editor 的迁移边界，并补充节点删除、视口重置和选择联动测试。
 */
import { defineStore } from 'pinia'
import type { GraphData, CanvasState } from '@/components/visual-editor/types/base-types'
import { useWidgetStore } from './widget'

// Visual editor canvas state.
interface EditorState {
  nodes: GraphData[]
  viewport: {
    zoom: number
    offsetX: number
    offsetY: number
  }
  mode: 'edit' | 'preview'
}

export const useEditorStore = defineStore('editor', {
  state: (): EditorState => ({
    nodes: [],
    viewport: {
      zoom: 1,
      offsetX: 0,
      offsetY: 0
    },
    mode: 'edit'
  }),
  getters: {
    selectedNodeId: (): string | undefined => {
      const widgetStore = useWidgetStore()
      return widgetStore.selectedIds[0]
    }
  },
  actions: {
    // 节点操作
    addNode(node: GraphData) {
      const widgetStore = useWidgetStore()
      this.nodes.push(node)
      widgetStore.selectNodes([node.id])
    },
    removeNode(id: string) {
      const widgetStore = useWidgetStore()
      this.nodes = this.nodes.filter(node => node.id !== id)
      widgetStore.removeNodeFromSelection(id)
    },
    updateNode(id: string, updates: Partial<GraphData>) {
      const nodeIndex = this.nodes.findIndex(node => node.id === id)
      if (nodeIndex !== -1) {
        this.nodes[nodeIndex] = {
          ...this.nodes[nodeIndex],
          ...updates,
          metadata: {
            ...this.nodes[nodeIndex].metadata,
            updatedAt: Date.now()
          }
        }
      }
    },

    // 视口操作
    updateViewport(updates: Partial<EditorState['viewport']>) {
      Object.assign(this.viewport, updates)
    },

    // 模式切换
    setMode(mode: 'edit' | 'preview') {
      this.mode = mode
    },

    // 重置状态
    reset() {
      this.nodes = []
      this.viewport = { zoom: 1, offsetX: 0, offsetY: 0 }
      this.mode = 'edit'
    }
  }
})
