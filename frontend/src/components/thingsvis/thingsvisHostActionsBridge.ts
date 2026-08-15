/**
 * 文件说明：
 * - 封装 ThingsVis iframe 宿主动作，包括保存 dashboard、打开预览页和发布 dashboard。
 * - 将 AppFrame 中的路由跳转、发布提示、保存 payload 构造和接口调用集中到一个小接口后面。
 * 维护提示：
 * - `tv:save`、`tv:preview`、`tv:publish` 属于 iframe 到宿主的跨系统契约，响应行为要与 ThingsVis guest 侧保持兼容。
 * - 保存前仍复用 `thingsvisHostSaveBridge.ts` 的数据源清理规则，避免把宿主自动生成的数据源误写回平台。
 * 审查建议：
 * - 后续如果补测试，可用注入的内存 adapter 覆盖 save/preview/publish 三条动作路径，不需要挂载 AppFrame。
 */
import type { UpdateDashboardData } from '@/service/api/thingsvis'
import {
  buildHostSaveUpdatePayload,
  type HostSaveBridgeOptions
} from '@/components/thingsvis/thingsvisHostSaveBridge'

type HostActionsLogger = {
  error: (...args: any[]) => void
}

type HostActionsMessage = {
  success?: (message: string) => void
  error?: (message: string) => void
}

type HostActionsBridgeOptions = HostSaveBridgeOptions & {
  getCurrentId: () => string
  saveDashboard: (id: string, payload: UpdateDashboardData) => Promise<{ data?: unknown; error?: any }>
  publishDashboard: (id: string) => Promise<{ data?: unknown; error?: any }>
  resolvePreviewHref: (id: string) => string
  openPreview: (href: string) => void
  emitHostSaveSuccess: (payload: { id: string; name?: string }) => void
  logger: HostActionsLogger
  message?: HostActionsMessage
  fallbackAlert?: (message: string) => void
}

export type ThingsVisHostActionsBridge = {
  save: (payload: Record<string, unknown>) => Promise<void>
  preview: (projectId: unknown) => void
  publish: (projectId: unknown) => Promise<void>
}

function resolveProjectId(projectId: unknown, fallbackId: string): string {
  return typeof projectId === 'string' && projectId.trim() ? projectId : fallbackId
}

export function createThingsVisHostActionsBridge(options: HostActionsBridgeOptions): ThingsVisHostActionsBridge {
  async function save(payload: Record<string, unknown>) {
    const currentId = options.getCurrentId()
    const updatePayload = buildHostSaveUpdatePayload(payload, options)
    const result = await options.saveDashboard(currentId, updatePayload)

    if (result.error) {
      options.logger.error('[AppFrame] Failed to save dashboard via host bridge:', result.error)
      return
    }

    options.emitHostSaveSuccess({
      id: currentId,
      name: typeof updatePayload.name === 'string' ? updatePayload.name : undefined
    })
  }

  function preview(projectId: unknown) {
    const href = options.resolvePreviewHref(resolveProjectId(projectId, options.getCurrentId()))
    options.openPreview(href)
  }

  async function publish(projectId: unknown) {
    try {
      const res = await options.publishDashboard(resolveProjectId(projectId, options.getCurrentId()))

      if (res.data) {
        if (options.message?.success) {
          options.message.success('发布成功')
        } else {
          options.fallbackAlert?.('发布成功')
        }
        return
      }

      options.logger.error('[AppFrame] Publish failed:', res.error)
      options.message?.error?.(`发布失败: ${res.error?.message || '未知错误'}`)
    } catch (error) {
      options.logger.error('[AppFrame] Publish exception:', error)
    }
  }

  return {
    save,
    preview,
    publish
  }
}
