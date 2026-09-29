/**
 * 文件用途: “分布/表格 + 下发弹窗”模块的列表查询与弹窗编辑态 composable。
 * 核心逻辑:
 * 1. useDistributionTable: 分页列表查询（fetchDataApi）、快捷命令按钮列表、刷新/翻页。
 * 2. useDistributionDialogState: 下发弹窗表单模型、命令标识选项与参数模板、属性集勾选行、校验与提交禁用规则。
 * 关键注意事项:
 * - fetchDataApi 查询参数 { page, page_size: 4, device_id } 是父层接口契约；noRefresh 时不带分页参数。
 * - 属性集请求必须传 { device_id }（历史上误传裸 id 会请求 /attribute/datas/undefined）。
 */
import { computed, reactive, ref } from 'vue'
import type { FormInst, FormRules } from 'naive-ui'
import { useLoading } from '@aetherlink/hooks'
import type { FlatResponseFailData, FlatResponseSuccessData } from '@aetherlink/axios'
import { commandDataById, deviceCustomCommandsIdList, getAttributeDataSet } from '@/service/api'
import { $t } from '@/locales'
import { isJSON } from '@/utils/common/tool'
import { createLogger } from '@/utils/logger'
import { normalizeAttributeItem } from './distributionAttributePayload'
import { commandParamsForIdentifier } from './distributionCommandPayload'
import { normalizeDistributionListView, shouldDisableDistributionSubmit } from './distributionTableState'

export const distributionLogger = createLogger('Table')

export const DISTRIBUTION_PAGE_SIZE = 4

type ApiCall = (params: any) => Promise<FlatResponseSuccessData | FlatResponseFailData>

export type DistributionAttributeRow = ReturnType<typeof normalizeAttributeItem> & {
  checked: boolean
  inputValue: string
  [key: string]: any
}

export function useDistributionTable(options: {
  deviceId: () => string
  noRefresh: () => boolean | undefined
  isCommand: () => boolean | undefined
  fetchDataApi: () => ApiCall
}) {
  const tableData = ref<any[] | undefined>()
  const page_coune = ref(0)
  const the_page = ref(1)
  const commandList = ref<any[]>()
  const { loading, startLoading, endLoading } = useLoading()

  // 页面挂载、手动刷新、翻页、提交成功后都会回到这里。
  const fetchDataFunction = async () => {
    startLoading()
    try {
      const paged = !options.noRefresh()
      const { data, error } = await options.fetchDataApi()({
        page: paged ? the_page.value : undefined,
        page_size: paged ? DISTRIBUTION_PAGE_SIZE : undefined,
        device_id: options.deviceId()
      })
      if (!error) {
        const listView = normalizeDistributionListView(data)
        tableData.value = listView.rows
        page_coune.value = listView.pageCount
      }
    } catch (error) {
      distributionLogger.warn('[DistributionAndTable] Failed to fetch distribution table data.', {
        deviceId: options.deviceId(),
        error: error instanceof Error ? error.message : error
      })
    } finally {
      endLoading()
    }
  }

  const updatePage = (page: number) => {
    the_page.value = page
    fetchDataFunction()
  }

  const refresh = () => {
    the_page.value = 1
    fetchDataFunction()
  }

  const loadCommandList = async () => {
    const { data } = await deviceCustomCommandsIdList(options.deviceId())
    commandList.value = data
  }

  const initialize = () => {
    if (options.isCommand()) loadCommandList()
    fetchDataFunction()
  }

  return { tableData, page_coune, the_page, commandList, loading, fetchDataFunction, updatePage, refresh, initialize }
}

export function useDistributionDialogState(options: {
  deviceId: () => string
  isCommand: () => boolean | undefined
  directMethodOnline: () => boolean | undefined
}) {
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
  const commandOptions = ref<any[]>()
  const paramsData = ref<any[]>([])
  const attributeList = ref<DistributionAttributeRow[]>([])
  const attributeLoading = ref(false)
  const isTextArea = ref(true)
  // 管理弹窗里的“可视化配置 / 文本直输”页签。
  const activeTab = ref('visual')

  const rules = computed<FormRules>(() => {
    const result: FormRules = {}
    if (options.isCommand() && isTextArea.value) {
      result.commandValue = {
        required: true,
        message: $t('page.manage.validation.commandIdentifierRequired'),
        trigger: ['input', 'blur']
      }
    }
    return result
  })

  const loadAttributeList = async () => {
    attributeLoading.value = true
    try {
      const { data, error } = await getAttributeDataSet({ device_id: options.deviceId() })
      attributeList.value =
        !error && Array.isArray(data)
          ? data.map((item: any) => ({ ...normalizeAttributeItem(item), checked: false, inputValue: '' }))
          : []
    } catch (error) {
      distributionLogger.error('loadAttributeList failed', { error: error instanceof Error ? error.message : error })
      attributeList.value = []
    } finally {
      attributeLoading.value = false
    }
  }

  const openDialog = async () => {
    // 属性下发依赖属性集元数据；命令下发则在命令标识变化时装载参数模板。
    showDialog.value = true
    if (!options.isCommand()) await loadAttributeList()
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

  const loadCommandOptions = async (show: boolean) => {
    if (!show) return
    const res = await commandDataById(options.deviceId())
    commandOptions.value = res && Array.isArray(res.data) ? res.data : []
  }

  // 既支持选择已有命令，也允许手输自定义标识；命中已有命令时把参数模板灌入 visual 页签。
  const handleCommandInput = (value: string) => {
    formModel.commandValue = value
    paramsData.value = commandParamsForIdentifier(commandOptions.value, value)
  }

  const jsonInvalid = computed(() => Boolean(formModel.textValue) && !isJSON(formModel.textValue))
  const hasAttributeSelection = computed(() => attributeList.value.some((item) => item.checked))

  const isSubmitDisabled = computed(() => {
    const payloadDisabled = shouldDisableDistributionSubmit({
      isCommand: options.isCommand(),
      commandValue: formModel.commandValue,
      textValue: formModel.textValue,
      isValidJson: isJSON
    })
    if (payloadDisabled) return true
    if (!formModel.waitForResponse) return false
    const timeoutSeconds = Number(formModel.timeoutSeconds)
    return (
      options.directMethodOnline() === false ||
      !Number.isFinite(timeoutSeconds) ||
      timeoutSeconds < 1 ||
      timeoutSeconds > 30
    )
  })

  return {
    showDialog,
    formRef,
    formModel,
    commandOptions,
    paramsData,
    attributeList,
    attributeLoading,
    activeTab,
    rules,
    jsonInvalid,
    hasAttributeSelection,
    isSubmitDisabled,
    openDialog,
    closeDialog,
    loadCommandOptions,
    handleCommandInput
  }
}
