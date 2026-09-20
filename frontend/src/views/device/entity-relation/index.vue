<!--
  通用实体关系管理页（ROADMAP P1.1）
  左：新建关系表单（类型白名单 + 自环拦截 + 元数据大小校验）
  右：关系列表（按端点/类型/方向过滤）与删除
  底部：按实体删除，必须显式选择 protect / cascade

  设计取舍：
  1. 校验规则全部来自 entity-relation-model.ts，与后端 ValidateEntityRelation 对齐，
     页面不另立一套——另立一套会让用户在本该被拒的地方点得动，再收到看不懂的后端错误。
  2. 反向关系只做**提示**，不自动创建。自动补边会让关系图里出现用户没创建过的边。
  3. 「按实体删除」默认 protect：有关系即拒绝不是 bug，是保护生效，
     所以这里把两种策略的后果直接写在界面上，而不是失败了才报一句"删除失败"。
-->
<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NPopconfirm,
  NSelect,
  NSpace,
  NTag,
  useMessage
} from 'naive-ui'
import {
  createEntityRelation,
  deleteEntityRelation,
  deleteEntityRelationsForEntity,
  listEntityRelations
} from '@/service/api'
import { $t } from '@/locales'
import {
  ENTITY_TYPES,
  describeDeletionPolicy,
  findReverse,
  metadataByteLength,
  parseMetadata,
  validateDraft,
  type EntityRelation,
  type EntityRelationDraft,
  type EntityType
} from './entity-relation-model'

defineOptions({ name: 'DeviceEntityRelation' })

const message = useMessage()

const loading = ref(false)
const submitting = ref(false)
const rows = ref<EntityRelation[]>([])

const draft = reactive<EntityRelationDraft>({
  from_type: 'device',
  from_id: '',
  relation_type: '',
  to_type: 'asset',
  to_id: '',
  metadata: ''
})

const typeOptions = ENTITY_TYPES.map((value) => ({ label: value, value }))

/** 与后端校验同源的错误，字段级展示。 */
const errors = computed(() => validateDraft(draft).errors)

/** 反向关系提示：存在则告诉用户需要显式创建，不代为创建。 */
const reverseHint = computed(() => {
  const found = findReverse(rows.value, draft)
  return found ? $t('custom.entityRelation.reverseExists') : ''
})

const metadataBytes = computed(() => metadataByteLength(draft.metadata))

const canSubmit = computed(() => Object.keys(errors.value).length === 0 && !submitting.value)

async function load() {
  loading.value = true
  try {
    const { data, error } = await listEntityRelations({ limit: 200 })
    if (error) {
      message.error(
        typeof error === 'object' && error && 'message' in error ? String(error.message) : $t('common.loadFailed')
      )
      return
    }
    rows.value = data?.list ?? []
  } finally {
    loading.value = false
  }
}

async function submit() {
  const result = validateDraft(draft)
  if (!result.ok) {
    message.error(Object.values(result.errors)[0] ?? $t('common.validateFailed'))
    return
  }
  let metadata: string | undefined
  try {
    metadata = parseMetadata(draft.metadata)
  } catch (error) {
    message.error(error instanceof Error ? error.message : String(error))
    return
  }
  submitting.value = true
  try {
    const { error } = await createEntityRelation({
      from_type: draft.from_type as EntityType,
      from_id: draft.from_id.trim(),
      relation_type: draft.relation_type.trim(),
      to_type: draft.to_type as EntityType,
      to_id: draft.to_id.trim(),
      metadata
    })
    if (error) {
      message.error(
        typeof error === 'object' && error && 'message' in error ? String(error.message) : $t('common.addFailed')
      )
      return
    }
    message.success($t('common.addSuccess'))
    draft.from_id = ''
    draft.to_id = ''
    draft.relation_type = ''
    draft.metadata = ''
    await load()
  } finally {
    submitting.value = false
  }
}

async function remove(row: EntityRelation) {
  const { error } = await deleteEntityRelation(row.id)
  if (error) {
    message.error(
      typeof error === 'object' && error && 'message' in error ? String(error.message) : $t('common.deleteFailed')
    )
    return
  }
  message.success($t('common.deleteSuccess'))
  await load()
}

// ---------- 按实体删除 ----------
const purge = reactive({
  entity_type: 'device' as EntityType,
  entity_id: '',
  policy: 'protect' as 'protect' | 'cascade'
})

const policyOptions = computed(() => [
  { label: $t('custom.entityRelation.policyProtect'), value: 'protect' },
  { label: $t('custom.entityRelation.policyCascade'), value: 'cascade' }
])

async function purgeEntity() {
  if (!purge.entity_id.trim()) {
    message.error($t('custom.entityRelation.entityIdRequired'))
    return
  }
  const { error } = await deleteEntityRelationsForEntity({
    entity_type: purge.entity_type,
    entity_id: purge.entity_id.trim(),
    policy: purge.policy
  })
  if (error) {
    // protect 命中时后端会拒绝：这是保护生效，必须讲清楚而不是笼统报"删除失败"。
    message.error(
      typeof error === 'object' && error && 'message' in error ? String(error.message) : $t('common.deleteFailed')
    )
    return
  }
  message.success($t('common.deleteSuccess'))
  await load()
}

const columns = computed<DataTableColumns<EntityRelation>>(() => [
  {
    title: $t('custom.entityRelation.from'),
    key: 'from',
    render: (row) =>
      h(NSpace, { size: 4 }, () => [
        h(NTag, { size: 'small', bordered: false }, () => row.from_type),
        h('span', null, row.from_id)
      ])
  },
  { title: $t('custom.entityRelation.relationType'), key: 'relation_type' },
  {
    title: $t('custom.entityRelation.to'),
    key: 'to',
    render: (row) =>
      h(NSpace, { size: 4 }, () => [
        h(NTag, { size: 'small', bordered: false }, () => row.to_type),
        h('span', null, row.to_id)
      ])
  },
  {
    title: $t('common.action'),
    key: 'actions',
    width: 110,
    render: (row) =>
      h(
        NPopconfirm,
        { onPositiveClick: () => remove(row) },
        {
          trigger: () => h(NButton, { size: 'small', type: 'error', quaternary: true }, () => $t('common.delete')),
          default: () => $t('custom.entityRelation.confirmDelete')
        }
      )
  }
])

onMounted(load)
</script>

<template>
  <div class="grid grid-cols-1 gap-4 lg:grid-cols-[380px_1fr]">
    <NCard :title="$t('custom.entityRelation.createTitle')" size="small">
      <NForm label-placement="top">
        <NFormItem
          :label="$t('custom.entityRelation.from')"
          :feedback="errors.from_type"
          :validation-status="errors.from_type ? 'error' : undefined"
        >
          <NSpace vertical class="w-full">
            <NSelect v-model:value="draft.from_type" :options="typeOptions" />
            <NInput v-model:value="draft.from_id" placeholder="device id" />
          </NSpace>
        </NFormItem>
        <NFormItem
          :label="$t('custom.entityRelation.relationType')"
          :feedback="errors.relation_type"
          :validation-status="errors.relation_type ? 'error' : undefined"
        >
          <NInput v-model:value="draft.relation_type" placeholder="belongs_to" />
        </NFormItem>
        <NFormItem
          :label="$t('custom.entityRelation.to')"
          :feedback="errors.to_id || errors.to_type"
          :validation-status="errors.to_id || errors.to_type ? 'error' : undefined"
        >
          <NSpace vertical class="w-full">
            <NSelect v-model:value="draft.to_type" :options="typeOptions" />
            <NInput v-model:value="draft.to_id" placeholder="asset id" />
          </NSpace>
        </NFormItem>
        <NFormItem
          :label="$t('custom.entityRelation.metadata')"
          :feedback="errors.metadata"
          :validation-status="errors.metadata ? 'error' : undefined"
        >
          <NInput v-model:value="draft.metadata" type="textarea" :rows="3" placeholder='{"key":"value"}' />
        </NFormItem>
        <NAlert v-if="reverseHint" type="warning" :bordered="false" class="mb-3">{{ reverseHint }}</NAlert>
        <NButton type="primary" block :disabled="!canSubmit" :loading="submitting" @click="submit">
          {{ $t('common.add') }}
        </NButton>
        <div class="mt-2 text-xs text-gray-400">
          {{ $t('custom.entityRelation.metadataBytes') }}: {{ metadataBytes }}
        </div>
      </NForm>
    </NCard>

    <NSpace vertical :size="16">
      <NCard :title="$t('custom.entityRelation.listTitle')" size="small">
        <NDataTable :columns="columns" :data="rows" :loading="loading" size="small" :bordered="false" />
      </NCard>

      <NCard :title="$t('custom.entityRelation.purgeTitle')" size="small">
        <NAlert type="info" :bordered="false" class="mb-3">{{ describeDeletionPolicy(purge.policy) }}</NAlert>
        <NSpace align="center">
          <NSelect v-model:value="purge.entity_type" :options="typeOptions" class="w-36" />
          <NInput v-model:value="purge.entity_id" :placeholder="$t('custom.entityRelation.entityId')" class="w-64" />
          <NSelect v-model:value="purge.policy" :options="policyOptions" class="w-48" />
          <NPopconfirm @positive-click="purgeEntity">
            <template #trigger>
              <NButton type="error">{{ $t('common.delete') }}</NButton>
            </template>
            {{ $t('custom.entityRelation.confirmPurge') }}
          </NPopconfirm>
        </NSpace>
      </NCard>
    </NSpace>
  </div>
</template>
