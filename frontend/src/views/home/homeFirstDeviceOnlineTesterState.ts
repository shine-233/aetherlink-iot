import type {
  FirstDeviceBrowserTestState,
  FirstDeviceChartState,
  FirstDeviceOnboardingGuard,
  FirstDeviceOnlineTesterState
} from './homeFirstDeviceWorkbench'

// Keep the browser-test status precedence separate from the broader workbench
// builders so the deferred connection panel has one focused presentation mapper.
export const buildFirstDeviceOnlineTesterState = (options: {
  guard: FirstDeviceOnboardingGuard
  browserTest?: FirstDeviceBrowserTestState | null
  chart: FirstDeviceChartState
  activeTestCommandLabel?: string
}): FirstDeviceOnlineTesterState => {
  const status = options.browserTest?.status || 'idle'
  const telemetryKey = options.browserTest?.telemetryKey || options.chart.primaryKey
  const telemetryValue = options.browserTest?.telemetryValue || options.chart.primaryValue
  const disabledReason = options.guard.canRunBrowserTest
    ? ''
    : options.guard.activeStep?.description || options.guard.summary || '先补齐连接参数，再运行浏览器在线测试。'
  const echoRows = [
    {
      label: '测试入口',
      value: options.guard.canRunBrowserTest ? '浏览器直接发送一条遥测' : '等待参数补齐'
    },
    {
      label: '测试命令',
      value: options.activeTestCommandLabel || '当前测试命令'
    },
    {
      label: '最后发送',
      value: options.browserTest?.sentAt || '尚未发送'
    },
    {
      label: '回显遥测',
      value: telemetryKey ? `${telemetryKey} = ${telemetryValue || '--'}` : '等待上报确认'
    },
    {
      label: '成功证明',
      value: options.chart.ready ? options.chart.summary : '等待在线、最新遥测和首图变绿'
    }
  ]

  if (status === 'confirmed') {
    return {
      type: 'success',
      statusLabel: '已确认',
      title: '浏览器测试已打通',
      description: options.browserTest?.message || '首页已经用最新遥测确认这条浏览器测试。',
      actionLabel: '再次发送测试',
      disabledReason,
      lastSignal: options.chart.ready ? options.chart.summary : '等待首张图表刷新。',
      echoRows
    }
  }

  if (status === 'failed') {
    return {
      type: 'error',
      statusLabel: '发送失败',
      title: '浏览器测试失败',
      description: options.browserTest?.message || '测试没有发出去，请先打开 Ready Check 查看端点、凭证或网络诊断。',
      actionLabel: '重试测试',
      disabledReason,
      lastSignal: disabledReason || '查看 Ready Check 的连接诊断后再重试。',
      echoRows
    }
  }

  if (status === 'sending') {
    return {
      type: 'info',
      statusLabel: '发送中',
      title: '正在发送测试遥测',
      description: options.browserTest?.message || '浏览器正在把测试遥测发到当前设备接入入口。',
      actionLabel: '发送中',
      disabledReason,
      lastSignal: '等待首页刷新在线状态和最新遥测。',
      echoRows
    }
  }

  if (status === 'sent') {
    return {
      type: 'warning',
      statusLabel: '等待确认',
      title: '测试已发送，等待遥测确认',
      description: options.browserTest?.message || '如果几秒后还没有最新遥测，请打开 Ready Check 查看诊断。',
      actionLabel: '再次发送测试',
      disabledReason,
      lastSignal: options.chart.ready ? options.chart.summary : '等待最新遥测生成第一张图表。',
      echoRows
    }
  }

  return {
    type: options.guard.canRunBrowserTest ? 'info' : 'warning',
    statusLabel: options.guard.canRunBrowserTest ? '可测试' : '待补齐',
    title: options.guard.canRunBrowserTest ? '现在可以直接在线测试' : '在线测试还差一步',
    description: options.guard.canRunBrowserTest
      ? '点一次按钮，浏览器会发送一条测试遥测；成功后首页会自动更新在线、最新遥测和首张图表。'
      : disabledReason,
    actionLabel: '浏览器在线测试',
    disabledReason,
    lastSignal: options.guard.canRunBrowserTest ? '完成标准：回显遥测出现具体 key/value，证明项逐步变绿。' : disabledReason,
    echoRows
  }
}
