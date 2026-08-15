import { repeat } from 'seemly'

type Translator = (key: string) => string

export const createScheduleConditionFields = () => ({
  task_type: null,
  params: null,
  execution_time: null,
  expiration_time: null,
  timeValue: null,
  onceTimeValue: null,
  hourTimeValue: null,
  dayTimeValue: null,
  weekTimeValue: null,
  monthTimeValue: null,
  weekChoseValue: [],
  monthChoseValue: null,
  startTimeValue: null,
  endTimeValue: null
})

export const buildTimeConditionOptions = (ifGroup: any[] = [], t: Translator) => {
  const hasDeviceCondition = ifGroup.some((item) => item.ifType === '1')
  return [
    {
      label: t('common.single'),
      value: '20',
      disabled: hasDeviceCondition
    },
    {
      label: t('common.repeat'),
      value: '21',
      disabled: hasDeviceCondition
    },
    {
      label: t('common.timeFrame'),
      value: '22'
    }
  ]
}

export const buildCycleOptions = (t: Translator) => [
  {
    label: t('common.everyHour'),
    value: 'HOUR'
  },
  {
    label: t('common.everyDay'),
    value: 'DAY'
  },
  {
    label: t('common.weekly'),
    value: 'WEEK'
  },
  {
    label: t('common.monthly'),
    value: 'MONTH'
  }
]

export const buildWeekOptions = (t: Translator) => [
  {
    label: t('common.monday'),
    value: '1'
  },
  {
    label: t('common.tuesday'),
    value: '2'
  },
  {
    label: t('common.wednesday'),
    value: '3'
  },
  {
    label: t('common.thursday'),
    value: '4'
  },
  {
    label: t('common.friday'),
    value: '5'
  },
  {
    label: t('common.saturday'),
    value: '6'
  },
  {
    label: t('common.sunday'),
    value: '7'
  }
]

export const buildExpirationTimeOptions = (t: Translator) => [
  {
    label: t('common.minutes5'),
    value: 5
  },
  {
    label: t('common.minutes10'),
    value: 10
  },
  {
    label: t('common.minutes30'),
    value: 30
  },
  {
    label: t('common.hours1'),
    value: 60
  },
  {
    label: t('common.days1'),
    value: 1440
  }
]

export const buildMonthRangeOptions = () =>
  repeat(31, undefined).map((_, i) => ({
    label: String(i + 1),
    value: i + 1
  }))

export const resetRepeatScheduleFields = (ifItem: any) => {
  ifItem.hourTimeValue = null
  ifItem.expiration_time = null
  ifItem.dayTimeValue = null
  ifItem.weekTimeValue = null
  ifItem.monthChoseValue = null
  ifItem.weekChoseValue = null
  ifItem.monthTimeValue = null
}
