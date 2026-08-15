type Translate = (key: string) => string

export type AutomationRouteQuery = Record<string, unknown>

function firstRouteQueryValue(value: unknown) {
  if (Array.isArray(value)) {
    return value[0] ? String(value[0]) : ''
  }

  return value ? String(value) : ''
}

export function createDefaultAutomationForm() {
  return {
    id: '',
    name: null as string | null,
    description: null as string | null,
    enabled: 'Y',
    trigger_condition_groups: [] as any[],
    actions: [] as any[]
  }
}

export function createAutomationFormRules(t: Translate) {
  return {
    name: {
      required: true,
      message: t('generate.enter-scene-linkage-name'),
      trigger: 'blur'
    },
    description: {
      required: false,
      message: t('generate.sceneLinkDesc'),
      trigger: 'blur'
    },
    trigger_condition_groups: {
      required: true,
      message: t('generate.addExecutionConditions')
    },
    actions: {
      required: true,
      message: t('generate.addExecutionAction')
    }
  }
}

export function readAutomationRouteContext(query: AutomationRouteQuery) {
  return {
    configId: firstRouteQueryValue(query.id),
    backType: firstRouteQueryValue(query.backType),
    onboarding: firstRouteQueryValue(query.onboarding),
    propsData: {
      device_id: firstRouteQueryValue(query.device_id),
      device_config_id: firstRouteQueryValue(query.device_config_id)
    },
    starter: {
      type: firstRouteQueryValue(query.starter),
      deviceName: firstRouteQueryValue(query.first_device_name),
      deviceNumber: firstRouteQueryValue(query.first_device_number),
      telemetryKey: firstRouteQueryValue(query.telemetry_key),
      telemetryValue: firstRouteQueryValue(query.telemetry_value),
      telemetryAt: firstRouteQueryValue(query.telemetry_at)
    }
  }
}
