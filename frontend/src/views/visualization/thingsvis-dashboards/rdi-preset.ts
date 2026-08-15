/**
 * 文件用途: 提供Rdi Preset在可视化场景下使用的配置、常量、组合函数或辅助逻辑。
 * 核心逻辑: 封装页面共享的数据结构、默认值、转换规则或外部依赖适配。
 * 关键注意事项: 这里的字段通常会被页面、测试或接口载荷复用，调整时要同步检查调用方契约。
 * 重构建议: 可按业务语义继续拆分类型、纯函数和副作用逻辑，提升复用性与可测试性。
 */
export type DashboardTemplateType = 'blank' | 'rdi'

type TranslateFn = (...args: [key: string, params?: Record<string, unknown>]) => string

export function getDashboardTemplateOptions(t: TranslateFn) {
  return [
    { label: t('rdi.thingsvis.blankDashboard'), value: 'blank' },
    { label: t('rdi.thingsvis.rdiPreset'), value: 'rdi' }
  ]
}

export function buildRdiDashboardPreset(width: number, height: number, t: TranslateFn) {
  const dataSourceId = 'aetherlink_rdi_device'
  const bind = (field: string) => `{{ ds.${dataSourceId}.data.${field} }}`
  const trendChartFontSizes = {
    title: 16,
    legend: 12,
    axisLabel: 12,
    axisName: 12,
    seriesLabel: 12,
    tooltip: 12
  }
  const card = (id: string, title: string, field: string, x: number, y: number, unit = '') => ({
    id,
    type: 'stat-card',
    name: title,
    x,
    y,
    width: 280,
    height: 150,
    props: {
      title,
      value: bind(field),
      unit,
      theme: 'rdi'
    }
  })

  return {
    canvasConfig: {
      mode: 'fixed',
      width,
      height,
      background: {
        color: '#f8fafc'
      }
    },
    dataSources: [
      {
        id: dataSourceId,
        name: t('rdi.thingsvis.dataSourceName'),
        type: 'PLATFORM_FIELD',
        config: {
          bufferSize: 300,
          requestedFields: [
            'temperature_1',
            'temperature_2',
            'is_online',
            'online_text',
            'device_alarm_count',
            'device_alarm_highest_level',
            'latest_device_alarm_title',
            'electricity_consumption',
            'switch_1',
            'switch_2',
            'dry_contact_output',
            'firmware_version'
          ]
        }
      }
    ],
    variables: [
      {
        id: 'rdi_template_version',
        name: t('rdi.thingsvis.templateVersion'),
        value: '2026-06-19'
      }
    ],
    nodes: [
      {
        id: 'rdi_title',
        type: 'text',
        name: t('rdi.thingsvis.monitoringOverview'),
        x: 48,
        y: 36,
        width: 720,
        height: 64,
        props: {
          text: t('rdi.thingsvis.monitoringOverview'),
          fontSize: 32,
          fontWeight: 700,
          color: '#0f172a'
        }
      },
      card('rdi_temp_1', t('rdi.thingsvis.temperature1'), 'temperature_1', 48, 128, 'C'),
      card('rdi_temp_2', t('rdi.thingsvis.temperature2'), 'temperature_2', 352, 128, 'C'),
      card('rdi_online', t('rdi.thingsvis.onlineStatus'), 'online_text', 656, 128),
      card('rdi_alarm_count', t('rdi.thingsvis.activeAlarms'), 'device_alarm_count', 960, 128),
      card('rdi_energy', t('rdi.thingsvis.energyTotal'), 'electricity_consumption', 1264, 128, 'kWh'),
      card('rdi_firmware', t('rdi.thingsvis.firmwareVersion'), 'firmware_version', 1568, 128),
      {
        id: 'rdi_switch_panel',
        type: 'status-list',
        name: t('rdi.thingsvis.switchContactStatus'),
        x: 48,
        y: 320,
        width: 560,
        height: 260,
        props: {
          title: t('rdi.thingsvis.switchDryContact'),
          items: [
            { label: t('rdi.thingsvis.switch1'), value: bind('switch_1') },
            { label: t('rdi.thingsvis.switch2'), value: bind('switch_2') },
            { label: t('rdi.thingsvis.dryContact'), value: bind('dry_contact_output') }
          ]
        }
      },
      {
        id: 'rdi_alarm_panel',
        type: 'alarm-summary',
        name: t('rdi.thingsvis.alarmSummary'),
        x: 640,
        y: 320,
        width: 560,
        height: 260,
        props: {
          title: t('rdi.thingsvis.alarmSummary'),
          highestLevel: bind('device_alarm_highest_level'),
          latestTitle: bind('latest_device_alarm_title'),
          activeCount: bind('device_alarm_count')
        }
      },
      {
        id: 'rdi_temperature_trend',
        type: 'line-chart',
        name: t('rdi.thingsvis.temperatureTrend'),
        x: 48,
        y: 640,
        width: 1152,
        height: 340,
        props: {
          title: t('rdi.thingsvis.temperatureTrend'),
          dataSourceId,
          fields: ['temperature_1', 'temperature_2'],
          fontSizes: trendChartFontSizes
        }
      },
      {
        id: 'rdi_energy_trend',
        type: 'line-chart',
        name: t('rdi.thingsvis.energyTrend'),
        x: 1240,
        y: 320,
        width: 560,
        height: 660,
        props: {
          title: t('rdi.thingsvis.energyTrend'),
          dataSourceId,
          fields: ['electricity_consumption'],
          fontSizes: trendChartFontSizes
        }
      }
    ]
  }
}
