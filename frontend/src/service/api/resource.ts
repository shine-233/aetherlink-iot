/*
 * 文件用途：REST 资源端点构造器，消除各业务域 API 模块里重复手写的 CRUD 包装。
 * 核心逻辑：给定集合路径与（可选）单体路径、更新风格，产出类型化的
 *   list / detail / create / update / remove 五个函数，统一做 ID 编码与配置透传。
 * 关键注意事项：
 *   - 只读 GET 默认开启 in-flight 去重，并返回独立副本；写操作不去重。
 *   - 调用方可透传 CustomAxiosRequestConfig（signal / silentError / needMessage），
 *     取消语义由 src/service/request/abortable.ts 的 runner 管理。
 *   - 各业务模块仍需 re-export 原有导出名，视图调用方式不得改变。
 * 重构建议：新增资源域时优先使用本构造器，避免再复制一份 request.get/post 样板。
 */
import type { CustomAxiosRequestConfig, FlatResponseData } from '@aetherlink/axios'
import { request } from '../request'
import { buildGetDedupeKey, dedupeGet } from '../request/dedupe'

/** 单个资源的 ID 编码策略：默认 URL 编码，避免 ID 中的 `/` 等字符破坏路径。 */
function encodeId(id: string | number): string {
  return encodeURIComponent(String(id))
}

export interface ResourceOptions<CreateBody> {
  /** 集合路径，如 `/widget-bundles` */
  collection: string
  /**
   * 列表路径。多数域与集合路径一致；少数域（如 user_groups）列表与单体前缀不同。
   * 未指定时回落到 collection。
   */
  listPath?: string
  /** 单体路径构造器，默认 `${collection}/${encodeURIComponent(id)}` */
  itemPath?: (id: string | number) => string
  /**
   * 更新语义：
   * - `body-id`（默认）：PUT 到集合路径，ID 放在请求体里（后端多数域采用）
   * - `item-path`：PUT 到单体路径，请求体只含字段
   */
  updateStyle?: 'body-id' | 'item-path'
  /**
   * 是否对只读 GET 做 in-flight 去重。
   *
   * @default true
   */
  dedupeReads?: boolean
  /** 透传到每个请求的默认配置（如 silentError）。 */
  defaultConfig?: CustomAxiosRequestConfig
  /** 创建请求的默认 body 变换（少数域需要包裹一层）。 */
  transformCreateBody?: (body: CreateBody) => unknown
}

export interface ResourceClient<ListParams, ListResult, Item, CreateBody, UpdateBody, DeleteResult> {
  list: (params?: ListParams, config?: CustomAxiosRequestConfig) => Promise<FlatResponseData<ListResult>>
  detail: (id: string | number, config?: CustomAxiosRequestConfig) => Promise<FlatResponseData<Item>>
  create: (body: CreateBody, config?: CustomAxiosRequestConfig) => Promise<FlatResponseData<Item>>
  update: (body: UpdateBody, config?: CustomAxiosRequestConfig) => Promise<FlatResponseData<Item>>
  remove: (id: string | number, config?: CustomAxiosRequestConfig) => Promise<FlatResponseData<DeleteResult>>
}

/**
 * 构造一组类型化的 REST 资源端点。
 */
export function createResource<
  ListParams = void,
  ListResult = unknown,
  Item = unknown,
  CreateBody = unknown,
  UpdateBody = unknown,
  DeleteResult = unknown
>(options: ResourceOptions<CreateBody>): ResourceClient<ListParams, ListResult, Item, CreateBody, UpdateBody, DeleteResult> {
  const {
    collection,
    listPath,
    itemPath,
    updateStyle = 'body-id',
    dedupeReads = true,
    defaultConfig,
    transformCreateBody
  } = options

  const resolveItemPath = (id: string | number) => itemPath?.(id) ?? `${collection}/${encodeId(id)}`
  const mergeConfig = (config?: CustomAxiosRequestConfig): CustomAxiosRequestConfig => ({
    ...defaultConfig,
    ...config
  })

  const read = <T>(url: string, config?: CustomAxiosRequestConfig): Promise<FlatResponseData<T>> => {
    if (!dedupeReads) return request.get<T>(url, mergeConfig(config))

    return dedupeGet<T>(buildGetDedupeKey(url, config), () => request.get<T>(url, mergeConfig(config)))
  }

  return {
    list: (params, config) => read<ListResult>(listPath ?? collection, { ...mergeConfig(config), params }),

    detail: (id, config) => read<Item>(resolveItemPath(id), config),

    create: (body, config) =>
      request.post<Item>(
        collection,
        transformCreateBody ? transformCreateBody(body) : body,
        mergeConfig(config)
      ),

    update: (body, config) => {
      if (updateStyle === 'item-path') {
        const { id, ...rest } = body as UpdateBody & { id: string | number }
        return request.put<Item>(resolveItemPath(id), rest, mergeConfig(config))
      }

      return request.put<Item>(collection, body, mergeConfig(config))
    },

    remove: (id, config) => request.delete<DeleteResult>(resolveItemPath(id), mergeConfig(config))
  }
}
