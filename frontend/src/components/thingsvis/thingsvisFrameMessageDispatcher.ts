/**
 * 文件说明：
 * - 负责 ThingsVis iframe 可信消息的类型分发，把 `type -> handler` 的路由表从 AppFrame 中拆出来。
 * - 模块只保存消息协议映射，不直接访问 iframe、window、接口请求或 Vue props，避免把宿主副作用混进分发层。
 * 维护提示：
 * - 新增 `tv:*` 或 `thingsvis:*` 消息时优先在这里登记类型，再把真实业务动作通过 actions 注入。
 * - `tv:ready` 和历史兼容 `READY` 必须指向同一初始化动作，不能拆成两套调度逻辑。
 * 审查建议：
 * - 后续可为本模块补纯单元测试：注入 spy actions 后验证每个消息类型只调用对应 handler。
 */
import type { TrustedGuestMessage } from '@/components/thingsvis/hostBridge'

export type TrustedThingsVisFrameMessage = TrustedGuestMessage<Record<string, unknown>>
export type ThingsVisFrameMessageHandler = (message: TrustedThingsVisFrameMessage) => void | Promise<void>

type DashboardMessageActions = {
  save: (payload: Record<string, unknown>) => void | Promise<void>
  platformWrite: (payload: Record<string, unknown>, requestId?: string) => void | Promise<void>
  preview: (projectId: unknown) => void | Promise<void>
  publish: (projectId: unknown) => void | Promise<void>
}

type DeviceMessageActions = {
  requestFieldData: (payload: Record<string, unknown>) => void | Promise<void>
  requestDeviceGroups: () => void | Promise<void>
  requestDeviceFilterOptions: (payload: Record<string, unknown>) => void | Promise<void>
  requestDeviceById: (payload: Record<string, unknown>) => void | Promise<void>
  requestDevicesByGroup: (payload: Record<string, unknown>) => void | Promise<void>
  searchDevicesPaged: (payload: Record<string, unknown>) => void | Promise<void>
  requestDeviceFields: (payload: Record<string, unknown>) => void | Promise<void>
}

type LifecycleMessageActions = {
  loaded: () => void | Promise<void>
  ready: () => void | Promise<void>
  requestInit: () => void | Promise<void>
  contentHeight: (payload: Record<string, unknown>, raw: Record<string, unknown>) => void | Promise<void>
}

export type ThingsVisFrameMessageDispatcherActions = {
  dashboard: DashboardMessageActions
  device: DeviceMessageActions
  lifecycle: LifecycleMessageActions
}

export type ThingsVisFrameMessageDispatcher = {
  dispatch: (message: TrustedThingsVisFrameMessage) => Promise<void>
  hasHandler: (type: string) => boolean
}

function buildDashboardFrameMessageHandlers(
  actions: DashboardMessageActions
): Record<string, ThingsVisFrameMessageHandler> {
  return {
    'tv:save': ({ payload }) => actions.save(payload),
    'tv:platform-write': ({ payload, requestId }) => actions.platformWrite(payload, requestId),
    'tv:preview': ({ projectId }) => actions.preview(projectId),
    'tv:publish': ({ projectId }) => actions.publish(projectId)
  }
}

function buildDeviceFrameMessageHandlers(actions: DeviceMessageActions): Record<string, ThingsVisFrameMessageHandler> {
  return {
    'thingsvis:requestFieldData': ({ payload }) => actions.requestFieldData(payload),
    'thingsvis:requestDeviceGroups': () => actions.requestDeviceGroups(),
    'thingsvis:requestDeviceFilterOptions': ({ payload }) => actions.requestDeviceFilterOptions(payload),
    'thingsvis:requestDeviceById': ({ payload }) => actions.requestDeviceById(payload),
    'thingsvis:requestDevicesByGroup': ({ payload }) => actions.requestDevicesByGroup(payload),
    'thingsvis:searchDevicesPaged': ({ payload }) => actions.searchDevicesPaged(payload),
    'thingsvis:requestDeviceFields': ({ payload }) => actions.requestDeviceFields(payload)
  }
}

function buildLifecycleFrameMessageHandlers(
  actions: LifecycleMessageActions
): Record<string, ThingsVisFrameMessageHandler> {
  return {
    LOADED: () => actions.loaded(),
    'tv:ready': () => actions.ready(),
    READY: () => actions.ready(),
    'tv:request-init': () => actions.requestInit(),
    'tv:content-height': ({ payload, raw }) => actions.contentHeight(payload, raw),
    'thingsvis:content-height': ({ payload, raw }) => actions.contentHeight(payload, raw),
    'tv:resize': ({ payload, raw }) => actions.contentHeight(payload, raw),
    'thingsvis:resize': ({ payload, raw }) => actions.contentHeight(payload, raw)
  }
}

export function createThingsVisFrameMessageDispatcher(
  actions: ThingsVisFrameMessageDispatcherActions
): ThingsVisFrameMessageDispatcher {
  const handlers: Record<string, ThingsVisFrameMessageHandler> = {
    ...buildDashboardFrameMessageHandlers(actions.dashboard),
    ...buildDeviceFrameMessageHandlers(actions.device),
    ...buildLifecycleFrameMessageHandlers(actions.lifecycle)
  }

  async function dispatch(message: TrustedThingsVisFrameMessage) {
    const handler = handlers[message.type]
    if (!handler) return

    await handler(message)
  }

  return {
    dispatch,
    hasHandler: (type: string) => Boolean(handlers[type])
  }
}
