import { describe, expect, it } from 'vitest'
import { buildBackendDryRunView, buildTraceView } from '../automationDryRunPreview'

describe('buildTraceView', () => {
  it('returns an empty simulation trace when the response has no execution_trace', () => {
    const trace = buildTraceView({ valid: true })

    expect(trace.steps).toEqual([])
    expect(trace.stepCount).toBe(0)
    expect(trace.isSimulation).toBe(true)
  })

  it('maps ordered backend steps and derives a status type per step', () => {
    const trace = buildTraceView({
      execution_trace: {
        step_count: 3,
        evaluated_at: '2026-07-28T00:00:00Z',
        explanation: 'ordered static preview',
        is_simulation: true,
        steps: [
          {
            index: 1,
            phase: 'trigger',
            kind: 'single_device',
            target: 'device',
            label: 'condition group #1 row #1',
            status: 'evaluated',
            detail: 'references device "dev-1"',
            notes: []
          },
          {
            index: 2,
            phase: 'trigger',
            kind: 'time_range',
            target: '',
            label: 'condition group #1 row #2',
            status: 'skipped',
            detail: 'time window not evaluated',
            notes: ['condition group #1 row #2 时间窗口不判断']
          },
          {
            index: 3,
            phase: 'action',
            kind: 'single_device',
            target: 'device',
            label: 'action #1',
            status: 'blocked',
            detail: 'missing target',
            notes: ['action #1 has no target']
          }
        ]
      }
    })

    expect(trace.stepCount).toBe(3)
    expect(trace.evaluatedAt).toBe('2026-07-28T00:00:00Z')
    expect(trace.explanation).toBe('ordered static preview')
    expect(trace.steps.map(step => step.statusType)).toEqual(['success', 'warning', 'error'])
    expect(trace.steps.map(step => step.phase)).toEqual(['trigger', 'trigger', 'action'])
    expect(trace.steps[1].notes).toEqual(['condition group #1 row #2 时间窗口不判断'])
    expect(trace.steps[0].key).toBe('trace-step-1')
  })

  it('tolerates missing fields and non-string notes without throwing', () => {
    const trace = buildTraceView({
      executionTrace: {
        steps: [{ label: 'loose step', notes: ['ok', 42, null] }]
      }
    })

    expect(trace.steps).toHaveLength(1)
    expect(trace.steps[0].index).toBe(1)
    expect(trace.steps[0].phase).toBe('trigger')
    expect(trace.steps[0].statusType).toBe('info')
    expect(trace.steps[0].notes).toEqual(['ok'])
  })

  it('is included in the full backend dry-run view', () => {
    const view = buildBackendDryRunView({
      valid: true,
      dry_run: { condition_group_count: 1, condition_count: 1, action_count: 1 },
      execution_trace: {
        steps: [{ index: 1, phase: 'action', status: 'evaluated', label: 'action #1' }]
      }
    })

    expect(view.trace.steps).toHaveLength(1)
    expect(view.trace.steps[0].statusType).toBe('success')
  })

  it('provides an empty trace on the empty backend view branch', () => {
    const view = buildBackendDryRunView(null)

    expect(view.trace.steps).toEqual([])
    expect(view.trace.isSimulation).toBe(true)
  })
})
