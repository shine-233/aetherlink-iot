<!--
文件用途：提供权限元素管理页的新增/编辑弹窗，负责采集元素层级、权限标识、图标与展示字段等核心元数据。
核心逻辑：组件在打开时按 `type` 回填表单，通过新增或编辑接口提交元素定义，并在成功后通知父页面刷新列表。
关键注意事项：
1. `authority` 是角色授权链路中的敏感权限字段，提交前会被序列化成字符串以匹配后端接口契约。
2. 父级元素选项通过额外查询获取，避免直接依赖当前页表格分页结果造成树形选择不完整。
3. 编辑场景使用深拷贝回填，避免用户在弹窗中尚未确认时就污染父页行数据。
静态审查建议：
1. 当前组件挂载即请求父级选项，后续可改成按弹窗打开懒加载并加缓存，减少无效请求。
2. 表单字段里 `param1`、`param2`、`param3` 仍沿用通用命名，可在后续重构中补语义化映射层。
-->
<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type { FormInst } from 'naive-ui'
import { routeSysFlagOptions, routeTypeOptions } from '@/constants/business'
import { addElement, editElement, fetchElementList } from '@/service/api/route'
import { smartDeepClone as deepClone } from '@/utils/deep-clone'
import { createRequiredFormRule } from '@/utils/form/rule'
import { icons } from '@/plugins/icon/icons'
import { $t } from '@/locales'

export interface Props {
  /** 弹窗可见性 */
  visible: boolean
  /** 弹窗类型 add: 新增 edit: 编辑 */
  type?: 'add' | 'edit'
  /** 编辑的表格行数据 */
  editData?: CustomRoute.Route | null
}

export type ModalType = NonNullable<Props['type']>

defineOptions({ name: 'TableActionModal' })

const common_cancel = $t('common.cancel')
const common_confirm = $t('common.confirm')

const props = withDefaults(defineProps<Props>(), {
  type: 'add',
  editData: null
})

interface Emits {
  (e: 'update:visible', visible: boolean): void

  (e: 'success'): void
}

const emit = defineEmits<Emits>()

const modalVisible = computed({
  get() {
    return props.visible
  },
  set(visible) {
    emit('update:visible', visible)
  }
})
const closeModal = () => {
  modalVisible.value = false
}

const title = computed(() => {
  const titles: Record<ModalType, string> = {
    add: $t('common.add'),
    edit: $t('common.edit')
  }
  return titles[props.type]
})

const parentOptions = ref<CustomRoute.Route[]>([])

// 父级元素选项单独拉全量近似数据，避免受当前页分页限制导致树选择缺项。
async function getTableData() {
  const { data } = await fetchElementList({
    page: 1,
    page_size: 99
  })
  if (data) {
    const list: Api.Route.MenuRoute[] = data.list
    parentOptions.value = list
  }
}

getTableData()

const formRef = ref<HTMLElement & FormInst>()

type FormModel = Pick<
  CustomRoute.Route,
  | 'parent_id'
  | 'element_code'
  | 'param1'
  | 'element_type'
  | 'authority'
  | 'route_path'
  | 'remark'
  | 'multilingual'
  | 'param2'
  | 'param3'
  | 'orders'
  | 'description'
>

const formModel = reactive<FormModel>(createDefaultFormModel())

const rules = {
  description: createRequiredFormRule($t('common.pleaseCheckValue')),
  element_code: createRequiredFormRule($t('common.pleaseCheckValue')),
  // 权限标识为空会直接影响角色授权和页面显隐，作为必填项强约束。
  authority: createRequiredFormRule($t('common.pleaseCheckValue'))
}

function createDefaultFormModel(): FormModel {
  return {
    parent_id: '0',
    element_code: '',
    param1: '',
    multilingual: 'default',
    param2: '',
    param3: '0',
    orders: 1,
    description: '',
    element_type: 1,
    authority: [],
    route_path: '',
    remark: ''
  }
}

function handleUpdateFormModel(model: Partial<FormModel>) {
  Object.assign(formModel, model)
}

function handleUpdateFormModelByModalType() {
  const handlers: Record<ModalType, () => void> = {
    add: () => {
      // 新增时使用最小默认集，确保元素层级、类型和隐藏标记有稳定初始值。
      const defaultFormModel = createDefaultFormModel()
      handleUpdateFormModel(defaultFormModel)
    },
    edit: () => {
      if (props.editData) {
        // 编辑时以深拷贝后的父行数据为准，避免直接引用父页对象。
        handleUpdateFormModel(props.editData)
      }
    }
  }

  handlers[props.type]()
}

async function handleSubmit() {
  await formRef.value?.validate()
  const formData = deepClone(formModel)
  formData.parent_id = formData.parent_id || '0'
  // 后端当前约定 `authority` 以 JSON 字符串接收；若接口契约调整，这里需要同步修改。
  formData.authority = JSON.stringify(formData.authority)
  let data: any
  if (props.type === 'add') {
    data = await addElement(formData)
  } else if (props.type === 'edit') {
    data = await editElement(formData)
  }
  if (!data.error) {
    window.$message?.success(data.msg)
    emit('success')
  }
  closeModal()
}

watch(
  () => props.visible,
  newValue => {
    if (newValue) {
      // 每次打开都重新回填，避免新增/编辑之间残留旧字段影响本次提交。
      handleUpdateFormModelByModalType()
    }
  }
)
</script>

<template>
  <NModal v-model:show="modalVisible" preset="card" :title="title" class="w-800px">
    <NForm ref="formRef" label-placement="left" :label-width="120" :model="formModel" :rules="rules">
      <NGrid :cols="24" :x-gap="18">
        <NFormItemGridItem :span="12" :label="$t('page.manage.menu.form.parent')" path="parent_id">
          <NTreeSelect
            v-model:value="formModel.parent_id"
            :options="parentOptions"
            label-field="description"
            key-field="id"
            clearable
          />
        </NFormItemGridItem>

        <NFormItemGridItem :span="12" :label="$t('page.manage.menu.form.title')" path="description">
          <NInput v-model:value="formModel.description" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="12" :label="$t('page.manage.menu.form.multilingual')" path="multilingual">
          <NInput v-model:value="formModel.multilingual" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="12" :label="$t('page.manage.menu.form.name')" path="element_code">
          <NInput v-model:value="formModel.element_code" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="12" :label="$t('page.manage.menu.form.path')" path="param1">
          <NInput v-model:value="formModel.param1" />
        </NFormItemGridItem>
        <!--
        <NFormItemGridItem :span="12" :label="$t('page.manage.menu.form.route_path')">
          <NInput v-model:value="formModel.route_path" />
        </NFormItemGridItem>
        -->
        <NFormItemGridItem :span="12" :label="$t('page.manage.menu.form.icon')" path="param2">
          <IconSelect v-model:value="formModel.param2" :icons="icons" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="12" :label="$t('page.manage.menu.form.order')" path="orders">
          <NInputNumber v-model:value="formModel.orders" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="12" :label="$t('page.manage.menu.form.type')" path="element_type">
          <NRadioGroup v-model:value="formModel.element_type">
            <NRadio v-for="item in routeTypeOptions" :key="item.value" :value="Number(item.value)">
              {{ item.label }}
            </NRadio>
          </NRadioGroup>
        </NFormItemGridItem>
        <NFormItemGridItem :span="12" :label="$t('page.manage.menu.form.authority')" path="authority">
          <NCheckboxGroup v-model:value="formModel.authority">
            <NSpace item-style="display: flex;">
              <NCheckbox v-for="item in routeSysFlagOptions" :key="item.value" :value="item.value" :label="item.label">
                {{ item.label }}
              </NCheckbox>
            </NSpace>
          </NCheckboxGroup>
        </NFormItemGridItem>
        <NFormItemGridItem :span="12" :label="$t('page.manage.menu.hideInMenu')" path="param3">
          <n-switch v-model:value="formModel.param3" checked-value="1" unchecked-value="0" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('common.description')">
          <NInput v-model:value="formModel.remark" type="textarea" />
        </NFormItemGridItem>
      </NGrid>
      <NSpace class="w-full pt-16px" :size="24" justify="end">
        <NButton class="w-72px" @click="closeModal">
          {{ common_cancel }}
        </NButton>
        <NButton class="w-72px" type="primary" @click="handleSubmit">
          {{ common_confirm }}
        </NButton>
      </NSpace>
    </NForm>
  </NModal>
</template>

<style scoped></style>
