import { buildCommandCenterGuideSteps } from '../commandCenterGuide'

const actions = {
  openFleet: vi.fn(),
  previewCommandJob: vi.fn(),
  submitCommandJob: vi.fn()
}

describe('commandCenterGuide', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('points the operator back to fleet before a command scope exists', () => {
    const steps = buildCommandCenterGuideSteps(
      {
        hasCommandJobScope: false,
        hasCommandIdentifier: false,
        hasPreviewResult: false,
        hasSubmitResult: false,
        canPreviewCommandJob: false,
        canSubmitCommandJob: false,
        previewLoading: false,
        submitLoading: false
      },
      actions
    )

    expect(steps.map((step) => step.key)).toEqual(['select', 'identify', 'preview', 'submit'])
    expect(steps[0]).toMatchObject({
      statusKey: 'custom.commandCenter.guideStatusAction',
      statusType: 'warning',
      disabled: false
    })
    expect(steps[2]).toMatchObject({
      statusKey: 'custom.commandCenter.guideStatusWait',
      statusType: 'default',
      disabled: true
    })
    expect(steps[3]).toMatchObject({
      statusKey: 'custom.commandCenter.guideStatusWait',
      statusType: 'default',
      disabled: true
    })
  })

  it('enables preview only after a scope exists and no preview is loading', () => {
    const steps = buildCommandCenterGuideSteps(
      {
        hasCommandJobScope: true,
        hasCommandIdentifier: true,
        hasPreviewResult: false,
        hasSubmitResult: false,
        canPreviewCommandJob: true,
        canSubmitCommandJob: false,
        previewLoading: false,
        submitLoading: false
      },
      actions
    )

    expect(steps[0]).toMatchObject({
      statusKey: 'custom.commandCenter.guideStatusReady',
      statusType: 'success'
    })
    expect(steps[1]).toMatchObject({
      statusKey: 'custom.commandCenter.guideStatusReady',
      statusType: 'success',
      disabled: true
    })
    expect(steps[2]).toMatchObject({
      action: actions.previewCommandJob,
      disabled: false
    })
  })

  it('shows submit as the next action only when the preview token is still valid', () => {
    const steps = buildCommandCenterGuideSteps(
      {
        hasCommandJobScope: true,
        hasCommandIdentifier: true,
        hasPreviewResult: false,
        hasSubmitResult: false,
        canPreviewCommandJob: true,
        canSubmitCommandJob: true,
        previewLoading: false,
        submitLoading: false
      },
      actions
    )

    expect(steps[3]).toMatchObject({
      statusKey: 'custom.commandCenter.guideStatusAction',
      statusType: 'warning',
      action: actions.submitCommandJob,
      disabled: false
    })
  })

  it('marks the job flow ready after submission and blocks duplicate submits while loading', () => {
    const steps = buildCommandCenterGuideSteps(
      {
        hasCommandJobScope: true,
        hasCommandIdentifier: true,
        hasPreviewResult: true,
        hasSubmitResult: true,
        canPreviewCommandJob: true,
        canSubmitCommandJob: true,
        previewLoading: false,
        submitLoading: true
      },
      actions
    )

    expect(steps[2]).toMatchObject({
      statusKey: 'custom.commandCenter.guideStatusReady',
      statusType: 'success'
    })
    expect(steps[3]).toMatchObject({
      statusKey: 'custom.commandCenter.guideStatusReady',
      statusType: 'success',
      disabled: true
    })
  })
})
