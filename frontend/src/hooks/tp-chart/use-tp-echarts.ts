/*
 * Compatibility entrypoint for existing tp-chart imports.
 * The shared lifecycle logic lives in hooks/chart/use-echarts.
 */
import { createEChartsHook, type ChartHooks, type EChartsHookOptions, type ECOption } from '@/hooks/chart/use-echarts'

export type { ChartHooks, EChartsHookOptions, ECOption } from '@/hooks/chart/use-echarts'

/**
 * Use the AetherLink IoT chart-compatible ECharts wrapper.
 *
 * @param optionsFactory echarts options factory function
 * @param hooks
 */
export function useTpECharts<T extends ECOption>(
  optionsFactory: () => T,
  hooks: ChartHooks = {},
  hookOptions: EChartsHookOptions = {}
) {
  return createEChartsHook(optionsFactory, hooks, {
    ...hookOptions,
    initOptions: { renderer: 'canvas', ...hookOptions.initOptions },
    hideLoadingAfterDefaultRender: hookOptions.hideLoadingAfterDefaultRender ?? true
  })
}
