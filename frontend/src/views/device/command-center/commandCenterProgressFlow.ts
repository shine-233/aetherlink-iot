export type CommandJobProgressState = 'done' | 'current' | 'waiting'
export type CommandJobProgressTagType = 'success' | 'warning' | 'info'

export interface CommandJobProgressFlowInput {
  scopeReady: boolean
  previewReady: boolean
  submitted: boolean
  supportReady: boolean
}

export interface CommandJobProgressStep {
  key: 'scope' | 'preview' | 'submit' | 'support'
  index: string
  state: CommandJobProgressState
  tagType: CommandJobProgressTagType
  statusKey: string
  titleKey: string
  descKey: string
}

const stateFor = (done: boolean, current: boolean): CommandJobProgressState => {
  if (done) return 'done'
  if (current) return 'current'
  return 'waiting'
}

const tagTypeFor = (state: CommandJobProgressState): CommandJobProgressTagType => {
  if (state === 'done') return 'success'
  if (state === 'current') return 'warning'
  return 'info'
}

const stepWithStatus = (step: Omit<CommandJobProgressStep, 'statusKey' | 'tagType'>): CommandJobProgressStep => ({
  ...step,
  statusKey: `custom.commandCenter.progressStatus.${step.state}`,
  tagType: tagTypeFor(step.state)
})

export const buildCommandJobProgressSteps = (input: CommandJobProgressFlowInput): CommandJobProgressStep[] => {
  const { scopeReady, previewReady, submitted, supportReady } = input

  return [
    stepWithStatus({
      key: 'scope',
      index: '01',
      state: stateFor(scopeReady, !scopeReady),
      titleKey: 'custom.commandCenter.progressScopeTitle',
      descKey: scopeReady
        ? 'custom.commandCenter.progressScopeReadyDesc'
        : 'custom.commandCenter.progressScopeTodoDesc'
    }),
    stepWithStatus({
      key: 'preview',
      index: '02',
      state: stateFor(previewReady, scopeReady && !previewReady),
      titleKey: 'custom.commandCenter.progressPreviewTitle',
      descKey: previewReady
        ? 'custom.commandCenter.progressPreviewReadyDesc'
        : scopeReady
          ? 'custom.commandCenter.progressPreviewTodoDesc'
          : 'custom.commandCenter.progressPreviewWaitingDesc'
    }),
    stepWithStatus({
      key: 'submit',
      index: '03',
      state: stateFor(submitted, previewReady && !submitted),
      titleKey: 'custom.commandCenter.progressSubmitTitle',
      descKey: submitted
        ? 'custom.commandCenter.progressSubmitReadyDesc'
        : previewReady
          ? 'custom.commandCenter.progressSubmitTodoDesc'
          : 'custom.commandCenter.progressSubmitWaitingDesc'
    }),
    stepWithStatus({
      key: 'support',
      index: '04',
      state: stateFor(supportReady, submitted && !supportReady),
      titleKey: 'custom.commandCenter.progressSupportTitle',
      descKey: supportReady
        ? 'custom.commandCenter.progressSupportReadyDesc'
        : submitted
          ? 'custom.commandCenter.progressSupportTodoDesc'
          : 'custom.commandCenter.progressSupportWaitingDesc'
    })
  ]
}