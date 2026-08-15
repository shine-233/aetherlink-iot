import { computed, onBeforeUnmount, ref, type ComputedRef } from 'vue'
import {
  getDefaultVisualizationProviderFacade,
  type VisualizationDashboardSummary,
  type VisualizationProject
} from '@/service/visualization-provider/index'
import { getDashboardThumbnailUrl, loadDashboardThumbnails } from './thingsVisDashboardThumbnails'

type Translate = (key: string, params?: Record<string, unknown>) => string
type MessageApi = {
  error: (content: string) => void
}

const THUMBNAIL_QUEUE_DELAY_MS = 80

export const useThingsVisDashboardList = (options: {
  projectId: ComputedRef<string>
  providerId: ComputedRef<string>
  message: MessageApi
  t: Translate
  registerMenuConfigDashboards: (list: VisualizationDashboardSummary[]) => void
}) => {
  const provider = getDefaultVisualizationProviderFacade({ providerId: options.providerId.value })
  const loading = ref(false)
  const project = ref<VisualizationProject | null>(null)
  const allDashboards = ref<VisualizationDashboardSummary[]>([])
  const searchKeyword = ref('')
  const requestedThumbnailIds = new Set<string>()
  const queuedThumbnailDashboards = new Map<string, VisualizationDashboardSummary>()
  let thumbnailLoadSeq = 0
  let thumbnailQueueTimer: ReturnType<typeof setTimeout> | null = null

  const dashboards = computed(() => {
    const keyword = searchKeyword.value.trim().toLowerCase()
    if (!keyword) return allDashboards.value

    return allDashboards.value.filter((item) => item.name.toLowerCase().includes(keyword))
  })

  const updateThumbnail = (dashboardId: string, thumbnail: string) => {
    const target = allDashboards.value.find((dashboard) => dashboard.id === dashboardId)
    if (target) {
      target.thumbnail = thumbnail
    }
  }

  const syncDashboardList = (list: VisualizationDashboardSummary[]) => {
    thumbnailLoadSeq += 1
    allDashboards.value = list
    requestedThumbnailIds.clear()
    queuedThumbnailDashboards.clear()
    cancelThumbnailQueue()
    options.registerMenuConfigDashboards(list)
  }

  const cancelThumbnailQueue = () => {
    if (thumbnailQueueTimer === null) return
    clearTimeout(thumbnailQueueTimer)
    thumbnailQueueTimer = null
  }

  const flushThumbnailQueue = () => {
    thumbnailQueueTimer = null
    const queued = Array.from(queuedThumbnailDashboards.values())
    queuedThumbnailDashboards.clear()
    if (queued.length === 0) return
    const loadSeq = thumbnailLoadSeq
    void loadDashboardThumbnails(queued, (dashboardId, thumbnail) => {
      if (loadSeq !== thumbnailLoadSeq) return
      updateThumbnail(dashboardId, thumbnail)
    }, options.providerId.value)
  }

  const requestThumbnail = (dashboard: VisualizationDashboardSummary) => {
    if (!dashboard.id || requestedThumbnailIds.has(dashboard.id)) return
    requestedThumbnailIds.add(dashboard.id)
    queuedThumbnailDashboards.set(dashboard.id, dashboard)
    if (thumbnailQueueTimer !== null) return
    thumbnailQueueTimer = setTimeout(flushThumbnailQueue, THUMBNAIL_QUEUE_DELAY_MS)
  }

  onBeforeUnmount(() => {
    thumbnailLoadSeq += 1
    queuedThumbnailDashboards.clear()
    cancelThumbnailQueue()
  })

  const loadProject = async () => {
    if (provider.selectionError) return false

    if (!options.projectId.value) {
      options.message.error(options.t('rdi.thingsvis.missingProjectId'))
      return false
    }

    try {
      const result = await provider.execute(current => current.getProject(options.projectId.value))
      if (result.ok) {
        project.value = result.data
        return true
      }
    } catch (error) {
      console.error(options.t('rdi.thingsvis.loadProjectFailed'), error)
    }

    return false
  }

  const fetchDashboards = async () => {
    if (provider.selectionError) return
    if (!options.projectId.value) return

    loading.value = true
    try {
      const result = await provider.execute(current => current.listDashboards({
        projectId: options.projectId.value,
        page: 1,
        limit: 100
      }))

      if (result.ok) {
        syncDashboardList(result.data.items)
      } else {
        options.message.error(options.t('rdi.thingsvis.loadDashboardsFailed'))
      }
    } finally {
      loading.value = false
    }
  }

  return {
    providerError: provider.selectionError,
    loading,
    project,
    allDashboards,
    dashboards,
    searchKeyword,
    loadProject,
    fetchDashboards,
    requestThumbnail,
    getThumbnailUrl: getDashboardThumbnailUrl
  }
}
