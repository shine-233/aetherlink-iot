import { describe, expect, it, vi } from 'vitest'
import { useAutomationExecutionPreview } from '../useAutomationExecutionPreview'

const payload = {
  id: 'rule-1',
  name: 'High temperature rule',
  description: 'Warn when temperature is high',
  enabled: 'Y',
  trigger_condition_groups: [
    [
      {
        trigger_conditions_type: '10',
        trigger_source: 'device-1',
        trigger_param_type: 'telemetry',
        trigger_param: 'temperature',
        trigger_operator: '>',
        trigger_value: 80
      }
    ]
  ],
  actions: [
    {
      action_type: '30',
      action_target: 'alarm-1'
    }
  ]
}

describe('useAutomationExecutionPreview', () => {
  it('generates local explanation without claiming backend execution', () => {
    const preview = useAutomationExecutionPreview({
      buildPayload: () => payload,
      dryRun: vi.fn()
    })

    expect(preview.localPreviewStatusText.value).toContain('No local explanation')
    expect(preview.refreshLocalExecutionExplanation()).toEqual(payload)
    expect(preview.localPreviewStatusText.value).toContain('not a backend execution result')
    expect(preview.previewConditionCount.value).toBe(1)
    expect(preview.previewActionCount.value).toBe(1)
    expect(preview.beginnerGuideCards.value.map((item) => item.key)).toEqual(['save', 'match', 'skipped', 'actions'])
    expect(preview.backendDryRunStatusText.value).toContain('尚未运行')
  })

  it('records available backend dry-run responses separately from local explanation', async () => {
    const preview = useAutomationExecutionPreview({
      buildPayload: () => payload,
      dryRun: vi.fn().mockResolvedValue({ data: { matched: false } })
    })

    await preview.runBackendDryRun()

    expect(preview.backendDryRunStatus.value).toBe('available')
    expect(preview.beginnerGuideCards.value[1]).toMatchObject({
      key: 'match',
      textKey: 'generate.automationDryRunBeginnerMatchNotEvaluated'
    })
    expect(preview.dryRunResponseText.value).toContain('"matched": false')
  })

  it('downgrades missing backend dry-run to unavailable instead of faking a result', async () => {
    const preview = useAutomationExecutionPreview({
      buildPayload: () => payload,
      dryRun: vi.fn().mockResolvedValue({ error: { message: 'not implemented' } })
    })

    await preview.runBackendDryRun()

    expect(preview.backendDryRunStatus.value).toBe('unavailable')
    expect(preview.backendDryRunError.value).toBe('not implemented')
    expect(preview.dryRunResponseText.value).toBe('')
  })
})
