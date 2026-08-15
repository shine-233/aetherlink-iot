import { describe, expect, it } from 'vitest'
import { buildFirstAutomationStarterChecklist, type FirstAutomationChecklistTexts } from '../automationStarterChecklist'

const texts: FirstAutomationChecklistTexts = {
  conditionTitle: 'Choose condition',
  conditionDesc: 'Use temperature = 36.5',
  actionTitle: 'Choose action',
  actionDesc: 'Pick a safe action',
  dryRunTitle: 'Run dry-run',
  dryRunDesc: 'Check blockers first',
  saveTitle: 'Save rule',
  saveDesc: 'Save after dry-run'
}

describe('automationStarterChecklist', () => {
  it('returns no checklist outside first-device starter mode', () => {
    expect(
      buildFirstAutomationStarterChecklist(
        {
          enabled: false,
          conditionCount: 0,
          actionCount: 0,
          backendDryRunStatus: 'waiting',
          customerDryRunStatus: 'unchecked',
          canSave: null
        },
        texts
      )
    ).toEqual([])
  })

  it('walks the first-rule flow from condition to action to dry-run to save', () => {
    expect(
      buildFirstAutomationStarterChecklist(
        {
          enabled: true,
          conditionCount: 0,
          actionCount: 0,
          backendDryRunStatus: 'waiting',
          customerDryRunStatus: 'unchecked',
          canSave: null
        },
        texts
      ).map((item) => item.status)
    ).toEqual(['active', 'todo', 'todo', 'todo'])

    expect(
      buildFirstAutomationStarterChecklist(
        {
          enabled: true,
          conditionCount: 1,
          actionCount: 0,
          backendDryRunStatus: 'ready',
          customerDryRunStatus: 'unchecked',
          canSave: null
        },
        texts
      ).map((item) => item.status)
    ).toEqual(['done', 'active', 'todo', 'todo'])

    expect(
      buildFirstAutomationStarterChecklist(
        {
          enabled: true,
          conditionCount: 1,
          actionCount: 1,
          backendDryRunStatus: 'ready',
          customerDryRunStatus: 'unchecked',
          canSave: null
        },
        texts
      ).map((item) => item.status)
    ).toEqual(['done', 'done', 'active', 'todo'])

    expect(
      buildFirstAutomationStarterChecklist(
        {
          enabled: true,
          conditionCount: 1,
          actionCount: 1,
          backendDryRunStatus: 'available',
          customerDryRunStatus: 'passed',
          canSave: true
        },
        texts
      ).map((item) => item.status)
    ).toEqual(['done', 'done', 'done', 'active'])
  })

  it('keeps save todo when backend dry-run returns risk', () => {
    expect(
      buildFirstAutomationStarterChecklist(
        {
          enabled: true,
          conditionCount: 1,
          actionCount: 1,
          backendDryRunStatus: 'available',
          customerDryRunStatus: 'risk',
          canSave: false
        },
        texts
      ).map((item) => item.status)
    ).toEqual(['done', 'done', 'active', 'todo'])
  })

  it('keeps save todo when backend dry-run is unavailable', () => {
    expect(
      buildFirstAutomationStarterChecklist(
        {
          enabled: true,
          conditionCount: 1,
          actionCount: 1,
          backendDryRunStatus: 'unavailable',
          customerDryRunStatus: 'risk',
          canSave: null
        },
        texts
      ).map((item) => item.status)
    ).toEqual(['done', 'done', 'active', 'todo'])
  })
})
