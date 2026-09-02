import { createProxyPattern } from '~/env.config'
import { getPlatformApiBaseUrl } from '@/utils/common/tool'

export type DeploymentHealthCheck = {
  ok: boolean
  latency_ms: number
  error?: string
}

export type NormalizedDeploymentHealthRow = {
  key: string
  label: string
  description: string
  source: string
  sourceLabel: string
  nextAction: string
  ok: boolean
  latency: number
  error: string
}

export type DeploymentHealthReport = {
  status: 'ok' | 'down' | 'degraded' | string
  version?: string
  timestamp?: string
  http_status?: number
  frontend_proxy?: DeploymentHealthCheck
  api?: DeploymentHealthCheck
  checks?: Record<string, DeploymentHealthCheck>
}

type Translate = (key: string, params?: Record<string, unknown>) => string

type DeploymentHealthEndpointOptions = {
  httpProxyEnabled?: boolean
  platformApiBaseUrl?: string
}

export const resolveDeploymentHealthEndpoint = (options: DeploymentHealthEndpointOptions = {}): string => {
  const httpProxyEnabled = options.httpProxyEnabled ?? import.meta.env.VITE_HTTP_PROXY === 'Y'
  if (httpProxyEnabled) return `${createProxyPattern()}/deployment/health`

  const platformApiBaseUrl = options.platformApiBaseUrl ?? getPlatformApiBaseUrl()
  return `${platformApiBaseUrl.replace(/\/$/, '')}/deployment/health`
}

const healthCopyKeyMap: Record<string, { label: string; description: string; nextAction: string }> = {
  frontend_proxy: {
    label: 'custom.home.firstDevice.health.checks.frontendProxy.label',
    description: 'custom.home.firstDevice.health.checks.frontendProxy.description',
    nextAction: 'custom.home.firstDevice.health.checks.frontendProxy.nextAction'
  },
  api: {
    label: 'custom.home.firstDevice.health.checks.api.label',
    description: 'custom.home.firstDevice.health.checks.api.description',
    nextAction: 'custom.home.firstDevice.health.checks.api.nextAction'
  },
  database: {
    label: 'custom.home.firstDevice.health.checks.database.label',
    description: 'custom.home.firstDevice.health.checks.database.description',
    nextAction: 'custom.home.firstDevice.health.checks.database.nextAction'
  },
  redis: {
    label: 'custom.home.firstDevice.health.checks.redis.label',
    description: 'custom.home.firstDevice.health.checks.redis.description',
    nextAction: 'custom.home.firstDevice.health.checks.redis.nextAction'
  },
  status_redis: {
    label: 'custom.home.firstDevice.health.checks.statusRedis.label',
    description: 'custom.home.firstDevice.health.checks.statusRedis.description',
    nextAction: 'custom.home.firstDevice.health.checks.statusRedis.nextAction'
  },
  mqtt: {
    label: 'custom.home.firstDevice.health.checks.mqtt.label',
    description: 'custom.home.firstDevice.health.checks.mqtt.description',
    nextAction: 'custom.home.firstDevice.health.checks.mqtt.nextAction'
  }
}

const healthSourceMap: Record<string, string> = {
  frontend_proxy: 'browser-same-origin',
  api: 'browser-api-fetch',
  database: 'backend-health-api',
  redis: 'backend-health-api',
  status_redis: 'backend-health-api',
  mqtt: 'backend-mqtt-probe'
}

const healthSourceLabelKeyMap: Record<string, string> = {
  'browser-same-origin': 'custom.home.firstDevice.health.sources.browserSameOrigin',
  'browser-api-fetch': 'custom.home.firstDevice.health.sources.browserApiFetch',
  'backend-health-api': 'custom.home.firstDevice.health.sources.backendHealthApi',
  'backend-mqtt-probe': 'custom.home.firstDevice.health.sources.backendMqttProbe'
}

const requiredDeploymentHealthKeys = ['frontend_proxy', 'api', 'database', 'redis', 'mqtt']

const translateHealthCopy = (key: string, field: 'label' | 'description' | 'nextAction', t: Translate) => {
  const translationKey = healthCopyKeyMap[key]?.[field]
  if (!translationKey) return field === 'label' ? key : ''
  return t(translationKey)
}

const missingDeploymentHealthCheck = (key: string, t: Translate): DeploymentHealthCheck => ({
  ok: false,
  latency_ms: 0,
  error: t('custom.home.firstDevice.health.missingCheck', {
    label: translateHealthCopy(key, 'label', t)
  })
})

export const normalizeDeploymentHealth = (
  report: DeploymentHealthReport | null,
  t: Translate
): NormalizedDeploymentHealthRow[] => {
  if (!report) return []
  const optionalChecks = report.checks || {}
  const checks: Record<string, DeploymentHealthCheck> = {}

  for (const key of requiredDeploymentHealthKeys) {
    const check = key === 'frontend_proxy' ? report.frontend_proxy : key === 'api' ? report.api : optionalChecks[key]
    checks[key] = check || missingDeploymentHealthCheck(key, t)
  }

  for (const [key, check] of Object.entries(optionalChecks)) {
    if (check && !checks[key]) checks[key] = check
  }

  return Object.entries(checks).map(([key, check]) => {
    const source = healthSourceMap[key] || 'backend-health-api'
    const sourceLabelKey = healthSourceLabelKeyMap[source]

    return {
      key,
      label: translateHealthCopy(key, 'label', t),
      description: translateHealthCopy(key, 'description', t),
      source,
      sourceLabel: sourceLabelKey ? t(sourceLabelKey) : t('custom.home.firstDevice.health.sources.default'),
      nextAction: translateHealthCopy(key, 'nextAction', t) || t('custom.home.firstDevice.health.defaultNextAction'),
      ok: Boolean(check.ok),
      latency: Number(check.latency_ms || 0),
      error: check.error || ''
    }
  })
}

export const fetchDeploymentHealthReport = async (t: Translate): Promise<DeploymentHealthReport | null> => {
  const started = performance.now()
  try {
    const response = await fetch(resolveDeploymentHealthEndpoint(), {
      cache: 'no-store',
      headers: {
        Accept: 'application/json'
      }
    })
    const report = (await response.json()) as DeploymentHealthReport
    const latencyMS = Math.max(0, Math.round(performance.now() - started))
    return {
      ...report,
      http_status: response.status,
      api: {
        ok: response.ok,
        latency_ms: latencyMS,
        error: response.ok ? '' : `HTTP ${response.status}`
      },
      frontend_proxy: {
        ok: response.ok,
        latency_ms: latencyMS,
        error: response.ok ? '' : `HTTP ${response.status}`
      }
    }
  } catch (error: any) {
    const latencyMS = Math.max(0, Math.round(performance.now() - started))
    return {
      status: 'down',
      timestamp: new Date().toISOString(),
      api: {
        ok: false,
        latency_ms: latencyMS,
        error: error?.message || t('custom.home.firstDevice.health.apiUnavailable')
      },
      frontend_proxy: {
        ok: true,
        latency_ms: 0
      },
      checks: {}
    }
  }
}
