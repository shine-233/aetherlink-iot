<!--
  文件用途:
  遥测详情中的图表承载组件，负责把父组件构造好的 ECharts 配置渲染到页面。

  数据流:
  1. 父组件通过 initialOptions 传入完整图表配置。
  2. useTpECharts 负责实例创建、容器绑定与 updateOptions 更新。
  3. watch 监听配置变化后，将新旧配置合并并推送给图表实例。

  使用注意:
  1. 该组件不生成业务数据，只消费外部 option，配置正确性取决于调用方。
  2. 当前更新策略为浅层对象合并，若 option 内存在数组或深层嵌套对象，需确认是否符合预期覆盖方式。
  3. 组件高度依赖容器尺寸，外层布局若未提供有效高度会导致图表不可见。

  静态审查建议:
  1. 重点检查 initialOptions 的引用变化频率，避免深度 watch 在大对象场景下带来额外开销。
  2. updateOptions 内部使用展开合并，审查时要确认不会保留过期子配置。
  3. 若后续接入更多图表组件，建议统一 option 输入约束与销毁时机测试。
-->
<script setup lang="ts">
import { watch } from 'vue'
import type { EChartsCoreOption } from 'echarts/core'
import { useTpECharts } from '@/hooks/tp-chart/use-tp-echarts'

// 图表配置完全由父组件提供；该组件只做渲染与更新桥接。
const props = defineProps<{
  initialOptions: EChartsCoreOption
}>()

// useTpECharts 持有真实图表实例，domRef 绑定容器，updateOptions 负责增量更新。
const { domRef, updateOptions } = useTpECharts(
  () => props.initialOptions,
  {},
  {
    requiredExtensions: ['dataZoom', 'toolbox']
  }
)

// 监听外部 option 变化并推送到图表实例。
// 静态审查建议：deep watch 对大型配置对象可能较敏感，后续如出现性能问题可优先排查这里。
watch(
  () => props.initialOptions,
  newOptions => {
    if (newOptions) {
      updateOptions(currentOptions => {
        // 这里通过浅合并保留旧配置中未被新配置覆盖的字段。
        // 使用注意：若期望“完全替换”某个嵌套配置，需要在调用方传入完整结构。
        return { ...currentOptions, ...newOptions }
      })
    }
  },
  { deep: true, immediate: true }
)
</script>

<template>
  <div ref="domRef" class="chart-container"></div>
</template>

<style scoped>
.chart-container {
  width: 100%;
  height: 100%;
}
</style>
