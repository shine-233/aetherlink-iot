/**
 * 文件用途：提供浏览器存储和 localforage 的统一访问工具。
 * 核心逻辑：创建 local/session 存储适配器，并在浏览器存储不可用时降级到内存 Map。
 * 关键注意事项：内存降级不会跨刷新持久化，调用方不能把它当作可靠长期存储。
 * 重构建议：可补充存储能力探测和降级路径测试，明确 SSR 或受限浏览器环境行为。
 */
import localforage from 'localforage'

/** The storage type */
export type StorageType = 'local' | 'session'

type StorageLike = Pick<Storage, 'setItem' | 'getItem' | 'removeItem' | 'clear'>

function createMemoryStorage(): StorageLike {
  const store = new Map<string, string>()

  return {
    setItem(key: string, value: string) {
      store.set(key, value)
    },
    getItem(key: string) {
      return store.get(key) ?? null
    },
    removeItem(key: string) {
      store.delete(key)
    },
    clear() {
      store.clear()
    }
  }
}

const memoryStorages: Record<StorageType, StorageLike> = {
  local: createMemoryStorage(),
  session: createMemoryStorage()
}

function getStorage(type: StorageType): StorageLike {
  if (typeof window !== 'undefined') {
    const browserStorage = type === 'session' ? window.sessionStorage : window.localStorage
    if (browserStorage) return browserStorage
  }

  return memoryStorages[type]
}

export function createStorage<T extends object>(type: StorageType) {
  const stg = getStorage(type)

  const storage = {
    /**
     * Set session
     *
     * @param key Session key
     * @param value Session value
     */
    set<K extends keyof T>(key: K, value: T[K]) {
      const json = JSON.stringify(value)

      stg.setItem(key as string, json)
    },
    /**
     * Get session
     *
     * @param key Session key
     */
    get<K extends keyof T>(key: K): T[K] | null {
      const json = stg.getItem(key as string)
      if (json) {
        let storageData: T[K] | null = null

        try {
          storageData = JSON.parse(json)
        } catch {
          /* empty */
        }

        if (storageData) {
          return storageData as T[K]
        }
      }

      stg.removeItem(key as string)

      return null
    },
    remove(key: keyof T) {
      stg.removeItem(key as string)
    },
    clear() {
      stg.clear()
    }
  }
  return storage
}

type LocalForage<T extends object> = Omit<typeof localforage, 'getItem' | 'setItem' | 'removeItem'> & {
  getItem<K extends keyof T>(key: K, callback?: (err: any, value: T[K] | null) => void): Promise<T[K] | null>

  setItem<K extends keyof T>(key: K, value: T[K], callback?: (err: any, value: T[K]) => void): Promise<T[K]>

  removeItem(key: keyof T, callback?: (err: any) => void): Promise<void>
}

type LocalforageDriver = 'local' | 'indexedDB' | 'webSQL'

const memoryLocalforageStore = new Map<string, any>()

function createMemoryLocalforage<T extends object>(): LocalForage<T> {
  return {
    ...localforage,
    async getItem<K extends keyof T>(key: K, callback?: (err: any, value: T[K] | null) => void): Promise<T[K] | null> {
      const value = (memoryLocalforageStore.get(key as string) ?? null) as T[K] | null
      callback?.(null, value)
      return value
    },
    async setItem<K extends keyof T>(key: K, value: T[K], callback?: (err: any, value: T[K]) => void): Promise<T[K]> {
      memoryLocalforageStore.set(key as string, value)
      callback?.(null, value)
      return value
    },
    async removeItem(key: keyof T, callback?: (err: any) => void): Promise<void> {
      memoryLocalforageStore.delete(key as string)
      callback?.(null)
    },
    async clear(callback?: (err: any) => void): Promise<void> {
      memoryLocalforageStore.clear()
      callback?.(null)
    }
  } as LocalForage<T>
}

export function createLocalforage<T extends object>(driver: LocalforageDriver) {
  if (typeof window === 'undefined' || !window.localStorage) {
    return createMemoryLocalforage<T>()
  }

  const driverMap: Record<LocalforageDriver, string> = {
    local: localforage.LOCALSTORAGE,
    indexedDB: localforage.INDEXEDDB,
    webSQL: localforage.WEBSQL
  }

  localforage.config({
    driver: driverMap[driver]
  })

  return localforage as LocalForage<T>
}
