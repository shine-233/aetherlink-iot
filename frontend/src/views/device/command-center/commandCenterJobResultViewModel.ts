import type { DataTableColumns } from 'naive-ui'
import type { CommandJobRowsStatusFilter, FleetCommandJobSubmitResult, FleetCommandJobSubmitRow } from '@/service/api/device'
import type {
  CommandJobLabelValueRow,
  CommandJobAuditSummaryCard,
  CommandJobActionConsequenceRow,
  CommandJobExecutionSummaryCard,
  CommandJobGovernanceSummaryCard,
  CommandJobOperatorNextAction,
  CommandJobOutcomeGroup,
  CommandJobDeviceProgressTrack,
  CommandJobProgressHealthCard,
  CommandJobStatusCountRow,
  CommandJobSupportBundlePreview,
  CommandJobTroubleshootingRow
} from './commandCenterJobView'

export type CommandJobEvidenceAlertType = 'success' | 'info' | 'warning' | 'error'

export interface CommandJobResultViewModel {
  canLoadMoreCommandJobRows: boolean
  canRetryCurrentCommandJob: boolean
  commandJobRowsLoading: boolean
  commandJobRowsSearch: string
  commandJobRowsStatusFilter: CommandJobRowsStatusFilter
  commandJobRowsStatusFilterOptions: Array<{ label: string; value: CommandJobRowsStatusFilter }>
  jobActionConsequenceRows: CommandJobActionConsequenceRow[]
  jobAuditSummaryCard: CommandJobAuditSummaryCard | null
  jobExecutionSummaryCard: CommandJobExecutionSummaryCard | null
  jobGovernanceSummaryCard: CommandJobGovernanceSummaryCard | null
  jobActionLoading: boolean
  jobAutoRefreshActive: boolean
  jobAutoRefreshDeferred: boolean
  jobDeviceProgressTracks: CommandJobDeviceProgressTrack[]
  jobOutcomeGroups: CommandJobOutcomeGroup[]
  jobOperatorNextAction: CommandJobOperatorNextAction | null
  jobProgressHealthCard: CommandJobProgressHealthCard | null
  jobProgressPercent: number
  jobProgressSummary: string
  jobHandoffSummary: string
  jobStatusCountRows: CommandJobStatusCountRow[]
  jobStatusLabel: string
  jobStatusRows: CommandJobLabelValueRow[]
  jobTimelineRows: CommandJobLabelValueRow[]
  jobTroubleshootingRows: CommandJobTroubleshootingRow[]
  postSubmitChecklist: string[]
  retryableFailedRows: FleetCommandJobSubmitRow[]
  submitCapabilitySummary: string
  submitColumns: DataTableColumns<FleetCommandJobSubmitRow>
  submitEvidenceAlertType: CommandJobEvidenceAlertType
  submitEvidenceSummary: string
  submitResult: FleetCommandJobSubmitResult | null
  submitRowsHiddenCount: number
  submitRowsForCustomer: FleetCommandJobSubmitRow[]
  supportBundleLoading: boolean
  supportBundlePreview: CommandJobSupportBundlePreview | null
}

export interface CommandJobResultActions {
  cancelCommandJob: () => void
  copyCommandJobCloseoutPacket: () => void
  copyCommandJobHandoffSummary: () => void
  copyCommandJobLink: () => void
  copyCommandJobSupportBundle: () => void
  copyRetryableDeviceIds: () => void
  downloadCommandJobSupportBundle: () => void
  loadCommandJobSupportBundle: () => void
  loadMoreCommandJobRows: () => void
  openCommandJobDeviceDiagnosis: (deviceId: string) => void
  refreshCommandJob: () => void
  reviewCommandJobRows: (statusFilter: CommandJobRowsStatusFilter) => void | Promise<void>
  retryCommandJob: () => void
  clearCommandJobRowsSearch: () => void
  setCommandJobRowsSearch: (search: string) => void
  setCommandJobRowsStatusFilter: (statusFilter: CommandJobRowsStatusFilter) => void
}

export const buildCommandJobResultViewModel = (input: CommandJobResultViewModel): CommandJobResultViewModel => input
