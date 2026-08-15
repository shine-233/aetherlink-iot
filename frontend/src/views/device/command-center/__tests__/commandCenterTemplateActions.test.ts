import { ref } from 'vue'
import { useCommandCenterTemplateActions } from '../useCommandCenterTemplateActions'
import type { CommandCenterSavedCommandTemplate } from '../useCommandCenterCommandTemplates'

const messages: Record<string, string> = {
  'common.copyFailed': 'Copy failed',
  'custom.commandCenter.applyCommandTemplateSuccess': 'Template applied',
  'custom.commandCenter.commandIdentifier': 'Command',
  'custom.commandCenter.copyCommandTemplateSuccess': 'Template copied',
  'custom.commandCenter.copyCommandTemplatesSuccess': 'Templates copied',
  'custom.commandCenter.deleteCommandTemplateSuccess': 'Template deleted',
  'custom.commandCenter.importCommandTemplatesInvalid': 'Invalid import',
  'custom.commandCenter.importCommandTemplatesSuccess': 'Imported {count} template(s)',
  'custom.commandCenter.saveCommandTemplateMissingIdentify': 'Missing identify',
  'custom.commandCenter.saveCommandTemplateSuccess': 'Template saved',
  'custom.commandCenter.saveJobAsTemplateSuccess': 'Job saved as template'
}

const t = (key: string) => messages[key] || key

const createTemplateActions = (overrides: Partial<Parameters<typeof useCommandCenterTemplateActions>[0]> = {}) => {
  const successMessages: string[] = []
  const warningMessages: string[] = []
  const savedDrafts: any[] = []
  const deletedIds: string[] = []
  const copiedTexts: string[] = []
  let resetCount = 0
  let clearReuseCount = 0

  const state = {
    commandIdentify: ref('reboot'),
    commandValue: ref('{"delay":1}'),
    timeoutSeconds: ref(30),
    commandTemplateName: ref('Reboot command')
  }

  const actions = useCommandCenterTemplateActions({
    ...state,
    saveCommandTemplate: draft => {
      savedDrafts.push(draft)
      return Boolean(draft.identify)
    },
    deleteCommandTemplate: templateId => {
      deletedIds.push(templateId)
    },
    importCommandTemplates: raw => {
      if (raw === 'bad') throw new Error('invalid json')
      return { imported: raw === 'empty' ? 0 : 2, skipped: 0 }
    },
    resetCommandJobDraft: () => {
      resetCount += 1
    },
    clearReusedCommandJobDraft: () => {
      clearReuseCount += 1
    },
    t,
    copyText: async text => {
      copiedTexts.push(text)
      return true
    },
    notifySuccess: message => successMessages.push(message),
    notifyWarning: message => warningMessages.push(message),
    ...overrides
  })

  return {
    ...state,
    actions,
    copiedTexts,
    deletedIds,
    get clearReuseCount() {
      return clearReuseCount
    },
    get resetCount() {
      return resetCount
    },
    savedDrafts,
    successMessages,
    warningMessages
  }
}

describe('useCommandCenterTemplateActions', () => {
  it('saves the current command draft as a reusable template', () => {
    const state = createTemplateActions()

    state.actions.saveCurrentCommandTemplate()

    expect(state.savedDrafts).toEqual([
      {
        identify: 'reboot',
        value: '{"delay":1}',
        timeoutSeconds: 30
      }
    ])
    expect(state.successMessages).toContain('Template saved')
  })

  it('applies a saved template and clears reused job state', () => {
    const state = createTemplateActions()
    const template: CommandCenterSavedCommandTemplate = {
      id: 'template-1',
      name: 'Restart pump',
      identify: 'restart',
      value: '{"target":"pump"}',
      timeoutSeconds: 45,
      updatedAt: '2026-07-06T00:00:00Z'
    }

    state.actions.applySavedCommandTemplate(template)

    expect(state.commandIdentify.value).toBe('restart')
    expect(state.commandValue.value).toBe('{"target":"pump"}')
    expect(state.timeoutSeconds.value).toBe(45)
    expect(state.commandTemplateName.value).toBe('Restart pump')
    expect(state.clearReuseCount).toBe(1)
    expect(state.resetCount).toBe(1)
    expect(state.successMessages).toContain('Template applied')
  })

  it('warns on invalid imports without changing command fields', () => {
    const state = createTemplateActions()

    state.actions.importSavedCommandTemplates('bad')

    expect(state.commandIdentify.value).toBe('reboot')
    expect(state.warningMessages).toContain('Invalid import')
  })

  it('copies exported templates through the injected clipboard boundary', async () => {
    const state = createTemplateActions()

    await state.actions.copyCommandTemplateExport([
      {
        id: 'template-1',
        name: 'Restart pump',
        identify: 'restart',
        value: '{}',
        timeoutSeconds: 30,
        updatedAt: '2026-07-06T00:00:00Z'
      }
    ])

    expect(state.copiedTexts[0]).toContain('"kind": "aetherlink.commandTemplates"')
    expect(state.successMessages).toContain('Template copied')
  })
})
