/*
 * 文件用途：提供版本信息 Hook，用于展示当前版本、最新版本和更新状态。
 * 核心逻辑：从环境变量、缓存和远端版本信息中归一化版本号，并计算是否为最新版本。
 * 关键注意事项：版本缓存与请求失败会影响提示准确性，不能据此替代发布校验。
 * 重构建议：可把版本归一化和缓存读写提取为可测纯函数。
 */
import { computed, onMounted, ref } from 'vue'

import { getSysVersion } from '@/service/api/system-data'

interface VersionInfoSnapshot {
  currentVersion: string
  latestVersion: string
}

const DEFAULT_VERSION = '--'
const LATEST_VERSION_CACHE_KEY = 'aetherlink_iot_latest_version_cache_v1'
const RETIRED_CACHE_KEYS = ['aetherlink_latest_version_cache']
const LATEST_VERSION_CACHE_TTL = 12 * 60 * 60 * 1000
const REMOTE_VERSION_TIMEOUT_MS = 5000

let versionInfoCache: VersionInfoSnapshot | null = null
let versionInfoPending: Promise<VersionInfoSnapshot> | null = null

interface LatestVersionCache {
  version: string
  expiresAt: number
}

interface GitHubTag {
  name?: unknown
}

function normalizeVersion(version: unknown): string {
  if (typeof version !== 'string') return DEFAULT_VERSION
  const normalized = version.trim().replace(/^v/i, '')
  return normalized || DEFAULT_VERSION
}

function getCachedLatestVersion(): string {
  if (typeof window === 'undefined') return DEFAULT_VERSION

  RETIRED_CACHE_KEYS.forEach(key => localStorage.removeItem(key))

  try {
    const raw = localStorage.getItem(LATEST_VERSION_CACHE_KEY)
    if (!raw) return DEFAULT_VERSION

    const cache = JSON.parse(raw) as LatestVersionCache
    if (!cache?.version || !cache?.expiresAt || Date.now() > cache.expiresAt) {
      localStorage.removeItem(LATEST_VERSION_CACHE_KEY)
      return DEFAULT_VERSION
    }

    return normalizeVersion(cache.version)
  } catch {
    localStorage.removeItem(LATEST_VERSION_CACHE_KEY)
    return DEFAULT_VERSION
  }
}

function setCachedLatestVersion(version: string) {
  if (typeof window === 'undefined' || version === DEFAULT_VERSION) return

  const cache: LatestVersionCache = {
    version,
    expiresAt: Date.now() + LATEST_VERSION_CACHE_TTL
  }

  localStorage.setItem(LATEST_VERSION_CACHE_KEY, JSON.stringify(cache))
}

async function fetchLatestVersionTags(): Promise<{ data: GitHubTag[] }> {
  const controller = new AbortController()
  let timeoutId: ReturnType<typeof setTimeout> | undefined
  const timeout = new Promise<never>((_, reject) => {
    timeoutId = setTimeout(() => {
      controller.abort()
      reject(new Error('REMOTE_VERSION_CHECK_TIMEOUT'))
    }, REMOTE_VERSION_TIMEOUT_MS)
  })

  try {
    const response = await Promise.race([
      fetch('https://api.github.com/repos/shine-233/aetherlink-iot/tags', { signal: controller.signal }),
      timeout
    ])

    if (!response.ok) {
      throw new Error('REMOTE_VERSION_CHECK_UNAVAILABLE')
    }

    const data = (await response.json()) as unknown
    return { data: Array.isArray(data) ? data : [] }
  } finally {
    if (timeoutId) clearTimeout(timeoutId)
  }
}

async function fetchVersionInfo(): Promise<VersionInfoSnapshot> {
  if (versionInfoCache) return versionInfoCache
  if (versionInfoPending) return versionInfoPending

  const cachedLatestVersion = getCachedLatestVersion()
  const remoteVersionCheckEnabled = import.meta.env.VITE_ENABLE_REMOTE_VERSION_CHECK === 'Y'

  versionInfoPending = Promise.allSettled([
    getSysVersion(),
    cachedLatestVersion !== DEFAULT_VERSION
      ? Promise.resolve({ data: [{ name: cachedLatestVersion }] })
      : remoteVersionCheckEnabled
        ? fetchLatestVersionTags()
        : Promise.resolve({ data: [] })
  ])
    .then(([currentResult, latestResult]) => {
      const currentVersion =
        currentResult.status === 'fulfilled' ? normalizeVersion(currentResult.value?.data?.version) : DEFAULT_VERSION

      const latestVersion =
        latestResult.status === 'fulfilled'
          ? normalizeVersion(latestResult.value?.data?.[0]?.name)
          : cachedLatestVersion

      setCachedLatestVersion(latestVersion)

      const snapshot = {
        currentVersion,
        latestVersion
      }
      const remoteLookupRequired = remoteVersionCheckEnabled && cachedLatestVersion === DEFAULT_VERSION
      if (!remoteLookupRequired || latestVersion !== DEFAULT_VERSION) {
        versionInfoCache = snapshot
      }

      return snapshot
    })
    .finally(() => {
      versionInfoPending = null
    })

  return versionInfoPending
}

export default function useVersionInfo() {
  const currentVersion = ref(DEFAULT_VERSION)
  const latestVersion = ref(DEFAULT_VERSION)
  const isLatestVersion = computed(() => {
    return (
      currentVersion.value !== DEFAULT_VERSION &&
      latestVersion.value !== DEFAULT_VERSION &&
      currentVersion.value === latestVersion.value
    )
  })

  async function loadVersionInfo() {
    const snapshot = await fetchVersionInfo()
    currentVersion.value = snapshot.currentVersion
    latestVersion.value = snapshot.latestVersion
  }

  onMounted(() => {
    void loadVersionInfo()
  })

  return {
    currentVersion,
    latestVersion,
    isLatestVersion,
    loadVersionInfo
  }
}
