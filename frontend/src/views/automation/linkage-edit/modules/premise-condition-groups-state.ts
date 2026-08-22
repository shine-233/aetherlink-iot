import { onMounted, ref, watch, type Ref } from 'vue'
import type { FormInst, CascaderOption } from 'naive-ui'
import {
  applyEchoedConditionData,
  applyInitialPremiseCondition,
  buildInitialPremiseCondition
} from './premise-trigger-lifecycle'
import {
  createInitialConditionFromProps as createInitialConditionFromPropsHelper,
  refreshStatusOptionsForLocale
} from './premise-edit-premise-state'
import { createScheduleConditionFields } from './premise-schedule-condition-state'

export type PremiseIfItem = {
  ifType?: string | number | null
  trigger_conditions_type?: string | number | null
  trigger_source?: string | number | null
  trigger_param_key?: string | number | null
  trigger_param_type?: string | null
  trigger_operator?: string | number | null
  trigger_value?: string | [string, string] | null
  minValue?: string | [string, string] | null
  maxValue?: string | [string, string] | null
  triggerParamOptions?: CascaderOption[]
  [key: string]: unknown
}
export type PremiseFormModel = { ifGroups: PremiseIfItem[][] }

type CreatePremiseConditionGroupsStateOptions = {
  premiseForm: Ref<PremiseFormModel>
  premiseFormRef: Ref<FormInst | null>
  props: {
    conditionData?: unknown[]
    device_id?: string
    device_config_id?: string
  }
  hasConfigId: boolean
  locale: Ref<string>
  deviceConfigDisabled: Ref<boolean>
  statusData: { value: unknown }
  ensureDevicesLoaded: () => void | Promise<void>
  ensureDeviceConfigsLoaded: () => void | Promise<void>
  loadTriggerParamOptions: (ifItem: PremiseIfItem) => void | Promise<void>
  normalizeIfItemForEcho: (ifItem: PremiseIfItem) => void
  emitConditionChose: (data: unknown) => void
}

const cloneJudgeItem = <T>(judgeItem: T): T => JSON.parse(JSON.stringify(judgeItem)) as T

export const createPremiseConditionGroupsState = ({
  premiseForm,
  premiseFormRef,
  props,
  hasConfigId,
  locale,
  deviceConfigDisabled,
  statusData,
  ensureDevicesLoaded,
  ensureDeviceConfigsLoaded,
  loadTriggerParamOptions,
  normalizeIfItemForEcho,
  emitConditionChose
}: CreatePremiseConditionGroupsStateOptions) => {
  const judgeItem = ref({
    ifType: null,
    trigger_conditions_type: null,
    trigger_source: null,
    trigger_param_type: null,
    trigger_param: null,
    trigger_param_key: null,
    trigger_operator: null,
    trigger_value: null,
    ...createScheduleConditionFields(),
    minValue: null,
    maxValue: null,
    deviceGroupId: null,
    triggerParamOptions: [],
    eventParamsRaw: null,
    eventParamOptions: [],
    eventParamConditions: []
  })

  const addIfGroupsSubItem = async (ifGroupIndex: number) => {
    await premiseFormRef.value?.validate?.()
    premiseForm.value.ifGroups[ifGroupIndex].push(cloneJudgeItem(judgeItem.value))
  }

  const deleteIfGroupsSubItem = (ifGroupIndex: number, ifIndex: number) => {
    premiseForm.value.ifGroups[ifGroupIndex].splice(ifIndex, 1)
  }

  const deleteIfGroupsItem = (ifIndex: number) => {
    premiseForm.value.ifGroups.splice(ifIndex, 1)
  }

  const addIfGroupItem = (data: PremiseIfItem | null) => {
    const groupObj: PremiseIfItem[] = []
    if (!data) {
      groupObj.push(cloneJudgeItem(judgeItem.value))
      premiseForm.value.ifGroups.push(groupObj)
      return
    }

    groupObj.push(data)
    premiseForm.value.ifGroups.push(groupObj)
  }
  const addConditionGroup = () => addIfGroupItem(null)

  const ifGroupsData = () => premiseForm.value.ifGroups
  const premiseFormRefReturn = () => premiseFormRef.value

  const loadSourceCatalogsForConditions = (conditionData: unknown) => {
    if (!conditionData || !Array.isArray(conditionData)) return

    conditionData.forEach((ifGroup) => {
      if (!Array.isArray(ifGroup)) return

      ifGroup.forEach((ifItem) => {
        if (ifItem?.trigger_conditions_type === '10') {
          void ensureDevicesLoaded()
        }
        if (ifItem?.trigger_conditions_type === '11') {
          void ensureDeviceConfigsLoaded()
        }
      })
    })
  }

  watch(
    () => props.conditionData,
    (newValue) => {
      const ifGroups = applyEchoedConditionData(newValue, normalizeIfItemForEcho, loadTriggerParamOptions)
      if (ifGroups) {
        premiseForm.value.ifGroups = ifGroups
        loadSourceCatalogsForConditions(ifGroups)
      }
    },
    { immediate: true }
  )

  const loadInitialOptions = () => {
    if (props.device_id) {
      void ensureDevicesLoaded()
      return
    }

    if (props.device_config_id) {
      void ensureDeviceConfigsLoaded()
    }
  }

  const createInitialConditionFromProps = () => {
    return buildInitialPremiseCondition(
      judgeItem.value,
      {
        deviceId: props.device_id,
        deviceConfigId: props.device_config_id
      },
      createInitialConditionFromPropsHelper
    )
  }

  const applyInitialConditionWhenCreating = () => {
    applyInitialPremiseCondition({
      hasConfigId,
      buildInitialCondition: createInitialConditionFromProps,
      addIfGroupItem,
      emitConditionChose,
      setDeviceConfigDisabled: (value) => {
        deviceConfigDisabled.value = value
      }
    })
  }

  onMounted(() => {
    loadInitialOptions()
    applyInitialConditionWhenCreating()
  })

  watch(locale, () => {
    refreshStatusOptionsForLocale(premiseForm.value.ifGroups, statusData.value)
  })

  return {
    judgeItem,
    addIfGroupsSubItem,
    deleteIfGroupsSubItem,
    deleteIfGroupsItem,
    addConditionGroup,
    addIfGroupItem,
    ifGroupsData,
    premiseFormRefReturn
  }
}
