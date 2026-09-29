/*
 * Device row quick actions (edit / share / claim-token / claim-redeem).
 *
 * `DeviceManageQuickActions.vue` is mounted lazily on first use, so a request issued before the
 * child exists is queued and replayed once its ref resolves.
 */
import { ref, watch } from 'vue'

type QuickActionKind = 'edit' | 'share' | 'claim-issue' | 'claim-redeem'

type PendingQuickAction = { kind: QuickActionKind; row?: any } | null

export interface DeviceManageQuickActionsExpose {
  openEditDevice: (row: any) => void
  openShareDevice: (row: any) => void
  openIssueClaimToken: (row: any) => void
  openClaimDevice: () => void
}

export function useDeviceManageQuickActions() {
  const quickActionsRef = ref<DeviceManageQuickActionsExpose | null>(null)
  const quickActionsVisited = ref(false)
  const pendingQuickAction = ref<PendingQuickAction>(null)

  function flushPendingQuickAction() {
    const instance = quickActionsRef.value
    const nextAction = pendingQuickAction.value
    if (!instance || !nextAction) return

    pendingQuickAction.value = null
    if (nextAction.kind === 'edit') {
      instance.openEditDevice(nextAction.row)
      return
    }
    if (nextAction.kind === 'claim-issue') {
      instance.openIssueClaimToken(nextAction.row)
      return
    }
    if (nextAction.kind === 'claim-redeem') {
      instance.openClaimDevice()
      return
    }
    instance.openShareDevice(nextAction.row)
  }

  watch(quickActionsRef, flushPendingQuickAction)

  function requestQuickAction(kind: QuickActionKind, row?: any) {
    quickActionsVisited.value = true
    pendingQuickAction.value = { kind, row }
    flushPendingQuickAction()
  }

  return {
    quickActionsRef,
    quickActionsVisited,
    openEditDevice: (row: any) => requestQuickAction('edit', row),
    openShareDevice: (row: any) => requestQuickAction('share', row),
    openIssueClaimToken: (row: any) => requestQuickAction('claim-issue', row),
    openClaimDeviceDialog: () => requestQuickAction('claim-redeem')
  }
}
