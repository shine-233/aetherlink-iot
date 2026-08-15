import { describe, expect, it } from 'vitest'
import { resolveDeploymentHealthEndpoint } from '../homeDeploymentHealth'

describe('home deployment health endpoint', () => {
  it('uses the shared Vite proxy route when the HTTP proxy is enabled', () => {
    expect(resolveDeploymentHealthEndpoint({ httpProxyEnabled: true })).toBe('/proxy-default/deployment/health')
  })

  it('uses the configured platform API base in direct or production mode', () => {
    expect(
      resolveDeploymentHealthEndpoint({
        httpProxyEnabled: false,
        platformApiBaseUrl: 'https://iot.example.com/api/v1/'
      })
    ).toBe('https://iot.example.com/api/v1/deployment/health')
  })
})
