/**
 * 文件用途: Pinia 组件选择 store，保存可视化编辑器侧栏和画布共享的选中节点 ID。
 * 核心逻辑: 根据 editor store 中的节点列表推导 selectedNodes，并提供选择、清空和删除后同步能力。
 * 关键注意事项: selectedIds 必须随节点删除及时收敛，否则侧栏可能引用已不存在的节点。
 * 重构建议: 与 editor store 的节点生命周期统一建模，并补充多选、清空和删除节点后的选择状态测试。
 */
import { defineStore } from 'pinia'
import { useEditorStore } from './editor'

// Widget selection state shared by visual-editor surfaces.
interface WidgetState {
  selectedIds: string[]
}

export const useWidgetStore = defineStore('widget', {
  state: (): WidgetState => ({
    selectedIds: []
  }),
  getters: {
    selectedNodes: state => {
      const editorStore = useEditorStore()
      return editorStore.nodes.filter(node => state.selectedIds.includes(node.id))
    }
  },
  actions: {
    // 选择操作
    selectNodes(ids: string[]) {
      this.selectedIds = [...ids]
    },
    clearSelection() {
      this.selectedIds = []
    },
    // 当节点被删除时，也需要更新选中状态
    removeNodeFromSelection(id: string) {
      this.selectedIds = this.selectedIds.filter(selectedId => selectedId !== id)
    },
    reset() {
      this.selectedIds = []
    }
  }
})
