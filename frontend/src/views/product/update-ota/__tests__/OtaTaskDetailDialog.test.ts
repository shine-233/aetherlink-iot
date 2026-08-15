import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import OtaTaskDetailDialog from '../OtaTaskDetailDialog.vue'

const passthrough = (tag: string, props: Record<string, unknown> = {}) =>
  defineComponent({
    props: Object.keys(props),
    setup(_, { slots }) {
      return () => h(tag, slots.default?.())
    }
  })

const button = defineComponent({
  inheritAttrs: false,
  props: {
    disabled: Boolean,
    loading: Boolean
  },
  emits: ['click'],
  setup(props, { attrs, slots, emit }) {
    return () =>
      h(
        'button',
        {
          ...attrs,
          disabled: props.disabled,
          onClick: () => !props.disabled && emit('click')
        },
        slots.default?.()
      )
  }
})

const mountDialog = (overrides: Partial<Record<string, unknown>> = {}) =>
  shallowMount(OtaTaskDetailDialog, {
    props: {
      show: true,
      readyCheckOtaDetailContextMessage: '',
      detailLastRefreshLabel: '2026-07-30 09:00:00',
      detailAutoRefreshActive: false,
      rolloutFailedCount: 0,
      rolloutSuccessRate: '100%',
      detailLoading: false,
      rolloutActiveCount: 0,
      detailAutoRefreshEnabled: false,
      rolloutSummaryItems: [],
      rolloutGuidanceItems: [],
      failedDeviceCount: 0,
      supportBundleLoading: false,
      canCopyFailureSupportBundle: false,
      hasFirstFailedDiagnosticDevice: false,
      retryRecommendationCards: [],
      failureGroups: [],
      detailQuery: {},
      statusOptions: [],
      detailColumns: [],
      detailList: [],
      detailPagination: {},
      ...overrides
    },
    global: {
      stubs: {
        NModal: passthrough('div', { show: Boolean }),
        NSpace: passthrough('div'),
        NAlert: passthrough('div'),
        NCard: passthrough('div'),
        NTag: passthrough('span'),
        NGrid: passthrough('div'),
        NGridItem: passthrough('div'),
        NInput: passthrough('input'),
        NSelect: passthrough('div'),
        NDataTable: passthrough('div'),
        NSwitch: passthrough('input'),
        NButton: button
      }
    }
  })

describe('OtaTaskDetailDialog', () => {
  it('renders the task support bundle action and emits its download event', async () => {
    const wrapper = mountDialog({ failedDeviceCount: 2 })
    const downloadButton = wrapper.get('[data-testid="ota-download-task-support-bundle"]')

    expect(downloadButton.text()).toMatch(/download/i)
    await downloadButton.trigger('click')

    expect(wrapper.emitted('downloadTaskSupportBundle')).toHaveLength(1)
  })

  it('keeps the support bundle action available while no failed rows are present', () => {
    const wrapper = mountDialog()

    expect(wrapper.get('[data-testid="ota-download-task-support-bundle"]').attributes('disabled')).toBeUndefined()
  })
})
