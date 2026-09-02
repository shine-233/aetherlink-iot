export type HomeCustomerGuideStepId =
  'setup' | 'deployment' | 'template' | 'device' | 'telemetry' | 'automation' | 'dashboard'

export type HomeCustomerGuideStep = {
  id: HomeCustomerGuideStepId
  title: string
  description: string
  route: string
  action: string
}

export type HomeCustomerGuideStepStatus = 'done' | 'active' | 'todo'

export type HomeCustomerGuideProgressStep = HomeCustomerGuideStep & {
  status: HomeCustomerGuideStepStatus
  statusLabel: string
  statusType: 'success' | 'warning' | 'default'
}

export type HomeCustomerGuideSummary = {
  doneCount: number
  totalCount: number
  percent: number
  nextStep: HomeCustomerGuideProgressStep | null
  headline: string
  description: string
}

export type HomeCustomerGuideProgressInput = {
  setupReady: boolean
  setupStep?: Partial<HomeCustomerGuideStep>
  hasDevice: boolean
  hasTemplate: boolean
  deviceOnline: boolean
  hasTelemetry: boolean
  hasFirstChart: boolean
  hasAutomation: boolean
  hasDashboard: boolean
  deploymentHealthy: boolean
}

export const homeCustomerGuideSteps: HomeCustomerGuideStep[] = [
  {
    id: 'setup',
    title: '完成租户初始化',
    description: '先确认当前账号已经有租户上下文；如果还没有，请创建租户管理员/租户，再接入第一台设备。',
    route: '/management/user?setup=tenant-admin',
    action: '创建租户管理员'
  },
  {
    id: 'deployment',
    title: '确认部署健康',
    description: '先确认前端代理、API、数据库、Redis 和 MQTT Broker 都正常，再开始接入第一台设备。',
    route: '/home?onboarding=first-device&focus=deployment',
    action: '检查部署'
  },
  {
    id: 'template',
    title: '定义物模型',
    description: '把遥测、属性、事件和命令字段建好，后续接入设备前就有统一的数据口径。',
    route: '/device/thingsmodel',
    action: '配置物模型'
  },
  {
    id: 'device',
    title: '接入第一台设备',
    description: '创建设备后复制 MQTT/HTTP 参数，先跑通在线状态和第一条遥测，再复制到批量设备。',
    route: '/device/manage?onboarding=first-device&add=manual',
    action: '添加设备'
  },
  {
    id: 'telemetry',
    title: '看到第一张图表',
    description: '设备有数据后，先看最新遥测和历史趋势，确认现场状态能被平台正确理解。',
    route: '/device/manage?onboarding=first-device',
    action: '打开 Ready Check'
  },
  {
    id: 'automation',
    title: '配置自动化',
    description: '把常见处置动作做成联动规则，例如异常告警、下发命令或通知人员。',
    route: '/automation/linkage-edit?backType=automation&onboarding=first-device',
    action: '新建联动规则'
  },
  {
    id: 'dashboard',
    title: '做客户看板',
    description: '把关键指标、设备状态和趋势图放到 ThingsVis，看板可作为客户交付入口。',
    route: '/visualization/thingsvis?onboarding=first-device',
    action: '创建首页看板'
  }
]

const isGuideStepComplete = (step: HomeCustomerGuideStep, input: HomeCustomerGuideProgressInput) => {
  switch (step.id) {
    case 'setup':
      return input.setupReady
    case 'deployment':
      return input.deploymentHealthy
    case 'template':
      return input.hasTemplate
    case 'device':
      return input.hasDevice && input.deviceOnline
    case 'telemetry':
      return input.hasTelemetry && input.hasFirstChart
    case 'automation':
      return input.hasAutomation
    case 'dashboard':
      return input.hasDashboard
    default:
      return false
  }
}

export const buildHomeCustomerGuideProgress = (
  input: HomeCustomerGuideProgressInput
): HomeCustomerGuideProgressStep[] => {
  let activeAssigned = false

  return homeCustomerGuideSteps.map((step) => {
    const resolvedStep =
      step.id === 'setup' && input.setupStep
        ? {
            ...step,
            ...input.setupStep
          }
        : step.id === 'template' && !input.hasTemplate
          ? {
              ...step,
              description:
                '不用先离开首页配置物模型；留在“接入第一台设备”工作台，一键生成时会同时创建默认产品、物模型、MQTT/HTTP 配置和第一台设备。',
              route: '/home?onboarding=first-device&focus=device',
              action: '生成默认物模型和设备'
            }
          : step.id === 'device' && !input.hasDevice
            ? {
                ...step,
                description:
                  '留在首页即可一键生成产品、物模型、MQTT/HTTP 配置和第一台设备；生成后继续在这里复制参数并发送第一条遥测。',
                route: '/home?onboarding=first-device&focus=device',
                action: '一键生成设备'
              }
            : step.id === 'device' && input.hasDevice && !input.deviceOnline
              ? {
                  ...step,
                  description: '已经有第一台设备；现在留在首页继续复制参数、浏览器在线测试和在线状态确认。',
                  route: '/home?onboarding=first-device&focus=test',
                  action: '继续接入'
                }
              : step.id === 'telemetry' && !input.hasTelemetry
                ? {
                    ...step,
                    description: '回到首页第 3 步，复制命令或直接点击浏览器在线测试，先发出第一条可见遥测。',
                    route: '/home?onboarding=first-device&focus=test',
                    action: '发送第一条遥测'
                  }
                : step.id === 'telemetry' && input.hasTelemetry && !input.hasFirstChart
                  ? {
                      ...step,
                      description: '最新遥测已经到了，留在首页第 4 步确认第一张图表和可交付状态。',
                      route: '/home?onboarding=first-device&focus=chart',
                      action: '查看第一张图表'
                    }
                  : step
    const complete = isGuideStepComplete(step, input)
    if (complete) {
      return {
        ...resolvedStep,
        status: 'done',
        statusLabel: '已完成',
        statusType: 'success'
      }
    }

    if (!activeAssigned) {
      activeAssigned = true
      return {
        ...resolvedStep,
        status: 'active',
        statusLabel: '当前要做',
        statusType: 'warning'
      }
    }

    return {
      ...resolvedStep,
      status: 'todo',
      statusLabel: '待处理',
      statusType: 'default'
    }
  })
}

export const buildHomeCustomerGuideSummary = (steps: HomeCustomerGuideProgressStep[]): HomeCustomerGuideSummary => {
  const totalCount = steps.length
  const doneCount = steps.filter((step) => step.status === 'done').length
  const nextStep = steps.find((step) => step.status === 'active') ?? null
  const percent = totalCount > 0 ? Math.round((doneCount / totalCount) * 100) : 0

  if (!nextStep) {
    return {
      doneCount,
      totalCount,
      percent: 100,
      nextStep,
      headline: '首次接入闭环已完成',
      description: '部署、物模型、设备、遥测、自动化和看板都已具备，可以复制到批量设备或交付客户看板。'
    }
  }

  return {
    doneCount,
    totalCount,
    percent,
    nextStep,
    headline: `下一步：${nextStep.title}`,
    description: nextStep.description
  }
}
