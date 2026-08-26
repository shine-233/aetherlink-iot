<!--
文件用途: 物模型自定义命令配置步骤。
核心逻辑: 管理自定义命令脚本、参数、表格和新增编辑弹窗。
关键注意事项: 自定义命令会影响设备控制能力，脚本内容和参数结构必须可追踪。
重构建议: 与自定义控制共享脚本编辑和参数管理逻辑。
-->
<script setup lang="tsx">
import { computed, defineAsyncComponent, getCurrentInstance, nextTick, onMounted, reactive, ref, shallowRef } from 'vue'
import { NButton, NDataTable, NForm, NFormItem, NInput, NModal, NPagination, NPopconfirm, NTag } from 'naive-ui'
import { $t } from '@/locales'
import { useThemeStore } from '@/store/modules/theme'
import {
  deviceCustomCommandsAdd,
  deviceCustomCommandsDel,
  deviceCustomCommandsList,
  deviceCustomCommandsPut
} from '@/service/api/system-data'
import {
  buildCustomCommandInstructStarter,
  formatCustomCommandInstruct,
  validateCustomCommandInstruct
} from './custom-command-json'

const props = defineProps<{
  id: string
}>()

// 主题系统集成
const themeStore = useThemeStore()

const configFormRules = ref({
  data_identifier: {
    required: true,
    message: $t('device_template.table_header.commandIdentifier'),
    trigger: 'blur'
  },
  buttom_name: {
    required: true,
    message: $t('generate.btnname'),
    trigger: 'blur'
  }
})

const commandjson: any = reactive({
  configForm: false,
  listData: [],
  total: 0,
  queryjson: {
    page: 1,
    page_size: 4
  },
  formjson: {
    buttom_name: '',
    data_identifier: '',
    description: '',
    instruct: '{}',
    enable_status: 'disable'
  }
})
const getCommandList = (page: number = 1) => {
  const queryjson = { ...commandjson.queryjson, page, device_template_id: props.id }
  deviceCustomCommandsList(queryjson).then(({ data }) => {
    commandjson.listData = data.list || []
    commandjson.total = data.total
  })
}
const cmRef = ref()
const instructFeedback = ref('')
const instructValidationStatus = ref<'error' | 'success' | undefined>()
const CodeMirror = defineAsyncComponent(() => import('vue-codemirror6'))
const editorLanguage = shallowRef<unknown>(null)
let editorLanguageLoadPromise: Promise<void> | null = null

const ensureScriptEditorLoaded = () => {
  if (editorLanguage.value) return Promise.resolve()
  if (!editorLanguageLoadPromise) {
    editorLanguageLoadPromise = import('@codemirror/lang-javascript')
      .then(({ javascript }) => {
        editorLanguage.value = javascript()
      })
      .catch(() => {
        editorLanguageLoadPromise = null
      })
  }
  return editorLanguageLoadPromise
}

const setupEditor = () => {
  void ensureScriptEditorLoaded()
  nextTick(() => undefined)
  /*
    // CodeMirror 6 会自动处理，不需要手动聚焦
  })
  */
}
const openCommandDialog = () => {
  const willOpen = !commandjson.configForm
  commandjson.formjson = {
    buttom_name: '',
    data_identifier: '',
    description: '',
    instruct: '{}',
    enable_status: 'disable'
  }
  instructFeedback.value = ''
  instructValidationStatus.value = undefined
  commandjson.configForm = willOpen
  if (willOpen) {
    void ensureScriptEditorLoaded()
  }
}
const handleDeleteTable = async (id) => {
  const { error } = await deviceCustomCommandsDel(id)

  if (!error) {
    getCommandList()
  }
}
const handleEditTable = (row: any) => {
  openCommandDialog()
  commandjson.formjson = row
  validateCommandInstruct()
}
const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})
const columns: any = [
  {
    key: 'buttom_name',
    minWidth: '100px',
    title: $t('generate.btnname')
  },

  {
    key: 'data_identifier',
    minWidth: '100px',
    title: $t('device_template.table_header.commandIdentifier')
  },
  {
    key: 'instruct',
    minWidth: '100px',
    title: $t('generate.commandConetnt')
  },
  {
    key: 'description',
    minWidth: '100px',
    title: $t('device_template.table_header.commandDescription')
  },
  {
    key: 'enable_status',
    minWidth: '100px',
    title: $t('generate.status'),
    render: (row) => {
      if (row?.enable_status === 'enable') {
        return <NTag type="success">{$t('page.manage.common.status.enable')}</NTag>
      }
      return <NTag type="warning">{$t('page.manage.common.status.disable')}</NTag>
    }
  },
  {
    key: 'actions',
    minWidth: '100px',
    title: $t('page.product.list.operate'),
    align: 'center',
    render: (row) => {
      return (
        <div class="flex gap-20px flex-justify-center">
          <NButton size={'small'} type="primary" onClick={() => handleEditTable(row)}>
            {$t('common.edit')}
          </NButton>

          <NPopconfirm onPositiveClick={() => handleDeleteTable(row.id)}>
            {{
              default: () => $t('common.confirmDelete'),
              trigger: () => (
                <NButton type="error" size={'small'}>
                  {$t('common.delete')}
                </NButton>
              )
            }}
          </NPopconfirm>
        </div>
      )
    }
  }
]
const configFormRef = ref()
const validateCommandInstruct = () => {
  const result = validateCustomCommandInstruct(commandjson.formjson?.instruct || '')
  instructValidationStatus.value = result.valid ? 'success' : 'error'
  instructFeedback.value = result.valid ? '' : result.error || $t('generate.inputRightJson')
  return result.valid
}
const formatCommandInstruct = () => {
  const result = formatCustomCommandInstruct(commandjson.formjson?.instruct || '')
  instructValidationStatus.value = result.valid ? 'success' : 'error'
  instructFeedback.value = result.valid ? '' : result.error || $t('generate.inputRightJson')
  if (!result.valid || !result.formatted) {
    window.$message?.error(instructFeedback.value || $t('generate.inputRightJson'))
    return
  }
  commandjson.formjson.instruct = result.formatted
  window.$message?.success($t('generate.commandJsonFormatSuccess'))
}
const insertCommandInstructStarter = () => {
  commandjson.formjson.instruct = buildCustomCommandInstructStarter()
  validateCommandInstruct()
  window.$message?.success($t('generate.commandJsonStarterLoaded'))
}
const onCommandSubmit = async (e) => {
  e.preventDefault()
  configFormRef.value?.validate(async (errors) => {
    if (!errors && validateCommandInstruct()) {
      const params = { ...commandjson.formjson, device_template_id: props.id }
      const { error } = commandjson.formjson?.id
        ? await deviceCustomCommandsPut(params)
        : await deviceCustomCommandsAdd(params)
      if (!error) {
        openCommandDialog()
        getCommandList()
      }
    }
  })
}

onMounted(() => {
  getCommandList()
})
</script>

<template>
  <div class="p-t-20px">
    <div class="m-b-20px flex flex-justify-end">
      <NButton class="justify-end" type="primary" @click="openCommandDialog">
        {{ $t('generate.addCustomCommand') }}
      </NButton>
    </div>
    <NDataTable :columns="columns" :data="commandjson.listData" class="flex-1-hidden">
      <template #empty>
        <n-empty :description="$t('common.noData')" />
      </template>
    </NDataTable>

    <div class="w-full flex justify-end">
      <NPagination
        :item-count="commandjson.total"
        :page-size="commandjson.queryjson.page_size"
        @update:page="getCommandList"
      />
    </div>
    <NModal
      v-model:show="commandjson.configForm"
      :title="$t('generate.customCommand')"
      :class="getPlatform ? 'w-90%' : 'custom-command-modal'"
      @after-enter="setupEditor"
    >
      <n-card>
        <NForm
          ref="configFormRef"
          :model="commandjson.formjson"
          label-placement="left"
          class="flex-wrap"
          :rules="configFormRules"
          label-width="120px"
        >
          <NFormItem :label="$t('generate.btnname')" path="buttom_name">
            <NInput v-model:value="commandjson.formjson.buttom_name" :placeholder="$t('generate.or-enter-here')" />
          </NFormItem>
          <NFormItem :label="$t('device_template.table_header.commandIdentifier')" path="data_identifier">
            <NInput v-model:value="commandjson.formjson.data_identifier" :placeholder="$t('generate.or-enter-here')" />
          </NFormItem>
          <NFormItem
            :label="$t('generate.commandConetnt')"
            path="instruct"
            :validation-status="instructValidationStatus"
            :feedback="instructFeedback"
          >
            <div class="command-json-editor">
              <div class="command-json-editor__toolbar">
                <NButton size="small" secondary @click="insertCommandInstructStarter">
                  {{ $t('generate.commandJsonInsertStarter') }}
                </NButton>
                <NButton size="small" secondary @click="formatCommandInstruct">
                  {{ $t('common.format') }}
                </NButton>
              </div>
              <CodeMirror
                v-if="editorLanguage"
                ref="cmRef"
                v-model="commandjson.formjson.instruct"
                basic
                :dark="themeStore.darkMode"
                :lang="editorLanguage as any"
                :style="{
                  width: '100%',
                  height: '200px',
                  border: '1px solid var(--n-border-color)',
                  borderRadius: 'var(--n-border-radius)'
                }"
                :placeholder="$t('generate.enter-json-format')"
                @blur="validateCommandInstruct"
              />
              <div v-else class="script-editor-loading">
                {{ $t('common.loading') }}
              </div>
              <div class="command-json-editor__hint">
                {{ $t('generate.commandJsonObjectHint') }}
              </div>
            </div>
          </NFormItem>
          <NFormItem :label="$t('device_template.table_header.commandDescription')" path="description">
            <NInput
              v-model:value="commandjson.formjson.description"
              type="textarea"
              :autosize="{ minRows: 3, maxRows: 6 }"
              :placeholder="$t('device_template.table_header.PleaseEnterADescription')"
            />
          </NFormItem>
          <NFormItem :label="$t('generate.status')" path="enable_status">
            <n-switch
              v-model:value="commandjson.formjson.enable_status"
              checked-value="enable"
              unchecked-value="disable"
            />
          </NFormItem>

          <NFlex justify="end">
            <NButton @click="openCommandDialog">{{ $t('generate.cancel') }}</NButton>
            <NButton type="primary" @click="onCommandSubmit">{{ $t('custom.groupPage.confirm') }}</NButton>
          </NFlex>
        </NForm>
      </n-card>
    </NModal>
  </div>
</template>

<style scoped lang="scss">
.custom-command-modal {
  width: 800px;
  min-width: 600px;
}

.command-json-editor {
  width: 100%;
}

.command-json-editor__toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-bottom: 8px;
}

.command-json-editor__hint {
  margin-top: 6px;
  color: var(--text-color-3);
  font-size: 12px;
}

.script-editor-loading {
  height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--n-border-color);
  border-radius: var(--n-border-radius);
  color: var(--n-text-color-3);
  background: var(--n-color);
}
</style>
