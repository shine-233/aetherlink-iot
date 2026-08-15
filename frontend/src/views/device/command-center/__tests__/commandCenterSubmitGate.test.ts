import {
  buildCommandJobReadiness,
  buildCommandSubmitDisabledHint,
  commandPreviewCoversFullFilterScope
} from '../commandCenterSubmitGate'

const t = (key: string) => key

describe('commandCenterSubmitGate', () => {
  it('blocks filtered fleet submit when the preview only covers a subset', () => {
    const previewResult = {
      requested_count: 10,
      eligible_count: 2,
      rows: [{ device_id: 'dev-1' }, { device_id: 'dev-2' }]
    } as any

    expect(commandPreviewCoversFullFilterScope({ isDeviceFilterScope: true, previewResult })).toBe(false)
    expect(
      buildCommandSubmitDisabledHint(
        {
          hasCommandJobScope: true,
          commandIdentify: 'reboot',
          previewResult,
          previewPayloadFingerprint: 'same',
          currentPayloadFingerprint: 'same',
          previewCoversFullFilterScope: false,
          maxDevices: 20
        },
        t
      )
    ).toBe('custom.commandCenter.submitBlockedSubsetOnly')

    expect(
      buildCommandJobReadiness(
        {
          hasCommandJobScope: true,
          commandIdentify: 'reboot',
          previewResult,
          previewPayloadFingerprint: 'same',
          currentPayloadFingerprint: 'same',
          previewCoversFullFilterScope: false,
          maxDevices: 20
        },
        t
      )
    ).toMatchObject({
      canSubmit: false,
      previewCoverageStatus: 'subset_only',
      customerRiskLevel: 'warning'
    })
  })

  it('allows submit when the preview covers the current scope and payload fingerprint', () => {
    const previewResult = {
      requested_count: 2,
      eligible_count: 2,
      rows: [{ device_id: 'dev-1' }, { device_id: 'dev-2' }]
    } as any

    expect(commandPreviewCoversFullFilterScope({ isDeviceFilterScope: true, previewResult })).toBe(true)
    expect(
      buildCommandJobReadiness(
        {
          hasCommandJobScope: true,
          commandIdentify: 'reboot',
          previewResult,
          previewPayloadFingerprint: 'same',
          currentPayloadFingerprint: 'same',
          previewCoversFullFilterScope: true,
          maxDevices: 20
        },
        t
      )
    ).toMatchObject({
      canSubmit: true,
      previewCoverageStatus: 'full',
      customerRiskLevel: 'ready'
    })
  })
})
