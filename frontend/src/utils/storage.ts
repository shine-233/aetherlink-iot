/*
 * 文件用途：创建本地存储、会话存储和 localforage 实例。
 * 核心逻辑：基于 storage adapter 封装 local/session/localforage 的类型化访问入口。
 * 关键注意事项：存储 key 和结构变更会影响登录态、主题和缓存兼容。
 * 重构建议：后续可为关键 key 增加迁移和版本管理。
 */
import { createLocalforage, createStorage } from '@aetherlink/utils'

export const localStg = createStorage<StorageType.Local>('local')

export const sessionStg = createStorage<StorageType.Session>('session')

export const localforage = createLocalforage<StorageType.Local>('local')
