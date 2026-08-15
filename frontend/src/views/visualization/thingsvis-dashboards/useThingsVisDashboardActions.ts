import { computed, ref, type ComputedRef } from 'vue'
import { deleteDashboardMenuConfig } from '@/service/api/dashboard-menu'
import { NATIVE_BOARD_PROJECT_ID, NATIVE_BOARD_PROVIDER_ID } from '@/service/visualization-provider/provider-ids'
import {
  getDefaultVisualizationProviderFacade,
  type VisualizationDashboardSummary
} from '@/service/visualization-provider/index'
import { writeClipboardText } from '@/utils/clipboard'
import { clearThingsVisHomeCache } from '@/utils/thingsvis/home-cache'
import { buildRdiDashboardPreset, getDashboardTemplateOptions, type DashboardTemplateType } from './rdi-preset'
import { buildThingsVisDashboardClipboardLink } from './thingsVisDashboardSharing'

type Translate = (key: string, params?: Record<string, unknown>) => string
type MessageApi = {
  success: (content: string) => void
  error: (content: string) => void
}

export function useThingsVisDashboardActions(options: {
  projectId: ComputedRef<string>
  providerId: ComputedRef<string>
  message: MessageApi
  t: Translate
  fetchDashboards: () => Promise<void>
}) {
  const provider = getDefaultVisualizationProviderFacade({ providerId: options.providerId.value })
  const deletingId = ref<string | null>(null)
  const publishingId = ref<string | null>(null)
  const duplicatingId = ref<string | null>(null)
  const creatingHomepageDashboard = ref(false)
  const showModal = ref(false)
  const deleteConfirmModal = ref(false)
  const pendingDeleteDashboard = ref<{ id: string; name: string } | null>(null)

  const formData = ref({
    name: '',
    template: 'blank' as DashboardTemplateType,
    canvasMode: 'fixed' as 'fixed' | 'grid' | 'infinite',
    canvasWidth: 1920,
    canvasHeight: 1080
  })

  const dashboardTemplateOptions = computed(() => getDashboardTemplateOptions(options.t))

  const resetCreateForm = () => {
    formData.value = {
      name: '',
      template: 'blank',
      canvasMode: 'fixed',
      canvasWidth: 1920,
      canvasHeight: 1080
    }
  }

  const openCreateModal = () => {
    resetCreateForm()
    showModal.value = true
  }

  const buildDashboardCreateData = (
    name: string,
    template: DashboardTemplateType,
    canvasMode: 'fixed' | 'grid' | 'infinite',
    canvasWidth: number,
    canvasHeight: number
  ) => {
    if (options.providerId.value === NATIVE_BOARD_PROVIDER_ID) {
      return {
        name,
        projectId: NATIVE_BOARD_PROJECT_ID,
        rendererData: {
          version: 1,
          columns: 24,
          rowHeight: 60,
          widgets: []
        }
      }
    }

    const baseCanvasConfig = {
      mode: canvasMode,
      width: canvasWidth,
      height: canvasHeight,
      background: '#f5f7fb'
    }
    const preset =
      template === 'rdi'
        ? buildRdiDashboardPreset(canvasWidth, canvasHeight, options.t)
        : null

    return {
      name,
      projectId: options.projectId.value,
      canvasConfig: preset?.canvasConfig || baseCanvasConfig,
      nodes: preset?.nodes || [],
      dataSources: preset?.dataSources || [],
      variables: preset?.variables || []
    }
  }

  const handleCreateDashboard = async () => {
    const name = formData.value.name.trim()
    if (!name) {
      options.message.error(options.t('rdi.thingsvis.dashboardNamePlaceholder'))
      return
    }

    const result = await provider.execute(current =>
      current.createDashboard(
        buildDashboardCreateData(
          name,
          formData.value.template,
          formData.value.canvasMode,
          formData.value.canvasWidth,
          formData.value.canvasHeight
        )
      )
    )

    if (result.ok) {
      options.message.success(options.t('rdi.thingsvis.createSuccess'))
      showModal.value = false
      await options.fetchDashboards()
    } else {
      options.message.error(options.t('rdi.thingsvis.createFailed'))
    }
  }

  const handleCreateHomepageDashboard = async () => {
    if (!options.projectId.value) {
      options.message.error(options.t('rdi.thingsvis.missingProjectId'))
      return
    }

    creatingHomepageDashboard.value = true
    try {
      const dashboardName = options.t('rdi.thingsvis.homepageStarterDashboardName')
      const createResult = await provider.execute(current =>
        current.createDashboard(buildDashboardCreateData(dashboardName, 'rdi', 'fixed', 1920, 1080))
      )
      if (!createResult.ok || !createResult.data.id) {
        options.message.error(options.t('rdi.thingsvis.createHomepageDashboardFailed'))
        return
      }

      const homeResult = await provider.execute(current => current.setHomeDashboard(createResult.data.id))
      if (!homeResult.ok) {
        options.message.error(options.t('rdi.thingsvis.createHomepageDashboardSetHomeFailed'))
        await options.fetchDashboards()
        return
      }

      clearThingsVisHomeCache()
      options.message.success(
        options.t('rdi.thingsvis.createHomepageDashboardSuccess', { name: createResult.data.name })
      )
      await options.fetchDashboards()
    } finally {
      creatingHomepageDashboard.value = false
    }
  }

  const openDeleteConfirm = (id: string, name: string) => {
    pendingDeleteDashboard.value = { id, name }
    deleteConfirmModal.value = true
  }

  const handleDeleteDashboard = async () => {
    if (!pendingDeleteDashboard.value) return

    const { id } = pendingDeleteDashboard.value
    deletingId.value = id
    try {
      const { error: menuError } = await deleteDashboardMenuConfig(id)
      if (menuError) {
        options.message.error(options.t('rdi.thingsvis.deleteMenuCleanupFailed'))
        return
      }

      const result = await provider.execute(current => current.deleteDashboard(id))
      if (result.ok) {
        deleteConfirmModal.value = false
        pendingDeleteDashboard.value = null
        await options.fetchDashboards()
      } else {
        options.message.error(options.t('rdi.thingsvis.deleteFailed'))
      }
    } catch {
      options.message.error(options.t('rdi.thingsvis.deleteFailed'))
    } finally {
      deletingId.value = null
    }
  }

  const handleSetAsHomepage = async (dashboard: VisualizationDashboardSummary) => {
    const result = await provider.execute(current => current.setHomeDashboard(dashboard.id))
    if (result.ok) {
      clearThingsVisHomeCache()
      options.message.success(options.t('rdi.thingsvis.setHomeSuccess', { name: dashboard.name }))
      await options.fetchDashboards()
    } else {
      options.message.error(options.t('rdi.thingsvis.setHomeFailed'))
    }
  }

  const handlePublishDashboard = async (dashboard: VisualizationDashboardSummary) => {
    if (dashboard.published) return

    publishingId.value = dashboard.id
    try {
      const result = await provider.execute(current => current.publishDashboard(dashboard.id))
      if (result.ok) {
        options.message.success(options.t('rdi.thingsvis.publishSuccess', { name: dashboard.name }))
        await options.fetchDashboards()
      } else {
        options.message.error(options.t('rdi.thingsvis.publishFailed'))
      }
    } catch {
      options.message.error(options.t('rdi.thingsvis.publishFailed'))
    } finally {
      publishingId.value = null
    }
  }

  const handleDuplicateDashboard = async (dashboard: VisualizationDashboardSummary) => {
    duplicatingId.value = dashboard.id
    try {
      const result = await provider.execute(current => current.duplicateDashboard(dashboard.id))
      if (result.ok) {
        options.message.success(options.t('rdi.thingsvis.duplicateSuccess', { name: dashboard.name }))
        await options.fetchDashboards()
      } else {
        options.message.error(options.t('rdi.thingsvis.duplicateFailed'))
      }
    } catch {
      options.message.error(options.t('rdi.thingsvis.duplicateFailed'))
    } finally {
      duplicatingId.value = null
    }
  }

  const copyDashboardViewerLink = async (dashboard: VisualizationDashboardSummary) => {
    const copied = await writeClipboardText(buildThingsVisDashboardClipboardLink(dashboard))
    if (copied) {
      options.message.success(options.t('rdi.thingsvis.copyLinkSuccess'))
    } else {
      options.message.error(options.t('rdi.thingsvis.copyLinkFailed'))
    }
  }

  return {
    deletingId,
    publishingId,
    duplicatingId,
    creatingHomepageDashboard,
    showModal,
    deleteConfirmModal,
    pendingDeleteDashboard,
    formData,
    dashboardTemplateOptions,
    openCreateModal,
    handleCreateDashboard,
    handleCreateHomepageDashboard,
    openDeleteConfirm,
    handleDeleteDashboard,
    handleSetAsHomepage,
    handlePublishDashboard,
    handleDuplicateDashboard,
    copyDashboardViewerLink
  }
}
