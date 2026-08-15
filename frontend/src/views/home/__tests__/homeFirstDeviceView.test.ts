import { describe, expect, it } from 'vitest'

import {
  buildFirstDeviceOperatorCue,
  buildFocusedQuickstartCopy,
  type HomeFirstDeviceGuideStep
} from '../homeFirstDeviceView'

describe('homeFirstDeviceView', () => {
  it('builds a single obvious operator cue for the active first-device step', () => {
    const activeStep: HomeFirstDeviceGuideStep = {
      key: 'publish',
      title: '发送第一条遥测',
      description: '复制测试命令或使用浏览器在线测试，让首页看到第一条遥测。',
      action: '发送测试',
      actionLabel: '浏览器在线测试',
      status: 'active'
    }
    const quickstart = buildFocusedQuickstartCopy({
      ready: false,
      activeStep,
      readyDescription: 'ready',
      guardSummary: '需要发送遥测',
      nextAction: '浏览器在线测试'
    })

    expect(
      buildFirstDeviceOperatorCue({
        ready: false,
        activeStep,
        actionLabel: quickstart.actionLabel,
        successSignal: quickstart.successSignal,
        readyDescription: 'ready'
      })
    ).toMatchObject({
      type: 'info',
      title: '现在只点：浏览器在线测试',
      actionLabel: '浏览器在线测试',
      successSignal: '完成标准：发送测试后，首页开始自动确认最新遥测并生成可视数据。'
    })
  })

  it('turns the operator cue into a handoff when the first device is ready', () => {
    expect(
      buildFirstDeviceOperatorCue({
        ready: true,
        activeStep: null,
        actionLabel: '创建第一条自动化',
        successSignal: '完成标准：设备已准备好。',
        readyDescription: '首台设备已跑通，现在继续做自动化。'
      })
    ).toMatchObject({
      type: 'success',
      title: '现在可以交付这台设备',
      detail: '首台设备已跑通，现在继续做自动化。',
      actionLabel: '创建第一条自动化'
    })
  })
})
