/**
 * 首设备工作台纯函数的稳定入口（barrel）。
 *
 * 实现按关注点拆分，依赖方向单向：
 * - homeFirstDeviceChart.ts       遥测归一化、浏览器测试状态、首图
 * - homeFirstDeviceOnboarding.ts  设备/连接参数归一化、五步快速开始守卫
 * - homeFirstDeviceProof.ts       跑通证明、闭环画布/进度、主操作、首图证明文本、交接
 *
 * 现有调用方（index.vue、composables、测试）继续从这里导入；新代码可以直接导入具体子模块。
 * 子模块之间互相直接导入，不得经由本文件，避免循环依赖。
 */
import { summarizeDeviceConnectionDiagnostics } from '@/views/device/details/modules/device-connection-diagnostics-state'

export * from './homeFirstDeviceChart'
export * from './homeFirstDeviceOnboarding'
export * from './homeFirstDeviceProof'

export { buildFirstDeviceSuccessProofPacket, type FirstDeviceSuccessProofPacket } from './homeFirstDeviceSuccessProof'
export { buildFirstDeviceSupportSummary, type FirstDeviceSupportSummaryOptions } from './homeFirstDeviceSupportSummary'
export { buildFirstDeviceOnlineTesterState } from './homeFirstDeviceOnlineTesterState'

export const summarizeFirstDeviceConnectionDiagnostics = summarizeDeviceConnectionDiagnostics
