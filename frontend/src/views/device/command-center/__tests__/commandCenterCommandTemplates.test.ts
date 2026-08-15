import { describe, expect, it } from 'vitest'
import { buildBuiltInCommandTemplates } from '../commandCenterCommandTemplates'

describe('commandCenterCommandTemplates', () => {
  it('builds customer-safe built-in command templates', () => {
    const templates = buildBuiltInCommandTemplates()

    expect(templates.map((template) => template.key)).toEqual(['reboot', 'sync', 'diagnose'])
    expect(templates.map((template) => template.identify)).toEqual([
      'reboot',
      'sync_config',
      'collect_diagnostics'
    ])
    templates.forEach((template) => {
      expect(template.titleKey).toContain('custom.commandCenter.template')
      expect(template.descKey).toContain('custom.commandCenter.template')
      expect(template.timeoutSeconds).toBeGreaterThanOrEqual(60)
      expect(() => JSON.parse(template.value)).not.toThrow()
    })
  })

  it('keeps diagnostic collection broad enough for support handoff', () => {
    const diagnose = buildBuiltInCommandTemplates().find((template) => template.key === 'diagnose')
    expect(diagnose?.identify).toBe('collect_diagnostics')

    const payload = JSON.parse(diagnose?.value || '{}')
    expect(payload).toEqual({
      action: 'collect_diagnostics',
      include: ['status', 'logs', 'telemetry']
    })
  })
})
