type MetricMenuFetcher = (payload: Record<string, any>) => Promise<{ data?: any[] } | null | undefined>

type TriggerParamOptionsLoadDeps = {
  deviceMetricsConditionMenu: MetricMenuFetcher
  configMetricsConditionMenu: MetricMenuFetcher
  statusOption: any
  syncSelectedEventParams: (ifItem: any) => void
  onError?: (error: unknown) => void
}

export const formatTriggerParamOptions = (items: any[] = []) => {
  return items.map((item: any) => ({
    ...item,
    value: item.data_source_type,
    label: `${item.data_source_type}${item.label ? `(${item.label})` : ''}`,
    options: (item.options || []).map((subItem: any) => ({
      ...subItem,
      value: `${item.data_source_type}/${subItem.key}`,
      label: `${subItem.key}${subItem.label ? `(${subItem.label})` : ''}`
    }))
  }))
}

export const buildTriggerParamOptions = (items: any[] = [], statusOption: any) => {
  const options = formatTriggerParamOptions(items)
  if (!options.some((option: any) => option.value === 'status')) {
    options.push(statusOption)
  }
  return options
}

export const canLoadTriggerParamOptions = (ifItem: any) => {
  return ifItem.trigger_source && (ifItem.trigger_conditions_type === '10' || ifItem.trigger_conditions_type === '11')
}

export const hasTriggerParamOptions = (ifItem: any) => {
  return Array.isArray(ifItem.triggerParamOptions) && ifItem.triggerParamOptions.length > 0
}

export const fetchTriggerParamOptions = async (
  ifItem: any,
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

export const setTriggerParamOptions = (ifItem: Record<string, any>, items: any[] = [], statusOption: any) => {
  ifItem.triggerParamOptions = buildTriggerParamOptions(items, statusOption)
}

export const clearTriggerParamOptions = (ifItem: Record<string, any>) => {
  ifItem.triggerParamOptions = []
}

export const resolveTriggerParamOptionItems = async (
  ifItem: any,
  deps: Pick<TriggerParamOptionsLoadDeps, 'deviceMetricsConditionMenu' | 'configMetricsConditionMenu'>
) => {
  const res = await fetchTriggerParamOptions(ifItem, deps)
  return res?.data || []
}

export const loadTriggerParamOptionsForIfItem = async (ifItem: any, deps: TriggerParamOptionsLoadDeps) => {
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
