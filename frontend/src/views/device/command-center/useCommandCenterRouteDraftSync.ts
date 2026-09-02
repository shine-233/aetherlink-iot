import { ref, watch } from 'vue'
import type { Ref } from 'vue'

type ReadonlyRef<T> = {
  readonly value: T
}

interface CommandCenterRouteDraftShape {
  identify: string
  value: string
  source: string
  timeoutSeconds: number | null
  signature: string
  hasDraft: boolean
}

interface UseCommandCenterRouteDraftSyncOptions {
  routeCommandDraft: ReadonlyRef<CommandCenterRouteDraftShape>
  commandIdentify: Ref<string>
  commandValue: Ref<string>
  timeoutSeconds: Ref<number | null>
  resetCommandJobDraft: () => void
}

export function useCommandCenterRouteDraftSync(options: UseCommandCenterRouteDraftSyncOptions) {
  const reusedCommandJobDraft = ref<{ jobId: string; identify: string } | null>(null)
  const routeCommandDraftNotice = ref<{ identify: string; source: string } | null>(null)
  const appliedRouteCommandDraftSignature = ref('')

  const clearReusedCommandJobDraft = () => {
    reusedCommandJobDraft.value = null
  }

  const clearRouteCommandDraftNotice = () => {
    routeCommandDraftNotice.value = null
  }

  const applyRouteCommandDraft = () => {
    const draft = options.routeCommandDraft.value
    if (!draft.hasDraft || draft.signature === appliedRouteCommandDraftSignature.value) return

    options.commandIdentify.value = draft.identify
    options.commandValue.value = draft.value
    if (draft.timeoutSeconds) {
      options.timeoutSeconds.value = draft.timeoutSeconds
    }
    reusedCommandJobDraft.value = null
    routeCommandDraftNotice.value = {
      identify: draft.identify,
      source: draft.source
    }
    appliedRouteCommandDraftSignature.value = draft.signature
    options.resetCommandJobDraft()
  }

  watch(options.commandIdentify, (identify) => {
    if (reusedCommandJobDraft.value && identify !== reusedCommandJobDraft.value.identify) {
      clearReusedCommandJobDraft()
    }
    if (routeCommandDraftNotice.value && identify !== routeCommandDraftNotice.value.identify) {
      clearRouteCommandDraftNotice()
    }
  })

  watch(options.routeCommandDraft, () => {
    applyRouteCommandDraft()
  })

  return {
    applyRouteCommandDraft,
    clearReusedCommandJobDraft,
    clearRouteCommandDraftNotice,
    reusedCommandJobDraft,
    routeCommandDraftNotice
  }
}
