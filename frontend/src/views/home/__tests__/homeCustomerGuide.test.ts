import { describe, expect, it } from 'vitest'
import {
  buildHomeCustomerGuideProgress,
  buildHomeCustomerGuideSummary,
  homeCustomerGuideSteps
} from '../homeCustomerGuide'

describe('homeCustomerGuide', () => {
  it('starts with tenant setup before deployment and device onboarding', () => {
    expect(homeCustomerGuideSteps.map((step) => step.id)).toEqual([
      'setup',
      'deployment',
      'template',
      'device',
      'telemetry',
      'automation',
      'dashboard'
    ])

    const steps = buildHomeCustomerGuideProgress({
      setupReady: false,
      deploymentHealthy: false,
      hasTemplate: false,
      hasDevice: false,
      deviceOnline: false,
      hasTelemetry: false,
      hasFirstChart: false,
      hasAutomation: false,
      hasDashboard: false
    })

    expect(steps[0]).toMatchObject({ id: 'setup', status: 'active', action: '创建租户管理员' })
    expect(steps[1]).toMatchObject({ id: 'deployment', status: 'todo' })
  })

  it('makes deployment health active after tenant setup is ready', () => {
    const steps = buildHomeCustomerGuideProgress({
      setupReady: true,
      deploymentHealthy: false,
      hasTemplate: false,
      hasDevice: false,
      deviceOnline: false,
      hasTelemetry: false,
      hasFirstChart: false,
      hasAutomation: false,
      hasDashboard: false
    })

    expect(steps[0]).toMatchObject({ id: 'setup', status: 'done' })
    expect(steps[1]).toMatchObject({ id: 'deployment', status: 'active', action: '检查部署' })
  })

  it('routes the telemetry step to first-device verification instead of alarm triage', () => {
    const telemetryStep = homeCustomerGuideSteps.find((step) => step.id === 'telemetry')

    expect(telemetryStep).toMatchObject({
      route: '/device/manage?onboarding=first-device',
      action: '打开 Ready Check'
    })
  })

  it('marks the first unfinished onboarding step as active', () => {
    const steps = buildHomeCustomerGuideProgress({
      setupReady: true,
      deploymentHealthy: true,
      hasTemplate: true,
      hasDevice: true,
      deviceOnline: false,
      hasTelemetry: false,
      hasFirstChart: false,
      hasAutomation: false,
      hasDashboard: false
    })

    expect(steps.map((step) => step.status)).toEqual(['done', 'done', 'done', 'active', 'todo', 'todo', 'todo'])
    expect(buildHomeCustomerGuideSummary(steps)).toMatchObject({
      doneCount: 3,
      totalCount: 7,
      percent: 43,
      nextStep: expect.objectContaining({ id: 'device' })
    })
  })

  it('keeps automation active after the first chart until a rule exists', () => {
    const steps = buildHomeCustomerGuideProgress({
      setupReady: true,
      deploymentHealthy: true,
      hasTemplate: true,
      hasDevice: true,
      deviceOnline: true,
      hasTelemetry: true,
      hasFirstChart: true,
      hasAutomation: false,
      hasDashboard: false
    })

    expect(steps.map((step) => step.status)).toEqual(['done', 'done', 'done', 'done', 'done', 'active', 'todo'])
    expect(buildHomeCustomerGuideSummary(steps)).toMatchObject({
      percent: 71,
      nextStep: expect.objectContaining({ id: 'automation' })
    })
  })

  it('summarizes a completed first-run loop', () => {
    const steps = buildHomeCustomerGuideProgress({
      setupReady: true,
      deploymentHealthy: true,
      hasTemplate: true,
      hasDevice: true,
      deviceOnline: true,
      hasTelemetry: true,
      hasFirstChart: true,
      hasAutomation: true,
      hasDashboard: true
    })

    expect(steps.every((step) => step.status === 'done')).toBe(true)
    expect(buildHomeCustomerGuideSummary(steps)).toMatchObject({
      percent: 100,
      nextStep: null,
      headline: '首次接入闭环已完成'
    })
  })
})
