/**
 * Reports embedded ThingsVis content height to the parent frame.
 *
 * This module is intentionally small and protocol-focused: callers decide what
 * element represents their visible content, while this module keeps the message
 * shape and targetOrigin handling consistent.
 */

export const THINGSVIS_CONTENT_HEIGHT_MESSAGE_TYPES = [
  'tv:content-height',
  'thingsvis:content-height',
  'tv:resize',
  'thingsvis:resize'
] as const

type ContentHeightExtraPayload = Record<string, unknown>

export type ThingsVisContentHeightReporter = {
  start: () => void
  stop: () => void
  report: (height?: unknown, extraPayload?: ContentHeightExtraPayload) => void
}

type ContentHeightReporterOptions = {
  getElement?: () => HTMLElement | null | undefined
  getExtraPayload?: () => ContentHeightExtraPayload
  measureHeight?: () => number | null
}

function normalizeHeightValue(value: unknown): number | null {
  if (typeof value === 'number') return Number.isFinite(value) && value > 0 ? value : null
  if (typeof value !== 'string') return null

  const parsed = Number.parseFloat(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

export function readThingsVisContentHeight(
  payload: Record<string, unknown> | null | undefined,
  raw: Record<string, unknown> | null | undefined = undefined
): number | null {
  return (
    normalizeHeightValue(payload?.height) ??
    normalizeHeightValue(payload?.contentHeight) ??
    normalizeHeightValue(payload?.clientHeight) ??
    normalizeHeightValue(payload?.documentHeight) ??
    normalizeHeightValue(raw?.height) ??
    normalizeHeightValue(raw?.contentHeight) ??
    normalizeHeightValue(raw?.clientHeight) ??
    normalizeHeightValue(raw?.documentHeight)
  )
}

function resolveParentTargetOrigin(): string {
  if (typeof document !== 'undefined' && document.referrer) {
    try {
      return new URL(document.referrer).origin
    } catch {
      // Fall through to the current origin.
    }
  }

  return window.location.origin
}

export function postThingsVisContentHeightToParent(
  height: unknown,
  extraPayload: ContentHeightExtraPayload = {}
): void {
  if (typeof window === 'undefined' || window.parent === window) return

  const normalizedHeight = normalizeHeightValue(height)
  if (normalizedHeight === null) return

  const contentHeight = Math.ceil(normalizedHeight)
  const payload = {
    ...extraPayload,
    height: contentHeight,
    contentHeight,
    documentHeight: contentHeight
  }

  window.parent.postMessage(
    {
      ...payload,
      type: 'tv:content-height',
      payload
    },
    resolveParentTargetOrigin()
  )
}

function measureElementHeight(element: HTMLElement | null | undefined): number | null {
  const target = element || document.documentElement
  if (!target) return null

  const rectHeight = target.getBoundingClientRect?.().height || 0
  const body = document.body
  const documentElement = document.documentElement

  return Math.max(
    rectHeight,
    target.scrollHeight || 0,
    target.offsetHeight || 0,
    body?.scrollHeight || 0,
    body?.offsetHeight || 0,
    documentElement?.scrollHeight || 0,
    documentElement?.offsetHeight || 0
  )
}

export function createThingsVisContentHeightReporter(
  options: ContentHeightReporterOptions = {}
): ThingsVisContentHeightReporter {
  let resizeObserver: ResizeObserver | null = null
  let frameHandle: number | null = null
  let fallbackTimer: ReturnType<typeof setTimeout> | null = null
  let started = false

  const readMeasuredHeight = () => options.measureHeight?.() ?? measureElementHeight(options.getElement?.())

  const report = (height: unknown = undefined, extraPayload: ContentHeightExtraPayload = {}) => {
    const measuredHeight = height === undefined || height === null ? readMeasuredHeight() : height
    postThingsVisContentHeightToParent(measuredHeight, {
      ...options.getExtraPayload?.(),
      ...extraPayload
    })
  }

  const clearScheduledReport = () => {
    if (frameHandle !== null && typeof window.cancelAnimationFrame === 'function') {
      window.cancelAnimationFrame(frameHandle)
    }
    frameHandle = null

    if (fallbackTimer) {
      clearTimeout(fallbackTimer)
      fallbackTimer = null
    }
  }

  const scheduleReport = () => {
    if (frameHandle !== null || fallbackTimer) return

    if (typeof window.requestAnimationFrame === 'function') {
      frameHandle = window.requestAnimationFrame(() => {
        frameHandle = null
        report()
      })
      return
    }

    fallbackTimer = setTimeout(() => {
      fallbackTimer = null
      report()
    }, 0)
  }

  const start = () => {
    if (started || typeof window === 'undefined' || window.parent === window) return
    started = true

    scheduleReport()
    window.addEventListener('resize', scheduleReport)

    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(scheduleReport)
      const element = options.getElement?.()
      if (element) resizeObserver.observe(element)
      if (document.body) resizeObserver.observe(document.body)
      if (document.documentElement) resizeObserver.observe(document.documentElement)
    }
  }

  const stop = () => {
    if (!started) return
    started = false

    clearScheduledReport()
    window.removeEventListener('resize', scheduleReport)
    resizeObserver?.disconnect()
    resizeObserver = null
  }

  return {
    start,
    stop,
    report
  }
}
