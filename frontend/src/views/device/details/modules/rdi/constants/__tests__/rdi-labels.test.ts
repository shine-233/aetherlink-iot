import { describe, expect, it } from 'vitest'

import { labels, systemExtraFieldLabels } from '../rdi-labels'

describe('rdi-labels english copy', () => {
  it('uses customer-facing English labels for the screenshot-aligned RDI field names', () => {
    expect(labels['en-US'].basicInfo).toBe('Basic Info')
    expect(labels['en-US'].statusLabel).toBe('Status')
    expect(labels['en-US'].deviceName).toBe('Device Name')
    expect(labels['en-US'].deviceId).toBe('Device ID')
    expect(labels['en-US'].addedAt).toBe('Added At')
    expect(labels['en-US'].lastHeartbeat).toBe('Last Heartbeat')
    expect(labels['en-US'].configuredParameters).toBe('Configured Parameter Settings')
    expect(labels['en-US'].notification).toBe('Email Notifications')
    expect(labels['en-US'].notificationFallbackHint).toContain('global warning-email recipients')
    expect(labels['en-US'].field).toBe('Global')
    expect(labels['en-US'].currentStatus).toBe('Current Level')
    expect(labels['en-US'].alarmLevel).toBe('Output Level During Alarm')
    expect(labels['en-US'].normalLevel).toBe('Output Level When Not in Alarm')
    expect(labels['en-US'].sensor1).toBe('Temperature Sensor T1')
    expect(labels['en-US'].sensor2).toBe('Temperature Sensor T2')
    expect(labels['en-US'].dryContact).toBe('Output Node 01')
    expect(labels['en-US'].switch1).toBe('Input Node 1')
    expect(labels['en-US'].switch2).toBe('Input Node 2')
    expect((labels['en-US'] as Record<string, string>).switchAlarmLevel).toBe('Alarm Trigger Level')
    expect((labels['en-US'] as Record<string, string>).triggerEffectiveTime).toBe('Trigger Effective Time')
    expect(labels['en-US'].alarmDelay).toBe('Alarm Delay')
    expect(labels['en-US'].normalDelay).toBe('Recovery Delay')
    expect(labels['en-US'].empty).toBe('No data available')
    expect(labels['en-US'].installationAddress).toBe('Installation address')
    expect(labels['en-US'].installationDate).toBe('Installation date')
    expect(labels['en-US'].installerCompany).toBe('Installer company')
    expect(labels['en-US'].installerContact).toBe('Installer contact')
    expect(labels['en-US'].installerName).toBe('Installer name')
    expect(labels['en-US'].installerPhone).toBe('Installer phone')
    expect(labels['en-US'].installerEmail).toBe('Installer email')
    expect(labels['en-US'].controllerSerialNumber).toBe('Controller serial number')
  })
})

describe('rdi-labels localized copy', () => {
  it('keeps key RDI labels localized in French instead of silently falling back to English', () => {
    expect(labels['fr-FR'].basicInfo).toBe('Informations de base')
    expect(labels['fr-FR'].configuredParameters).toBe('Parametres enregistres')
    expect(labels['fr-FR'].empty).toBe('Aucune donnee')
    expect(labels['fr-FR'].change).toBe('Changer')
    expect(labels['fr-FR'].installationAddress).toBe('Adresse d installation')
    expect(labels['fr-FR'].controllerSerialNumber).toBe('Numero de serie du controleur')
    expect(systemExtraFieldLabels['fr-FR'].site_name).toBe('Nom du site')
  })

  it('keeps key RDI labels localized in Spanish instead of silently falling back to English', () => {
    expect(labels['es-ES'].basicInfo).toBe('Informacion basica')
    expect(labels['es-ES'].configuredParameters).toBe('Parametros guardados')
    expect(labels['es-ES'].empty).toBe('Sin datos')
    expect(labels['es-ES'].change).toBe('Cambiar')
    expect(labels['es-ES'].installationAddress).toBe('Direccion de instalacion')
    expect(labels['es-ES'].controllerSerialNumber).toBe('Numero de serie del controlador')
    expect(systemExtraFieldLabels['es-ES'].site_name).toBe('Nombre del sitio')
  })

  it('keeps promoted system information labels localized in Chinese', () => {
    expect(labels['zh-CN'].configuredParameters).toBe('已保存参数设定')
    expect(labels['zh-CN'].installationAddress).toBe('安装地址')
    expect(labels['zh-CN'].installationDate).toBe('安装日期')
    expect(labels['zh-CN'].installerCompany).toBe('安装公司')
    expect(labels['zh-CN'].installerContact).toBe('安装联系人')
    expect(labels['zh-CN'].installerName).toBe('安装人员')
    expect(labels['zh-CN'].installerPhone).toBe('安装电话')
    expect(labels['zh-CN'].installerEmail).toBe('安装邮箱')
    expect(labels['zh-CN'].controllerSerialNumber).toBe('控制器序列号')
  })
})

describe('rdi-labels history failure and sampling-gap copy', () => {
  it('defines explicit English copy for failed, partial and gapped history instead of reusing the empty state', () => {
    expect(labels['en-US'].historyLoadFailed).toBe('History data could not be loaded. Retry these series:')
    expect(labels['en-US'].historyPartialData).toBe('Only part of the history could be loaded for:')
    expect(labels['en-US'].historyGapDetected).toBe('Sampling gaps over 90 seconds were detected in:')
    expect(labels['en-US'].historyGapNotConnected).toBe(
      'The chart intentionally leaves detected gaps unconnected.'
    )
    expect(labels['en-US'].historyLoadFailed).not.toBe(labels['en-US'].empty)
  })

  it('keeps the four history evidence messages localized in Chinese', () => {
    expect(labels['zh-CN'].historyLoadFailed).toBe('历史数据加载失败，请重试以下序列：')
    expect(labels['zh-CN'].historyPartialData).toBe('以下序列只加载到了部分历史数据：')
    expect(labels['zh-CN'].historyGapDetected).toBe('以下序列检测到超过 90 秒的采样缺口：')
    expect(labels['zh-CN'].historyGapNotConnected).toBe('图表会在检测到的缺口处主动断线，不会伪装成连续数据。')
  })

  it('keeps the four history evidence messages localized in French', () => {
    expect(labels['fr-FR'].historyLoadFailed).toBe(
      'Impossible de charger les donnees historiques. Reessayez ces series :'
    )
    expect(labels['fr-FR'].historyPartialData).toBe('Seule une partie de l historique a pu etre chargee pour :')
    expect(labels['fr-FR'].historyGapDetected).toBe(
      'Des interruptions d echantillonnage de plus de 90 secondes ont ete detectees pour :'
    )
    expect(labels['fr-FR'].historyGapNotConnected).toBe(
      'Le graphique laisse volontairement les interruptions detectees non reliees.'
    )
  })

  it('keeps the four history evidence messages localized in Spanish', () => {
    expect(labels['es-ES'].historyLoadFailed).toBe(
      'No se pudieron cargar los datos historicos. Reintente estas series:'
    )
    expect(labels['es-ES'].historyPartialData).toBe('Solo se pudo cargar parte del historial para:')
    expect(labels['es-ES'].historyGapDetected).toBe(
      'Se detectaron interrupciones de muestreo de mas de 90 segundos en:'
    )
    expect(labels['es-ES'].historyGapNotConnected).toBe(
      'El grafico deja intencionadamente sin unir las interrupciones detectadas.'
    )
  })
})
