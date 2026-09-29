/**
 * 文件用途：首台设备工作台的区块定位与延迟挂载编排。
 * 核心逻辑：连接测试 / 成功证明 / 支持摘要三块按视口延迟挂载；定位某区块前先强制挂载所在分组，
 *   再滚动到子组件暴露的锚点元素（未挂载时退回视口占位元素）。
 * 关键注意事项：流程节点键（identity/browser_test/...）与视图区块键（device/test/...）在这里统一归一，
 *   原先两个 focus 函数各维护一张映射，browser_test 定位时不会触发延迟挂载，这里一并修正。
 */
import { nextTick, ref, watch, type ComponentPublicInstance } from 'vue'
import { useViewportDeferredMount } from './useViewportDeferredMount'
import { deferredSectionFor, normalizeFirstDeviceSectionKey } from './homeFirstDeviceWorkbenchActions'

type ConnectionTestExpose = { connectionEl: HTMLElement | null; testCommandEl: HTMLElement | null }
type SuccessProofExpose = { chartSectionEl: HTMLElement | null; proofSectionEl: HTMLElement | null }
type SupportSummaryExpose = { openPreview: () => void }

export function useFirstDeviceSectionFocus() {
  const deviceIdentitySectionRef = ref<HTMLElement | null>(null)
  const quickstartSectionRef = ref<HTMLElement | null>(null)
  const deploymentHealthSectionRef = ref<HTMLElement | null>(null)
  const connectionTestViewportRef = ref<HTMLElement | null>(null)
  const connectionTestSectionRef = ref<ConnectionTestExpose | null>(null)
  const successProofViewportRef = ref<HTMLElement | null>(null)
  const successProofSectionRef = ref<SuccessProofExpose | null>(null)
  const supportSummaryViewportRef = ref<HTMLElement | null>(null)
  const supportSummarySectionRef = ref<SupportSummaryExpose | null>(null)

  const asExpose = <T>(instance: Element | ComponentPublicInstance | null) => instance as unknown as T | null
  const setters = {
    setConnectionTestViewportRef: (el: HTMLElement | null) => (connectionTestViewportRef.value = el),
    setConnectionTestSectionRef: (instance: Element | ComponentPublicInstance | null) =>
      (connectionTestSectionRef.value = asExpose<ConnectionTestExpose>(instance)),
    setSuccessProofViewportRef: (el: HTMLElement | null) => (successProofViewportRef.value = el),
    setSuccessProofSectionRef: (instance: Element | ComponentPublicInstance | null) =>
      (successProofSectionRef.value = asExpose<SuccessProofExpose>(instance)),
    setSupportSummaryViewportRef: (el: HTMLElement | null) => (supportSummaryViewportRef.value = el),
    setSupportSummarySectionRef: (instance: Element | ComponentPublicInstance | null) =>
      (supportSummarySectionRef.value = asExpose<SupportSummaryExpose>(instance))
  }

  const connectionTest = useViewportDeferredMount(connectionTestViewportRef, {
    rootMargin: '480px 0px',
    fallbackDelay: 600
  })
  const successProof = useViewportDeferredMount(successProofViewportRef, {
    rootMargin: '520px 0px',
    fallbackDelay: 700
  })
  const supportSummary = useViewportDeferredMount(supportSummaryViewportRef, {
    rootMargin: '480px 0px',
    fallbackDelay: 600
  })
  const deferredGroups = { connectionTest, successProof, supportSummary }

  const ensureMounted = async (key: string) => {
    const group = deferredSectionFor(key)
    if (!group) return
    const deferred = deferredGroups[group]
    if (deferred.shouldMount.value) return
    deferred.mountNow()
    await nextTick()
  }

  const resolveTarget = (section: string): HTMLElement | null => {
    const connection = connectionTestSectionRef.value
    const proof = successProofSectionRef.value
    const targets: Record<string, HTMLElement | null> = {
      device: deviceIdentitySectionRef.value,
      connection: connection?.connectionEl || connectionTestViewportRef.value,
      test: connection?.testCommandEl || connectionTestViewportRef.value,
      chart: proof?.chartSectionEl || successProofViewportRef.value,
      proof: proof?.proofSectionEl || successProofViewportRef.value,
      quickstart: quickstartSectionRef.value,
      support: supportSummaryViewportRef.value,
      deployment: deploymentHealthSectionRef.value
    }
    return targets[section] || quickstartSectionRef.value || supportSummaryViewportRef.value
  }

  /** 定位到区块；同时接受流程节点键与视图区块键。 */
  const focusSection = async (key: string) => {
    await ensureMounted(key)
    const target = resolveTarget(normalizeFirstDeviceSectionKey(key))
    if (!target) return
    await nextTick()
    target.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }

  const focusDeploymentHealth = async () => {
    await nextTick()
    deploymentHealthSectionRef.value?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }

  // 支持摘要子组件是异步组件：挂载后 ref 可能晚一拍才可用，用 pending 标记在 ref 到达时补开预览。
  const pendingSupportSummaryPreviewOpen = ref(false)
  const openSupportSummaryPreview = async () => {
    await ensureMounted('support')
    if (supportSummarySectionRef.value) {
      supportSummarySectionRef.value.openPreview()
      return
    }
    pendingSupportSummaryPreviewOpen.value = true
  }
  watch(supportSummarySectionRef, (section) => {
    if (!section || !pendingSupportSummaryPreviewOpen.value) return
    pendingSupportSummaryPreviewOpen.value = false
    section.openPreview()
  })

  return {
    deviceIdentitySectionRef,
    quickstartSectionRef,
    deploymentHealthSectionRef,
    ...setters,
    shouldMountConnectionTestSection: connectionTest.shouldMount,
    shouldMountSuccessProofSection: successProof.shouldMount,
    shouldMountSupportSummarySection: supportSummary.shouldMount,
    focusSection,
    focusDeploymentHealth,
    openSupportSummaryPreview
  }
}
