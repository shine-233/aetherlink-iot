<!--
  文件用途: 设备配置页中的“数据处理”模块，负责脚本列表展示、启停、增删改、调试。
  核心逻辑: 列表状态交给 useListPage（筛选/分页/过期请求丢弃），弹窗表单与调试由
    DataHandleScriptModal + useScriptQuiz 承载，本文件只做编排。
  保存链路: 打开弹窗时先重置默认表单，编辑时再回填记录；保存前统一校验并补齐 `device_config_id`，
    随后按是否存在 `id` 分流到新增或编辑接口，成功后关闭弹窗并刷新列表。
  关键注意事项: 脚本内容会直接影响平台与设备之间的数据语义，注释和默认模板需要帮助维护者快速
    识别处理类型、调试输入和返回结果的含义。
-->
<script setup lang="ts">
import { computed, getCurrentInstance, nextTick, onMounted, ref, watch } from 'vue'
import { type FormInst, useDialog } from 'naive-ui'
import { PencilOutline as editIcon, TrashOutline as trashIcon } from '@vicons/ionicons5'
import ItemCard from '@/components/dev-card-item/index.vue'
import { useListPage } from '@/components/data-table-page/useListPage'
import { dataScriptAdd, dataScriptDel, dataScriptEdit, getDataScriptList, setDeviceScriptEnable } from '@/service/api/device'
import { $t } from '@/locales'
import DataHandleScriptModal from './DataHandleScriptModal.vue'
import { useScriptQuiz } from './useScriptQuiz'
import { createScriptFormRules, createScriptTypeOptions, defaultScriptForm, useScriptEditorControls } from './script-form-model'

const dialog = useDialog()

interface Props {
  configInfo?: object | any
}

const props = withDefaults(defineProps<Props>(), {
  configInfo: null
})

const configFormRef = ref<HTMLElement & FormInst>()
// Templates unwrap refs, so the modal receives a setter instead of the ref itself.
const setConfigFormRef = (instance: any) => {
  configFormRef.value = instance
}
const modalTitle = ref($t('generate.add'))
const configForm: any = ref({})
const scripTypeOpt = ref(createScriptTypeOptions())
const configFormRules = ref(createScriptFormRules())
const { editorOptions, toggleMinimap, toggleWordWrap, changeFontSize } = useScriptEditorControls()

const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})

interface DataScriptItem {
  id: string
  name: string
  content: string
  description: string
  device_config_id: string
  enable_flag: string
  script_type: string
}

// 脚本列表：筛选、分页、过期请求丢弃由 useListPage 统一管理。
// 列表查询依赖父级传入的设备配置 id；每次请求前从 props 读取，避免 props 异步更新后沿用旧值。
const scripts = useListPage<DataScriptItem, { device_config_id: string; script_type: string }>({
  initialQuery: () => ({ device_config_id: '', script_type: '' }),
  serialize: q => {
    q.device_config_id = props.configInfo.id
    return { ...q }
  },
  fetcher: async params => {
    const { data, error } = await getDataScriptList(params)
    // 查询失败时清空列表，确保界面状态与“当前没有可展示结果”保持一致，而不是残留旧数据。
    if (error || !data) return { list: [], total: 0 }
    // 接口返回结构容错：优先使用后端 total，缺失时退回当前 list 长度。
    const list = Array.isArray(data.list) ? data.list : []
    return { list, total: typeof data.total === 'number' ? data.total : list.length }
  }
})
const { rows: dataScriptList, total: dataScriptTotal, load: queryDataScriptList, search: searchDataScript } = scripts
// 模板/弹窗沿用 queryData.value.{script_type,page,page_size} 形状，直接映射到 useListPage 状态。
const queryData = computed(() => scripts.flatQuery)

const { doQuiz } = useScriptQuiz(configFormRef, configForm)

// 弹窗显隐与 configForm 一起构成“列表态 <-> 编辑态”的切换边界。
// 列表页只维护筛选条件；进入弹窗后再承载脚本内容、调试入参和保存提交数据。
const showModal = ref(false)

const openModal = (type: any, item: any) => {
  modalTitle.value = type
  // 先用默认值初始化表单
  configForm.value = defaultScriptForm()

  if (modalTitle.value === $t('common.edit')) {
    // 编辑模式：基于列表项深拷贝回填，避免弹窗里的双向绑定直接污染卡片区数据。
    configForm.value = JSON.parse(JSON.stringify(item))
  } else {
    // 新增模式：如果当前列表已经按脚本类型筛选，则沿用筛选值，减少重复选择。
    if (queryData.value.script_type) {
      configForm.value.script_type = queryData.value.script_type
    }
  }
  // 先让弹窗渲染，再在 nextTick 中清理校验残留，避免旧错误态闪现到新会话里。
  showModal.value = true

  nextTick(() => {
    configFormRef.value?.restoreValidation()
  })
}

const handleChange = async (item: object) => {
  // 启停操作直接提交当前项，不额外刷新列表，默认信任开关组件已同步最新 enable_flag。
  await setDeviceScriptEnable(item)
}

const handleClose = () => {
  // 关闭弹窗时只回收校验和显示状态，真实表单数据在下次 openModal 时重新初始化。
  configFormRef.value?.restoreValidation()
  showModal.value = false
}

// 保存链路:
// 1. 先做前端表单校验；
// 2. 补齐当前设备 id，确保脚本与设备配置绑定；
// 3. 按 id 是否存在分流到新增/编辑接口；
// 4. 接口成功后统一关闭弹窗并刷新列表，让卡片区状态回到服务端真值。
const handleSubmit = async () => {
  await configFormRef?.value?.validate()
  configForm.value.device_config_id = props.configInfo.id
  if (!configForm.value.id) {
    const res = await dataScriptAdd(configForm.value)
    if (!res.error) {
      // message.success('新增成功');
      handleClose()
      searchDataScript()
    }
  } else {
    const res = await dataScriptEdit(configForm.value)
    if (!res.error) {
      handleClose()
      // message.success('修改成功');
      searchDataScript()
    }
  }
}

const deleteData = async (item: any) => {
  dialog.warning({
    title: $t('common.tip'),
    content: $t('common.deleteProcessing'),
    positiveText: $t('device_template.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: async () => {
      await dataScriptDel({ id: item.id })
      // message.success($t('custom.grouping_details.operationSuccess'));
      searchDataScript()
    }
  })
}

// 处理类型筛选变化时回到第一页重新拉取；分页变化由 useListPage 自己驱动。初次挂载时补一次首屏查询。
watch(
  () => scripts.query.script_type,
  () => searchDataScript()
)
onMounted(() => {
  queryDataScriptList()
})
</script>

<template>
  <div class="m-b-20px flex items-center gap-20px">
    <n-select v-model:value="queryData.script_type" :options="scripTypeOpt" class="max-w-40" clearable />
    <NButton type="primary" @click="openModal($t('common.add'), null)">
      {{ $t('generate.add-data-processing') }}
    </NButton>
  </div>
  <n-empty v-if="dataScriptList.length === 0" size="huge" :description="$t('common.noData')"></n-empty>
  <NGrid v-else x-gap="24" y-gap="16" cols="1 s:2 m:3 l:4" responsive="screen">
    <NGridItem v-for="item in dataScriptList" :key="item.id">
      <ItemCard
        :title="item.name"
        :status-active="true"
        :status-type="'success'"
        :is-status="false"
        :hide-footer-left="true"
        hoverable
      >
        <template #default>
          <div class="item-desc">{{ item.description }}</div>
        </template>
        <!-- 右上角开关 -->
        <template #top-right-icon>
          <NSwitch
            v-model:value="item.enable_flag"
            checked-value="Y"
            unchecked-value="N"
            @update-value="handleChange(item)"
          />
        </template>

        <!-- 底部操作按钮 -->
        <template #footer>
          <div class="flex items-center gap-2 w-full justify-between">
            <NButton
              size="small"
              quaternary
              circle
              :aria-label="$t('common.edit')"
              @click="openModal($t('common.edit'), item)"
            >
              <template #icon>
                <n-icon color="#888">
                  <editIcon />
                </n-icon>
              </template>
            </NButton>
            <NButton size="small" quaternary circle :aria-label="$t('common.delete')" @click="deleteData(item)">
              <template #icon>
                <n-icon color="#888">
                  <trashIcon />
                </n-icon>
              </template>
            </NButton>
          </div>
        </template>
      </ItemCard>
    </NGridItem>
  </NGrid>
  <div v-if="scripts.pageCount.value > 1" class="m-t-16px flex justify-end">
    <n-pagination
      :page="scripts.page.value"
      :page-size="scripts.pageSize.value"
      :item-count="dataScriptTotal"
      @update:page="scripts.setPage"
    />
  </div>

  <DataHandleScriptModal
    v-model:show="showModal"
    v-model:form="configForm"
    :title="modalTitle"
    :rules="configFormRules"
    :editor-options="editorOptions"
    :script-type-options="scripTypeOpt"
    :set-form-ref="setConfigFormRef"
    :wide="getPlatform"
    @close="handleClose"
    @submit="handleSubmit"
    @quiz="doQuiz"
    @toggle-word-wrap="toggleWordWrap"
    @toggle-minimap="toggleMinimap"
    @change-font-size="changeFontSize"
  />
</template>
