export type CommandCenterGuideStatusType = 'default' | 'success' | 'warning'

export interface CommandCenterGuideState {
  hasCommandJobScope: boolean
  hasCommandIdentifier: boolean
  hasPreviewResult: boolean
  hasSubmitResult: boolean
  canPreviewCommandJob: boolean
  canSubmitCommandJob: boolean
  previewLoading: boolean
  submitLoading: boolean
}

export interface CommandCenterGuideActions {
  openFleet: () => void
  previewCommandJob: () => void
  submitCommandJob: () => void
}

export interface CommandCenterGuideStep {
  key: 'select' | 'identify' | 'preview' | 'submit'
  index: string
  titleKey: string
  descKey: string
  statusKey: string
  statusType: CommandCenterGuideStatusType
  actionLabelKey?: string
  action?: () => void
  disabled: boolean
}

export const buildCommandCenterGuideSteps = (
  state: CommandCenterGuideState,
  actions: CommandCenterGuideActions
): CommandCenterGuideStep[] => [
  {
    key: 'select',
    index: '1',
    titleKey: 'custom.commandCenter.guideSelectTitle',
    descKey: 'custom.commandCenter.guideSelectDesc',
    statusKey: state.hasCommandJobScope
      ? 'custom.commandCenter.guideStatusReady'
      : 'custom.commandCenter.guideStatusAction',
    statusType: state.hasCommandJobScope ? 'success' : 'warning',
    actionLabelKey: 'custom.commandCenter.backToFleet',
    action: actions.openFleet,
    disabled: false
  },
  {
    key: 'identify',
    index: '2',
    titleKey: 'custom.commandCenter.guideIdentifyTitle',
    descKey: 'custom.commandCenter.guideIdentifyDesc',
    statusKey: state.hasCommandIdentifier
      ? 'custom.commandCenter.guideStatusReady'
      : 'custom.commandCenter.guideStatusWait',
    statusType: state.hasCommandIdentifier ? 'success' : 'default',
    disabled: true
  },
  {
    key: 'preview',
    index: '3',
    titleKey: 'custom.commandCenter.guidePreviewTitle',
    descKey: 'custom.commandCenter.guidePreviewDesc',
    statusKey: state.hasPreviewResult
      ? 'custom.commandCenter.guideStatusReady'
      : state.canPreviewCommandJob
        ? 'custom.commandCenter.guideStatusAction'
        : 'custom.commandCenter.guideStatusWait',
    statusType: state.hasPreviewResult ? 'success' : state.canPreviewCommandJob ? 'warning' : 'default',
    actionLabelKey: 'custom.commandCenter.previewCommandJob',
    action: actions.previewCommandJob,
    disabled: !state.canPreviewCommandJob || state.previewLoading
  },
  {
    key: 'submit',
    index: '4',
    titleKey: 'custom.commandCenter.guideSubmitTitle',
    descKey: 'custom.commandCenter.guideSubmitDesc',
    statusKey: state.hasSubmitResult
      ? 'custom.commandCenter.guideStatusReady'
      : state.canSubmitCommandJob
        ? 'custom.commandCenter.guideStatusAction'
        : 'custom.commandCenter.guideStatusWait',
    statusType: state.hasSubmitResult ? 'success' : state.canSubmitCommandJob ? 'warning' : 'default',
    actionLabelKey: 'custom.commandCenter.submitEligibleDevices',
    action: actions.submitCommandJob,
    disabled: !state.canSubmitCommandJob || state.submitLoading
  }
]
