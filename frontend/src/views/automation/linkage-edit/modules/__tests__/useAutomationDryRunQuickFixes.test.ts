import { nextTick, ref, computed } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { useAutomationDryRunQuickFixes } from '../useAutomationDryRunQuickFixes'
import { AUTOMATION_DRY_RUN_QUICK_FIX_KEYS } from '../automationDryRunQuickFixActions'

const t = (key: string) => key

const createQuickFixes = (overrides: Record<string, any> = {}) => {
  const conditionData = ref<any[]>([])
  const actionData = ref<any[]>([])
  const refreshLocalExecutionExplanation = vi.fn()
  const quickFixes = useAutomationDryRunQuickFixes({
    isFirstDeviceAutomationStarter: computed(() => false),
    previewConditionGroups: computed(() => []),
    previewActions: computed(() => []),
    conditionData,
    actionData,
    editPremise: ref(null),
    editAction: ref(null),
    applyFirstAutomationRecommendedAction: vi.fn(),
    openFirstAutomationAlarmCreator: vi.fn(),
    refreshLocalExecutionExplanation,
    t,
    ...overrides
  })

  return { actionData, conditionData, quickFixes, refreshLocalExecutionExplanation }
}

describe('useAutomationDryRunQuickFixes', () => {
  it('refreshes the dry-run explanation after fallback condition mutation', async () => {
    const { conditionData, quickFixes, refreshLocalExecutionExplanation } = createQuickFixes()

    quickFixes.handleAutomationDryRunQuickFix(AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.addConditionGroup)
    await nextTick()

    expect(conditionData.value).toEqual([[{ ifType: null }]])
    expect(refreshLocalExecutionExplanation).toHaveBeenCalledTimes(1)
  })

  it('refreshes the dry-run explanation after fallback alarm action mutation', async () => {
    const { actionData, quickFixes, refreshLocalExecutionExplanation } = createQuickFixes()

    quickFixes.handleAutomationDryRunQuickFix(AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.addAlarmActionSlot)
    await nextTick()

    expect(actionData.value).toEqual([{ actionType: '30', action_type: '30', action_target: null }])
    expect(refreshLocalExecutionExplanation).toHaveBeenCalledTimes(1)
  })
})