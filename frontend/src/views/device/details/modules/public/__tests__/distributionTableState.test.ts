import {
  createDeliveryModeView,
  createSubmitTrackingView,
  normalizeDistributionListView,
  shouldDisableDistributionSubmit
} from '../distributionTableState'

const t = (key: string) => key
const isValidJson = (value: string) => {
  try {
    JSON.parse(value)
    return true
  } catch {
    return false
  }
}

describe('distributionTableState', () => {
  it('normalizes list rows and pagination from compatible response shapes', () => {
    expect(normalizeDistributionListView({ value: [{ id: 1 }], count: 9 })).toEqual({
      rows: [{ id: 1 }],
      pageCount: 3
    })
    expect(normalizeDistributionListView({ list: [{ id: 2 }], total: 4 })).toEqual({
      rows: [{ id: 2 }],
      pageCount: 1
    })
    expect(normalizeDistributionListView([{ id: 3 }])).toEqual({
      rows: [{ id: 3 }],
      pageCount: 0
    })
  })

  it('keeps command submit disabled until command id and valid JSON are present', () => {
    expect(shouldDisableDistributionSubmit({ isCommand: true, commandValue: '', isValidJson })).toBe(true)
    expect(
      shouldDisableDistributionSubmit({
        isCommand: true,
        commandValue: 'restart',
        textValue: '{bad',
        isValidJson
      })
    ).toBe(true)
    expect(
      shouldDisableDistributionSubmit({
        isCommand: true,
        commandValue: 'restart',
        textValue: '{"delay":1}',
        isValidJson
      })
    ).toBe(false)
  })

  it('builds customer-facing delivery and tracking hints', () => {
    expect(createDeliveryModeView(false, false, t).title).toBe('generate.deliveryModeImmediateTitle')
    expect(createDeliveryModeView(true, false, t).hint).toBe('generate.deliveryModeExpectedHint')
    expect(createDeliveryModeView(false, true, t)).toEqual({
      title: 'generate.deliveryModeDirectTitle',
      hint: 'generate.deliveryModeDirectHint'
    })

    expect(createSubmitTrackingView(null, t).visible).toBe(false)
    expect(createSubmitTrackingView({ messageId: 'msg-1', logRecorded: false }, t)).toEqual({
      visible: true,
      type: 'warning',
      text: 'generate.commandSubmittedLogUnavailable msg-1'
    })
  })
})
