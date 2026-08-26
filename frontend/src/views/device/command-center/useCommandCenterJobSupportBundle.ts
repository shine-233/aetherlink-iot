import { ref } from 'vue'
import { writeClipboardText } from '@/utils/clipboard'
import { getFleetCommandJobSupportBundle } from '@/service/api/device'
import type { FleetCommandJobSupportBundle } from '@/service/api/device'
import { normalizeApiData } from './commandCenterState'

interface UseCommandCenterJobSupportBundleOptions {
  activeJobId: () => string | undefined
  setError: (message: string) => void
  t: (key: string) => string
}

interface ClearCommandJobSupportBundleOptions {
  cancelInFlight?: boolean
}

const commandJobSupportBundleFileName = (jobId: string) =>
  `aetherlink-command-job-${jobId.replace(/[^a-zA-Z0-9._-]/g, '_')}-support-bundle.json`

const saveCommandJobSupportBundleFile = (jobId: string, bundle: FleetCommandJobSupportBundle) => {
  if (typeof window === 'undefined' || typeof document === 'undefined') return false
  const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: 'application/json' })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = commandJobSupportBundleFileName(jobId)
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.URL.revokeObjectURL(url)
  return true
}

export const useCommandCenterJobSupportBundle = (options: UseCommandCenterJobSupportBundleOptions) => {
  const supportBundleLoading = ref(false)
  const supportBundle = ref<FleetCommandJobSupportBundle | null>(null)
  let supportBundleRequestSeq = 0

  const clearCommandJobSupportBundle = (clearOptions: ClearCommandJobSupportBundleOptions = {}) => {
    if (clearOptions.cancelInFlight !== false) {
      supportBundleRequestSeq++
      supportBundleLoading.value = false
    }
    supportBundle.value = null
  }

  const loadCommandJobSupportBundle = async () => {
    const jobId = options.activeJobId()
    if (!jobId) return
    const requestSeq = ++supportBundleRequestSeq
    supportBundleLoading.value = true
    try {
      const bundle = normalizeApiData(await getFleetCommandJobSupportBundle(jobId))
      if (requestSeq !== supportBundleRequestSeq || options.activeJobId() !== jobId) return
      supportBundle.value = bundle
    } catch (error) {
      if (requestSeq === supportBundleRequestSeq) {
        options.setError(
          error instanceof Error ? error.message : options.t('custom.commandCenter.copySupportBundleFailed')
        )
      }
    } finally {
      if (requestSeq === supportBundleRequestSeq) {
        supportBundleLoading.value = false
      }
    }
  }

  const copyCommandJobSupportBundle = async () => {
    const jobId = options.activeJobId()
    if (!jobId) return
    const requestSeq = ++supportBundleRequestSeq
    supportBundleLoading.value = true
    try {
      const bundle = supportBundle.value ?? normalizeApiData(await getFleetCommandJobSupportBundle(jobId))
      if (requestSeq !== supportBundleRequestSeq || options.activeJobId() !== jobId) return
      supportBundle.value = bundle
      const ok = await writeClipboardText(JSON.stringify(bundle, null, 2))
      if (ok) window.$message?.success(options.t('custom.commandCenter.copySupportBundleSuccess'))
      else window.$message?.warning(options.t('common.copyFailed'))
    } catch (error) {
      if (requestSeq === supportBundleRequestSeq) {
        options.setError(
          error instanceof Error ? error.message : options.t('custom.commandCenter.copySupportBundleFailed')
        )
      }
    } finally {
      if (requestSeq === supportBundleRequestSeq) {
        supportBundleLoading.value = false
      }
    }
  }

  const downloadCommandJobSupportBundle = async () => {
    const jobId = options.activeJobId()
    if (!jobId) return
    const requestSeq = ++supportBundleRequestSeq
    supportBundleLoading.value = true
    try {
      const bundle = supportBundle.value ?? normalizeApiData(await getFleetCommandJobSupportBundle(jobId))
      if (requestSeq !== supportBundleRequestSeq || options.activeJobId() !== jobId) return
      supportBundle.value = bundle
      if (saveCommandJobSupportBundleFile(jobId, bundle)) {
        window.$message?.success(options.t('custom.commandCenter.downloadSupportBundleSuccess'))
      } else {
        window.$message?.warning(options.t('custom.commandCenter.downloadSupportBundleFailed'))
      }
    } catch (error) {
      if (requestSeq === supportBundleRequestSeq) {
        options.setError(
          error instanceof Error ? error.message : options.t('custom.commandCenter.downloadSupportBundleFailed')
        )
      }
    } finally {
      if (requestSeq === supportBundleRequestSeq) {
        supportBundleLoading.value = false
      }
    }
  }

  return {
    clearCommandJobSupportBundle,
    copyCommandJobSupportBundle,
    downloadCommandJobSupportBundle,
    loadCommandJobSupportBundle,
    supportBundle,
    supportBundleLoading
  }
}
