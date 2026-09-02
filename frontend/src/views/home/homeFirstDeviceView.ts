import type { HomeFirstRunProtocol } from './homeFirstRunWizard'

export type HomeFirstDeviceGuideStep = {
  id?: string
  key?: string
  route?: string
  status?: string
  statusType?: string
  statusLabel?: string
  title?: string
  description?: string
  action?: string
  actionLabel?: string
  disabled?: boolean
  detail?: string
}

export type HomeFirstDeviceCoreGuideSummary = {
  doneCount: number
  totalCount: number
  percent: number
  nextStep: HomeFirstDeviceGuideStep | null
  headline: string
  description: string
}

export type FirstDeviceOperatorCue = {
  type: 'success' | 'warning' | 'info'
  title: string
  detail: string
  actionLabel: string
  successSignal: string
}

export type FirstDeviceMissionControl = {
  type: 'success' | 'warning' | 'info'
  currentStateLabel: string
  nextClickLabel: string
  whyThisClick: string
  finishSignal: string
  stuckHint: string
  supportLabel: string
}

export const FIRST_DEVICE_CORE_GUIDE_IDS = ['setup', 'deployment', 'template', 'device', 'telemetry']
export const FIRST_DEVICE_NEXT_GUIDE_IDS = ['automation', 'dashboard']

export const isHomeGuideStepActionDisabled = (step: HomeFirstDeviceGuideStep) => step.status === 'todo'

export const getHomeGuideStepActionLabel = (step: HomeFirstDeviceGuideStep) =>
  isHomeGuideStepActionDisabled(step) ? '等待上一步' : step.action

export const filterFirstDeviceCoreGuideSteps = (steps: HomeFirstDeviceGuideStep[]) =>
  steps.filter((step) => FIRST_DEVICE_CORE_GUIDE_IDS.includes(String(step.id || '')))

export const filterFirstDeviceNextGuideSteps = (steps: HomeFirstDeviceGuideStep[]) =>
  steps.filter((step) => FIRST_DEVICE_NEXT_GUIDE_IDS.includes(String(step.id || '')))

export const buildFirstDeviceCoreGuideSummary = (
  steps: HomeFirstDeviceGuideStep[]
): HomeFirstDeviceCoreGuideSummary => {
  const totalCount = steps.length
  const doneCount = steps.filter((step) => step.status === 'done').length
  const nextStep = steps.find((step) => step.status === 'active') || null
  const percent = totalCount > 0 ? Math.round((doneCount / totalCount) * 100) : 0

  return {
    doneCount,
    totalCount,
    percent: nextStep ? percent : 100,
    nextStep,
    headline: nextStep ? `先做：${nextStep.title}` : '首台设备接入闭环已完成',
    description: nextStep ? nextStep.description || '' : '第一台设备已经跑通，下面可以继续做自动化和客户看板。'
  }
}

export const shouldExpandHomeGuideStep = (step: HomeFirstDeviceGuideStep, summary: HomeFirstDeviceCoreGuideSummary) =>
  step.status === 'active' || (!summary.nextStep && step.status === 'done')

export const buildFirstRunWizardEvidence = (
  step: HomeFirstDeviceGuideStep,
  options: {
    setupBlockerDescription: string
    deploymentHealthOk: boolean
    firstFailedDeploymentHealthRow?: any
    firstDevice?: any
    firstRunProtocol: HomeFirstRunProtocol
    firstDeviceChart: any
    firstDeviceTestResult: string
  }
) => {
  if (step.id === 'setup') {
    return step.status === 'done' ? '账号 / 租户上下文已就绪' : options.setupBlockerDescription
  }
  if (step.id === 'deployment') {
    return options.deploymentHealthOk
      ? '前端、API、DB、Redis、MQTT 已可支撑接入'
      : options.firstFailedDeploymentHealthRow?.error ||
          options.firstFailedDeploymentHealthRow?.description ||
          '等待部署健康检查通过'
  }
  if (step.id === 'template') {
    return options.firstDevice?.configName
      ? `物模型：${options.firstDevice.configName}`
      : `一键生成会创建默认物模型 / ${options.firstRunProtocol} 配置`
  }
  if (step.id === 'device') {
    return options.firstDevice
      ? `${options.firstDevice.name || options.firstDevice.number || '第一台设备'} · ${
          options.firstDevice.online ? '在线' : '等待上线'
        }`
      : '尚未生成第一台设备'
  }
  if (step.id === 'telemetry') {
    if (options.firstDeviceChart.ready) return `首图已生成：${options.firstDeviceChart.primaryKey || 'telemetry'}`
    if (options.firstDeviceTestResult) return options.firstDeviceTestResult
    return '等待发送第一条可见遥测'
  }
  return step.description || ''
}

export const buildFirstRunWizardSteps = (
  steps: HomeFirstDeviceGuideStep[],
  options: Parameters<typeof buildFirstRunWizardEvidence>[1]
) =>
  steps.map((step, index) => ({
    ...step,
    order: index + 1,
    evidence: buildFirstRunWizardEvidence(step, options),
    actionLabel: step.status === 'done' ? '查看' : step.status === 'active' ? step.action : '等待'
  }))

export const buildFocusedQuickstartCopy = (options: {
  ready: boolean
  activeStep?: HomeFirstDeviceGuideStep | null
  readyDescription: string
  guardSummary: string
  nextAction?: string
  postReadyHandoff?: { completionSignal?: string; primaryLabel?: string } | null
}) => {
  const key = options.activeStep?.key
  const title = options.ready ? '首台设备已准备好' : options.activeStep?.title || '现在只做这一步'
  const description = options.ready ? options.readyDescription : options.activeStep?.description || options.guardSummary
  const successSignal = options.ready
    ? options.postReadyHandoff?.completionSignal ||
      '完成标准：第 5 步显示“设备已准备好”，并且在线状态与最新遥测都通过。'
    : key === 'health'
      ? '完成标准：部署健康里的前端、API、数据库、Redis、MQTT Broker 全部恢复为正常。'
      : key === 'create'
        ? '完成标准：首页出现设备编号、连接参数和可复制测试命令。'
        : key === 'connect'
          ? '完成标准：第 2 步里的连接参数完整可见，复制命令里不再有占位符。'
          : key === 'publish'
            ? '完成标准：发送测试后，首页开始自动确认最新遥测并生成可视数据。'
            : key === 'verify'
              ? '完成标准：第 4 步看到最新遥测图表，第 5 步里的在线状态和证明项逐步变绿。'
              : '完成标准：沿着当前高亮步骤继续，直到首页出现“设备已准备好”。'
  const actionLabel = options.ready
    ? options.postReadyHandoff?.primaryLabel || '查看完整接入指南'
    : options.activeStep?.actionLabel || options.nextAction || '继续下一步'

  return {
    title,
    description,
    successSignal,
    actionLabel,
    actionDisabled: options.ready ? false : Boolean(options.activeStep?.disabled)
  }
}

export const buildFirstDeviceOperatorCue = (options: {
  ready: boolean
  activeStep?: HomeFirstDeviceGuideStep | null
  actionLabel: string
  successSignal: string
  readyDescription: string
  currentBlocker?: { label?: string; detail?: string } | null
}): FirstDeviceOperatorCue => {
  if (options.ready) {
    return {
      type: 'success',
      title: '现在可以交付这台设备',
      detail: options.readyDescription,
      actionLabel: options.actionLabel || '继续下一步',
      successSignal: options.successSignal
    }
  }

  const activeStep = options.activeStep
  if (activeStep) {
    return {
      type: activeStep.disabled ? 'warning' : 'info',
      title: `现在只点：${options.actionLabel || activeStep.action || activeStep.title || '继续下一步'}`,
      detail: activeStep.description || options.currentBlocker?.detail || '按当前高亮步骤继续，不需要先理解其它模块。',
      actionLabel: options.actionLabel || activeStep.action || '继续下一步',
      successSignal: options.successSignal
    }
  }

  return {
    type: options.currentBlocker ? 'warning' : 'info',
    title: options.currentBlocker?.label ? `先处理：${options.currentBlocker.label}` : '等待首页确认下一步',
    detail: options.currentBlocker?.detail || '刷新后首页会继续把当前卡点高亮出来。',
    actionLabel: options.actionLabel || '刷新进度',
    successSignal: options.successSignal
  }
}

export const buildFirstDeviceMissionControl = (options: {
  ready: boolean
  activeStep?: HomeFirstDeviceGuideStep | null
  actionLabel: string
  successSignal: string
  readyDescription: string
  currentBlocker?: { label?: string; detail?: string } | null
}): FirstDeviceMissionControl => {
  if (options.ready) {
    return {
      type: 'success',
      currentStateLabel: '首台设备闭环已完成',
      nextClickLabel: options.actionLabel || '继续交付下一步',
      whyThisClick: options.readyDescription,
      finishSignal: options.successSignal,
      stuckHint: '如果现场仍有疑问，直接复制支持摘要，里面已经带上设备、遥测、健康检查和下一步建议。',
      supportLabel: '复制支持摘要'
    }
  }

  const activeStep = options.activeStep
  if (activeStep) {
    const nextClickLabel = options.actionLabel || activeStep.action || activeStep.title || '继续下一步'
    return {
      type: activeStep.disabled ? 'warning' : 'info',
      currentStateLabel: activeStep.title ? `当前阶段：${activeStep.title}` : '首台设备接入进行中',
      nextClickLabel,
      whyThisClick:
        activeStep.description ||
        options.currentBlocker?.detail ||
        '首页已经判断出当前最短路径，先完成这一步，不需要理解其它模块。',
      finishSignal: options.successSignal,
      stuckHint: activeStep.disabled
        ? '按钮暂时不可点时，先定位当前工作区，看缺的是部署健康、连接参数还是上一环证据。'
        : '点了没变化时，定位当前工作区；仍卡住就复制支持摘要给工程支持。',
      supportLabel: '卡住了，复制摘要'
    }
  }

  return {
    type: options.currentBlocker ? 'warning' : 'info',
    currentStateLabel: options.currentBlocker?.label
      ? `当前卡点：${options.currentBlocker.label}`
      : '等待首页确认下一步',
    nextClickLabel: options.actionLabel || '刷新进度',
    whyThisClick: options.currentBlocker?.detail || '首页还在等待最新状态，刷新后会继续把当前卡点高亮出来。',
    finishSignal: options.successSignal,
    stuckHint: '刷新后仍没有下一步时，复制支持摘要，让支持人员直接看到当前部署和首设备证据。',
    supportLabel: '复制支持摘要'
  }
}

export const getFocusedQuickstartActionLoading = (options: {
  ready: boolean
  activeStep?: HomeFirstDeviceGuideStep | null
  firstDeviceActionLoading: boolean
  deploymentHealthLoading: boolean
  firstRunCreateLoading: boolean
}) => {
  if (options.ready) return false
  const action = options.activeStep?.action
  if (action === 'test') return options.firstDeviceActionLoading
  if (action === 'health') return options.deploymentHealthLoading
  if (action === 'create') return options.firstRunCreateLoading
  return false
}

export const buildFirstDeviceStatusHeroCopy = (options: {
  ready: boolean
  currentBlocker?: any
  activeStep?: HomeFirstDeviceGuideStep | null
  guardSummary: string
}) => {
  if (options.ready) {
    return {
      title: '首页已拿到接入成功证明',
      description: '这台设备已经在线，首页也已经看到了最新遥测、首张图表和最终证明项。'
    }
  }
  if (options.currentBlocker) {
    return {
      title: `当前卡点：${options.currentBlocker.label}`,
      description: options.currentBlocker.detail || options.guardSummary
    }
  }
  return {
    title: options.activeStep?.title ? `当前阶段：${options.activeStep.title}` : '首台设备接入进行中',
    description: options.guardSummary
  }
}

export const buildFirstDeviceSuccessProofCopy = (options: {
  ready: boolean
  chartReady: boolean
  testResult: string
}) => {
  if (options.ready) {
    return {
      title: '设备已准备好',
      description: '首页已经确认在线状态、最新遥测、第一张图表和成功证明项，这台设备现在可以视为接入完成。'
    }
  }
  if (options.chartReady) {
    return {
      title: '正在生成接入成功证明',
      description: '图表已经出来了，现在只要右侧证明项继续变绿，首页就会把这台设备标记为已准备好。'
    }
  }
  if (options.testResult) {
    return {
      title: '等待接入成功证明',
      description: '测试已经发送或正在确认；一旦首页读到最新遥测，这里会自动生成首张图表并更新成功证明。'
    }
  }
  return {
    title: '等待接入成功证明',
    description: '先完成上面的参数和发送测试；这里会在收到数据后自动汇总图表、在线状态和最终成功证明。'
  }
}

export const buildFirstDeviceTestCommands = (options: { accessGuide?: any; publishCommand: string }) => {
  const guideCommands = options.accessGuide?.commands || []
  if (guideCommands.length) return guideCommands
  if (!options.publishCommand) return []

  return [
    {
      titleKey: '命令行',
      language: 'bash',
      code: options.publishCommand
    }
  ]
}

export const getFirstDeviceTestCommandLabel = (command: any) => {
  const language = String(command.language || '').toLowerCase()
  if (language === 'bash') return 'CLI'
  if (language === 'javascript') return 'Node.js'
  if (language === 'python') return 'Python'
  if (language === 'c') return 'C'
  return command.titleKey || language || '测试命令'
}

export const buildFirstDeviceOperationChecklist = (options: {
  canCopyCommand: boolean
  activeTestCommand?: any
  canRunBrowserTest: boolean
  deploymentHealthOk: boolean
}) => [
  {
    key: 'params',
    label: '连接参数',
    ok: options.canCopyCommand,
    detail: options.canCopyCommand
      ? '参数已完整，可以直接复制命令或测试代码。'
      : '还存在占位符或缺少端点，请先补齐连接参数。'
  },
  {
    key: 'test',
    label: '发布测试遥测',
    ok: Boolean(options.activeTestCommand?.code),
    detail: options.activeTestCommand
      ? `当前默认测试命令：${getFirstDeviceTestCommandLabel(options.activeTestCommand)}`
      : '正在等待生成可复制测试命令。'
  },
  {
    key: 'browser',
    label: '浏览器测试',
    ok: options.canRunBrowserTest,
    detail: options.canRunBrowserTest
      ? '当前浏览器可以直接发一条测试遥测。'
      : options.deploymentHealthOk
        ? '还需要先补齐参数或让设备进入可测试状态。'
        : '先把部署健康恢复为正常，再运行浏览器测试。'
  }
]
