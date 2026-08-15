<!--
  文件用途: 设备配置页中的“数据处理”模块，负责脚本列表展示、启停、增删改、调试。
  核心逻辑: 围绕 `queryData -> dataScriptList -> 弹窗表单 configForm -> 保存/调试接口` 这一状态流组织页面行为。
  保存链路: 打开弹窗时先重置默认表单，编辑时再回填记录；保存前统一校验并补齐 `device_config_id`，随后按是否存在 `id` 分流到新增或编辑接口，成功后关闭弹窗并刷新列表。
  关键注意事项: 脚本内容会直接影响平台与设备之间的数据语义，注释和默认模板需要帮助维护者快速识别处理类型、调试输入和返回结果的含义。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, getCurrentInstance, nextTick, onMounted, ref, watch } from 'vue'
import { type FormInst, NButton, useDialog } from 'naive-ui'
import { PencilOutline as editIcon, TrashOutline as trashIcon } from '@vicons/ionicons5'
import ItemCard from '@/components/dev-card-item/index.vue'
import {
  dataScriptAdd,
  dataScriptDel,
  dataScriptEdit,
  dataScriptQuiz,
  getDataScriptList,
  setDeviceScriptEnable
} from '@/service/api/device'
import { $t } from '@/locales'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const dialog = useDialog()
const ScriptEditor = defineAsyncComponent(() => import('@/components/LuaScriptEditor.vue'))

interface Props {
  configInfo?: object | any
}

type ScriptQuizResponse = {
  code?: number | string
  message?: string
  data?: unknown
}

const props = withDefaults(defineProps<Props>(), {
  configInfo: null
})
const configFormRef = ref<HTMLElement & FormInst>()

const modalTitle = ref($t('generate.add'))
const configForm: any = ref({})
const scripTypeOpt = ref([
  {
    label: $t('generate.all'),
    value: ''
  },
  {
    label: $t('custom.devicePage.reportPreprocessing'),
    value: 'A'
  },
  {
    label: $t('custom.devicePage.transmissionPreprocessing'),
    value: 'B'
  },
  {
    label: $t('custom.devicePage.attributeReporting'),
    value: 'C'
  },
  {
    label: $t('custom.devicePage.attributeDistribution'),
    value: 'D'
  },
  {
    label: $t('custom.devicePage.commandDeliveryPreprocessing'),
    value: 'E'
  },
  {
    label: $t('custom.devicePage.eventReportPreprocessing'),
    value: 'F'
  }
])

function defaultConfigForm() {
  // 默认表单既是“新增脚本”的初始状态，也是“编辑脚本”前的清空基线，
  // 这样可以避免上一次打开弹窗留下的脚本内容、调试输入或结果污染当前操作。
  return {
    id: null,
    content: `function encodeInp(msg,topic)
 -- 说明：该函数为编码函数，将输入的消息编码为平台可识别的消息格式或者设备可识别的消息格式，请根据实际需求编写编码逻辑
 -- 入参：输入的msg，可以是任意数据类型的字符串。
 -- 出参：返回值为编码后的消息,需要是json字符串形式
 -- 注意：string与jsonObj互转需导入json库：local json = require("json")
 -- 例，string转jsonObj：local jsonData = json.decode(msgString)
 -- 例，jsonObj转string：local jsonStr = json.encode(jsonTable)
 local json = require("json")
 local jsonData = json.decode(msg)
 -- 例 if jsonData.temp then
 -- 例 jsonData.temp = jsonData.temp * 10
 -- 例 end
 local newJsonString = json.encode(jsonData)
 return newJsonString
 end`,
    description: null,
    device_config_id: null,
    enable_flag: 'Y',
    analog_input: null,
    last_analog_input: null,
    name: null,
    remark: null,
    script_type: null,
    resolt_analog_input: ''
  }
}

// Script editor options. The compatibility component renders CodeMirror.
const editorOptions = ref({
  automaticLayout: true,
  theme: 'vs',
  language: 'lua',
  fontSize: 14,
  lineHeight: 20,
  fontFamily: 'Consolas, "Courier New", monospace',
  wordWrap: 'on',
  lineNumbers: 'on',
  glyphMargin: true,
  folding: true,
  lineDecorationsWidth: 10,
  lineNumbersMinChars: 3,
  minimap: {
    enabled: true,
    side: 'right',
    size: 'proportional',
    showSlider: 'mouseover'
  },
  scrollBeyondLastLine: false,
  readOnly: false,
  cursorStyle: 'line',
  cursorBlinking: 'blink',
  renderWhitespace: 'selection',
  renderControlCharacters: false,
  fontLigatures: true,
  suggestOnTriggerCharacters: true,
  acceptSuggestionOnEnter: 'on',
  tabCompletion: 'on',
  wordBasedSuggestions: true,
  parameterHints: {
    enabled: true
  },
  quickSuggestions: {
    other: true,
    comments: false,
    strings: false
  },
  bracketPairColorization: {
    enabled: true
  },
  guides: {
    bracketPairs: true,
    indentation: true
  },
  formatOnPaste: true,
  formatOnType: true
})

// 当前本地编辑器未引入 Lua formatter；不展示会静默成功的伪格式化操作。
const toggleMinimap = () => {
  editorOptions.value.minimap.enabled = !editorOptions.value.minimap.enabled
}

const toggleWordWrap = () => {
  editorOptions.value.wordWrap = editorOptions.value.wordWrap === 'on' ? 'off' : 'on'
}

const changeFontSize = (delta: number) => {
  const newSize = editorOptions.value.fontSize + delta
  if (newSize >= 10 && newSize <= 24) {
    editorOptions.value.fontSize = newSize
  }
}

const configFormRules = ref({
  name: {
    required: true,
    message: $t('generate.enter-title'),
    trigger: 'blur'
  },
  content: {
    required: true,
    message: $t('generate.parse-script'),
    trigger: 'blur'
  },
  enable_flag: {
    required: true,
    message: $t('common.select'),
    trigger: 'change'
  },
  script_type: {
    required: true,
    message: $t('generate.select-processing-type'),
    trigger: 'change'
  }
})
// 弹窗显隐与 configForm 一起构成“列表态 <-> 编辑态”的切换边界。
// 列表页只维护筛选条件；进入弹窗后再承载脚本内容、调试入参和保存提交数据。
const showModal = ref(false)

const openModal = (type: any, item: any) => {
  modalTitle.value = type
  // 先用默认值初始化表单
  configForm.value = defaultConfigForm()

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

const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})
const bodyStyle = ref({
  width: getPlatform.value ? '90%' : '800px'
})
const queryData: any = ref({
  device_config_id: '',
  script_type: '',
  page: 1,
  page_size: 10
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
const dataScriptList = ref<Array<DataScriptItem>>([])
const dataScriptTotal = ref(0)
const queryDataScriptList = async () => {
  // 列表查询依赖父级传入的设备配置 id；每次请求前重新同步，避免 props 异步更新后沿用旧值。
  queryData.value.device_config_id = props.configInfo.id
  const { data, error } = await getDataScriptList(queryData.value)
  if (error || !data) {
    // 查询失败时清空列表，确保界面状态与“当前没有可展示结果”保持一致，而不是残留旧数据。
    dataScriptList.value = []
    dataScriptTotal.value = 0
    return
  }

  // 接口返回结构容错：优先使用后端 total，缺失时退回当前 list 长度，保证分页区至少可正常渲染。
  const payload = data as { list?: DataScriptItem[]; total?: number }
  dataScriptList.value = Array.isArray(payload.list) ? payload.list : []
  dataScriptTotal.value = typeof payload.total === 'number' ? payload.total : dataScriptList.value.length
}
const searchDataScript = () => {
  // 主动搜索时重置到第一页，避免切换筛选条件后仍停留在旧页码导致“看起来没数据”。
  queryData.value.page = 1
  queryDataScriptList()
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
const doQuiz = async () => {
  // 调试链路与保存链路共享同一份 configForm：
  // 先校验必填项，再把模拟输入和脚本内容交给后端执行，将返回文本统一落到结果框中。
  await configFormRef?.value?.validate()

  try {
    const response = await dataScriptQuiz(configForm.value)
    // 兼容“请求层失败”和“业务层返回错误对象”两类响应结构。
    if (response.error && response.data === null) {
      const errorInfo = response.error
      const errorMessage = errorInfo.message || t('page.dataForward.requestFailed')
      configForm.value.resolt_analog_input = `${t('page.dataForward.debugFailed')}\n${t('page.dataForward.errorType')}: ${errorInfo.name || 'Unknown'}\n${t('page.dataForward.errorCode')}: ${errorInfo.code || 'N/A'}\n${t('page.dataForward.errorMessage')}: ${errorMessage}`
      return
    }

    // 有些接口封装会把真正的 code/data/message 再包进 response.data，需要先解包再判定结果。
    let actualResponse = response as unknown as ScriptQuizResponse
    if (response.data && typeof response.data === 'object' && 'code' in response.data) {
      actualResponse = response.data as ScriptQuizResponse
    }
    // 结果区只接收字符串，因此这里把各种 data 形态统一转换成可直接展示的文本。
    // 使用宽松比较是因为后端 code 可能返回 number 也可能返回 string。
    if (actualResponse.code == 200 || actualResponse.code === '200') {
      if (typeof actualResponse.data === 'string') {
        configForm.value.resolt_analog_input =
          actualResponse.data === 'null' ? t('page.dataForward.debugSuccessWithNull') : actualResponse.data
      } else if (actualResponse.data === null || actualResponse.data === undefined) {
        configForm.value.resolt_analog_input = t('page.dataForward.debugSuccessWithNull')
      } else {
        configForm.value.resolt_analog_input = JSON.stringify(actualResponse.data, null, 2)
      }
    } else {
      const errorMessage = actualResponse.message || t('page.dataForward.noErrorMessage')
      configForm.value.resolt_analog_input = `${t('page.dataForward.debugFailed')}\ncode: ${actualResponse.code}\nmessage: ${errorMessage}`
    }
  } catch (error: unknown) {
    console.error('调试请求异常:', error)
    const errorMessage = error instanceof Error ? error.message : String(error)
    configForm.value.resolt_analog_input =
      t('page.dataForward.debugRequestFailed') + ': ' + (errorMessage || t('page.dataForward.unknownError'))
  }
}
// `queryData` 变化时自动回拉列表；初次挂载时补一次首屏查询，
// 这样无论是父级传参更新还是本地筛选切换，都能复用同一条取数路径。
watch(queryData.value, () => queryDataScriptList(), { deep: true })
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
        :isStatus="false"
        :hideFooterLeft="true"
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
            <NButton size="small" quaternary circle @click="openModal($t('common.edit'), item)">
              <template #icon>
                <n-icon color="#888">
                  <editIcon />
                </n-icon>
              </template>
            </NButton>
            <NButton size="small" quaternary circle @click="deleteData(item)">
              <template #icon>
                <n-icon color="#888">
                  <trashIcon />
                </n-icon>
              </template>
            </NButton>
          </div>
        </template>
      </ItemCard>
      <!-- <NCard hoverable style="height: 180px">
        <div class="item-name item-center flex">
          <div class="flex-1">
            {{ item.name }}
          </div>
          <NSwitch
            v-model:value="item.enable_flag"
            checked-value="Y"
            unchecked-value="N"
            @update-value="handleChange(item)"
          />
        </div>
        <div class="h-80px flex-1">
          <div class="item-desc description">{{ item.description }}</div>
          <div class="item-desc">{{ findScriptType(item.script_type) }}</div>
        </div>
        <NFlex justify="end">
          <NButton tertiary circle type="info" @click="openModal($t('common.edit'), item)">
            <template #icon>
              <n-icon>
                <editIcon />
              </n-icon>
            </template>
          </NButton>
          <NButton circle tertiary type="error" @click="deleteData(item)">
            <template #icon>
              <n-icon>
                <trashIcon />
              </n-icon>
            </template>
          </NButton>
        </NFlex>
      </NCard> -->
    </NGridItem>
  </NGrid>

  <n-modal
    v-model:show="showModal"
    preset="dialog"
    :width="800"
    :title="modalTitle + $t('common.dataProces')"
    :show-icon="false"
    :style="bodyStyle"
    :closable="false"
  >
    <NForm
      ref="configFormRef"
      class="flex-wrap"
      :class="getPlatform ? 'flex-col' : 'flex'"
      :model="configForm"
      :rules="configFormRules"
      label-placement="left"
      label-width="auto"
    >
      <NFormItem :class="getPlatform ? 'w-100%' : 'w-50%'" :label="$t('page.manage.menu.form.title')" path="name">
        <NInput v-model:value="configForm.name" :placeholder="$t('generate.enter-title')" />
      </NFormItem>
      <NFormItem :class="getPlatform ? 'w-100%' : 'w-50%'" :label="$t('generate.processing-type')" path="script_type">
        <NSelect
          v-model:value="configForm.script_type"
          :options="scripTypeOpt"
          :placeholder="$t('generate.select-processing-type')"
        ></NSelect>
      </NFormItem>
      <NFormItem class="w-100%" :label="$t('device_template.table_header.description')" path="description">
        <NInput
          v-model:value="configForm.description"
          type="textarea"
          :rows="2"
          :placeholder="$t('generate.enter-description')"
        />
      </NFormItem>
      <NFormItem class="w-100%" :label="$t('generate.parse-script')" :rules="configFormRules" path="content">
        <div class="editor-container">
          <!-- 编辑器工具栏 -->
          <div class="editor-toolbar">
            <div class="toolbar-left">
              <NButton size="small" tertiary @click="toggleWordWrap">
                <template #icon>
                  <n-icon>
                    <svg viewBox="0 0 24 24">
                      <path
                        fill="currentColor"
                        d="M4 19h6v-2H4v2zM20 5H4v2h16V5zm-3 6H4v2h13.25c1.1 0 2 .9 2 2s-.9 2-2 2H15v-2l-3 3l3 3v-2h2.25c2.3 0 4.25-2.05 4.25-4.5S19.55 11 17.25 11z"
                      />
                    </svg>
                  </n-icon>
                </template>
                自动换行
              </NButton>
              <NButton size="small" tertiary @click="toggleMinimap">
                <template #icon>
                  <n-icon>
                    <svg viewBox="0 0 24 24">
                      <path
                        fill="currentColor"
                        d="M3 3h18v18H3V3zm16 16V5H5v14h14zM7 7h2v2H7V7zm0 4h2v2H7v-2zm0 4h2v2H7v-2zm4-8h6v2h-6V7zm0 4h6v2h-6v-2zm0 4h6v2h-6v-2z"
                      />
                    </svg>
                  </n-icon>
                </template>
                小地图
              </NButton>
            </div>
            <div class="toolbar-right">
              <NButton size="small" tertiary @click="changeFontSize(-1)">
                <template #icon>
                  <n-icon>
                    <svg viewBox="0 0 24 24"><path fill="currentColor" d="M19 13H5v-2h14v2z" /></svg>
                  </n-icon>
                </template>
              </NButton>
              <span class="font-size-display">{{ editorOptions.fontSize }}px</span>
              <NButton size="small" tertiary @click="changeFontSize(1)">
                <template #icon>
                  <n-icon>
                    <svg viewBox="0 0 24 24"><path fill="currentColor" d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" /></svg>
                  </n-icon>
                </template>
              </NButton>
            </div>
          </div>
          <!-- Script editor -->
          <div class="editor-wrapper">
            <ScriptEditor
              v-model:value="configForm.content"
              :options="editorOptions"
              height="300"
              language="lua"
              class="custom-script-editor"
            />
          </div>
        </div>
      </NFormItem>
      <NFormItem
        v-if="0"
        class="w-100%"
        :label="$t('page.manage.setting.dataClearSetting.form.enabled')"
        path="enable_flag"
      >
        <NSwitch v-model:value="configForm.enable_flag" checked-value="Y" unchecked-value="N" />
      </NFormItem>
      <NFormItem class="w-100%" :label="$t('generate.simulate-input')" path="last_analog_input">
        <NInput v-model:value="configForm.last_analog_input" type="textarea" :rows="2" />
      </NFormItem>
      <NFormItem class="w-100%" :label="$t('generate.debug-run-result')" path="resolt_analog_input">
        <NInput v-model:value="configForm.resolt_analog_input" :rows="5" :disabled="true" type="textarea" />
      </NFormItem>
      <NFormItem>
        <NButton type="primary" @click="doQuiz">{{ $t('common.debug') }}</NButton>
      </NFormItem>
    </NForm>
    <NFlex justify="end">
      <NButton @click="handleClose">{{ $t('generate.cancel') }}</NButton>
      <NButton type="primary" @click="handleSubmit">{{ $t('common.save') }}</NButton>
    </NFlex>
  </n-modal>
</template>

<style scoped lang="scss">
.alarm-box {
  display: flex;
  flex-flow: row;
  justify-content: flex-start;
  align-items: center;
  flex-wrap: wrap;
  padding: 10px 40px;

  .alarm-item {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    padding: 18px;
    flex: 0 0 23%;
    margin-right: calc(30% / 3);
    margin-bottom: 30px;

    .item-name {
      display: flex;
      flex-flow: row;
      align-items: center;
      justify-content: space-between;
    }

    .item-desc {
      margin: 15px 0;
    }

    .item-operate {
      display: flex;
      flex-flow: row;
      justify-content: space-between;
      align-items: center;
    }
  }
}

.description {
  height: 40px;
  word-break: break-all;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
}

/* 编辑器容器样式 */
.editor-container {
  width: 100%;
  border: 1px solid #e0e0e6;
  border-radius: 6px;
  overflow: hidden;
  background: #fff;
}

.editor-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #f8f9fa;
  border-bottom: 1px solid #e0e0e6;
  min-height: 40px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.font-size-display {
  font-size: 12px;
  color: #666;
  min-width: 35px;
  text-align: center;
}

.editor-wrapper {
  position: relative;
  background: #fff;
  width: 100%;
}

.custom-script-editor {
  border: none !important;
  width: 100% !important;
}

/* 编辑器工具栏按钮样式优化 */
.editor-toolbar .n-button {
  height: 28px;
  padding: 0 8px;
  font-size: 12px;
}

.editor-toolbar .n-button .n-icon {
  font-size: 14px;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .editor-toolbar {
    flex-direction: column;
    gap: 8px;
    padding: 12px;
  }

  .toolbar-left,
  .toolbar-right {
    width: 100%;
    justify-content: center;
  }
}
</style>
