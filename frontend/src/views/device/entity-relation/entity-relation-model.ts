/**
 * 文件用途: 通用实体关系的纯前端模型（ROADMAP P1.1）。
 * 核心逻辑: 实体类型白名单、写入前校验、反向关系判定、删除策略语义。
 * 关键注意事项:
 *  1. **规则必须与后端 `internal/model/entity_relation.go` 保持一致**。
 *     这里不是"前端自己定一套"，任何偏离都会让用户在本该被拒的地方点得动，
 *     然后收到一个看不懂的后端错误——等于把校验失败的成本推给用户。
 *  2. **自环的判定是"同类型且同 ID"**：不同类型但同 ID 不算自环，
 *     因为 ID 空间本身按类型隔离。写成"ID 相同即自环"会误伤 device:X -> asset:X。
 *  3. **反向关系不会自动生成**，只能显式写入。本模型只负责"提示这是反向"，
 *     绝不自动补一条——自动补会让关系图里出现用户没创建过的边。
 *  4. 删除策略默认 protect（有关系则拒绝），cascade 必须由用户显式声明。
 */

/** 受控实体类型白名单，顺序与后端 AllowedEntityTypes() 一致。 */
export const ENTITY_TYPES = ['device', 'asset', 'customer', 'gateway'] as const

export type EntityType = (typeof ENTITY_TYPES)[number]

/** 关系类型最大长度（后端 entityRelationMaxTypeLength）。 */
export const RELATION_TYPE_MAX_LENGTH = 64

/** 元数据最大字节数（后端 entityRelationMaxMetadataBytes）。 */
export const METADATA_MAX_BYTES = 4096

/** 删除策略。 */
export type DeletionPolicy = 'protect' | 'cascade'

/** 查询方向。 */
export type RelationDirection = '' | 'from' | 'to'

export interface EntityRelation {
  id: string
  tenant_id: string
  from_type: EntityType
  from_id: string
  relation_type: string
  to_type: EntityType
  to_id: string
  metadata?: string | null
  created_at: string
  updated_at: string
}

/** 新建关系的表单载荷。 */
export interface EntityRelationDraft {
  from_type: EntityType | ''
  from_id: string
  relation_type: string
  to_type: EntityType | ''
  to_id: string
  metadata: string
}

export interface ValidationResult {
  ok: boolean
  /** 字段级错误，key 为字段名。空对象表示通过。 */
  errors: Record<string, string>
}

const ok = (): ValidationResult => ({ ok: true, errors: {} })

const fail = (field: string, message: string): ValidationResult => ({ ok: false, errors: { [field]: message } })

/** 判断实体类型是否合法。 */
export function isAllowedEntityType(value: string): value is EntityType {
  return (ENTITY_TYPES as readonly string[]).includes(value)
}

/**
 * 校验新建关系草稿。
 * 与后端 ValidateEntityRelation 对齐：必填 → 类型白名单 → 自环 → 关系类型长度 → 元数据大小。
 */
export function validateDraft(draft: EntityRelationDraft): ValidationResult {
  if (!draft.from_id.trim()) return fail('from_id', '起点 ID 必填')
  if (!draft.to_id.trim()) return fail('to_id', '终点 ID 必填')
  if (!draft.relation_type.trim()) return fail('relation_type', '关系类型必填')

  if (!isAllowedEntityType(draft.from_type)) return fail('from_type', '起点类型不在白名单内')
  if (!isAllowedEntityType(draft.to_type)) return fail('to_type', '终点类型不在白名单内')
  if (!isAllowedEntityType(draft.from_type) || !isAllowedEntityType(draft.to_type)) {
    return fail('to_type', '终点类型不在白名单内')
  }

  // 自环只看"同类型且同 ID"：device:X -> asset:X 是合法的。
  if (draft.from_type === draft.to_type && draft.from_id.trim() === draft.to_id.trim()) {
    return fail('to_id', '不能连接到自己')
  }

  if (draft.relation_type.trim().length > RELATION_TYPE_MAX_LENGTH) {
    return fail('relation_type', `关系类型不得超过 ${RELATION_TYPE_MAX_LENGTH} 个字符`)
  }

  const bytes = metadataByteLength(draft.metadata)
  if (bytes > METADATA_MAX_BYTES) {
    return fail('metadata', `元数据不得超过 ${METADATA_MAX_BYTES} 字节`)
  }

  return ok()
}

/** 计算字符串 UTF-8 字节数；空串按 0 处理（与后端"无元数据"同一口径）。 */
export function metadataByteLength(raw: string): number {
  if (!raw || !raw.trim()) return 0
  return new TextEncoder().encode(raw).length
}

/**
 * 解析元数据输入框内容。
 * 空串返回 undefined（不写 metadata），非法 JSON 直接抛错——
 * 静默接受非法 JSON 等于让用户以为填对了，直到后端拒绝才知道。
 */
export function parseMetadata(raw: string): string | undefined {
  if (!raw || !raw.trim()) return undefined
  try {
    const parsed = JSON.parse(raw)
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      throw new Error('metadata must be a JSON object')
    }
    return raw.trim()
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error)
    throw new Error(`元数据必须是合法 JSON 对象：${reason}`)
  }
}

/** 判断两条关系是否互为反向（只比较端点与类型，不比较元数据与租户）。 */
export function isReverseOf(a: EntityRelation, b: EntityRelation): boolean {
  return (
    a.from_type === b.to_type &&
    a.from_id === b.to_id &&
    a.to_type === b.from_type &&
    a.to_id === b.from_id &&
    a.relation_type === b.relation_type
  )
}

/**
 * 在已有关系中找出草稿的反向边是否存在。
 * 用于提示"反向需要显式创建"，而不是替用户创建。
 */
export function findReverse(relations: EntityRelation[], draft: EntityRelationDraft): EntityRelation | undefined {
  if (!isAllowedEntityType(draft.from_type) || !isAllowedEntityType(draft.to_type)) return undefined
  const ghost: EntityRelation = {
    id: '',
    tenant_id: '',
    from_type: draft.from_type,
    from_id: draft.from_id,
    relation_type: draft.relation_type,
    to_type: draft.to_type,
    to_id: draft.to_id,
    created_at: '',
    updated_at: ''
  }
  return relations.find(item => isReverseOf(ghost, item))
}

/** 关系列表去重键：端点与类型相同即视为同一条（后端唯一约束口径）。 */
export function relationKey(relation: Pick<EntityRelation, 'from_type' | 'from_id' | 'relation_type' | 'to_type' | 'to_id'>): string {
  return [relation.from_type, relation.from_id, relation.relation_type, relation.to_type, relation.to_id].join('|')
}

/**
 * 删除策略的文案解释。
 * 后端默认 protect，cascade 必须显式声明——这里把后果讲清楚，
 * 避免用户以为"删除实体"只是删掉实体本身。
 */
export function describeDeletionPolicy(policy: DeletionPolicy): string {
  return policy === 'cascade'
    ? '级联：连同该实体挂载的全部关系一并删除，不可恢复'
    : '保护：只要还存在任何关系就拒绝删除'
}
