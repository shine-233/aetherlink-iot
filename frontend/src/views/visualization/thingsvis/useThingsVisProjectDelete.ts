// 文件用途：ThingsVis 项目的删除确认与级联清理。
// 核心逻辑：删除前先列出项目下所有仪表盘并逐个清理菜单配置，再删项目本体，
// 成功后刷新路由与首页缓存并重载列表。
// 关键注意事项：有仪表盘的项目禁止删除；菜单清理任一步失败必须中止，
// 否则会留下指向已删项目的孤儿菜单项。
import { ref } from 'vue'
import { useMessage } from 'naive-ui'
import { deleteDashboardMenuConfig } from '@/service/api/dashboard-menu'
import { refreshAuthRoutes } from '@/utils/router/refresh-auth-routes'
import { clearThingsVisHomeCache } from '@/utils/thingsvis/home-cache'
import { $t } from '@/locales'
import type { ThingsVisProjectList } from './useThingsVisProjectList'

export function useThingsVisProjectDelete(list: ThingsVisProjectList) {
  const message = useMessage()
  const { projectCapabilities, allProjects, provider, route } = list

  const deletingId = ref<string | null>(null)
  const deleteConfirmModal = ref(false)
  const pendingDeleteProject = ref<{ id: string; name: string } | null>(null)

  /** Open delete confirmation */
  const openDeleteConfirm = (id: string, name: string) => {
    if (!projectCapabilities.value.delete) {
      message.warning($t('rdi.thingsvis.projectDeletionUnsupported'))
      return
    }
    const project = allProjects.value.find((item) => item.id === id)
    if ((project?.dashboardCount || 0) > 0) {
      message.warning($t('rdi.thingsvis.projectHasDashboardsWarning'))
      return
    }

    pendingDeleteProject.value = { id, name }
    deleteConfirmModal.value = true
  }

  const handleDeleteProject = async () => {
    if (!pendingDeleteProject.value || deletingId.value) return
    deletingId.value = pendingDeleteProject.value.id
    try {
      const { id } = pendingDeleteProject.value
      const dashboardsResult = await provider.execute((current) =>
        current.listDashboards({
          projectId: id,
          page: 1,
          limit: 1000
        })
      )
      if (!dashboardsResult.ok) {
        message.error($t('rdi.thingsvis.deleteFailed'))
        return
      }
      const dashboardIds = dashboardsResult.data.items.map((dashboard) => dashboard.id)

      for (const did of dashboardIds) {
        const { error } = await deleteDashboardMenuConfig(did)
        if (error) {
          message.error($t('rdi.thingsvis.deleteMenuCleanupFailed'))
          return
        }
      }

      const deleteResult = await provider.execute((current) => current.deleteProject(id))
      if (deleteResult.ok) {
        const deletedName = pendingDeleteProject.value?.name || ''
        deleteConfirmModal.value = false
        pendingDeleteProject.value = null
        await refreshAuthRoutes(route.fullPath)
        clearThingsVisHomeCache()
        message.success($t('rdi.thingsvis.projectDeleted', { name: deletedName }))
        await list.fetchProjects(() => message.error($t('rdi.thingsvis.loadProjectsFailed')))
      } else {
        console.warn(`[handleDeleteProject] Failed to delete project ${id}`)
        message.error($t('rdi.thingsvis.deleteFailed'))
      }
    } catch (e) {
      message.error($t('rdi.thingsvis.deleteFailed'))
      console.error(e)
    } finally {
      deletingId.value = null
    }
  }

  return {
    deletingId,
    deleteConfirmModal,
    pendingDeleteProject,
    openDeleteConfirm,
    handleDeleteProject
  }
}
