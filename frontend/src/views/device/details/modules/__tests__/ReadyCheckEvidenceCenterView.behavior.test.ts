import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import ReadyCheckEvidenceCenterView from '../ReadyCheckEvidenceCenterView.vue'

const ButtonStub = defineComponent({
  name: 'NButton',
  props: { loading: Boolean },
  emits: ['click'],
  setup(props, { emit, slots }) {
    return () =>
      h(
        'button',
        {
          'data-loading': String(props.loading),
          onClick: () => emit('click')
        },
        slots.default?.()
      )
  }
})

const TagStub = defineComponent({
  name: 'NTag',
  setup(_, { slots }) {
    return () => h('span', { class: 'tag' }, slots.default?.())
  }
})

const EvidenceLinksStub = defineComponent({
  name: 'ReadyCheckEvidenceLinksView',
  props: ['links'],
  emits: ['open', 'copy', 'copy-all'],
  setup(props, { emit }) {
    return () =>
      h('div', { 'data-testid': 'evidence-links' }, [
        h('span', props.links.map((link: any) => link.label).join(', ')),
        h('button', { 'data-testid': 'open-link', onClick: () => emit('open', props.links[0]) }, 'open'),
        h('button', { 'data-testid': 'copy-link', onClick: () => emit('copy', props.links[0]) }, 'copy'),
        h('button', { 'data-testid': 'copy-all-links', onClick: () => emit('copy-all') }, 'copy all')
      ])
  }
})

const EvidenceCardsStub = defineComponent({
  name: 'ReadyCheckEvidenceCardsView',
  props: ['evidenceCards'],
  emits: ['run'],
  setup(props, { emit }) {
    return () =>
      h('div', { 'data-testid': 'evidence-cards' }, [
        h('span', props.evidenceCards.map((card: any) => card.title).join(', ')),
        h('button', { 'data-testid': 'run-card', onClick: () => emit('run', props.evidenceCards[0]) }, 'run')
      ])
  }
})

const deepLink = {
  key: 'telemetry-history',
  label: 'Telemetry history',
  description: 'Inspect persisted telemetry',
  route: { path: '/device/details', query: { tab: 'telemetry' } }
}

const evidenceCard = {
  key: 'mqtt-ingress',
  title: 'MQTT ingress',
  description: 'Verify a real packet reached the platform',
  status: 'ready'
}

const baseProps = {
  loading: false,
  readySummary: 'Ready: identity and telemetry verified',
  latestTelemetryText: 'temperature = 23 C',
  nextActions: ['Send an attribute', 'Review alarm state'],
  evidenceCenterItems: [
    {
      key: 'device-identity',
      labelKey: 'custom.device_details.readyCheckIdentity',
      value: 'pump-007',
      detail: 'Bound to Pump profile'
    }
  ],
  evidenceCards: [evidenceCard],
  backendNextSteps: [
    {
      key: 'persisted-history',
      status: 'pending',
      title: 'Confirm persisted history',
      description: 'Query the backend history endpoint after a real packet.'
    }
  ],
  deepLinks: [deepLink]
}

const mountedWrappers: Array<ReturnType<typeof mount>> = []

function mountEvidenceCenter(props: Partial<typeof baseProps> = {}) {
  const wrapper = mount(ReadyCheckEvidenceCenterView, {
    props: { ...baseProps, ...props } as any,
    global: {
      stubs: {
        NButton: ButtonStub,
        NTag: TagStub,
        ReadyCheckEvidenceLinksView: EvidenceLinksStub,
        ReadyCheckEvidenceCardsView: EvidenceCardsStub
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('ReadyCheckEvidenceCenterView business evidence', () => {
  afterEach(() => {
    while (mountedWrappers.length) mountedWrappers.pop()?.unmount()
  })

  it('renders the operator summary, evidence values, and backend follow-up work', () => {
    const wrapper = mountEvidenceCenter()
    const diagnostics = wrapper.get('[data-testid="device-ready-check-diagnostics"]')
    const evidenceCenter = wrapper.get('[data-testid="device-ready-check-evidence-center"]')
    const backendSteps = wrapper.get('[data-testid="device-ready-check-backend-steps"]')

    expect(diagnostics.text()).toContain('Ready: identity and telemetry verified')
    expect(diagnostics.text()).toContain('temperature = 23 C')
    expect(diagnostics.text()).toContain('Send an attribute; Review alarm state')
    expect(evidenceCenter.text()).toContain('custom.device_details.readyCheckIdentity')
    expect(evidenceCenter.text()).toContain('pump-007')
    expect(evidenceCenter.text()).toContain('Bound to Pump profile')
    expect(backendSteps.text()).toContain('pending')
    expect(backendSteps.text()).toContain('Confirm persisted history')
    expect(backendSteps.text()).toContain('Query the backend history endpoint after a real packet.')
    expect(wrapper.get('[data-testid="evidence-links"]').text()).toContain('Telemetry history')
    expect(wrapper.get('[data-testid="evidence-cards"]').text()).toContain('MQTT ingress')
  })

  it('shows loading and unknown-action states without inventing backend evidence', async () => {
    const wrapper = mountEvidenceCenter()

    await wrapper.setProps({
      loading: true,
      nextActions: [],
      evidenceCenterItems: [],
      backendNextSteps: []
    })

    const diagnostics = wrapper.get('[data-testid="device-ready-check-diagnostics"]')
    expect(diagnostics.text()).toContain('common.loading')
    expect(diagnostics.text()).toContain('custom.device_details.accessGuideDiagnosticUnknown')
    expect(wrapper.get('[data-testid="device-ready-check-refresh"]').attributes('data-loading')).toBe('true')
    expect(wrapper.findAll('[data-testid="device-ready-check-backend-steps"]')).toHaveLength(0)
  })

  it('forwards operator actions with the exact selected evidence objects', async () => {
    const wrapper = mountEvidenceCenter()

    await wrapper.get('[data-testid="device-ready-check-refresh"]').trigger('click')
    await wrapper.get('[data-testid="device-ready-check-support-bundle"]').trigger('click')
    await wrapper.get('[data-testid="device-ready-check-download-support-bundle"]').trigger('click')
    await wrapper.get('[data-testid="open-link"]').trigger('click')
    await wrapper.get('[data-testid="copy-link"]').trigger('click')
    await wrapper.get('[data-testid="copy-all-links"]').trigger('click')
    await wrapper.get('[data-testid="run-card"]').trigger('click')

    expect(wrapper.emitted('refresh')).toEqual([[]])
    expect(wrapper.emitted('copySupportBundle')).toEqual([[]])
    expect(wrapper.emitted('downloadSupportBundle')).toEqual([[]])
    expect(wrapper.emitted('openDeepLink')).toEqual([[deepLink]])
    expect(wrapper.emitted('copyDeepLink')).toEqual([[deepLink]])
    expect(wrapper.emitted('copyAllDeepLinks')).toEqual([[]])
    expect(wrapper.emitted('runEvidenceCard')).toEqual([[evidenceCard]])
  })
})
