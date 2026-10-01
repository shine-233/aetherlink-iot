<!--
  文件用途: 设备详情页里的“分布/表格 + 下发弹窗”公共模块（命令下发 / 属性下发 / 事件上报共用）。
  核心逻辑: 列表查询由 useDistributionTable 负责，弹窗编辑态由 useDistributionDialogState 负责，
  提交分流（submitApi / expectApi / directMethodApi）由 useDistributionSubmitFlow 负责；
  可视化页签拆为 DistributionCommandParams（命令参数模板）与 DistributionAttributePicker（属性勾选）。
  查询/提交/期望消息联动链路:
  1. 页面挂载、手动刷新、翻页后，统一通过 fetchDataApi 拉取列表数据。
  2. 打开弹窗后，根据 isCommand 决定加载命令标识及参数模板，或加载属性集供勾选编辑。
  3. 提交时先把可视化表单统一折叠为 textValue JSON，再根据 expected 分流到 submitApi 或 expectApi。
  4. 成功后统一重新查询列表并关闭弹窗，确保表格展示与最近一次操作结果保持同一观察面。
-->
<script setup lang="ts">
import { computed, getCurrentInstance, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
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
import { Refresh } from '@vicons/ionicons5'
import type { FlatResponseFailData, FlatResponseSuccessData } from '@aetherlink/axios'
import type { DirectMethodResult } from '@/service/api'
import { $t } from '@/locales'
import { quickCommandKey } from './distributionSubmitPayload'
import { createDeliveryModeView, createSubmitTrackingView } from './distributionTableState'
import type { CommandSubmitTracking } from './useDistributionSubmitFlow'
import { useDistributionSubmitFlow } from './useDistributionSubmitFlow'
import {
  DISTRIBUTION_PAGE_SIZE,
  distributionLogger,
  useDistributionDialogState,
  useDistributionTable
} from './useDistributionTable'
import DistributionCommandParams from './DistributionCommandParams.vue'
import DistributionAttributePicker from './DistributionAttributePicker.vue'

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

const deviceId = () => props.id
// 注意：不能命名为 isCommand，否则会在模板中遮蔽同名 prop。
const getIsCommand = () => props.isCommand

const { tableData, page_coune, the_page, commandList, loading, fetchDataFunction, updatePage, refresh, initialize } =
  useDistributionTable({
    deviceId,
    isCommand: getIsCommand,
    noRefresh: () => props.noRefresh,
    fetchDataApi: () => props.fetchDataApi
  })

const {
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
} = useDistributionDialogState({ deviceId, isCommand: getIsCommand, directMethodOnline: () => props.directMethodOnline })

const latestSubmitTracking = ref<CommandSubmitTracking | null>(null)
const handleSubmitTracking = async (tracking: CommandSubmitTracking) => {
  latestSubmitTracking.value = tracking
  await props.onSubmittedTracking?.(tracking)
}

const { onCommandChange, quickCommandLoadingId, submit, submitting } = useDistributionSubmitFlow({
  activeTab,
  attributeList,
  closeDialog,
  deviceId,
  directMethodApi: () => props.directMethodApi,
  expectApi: () => props.expectApi,
  fetchData: fetchDataFunction,
  formModel,
  formRef,
  hasAttributeSelection,
  isCommand: getIsCommand,
  logger: distributionLogger,
  onDirectMethodResult: props.onDirectMethodResult,
  onSubmitTracking: handleSubmitTracking,
  paramsData,
  submitApi: () => props.submitApi
})

defineExpose({ refresh })
onMounted(initialize)

const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
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
        :page-size="DISTRIBUTION_PAGE_SIZE"
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
              :options="commandOptions"
              filterable
              tag
              clearable
              :placeholder="$t('generate.command-identifier-placeholder')"
              @update:show="loadCommandOptions"
              @update:value="handleCommandInput"
            />
          </NFormItem>

          <!-- 页签切换只影响载荷来源，不改变 submit / expect 的最终分流。 -->
          <NTabs v-model:value="activeTab" type="line" animated>
            <NTabPane name="visual" :tab="visualTabLabel">
              <DistributionCommandParams
                v-if="isCommand"
                :command-value="formModel.commandValue"
                :params="paramsData"
              />
              <DistributionAttributePicker
                v-else
                :loading="attributeLoading"
                :rows="attributeList"
                :has-selection="hasAttributeSelection"
              />
            </NTabPane>
            <NTabPane name="command" :tab="customTabLabel">
              <NFormItem
                label=""
                :validation-status="jsonInvalid ? 'error' : undefined"
                :feedback="jsonInvalid ? $t('generate.inputRightJson') : ''"
              >
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

.button-group {
  margin-top: 16px;
  gap: 12px;
}
</style>
