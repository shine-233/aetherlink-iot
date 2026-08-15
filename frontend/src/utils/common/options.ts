import { $t } from '@/locales'

/**
 * Convert a string record into value/label options for select-like controls.
 */
export function transformRecordToOption<T extends Record<string, string>>(record: T) {
  return Object.entries(record).map(([value, label]) => ({
    value,
    label
  })) as CommonType.Option<keyof T>[]
}

/**
 * Translate option labels that are stored as i18n keys.
 */
export function translateOptions(options: CommonType.Option<string>[]) {
  return options.map(option => ({
    ...option,
    label: $t(option.label as App.I18n.I18nKey)
  }))
}

export function transformObjectToOption<T extends object>(obj: T) {
  return Object.entries(obj).map(([value, label]) => ({
    value,
    label
  })) as Common2.OptionWithKey<keyof T>[]
}
