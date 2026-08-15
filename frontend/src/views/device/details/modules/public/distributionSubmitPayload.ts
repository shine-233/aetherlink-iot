type DistributionSubmitInput = {
  deviceId: string
  isCommand?: boolean
  textValue?: string | null
  commandValue?: string | null
}

type ExpectedSubmitInput = DistributionSubmitInput & {
  expiry: string | null
}

export const normalizedPayloadValue = (textValue?: string | null) => (textValue ? textValue : null)

export function buildDistributionSubmitPayload(input: DistributionSubmitInput) {
  if (input.isCommand) {
    return {
      device_id: input.deviceId,
      value: normalizedPayloadValue(input.textValue),
      identify: input.commandValue
    }
  }

  return {
    device_id: input.deviceId,
    value: normalizedPayloadValue(input.textValue)
  }
}

export function buildExpectedMessagePayload(input: ExpectedSubmitInput) {
  return {
    device_id: input.deviceId,
    payload: normalizedPayloadValue(input.textValue),
    send_type: input.isCommand ? 'command' : 'attribute',
    expiry: input.expiry,
    identify: input.isCommand ? input.commandValue : null
  }
}

export function buildQuickCommandPayload(deviceId: string, row: any) {
  return {
    device_id: deviceId,
    value: row?.instruct,
    identify: row?.data_identifier
  }
}

export const quickCommandKey = (row: any) => String(row?.id || row?.data_identifier || row?.buttom_name || '')

export const isApiError = (response: { error?: unknown } | undefined) => Boolean(response?.error)
