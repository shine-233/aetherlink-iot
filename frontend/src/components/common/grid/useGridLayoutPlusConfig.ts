import { computed, type ComputedRef } from 'vue'

import type { GridLayoutPlusConfig, GridLayoutPlusItem } from './gridLayoutPlusTypes'
import { DEFAULT_GRID_LAYOUT_PLUS_CONFIG, EXTENDED_GRID_LAYOUT_CONFIG, GridSizePresets } from './gridLayoutPlusTypes'
import { validateExtendedGridConfig, validateLargeGridPerformance } from './utils/validation'

/**
 * 文件用途：根据网格尺寸预设解析出最终配置，并做配置 / 性能校验。
 * 核心逻辑：gridSize 决定基础配置，再与用户自定义 config 合并；校验结果供外部展示与告警。
 * 关键注意事项：校验失败仅打日志，不阻断渲染，保持与拆分前一致的行为。
 */

export type GridSizePresetName = 'mini' | 'standard' | 'large' | 'mega' | 'extended' | 'custom'

export interface GridLayoutPlusConfigProps {
  gridSize?: GridSizePresetName
  customColumns?: number
  config?: Partial<GridLayoutPlusConfig>
  layout?: GridLayoutPlusItem[]
}

export interface GridValidationResult {
  isValid: boolean
  colNum: number
  performance?: unknown
}

export interface UseGridLayoutPlusConfigResult {
  config: ComputedRef<GridLayoutPlusConfig>
  gridValidation: ComputedRef<GridValidationResult>
}

function resolveBaseConfig(props: GridLayoutPlusConfigProps): GridLayoutPlusConfig {
  switch (props.gridSize) {
    case 'mini':
      return { ...DEFAULT_GRID_LAYOUT_PLUS_CONFIG, ...GridSizePresets.MINI }
    case 'standard':
      return { ...DEFAULT_GRID_LAYOUT_PLUS_CONFIG, ...GridSizePresets.STANDARD }
    case 'large':
      return { ...DEFAULT_GRID_LAYOUT_PLUS_CONFIG, ...GridSizePresets.LARGE }
    case 'mega':
      return { ...EXTENDED_GRID_LAYOUT_CONFIG, ...GridSizePresets.MEGA }
    case 'extended':
      return { ...EXTENDED_GRID_LAYOUT_CONFIG }
    case 'custom':
      return { ...EXTENDED_GRID_LAYOUT_CONFIG, ...GridSizePresets.CUSTOM(props.customColumns || 50) }
    default:
      return { ...DEFAULT_GRID_LAYOUT_PLUS_CONFIG }
  }
}

export function useGridLayoutPlusConfig(props: GridLayoutPlusConfigProps): UseGridLayoutPlusConfigResult {
  const config = computed<GridLayoutPlusConfig>(() => ({
    ...resolveBaseConfig(props),
    ...props.config
  }))

  const gridValidation = computed<GridValidationResult>(() => {
    const colNum = config.value.colNum

    const configValidation = validateExtendedGridConfig(colNum)
    if (!configValidation.success) {
      console.error('Grid configuration validation failed:', configValidation.message)
    }

    const performanceCheck = validateLargeGridPerformance(props.layout ?? [], colNum)
    if (performanceCheck.success && (performanceCheck.data?.warning || performanceCheck.data?.recommendation)) {
      console.error('Grid performance warning:', performanceCheck.data.warning)
      console.info('Grid performance recommendation:', performanceCheck.data.recommendation)
    }

    return {
      isValid: configValidation.success,
      colNum,
      performance: performanceCheck.data
    }
  })

  return { config, gridValidation }
}
