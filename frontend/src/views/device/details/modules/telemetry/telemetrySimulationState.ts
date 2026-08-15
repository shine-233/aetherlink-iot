export const DEFAULT_SIMULATION_TOPIC = 'devices/telemetry'
export const DEFAULT_SIMULATION_PAYLOAD =
  '{"temperature":25.5,"humidity":60,"rssi":-52,"online":true,"alarm_count":0}'
export const DEFAULT_SIMULATION_EVENT_PAYLOAD =
  '{"method":"report_alarm","params":{"alarm_code":"over_temperature","level":"warning","value":38.5}}'

export interface TelemetrySimulationFormState {
  username: string
  password: string
  client_id: string
  server: string
  port: number
  topic: string
  topic_options: { label: string; value: string }[]
  default_data: string
  event_default_data: string
  normal_default_data: string
}

export const createTelemetrySimulationFormState = (): TelemetrySimulationFormState => ({
  username: '',
  password: '',
  client_id: '',
  server: '',
  port: 1883,
  topic: DEFAULT_SIMULATION_TOPIC,
  topic_options: [],
  default_data: DEFAULT_SIMULATION_PAYLOAD,
  event_default_data: DEFAULT_SIMULATION_EVENT_PAYLOAD,
  normal_default_data: DEFAULT_SIMULATION_PAYLOAD
})

export const applySimulationPayloadDefaults = (form: TelemetrySimulationFormState) => {
  form.default_data = DEFAULT_SIMULATION_PAYLOAD
  form.event_default_data = DEFAULT_SIMULATION_EVENT_PAYLOAD
  form.normal_default_data = DEFAULT_SIMULATION_PAYLOAD
}

export const applySimulationInitData = (form: TelemetrySimulationFormState, data: Record<string, any>) => {
  form.username = data.username || ''
  form.password = data.password || ''
  form.client_id = data.client_id || ''
  form.server = data.server || ''
  form.port = data.port || 1883
  form.topic = data.topic || DEFAULT_SIMULATION_TOPIC
  form.topic_options = data.topic_options || []
  form.default_data = data.default_data || DEFAULT_SIMULATION_PAYLOAD
  form.event_default_data = data.event_default_data || DEFAULT_SIMULATION_EVENT_PAYLOAD
  form.normal_default_data = data.default_data || DEFAULT_SIMULATION_PAYLOAD
}

export const buildSimulationPayload = (form: TelemetrySimulationFormState, deviceId: string) => ({
  device_id: deviceId,
  data: form.default_data,
  server: form.server,
  port: form.port,
  topic: form.topic
})

export const syncSimulationPayloadByTopic = (form: TelemetrySimulationFormState, topic: string) => {
  if (!topic) return
  if (topic.includes('/event/')) {
    form.default_data = form.event_default_data
  } else {
    form.default_data = form.normal_default_data
  }
}

export const rememberSimulationPayloadByTopic = (
  form: TelemetrySimulationFormState,
  topic: string,
  payload: string
) => {
  if (!topic) return
  if (topic.includes('/event/')) {
    form.event_default_data = payload
  } else {
    form.normal_default_data = payload
  }
}

export const extractSimulationErrorMessage = (error: any) => error?.response?.data?.message || error?.message || ''
