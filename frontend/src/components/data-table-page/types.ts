import type { TreeSelectOption } from 'naive-ui'

export type theLabel = string | (() => string) | undefined

// Search configuration shared by the data-table page and its consumers.
export type SearchConfig =
  | {
      key: string
      label: string
      type: 'input' | 'date' | 'date-range'
      initValue?: any
    }
  | {
      key: string
      label: string
      type: 'select'
      renderLabel?: any
      renderTag?: any
      initValue?: any
      extendParams?: object
      options: { label: theLabel; value: any }[]
      labelField?: string
      valueField?: string
      loadOptions?: () => Promise<{ label: theLabel; value: any }[]>
    }
  | {
      key: string
      label: string
      type: 'tree-select'
      initValue?: any
      options: TreeSelectOption[]
      multiple: boolean
      loadOptions?: () => Promise<TreeSelectOption[]>
    }
