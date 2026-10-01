<!--
  文件用途：原生看板创建弹窗（从 index.vue 拆出）。
  核心逻辑：名称/描述/租户表单 → provider.createDashboard（rendererData 固定为 NATIVE_BOARD_CONFIG）；
  成功后 emit created 由页面路由到查看器；provider 失败或返回空白 ID 时保持弹窗打开。
  SYS_ADMIN 必须显式选择租户：prefill 由页面在打开时按当前列表租户过滤注入。
-->
<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { NButton, NForm, NFormItem, NInput, NModal, NSelect, useMessage } from 'naive-ui'
import type { SelectOption } from 'naive-ui'
import { $t } from '@/locales'
import { getDefaultVisualizationProviderFacade } from '@/service/visualization-provider/composition'
import { NATIVE_BOARD_PROJECT_ID } from '@/service/visualization-provider/provider-ids'

defineOptions({ name: 'NativeBoardCreateModal' })

const props = defineProps<{
  show: boolean
  isSysAdmin: boolean
  tenantOptions: SelectOption[]
  loadingTenants: boolean
  /** 打开弹窗时注入的初始租户（取自列表过滤上下文）。 */
  prefillTenantId: string | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  created: [id: string]
}>()

const message = useMessage()
const providerFacade = getDefaultVisualizationProviderFacade()

const NATIVE_BOARD_CONFIG = { version: 1, columns: 24, rowHeight: 60, widgets: [] }

const creating = ref(false)
const createTenantId = ref<string | null>(null)
const createForm = reactive({ name: '', description: '' })

// 打开时重置表单，并按页面注入的 prefill 预选租户：
// SYS_ADMIN 的列表租户过滤即创建上下文，避免列表页与创建请求之间丢失 tenant_id。
watch(
  () => props.show,
  (show) => {
    if (!show) return
    createForm.name = ''
    createForm.description = ''
    if (props.isSysAdmin) {
      createTenantId.value = props.prefillTenantId
    } else {
      createTenantId.value = null
    }
  }
)

const close = () => {
  emit('update:show', false)
}

async function handleCreate() {
  if (creating.value) return
  const name = createForm.name.trim()
  if (!name || name.length > 255) {
    message.error($t('custom.nativeBoards.nameInvalid'))
    return
  }
  if (createForm.description.length > 500) {
    message.error($t('custom.nativeBoards.descriptionInvalid'))
    return
  }
  const tenantId = createTenantId.value?.trim() || ''
  if (props.isSysAdmin && !tenantId) {
    message.error('Select a tenant before creating a native board')
    return
  }

  creating.value = true
  try {
    const result = await providerFacade.execute((provider) =>
      provider.createDashboard({
        name,
        description: createForm.description,
        projectId: NATIVE_BOARD_PROJECT_ID,
        rendererData: NATIVE_BOARD_CONFIG,
        ...(tenantId ? { tenantId } : {})
      })
    )
    if (!result.ok || !result.data.id.trim()) {
      message.error($t('custom.nativeBoards.createFailed'))
      return
    }
    message.success($t('custom.nativeBoards.createSuccess'))
    close()
    emit('created', result.data.id)
  } catch {
    message.error($t('custom.nativeBoards.createFailed'))
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <NModal
    :show="props.show"
    preset="card"
    :title="$t('custom.nativeBoards.createTitle')"
    class="w-500px"
    data-testid="native-board-create-modal"
    @update:show="emit('update:show', $event)"
  >
    <NForm :model="createForm">
      <NFormItem v-if="props.isSysAdmin" label="Tenant" path="tenantId">
        <NSelect
          v-model:value="createTenantId"
          :options="props.tenantOptions"
          :loading="props.loadingTenants"
          filterable
          placeholder="Select tenant"
          data-testid="native-board-tenant-select"
        />
      </NFormItem>
      <NFormItem :label="$t('custom.nativeBoards.name')" path="name">
        <NInput
          v-model:value="createForm.name"
          :maxlength="255"
          show-count
          :placeholder="$t('custom.nativeBoards.namePlaceholder')"
          data-testid="native-board-name"
        />
      </NFormItem>
      <NFormItem :label="$t('custom.nativeBoards.description')" path="description">
        <NInput
          v-model:value="createForm.description"
          type="textarea"
          :maxlength="500"
          show-count
          :placeholder="$t('custom.nativeBoards.descriptionPlaceholder')"
          data-testid="native-board-description"
        />
      </NFormItem>
    </NForm>
    <template #footer>
      <div class="flex justify-end gap-2">
        <NButton :disabled="creating" @click="close">{{ $t('custom.nativeBoards.cancel') }}</NButton>
        <NButton type="primary" :loading="creating" data-testid="native-board-submit" @click="handleCreate">
          {{ $t('custom.nativeBoards.submit') }}
        </NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped></style>
