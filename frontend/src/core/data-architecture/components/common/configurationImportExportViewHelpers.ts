export type Translate = (key: string) => string

export interface AvailableDataSourceSummary {
  sourceId: string
  sourceIndex: number
  hasData: boolean
  dataItemCount?: number
}

export interface ImportPreviewSlot {
  slotId: string
  slotIndex: number
  isEmpty: boolean
}

export interface TargetSlotOption {
  label: string
  value: string
  disabled: boolean
  occupied: boolean
}

export const isJsonFileName = (fileName: string): boolean => fileName.endsWith('.json')

export const readFileAsText = (file: File, fileReadErrorMessage: string): Promise<string> => {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(new Error(fileReadErrorMessage))
    reader.readAsText(file)
  })
}

export const readImportJsonFile = async (file: File, fileReadErrorMessage: string): Promise<any> => {
  const fileContent = await readFileAsText(file, fileReadErrorMessage)
  return JSON.parse(fileContent)
}

export const isSingleDataSourceImportFile = (importData: any): boolean => {
  return importData.exportType === 'single-datasource' || importData.type === 'singleDataSource'
}

export const createExportTimestamp = (date = new Date()): string => {
  return date.toISOString().slice(0, 16).replace(/[:-]/g, '')
}

export const buildConfigurationExportFileName = (componentId: string, timestamp = createExportTimestamp()): string => {
  return `config_${componentId.substring(0, 8)}_${timestamp}.json`
}

export const buildSingleDataSourceExportFileName = (sourceId: string, timestamp = createExportTimestamp()): string => {
  return `datasource_${sourceId}_${timestamp}.json`
}

export const downloadJsonFile = (data: unknown, fileName: string): void => {
  const blob = new Blob([JSON.stringify(data, null, 2)], {
    type: 'application/json'
  })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  link.click()
  URL.revokeObjectURL(url)
}

export const formatImportDateTime = (timestamp: number): string => {
  return new Date(timestamp).toLocaleString()
}

export const buildTargetSlotOptionsFromPreviewSlots = (
  slots: ImportPreviewSlot[],
  t: Translate
): TargetSlotOption[] => {
  return slots.map(slot => ({
    label: `${t('configuration.export.dataSource')} ${slot.slotIndex + 1} (${slot.slotId})`,
    value: slot.slotId,
    disabled: false,
    occupied: !slot.isEmpty
  }))
}

export const buildTargetSlotOptionsFromAvailableSources = (
  sources: AvailableDataSourceSummary[],
  t: Translate
): TargetSlotOption[] => {
  return sources.map(source => ({
    label: `${t('configuration.export.dataSource')} ${source.sourceIndex + 1} (${source.sourceId})`,
    value: source.sourceId,
    disabled: false,
    occupied: source.hasData
  }))
}

export const selectDefaultTargetSlot = (options: TargetSlotOption[]): string => {
  return options.find(slot => !slot.occupied)?.value || options[0]?.value || ''
}
