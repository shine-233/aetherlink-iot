/**
 * 文件用途: RDI 操作视图的状态装配与 provide/inject 上下文。
 * 核心逻辑: 统一组合 useRdiConfig / useRdiTelemetry / useRdiHistory / useRdiCommands / useRdiShare /
 * useRdiDeviceBasicInfo / useRdiOnDemandLoads，产出一个类型化上下文供 RdiDeviceOperationsView 各分区子组件注入，
 * 替代原先单文件 1000+ 行模板里对 60+ 个解构变量的直接引用。
 * 关键注意事项:
 * 1. 只允许 RdiDeviceOperationsView 调用 provideRdiOperationsContext；分区子组件只调用 useRdiOperationsContext。
 * 2. 各 composable 的调用顺序与依赖（temperatureUnit -> history，config -> commands）与原视图保持一致。
 * 3. 纯派生（下拉选项、干接点延时折叠）拆成导出的纯函数，便于单测。
 */
import { computed, inject, provide } from 'vue'
import type { InjectionKey } from 'vue'
import type { RDIConfig } from '@/service/api/rdi'
import type { LabelKey } from '../constants/rdi-labels'
import { useRdiConfig } from './useRdiConfig'
import { useRdiTelemetry } from './useRdiTelemetry'
import { useRdiHistory } from './useRdiHistory'
import { useRdiCommands } from './useRdiCommands'
import { useRdiShare } from './useRdiShare'
import { useRdiDeviceBasicInfo } from './useRdiDeviceBasicInfo'
import { useRdiOnDemandLoads } from './useRdiOnDemandLoads'

type Translate = (key: LabelKey) => string

export type RdiOperationsSource = {
  id: () => string
  online: () => number | undefined
  onlineUpdatedAt: () => string | undefined
  deviceData: () => Record<string, any> | undefined
  onChange: () => void
}

export function buildSwitchModeOptions(t: Translate) {
  return [
    { label: t('poweredOn'), value: 'powered_on' },
    { label: t('poweredOff'), value: 'powered_off' },
    { label: t('disabled'), value: 'disabled' }
  ]
}

export function buildTemperatureUnitOptions(t: Translate) {
  return [
    { label: `${t('celsius')} (C)`, value: 'C' },
    { label: `${t('fahrenheit')} (F)`, value: 'F' }
  ]
}

export function buildLevelOptions(t: Translate) {
  return [
    { label: t('high'), value: 'high' },
    { label: t('low'), value: 'low' }
  ]
}

/** 干接点告警/恢复延时是否不同；相同时 UI 折叠为单个“触发生效时间”字段。 */
export function hasDistinctDryContactDelays(
  config: Pick<RDIConfig, 'dry_contact_alarm_delay' | 'dry_contact_normal_delay'>
) {
  return config.dry_contact_alarm_delay !== config.dry_contact_normal_delay
}

/** 折叠模式下写入统一延时：告警与恢复延时同步修改。 */
export function writeUnifiedDryContactDelay(
  config: Pick<RDIConfig, 'dry_contact_alarm_delay' | 'dry_contact_normal_delay'>,
  value: number
) {
  config.dry_contact_alarm_delay = value
  config.dry_contact_normal_delay = value
}

export function createRdiOperationsState(source: RdiOperationsSource) {
  const configState = useRdiConfig(source.id, source.onChange)
  const { t, config } = configState
  const telemetryState = useRdiTelemetry(source.id, source.online, source.deviceData, t)
  const historyState = useRdiHistory(source.id, () => telemetryState.temperatureUnit.value, t)
  const commandState = useRdiCommands(source.id, config, t)
  const shareState = useRdiShare(source.id, t)

  const basicInfo = useRdiDeviceBasicInfo({
    deviceId: source.id,
    online: source.online,
    onlineUpdatedAt: source.onlineUpdatedAt,
    deviceData: source.deviceData,
    liveOnlineStatus: telemetryState.liveOnlineStatus,
    deviceOnlineText: telemetryState.deviceOnlineText,
    deviceDescriptionText: telemetryState.deviceDescriptionText,
    t
  })

  const loads = useRdiOnDemandLoads({
    deviceId: source.id,
    loadConfig: configState.loadConfig,
    loadRealtimeState: telemetryState.loadRealtimeState,
    loadEnergyStatistics: historyState.loadEnergyStatistics,
    loadOtaPackages: commandState.loadOtaPackages,
    otaPackageLoading: commandState.otaPackageLoading,
    resetShareState: shareState.resetShareState,
    liveOnlineStatus: telemetryState.liveOnlineStatus,
    startTelemetryRefresh: telemetryState.startTelemetryRefresh
  })

  const options = {
    switchMode: computed(() => buildSwitchModeOptions(t)),
    temperatureUnit: computed(() => buildTemperatureUnitOptions(t)),
    level: computed(() => buildLevelOptions(t))
  }

  const dryContact = {
    hasDistinctDelays: computed(() => hasDistinctDryContactDelays(config)),
    unifiedDelay: computed({
      get: () => config.dry_contact_alarm_delay,
      set: (value: number) => writeUnifiedDryContactDelay(config, value)
    })
  }

  return {
    source,
    t,
    config: configState,
    telemetry: telemetryState,
    history: historyState,
    commands: commandState,
    share: shareState,
    basicInfo,
    loads,
    options,
    dryContact
  }
}

export type RdiOperationsState = ReturnType<typeof createRdiOperationsState>

const RDI_OPERATIONS_KEY: InjectionKey<RdiOperationsState> = Symbol('rdi-operations')

export function provideRdiOperationsContext(source: RdiOperationsSource) {
  const state = createRdiOperationsState(source)
  provide(RDI_OPERATIONS_KEY, state)
  return state
}

export function useRdiOperationsContext(): RdiOperationsState {
  const state = inject(RDI_OPERATIONS_KEY, null)
  if (!state) {
    throw new Error('useRdiOperationsContext must be used inside RdiDeviceOperationsView')
  }
  return state
}
