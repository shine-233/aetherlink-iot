import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import {
  buildActionSummaryItems,
  buildAutomationDryRunBeginnerGuide,
  buildAutomationDryRunCustomerView,
  buildAutomationOperatorPlan,
  buildConditionSummaryItems,
  getAutomationDryRunAlertType,
  getAutomationDryRunStatusText,
  getPreviewErrorText,
  stringifyDryRunResponse
} from '../automationDryRunPreview'
import AutomationDryRunPreview from '../AutomationDryRunPreview.vue'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

const mountDryRunPreview = (quickFixActions: any[]) =>
  shallowMount(AutomationDryRunPreview, {
    props: {
      localStatusText: 'local ready',
      backendStatusText: 'backend waiting',
      backendAlertType: 'info',
      backendError: '',
      conditionGroupCount: 0,
      conditionCount: 0,
      actionCount: 0,
      conditionSummaryItems: [],
      actionSummaryItems: [],
      operatorPlan: {
        source: [],
        conditions: [],
        actions: [],
        limits: []
      },
      backendDryRunView: {
        metrics: [],
        conditionTypes: [],
        actionTypes: [],
        targetKinds: [],
        diagnostics: [],
        nextSteps: []
      },
      customerDryRunView: {
        status: 'unchecked',
        tagType: 'default',
        alertType: 'info',
        blockingErrors: [],
        warnings: [],
        referenceCounts: [],
        nextSteps: [],
        responseAvailable: false,
        canSave: null
      },
      beginnerGuideCards: [],
      quickFixActions,
      localBlockingErrors: [],
      dryRunResponseText: '',
      isBackendDryRunLoading: false
    },
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NAlert: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        NEmpty: defineComponent({ setup() { return () => h('div') } }),
        NButton: defineComponent({
          props: { disabled: { type: Boolean, default: false } },
          emits: ['click'],
          setup(props, { slots, emit }) {
            return () =>
              h(
                'button',
                {
                  disabled: props.disabled,
                  onClick: () => {
                    if (!props.disabled) emit('click')
                  }
                },
                slots.default?.()
              )
          }
        })
      }
    }
  })

describe('automationDryRunPreview', () => {
  it('emits quick-fix keys only for enabled quick-fix buttons', async () => {
    const wrapper = mountDryRunPreview([
      {
        key: 'apply-first-alarm-action',
        title: 'Apply action',
        desc: 'Add an alarm action slot',
        buttonLabel: 'Apply',
        type: 'primary'
      },
      {
        key: 'create-first-alarm-target',
        title: 'Create target',
        desc: 'Create alarm target',
        buttonLabel: 'Create',
        disabled: true
      }
    ])

    const buttons = wrapper.findAll('button')
    await buttons[0].trigger('click')
    await buttons[1].trigger('click')

    expect(wrapper.text()).toContain('generate.automationDryRunQuickFixTitle')
    expect(wrapper.emitted('quickFix')).toEqual([['apply-first-alarm-action']])
  })

  it('keeps backend dry-run states honest', () => {
    expect(getAutomationDryRunStatusText('waiting')).toContain('尚未请求后端预演')
    expect(getAutomationDryRunStatusText('ready')).toContain('尚未运行')
    expect(getAutomationDryRunStatusText('pending')).toContain('正在请求')
    expect(getAutomationDryRunStatusText('available')).toContain('返回结果')
    expect(getAutomationDryRunStatusText('unavailable')).toContain('仅显示本地说明')

    expect(getAutomationDryRunAlertType('available')).toBe('success')
    expect(getAutomationDryRunAlertType('unavailable')).toBe('warning')
    expect(getAutomationDryRunAlertType('ready')).toBe('info')
  })

  it('summarizes conditions and actions from submit payload fields', () => {
    expect(
      buildConditionSummaryItems([
        [
          {
            trigger_conditions_type: '10',
            trigger_source: 'device-1',
            trigger_param_type: 'telemetry',
            trigger_param: 'temperature',
            trigger_operator: '>',
            trigger_value: 80
          }
        ],
        [
          {
            trigger_conditions_type: '21',
            task_type: 'DAY',
            params: '08:00:00Z'
          }
        ]
      ])
    ).toEqual([
      {
        key: 'condition-group-0',
        lines: [
          {
            key: 'condition-0-0',
            text: 'Single-device condition: device-1 / telemetry:temperature > 80'
          }
        ]
      },
      {
        key: 'condition-group-1',
        lines: [
          {
            key: 'condition-1-0',
            text: 'Recurring schedule: DAY / 08:00:00Z'
          }
        ]
      }
    ])

    expect(
      buildActionSummaryItems([
        {
          action_type: '10',
          action_target: 'device-1',
          action_param_type: 'command',
          action_param: 'reboot',
          action_value: '{"method":"reboot","params":{}}'
        },
        {
          action_type: '30',
          action_target: 'alarm-1'
        }
      ])
    ).toEqual([
      {
        key: 'action-0',
        text: 'Single-device action: device-1 / command:reboot = {"method":"reboot","params":{}}'
      },
      {
        key: 'action-1',
        text: 'Trigger alarm: alarm-1'
      }
    ])
  })

  it('formats dry-run response and errors without implying success', () => {
    expect(stringifyDryRunResponse(null)).toBe('')
    expect(stringifyDryRunResponse({ ok: true })).toBe('{\n  "ok": true\n}')
    expect(getPreviewErrorText({ response: { data: { message: 'missing endpoint' } } })).toBe('missing endpoint')
    expect(getPreviewErrorText({})).toBe('后端预演暂不可用。')
  })

  it('builds customer-facing dry-run closure without claiming execution trace', () => {
    expect(buildAutomationDryRunCustomerView('ready', null, '')).toMatchObject({
      status: 'unchecked',
      responseAvailable: false,
      blockingErrors: [],
      warnings: []
    })

    expect(
      buildAutomationDryRunCustomerView(
        'available',
        {
          valid: true,
          dry_run: { target_kinds: { device: 1, alarm: 1 } },
          next_steps: ['save only after reviewing the preview']
        },
        ''
      )
    ).toMatchObject({
      status: 'passed',
      referenceCounts: [
        { key: 'reference-device', text: 'device: 1' },
        { key: 'reference-alarm', text: 'alarm: 1' }
      ],
      nextSteps: [{ key: 'next-step-0', text: 'save only after reviewing the preview' }]
    })

    expect(
      buildAutomationDryRunCustomerView(
        'available',
        {
          valid: false,
          blocking_errors: ['device is inaccessible'],
          warnings: ['preview does not evaluate live telemetry']
        },
        ''
      )
    ).toMatchObject({
      status: 'risk',
      blockingErrors: [{ key: 'blocking-error-0', text: 'device is inaccessible' }],
      warnings: [{ key: 'warning-0', text: 'preview does not evaluate live telemetry' }]
    })
  })

  it('builds a beginner guide that separates save blockers from live device match proof', () => {
    const customerView = buildAutomationDryRunCustomerView(
      'available',
      {
        valid: true,
        can_save: true,
        warnings: ['dry-run validates references only'],
        skipped_conditions: [
          'condition group #1 row #1 live telemetry value and online-state matching are not evaluated by dry-run'
        ],
        unavailable_actions: ['action #1 has no command parameter']
      },
      ''
    )

    const cards = buildAutomationDryRunBeginnerGuide({
      status: 'available',
      response: {
        valid: true,
        can_save: true,
        warnings: ['dry-run validates references only'],
        skipped_conditions: [
          'condition group #1 row #1 live telemetry value and online-state matching are not evaluated by dry-run'
        ],
        unavailable_actions: ['action #1 has no command parameter']
      },
      backendError: '',
      customerView,
      localBlockingErrors: [],
      conditionGroupCount: 1,
      conditionCount: 1,
      actionCount: 1
    })

    expect(cards[0]).toMatchObject({
      key: 'save',
      type: 'warning',
      titleKey: 'generate.automationDryRunBeginnerSaveTitle'
    })
    expect(cards[1]).toMatchObject({
      key: 'match',
      type: 'warning',
      textKey: 'generate.automationDryRunBeginnerMatchNotEvaluated'
    })
    expect(cards[2]).toMatchObject({
      key: 'skipped',
      type: 'warning',
      detail: 'condition group #1 row #1 live telemetry value and online-state matching are not evaluated by dry-run'
    })
    expect(cards[3]).toMatchObject({
      key: 'actions',
      type: 'warning',
      textKey: 'generate.automationDryRunBeginnerActionUnavailable',
      detail: 'action #1 has no command parameter'
    })
  })

  it('uses structured matched device counts when backend dry-run can prove them', () => {
    const customerView = buildAutomationDryRunCustomerView(
      'available',
      {
        valid: true,
        can_save: true,
        matched_devices: 2
      },
      ''
    )

    const matchedCards = buildAutomationDryRunBeginnerGuide({
      status: 'available',
      response: {
        valid: true,
        can_save: true,
        matched_devices: 2
      },
      backendError: '',
      customerView,
      localBlockingErrors: [],
      conditionGroupCount: 1,
      conditionCount: 1,
      actionCount: 1
    })

    expect(matchedCards[1]).toMatchObject({
      key: 'match',
      type: 'success',
      textKey: 'generate.automationDryRunBeginnerMatchKnown',
      detail: '2'
    })

    const zeroCards = buildAutomationDryRunBeginnerGuide({
      status: 'available',
      response: {
        valid: true,
        can_save: true,
        matchedDevices: 0,
        skippedConditions: ['condition group #1 row #1 is outside the dry-run evaluator'],
        unavailableActions: ['action #1 cannot be previewed']
      },
      backendError: '',
      customerView,
      localBlockingErrors: [],
      conditionGroupCount: 1,
      conditionCount: 1,
      actionCount: 1
    })

    expect(zeroCards[1]).toMatchObject({
      key: 'match',
      type: 'warning',
      textKey: 'generate.automationDryRunBeginnerMatchKnown',
      detail: '0'
    })
    expect(zeroCards[2]).toMatchObject({
      key: 'skipped',
      detail: 'condition group #1 row #1 is outside the dry-run evaluator'
    })
    expect(zeroCards[3]).toMatchObject({
      key: 'actions',
      detail: 'action #1 cannot be previewed'
    })
  })

  it('puts the first blocker in the beginner save card', () => {
    const customerView = buildAutomationDryRunCustomerView(
      'available',
      {
        valid: false,
        can_save: false,
        blocking_errors: ['add at least one action before saving']
      },
      ''
    )

    expect(
      buildAutomationDryRunBeginnerGuide({
        status: 'available',
        response: { valid: false, can_save: false },
        backendError: '',
        customerView,
        localBlockingErrors: [],
        conditionGroupCount: 1,
        conditionCount: 1,
        actionCount: 0
      })[0]
    ).toMatchObject({
      key: 'save',
      type: 'error',
      textKey: 'generate.automationDryRunBeginnerSaveBlocked',
      detail: 'add at least one action before saving'
    })
  })

  it('builds an operator execution plan that states scope and evidence limits', () => {
    const plan = buildAutomationOperatorPlan(
      {
        name: 'High temperature alarm',
        enabled: true,
        trigger_condition_groups: [
          [
            {
              trigger_conditions_type: '10',
              trigger_source: 'device-1'
            }
          ]
        ],
        actions: [
          {
            action_type: '30',
            action_target: 'alarm-1'
          }
        ]
      },
      'available',
      {
        can_save: false,
        dry_run: {
          condition_types: { '10': 1 },
          action_types: { '30': 1 },
          target_kinds: { alarm: 1 }
        }
      },
      ''
    )

    expect(plan.source.map((item) => item.text)).toContain('规则：High temperature alarm')
    expect(plan.conditions.map((item) => item.text)).toContain('载荷包含 1 个条件组和 1 条条件。')
    expect(plan.actions.map((item) => item.text)).toContain('如果规则触发，保存后的定义会包含 1 条已配置动作。')
    expect(plan.actions).toContainEqual({ key: 'operator-reference-alarm', text: 'alarm: 1' })
    const limitTexts = plan.limits.map((item) => item.text)
    expect(limitTexts.some((text) => text.includes('暂不适合保存'))).toBe(true)
    expect(limitTexts.some((text) => text.includes('预演不会保存规则') && text.includes('实时设备遥测'))).toBe(true)
  })
})
