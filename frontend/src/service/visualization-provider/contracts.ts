export type VisualizationProviderId = string

// Deployment mode is part of the provider contract: local capability is the
// default path, while external providers require explicit opt-in.
export type VisualizationProviderDeploymentMode = 'local-default' | 'optional-external'

export type VisualizationErrorCode =
  | 'unknown-provider'
  | 'provider-unavailable'
  | 'provider-unauthenticated'
  | 'external-blocked'
  | 'unsupported-operation'
  | 'ownership-mismatch'
  | 'invalid-response'
  | 'provider-failure'

export interface VisualizationError {
  code: VisualizationErrorCode
  message: string
  status?: number
  cause?: unknown
}

export type VisualizationResult<T> =
  | { ok: true; data: T }
  | { ok: false; error: VisualizationError }

export interface VisualizationPage<T> {
  items: T[]
  page: number
  limit: number
  total: number
  totalPages: number
}

export interface VisualizationProject {
  id: string
  name: string
  description: string | null
  thumbnail: string | null
  tenantId?: string
  ownerId?: string
  createdAt: string
  updatedAt: string
  dashboardCount?: number
}

export type VisualizationDashboardNode = Record<string, unknown> & {
  id?: string
  type?: string
  name?: string
  props?: Record<string, unknown>
}

export interface VisualizationDashboardSummary {
  id: string
  name: string
  tenantId?: string
  description: string | null
  thumbnail: string | null
  version: number
  published: boolean
  readonly isPublished?: boolean
  publishedAt?: string | null
  shareToken?: string | null
  home: boolean
  readonly homeFlag?: boolean
  projectId: string
  createdAt: string
  updatedAt: string
}

export interface VisualizationDashboardSchema {
  id: string
  name: string
  tenantId?: string
  description: string | null
  thumbnail: string | null
  version: number
  canvasConfig: {
    mode: string
    width: number
    height: number
    background: string | Record<string, unknown> | null
  }
  nodes: VisualizationDashboardNode[]
  dataSources: unknown[]
  variables?: unknown[]
  rendererData?: unknown
  published: boolean
  publishedAt: string | null
  shareToken: string | null
  projectId: string
  ownerId?: string
  createdAt: string
  updatedAt: string
}

export interface CreateVisualizationProjectPayload {
  name: string
  description?: string
}

export interface UpdateVisualizationProjectPayload {
  name?: string
  description?: string
  thumbnail?: string
}

export interface CreateVisualizationDashboardPayload {
  name: string
  description?: string
  projectId: string
  tenantId?: string
  canvasConfig?: {
    mode?: string
    width?: number
    height?: number
    background?: string | Record<string, unknown>
  }
  nodes?: VisualizationDashboardNode[]
  dataSources?: unknown[]
  variables?: unknown[]
  rendererData?: unknown
}

export interface UpdateVisualizationDashboardPayload {
  name?: string
  description?: string
  thumbnail?: string | null
  canvasConfig?: unknown
  nodes?: VisualizationDashboardNode[]
  dataSources?: unknown[]
  variables?: unknown[]
  rendererData?: unknown
}

export interface VisualizationProviderContext {
  available: boolean
  authenticated: boolean
  ownerId?: string
}

export interface VisualizationProvider {
  readonly id: VisualizationProviderId
  readonly kind: 'third-party' | 'local'
  readonly deploymentMode: VisualizationProviderDeploymentMode
  listProjects(params?: { page?: number; limit?: number }): Promise<VisualizationResult<VisualizationPage<VisualizationProject>>>
  getProject(id: string): Promise<VisualizationResult<VisualizationProject>>
  createProject(payload: CreateVisualizationProjectPayload): Promise<VisualizationResult<VisualizationProject>>
  updateProject(id: string, payload: UpdateVisualizationProjectPayload): Promise<VisualizationResult<VisualizationProject>>
  deleteProject(id: string): Promise<VisualizationResult<void>>
  listDashboards(params: { projectId: string; page?: number; limit?: number; name?: string; tenantId?: string }): Promise<VisualizationResult<VisualizationPage<VisualizationDashboardSummary>>>
  getDashboard(id: string): Promise<VisualizationResult<VisualizationDashboardSchema>>
  /** Optional unauthenticated lookup used only by a provider's public viewer. */
  getDashboardByShareToken?(token: string): Promise<VisualizationResult<VisualizationDashboardSchema>>
  getDashboardThumbnail(id: string): Promise<VisualizationResult<string | null>>
  createDashboard(payload: CreateVisualizationDashboardPayload): Promise<VisualizationResult<VisualizationDashboardSchema>>
  updateDashboard(id: string, payload: UpdateVisualizationDashboardPayload): Promise<VisualizationResult<VisualizationDashboardSchema>>
  deleteDashboard(id: string): Promise<VisualizationResult<void>>
  publishDashboard(id: string): Promise<VisualizationResult<VisualizationDashboardSchema>>
  duplicateDashboard(id: string): Promise<VisualizationResult<VisualizationDashboardSchema>>
  setHomeDashboard(id: string): Promise<VisualizationResult<void>>
  unsetHomeDashboard(id: string): Promise<VisualizationResult<void>>
  getHomeDashboard(params?: { tenantId?: string }): Promise<VisualizationResult<VisualizationDashboardSchema | null>>
}

export interface LocalVisualizationProvider extends VisualizationProvider {
  readonly kind: 'local'
}

export interface ThirdPartyVisualizationProvider extends VisualizationProvider {
  readonly kind: 'third-party'
}
