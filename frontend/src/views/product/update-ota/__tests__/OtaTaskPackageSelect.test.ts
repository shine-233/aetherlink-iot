/**
 * 文件用途: 覆盖 OtaTaskPackageSelect 升级包选择卡片的展示与回抛契约。
 * 核心逻辑: 用可断言的 NSelect/NTag stub 挂载,验证下拉状态透传、选中/搜索事件回抛与标签显隐。
 * 关键注意事项: 该组件是纯展示组件,不得在内部取数,断言只针对 props 透传与 emit 载荷。
 * 重构建议: 后续若加入签名校验等本地逻辑,需同步补失败分支断言。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import OtaTaskPackageSelect from '../OtaTaskPackageSelect.vue'

const nSelectStub = defineComponent({
  name: 'NSelectStub',
  props: {
    value: { default: null },
    loading: Boolean,
    options: { default: () => [] },
    placeholder: { type: String, default: '' },
    filterable: Boolean,
    remote: Boolean,
    clearable: Boolean
  },
  emits: ['update:value', 'search'],
  setup(props, { emit }) {
    return () =>
      h('div', {
        'data-value': String(props.value),
        'data-loading': String(props.loading),
        'data-remote': String(props.remote),
        'data-filterable': String(props.filterable),
        'data-clearable': String(props.clearable),
        'data-placeholder': props.placeholder
      })
  }
})

const passthrough = (name: string) =>
  defineComponent({
    name,
    setup(_, { slots }) {
      return () => h('div', slots.default?.())
    }
  })

const tagStub = defineComponent({
  name: 'NTagStub',
  props: { type: { type: String, default: 'default' } },
  setup(props, { slots }) {
    return () => h('span', { 'data-type': props.type }, slots.default?.())
  }
})

const globalStubs = {
  NSelect: nSelectStub,
  NTag: tagStub,
  NCard: passthrough('NCardStub'),
  NSpace: passthrough('NSpaceStub')
}

const mountCard = (props: Record<string, unknown>) =>
  shallowMount(OtaTaskPackageSelect, {
    props: {
      selectedPackageId: null,
      packageOptions: [],
      loading: false,
      selectedPackage: null,
      ...props
    },
    global: { stubs: globalStubs }
  })

const findSelect = (wrapper: ReturnType<typeof mountCard>) => wrapper.findComponent(nSelectStub)

describe('OtaTaskPackageSelect', () => {
  it('passes selection state down to the remote-searchable select', () => {
    const wrapper = mountCard({
      selectedPackageId: 'pkg-1',
      loading: true,
      packageOptions: [{ label: 'Pkg1 (1.0)', value: 'pkg-1' }]
    })
    const select = findSelect(wrapper)

    expect(select.props('value')).toBe('pkg-1')
    expect(select.props('loading')).toBe(true)
    expect(select.props('options')).toEqual([{ label: 'Pkg1 (1.0)', value: 'pkg-1' }])
    expect(select.props('remote')).toBe(true)
    expect(select.props('filterable')).toBe(true)
    expect(select.props('clearable')).toBe(true)
    expect(select.props('placeholder')).toBe('page.product.update-package.packagePlaceholder')
  })

  it('re-emits the selected package id (including clear to null)', async () => {
    const wrapper = mountCard({ selectedPackageId: 'pkg-1' })

    await findSelect(wrapper).vm.$emit('update:value', 'pkg-2')
    await findSelect(wrapper).vm.$emit('update:value', null)

    expect(wrapper.emitted('update:selectedPackageId')).toEqual([['pkg-2'], [null]])
  })

  it('re-emits the remote search keyword as a plain string', async () => {
    const wrapper = mountCard({ selectedPackageId: 'pkg-1' })

    await findSelect(wrapper).vm.$emit('search', 'pump')

    expect(wrapper.emitted('search')).toEqual([['pump']])
  })

  it('shows the device config name and signature tags only when present', () => {
    const wrapper = mountCard({
      selectedPackageId: 'pkg-1',
      selectedPackage: { id: 'pkg-1', device_config_name: 'Default Config', signature: 'abc123' }
    })
    const tags = wrapper.findAll('[data-type]')

    expect(tags.map(tag => tag.attributes('data-type'))).toEqual(['info', 'success'])
    expect(tags[0].text()).toBe('Default Config')
    expect(tags[1].text()).toBe('page.product.update-ota.packageSign: abc123')
  })

  it('renders no package tags without a selected package', () => {
    const wrapper = mountCard({})

    expect(wrapper.findAll('[data-type]')).toHaveLength(0)
  })
})
