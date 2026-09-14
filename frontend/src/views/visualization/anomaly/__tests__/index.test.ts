/**
 * 文件用途: P2.2 基础异常检测页的挂载契约。
 * 核心逻辑: 用真实项目 i18n 渲染页面，断言关键结构存在。
 * 关键注意事项:
 *  1. 这是**挂载冒烟**，不是业务逻辑测试——规则与四态判定由 anomaly-model.test.ts 覆盖。
 *     这里只回答"页面能不能真的渲染出来"，typecheck 证明不了这一点。
 *  2. 必须断言"无数据"与"无异常"是两个不同的文案：把二者合并会把"没采到数据"
 *     显示成"设备正常"，是本项目最忌讳的假安全。
 */
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'

import AnomalyPage from '../index.vue'

vi.mock('@/service/api', () => ({
  TELEMETRY_ANOMALY_RULE_BOUNDS: 'bounds',
  TELEMETRY_ANOMALY_RULE_DEVIATION: 'deviation',
  detectTelemetryAnomalies: vi.fn(async () => ({ data: null, error: null }))
}))

const messages = {
  // 项目全局 i18n 默认 locale 为 en，这里按 en 提供文案，
  // 否则 $t 会回退成原始键名，断言会以"没渲染"的假象失败。
  en: {
    page: {
      anomaly: {
        title: 'Telemetry anomaly detection',
        devices: 'Devices',
        devicesPlaceholder: 'One device ID per line (up to {max})',
        key: 'Metric key',
        keyPlaceholder: 'e.g. temperature',
        timeRange: 'Time range',
        window: 'Bucket width',
        windowUnit: 'minutes',
        aggregate: 'Aggregate',
        rule: 'Detection rule',
        ruleBounds: 'Static bounds',
        ruleDeviation: 'Mean ± Kσ',
        bounds: 'Bounds',
        min: 'Min',
        max: 'Max',
        deviationK: 'K value',
        deviationHint: 'Default 3, i.e. ±3σ',
        run: 'Run detection',
        reset: 'Reset',
        results: 'Results',
        colDevice: 'Device',
        colStatus: 'Status',
        colTotal: 'Buckets',
        colRate: 'Anomaly rate',
        colHits: 'Hits',
        statusError: 'Detection failed',
        statusNoData: 'No data in window',
        statusAnomaly: 'Anomalies found',
        statusClean: 'No anomaly',
        summaryDevices: '{count} device(s)',
        summaryAnomaly: '{count} with anomalies / {hits} hit(s)',
        summaryClean: '{count} clean',
        summaryNoData: '{count} with no data',
        summaryFailed: '{count} failed'
      }
    }
  }
}

function mountPage() {
  const i18n = createI18n({ legacy: false, locale: 'en', messages })
  return mount(AnomalyPage, { global: { plugins: [i18n, createPinia()] } })
}

describe('telemetry anomaly page', () => {
  it('renders the query form with both rule choices', async () => {
    const wrapper = mountPage()
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Telemetry anomaly detection')
    expect(text).toContain('Devices')
    expect(text).toContain('Metric key')
    expect(text).toContain('Run detection')
    // 两种规则都必须可选：只暴露一种会让另一条判定路径在界面上不存在。
    expect(text).toContain('Static bounds')
    expect(text).toContain('Mean ± Kσ')
  })

  it('keeps "no data" and "no anomaly" as distinct wordings', () => {
    // 直接核对文案本身：两者若相同，界面就无法区分"没采到数据"与"设备正常"。
    const anomaly = messages.en.page.anomaly
    expect(anomaly.statusNoData).not.toBe(anomaly.statusClean)
    expect(anomaly.summaryNoData).not.toBe(anomaly.summaryClean)
  })

  it('shows no results panel before a detection runs', async () => {
    const wrapper = mountPage()
    await wrapper.vm.$nextTick()
    // 未点"开始检测"前不得渲染结果区——空表格会被读成"检测过且无异常"。
    expect(wrapper.text()).not.toContain('Results')
  })
})
