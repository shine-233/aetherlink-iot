<!--
  文件用途: 设备详情页里的“分布/表格 + 下发弹窗”公共模块。
  核心逻辑: 承接父层传入的查询、下发、期望消息接口，统一维护列表展示、命令/属性下发和提交后刷新。
  查询/提交/期望消息联动链路:
  1. 页面挂载、手动刷新、翻页后，统一通过 fetchDataApi 拉取列表数据。
  2. 打开弹窗后，根据 isCommand 决定加载命令标识及参数模板，或加载属性集供勾选编辑。
  3. 提交时先把可视化表单统一折叠为 textValue JSON，再根据 expected 分流到 submitApi 或 expectApi。
  4. 成功后统一重新查询列表并关闭弹窗，确保表格展示与最近一次操作结果保持同一观察面。
  静态审查建议:
  - props、列表项、参数项大量使用 any，后续适合收敛成显式类型，降低接口口径漂移风险。
  - fetchDataApi / submitApi / expectApi 目前依赖多种 data 字段兜底兼容，建议沉淀统一返回契约。
  - visual 页签与文本页签共同读写 formModel.textValue，后续若继续扩展可考虑拆分“编辑态”和“提交态”。
-->
<script setup lang="ts">
import { computed, getCurrentInstance, onMounted, reactive, ref } from 'vue'
import {
  type FormInst,
  type FormRules,
  NAlert,
  NButton,
  NCard,
  NCheckbox,
  NDataTable,
  NFlex,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NIcon,
  NInput,
  NInputNumber,
  NModal,
  NPagination,
  NPopover,
  NSelect,
  NSwitch,
  NTabs,
  NTabPane
} from 'naive-ui'
import { useLoading } from '@aetherlink/hooks'
import { Refresh } from '@vicons/ionicons5'
import type { FlatResponseFailData, FlatResponseSuccessData } from '@aetherlink/axios'
import { commandDataById, deviceCustomCommandsIdList, getAttributeDataSet } from '@/service/api'
import type { DirectMethodResult } from '@/service/api'
import { $t } from '@/locales'
import { isJSON } from '@/utils/common/tool'
import { createLogger } from '@/utils/logger'
import { getDescriptionText, normalizeAttributeItem } from './distributionAttributePayload'
import { commandParamsForIdentifier } from './distributionCommandPayload'
import { quickCommandKey } from './distributionSubmitPayload'
import {
  createDeliveryModeView,
  createSubmitTrackingView,
  normalizeDistributionListView,
  shouldDisableDistributionSubmit
} from './distributionTableState'
import type { CommandSubmitTracking } from './useDistributionSubmitFlow'
import { useDistributionSubmitFlow } from './useDistributionSubmitFlow'
const logger = createLogger('Table')
// props 契约:
// - id 和 fetchDataApi 是最小必需输入，决定当前设备上下文和列表查询能力。
// - isCommand 决定弹窗走“命令下发”还是“属性下发”分支。
// - submitApi / expectApi 都是可选能力，由 formModel.expected 决定最终调用哪条提交链路。
// - tableColumns / buttonName / noRefresh / expect 只影响展示与交互开关，不改变核心载荷结构。
const props = defineProps<{
  id: string
  noRefresh?: boolean
  isCommand?: boolean
  buttonName?: string
  tableColumns: any[] | undefined
  expect?: boolean
  submitApi?: (params: any) => Promise<FlatResponseSuccessData | FlatResponseFailData>
  directMethodApi?: (params: any) => Promise<FlatResponseSuccessData | FlatResponseFailData>
  directMethodOnline?: boolean
  expectApi?: (params: any) => Promise<FlatResponseSuccessData | FlatResponseFailData>
  fetchDataApi: (params: any) => Promise<FlatResponseSuccessData | FlatResponseFailData>
  onDirectMethodResult?: (result: DirectMethodResult) => void | Promise<void>
  onSubmittedTracking?: (tracking: CommandSubmitTracking) => void | Promise<void>
}>()
const tableData = ref<any[] | undefined>()
const page_coune = ref(0)
const the_page = ref(1)
const showDialog = ref(false)
const formRef = ref<FormInst | null>(null)
const formModel = reactive({
  commandValue: '',
  textValue: '',
  expected: false,
  time: null as number | null,
  waitForResponse: false,
  timeoutSeconds: 10 as number | null
})
const options = ref()
const { loading, startLoading, endLoading } = useLoading()
const paramsSelect = ref<any>([
  { label: 'true', value: true },
  { label: 'false', value: false }
])
const paramsData = ref<any>([])
const attributeList = ref<any[]>([])
const attributeLoading = ref(false)
const isTextArea = ref<any>(true)
const latestSubmitTracking = ref<CommandSubmitTracking | null>(null)

// 管理弹窗里的“可视化配置 / 文本直输”页签。
const activeTab = ref('visual')
const rules = computed<FormRules>(() => {
  const r: FormRules = {}
  if (props.isCommand && isTextArea.value) {
    r.commandValue = {
      required: true,
      message: $t('page.manage.validation.commandIdentifierRequired'),
      trigger: ['input', 'blur']
    }
  }
  return r
})

// 查询链路入口:
// - 页面挂载、手动刷新、翻页、提交成功后都会回到这里。
// - noRefresh 为 true 时不带分页参数，表示由父层接口自行控制返回规模。
// - 当前兼容 data.value / data.list / 数组本体，以及 count / total 两类分页字段。
const fetchDataFunction = async () => {
  startLoading()

  try {
    const { data, error } = await props.fetchDataApi({
      page: !props.noRefresh ? the_page.value : undefined,
      page_size: !props.noRefresh ? 4 : undefined,
      device_id: props.id
    })
    if (!error) {
      const listView = normalizeDistributionListView(data)
      tableData.value = listView.rows
      page_coune.value = listView.pageCount
    }
  } catch (error) {
    logger.warn('[DistributionAndTable] Failed to fetch distribution table data.', {
      deviceId: props.id,
      error: error instanceof Error ? error.message : error
    })
  } finally {
    endLoading()
  }
}

// 属性下发弹窗打开前装载当前设备的属性集元数据，映射成勾选行模型。
const loadAttributeList = async () => {
  attributeLoading.value = true
  try {
    const { data, error } = await getAttributeDataSet({ device_id: props.id })
    if (!error && Array.isArray(data)) {
      attributeList.value = data.map((item: any) => ({
        ...normalizeAttributeItem(item),
        checked: false,
        inputValue: ''
      }))
    } else {
      attributeList.value = []
    }
  } catch (error) {
    logger.error('loadAttributeList failed', {
      error: error instanceof Error ? error.message : error
    })
    attributeList.value = []
  } finally {
    attributeLoading.value = false
  }
}

const openDialog = async () => {
  // 属性下发依赖属性集元数据；命令下发则主要依赖命令标识变化时的参数模板装载。
  showDialog.value = true
  if (!props.isCommand) {
    await loadAttributeList()
  }
}

const closeDialog = () => {
  showDialog.value = false
  formModel.textValue = ''
  paramsData.value = []
  formModel.commandValue = ''
  isTextArea.value = true
  formModel.expected = false
  formModel.time = null
  formModel.waitForResponse = false
  formModel.timeoutSeconds = 10
  activeTab.value = 'visual'
  attributeList.value = []
  formRef.value?.restoreValidation?.()
}

const handleSubmitTracking = async (tracking: CommandSubmitTracking) => {
  latestSubmitTracking.value = tracking
  await props.onSubmittedTracking?.(tracking)
}

const updatePage = (page: number) => {
  the_page.value = page
  fetchDataFunction()
}
const refresh = () => {
  the_page.value = 1
  fetchDataFunction()
}

defineExpose({ refresh })
const getOptions = async (show) => {
  if (show) {
    const res = await commandDataById(props.id)

    if (res && Array.isArray(res.data)) {
      options.value = res.data
    } else {
      options.value = []
    }
  }
}

// 处理命令标识输入：既支持选择已有命令，也允许手输自定义标识。
// 若命中已有命令且携带 params，则自动把参数模板灌入 visual 页签。
const handleCommandInput = (value: string) => {
  formModel.commandValue = value
  paramsData.value = commandParamsForIdentifier(options.value, value)
}

const commandList = ref()

const getListData = async () => {
  const { data } = await deviceCustomCommandsIdList(props.id)
  commandList.value = data
}
onMounted(() => {
  props.isCommand && getListData()
  fetchDataFunction()
})
const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})
const validationJson = computed(() => {
  // 命令与属性下发共用同一套 JSON 静态校验反馈。
  if (formModel.textValue && !isJSON(formModel.textValue)) {
    return 'error'
  }
  return undefined
})
const inputFeedback = computed(() => {
  if (formModel.textValue && !isJSON(formModel.textValue)) {
    return $t('generate.inputRightJson')
  }
  return ''
})

const hasAttributeSelection = computed(() => attributeList.value.some((item) => item.checked))
const { onCommandChange, quickCommandLoadingId, submit, submitting } = useDistributionSubmitFlow({
  activeTab,
  attributeList,
  closeDialog,
  deviceId: () => props.id,
  directMethodApi: () => props.directMethodApi,
  expectApi: () => props.expectApi,
  fetchData: fetchDataFunction,
  formModel,
  formRef,
  hasAttributeSelection,
  isCommand: () => props.isCommand,
  logger,
  onDirectMethodResult: props.onDirectMethodResult,
  onSubmitTracking: handleSubmitTracking,
  paramsData,
  submitApi: () => props.submitApi
})
const selectAllAttributes = computed({
  get: () => attributeList.value.length > 0 && attributeList.value.every((item: any) => item.checked),
  set: (value) => {
    attributeList.value.forEach((item) => {
      item.checked = value
    })
  }
})
const isAttributeIndeterminate = computed(() => {
  const checkedCount = attributeList.value.filter((item) => item.checked).length
  return checkedCount > 0 && checkedCount < attributeList.value.length
})

const isSubmitDisabled = computed(() => {
  const payloadDisabled = shouldDisableDistributionSubmit({
    isCommand: props.isCommand,
    commandValue: formModel.commandValue,
    textValue: formModel.textValue,
    isValidJson: isJSON
  })
  if (payloadDisabled) return true
  if (!formModel.waitForResponse) return false
  const timeoutSeconds = Number(formModel.timeoutSeconds)
  return props.directMethodOnline === false || !Number.isFinite(timeoutSeconds) || timeoutSeconds < 1 || timeoutSeconds > 30
})

const visualTabLabel = computed(() =>
  props.isCommand ? $t('generate.visual-config') : $t('generate.attribute-config')
)
const customTabLabel = computed(() => (props.isCommand ? $t('generate.command-line') : $t('generate.custom-attribute')))
const deliveryModeView = computed(() => createDeliveryModeView(formModel.expected, formModel.waitForResponse, $t))
const latestSubmitTrackingView = computed(() => createSubmitTrackingView(latestSubmitTracking.value, $t))
const directMethodModeDisabled = computed(() => props.directMethodOnline === false)
const submitButtonLabel = computed(() =>
  formModel.waitForResponse && !formModel.expected ? $t('generate.directMethodSendAndWait') : $t('common.send')
)
</script>

<template>
  <div class="">
    <div class="m-b-20px flex items-center">
      <NButton v-if="buttonName" type="primary" @click="openDialog">{{ buttonName }}</NButton>
      <div class="flex flex-1 flex-justify-end">
        <NButton v-if="!noRefresh" :bordered="false" class="justify-end" @click="refresh">
          <NIcon size="18">
            <Refresh />
          </NIcon>
          {{ $t('generate.refresh') }}
        </NButton>
      </div>
    </div>

    <NGrid v-if="isCommand" x-gap="20" y-gap="20" cols="1 s:2 m:3 l:4" responsive="screen">
      <NGridItem v-for="item in commandList" :key="item.id">
        <NButton
          size="large"
          :loading="quickCommandLoadingId === quickCommandKey(item)"
          :disabled="item.enable_status === 'disable' || Boolean(quickCommandLoadingId)"
          class="title w-160px p-24px cursor-pointer ellipsis-text text-16px font-600"
          @click="onCommandChange(item)"
        >
          {{ item.buttom_name }}
        </NButton>
      </NGridItem>
    </NGrid>
    <NDataTable class="mb-4 mt-4" :loading="loading" :columns="tableColumns" :data="tableData" />
    <div class="flex flex-justify-end">
      <NPagination
        v-if="!noRefresh"
        :page-count="page_coune"
        :page="the_page"
        :page-size="4"
        @update:page="updatePage"
      />
    </div>
    <NModal v-if="submitApi" v-model:show="showDialog" :class="getPlatform ? 'w-90%' : 'w-450px'">
      <n-card :title="isCommand ? $t('generate.issueCommand') : $t('generate.issue-attribute')">
        <NForm ref="formRef" :model="formModel" :rules="rules" :label-placement="formModel.expected ? 'left' : 'top'">
          <div v-if="expect" class="flex">
            <NFormItem>
              <template #label>
                <div class="flex-ai-c flex">
                  {{ $t('generate.expectedMessage') }}
                  <n-popover trigger="hover">
                    <template #trigger>
                      <SvgIcon icon="mdi:help-circle-outline" class="text-20px" />
                    </template>
                    <span>{{ $t('generate.expectedMessageTip') }}</span>
                  </n-popover>
                </div>
              </template>

              <n-switch v-model:value="formModel.expected" />
            </NFormItem>
            <NFormItem v-if="formModel.expected" :label="$t('generate.expirationTime')" class="ml-20px">
              <div class="flex-ai-c flex">
                <n-input-number v-model:value="formModel.time" :show-button="false" class="w-80px" />
                <div class="fs-0">{{ $t('generate.hour') }}</div>
              </div>
            </NFormItem>
          </div>
          <div v-if="isCommand && !formModel.expected && directMethodApi" class="direct-method-options">
            <NFormItem :label="$t('generate.waitForDeviceResponse')">
              <NSwitch v-model:value="formModel.waitForResponse" :disabled="directMethodModeDisabled" />
            </NFormItem>
            <NFormItem
              v-if="formModel.waitForResponse"
              :label="$t('generate.directMethodTimeoutSeconds')"
              class="direct-method-timeout"
            >
              <NInputNumber
                v-model:value="formModel.timeoutSeconds"
                :min="1"
                :max="30"
                :precision="0"
                :show-button="true"
              />
              <span>{{ $t('generate.second') }}</span>
            </NFormItem>
          </div>
          <NAlert
            v-if="isCommand && formModel.waitForResponse && directMethodModeDisabled"
            type="error"
            :show-icon="false"
            class="delivery-mode-hint"
          >
            {{ $t('generate.directMethodOfflineHint') }}
          </NAlert>
          <NAlert v-if="isCommand" type="info" :show-icon="false" class="delivery-mode-hint">
            <strong>{{ deliveryModeView.title }}</strong>
            <div>{{ deliveryModeView.hint }}</div>
          </NAlert>
          <NAlert
            v-if="isCommand && latestSubmitTrackingView.visible"
            :type="latestSubmitTrackingView.type"
            :show-icon="false"
            class="delivery-mode-hint"
          >
            <strong>{{ $t('custom.device_details.messageId') }}</strong>
            <div>{{ latestSubmitTrackingView.text }}</div>
          </NAlert>
          <NFormItem
            v-if="isCommand"
            path="commandValue"
            :label="$t('generate.command-identifier')"
            required
            class="command-selector"
          >
            <NSelect
              v-model:value="formModel.commandValue"
              label-field="data_name"
              value-field="data_identifier"
              :options="options"
              filterable
              tag
              clearable
              :placeholder="$t('generate.command-identifier-placeholder')"
              @update:show="getOptions"
              @update:value="handleCommandInput"
            />
          </NFormItem>

          <!-- 页签切换只影响载荷来源，不改变 submit / expect 的最终分流。 -->
          <NTabs v-model:value="activeTab" type="line" animated>
            <NTabPane name="visual" :tab="visualTabLabel">
              <template v-if="isCommand">
                <div v-if="formModel.commandValue !== ''">
                  <div v-for="item in paramsData" :key="item.id" class="form_box">
                    <div class="form_table">
                      <NFormItem :label="item.data_name" label-placement="left" label-width="80px" label-align="left">
                        <NInput v-if="item.param_type === 'string'" v-model:value="item[item.data_identifier]" />
                        <n-input-number
                          v-else-if="item.param_type === 'Number'"
                          v-model:value="item[item.data_identifier]"
                        />
                        <n-select
                          v-else-if="item.param_type === 'Boolean'"
                          v-model:value="item[item.data_identifier]"
                          :options="paramsSelect"
                        />
                        <n-select
                          v-else-if="item.param_type === 'Enum'"
                          v-model:value="item[item.data_identifier]"
                          :options="
                            item.enum_config?.map((v) => {
                              return {
                                ...v,
                                label: v.desc
                              }
                            }) || []
                          "
                          :placeholder="$t('generate.please-select')"
                        />
                        <div class="description">
                          {{ $t('generate.description-label') }}：{{
                            getDescriptionText(item) || $t('generate.description-empty')
                          }}
                        </div>
                      </NFormItem>
                    </div>
                  </div>
                  <div v-if="paramsData.length === 0" class="empty-params">
                    <p>{{ $t('generate.no-params-available') }}</p>
                  </div>
                </div>
                <div v-else class="empty-params">
                  <p>{{ $t('generate.select-command-first') }}</p>
                </div>
              </template>
              <template v-else>
                <div v-if="attributeLoading" class="empty-params">
                  <p>{{ $t('generate.loading') }}</p>
                </div>
                <div v-else-if="attributeList.length">
                  <div class="attribute-toolbar">
                    <NCheckbox v-model:checked="selectAllAttributes" :indeterminate="isAttributeIndeterminate">
                      {{ $t('generate.select-all') }}
                    </NCheckbox>
                  </div>
                  <div v-for="item in attributeList" :key="item.key || item.id" class="attribute-row">
                    <div class="attribute-info">
                      <NCheckbox v-model:checked="item.checked">
                        <div class="attribute-label">
                          <div v-if="item.data_name" class="attribute-name">
                            {{ item.data_name }}
                          </div>
                          <div class="attribute-key">{{ item.key }}</div>
                        </div>
                      </NCheckbox>
                    </div>
                    <div class="attribute-input">
                      <NInput
                        v-model:value="item.inputValue"
                        :placeholder="$t('generate.attribute-value-placeholder')"
                        :disabled="!item.checked"
                      />
                    </div>
                  </div>
                  <div v-if="!hasAttributeSelection" class="attribute-helper">
                    {{ $t('generate.attribute-helper-text') }}
                  </div>
                </div>
                <div v-else class="empty-params">
                  <p>{{ $t('generate.no-attributes-available') }}</p>
                </div>
              </template>
            </NTabPane>
            <NTabPane name="command" :tab="customTabLabel">
              <NFormItem label="" :validation-status="validationJson" :feedback="inputFeedback">
                <NInput
                  v-model:value="formModel.textValue"
                  type="textarea"
                  :placeholder="isCommand ? $t('generate.or-enter-here') : $t('generate.custom-attribute-placeholder')"
                />
              </NFormItem>
            </NTabPane>
          </NTabs>
          <NFlex justify="end" class="button-group">
            <NButton @click="closeDialog">{{ $t('generate.cancel') }}</NButton>
            <NButton type="primary" :loading="submitting" :disabled="isSubmitDisabled || submitting" @click="submit">
              {{ submitButtonLabel }}
            </NButton>
          </NFlex>
        </NForm>
      </n-card>
    </NModal>
  </div>
</template>

<style lang="scss" scoped>
.form_box {
  width: 100%;
}

.title {
  font-weight: 900;
  font-size: 16px;
  margin-bottom: 10px;
}

.delivery-mode-hint {
  margin-bottom: 12px;
  line-height: 1.45;
}

.direct-method-options {
  display: flex;
  align-items: flex-start;
  gap: 20px;
}

.direct-method-timeout :deep(.n-form-item-blank) {
  display: flex;
  align-items: center;
  gap: 8px;
}

.form_table {
  display: flex;
  gap: 12px;
  margin-bottom: 8px;

  .n-form-item {
    flex: 1;
    margin-right: 0;

    :deep(.n-form-item-blank) {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
    }

    .description {
      margin-top: 4px;
      font-size: 11px;
      color: #6b7280;
      line-height: 1.3;
    }

    // 输入框样式优化
    :deep(.n-input),
    :deep(.n-input-number),
    :deep(.n-select) {
      .n-input__input-el,
      .n-input-number-input,
      .n-base-selection {
        height: 32px;
        border-radius: 4px;
        font-size: 13px;
      }
    }

    // 文本域样式
    :deep(.n-input--textarea) {
      .n-input__textarea-el {
        min-height: 60px;
        border-radius: 4px;
        font-size: 13px;
        line-height: 1.4;
      }
    }
  }

  .n-input-number {
    width: 100%;
  }
}

.selectBtn {
  margin-left: 20px;
}

.empty-params {
  text-align: center;
  padding: 20px 16px;
  color: #999;

  p {
    margin: 0;
    font-size: 13px;
  }
}

.attribute-toolbar {
  margin-bottom: 12px;
}

.attribute-row {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid #f2f4f7;

  &:first-of-type {
    border-top: 1px solid #f2f4f7;
  }

  &:last-of-type {
    margin-bottom: 12px;
  }
}

.attribute-info {
  flex: 1;

  .attribute-label {
    display: flex;
    flex-direction: column;
    line-height: 1.3;
  }

  .attribute-name {
    font-weight: 500;
    color: #1f2937;
  }

  .attribute-key {
    font-size: 14px;
    color: #6b7280;
  }
}

.attribute-input {
  flex: 1.2;
}

.button-group {
  margin-top: 16px;
  gap: 12px;
}
</style>
