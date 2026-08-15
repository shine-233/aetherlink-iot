/**
 * 文件说明：
 * - 承接联动前提编辑器中的本地化条件选项与轻量派生值。
 * - 负责状态、比较符、周期、星期、失效时间和时间条件选项的统一构造。
 * 维护提示：
 * - 这里只收口与文案/选项装配相关的轻状态，不应重新吸收父组件的生命周期、设备来源或条件组编排逻辑。
 */
import { computed } from 'vue'
import {
  buildCycleOptions,
  buildExpirationTimeOptions,
  buildMonthRangeOptions,
  buildTimeConditionOptions,
  buildWeekOptions
} from './premise-schedule-condition-state'

type Translator = (key: string) => string

export const createPremiseLocalizedConditionOptions = (t: Translator) => {
  const getTimeConditionOptions = (ifGroup: any[]) => buildTimeConditionOptions(ifGroup, t)

  const statusData = computed(() => ({
    value: 'status',
    label: t('page.automation.linkage.premise.status.label'),
    options: [
      {
        value: 'status/On-line',
        label: t('page.automation.linkage.premise.status.options.online'),
        key: 'On-line'
      },
      {
        value: 'status/Off-line',
        label: t('page.automation.linkage.premise.status.options.offline'),
        key: 'Off-line'
      },
      {
        value: 'status/All',
        label: t('page.automation.linkage.premise.status.options.all'),
        key: 'All'
      }
    ]
  }))

  const cycleOptions = computed(() => buildCycleOptions(t))
  const weekOptions = computed(() => buildWeekOptions(t))

  const determineOptions = computed(() => [
    {
      label: t('common.equal'),
      value: '='
    },
    {
      label: t('common.unequal'),
      value: '!='
    },
    {
      label: t('common.pass'),
      value: '>'
    },
    {
      label: t('common.under'),
      value: '<'
    },
    {
      label: t('common.greaterOrEqual'),
      value: '>='
    },
    {
      label: t('common.lessOrEqual'),
      value: '<='
    },
    {
      label: t('common.between'),
      value: 'between'
    },
    {
      label: t('common.includeList'),
      value: 'in'
    }
  ])

  const expirationTimeOptions = computed(() => buildExpirationTimeOptions(t))
  const monthRangeOptions = buildMonthRangeOptions()

  return {
    getTimeConditionOptions,
    statusData,
    cycleOptions,
    weekOptions,
    determineOptions,
    expirationTimeOptions,
    monthRangeOptions
  }
}
