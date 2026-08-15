import type { HomeCustomerGuideStepId } from './homeCustomerGuide'

export type HomeFirstRunGuideState = {
  lastStep: HomeCustomerGuideStepId | null
  lastTitle: string
  lastAction: string
  lastRoute: string
  quickCreateDeviceName: string
  updatedAt: string
}

type HomeFirstRunStorageLike = Pick<Storage, 'getItem' | 'setItem'>

const HOME_FIRST_RUN_STORAGE_KEY = 'aetherlink.home.firstRunGuide'

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function normalizeFirstRunGuideState(value: unknown): HomeFirstRunGuideState | null {
  if (!isRecord(value)) return null

  return {
    lastStep: typeof value.lastStep === 'string' ? (value.lastStep as HomeCustomerGuideStepId) : null,
    lastTitle: typeof value.lastTitle === 'string' ? value.lastTitle : '',
    lastAction: typeof value.lastAction === 'string' ? value.lastAction : '',
    lastRoute: typeof value.lastRoute === 'string' ? value.lastRoute : '',
    quickCreateDeviceName: typeof value.quickCreateDeviceName === 'string' ? value.quickCreateDeviceName : '',
    updatedAt: typeof value.updatedAt === 'string' ? value.updatedAt : ''
  }
}

export function loadHomeFirstRunGuideState(
  storage: HomeFirstRunStorageLike | null | undefined
): HomeFirstRunGuideState | null {
  if (!storage) return null

  try {
    const raw = storage.getItem(HOME_FIRST_RUN_STORAGE_KEY)
    return raw ? normalizeFirstRunGuideState(JSON.parse(raw)) : null
  } catch {
    return null
  }
}

export function saveHomeFirstRunGuideState(
  storage: HomeFirstRunStorageLike | null | undefined,
  state: Omit<HomeFirstRunGuideState, 'updatedAt'>,
  now = new Date()
): HomeFirstRunGuideState {
  const nextState: HomeFirstRunGuideState = {
    ...state,
    updatedAt: now.toISOString()
  }

  storage?.setItem(HOME_FIRST_RUN_STORAGE_KEY, JSON.stringify(nextState))
  return nextState
}
