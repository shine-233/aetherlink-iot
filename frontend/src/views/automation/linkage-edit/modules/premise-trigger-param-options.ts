/** 触发条件行（编辑器表单数据，字段宽松） */
type TriggerIfItemLike = Record<string, unknown>

/** 条件指标菜单项（后端返回，字段宽松） */
type TriggerParamMenuItem = {
  data_source_type?: unknown
  label?: unknown
  key?: unknown
  options?: unknown[] | null
  [key: string]: unknown
}

export type TriggerParamOptionItem = Record<string, unknown>

/** 条件指标菜单接口（后端返回 data 列表） */
export type MetricMenuFetcher = (
  payload: Record<string, unknown>
) => Promise<{ data?: TriggerParamMenuItem[] | null } | null | undefined>

export type TriggerParamOptionsLoadDeps = {
  deviceMetricsConditionMenu: MetricMenuFetcher
  configMetricsConditionMenu: MetricMenuFetcher
  statusOption: TriggerParamOptionItem | null
  syncSelectedEventParams: (ifItem: TriggerIfItemLike) => void
  onError?: (error: unknown) => void
}

export const formatTriggerParamOptions = (items: TriggerParamMenuItem[] = []): TriggerParamOptionItem[] => {
  return items.map(item => ({
    ...item,
    value: item.data_source_type,
    label: `${item.data_source_type}${item.label ? `(${item.label})` : ''}`,
    options: ((item.options as TriggerParamMenuItem[] | null) || []).map(subItem => ({
      ...subItem,
      value: `${item.data_source_type}/${subItem.key}`,
      label: `${subItem.key}${subItem.label ? `(${subItem.label})` : ''}`
    }))
  }))
}

export const buildTriggerParamOptions = (
  items: TriggerParamMenuItem[] = [],
  statusOption: TriggerParamOptionItem | null
): Array<TriggerParamOptionItem | null> => {
  const options: Array<TriggerParamOptionItem | null> = formatTriggerParamOptions(items)
  if (!options.some(option => option?.value === 'status')) {
    options.push(statusOption)
  }
  return options
}

export const canLoadTriggerParamOptions = (ifItem: TriggerIfItemLike) => {
  return ifItem.trigger_source && (ifItem.trigger_conditions_type === '10' || ifItem.trigger_conditions_type === '11')
}

export const hasTriggerParamOptions = (ifItem: TriggerIfItemLike) => {
  return Array.isArray(ifItem.triggerParamOptions) && ifItem.triggerParamOptions.length > 0
}

export const fetchTriggerParamOptions = async (
  ifItem: TriggerIfItemLike,
  {
    deviceMetricsConditionMenu,
    configMetricsConditionMenu
  }: Pick<TriggerParamOptionsLoadDeps, 'deviceMetricsConditionMenu' | 'configMetricsConditionMenu'>
) => {
  if (ifItem.trigger_conditions_type === '10') {
    return deviceMetricsConditionMenu({
      device_id: ifItem.trigger_source
    })
  }

  if (ifItem.trigger_conditions_type === '11') {
    return configMetricsConditionMenu({
      device_config_id: ifItem.trigger_source
    })
  }

  return null
}

export const setTriggerParamOptions = (
  ifItem: TriggerIfItemLike,
  items: TriggerParamMenuItem[] = [],
  statusOption: TriggerParamOptionItem | null
) => {
  ifItem.triggerParamOptions = buildTriggerParamOptions(items, statusOption)
}

export const clearTriggerParamOptions = (ifItem: TriggerIfItemLike) => {
  ifItem.triggerParamOptions = []
}

export const resolveTriggerParamOptionItems = async (
  ifItem: TriggerIfItemLike,
  deps: Pick<TriggerParamOptionsLoadDeps, 'deviceMetricsConditionMenu' | 'configMetricsConditionMenu'>
) => {
  const res = await fetchTriggerParamOptions(ifItem, deps)
  return res?.data || []
}

export const loadTriggerParamOptionsForIfItem = async (ifItem: TriggerIfItemLike, deps: TriggerParamOptionsLoadDeps) => {
  if (!canLoadTriggerParamOptions(ifItem)) {
    return
  }

  if (hasTriggerParamOptions(ifItem)) {
    deps.syncSelectedEventParams(ifItem)
    return
  }

  clearTriggerParamOptions(ifItem)
  try {
    const items = await resolveTriggerParamOptionItems(ifItem, deps)
    setTriggerParamOptions(ifItem, items, deps.statusOption)
  } catch (error) {
    deps.onError?.(error)
    clearTriggerParamOptions(ifItem)
  } finally {
    deps.syncSelectedEventParams(ifItem)
  }
}
