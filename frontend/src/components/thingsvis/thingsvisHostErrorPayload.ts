export type ThingsVisHostErrorPayload = {
  code: string
  message: string
  scope: string
}

export type ThingsVisHostDiagnostic = ThingsVisHostErrorPayload & {
  source: 'auth' | 'init' | 'device_bridge' | 'field_bridge' | 'host_bridge'
  at: number
}

export function resolveThingsVisHostErrorMessage(error: unknown) {
  if (error instanceof Error && error.message) return error.message
  if (typeof error === 'string' && error) return error

  return 'ThingsVis host request failed.'
}

export function buildThingsVisHostErrorPayload(scope: string, error: unknown, code = 'host_request_failed') {
  return {
    success: false,
    error: {
      code,
      message: resolveThingsVisHostErrorMessage(error),
      scope
    } satisfies ThingsVisHostErrorPayload
  }
}

export function buildThingsVisHostDiagnostic(
  source: ThingsVisHostDiagnostic['source'],
  scope: string,
  error: unknown,
  code = 'host_request_failed'
): ThingsVisHostDiagnostic {
  const payload = buildThingsVisHostErrorPayload(scope, error, code)
  return {
    ...payload.error,
    source,
    at: Date.now()
  }
}
