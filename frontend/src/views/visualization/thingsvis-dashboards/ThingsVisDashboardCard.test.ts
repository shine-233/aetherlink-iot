/**
 * 文件用途：验证 ThingsVisDashboardCard 的真实卡片链接、缩略图和操作事件。
 * 核心逻辑：直接挂载生产卡片组件，使用最小 UI stub 触发可见按钮和确认动作。
 * 关键注意事项：预览 href 必须来自生产组件，不得由列表页的卡片 stub 自行拼装。
 * 重构建议：若卡片操作继续增加，按 data-testid 拆分动作合同并保留真实 href 校验。
 */
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import ThingsVisDashboardCard from './ThingsVisDashboardCard.vue'

const NButtonStub = defineComponent({
  name: 'NButton',
  inheritAttrs: false,
  props: {
    disabled: Boolean,
    loading: Boolean,
    type: { type: String, default: 'default' },
    size: { type: String, default: 'medium' },
    secondary: Boolean
  },
  emits: ['click'],
  setup(props, { attrs, emit, slots }) {
    return () =>
      h(
        'button',
        {
          ...attrs,
          type: 'button',
          disabled: props.disabled,
          'data-loading': String(props.loading),
          onClick: (event: MouseEvent) => emit('click', event)
        },
        [slots.icon?.(), slots.default?.()]
      )
  }
})

const NPopconfirmStub = defineComponent({
  name: 'NPopconfirm',
  emits: ['positive-click'],
  setup(_, { emit, slots }) {
    return () =>
      h('div', { class: 'n-popconfirm-stub' }, [
        h('div', { class: 'n-popconfirm-trigger', onClick: () => emit('positive-click') }, slots.trigger?.()),
        slots.default?.()
      ])
  }
})

const NTooltipStub = defineComponent({
  name: 'NTooltip',
  setup(_, { slots }) {
    return () => h('div', { class: 'n-tooltip-stub' }, [slots.trigger?.(), slots.default?.()])
  }
})

const NTagStub = defineComponent({
  name: 'NTag',
  setup(_, { slots }) {
    return () => h('span', { class: 'n-tag-stub' }, slots.default?.())
  }
})

const dashboard = {
  id: 'dashboard with spaces',
  name: 'RDI Dashboard',
  thumbnail: null,
  version: 3,
  published: false,
  home: false,
  projectId: 'project-1',
  createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-30T00:00:00Z'
}

const mountCard = (dashboardOverride: Record<string, unknown> = {}) =>
  mount(ThingsVisDashboardCard, {
    props: {
      dashboard: { ...dashboard, ...dashboardOverride },
      thumbnailUrl: 'data:image/png;base64,thumb',
      menuConfigLoaded: true
    },
    global: {
      stubs: {
        NButton: NButtonStub,
        'n-button': NButtonStub,
        NPopconfirm: NPopconfirmStub,
        'n-popconfirm': NPopconfirmStub,
        NTooltip: NTooltipStub,
        'n-tooltip': NTooltipStub,
        NTag: NTagStub,
        'n-tag': NTagStub,
        'icon-mdi:chart-box': true,
        'icon-mdi:tag-outline': true,
        'icon-mdi:clock-outline': true,
        'icon-mdi:pencil': true,
        'icon-mdi:cloud-upload-outline': true,
        'icon-mdi:content-copy': true,
        'icon-mdi:link-variant': true,
        'icon-mdi:menu': true,
        'icon-mdi:home-outline': true,
        'icon-mdi:home': true,
        'icon-mdi:delete': true
      }
    }
  })

describe('ThingsVisDashboardCard.vue', () => {
  const wrappers: Array<ReturnType<typeof mountCard>> = []

  afterEach(() => {
    while (wrappers.length) wrappers.pop()?.unmount()
  })

  it('renders the real standalone preview href from the dashboard id', () => {
    const wrapper = mountCard()
    wrappers.push(wrapper)

    const link = wrapper.get<HTMLAnchorElement>('[data-testid="thingsvis-dashboard-card"] a')
    expect(link.attributes('href')).toBe('/tv-preview?id=dashboard%20with%20spaces')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
    expect(wrapper.get('img').attributes('src')).toBe('data:image/png;base64,thumb')
    expect(wrapper.get('[data-testid="thingsvis-dashboard-card"]').attributes('data-dashboard-id')).toBe(
      dashboard.id
    )
  })

  it('adds the local provider and project to native board preview links', () => {
    const wrapper = mountCard({ projectId: 'native-boards' })
    wrappers.push(wrapper)

    expect(wrapper.get('[data-testid="thingsvis-dashboard-card"] a').attributes('href')).toBe(
      '/tv-preview?id=dashboard%20with%20spaces&projectId=native-boards&provider=native'
    )
  })

  it('emits each dashboard action from the rendered controls', async () => {
    const wrapper = mountCard()
    wrappers.push(wrapper)

    await wrapper.get('[data-testid="thingsvis-dashboard-edit"]').trigger('click')
    await wrapper.get('[data-testid="thingsvis-dashboard-publish"]').trigger('click')
    await wrapper.get('[data-testid="thingsvis-dashboard-duplicate"]').trigger('click')
    await wrapper.get('[data-testid="thingsvis-dashboard-copy-link"]').trigger('click')
    await wrapper.get('[data-testid="thingsvis-dashboard-menu"]').trigger('click')
    await wrapper.get('[data-testid="thingsvis-dashboard-delete"]').trigger('click')
    const popconfirms = wrapper.findAllComponents({ name: 'Popconfirm' })
    expect(popconfirms).toHaveLength(1)
    await popconfirms[0].vm.$emit('positive-click', {
      stopPropagation: vi.fn()
    })

    expect(wrapper.emitted('edit')).toEqual([[dashboard.id]])
    expect(wrapper.emitted('publish')).toEqual([[dashboard]])
    expect(wrapper.emitted('duplicate')).toEqual([[dashboard]])
    expect(wrapper.emitted('copyLink')).toEqual([[dashboard]])
    expect(wrapper.emitted('menu')).toEqual([[dashboard]])
    expect(wrapper.emitted('delete')).toEqual([[dashboard.id, dashboard.name]])
    expect(wrapper.emitted('setHome')).toEqual([[dashboard]])
  })
})
