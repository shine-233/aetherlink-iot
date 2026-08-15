export interface SummaryOption {
  id?: unknown
  name?: string
}

export interface LinkageActionInstructionFormState {
  action_type?: unknown
  action_target?: unknown
  action_param?: unknown
  actionParamData?: {
    label?: string
  }
  actionValue?: unknown
}

export interface LinkageActionGroupFormState {
  actionType?: unknown
  action_target?: unknown
  actionInstructList?: LinkageActionInstructionFormState[]
}

export interface LinkageActionSummaryCatalogs {
  deviceOptions: SummaryOption[]
  deviceConfigOptions: SummaryOption[]
  sceneOptions: SummaryOption[]
  alarmOptions: SummaryOption[]
}

export interface LinkageActionSummaryLabels {
  unset: string
  singleDevice: string
  singleClassDevice: string
  operateDevice: string
  activateScene: string
  triggerAlarm: string
  activate: string
  trigger: string
}

export interface LinkageActionSummaryLine {
  key: string
  tag: string
  text: string
}

export interface LinkageActionSummaryItem {
  key: string
  tag: string
  lines: LinkageActionSummaryLine[]
}

export function findOptionName(options: SummaryOption[], id: unknown, fallbackText: string) {
  return options.find((item) => String(item.id) === String(id))?.name || (id ? String(id) : fallbackText)
}

export function formatSummaryValue(value: unknown, fallbackText: string) {
  if (value === null || value === undefined || value === '') {
    return fallbackText
  }

  if (typeof value === 'object') {
    return JSON.stringify(value)
  }

  return String(value)
}

export function buildDeviceInstructionSummary(
  instructItem: LinkageActionInstructionFormState,
  instructIndex: number,
  catalogs: Pick<LinkageActionSummaryCatalogs, 'deviceOptions' | 'deviceConfigOptions'>,
  labels: Pick<LinkageActionSummaryLabels, 'singleDevice' | 'singleClassDevice' | 'unset'>
): LinkageActionSummaryLine {
  const isSingleDevice = String(instructItem.action_type) === '10'
  const targetName = findOptionName(
    isSingleDevice ? catalogs.deviceOptions : catalogs.deviceConfigOptions,
    instructItem.action_target,
    labels.unset
  )
  const paramName = instructItem.actionParamData?.label || instructItem.action_param || labels.unset
  const value = formatSummaryValue(instructItem.actionValue, labels.unset)

  return {
    key: `device-${instructIndex}`,
    tag: isSingleDevice ? labels.singleDevice : labels.singleClassDevice,
    text: `${targetName} / ${paramName} = ${value}`
  }
}

export function buildActionExecutionSummaryItems(
  actionGroups: LinkageActionGroupFormState[],
  catalogs: LinkageActionSummaryCatalogs,
  labels: LinkageActionSummaryLabels
): LinkageActionSummaryItem[] {
  return actionGroups.map((group, groupIndex) => {
    if (String(group.actionType) === '1') {
      return {
        key: `group-${groupIndex}`,
        tag: labels.operateDevice,
        lines: (group.actionInstructList || []).map((instructItem, instructIndex) =>
          buildDeviceInstructionSummary(instructItem, instructIndex, catalogs, labels)
        )
      }
    }

    if (String(group.actionType) === '20') {
      return {
        key: `group-${groupIndex}`,
        tag: labels.activateScene,
        lines: [
          {
            key: 'scene',
            tag: labels.activate,
            text: findOptionName(catalogs.sceneOptions, group.action_target, labels.unset)
          }
        ]
      }
    }

    return {
      key: `group-${groupIndex}`,
      tag: labels.triggerAlarm,
      lines: [
        {
          key: 'alarm',
          tag: labels.trigger,
          text: findOptionName(catalogs.alarmOptions, group.action_target, labels.unset)
        }
      ]
    }
  })
}
