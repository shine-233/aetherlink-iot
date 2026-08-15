import dayjs from 'dayjs'

export const buildExpectedTelemetryPayload = (options: {
  deviceId: string
  payload: string
  expiryHours: number | null
}) => {
  const expiry = new Date().getTime() + (options.expiryHours ? options.expiryHours * 60 * 60 * 1000 : 0)
  return {
    device_id: options.deviceId,
    payload: options.payload,
    send_type: 'telemetry',
    expiry: dayjs(expiry).format('YYYY-MM-DDTHH:mm:ssZ')
  }
}

export const buildDirectTelemetryPayload = (deviceId: string, value: string) => ({
  device_id: deviceId,
  value
})

export const hasInvalidJsonInput = (value: string, isJSON: (value: string) => boolean) =>
  Boolean(value) && !isJSON(value)
