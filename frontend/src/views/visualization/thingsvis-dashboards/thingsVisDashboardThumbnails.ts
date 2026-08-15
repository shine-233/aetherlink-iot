import {
  getDefaultVisualizationProviderFacade,
  type VisualizationDashboardSummary
} from '@/service/visualization-provider/index'
import type { VisualizationProviderId } from '@/service/visualization-provider/contracts'
import { LEGACY_THINGSVIS_PROVIDER_ID } from '@/service/visualization-provider/provider-ids'

type DashboardThumbnailResponse = string | {
  thumbnail?: string | null
  data?: {
    thumbnail?: string | null
  }
} | null

const THUMBNAIL_CONCURRENCY = 5

const hasInlineThumbnail = (thumbnail: string | null | undefined) =>
  Boolean(thumbnail && thumbnail.trim().startsWith('data:'))

const extractThumbnail = (resultData: DashboardThumbnailResponse) =>
  typeof resultData === 'string' ? resultData : resultData?.thumbnail || resultData?.data?.thumbnail

const processInBatches = async <T>(items: T[], batchSize: number, handler: (item: T) => Promise<void>) => {
  const queue = [...items]

  while (queue.length > 0) {
    const batch = queue.splice(0, batchSize)
    await Promise.all(batch.map(handler))
  }
}

export const getDashboardThumbnailUrl = (thumbnail: string | null | undefined): string | undefined => {
  if (!thumbnail) return undefined
  if (thumbnail.startsWith('data:')) return thumbnail
  if (thumbnail.startsWith('http')) return thumbnail
  return `data:image/png;base64,${thumbnail}`
}

export const loadDashboardThumbnail = async (
  item: VisualizationDashboardSummary,
  updateThumbnail: (dashboardId: string, thumbnail: string) => void,
  providerId: VisualizationProviderId = LEGACY_THINGSVIS_PROVIDER_ID
) => {
  if (hasInlineThumbnail(item.thumbnail)) return

  try {
    const result = await getDefaultVisualizationProviderFacade({ providerId }).execute(current =>
      current.getDashboardThumbnail(item.id)
    )
    if (!result.ok) return

    const thumbnail = extractThumbnail(result.data as DashboardThumbnailResponse)
    if (thumbnail) {
      updateThumbnail(item.id, thumbnail)
    }
  } catch (error) {
    console.error(`[loadThumbnails] load dashboard thumbnail failed ${item.id}:`, error)
  }
}

export const loadDashboardThumbnails = async (
  list: VisualizationDashboardSummary[],
  updateThumbnail: (dashboardId: string, thumbnail: string) => void,
  providerId: VisualizationProviderId = LEGACY_THINGSVIS_PROVIDER_ID
) => {
  await processInBatches(list, THUMBNAIL_CONCURRENCY, item => loadDashboardThumbnail(item, updateThumbnail, providerId))
}
