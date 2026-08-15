import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const nginxConfigPath = resolve(process.cwd(), 'nginx.conf')
const optionalNginxConfigPath = resolve(process.cwd(), 'nginx.thingsvis.conf')
const optionalComposePath = resolve(process.cwd(), '../deploy/docker-compose.optional-integrations.yml')

describe('frontend nginx lightweight deployment contract', () => {
  it('does not require optional ThingsVis services in the default compose stack', () => {
    const config = readFileSync(nginxConfigPath, 'utf8')

    expect(config).not.toMatch(/(?:proxy_pass|upstream)\s+https?:\/\/thingsvis-/)
    expect(config).toContain('THINGSVIS_OPTIONAL_SERVICE_DISABLED')
    expect(config).toContain('location = /main')
    expect(config).toContain('location = /main.html')
    expect(config).toContain('location /main/')
    expect(config).toContain('location /thingsvis-api/')
    expect(config).toContain('location = /registry.json')
    expect(config).toContain('location /widgets/')
    expect(config).toContain('location = /mf-manifest.json')
  })

  it('keeps a separate real proxy contract for the optional integration profile', () => {
    const optionalConfig = readFileSync(optionalNginxConfigPath, 'utf8')
    const compose = readFileSync(optionalComposePath, 'utf8')

    expect(compose).toContain('profiles: [optional-integrations]')
    expect(compose).toContain('thingsvis-server')
    expect(compose).toContain('thingsvis-studio')
    expect(compose).toContain('http_adapter')
    expect(compose).toContain('THINGSVIS_AUTH_SECRET')
    expect(compose).toContain('P_PLATFORM_URL: http://backend:9999')
    expect(compose).toContain('P_PLATFORM_MQTT_BROKER: tcp://mqtt-broker:1883')

    expect(optionalConfig).toContain('proxy_pass http://thingsvis-server:8000/api/v1/')
    expect(optionalConfig).toContain('proxy_pass http://thingsvis-studio:3000/')
    expect(optionalConfig).not.toContain('THINGSVIS_OPTIONAL_SERVICE_DISABLED')
  })
})
