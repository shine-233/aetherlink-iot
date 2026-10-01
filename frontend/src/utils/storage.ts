/*
 * 文件用途：创建本地存储的类型化访问入口。
 * 核心逻辑：基于 @aetherlink/utils 的 storage adapter 封装 localStorage 读写。
 * 关键注意事项：存储 key 和结构变更会影响登录态、主题和缓存兼容。
 *   这里刻意不再在模块顶层创建 localforage 实例：全仓库无调用方，但顶层的
 *   localforage.config() 副作用会把整个 localforage（~96KB 未压缩）钉进入口 chunk。
 *   确有异步大对象存储需求时，在使用处按需 `await import` 后调用 createLocalforage。
 * 重构建议：后续可为关键 key 增加迁移和版本管理。
 */
import { createStorage } from '@aetherlink/utils'

export const localStg = createStorage<StorageType.Local>('local')
