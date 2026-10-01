import { describe, expect, it } from 'vitest'
import { emptyNormalizedDryRun, isNormalizedDryRun, normalizeDryRunResponse } from '../automationDryRunNormalize'
import { buildAutomationDryRunCustomerView, buildBackendDryRunView } from '../automationDryRunViews'

/** 与 backend SceneAutomationDryRunResult 完全对齐的 snake_case 响应（引用校验失败场景） */
const snakeCasePayload = () => ({
  supported: true,
  valid: false,
  can_save: false,
  summary: 'static preview',
  dry_run: {
    condition_group_count: 1,
    condition_count: 2,
    action_count: 1,
    condition_types: { '10': 2 },
    action_types: { '30': 1 },
    target_kinds: { device: 2 }
  },
  reference_counts: { device: 1, alarm: 1 },
  warnings: ['time window not evaluated'],
  errors: ['scene automation reference validation failed: device missing'],
  blocking_errors: ['scene automation reference validation failed: device missing', 'add an action'],
  skipped_conditions: ['condition group #1 row #2'],
  unavailable_actions: [],
  matched_devices: 3,
  diagnostics: [
    { severity: 'warning', scope: 'warning', message: 'time window not evaluated' },
    { severity: 'error', scope: 'validation', message: 'scene automation reference validation failed: device missing' },
    { severity: 'error', scope: 'save', message: 'add an action' }
  ],
  next_steps: ['fix the device reference'],
  execution_trace: {
    steps: [{ index: 1, phase: 'trigger', status: 'blocked', label: 'row #1', notes: ['n', 7] }],
    step_count: 1,
    evaluated_at: '2026-09-30T00:00:00Z',
    explanation: 'ordered',
    is_simulation: true
  }
})

describe('normalizeDryRunResponse', () => {
  it('reads the snake_case backend contract as the primary source', () => {
    const dryRun = normalizeDryRunResponse(snakeCasePayload())

    expect(dryRun.present).toBe(true)
    expect(dryRun.supported).toBe(true)
    expect(dryRun.valid).toBe(false)
    expect(dryRun.canSave).toBe(false)
    expect(dryRun.stats).toEqual({
      conditionGroupCount: 1,
      conditionCount: 2,
      actionCount: 1,
      conditionTypes: { '10': 2 },
      actionTypes: { '30': 1 },
      targetKinds: { device: 2 }
    })
    expect(dryRun.referenceCounts).toEqual({ device: 1, alarm: 1 })
    expect(dryRun.skippedConditions).toEqual(['condition group #1 row #2'])
    expect(dryRun.unavailableActions).toEqual([])
    expect(dryRun.matchedDevices).toBe(3)
    expect(dryRun.nextSteps).toEqual(['fix the device reference'])
    expect(dryRun.trace).toEqual({
      steps: [
        {
          index: 1,
          phase: 'trigger',
          status: 'blocked',
          label: 'row #1',
          kind: '',
          target: '',
          detail: '',
          notes: ['n']
        }
      ],
      stepCount: 1,
      evaluatedAt: '2026-09-30T00:00:00Z',
      explanation: 'ordered',
      isSimulation: true
    })
  })

  it('dedupes the reference failure the backend writes to errors, blocking_errors and diagnostics', () => {
    const dryRun = normalizeDryRunResponse(snakeCasePayload())

    expect(dryRun.blockingErrors).toEqual([
      'scene automation reference validation failed: device missing',
      'add an action'
    ])
    expect(dryRun.warnings).toEqual(['time window not evaluated'])

    const view = buildAutomationDryRunCustomerView('available', snakeCasePayload(), '')
    expect(view.blockingErrors.map((line) => line.text)).toEqual([
      'scene automation reference validation failed: device missing',
      'add an action'
    ])
    expect(view.status).toBe('risk')
  })

  it('keeps legacy camelCase and `blockers` payloads working through the single fallback point', () => {
    const dryRun = normalizeDryRunResponse({
      canSave: true,
      dryRun: { condition_group_count: 2, action_count: 4, target_kinds: { scene: 1 } },
      referenceCounts: { scene: 1 },
      blockers: ['legacy blocker'],
      skippedConditions: ['legacy skipped'],
      unavailableActions: ['legacy unavailable'],
      matchedDevices: 0,
      nextSteps: ['legacy next'],
      executionTrace: { evaluatedAt: 'legacy-time', steps: [{ label: 'loose' }] }
    })

    expect(dryRun.canSave).toBe(true)
    expect(dryRun.stats.conditionGroupCount).toBe(2)
    expect(dryRun.stats.actionCount).toBe(4)
    expect(dryRun.stats.conditionCount).toBe(0)
    expect(dryRun.referenceCounts).toEqual({ scene: 1 })
    expect(dryRun.blockingErrors).toEqual(['legacy blocker'])
    expect(dryRun.skippedConditions).toEqual(['legacy skipped'])
    expect(dryRun.unavailableActions).toEqual(['legacy unavailable'])
    expect(dryRun.matchedDevices).toBe(0)
    expect(dryRun.nextSteps).toEqual(['legacy next'])
    expect(dryRun.trace?.evaluatedAt).toBe('legacy-time')
    expect(dryRun.trace?.steps[0]).toMatchObject({ index: null, label: 'loose', status: '' })
  })

  it('prefers snake_case over camelCase when both are present', () => {
    const dryRun = normalizeDryRunResponse({
      can_save: false,
      canSave: true,
      matched_devices: 1,
      matchedDevices: 9,
      dry_run: { action_count: 1 },
      dryRun: { action_count: 9 },
      next_steps: ['snake'],
      nextSteps: ['camel']
    })

    expect(dryRun.canSave).toBe(false)
    expect(dryRun.matchedDevices).toBe(1)
    expect(dryRun.stats.actionCount).toBe(1)
    expect(dryRun.nextSteps).toEqual(['snake'])
  })

  it('falls back from can_save to valid, and to null when both are absent', () => {
    expect(normalizeDryRunResponse({ valid: false }).canSave).toBe(false)
    expect(normalizeDryRunResponse({ valid: true }).canSave).toBe(true)
    expect(normalizeDryRunResponse({}).canSave).toBeNull()
  })

  it('treats an explicit empty reference_counts as authoritative and only falls back when absent', () => {
    expect(
      normalizeDryRunResponse({ reference_counts: {}, dry_run: { target_kinds: { device: 1 } } }).referenceCounts
    ).toEqual({})
    expect(normalizeDryRunResponse({ dry_run: { target_kinds: { device: 1 } } }).referenceCounts).toEqual({
      device: 1
    })
  })

  it('normalizes null, undefined and non-object payloads to an empty, absent result', () => {
    for (const raw of [null, undefined, 'oops', 42, []]) {
      const dryRun = normalizeDryRunResponse(raw)
      expect(dryRun).toEqual(emptyNormalizedDryRun())
      expect(dryRun.present).toBe(false)
      expect(dryRun.trace).toBeNull()
    }

    expect(buildBackendDryRunView(null).metrics).toEqual([])
  })

  it('drops malformed collection members instead of throwing', () => {
    const dryRun = normalizeDryRunResponse({
      warnings: [null, '', 'real warning', { message: 'object warning' }],
      diagnostics: [null, 'bare string', { severity: 'mystery', message: 'unknown severity' }],
      dry_run: { condition_types: { '10': 'two', '11': 1 } },
      execution_trace: { steps: [null] }
    })

    expect(dryRun.rawWarnings).toEqual(['real warning', 'object warning'])
    expect(dryRun.diagnostics.map((item) => item.severity)).toEqual(['info', 'info'])
    expect(dryRun.blockingErrors).toEqual([])
    expect(dryRun.stats.conditionTypes).toEqual({ '11': 1 })
    expect(dryRun.trace?.steps).toHaveLength(1)
  })

  it('is idempotent and its brand stays out of serialization', () => {
    const once = normalizeDryRunResponse(snakeCasePayload())
    const twice = normalizeDryRunResponse(once)

    expect(twice).toBe(once)
    expect(isNormalizedDryRun(once)).toBe(true)
    expect(isNormalizedDryRun(snakeCasePayload())).toBe(false)
    expect(Object.keys(once)).not.toContain('__aetherlinkDryRunNormalized')
    expect(JSON.stringify(once)).not.toContain('__aetherlinkDryRunNormalized')
  })

  it('lets every builder accept the normalized result directly', () => {
    const dryRun = normalizeDryRunResponse(snakeCasePayload())

    expect(buildBackendDryRunView(dryRun)).toEqual(buildBackendDryRunView(snakeCasePayload()))
    expect(buildAutomationDryRunCustomerView('available', dryRun, '')).toEqual(
      buildAutomationDryRunCustomerView('available', snakeCasePayload(), '')
    )
  })
})
