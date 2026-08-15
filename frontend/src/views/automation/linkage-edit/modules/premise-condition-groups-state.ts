import { onMounted, ref, watch, type Ref } from 'vue'
import type { FormInst } from 'naive-ui'
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

type CreatePremiseConditionGroupsStateOptions = {
  premiseForm: Ref<any>
  premiseFormRef: Ref<FormInst | null>
  props: {
    conditionData?: any[]
    device_id?: string
    device_config_id?: string
  }
  hasConfigId: boolean
  locale: Ref<any>
  deviceConfigDisabled: Ref<boolean>
  statusData: Ref<any> | { value: any }
  ensureDevicesLoaded: () => void | Promise<void>
  ensureDeviceConfigsLoaded: () => void | Promise<void>
  loadTriggerParamOptions: (ifItem: any) => void | Promise<void>
  normalizeIfItemForEcho: (ifItem: any) => void
  emitConditionChose: (data: any) => void
}

const cloneJudgeItem = (judgeItem: any) => JSON.parse(JSON.stringify(judgeItem))

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

  const addIfGroupsSubItem = async (ifGroupIndex: any) => {
    await premiseFormRef.value?.validate?.()
    premiseForm.value.ifGroups[ifGroupIndex].push(cloneJudgeItem(judgeItem.value))
  }

  const deleteIfGroupsSubItem = (ifGroupIndex: any, ifIndex: any) => {
    premiseForm.value.ifGroups[ifGroupIndex].splice(ifIndex, 1)
  }

  const deleteIfGroupsItem = (ifIndex: any) => {
    premiseForm.value.ifGroups.splice(ifIndex, 1)
  }

  const addIfGroupItem = (data: any) => {
    const groupObj: any[] = []
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

  const loadSourceCatalogsForConditions = (conditionData: any) => {
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
