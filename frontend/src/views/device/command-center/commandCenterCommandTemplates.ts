export type BuiltInCommandTemplate = {
  key: 'reboot' | 'sync' | 'diagnose'
  titleKey: string
  descKey: string
  identify: string
  value: string
  timeoutSeconds: number
  tagType: 'warning' | 'info' | 'success'
}

export function buildBuiltInCommandTemplates(): BuiltInCommandTemplate[] {
  return [
    {
      key: 'reboot',
      titleKey: 'custom.commandCenter.templateRebootTitle',
      descKey: 'custom.commandCenter.templateRebootDesc',
      identify: 'reboot',
      value: JSON.stringify({ action: 'reboot', reason: 'operator_job_template' }, null, 2),
      timeoutSeconds: 120,
      tagType: 'warning'
    },
    {
      key: 'sync',
      titleKey: 'custom.commandCenter.templateSyncTitle',
      descKey: 'custom.commandCenter.templateSyncDesc',
      identify: 'sync_config',
      value: JSON.stringify({ action: 'sync_config', source: 'command_center_template' }, null, 2),
      timeoutSeconds: 90,
      tagType: 'info'
    },
    {
      key: 'diagnose',
      titleKey: 'custom.commandCenter.templateDiagnoseTitle',
      descKey: 'custom.commandCenter.templateDiagnoseDesc',
      identify: 'collect_diagnostics',
      value: JSON.stringify({ action: 'collect_diagnostics', include: ['status', 'logs', 'telemetry'] }, null, 2),
      timeoutSeconds: 180,
      tagType: 'success'
    }
  ]
}
