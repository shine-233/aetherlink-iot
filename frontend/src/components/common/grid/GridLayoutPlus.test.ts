/**
 * 文件用途：覆盖 GridLayoutPlus 组件的渲染、事件转发和配置交互回归。
 * 核心逻辑：通过 Vue 测试挂载组件，构造布局数据与用户事件，断言输出事件和布局状态。
 * 关键注意事项：测试应保持在组件公开契约层，不要依赖第三方 grid-layout-plus 的私有实现细节。
 * 重构建议：可按只读模式、拖拽缩放、响应式断点和插槽渲染拆分场景，提升失败定位速度。
 */
import { defineComponent, h, reactive } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  themeStore: {
    darkMode: false
  },
  validateExtendedGridConfig: vi.fn(),
  validateLargeGridPerformance: vi.fn(),
  optimizeItemForLargeGrid: vi.fn()
}))

vi.mock('@/store/modules/theme', () => ({
  useThemeStore: () => hoisted.themeStore
}))

vi.mock('./utils/validation', () => ({
  validateExtendedGridConfig: hoisted.validateExtendedGridConfig,
  validateLargeGridPerformance: hoisted.validateLargeGridPerformance,
  optimizeItemForLargeGrid: hoisted.optimizeItemForLargeGrid
}))

vi.mock('./components', () => ({
  GridCore: defineComponent({
    props: ['layout', 'config', 'readonly', 'showTitle'],
    emits: [
      'layout-created',
      'layout-before-mount',
      'layout-mounted',
      'layout-updated',
      'layout-ready',
      'layout-change',
      'breakpoint-changed',
      'container-resized',
      'item-resize',
      'item-resized',
      'item-move',
      'item-moved',
      'item-container-resized'
    ],
    setup(props, { emit, expose, slots }) {
      const internalLayout = reactive([...(props.layout || [])])
      expose({ internalLayout })

      return () =>
        h('div', { class: 'grid-core-stub', 'data-col-num': props.config?.colNum }, [
          h('button', { class: 'emit-created', type: 'button', onClick: () => emit('layout-created', internalLayout) }, 'created'),
          h('button', { class: 'emit-before-mount', type: 'button', onClick: () => emit('layout-before-mount', internalLayout) }, 'before'),
          h('button', { class: 'emit-mounted', type: 'button', onClick: () => emit('layout-mounted', internalLayout) }, 'mounted'),
          h('button', { class: 'emit-updated', type: 'button', onClick: () => emit('layout-updated', internalLayout) }, 'updated'),
          h('button', { class: 'emit-ready', type: 'button', onClick: () => emit('layout-ready', internalLayout) }, 'ready'),
          h('button', { class: 'emit-change', type: 'button', onClick: () => emit('layout-change', internalLayout) }, 'change'),
          h('button', { class: 'emit-breakpoint', type: 'button', onClick: () => emit('breakpoint-changed', 'lg', internalLayout) }, 'breakpoint'),
          h('button', { class: 'emit-container', type: 'button', onClick: () => emit('container-resized', 1200, 800, props.config?.colNum) }, 'container'),
          h('button', { class: 'emit-resize', type: 'button', onClick: () => emit('item-resize', 'node-a', 3, 4, 300, 400) }, 'resize'),
          h('button', { class: 'emit-resized', type: 'button', onClick: () => emit('item-resized', 'node-a', 5, 6, 500, 600) }, 'resized'),
          h('button', { class: 'emit-move', type: 'button', onClick: () => emit('item-move', 'node-a', 7, 8) }, 'move'),
          h('button', { class: 'emit-moved', type: 'button', onClick: () => emit('item-moved', 'node-a', 9, 10) }, 'moved'),
          h('button', { class: 'emit-item-container', type: 'button', onClick: () => emit('item-container-resized', 'node-a', 11, 12, 1100, 1200) }, 'item container'),
          ...(internalLayout || []).map((item: any) => h('div', { class: 'slot-item', 'data-id': item.i }, slots.default?.({ item })))
        ])
    }
  }),
  GridDropZone: defineComponent({
    props: ['readonly', 'showDropZone'],
    emits: ['drag-enter', 'drag-over', 'drag-leave', 'drop'],
    setup(_, { emit }) {
      const dropEvent = () => {
        const event = new Event('drop') as DragEvent
        Object.defineProperty(event, 'dataTransfer', {
          value: {
            getData: () => 'chart-card'
          }
        })
        return event
      }

      return () =>
        h('div', { class: 'grid-drop-zone-stub' }, [
          h('button', { class: 'emit-drag-enter', type: 'button', onClick: () => emit('drag-enter', new Event('dragenter')) }, 'enter'),
          h('button', { class: 'emit-drag-over', type: 'button', onClick: () => emit('drag-over', new Event('dragover')) }, 'over'),
          h('button', { class: 'emit-drag-leave', type: 'button', onClick: () => emit('drag-leave', new Event('dragleave')) }, 'leave'),
          h('button', { class: 'emit-drop', type: 'button', onClick: () => emit('drop', dropEvent()) }, 'drop')
        ])
    }
  })
}))

import GridLayoutPlus from './GridLayoutPlus.vue'

const mountedWrappers: VueWrapper[] = []

const mountGrid = (props: Record<string, unknown> = {}) => {
  const wrapper = mount(GridLayoutPlus, {
    props: {
      layout: [
        {
          id: 'node-a',
          x: 0,
          y: 0,
          w: 2,
          h: 2,
          type: 'chart',
          data: { title: 'A' }
        }
      ],
      idKey: 'id',
      ...props
    },
    slots: {
      default: ({ item }: any) => h('span', { class: 'rendered-slot' }, item.id || item.i)
    },
    attachTo: document.body
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('GridLayoutPlus.vue', () => {
  let randomSpy: ReturnType<typeof vi.spyOn>
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>
  let consoleInfoSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-27T15:00:00.000Z'))
    vi.clearAllMocks()
    document.body.innerHTML = ''
    hoisted.themeStore.darkMode = false
    hoisted.validateExtendedGridConfig.mockReturnValue({ success: true })
    hoisted.validateLargeGridPerformance.mockReturnValue({ success: true, data: null })
    hoisted.optimizeItemForLargeGrid.mockImplementation((item, targetColumns) => ({
      ...item,
      x: item.x * 2,
      w: Math.min(item.w * 2, targetColumns)
    }))
    randomSpy = vi.spyOn(Math, 'random').mockReturnValue(0.123456789)
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    consoleInfoSpy = vi.spyOn(console, 'info').mockImplementation(() => undefined)
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    randomSpy.mockRestore()
    consoleErrorSpy.mockRestore()
    consoleInfoSpy.mockRestore()
    vi.useRealTimers()
    document.body.innerHTML = ''
  })

  it('normalizes custom id keys, applies grid size config, theme classes, and exposes validation info', () => {
    hoisted.themeStore.darkMode = true
    const wrapper = mountGrid({
      readonly: false,
      showGrid: true,
      gridSize: 'custom',
      customColumns: 80,
      config: { rowHeight: 42 }
    })

    expect(wrapper.classes()).toEqual(
      expect.arrayContaining(['grid-layout-plus-wrapper', 'dark-theme', 'show-grid'])
    )
    expect(wrapper.get('.grid-core-stub').attributes('data-col-num')).toBe('80')
    expect(wrapper.get('.rendered-slot').text()).toBe('node-a')
    expect((wrapper.vm as any).getGridInfo()).toMatchObject({
      colNum: 80,
      gridSize: 'custom',
      validation: {
        isValid: true,
        colNum: 80
      }
    })
    expect(hoisted.validateExtendedGridConfig).toHaveBeenCalledWith(80)
    expect(hoisted.validateLargeGridPerformance).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({ id: 'node-a' })]),
      80
    )
  })

  it('forwards layout and resize events while preserving custom id aliases in outbound layout payloads', async () => {
    const wrapper = mountGrid()

    await wrapper.get('.emit-created').trigger('click')
    await wrapper.get('.emit-before-mount').trigger('click')
    await wrapper.get('.emit-mounted').trigger('click')
    await wrapper.get('.emit-updated').trigger('click')
    await wrapper.get('.emit-ready').trigger('click')
    await wrapper.get('.emit-change').trigger('click')
    await wrapper.get('.emit-breakpoint').trigger('click')
    await wrapper.get('.emit-container').trigger('click')
    await wrapper.get('.emit-resize').trigger('click')
    await wrapper.get('.emit-resized').trigger('click')
    await wrapper.get('.emit-move').trigger('click')
    await wrapper.get('.emit-moved').trigger('click')
    await wrapper.get('.emit-item-container').trigger('click')

    expect(wrapper.emitted('layout-created')?.[0]?.[0]).toEqual([
      expect.objectContaining({ i: 'node-a', id: 'node-a' })
    ])
    expect(wrapper.emitted('layout-before-mount')?.[0]?.[0]).toEqual([
      expect.objectContaining({ id: 'node-a' })
    ])
    expect(wrapper.emitted('layout-mounted')?.[0]?.[0]).toEqual([expect.objectContaining({ id: 'node-a' })])
    expect(wrapper.emitted('layout-updated')?.[0]?.[0]).toEqual([expect.objectContaining({ id: 'node-a' })])
    expect(wrapper.emitted('layout-ready')?.[0]?.[0]).toEqual([expect.objectContaining({ id: 'node-a' })])
    expect(wrapper.emitted('layout-change')?.[0]?.[0]).toEqual([expect.objectContaining({ id: 'node-a' })])
    expect(wrapper.emitted('update:layout')?.[0]?.[0]).toEqual([expect.objectContaining({ id: 'node-a' })])
    expect(wrapper.emitted('breakpoint-changed')?.[0]).toEqual([
      'lg',
      [expect.objectContaining({ id: 'node-a' })]
    ])
    expect(wrapper.emitted('container-resized')?.[0]).toEqual([1200, 800, 24])
    expect(wrapper.emitted('item-resize')?.[0]).toEqual(['node-a', 3, 4, 300, 400])
    expect(wrapper.emitted('item-resized')?.[0]).toEqual(['node-a', 5, 6, 500, 600])
    expect(wrapper.emitted('item-move')?.[0]).toEqual(['node-a', 7, 8])
    expect(wrapper.emitted('item-moved')?.[0]).toEqual(['node-a', 9, 10])
    expect(wrapper.emitted('item-container-resized')?.[0]).toEqual(['node-a', 11, 12, 1100, 1200])
  })

  it('adds, updates, removes, reads, clears, and optimizes layout items through the public API', () => {
    const wrapper = mountGrid()
    const vm = wrapper.vm as any
    const expectedGeneratedId = `item-${Date.now()}-4fzzzxjyl`

    const added = vm.addItem('table-card', { w: 3, h: 2, data: { title: 'Table' } })

    expect(added).toMatchObject({
      i: expectedGeneratedId,
      id: expectedGeneratedId,
      x: 2,
      y: 0,
      w: 3,
      h: 2,
      type: 'table-card',
      data: { title: 'Table' }
    })
    expect(wrapper.emitted('item-add')?.[0]?.[0]).toMatchObject({
      id: expectedGeneratedId,
      type: 'table-card'
    })

    expect(vm.getItem('node-a')).toMatchObject({ id: 'node-a' })
    expect(vm.getAllItems()).toHaveLength(2)

    const updated = vm.updateItem('node-a', { w: 5, data: { title: 'Updated' } })
    expect(updated).toMatchObject({ w: 5, data: { title: 'Updated' } })
    expect(wrapper.emitted('item-update')?.[0]).toEqual(['node-a', { w: 5, data: { title: 'Updated' } }])

    const removed = vm.removeItem('node-a')
    expect(removed).toMatchObject({ id: 'node-a' })
    expect(wrapper.emitted('item-delete')?.[0]).toEqual(['node-a'])

    vm.optimizeLayoutForGridSize(60, 24)
    expect(hoisted.optimizeItemForLargeGrid).toHaveBeenCalledWith(
      expect.objectContaining({ id: expectedGeneratedId }),
      60,
      24
    )
    expect(wrapper.emitted('layout-change')?.at(-1)?.[0]).toEqual([
      expect.objectContaining({
        id: expectedGeneratedId,
        w: 6
      })
    ])

    vm.clearLayout()
    expect(vm.getAllItems()).toEqual([])
    expect(wrapper.emitted('update:layout')?.at(-1)?.[0]).toEqual([])
    expect(vm.removeItem('missing')).toBeNull()
    expect(vm.updateItem('missing', { w: 1 })).toBeNull()
  })

  it('adds dropped component types and forwards drag lifecycle events', async () => {
    const wrapper = mountGrid({ showDropZone: true })
    const expectedGeneratedId = `item-${Date.now()}-4fzzzxjyl`

    await wrapper.get('.emit-drag-enter').trigger('click')
    await wrapper.get('.emit-drag-over').trigger('click')
    await wrapper.get('.emit-drag-leave').trigger('click')
    await wrapper.get('.emit-drop').trigger('click')

    expect(wrapper.emitted('drag-enter')?.[0]?.[0]).toBeInstanceOf(Event)
    expect(wrapper.emitted('drag-over')?.[0]?.[0]).toBeInstanceOf(Event)
    expect(wrapper.emitted('drag-leave')?.[0]?.[0]).toBeInstanceOf(Event)
    expect(wrapper.emitted('drop')?.[0]?.[0]).toBeInstanceOf(Event)
    expect(wrapper.emitted('item-add')?.[0]?.[0]).toMatchObject({
      type: 'chart-card',
      id: expectedGeneratedId
    })
  })

  it('reports invalid grid configuration and large-grid performance recommendations through validation API', () => {
    hoisted.validateExtendedGridConfig.mockReturnValueOnce({ success: false, message: 'too many columns' })
    hoisted.validateLargeGridPerformance.mockReturnValueOnce({
      success: true,
      data: {
        warning: 'layout is dense',
        recommendation: 'reduce item count'
      }
    })
    const wrapper = mountGrid({ gridSize: 'mega' })

    expect((wrapper.vm as any).getGridValidation()).toMatchObject({
      isValid: false,
      performance: {
        warning: 'layout is dense',
        recommendation: 'reduce item count'
      }
    })
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      'Grid configuration validation failed:',
      'too many columns'
    )
    expect(consoleErrorSpy).toHaveBeenCalledWith('Grid performance warning:', 'layout is dense')
    expect(consoleInfoSpy).toHaveBeenCalledWith('Grid performance recommendation:', 'reduce item count')
  })
})
